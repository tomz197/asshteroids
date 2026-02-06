package loop

import (
	"bufio"
	"fmt"
	"io"
	"time"

	"github.com/tomz197/asteroids/internal/draw"
	"github.com/tomz197/asteroids/internal/input"
	"github.com/tomz197/asteroids/internal/object"
)

const clientTargetFPS = 60
const clientTargetFrameTime = time.Second / clientTargetFPS

// Client handles rendering and input for a single connection.
type Client struct {
	server       GameServer
	handle       *ClientHandle
	state        *ClientState
	canvas       *draw.Canvas
	reader       *bufio.Reader
	writer       io.Writer
	inputStream  *input.Stream
	termSizeFunc draw.TermSizeFunc
}

// ClientOptions configures the client.
type ClientOptions struct {
	TermSizeFunc draw.TermSizeFunc
}

// NewClient creates a new client connected to the given server.
func NewClient(server GameServer, r *bufio.Reader, w io.Writer, opts ClientOptions) *Client {
	termSizeFunc := opts.TermSizeFunc
	if termSizeFunc == nil {
		termSizeFunc = draw.DefaultTermSizeFunc
	}

	handle := server.RegisterClient()
	state := NewClientState()
	state.termSizeFunc = termSizeFunc

	// Set up view dimensions
	state.View = object.Screen{
		Width:   viewWidth,
		Height:  viewHeight,
		CenterX: viewWidth / 2,
		CenterY: viewHeight / 2,
	}

	// Camera starts at world center
	state.Camera = object.Camera{
		X: float64(worldWidth) / 2,
		Y: float64(worldHeight) / 2,
	}

	// Create canvas
	termWidth, termHeight, _ := draw.TerminalSizeRawWith(termSizeFunc)
	canvas := draw.NewScaledCanvas(termWidth, termHeight, viewWidth, viewHeight)

	return &Client{
		server:       server,
		handle:       handle,
		state:        state,
		canvas:       canvas,
		reader:       r,
		writer:       w,
		inputStream:  input.StartStream(r),
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
		if elapsed < clientTargetFrameTime {
			time.Sleep(clientTargetFrameTime - elapsed)
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
			case EventPlayerDied:
				c.state.Lives--
				c.state.GameState = GameStateDead
				c.state.Player = nil
			case EventScoreAdd:
				c.state.Score += event.ScoreAdd
			case EventServerShutdown:
				c.state.GameState = GameStateShutdown
				c.state.shutdownTimer = shutdownDisplaySeconds
			}
		default:
			return
		}
	}
}

// updateScreen handles terminal resize.
func (c *Client) updateScreen() {
	termWidth, termHeight, err := draw.TerminalSizeRawWith(c.termSizeFunc)
	if err != nil {
		return
	}
	c.canvas.Resize(termWidth, termHeight)
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
	if c.state.Input.Space || c.state.Input.Enter {
		c.startGame()
	}
}

// startGame starts or restarts the game.
func (c *Client) startGame() {
	input.ResetKeyInput(c.inputStream)

	if c.state.GameState == GameStateStart || c.state.Lives <= 0 {
		// Full restart
		c.state.Score = 0
		c.state.Lives = InitialLives
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
	c.state.InvincibleTime = InvincibilitySeconds

	c.state.GameState = GameStatePlaying
}

// updateShutdownState handles the shutdown screen countdown.
func (c *Client) updateShutdownState() {
	c.state.shutdownTimer -= c.state.delta.Seconds()
	if c.state.shutdownTimer <= 0 {
		c.state.Running = false
	}
}

// drawFrame draws the current frame.
func (c *Client) drawFrame() error {
	draw.ClearScreen(c.writer)
	c.canvas.Clear()

	// Get world snapshot
	snapshot := c.server.GetSnapshot()

	// Create draw context
	ctx := object.DrawContext{
		Canvas: c.canvas,
		Writer: c.writer,
		Camera: c.state.Camera,
		View:   c.state.View,
		World:  snapshot.World,
	}

	// Draw all objects from snapshot
	for _, obj := range snapshot.Objects {
		// Skip drawing player when blinking (invincible)
		if obj == c.state.Player && !object.ShouldRenderBlink(c.state.InvincibleTime, PlayerBlinkFrequency) {
			continue
		}
		if err := obj.Draw(ctx); err != nil {
			return err
		}
	}

	// Render canvas to terminal
	c.canvas.Render(c.writer)

	// Draw UI overlay
	c.drawUI()

	return nil
}

// drawUI draws the game UI overlay.
func (c *Client) drawUI() {
	termWidth := c.canvas.TerminalWidth()
	termHeight := c.canvas.TerminalHeight()
	centerX := termWidth / 2
	centerY := termHeight / 2

	switch c.state.GameState {
	case GameStateShutdown:
		c.drawShutdownScreen(centerX, centerY)
	case GameStatePlaying:
		c.drawPlayingHUD(termWidth, termHeight)
	case GameStateStart:
		c.drawStartScreen(centerX, centerY)
	case GameStateDead:
		c.drawDeadScreen(centerX, centerY)
	}
}

// drawStartScreen draws the title screen.
func (c *Client) drawStartScreen(centerX, centerY int) {
	title := "A S T E R O I D S"
	draw.MoveCursor(c.writer, centerX-len(title)/2, centerY-2)
	fmt.Fprint(c.writer, title)

	subtitle := "Press SPACE to Start"
	draw.MoveCursor(c.writer, centerX-len(subtitle)/2, centerY+1)
	fmt.Fprint(c.writer, subtitle)

	controls := "Controls: A/D or Arrows to rotate, W or Up to thrust, SPACE to shoot, Q to quit"
	draw.MoveCursor(c.writer, centerX-len(controls)/2, centerY+4)
	fmt.Fprint(c.writer, controls)
}

// drawPlayingHUD draws the in-game HUD.
func (c *Client) drawPlayingHUD(termWidth, termHeight int) {
	snapshot := c.server.GetSnapshot()
	// Score display (top left)
	scoreText := fmt.Sprintf("Score: %d", c.state.Score)
	draw.MoveCursor(c.writer, 2, 1)
	fmt.Fprint(c.writer, scoreText)

	// Lives display (top right)
	livesText := fmt.Sprintf("Lives: %d", c.state.Lives)
	draw.MoveCursor(c.writer, termWidth-len(livesText)-1, 1)
	fmt.Fprint(c.writer, livesText)

	// Live players (bottom right)
	livePlayersText := fmt.Sprintf("Players: %d", snapshot.Players)
	draw.MoveCursor(c.writer, termWidth-len(livePlayersText)-1, termHeight)
	fmt.Fprint(c.writer, livePlayersText)

	// Coordinates display (bottom left)
	if c.state.Player != nil {
		px, py := c.state.Player.GetPosition()
		coordText := fmt.Sprintf("X:%.0f Y:%.0f", px, py)
		draw.MoveCursor(c.writer, 2, termHeight)
		fmt.Fprint(c.writer, coordText)
	}
}

// drawDeadScreen draws the death/game over screen.
func (c *Client) drawDeadScreen(centerX, centerY int) {
	var title string
	if c.state.Lives > 0 {
		title = "YOU DIED"
	} else {
		title = "GAME OVER"
	}
	draw.MoveCursor(c.writer, centerX-len(title)/2, centerY-2)
	fmt.Fprint(c.writer, title)

	scoreText := fmt.Sprintf("Score: %d", c.state.Score)
	draw.MoveCursor(c.writer, centerX-len(scoreText)/2, centerY)
	fmt.Fprint(c.writer, scoreText)

	var prompt string
	if c.state.Lives > 0 {
		prompt = fmt.Sprintf("Lives remaining: %d - Press SPACE to continue", c.state.Lives)
	} else {
		prompt = "Press SPACE to Restart"
	}
	draw.MoveCursor(c.writer, centerX-len(prompt)/2, centerY+2)
	fmt.Fprint(c.writer, prompt)
}

// drawShutdownScreen draws the server shutdown notification screen.
func (c *Client) drawShutdownScreen(centerX, centerY int) {
	title := "SERVER SHUTTING DOWN"
	draw.MoveCursor(c.writer, centerX-len(title)/2, centerY-3)
	fmt.Fprint(c.writer, title)

	msg1 := "The server is restarting for maintenance."
	draw.MoveCursor(c.writer, centerX-len(msg1)/2, centerY-1)
	fmt.Fprint(c.writer, msg1)

	msg2 := "Please reconnect in a moment."
	draw.MoveCursor(c.writer, centerX-len(msg2)/2, centerY)
	fmt.Fprint(c.writer, msg2)

	remaining := int(c.state.shutdownTimer) + 1
	countdown := fmt.Sprintf("Disconnecting in %d seconds...", remaining)
	draw.MoveCursor(c.writer, centerX-len(countdown)/2, centerY+2)
	fmt.Fprint(c.writer, countdown)

	hint := "Press Q to disconnect now"
	draw.MoveCursor(c.writer, centerX-len(hint)/2, centerY+4)
	fmt.Fprint(c.writer, hint)
}
