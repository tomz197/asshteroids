// Package loop provides the main game loop and state management.
package loop

import (
	"bufio"
	"io"
	"time"

	"github.com/tomz197/asteroids/internal/draw"
	"github.com/tomz197/asteroids/internal/input"
	"github.com/tomz197/asteroids/internal/object"
)

const targetFPS = 60
const targetFrameTime = time.Second / targetFPS

// View resolution - the visible viewport in logical units.
// Actual rendering scales to fit terminal size.
const (
	viewWidth  = 120 // Logical viewport width
	viewHeight = 80  // Logical viewport height (in sub-pixels, so 40 terminal rows)
)

// World dimensions - the total game area (larger than viewport).
// Ship stays centered while the camera follows it.
const (
	worldWidth  = 400 // Total world width
	worldHeight = 300 // Total world height
)

// Options configures the game loop.
type Options struct {
	// TermSizeFunc provides terminal dimensions. If nil, uses default (os.Stdout).
	TermSizeFunc draw.TermSizeFunc
}

// Run starts the main game loop with the standard Input → Update → Draw cycle.
// Uses default terminal size detection from os.Stdout.
// This is the legacy single-threaded mode for backward compatibility.
func Run(r *bufio.Reader, w io.Writer) error {
	return RunWithOptions(r, w, Options{})
}

// RunWithOptions starts the game loop with custom options.
// This is the legacy single-threaded mode for backward compatibility.
func RunWithOptions(r *bufio.Reader, w io.Writer, opts Options) error {
	termSizeFunc := opts.TermSizeFunc
	if termSizeFunc == nil {
		termSizeFunc = draw.DefaultTermSizeFunc
	}

	state := NewState()
	state.InputStream = input.StartStream(r)

	draw.HideCursor(w)
	defer draw.ShowCursor(w)
	draw.ClearScreen(w)

	// View is the visible viewport
	state.View = object.Screen{
		Width:   viewWidth,
		Height:  viewHeight,
		CenterX: viewWidth / 2,
		CenterY: viewHeight / 2,
	}

	// World is the full game area (larger, objects wrap here)
	state.World = object.Screen{
		Width:   worldWidth,
		Height:  worldHeight,
		CenterX: worldWidth / 2,
		CenterY: worldHeight / 2,
	}

	// Screen is set to World for object updates (wrapping happens at world edges)
	state.Screen = state.World

	// Camera starts at world center
	state.Camera = object.Camera{
		X: float64(worldWidth) / 2,
		Y: float64(worldHeight) / 2,
	}

	// Create scaled canvas - maps logical view coordinates to terminal pixels
	termWidth, termHeight, _ := draw.TerminalSizeRawWith(termSizeFunc)
	canvas := draw.NewScaledCanvas(termWidth, termHeight, viewWidth, viewHeight)
	state.termSizeFunc = termSizeFunc

	lastTime := time.Now()

	for state.Running {
		frameStart := time.Now()
		state.WorldState.Delta = frameStart.Sub(lastTime)
		lastTime = frameStart

		// ===== INPUT PHASE =====
		if err := processInput(state); err != nil {
			return err
		}

		// ===== UPDATE PHASE =====
		if err := updateScreen(state, canvas); err != nil {
			return err
		}

		switch state.GameState {
		case GameStateStart:
			updateStartState(state)
		case GameStatePlaying:
			if err := updatePlayingState(state); err != nil {
				return err
			}
			// Update camera to follow player
			updateCamera(state)
		case GameStateDead:
			updateDeadState(state)
		}

		// ===== DRAW PHASE =====
		if err := drawFrame(state, w, canvas); err != nil {
			return err
		}

		// ===== FRAME TIMING =====
		elapsed := time.Since(frameStart)
		if elapsed < targetFrameTime {
			time.Sleep(targetFrameTime - elapsed)
		}
	}

	draw.ClearScreen(w)
	return nil
}

// RunClientServer starts the game in client-server mode.
// The server runs in a separate goroutine and the client runs in the calling goroutine.
// This is the recommended mode for multiplayer support.
func RunClientServer(r *bufio.Reader, w io.Writer, opts Options) error {
	termSizeFunc := opts.TermSizeFunc
	if termSizeFunc == nil {
		termSizeFunc = draw.DefaultTermSizeFunc
	}

	// Create and start server
	server := NewServer()
	go server.Run()
	defer server.Stop()

	// Create and run client
	client := NewClient(server, r, w, ClientOptions{
		TermSizeFunc: termSizeFunc,
	})
	return client.Run()
}

// processInput reads and processes all pending input (legacy single-player).
func processInput(state *State) error {
	state.Input = input.ReadInput(state.InputStream)

	if state.Input.Quit {
		state.Running = false
	}

	return nil
}

// updateScreen checks for terminal resize and updates canvas scaling (legacy single-player).
func updateScreen(state *State, canvas *draw.Canvas) error {
	termWidth, termHeight, err := draw.TerminalSizeRawWith(state.termSizeFunc)
	if err != nil {
		return err
	}

	// Resize canvas if terminal changed (updates scaling)
	canvas.Resize(termWidth, termHeight)

	return nil
}

// updateCamera updates camera position to follow the player (legacy single-player).
func updateCamera(state *State) {
	if state.Player == nil {
		return
	}

	// Camera follows player directly (centered on player)
	px, py := state.Player.GetPosition()
	state.Camera.X = px
	state.Camera.Y = py
}

// drawFrame clears the screen and draws all objects (legacy single-player).
func drawFrame(state *State, w io.Writer, canvas *draw.Canvas) error {
	draw.ClearScreen(w)
	canvas.Clear()

	// Create draw context with camera info
	ctx := object.DrawContext{
		Canvas: canvas,
		Writer: w,
		Camera: state.Camera,
		View:   state.View,
		World:  state.World,
	}

	// Draw all objects to canvas
	for _, obj := range state.Objects {
		// Skip drawing player when blinking (invincible)
		if obj == state.Player && !object.ShouldRenderBlink(state.InvincibleTime, 10.0) {
			continue
		}
		if err := obj.Draw(ctx); err != nil {
			return err
		}
	}

	// Render canvas to terminal
	canvas.Render(w)

	// Draw UI overlay (after canvas render so it's on top)
	drawUI(state, w, canvas)

	return nil
}
