// Package loop provides the main game loop and state management.
package loop

import (
	"bufio"
	"io"
	"math"
	"time"

	"github.com/tomz197/asteroids/internal/draw"
	"github.com/tomz197/asteroids/internal/input"
	"github.com/tomz197/asteroids/internal/object"
)

const targetFPS = 60
const targetFrameTime = time.Second / targetFPS

// Target resolution - game objects use these logical dimensions.
// Actual rendering scales to fit terminal size.
const (
	targetWidth  = 120 // Logical width
	targetHeight = 80  // Logical height (in sub-pixels, so 40 terminal rows)
)

// Options configures the game loop.
type Options struct {
	// TermSizeFunc provides terminal dimensions. If nil, uses default (os.Stdout).
	TermSizeFunc draw.TermSizeFunc
}

// Run starts the main game loop with the standard Input → Update → Draw cycle.
// Uses default terminal size detection from os.Stdout.
func Run(r *bufio.Reader, w io.Writer) error {
	return RunWithOptions(r, w, Options{})
}

// RunWithOptions starts the game loop with custom options.
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

	// Game uses fixed logical resolution
	state.Screen = object.Screen{
		Width:   targetWidth,
		Height:  targetHeight,
		CenterX: targetWidth / 2,
		CenterY: targetHeight / 2,
	}

	// Create scaled canvas - maps logical coordinates to terminal pixels
	termWidth, termHeight, _ := draw.TerminalSizeRawWith(termSizeFunc)
	canvas := draw.NewScaledCanvas(termWidth, termHeight, targetWidth, targetHeight)
	state.termSizeFunc = termSizeFunc

	lastTime := time.Now()

	for state.Running {
		frameStart := time.Now()
		state.Delta = frameStart.Sub(lastTime)
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

// processInput reads and processes all pending input.
func processInput(state *State) error {
	inp := input.ReadInput(state.InputStream)

	state.Input = object.Input{
		Quit:      inp.Quit,
		Left:      inp.Left,
		Right:     inp.Right,
		UpLeft:    inp.UpLeft,
		UpRight:   inp.UpRight,
		Up:        inp.Up,
		Down:      inp.Down,
		Space:     inp.Space,
		Enter:     inp.Enter,
		Backspace: inp.Backspace,
		Delete:    inp.Delete,
		Escape:    inp.Escape,
		Number:    inp.Number,
		Pressed:   inp.Pressed,
	}

	if inp.Quit {
		state.Running = false
	}

	return nil
}

// updateScreen checks for terminal resize and updates canvas scaling.
func updateScreen(state *State, canvas *draw.Canvas) error {
	termWidth, termHeight, err := draw.TerminalSizeRawWith(state.termSizeFunc)
	if err != nil {
		return err
	}

	// Resize canvas if terminal changed (updates scaling)
	canvas.Resize(termWidth, termHeight)

	return nil
}

// drawFrame clears the screen and draws all objects.
func drawFrame(state *State, w io.Writer, canvas *draw.Canvas) error {
	draw.ClearScreen(w)
	canvas.Clear()

	// Create draw context
	ctx := object.DrawContext{
		Canvas: canvas,
		Writer: w,
	}

	// Draw all objects to canvas
	for _, obj := range state.Objects {
		// Skip drawing player when blinking (invincible)
		if obj == state.Player && state.InvincibleTime > 0 {
			// Blink at ~10Hz: skip drawing when in "off" phase
			_, frac := math.Modf(state.InvincibleTime * 10)
			if frac < 0.5 {
				continue
			}
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
