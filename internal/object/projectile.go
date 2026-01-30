package object

import (
	"io"
	"math"

	"github.com/tomz197/asteroids/internal/draw"
)

// Projectile is a bullet fired by the player.
type Projectile struct {
	X, Y     float64 // Position
	VX, VY   float64 // Velocity
	Lifetime float64 // Seconds remaining before removal
	Symbol   rune    // Character to display
}

// ProjectileSpeed is the base speed of projectiles.
const ProjectileSpeed = 50.0

// ProjectileLifetime is how long projectiles last before disappearing.
const ProjectileLifetime = 2.0

// NewProjectile creates a projectile at position (x,y) traveling in direction angle.
// The projectile inherits the shooter's velocity plus its own speed.
func NewProjectile(x, y, angle, shooterVX, shooterVY float64) *Projectile {
	return &Projectile{
		X:        x,
		Y:        y,
		VX:       shooterVX + math.Cos(angle)*ProjectileSpeed,
		VY:       shooterVY + math.Sin(angle)*ProjectileSpeed,
		Lifetime: ProjectileLifetime,
		Symbol:   '•',
	}
}

// Update moves the projectile and checks lifetime.
func (p *Projectile) Update(ctx UpdateContext) (bool, error) {
	dt := ctx.Delta.Seconds()

	// Decrease lifetime
	p.Lifetime -= dt
	if p.Lifetime <= 0 {
		return true, nil // Remove projectile
	}

	// Apply velocity
	p.X += p.VX * dt
	p.Y += p.VY * dt

	// Screen wrapping
	ctx.Screen.WrapPosition(&p.X, &p.Y)

	return false, nil
}

// Draw renders the projectile.
func (p *Projectile) Draw(w io.Writer) error {
	x := int(math.Round(p.X))
	y := int(math.Round(p.Y))
	draw.DrawChar(w, x, y, p.Symbol)
	return nil
}
