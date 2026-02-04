package object

import (
	"io"
	"time"

	"github.com/tomz197/asteroids/internal/draw"
)

// Spawner allows objects to spawn new objects during update.
type Spawner interface {
	Spawn(obj Object)
}

// UpdateContext provides all the information an object needs during update.
type UpdateContext struct {
	Delta   time.Duration
	Input   Input
	Screen  Screen
	Spawner Spawner
	Objects []Object
}

// DrawContext provides drawing resources for objects.
type DrawContext struct {
	Canvas *draw.Canvas // High-resolution canvas (2x vertical)
	Writer io.Writer    // Direct terminal output (for text/particles)
}

// Input represents the current input state.
type Input struct {
	Quit      bool
	Left      bool
	Right     bool
	UpLeft    bool
	UpRight   bool
	Up        bool
	Down      bool
	Space     bool
	Enter     bool
	Backspace bool
	Delete    bool
	Escape    bool
	Number    int
	Pressed   []byte
}

// Screen represents terminal dimensions.
type Screen struct {
	Width   int
	Height  int
	CenterX int
	CenterY int
}

// WrapPosition wraps x and y coordinates around screen boundaries (Asteroids-style).
func (s Screen) WrapPosition(x, y *float64) {
	w := float64(s.Width)
	h := float64(s.Height)

	if w > 0 {
		if *x < 0 {
			*x += w
		} else if *x > w {
			*x -= w
		}
	}
	if h > 0 {
		if *y < 0 {
			*y += h
		} else if *y > h {
			*y -= h
		}
	}
}

// Object is a drawable and updatable game entity.
type Object interface {
	// Update updates the object state. Returns true if the object should be removed.
	Update(ctx UpdateContext) (remove bool, err error)

	// Draw draws the object. Use ctx.Canvas for high-res shapes, ctx.Writer for text/particles.
	Draw(ctx DrawContext) error
}
