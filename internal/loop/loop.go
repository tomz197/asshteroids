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
const initialAsteroids = 4

// Run starts the main game loop with the standard Input → Update → Draw cycle.
func Run(r *bufio.Reader, w io.Writer) error {
	state := NewState()
	stream := input.StartStream(r)

	draw.HideCursor(w)
	defer draw.ShowCursor(w)
	draw.ClearScreen(w)

	// Get initial screen size and spawn user at center
	screen, err := draw.TerminalSize()
	if err != nil {
		return err
	}
	state.Screen = object.Screen{
		Width:   screen.Width,
		Height:  screen.Height,
		CenterX: screen.CenterX,
		CenterY: screen.CenterY,
	}

	user := object.NewUser(float64(screen.CenterX), float64(screen.CenterY))
	state.AddObject(user)

	// Spawn initial asteroids
	for i := 0; i < initialAsteroids; i++ {
		asteroid := object.NewAsteroidAtEdge(state.Screen, object.AsteroidLarge)
		state.AddObject(asteroid)
	}

	lastTime := time.Now()

	for state.Running {
		frameStart := time.Now()
		state.Delta = frameStart.Sub(lastTime)
		lastTime = frameStart

		// ===== INPUT PHASE =====
		if err := processInput(state, stream); err != nil {
			return err
		}

		// ===== UPDATE PHASE =====
		if err := updateScreen(state); err != nil {
			return err
		}
		if err := updateObjects(state); err != nil {
			return err
		}

		// ===== COLLISION PHASE =====
		checkCollisions(state)

		// ===== DRAW PHASE =====
		if err := drawFrame(state, w); err != nil {
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
func processInput(state *State, stream *input.Stream) error {
	inp := input.ReadInput(stream)

	state.Input = object.Input{
		Quit:      inp.Quit,
		Left:      inp.Left,
		Right:     inp.Right,
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

// updateScreen fetches the current terminal size.
func updateScreen(state *State) error {
	screen, err := draw.TerminalSize()
	if err != nil {
		return err
	}
	state.Screen = object.Screen{
		Width:   screen.Width,
		Height:  screen.Height,
		CenterX: screen.CenterX,
		CenterY: screen.CenterY,
	}
	return nil
}

// updateObjects updates all objects and removes any that request removal.
func updateObjects(state *State) error {
	ctx := state.UpdateContext()

	// Update objects and collect ones to keep
	kept := state.Objects[:0] // reuse backing array
	for _, obj := range state.Objects {
		remove, err := obj.Update(ctx)
		if err != nil {
			return err
		}
		if !remove {
			kept = append(kept, obj)
		}
	}
	state.Objects = kept

	// Add any newly spawned objects
	state.FlushSpawned()

	return nil
}

// drawFrame clears the screen and draws all objects.
func drawFrame(state *State, w io.Writer) error {
	draw.ClearScreen(w)

	for _, obj := range state.Objects {
		if err := obj.Draw(w); err != nil {
			return err
		}
	}

	return nil
}

// checkCollisions detects and handles collisions between objects.
func checkCollisions(state *State) {
	// Collect projectiles and asteroids
	var projectiles []*object.Projectile
	var asteroids []*object.Asteroid

	for _, obj := range state.Objects {
		switch o := obj.(type) {
		case *object.Projectile:
			projectiles = append(projectiles, o)
		case *object.Asteroid:
			asteroids = append(asteroids, o)
		}
	}

	// Check projectile-asteroid collisions
	for _, p := range projectiles {
		for _, a := range asteroids {
			if a.Destroyed {
				continue
			}
			if collides(p.X, p.Y, a.X, a.Y, a.GetRadius()) {
				// Destroy both projectile and asteroid
				p.Lifetime = 0 // Mark projectile for removal
				a.Hit()        // Mark asteroid for splitting/removal
			}
		}
	}
}

// collides checks if a point is within radius of a target position.
func collides(px, py, tx, ty, radius float64) bool {
	dx := px - tx
	dy := py - ty
	distSq := dx*dx + dy*dy
	return distSq <= radius*radius
}

// distance calculates the distance between two points.
func distance(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	return math.Sqrt(dx*dx + dy*dy)
}
