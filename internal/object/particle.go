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
}

// NewParticle creates a single particle from the pool.
func NewParticle(x, y, vx, vy, lifetime float64) *Particle {
	p := particlePool.Get().(*Particle)
	p.X = x
	p.Y = y
	p.VX = vx
	p.VY = vy
	p.Lifetime = lifetime
	p.MaxLifetime = lifetime
	p.Drag = 0.95
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

	for i := 0; i < count; i++ {
		angle := rand.Float64() * 2 * math.Pi
		spd := speed * (0.5 + rand.Float64())
		life := lifetime * (0.5 + rand.Float64()*0.5)

		vx := math.Cos(angle) * spd
		vy := math.Sin(angle) * spd

		p := NewParticle(x, y, vx, vy, life)
		spawner.Spawn(p)
	}
}

// SpawnThrust creates particles behind a thrusting ship.
func SpawnThrust(x, y, angle float64, spawner Spawner) {
	if spawner == nil {
		return
	}

	count := 1 + rand.Intn(2)

	for i := 0; i < count; i++ {
		thrustAngle := angle + math.Pi + (rand.Float64()-0.5)*0.5
		speed := 8.0 + rand.Float64()*4.0
		lifetime := 0.1 + rand.Float64()*0.15

		vx := math.Cos(thrustAngle) * speed
		vy := math.Sin(thrustAngle) * speed

		p := NewParticle(x, y, vx, vy, lifetime)
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

	// Apply drag (frame-rate-independent linear approximation of exponential decay)
	dragFactor := 1.0 - (1.0-p.Drag)*(dt*60)
	if dragFactor < 0 {
		dragFactor = 0
	}
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
	// Skip faded particles (< 25% lifetime remaining)
	if p.MaxLifetime > 0 && p.Lifetime/p.MaxLifetime < 0.25 {
		return nil
	}

	// Get screen positions (handles world wrapping)
	positions := WorldToScreen(p.X, p.Y, ctx.Camera, ctx.View, ctx.World)
	for i := 0; i < positions.Count; i++ {
		pos := positions.Positions[i]
		ctx.Canvas.SetFloat(pos.X, pos.Y)
	}

	return nil
}
