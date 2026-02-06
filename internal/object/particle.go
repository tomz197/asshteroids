package object

import (
	"math"
	"math/rand"
	"sync"
)

// particlePool is a sync.Pool for reusing Particle objects to reduce allocations.
var particlePool = sync.Pool{
	New: func() any {
		return &Particle{}
	},
}

// Particle is a short-lived visual effect.
type Particle struct {
	X, Y        float64 // Position
	VX, VY      float64 // Velocity
	Lifetime    float64 // Seconds remaining
	MaxLifetime float64 // Initial lifetime (for fade calculation)
	Drag        float64 // Velocity decay (1.0 = no drag)
	Symbol      rune    // Character to display
	Fade        bool    // Whether to fade out over lifetime
}

// NewParticle creates a single particle from the pool.
func NewParticle(x, y, vx, vy, lifetime float64, symbol rune) *Particle {
	p := particlePool.Get().(*Particle)
	p.X = x
	p.Y = y
	p.VX = vx
	p.VY = vy
	p.Lifetime = lifetime
	p.MaxLifetime = lifetime
	p.Drag = 0.95
	p.Symbol = symbol
	p.Fade = true
	return p
}

// Release returns the particle to the pool for reuse.
// Should be called when the particle is removed from the game.
func (p *Particle) Release() {
	particlePool.Put(p)
}

// SpawnExplosion creates particles in a circular burst pattern.
// Returns a slice of particles to be spawned.
func SpawnExplosion(x, y float64, count int, speed, lifetime float64, spawner Spawner) {
	if spawner == nil {
		return
	}

	symbols := []rune{'#', '@', '*', '%', 'X', 'O', '+', '▪'}

	for i := 0; i < count; i++ {
		// Random direction
		angle := rand.Float64() * 2 * math.Pi
		// Random speed variation (50% to 150%)
		spd := speed * (0.5 + rand.Float64())
		// Random lifetime variation (50% to 100%)
		life := lifetime * (0.5 + rand.Float64()*0.5)

		vx := math.Cos(angle) * spd
		vy := math.Sin(angle) * spd

		symbol := symbols[rand.Intn(len(symbols))]

		p := NewParticle(x, y, vx, vy, life, symbol)
		spawner.Spawn(p)
	}
}

// SpawnThrust creates particles behind a thrusting ship.
func SpawnThrust(x, y, angle float64, spawner Spawner) {
	if spawner == nil {
		return
	}

	// Spawn 1-2 particles behind the ship
	count := 1 + rand.Intn(2)
	symbols := []rune{'*', '+', '#', '^', '~'}

	for i := 0; i < count; i++ {
		// Opposite direction of ship facing, with spread
		thrustAngle := angle + math.Pi + (rand.Float64()-0.5)*0.5
		speed := 8.0 + rand.Float64()*4.0
		lifetime := 0.1 + rand.Float64()*0.15

		vx := math.Cos(thrustAngle) * speed
		vy := math.Sin(thrustAngle) * speed

		symbol := symbols[rand.Intn(len(symbols))]

		p := NewParticle(x, y, vx, vy, lifetime, symbol)
		p.Drag = 0.85
		spawner.Spawn(p)
	}
}

// Update moves the particle and checks lifetime.
func (p *Particle) Update(ctx UpdateContext) (bool, error) {
	dt := ctx.Delta.Seconds()

	// Decrease lifetime
	p.Lifetime -= dt
	if p.Lifetime <= 0 {
		return true, nil // Remove particle
	}

	// Apply drag
	dragFactor := math.Pow(p.Drag, dt*60) // Normalize drag to ~60fps
	p.VX *= dragFactor
	p.VY *= dragFactor

	// Apply velocity
	p.X += p.VX * dt
	p.Y += p.VY * dt

	// No screen wrapping for particles - they just disappear at edges
	// (optional: could add wrapping if desired)

	return false, nil
}

// Draw renders the particle as a pixel on the canvas.
func (p *Particle) Draw(ctx DrawContext) error {
	// Skip faded particles (< 25% lifetime)
	if p.Fade && p.MaxLifetime > 0 {
		if p.Lifetime/p.MaxLifetime < 0.25 {
			return nil
		}
	}

	// Get screen positions (handles world wrapping)
	positions := WorldToScreen(p.X, p.Y, ctx.Camera, ctx.View, ctx.World)
	for i := 0; i < positions.Count; i++ {
		pos := positions.Positions[i]
		ctx.Canvas.SetFloat(pos.X, pos.Y)
	}

	return nil
}
