package object

import (
	"math"
	"math/rand"

	"github.com/tomz197/asteroids/internal/draw"
)

// AsteroidSize represents the size category of an asteroid.
type AsteroidSize int

const (
	AsteroidSmall  AsteroidSize = 1
	AsteroidMedium AsteroidSize = 2
	AsteroidLarge  AsteroidSize = 3
)

// Size properties for each asteroid size.
var asteroidRadii = map[AsteroidSize]float64{
	AsteroidSmall:  1.5,
	AsteroidMedium: 3.0,
	AsteroidLarge:  5.0,
}

var asteroidSpeeds = map[AsteroidSize]float64{
	AsteroidSmall:  15.0,
	AsteroidMedium: 10.0,
	AsteroidLarge:  6.0,
}

// Asteroid is a destructible space rock.
type Asteroid struct {
	X, Y            float64      // Position (center)
	VX, VY          float64      // Velocity
	Angle           float64      // Current rotation angle
	RotationSpeed   float64      // Rotation speed (radians/sec)
	Size            AsteroidSize // Size category
	Radius          float64      // Collision/draw radius
	Vertices        []float64    // Vertex distances from center (for irregular shape)
	Destroyed       bool         // Mark for removal and splitting
	SpawnProtection float64      // Seconds of invulnerability remaining after spawn
}

// NewAsteroid creates an asteroid at position (x,y) with the given size.
// Direction is random if angle is < 0.
func NewAsteroid(x, y float64, size AsteroidSize, angle float64) *Asteroid {
	radius := asteroidRadii[size]
	speed := asteroidSpeeds[size]

	// Random direction if not specified
	if angle < 0 {
		angle = rand.Float64() * 2 * math.Pi
	}

	// Random rotation speed (-1 to 1 radians/sec)
	rotSpeed := (rand.Float64() - 0.5) * 2.0

	// Generate irregular polygon vertices (8-12 vertices)
	numVerts := 8 + rand.Intn(5)
	vertices := make([]float64, numVerts)
	for i := 0; i < numVerts; i++ {
		// Vary radius by ±30% for irregular shape
		vertices[i] = radius * (0.7 + rand.Float64()*0.6)
	}

	return &Asteroid{
		X:             x,
		Y:             y,
		VX:            math.Cos(angle) * speed,
		VY:            math.Sin(angle) * speed,
		Angle:         rand.Float64() * 2 * math.Pi,
		RotationSpeed: rotSpeed,
		Size:          size,
		Radius:        radius,
		Vertices:      vertices,
	}
}

// NewAsteroidAtEdge creates an asteroid at a random screen edge.
func NewAsteroidAtEdge(screen Screen, size AsteroidSize) *Asteroid {
	var x, y float64
	w := float64(screen.Width)
	h := float64(screen.Height)

	// Pick a random edge
	switch rand.Intn(4) {
	case 0: // Top
		x = rand.Float64() * w
		y = 1
	case 1: // Bottom
		x = rand.Float64() * w
		y = h - 1
	case 2: // Left
		x = 1
		y = rand.Float64() * h
	case 3: // Right
		x = w - 1
		y = rand.Float64() * h
	}

	// Aim roughly toward center with some randomness
	centerX := w / 2
	centerY := h / 2
	angle := math.Atan2(centerY-y, centerX-x)
	angle += (rand.Float64() - 0.5) * math.Pi / 2 // ±45° variation

	return NewAsteroid(x, y, size, angle)
}

// NewAsteroidRandom creates an asteroid at a random position in the world.
// The asteroid has spawn protection for the specified duration.
func NewAsteroidRandom(screen Screen, size AsteroidSize, spawnProtection float64) *Asteroid {
	w := float64(screen.Width)
	h := float64(screen.Height)

	// Random position anywhere in the world
	x := rand.Float64() * w
	y := rand.Float64() * h

	// Random direction
	angle := rand.Float64() * 2 * math.Pi

	asteroid := NewAsteroid(x, y, size, angle)
	asteroid.SpawnProtection = spawnProtection
	return asteroid
}

// IsProtected returns true if the asteroid still has spawn protection.
func (a *Asteroid) IsProtected() bool {
	return a.SpawnProtection > 0
}

// Update moves the asteroid and handles rotation.
func (a *Asteroid) Update(ctx UpdateContext) (bool, error) {
	if a.Destroyed {
		// Spawn explosion particles
		particleCount := int(a.Size) * 4 // More particles for larger asteroids
		SpawnExplosion(a.X, a.Y, particleCount, 20.0, 0.5, ctx.Spawner)

		// Spawn smaller asteroids if not the smallest size
		if a.Size > AsteroidSmall && ctx.Spawner != nil {
			// Spawn 2 smaller asteroids
			newSize := a.Size - 1
			for i := 0; i < 2; i++ {
				// Random direction for fragments
				angle := rand.Float64() * 2 * math.Pi
				child := NewAsteroid(a.X, a.Y, newSize, angle)
				ctx.Spawner.Spawn(child)
			}
		}
		return true, nil // Remove this asteroid
	}

	dt := ctx.Delta.Seconds()

	// Decrement spawn protection
	if a.SpawnProtection > 0 {
		a.SpawnProtection -= dt
		if a.SpawnProtection < 0 {
			a.SpawnProtection = 0
		}
	}

	// Rotate
	a.Angle += a.RotationSpeed * dt

	// Move
	a.X += a.VX * dt
	a.Y += a.VY * dt

	// Screen wrapping
	ctx.Screen.WrapPosition(&a.X, &a.Y)

	return false, nil
}

// Draw renders the asteroid as an irregular polygon.
func (a *Asteroid) Draw(ctx DrawContext) error {
	// Blink when protected (skip drawing in "off" phase)
	if !ShouldRenderBlink(a.SpawnProtection, 5.0) {
		return nil
	}

	// Get screen positions (handles world wrapping)
	positions := WorldToScreen(a.X, a.Y, ctx.Camera, ctx.View, ctx.World)

	for i := 0; i < positions.Count; i++ {
		pos := positions.Positions[i]
		a.drawAt(ctx, pos.X, pos.Y)
	}

	return nil
}

// drawAt draws the asteroid at a specific screen position.
func (a *Asteroid) drawAt(ctx DrawContext, screenX, screenY float64) {
	numVerts := len(a.Vertices)

	// Use reusable buffer from canvas to avoid per-frame allocations.
	// Safe for concurrent rendering because each client has its own Canvas.
	points := ctx.Canvas.BorrowPoints(numVerts)

	for i, dist := range a.Vertices {
		// Angle for this vertex
		vertAngle := a.Angle + float64(i)*2*math.Pi/float64(numVerts)
		points[i] = draw.Point{
			X: screenX + math.Cos(vertAngle)*dist,
			Y: screenY + math.Sin(vertAngle)*dist,
		}
	}

	// Draw to canvas (no aspect ratio needed with 2x vertical resolution)
	ctx.Canvas.DrawPolygon(points, false)
}

// MarkDestroyed marks the asteroid for removal (implements Destructible).
func (a *Asteroid) MarkDestroyed() {
	a.Destroyed = true
}

// IsDestroyed returns true if the asteroid is marked for destruction (implements Destructible).
func (a *Asteroid) IsDestroyed() bool {
	return a.Destroyed
}

// GetPosition returns the asteroid's center position.
func (a *Asteroid) GetPosition() (float64, float64) {
	return a.X, a.Y
}

// GetRadius returns the asteroid's collision radius.
func (a *Asteroid) GetRadius() float64 {
	return a.Radius
}
