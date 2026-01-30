package object

import (
	"io"
	"math"

	"github.com/tomz197/asteroids/internal/draw"
)

// User is the player-controlled spaceship (Asteroids-style).
type User struct {
	X, Y   float64 // Position (center of ship)
	VX, VY float64 // Velocity (momentum)
	Angle  float64 // Rotation in radians (0 = pointing right, increases counter-clockwise)

	ThrustPower   float64 // Acceleration when thrusting
	RotationSpeed float64 // Radians per second
	MaxSpeed      float64 // Maximum velocity magnitude
	Drag          float64 // Velocity decay per second (1.0 = no drag, 0.5 = 50% speed loss/sec)
	Size          float64 // Size of the ship triangle

	// Shooting
	FireRate     float64 // Minimum seconds between shots
	fireCooldown float64 // Time until next shot allowed
}

// NewUser creates a new spaceship at the given position.
func NewUser(x, y float64) *User {
	return &User{
		X:             x,
		Y:             y,
		Angle:         -math.Pi / 2, // Start pointing up
		ThrustPower:   40.0,         // Acceleration units per second²
		RotationSpeed: 5.0,          // ~286 degrees per second
		MaxSpeed:      25.0,         // Max speed cap
		Drag:          0.5,          // Lose 50% speed per second when not thrusting
		Size:          2.0,          // Triangle size
		FireRate:      0.15,         // 6-7 shots per second max
	}
}

// Update handles rotation, thrust, momentum physics, and shooting.
func (u *User) Update(ctx UpdateContext) (bool, error) {
	dt := ctx.Delta.Seconds()

	// Rotation (left/right)
	if ctx.Input.Left {
		u.Angle -= u.RotationSpeed * dt
	}
	if ctx.Input.Right {
		u.Angle += u.RotationSpeed * dt
	}

	// Normalize angle to [-π, π]
	for u.Angle > math.Pi {
		u.Angle -= 2 * math.Pi
	}
	for u.Angle < -math.Pi {
		u.Angle += 2 * math.Pi
	}

	// Thrust (accelerate in facing direction)
	if ctx.Input.Up {
		u.VX += math.Cos(u.Angle) * u.ThrustPower * dt
		u.VY += math.Sin(u.Angle) * u.ThrustPower * dt

		// Spawn thrust particles from the back of the ship
		const aspectRatio = 2.0
		backX := u.X - math.Cos(u.Angle)*u.Size*aspectRatio*0.5
		backY := u.Y - math.Sin(u.Angle)*u.Size*0.5
		SpawnThrust(backX, backY, u.Angle, ctx.Spawner)
	}

	// Apply drag (velocity decay when not thrusting)
	if !ctx.Input.Up {
		dragFactor := math.Pow(u.Drag, dt)
		u.VX *= dragFactor
		u.VY *= dragFactor
	}

	// Clamp to max speed
	speed := math.Sqrt(u.VX*u.VX + u.VY*u.VY)
	if speed > u.MaxSpeed {
		scale := u.MaxSpeed / speed
		u.VX *= scale
		u.VY *= scale
	}

	// Apply velocity to position
	u.X += u.VX * dt
	u.Y += u.VY * dt

	// Screen wrapping
	ctx.Screen.WrapPosition(&u.X, &u.Y)

	// Shooting
	u.fireCooldown -= dt
	if ctx.Input.Space && u.fireCooldown <= 0 && ctx.Spawner != nil {
		u.fireCooldown = u.FireRate

		// Spawn projectile from the nose of the ship
		const aspectRatio = 2.0
		noseX := u.X + math.Cos(u.Angle)*u.Size*aspectRatio
		noseY := u.Y + math.Sin(u.Angle)*u.Size

		projectile := NewProjectile(noseX, noseY, u.Angle, u.VX, u.VY)
		ctx.Spawner.Spawn(projectile)
	}

	return false, nil
}

// Draw renders the spaceship as a triangle pointing in the direction of travel.
func (u *User) Draw(w io.Writer) error {
	// Terminal characters are roughly 2:1 (height:width), so we adjust X coordinates
	const aspectRatio = 2.0

	// Triangle vertices relative to center:
	// - Nose (front): in the direction of Angle
	// - Left wing: 140° from nose
	// - Right wing: -140° from nose
	noseAngle := u.Angle
	leftAngle := u.Angle + 2.5 // ~143 degrees
	rightAngle := u.Angle - 2.5

	size := u.Size

	// Calculate vertex positions (adjust X for terminal aspect ratio)
	triangle := []draw.Point{
		{X: u.X + math.Cos(noseAngle)*size*aspectRatio, Y: u.Y + math.Sin(noseAngle)*size},
		{X: u.X + math.Cos(leftAngle)*size*0.7*aspectRatio, Y: u.Y + math.Sin(leftAngle)*size*0.7},
		{X: u.X + math.Cos(rightAngle)*size*0.7*aspectRatio, Y: u.Y + math.Sin(rightAngle)*size*0.7},
	}

	// Draw the triangle
	draw.DrawPolygon(w, triangle, draw.BlockFull, true)

	return nil
}
