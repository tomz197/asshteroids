package client

import (
	"bufio"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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
	hudBuf       []byte // Reusable buffer for HUD text formatting
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

		// Cursor visibility: show when chat is open for typing
		if c.state.ChatOpen {
			draw.ShowCursor(c.writer)
		} else {
			draw.HideCursor(c.writer)
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
	} else {
		idle := time.Since(c.lastInput).Seconds()
		if idle > config.InactivityDisconnectUser {
			c.state.Running = false
		} else if idle > config.InactivityWarnUser {
			c.state.isInactive = true
		}
	}

	// Chat mode: handle chat-specific input first
	if c.state.ChatOpen {
		if c.state.Input.Escape {
			c.state.ChatOpen = false
			c.state.ChatInput = ""
			input.ResetKeyInput(c.inputStream)
			c.state.Input.Escape = false // Prevent same-frame game action (e.g. dead screen return)
			return
		}
		if c.state.Input.Enter {
			text := c.state.ChatInput
			c.state.ChatOpen = false
			c.state.ChatInput = ""
			input.ResetKeyInput(c.inputStream)
			c.state.Input.Enter = false // Prevent same-frame respawn/start
			c.state.Input.Space = false
			if text != "" {
				c.server.SendChatMessage(c.handle.ID, text)
			}
			return
		}
		if c.state.Input.Backspace || c.state.Input.Delete {
			runes := []rune(c.state.ChatInput)
			if len(runes) > 0 {
				c.state.ChatInput = string(runes[:len(runes)-1])
			}
			return
		}
		// Append printable runes from Pressed
		printable := extractPrintableRunes(c.state.Input.Pressed)
		if len(printable) > 0 {
			var b strings.Builder
			b.WriteString(c.state.ChatInput)
			runeCount := utf8.RuneCountInString(c.state.ChatInput)
			for _, r := range printable {
				if runeCount >= config.MaxChatMessageLength {
					break
				}
				b.WriteRune(r)
				runeCount++
			}
			c.state.ChatInput = b.String()
		}
		return
	}

	// C opens chat (when not already open)
	if c.state.Input.Chat {
		c.state.ChatOpen = true
		input.ResetKeyInput(c.inputStream)
		return
	}

	if c.state.Input.Quit {
		c.state.Running = false
	}

	// Send input to server if playing
	if c.state.GameState == GameStatePlaying {
		c.server.SendInput(c.handle.ID, c.state.Input)
	}
}

// extractPrintableRunes returns printable runes from raw input bytes, skipping control chars and escape sequences.
func extractPrintableRunes(pressed []byte) []rune {
	var result []rune
	for i := 0; i < len(pressed); i++ {
		r, size := utf8.DecodeRune(pressed[i:])
		if size == 0 {
			break
		}
		i += size - 1
		if unicode.IsPrint(r) && r != '\x1b' {
			result = append(result, r)
		}
	}
	return result
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
				c.state.RespawnTimeRemaining = config.RespawnTimeout.Seconds()
				c.state.KilledBy = event.KilledBy
			case server.EventScoreAdd:
				c.state.Score += event.ScoreAdd
			case server.EventServerShutdown:
				c.state.GameState = GameStateShutdown
				c.state.shutdownTimer = config.ShutdownDisplayTime.Seconds()
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
	if c.state.ChatOpen {
		return // Chat consumes input; don't trigger game actions
	}
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
	if c.state.ChatOpen {
		// Chat consumes input; only update respawn timer
		if c.state.RespawnTimeRemaining > 0 {
			c.state.RespawnTimeRemaining -= c.state.delta.Seconds()
			if c.state.RespawnTimeRemaining < 0 {
				c.state.RespawnTimeRemaining = 0
			}
		}
		return
	}
	if c.state.Input.Escape {
		input.ResetKeyInput(c.inputStream)
		c.state.GameState = GameStateStart
		return
	}
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
		c.server.ResetScore(c.handle.ID)
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
	c.state.InvincibleTime = config.InvincibilityTime.Seconds()

	c.state.GameState = GameStatePlaying
}

// updateShutdownState handles the shutdown screen countdown.
func (c *Client) updateShutdownState() {
	c.state.shutdownTimer -= c.state.delta.Seconds()
	if c.state.shutdownTimer <= 0 {
		c.state.Running = false
	}
}
