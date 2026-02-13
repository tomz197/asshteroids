package client

import (
	"bufio"
	"io"
	"time"

	"github.com/tomz197/asteroids/internal/draw"
	"github.com/tomz197/asteroids/internal/input"
	"github.com/tomz197/asteroids/internal/loop/config"
	"github.com/tomz197/asteroids/internal/loop/server"
	"github.com/tomz197/asteroids/internal/object"
)

// Client handles rendering and input for a single connection.
type Client struct {
	server       server.GameServer
	handle       *server.ClientHandle
	state        *ClientState
	canvas       *draw.Canvas
	chunkWriter  *draw.ChunkWriter // Accumulates UI text for chunked output
	reader       *bufio.Reader
	writer       io.Writer
	inputStream  *input.Stream
	lastInput    time.Time
	username     string
	termSizeFunc draw.TermSizeFunc
}

// ClientOptions configures the client.
type ClientOptions struct {
	TermSizeFunc draw.TermSizeFunc
	Username     string
}

// NewClient creates a new client connected to the given server.
func NewClient(gs server.GameServer, r *bufio.Reader, w io.Writer, opts ClientOptions) *Client {
	termSizeFunc := opts.TermSizeFunc
	if termSizeFunc == nil {
		termSizeFunc = draw.DefaultTermSizeFunc
	}

	handle := gs.RegisterClient(opts.Username)
	state := NewClientState()
	state.termSizeFunc = termSizeFunc

	// Set up view dimensions
	state.View = object.Screen{
		Width:   config.ViewWidth,
		Height:  config.ViewHeight,
		CenterX: config.ViewWidth / 2,
		CenterY: config.ViewHeight / 2,
	}

	// Camera starts at world center
	state.Camera = object.Camera{
		X: float64(config.WorldWidth) / 2,
		Y: float64(config.WorldHeight) / 2,
	}

	// Create canvas with clamped dimensions for max render resolution
	termWidth, termHeight, _ := draw.TerminalSizeRawWith(termSizeFunc)
	renderWidth, renderHeight, offsetCol, offsetRow := clampTermSize(termWidth, termHeight)
	canvas := draw.NewScaledCanvas(renderWidth, renderHeight, config.ViewWidth, config.ViewHeight)
	canvas.SetOffset(offsetCol, offsetRow)
	chunkWriter := draw.NewChunkWriter(w, offsetCol, offsetRow)

	return &Client{
		server:       gs,
		handle:       handle,
		state:        state,
		canvas:       canvas,
		chunkWriter:  chunkWriter,
		reader:       r,
		writer:       w,
		lastInput:    time.Now(),
		inputStream:  input.StartStream(r),
		username:     opts.Username,
		termSizeFunc: termSizeFunc,
	}
}

// Run starts the client loop. Blocks until the client disconnects or server stops.
func (c *Client) Run() error {
	draw.HideCursor(c.writer)
	defer draw.ShowCursor(c.writer)
	draw.ClearScreen(c.writer)

	lastTime := time.Now()

	for c.state.Running {
		frameStart := time.Now()
		c.state.delta = frameStart.Sub(lastTime)
		lastTime = frameStart

		// Process input
		c.processInput()

		// Check for server events
		c.processServerEvents()

		// Handle screen resize
		c.updateScreen()

		// Handle game state
		switch c.state.GameState {
		case GameStateStart:
			c.updateStartState()
		case GameStatePlaying:
			c.updatePlayingState()
		case GameStateDead:
			c.updateDeadState()
		case GameStateShutdown:
			c.updateShutdownState()
		}

		// Draw frame
		if err := c.drawFrame(); err != nil {
			return err
		}

		// Frame timing
		elapsed := time.Since(frameStart)
		if elapsed < config.ClientTargetFrameTime {
			time.Sleep(config.ClientTargetFrameTime - elapsed)
		}
	}

	// Unregister from server
	c.server.UnregisterClient(c.handle.ID)

	draw.ClearScreen(c.writer)
	return nil
}

// processInput reads input and sends it to the server.
func (c *Client) processInput() {
	c.state.Input = input.ReadInput(c.inputStream)

	if len(c.state.Input.Pressed) > 0 {
		c.lastInput = time.Now()
		c.state.isInactive = false
	} else if time.Since(c.lastInput).Seconds() > config.InactivityDisconnectUser {
		c.state.Running = false
	} else if time.Since(c.lastInput).Seconds() > config.InactivityWarnUser {
		c.state.isInactive = true
	}

	if c.state.Input.Quit {
		c.state.Running = false
	}

	// Send input to server if playing
	if c.state.GameState == GameStatePlaying {
		c.server.SendInput(c.handle.ID, c.state.Input)
	}
}

// processServerEvents handles events from the server.
func (c *Client) processServerEvents() {
	for {
		select {
		case event, ok := <-c.handle.EventsCh:
			if !ok {
				// Server closed the channel
				c.state.Running = false
				return
			}
			switch event.Type {
			case server.EventPlayerDied:
				c.state.Lives--
				c.state.GameState = GameStateDead
				c.state.Player = nil
				c.state.RespawnTimeRemaining = config.RespawnTimeoutSeconds
			case server.EventScoreAdd:
				c.state.Score += event.ScoreAdd
			case server.EventServerShutdown:
				c.state.GameState = GameStateShutdown
				c.state.shutdownTimer = config.ShutdownDisplaySeconds
			}
		default:
			return
		}
	}
}

// updateScreen handles terminal resize, clamping to max render resolution.
// On actual size changes, clears the terminal to remove residual pixels
// outside the new canvas area (e.g. old borders or offset content).
func (c *Client) updateScreen() {
	termWidth, termHeight, err := draw.TerminalSizeRawWith(c.termSizeFunc)
	if err != nil {
		return
	}
	renderWidth, renderHeight, offsetCol, offsetRow := clampTermSize(termWidth, termHeight)

	if renderWidth != c.canvas.TerminalWidth() || renderHeight != c.canvas.TerminalHeight() ||
		offsetCol != c.canvas.OffsetCol() || offsetRow != c.canvas.OffsetRow() {
		draw.ClearScreen(c.writer)
		c.canvas.ForceRedraw()
	}

	c.canvas.Resize(renderWidth, renderHeight)
	c.canvas.SetOffset(offsetCol, offsetRow)
	c.chunkWriter.SetOffset(offsetCol, offsetRow)
}

// clampTermSize clamps terminal dimensions to the max render resolution and computes
// the centering offset for the render area.
func clampTermSize(termWidth, termHeight int) (renderWidth, renderHeight, offsetCol, offsetRow int) {
	renderWidth = termWidth
	renderHeight = termHeight
	if renderWidth > config.MaxTermWidth {
		renderWidth = config.MaxTermWidth
	}
	if renderHeight > config.MaxTermHeight {
		renderHeight = config.MaxTermHeight
	}
	offsetCol = (termWidth - renderWidth) / 2
	offsetRow = (termHeight - renderHeight) / 2
	return
}

// updateStartState handles the start screen.
func (c *Client) updateStartState() {
	if c.state.Input.Space || c.state.Input.Enter {
		c.startGame()
	}
}

// updatePlayingState handles the playing state.
func (c *Client) updatePlayingState() {
	// Decrement invincibility timer
	if c.state.InvincibleTime > 0 {
		c.state.InvincibleTime -= c.state.delta.Seconds()
		if c.state.InvincibleTime < 0 {
			c.state.InvincibleTime = 0
		}
	}

	// Update camera to follow player
	c.state.Player = c.server.GetClientPlayer(c.handle.ID)
	if c.state.Player != nil {
		px, py := c.state.Player.GetPosition()
		c.state.Camera.X = px
		c.state.Camera.Y = py
	}
}

// updateDeadState handles the death screen.
func (c *Client) updateDeadState() {
	if c.state.RespawnTimeRemaining > 0 {
		c.state.RespawnTimeRemaining -= c.state.delta.Seconds()
		if c.state.RespawnTimeRemaining < 0 {
			c.state.RespawnTimeRemaining = 0
		}
	}
	if (c.state.Input.Space || c.state.Input.Enter) && c.state.RespawnTimeRemaining <= 0 {
		c.startGame()
	}
}

// startGame starts or restarts the game.
func (c *Client) startGame() {
	input.ResetKeyInput(c.inputStream)

	if c.state.GameState == GameStateStart || c.state.Lives <= 0 {
		// Full restart
		c.state.Score = 0
		c.state.Lives = config.InitialLives
	}

	// Request server to spawn player
	c.server.SpawnPlayer(c.handle.ID)
	c.state.Player = c.server.GetClientPlayer(c.handle.ID)

	// Reset camera to player position
	if c.state.Player != nil {
		px, py := c.state.Player.GetPosition()
		c.state.Camera.X = px
		c.state.Camera.Y = py
	}

	// Grant invincibility on spawn
	c.state.InvincibleTime = config.InvincibilitySeconds

	c.state.GameState = GameStatePlaying
}

// updateShutdownState handles the shutdown screen countdown.
func (c *Client) updateShutdownState() {
	c.state.shutdownTimer -= c.state.delta.Seconds()
	if c.state.shutdownTimer <= 0 {
		c.state.Running = false
	}
}
