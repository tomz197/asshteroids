package object

import (
	"math"
)

// Projectile is a bullet fired by the player.
type Projectile struct {
	X, Y      float64 // Position
	VX, VY    float64 // Velocity
	Lifetime  float64 // Seconds remaining before removal
	Symbol    rune    // Character to display
	OwnerID   int     // Client ID that fired this projectile
	destroyed bool    // Marked for destruction
}

// ProjectileSpeed is the base speed of projectiles.
const ProjectileSpeed = 50.0

// ProjectileLifetime is how long projectiles last before disappearing.
const ProjectileLifetime = 2.0

// ProjectileRadius is the collision radius for projectile-projectile collisions.
const ProjectileRadius = 0.5

// NewProjectile creates a projectile at position (x,y) traveling in direction angle.
// The projectile inherits the shooter's velocity plus its own speed.
// ownerID identifies the client that fired it (for score attribution).
func NewProjectile(x, y, angle, shooterVX, shooterVY float64, ownerID int) *Projectile {
	return &Projectile{
		X:        x,
		Y:        y,
		VX:       shooterVX + math.Cos(angle)*ProjectileSpeed,
		VY:       shooterVY + math.Sin(angle)*ProjectileSpeed,
		Lifetime: ProjectileLifetime,
		Symbol:   '•',
		OwnerID:  ownerID,
	}
}

// MarkDestroyed marks the projectile for removal.
func (p *Projectile) MarkDestroyed() {
	p.destroyed = true
	p.Lifetime = 0
}

// IsDestroyed returns true if the projectile is marked for destruction.
func (p *Projectile) IsDestroyed() bool {
	return p.destroyed || p.Lifetime <= 0
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
func (p *Projectile) Draw(ctx DrawContext) error {
	// Get screen positions (handles world wrapping)
	positions := WorldToScreen(p.X, p.Y, ctx.Camera, ctx.View, ctx.World)
	for i := 0; i < positions.Count; i++ {
		pos := positions.Positions[i]
		ctx.Canvas.SetFloat(pos.X, pos.Y)
	}

	return nil
}
