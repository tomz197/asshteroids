package object

import (
	"io"
	"time"

	"github.com/tomz197/asteroids/internal/draw"
	"github.com/tomz197/asteroids/internal/input"
)

// Spawner allows objects to spawn new objects during update.
type Spawner interface {
	Spawn(obj Object)
}

// Input is an alias for the input package's Input type.
type Input = input.Input

// UpdateContext provides all the information an object needs during update.
type UpdateContext struct {
	Delta   time.Duration
	Input   Input
	Screen  Screen
	Spawner Spawner
	Objects []Object
}

// Camera represents the viewport position in world space.
type Camera struct {
	X, Y float64 // Camera center position in world coordinates
}

// DrawContext provides drawing resources for objects.
type DrawContext struct {
	Canvas *draw.Canvas // High-resolution canvas (2x vertical)
	Writer io.Writer    // Direct terminal output (for text/particles)
	Camera Camera       // Camera position for viewport offset
	View   Screen       // Viewport dimensions (what the camera sees)
	World  Screen       // World dimensions (total game area)
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
		for *x < 0 {
			*x += w
		}
		for *x > w {
			*x -= w
		}
	}
	if h > 0 {
		for *y < 0 {
			*y += h
		}
		for *y > h {
			*y -= h
		}
	}
}

// WorldToScreen converts world coordinates to screen coordinates relative to camera.
// Returns the screen positions where the object should be drawn (handles world wrapping).
func WorldToScreen(worldX, worldY float64, cam Camera, view, world Screen) []draw.Point {
	viewW := float64(view.Width)
	viewH := float64(view.Height)
	worldW := float64(world.Width)
	worldH := float64(world.Height)

	// Camera position is the center of the view
	camLeft := cam.X - viewW/2
	camTop := cam.Y - viewH/2

	// Calculate screen position
	screenX := worldX - camLeft
	screenY := worldY - camTop

	// Wrap the offset to handle world wrapping
	var positions []draw.Point

	// Check all possible wrap positions (original + wrapped copies)
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			sx := screenX + float64(dx)*worldW
			sy := screenY + float64(dy)*worldH

			// Check if this position is within the view (with some margin for large objects)
			margin := 10.0
			if sx >= -margin && sx <= viewW+margin && sy >= -margin && sy <= viewH+margin {
				positions = append(positions, draw.Point{X: sx, Y: sy})
			}
		}
	}

	return positions
}

// Object is a drawable and updatable game entity.
type Object interface {
	// Update updates the object state. Returns true if the object should be removed.
	Update(ctx UpdateContext) (remove bool, err error)

	// Draw draws the object. Use ctx.Canvas for high-res shapes, ctx.Writer for text/particles.
	Draw(ctx DrawContext) error
}

// Destructible is implemented by objects that can be destroyed/marked for removal.
type Destructible interface {
	// MarkDestroyed marks the object for removal on next update cycle.
	MarkDestroyed()
	// IsDestroyed returns true if the object is marked for destruction.
	IsDestroyed() bool
}

// ShouldRenderBlink returns true if an object with remaining protection/invincibility
// time should be rendered this frame (for blinking effect).
// Returns true always if remainingTime <= 0 (no protection).
func ShouldRenderBlink(remainingTime float64, frequency float64) bool {
	if remainingTime <= 0 {
		return true
	}
	// Blink based on frequency (e.g., 5.0 = 5Hz, 10.0 = 10Hz)
	phase := int(remainingTime * frequency)
	return phase%2 != 0
}
