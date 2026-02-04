package loop

import (
	"math"

	"github.com/tomz197/asteroids/internal/object"
)

// updatePlayingState handles the playing game state.
func updatePlayingState(state *State) error {
	// Decrement invincibility timer
	if state.InvincibleTime > 0 {
		state.InvincibleTime -= state.Delta.Seconds()
		if state.InvincibleTime < 0 {
			state.InvincibleTime = 0
		}
	}

	if err := updateObjects(state); err != nil {
		return err
	}
	checkCollisions(state)
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
		if p.Lifetime <= 0 {
			continue
		}
		for _, a := range asteroids {
			if a.Destroyed || a.IsProtected() {
				continue
			}
			if collides(p.X, p.Y, a.X, a.Y, a.GetRadius()) {
				// Destroy both projectile and asteroid
				p.Lifetime = 0 // Mark projectile for removal
				a.Hit()        // Mark asteroid for splitting/removal

				// Award points based on asteroid size
				switch a.Size {
				case object.AsteroidLarge:
					state.Score += 20
				case object.AsteroidMedium:
					state.Score += 50
				case object.AsteroidSmall:
					state.Score += 100
				}
			}
		}
	}

	// Check projectile-projectile collisions
	for i := 0; i < len(projectiles); i++ {
		p1 := projectiles[i]
		if p1.Lifetime <= 0 {
			continue
		}
		for j := i + 1; j < len(projectiles); j++ {
			p2 := projectiles[j]
			if p2.Lifetime <= 0 {
				continue
			}
			// Check if projectiles are close enough to collide
			dist := distance(p1.X, p1.Y, p2.X, p2.Y)
			if dist < object.ProjectileRadius*2 {
				// Destroy both projectiles
				p1.Lifetime = 0
				p2.Lifetime = 0
			}
		}
	}

	// Check projectile-player collisions (skip if invincible)
	if state.Player != nil && state.GameState == GameStatePlaying && state.InvincibleTime <= 0 {
		px, py := state.Player.GetPosition()
		pr := state.Player.GetRadius()

		for _, p := range projectiles {
			if p.Lifetime <= 0 {
				continue
			}
			if collides(p.X, p.Y, px, py, pr) {
				p.Lifetime = 0 // Remove projectile
				killPlayer(state)
				return
			}
		}
	}

	// Check asteroid-asteroid collisions (bounce)
	for i := 0; i < len(asteroids); i++ {
		a1 := asteroids[i]
		if a1.Destroyed {
			continue
		}
		for j := i + 1; j < len(asteroids); j++ {
			a2 := asteroids[j]
			if a2.Destroyed {
				continue
			}
			// Circle-circle collision
			dist := distance(a1.X, a1.Y, a2.X, a2.Y)
			minDist := a1.GetRadius() + a2.GetRadius()
			if dist < minDist && dist > 0 {
				// Elastic collision response
				bounceAsteroids(a1, a2, dist)
			}
		}
	}

	// Check player-asteroid collisions (skip if invincible)
	if state.Player != nil && state.GameState == GameStatePlaying && state.InvincibleTime <= 0 {
		px, py := state.Player.GetPosition()
		pr := state.Player.GetRadius()

		for _, a := range asteroids {
			if a.Destroyed || a.IsProtected() {
				continue
			}
			// Circle-circle collision
			dist := distance(px, py, a.X, a.Y)
			if dist < pr+a.GetRadius() {
				killPlayer(state)
				return
			}
		}
	}
}

// killPlayer handles player death.
func killPlayer(state *State) {
	if state.Player == nil {
		return
	}

	// Spawn death explosion
	x, y := state.Player.GetPosition()
	object.SpawnExplosion(x, y, 20, 25.0, 1.0, state)

	// Remove player from objects
	kept := state.Objects[:0]
	for _, obj := range state.Objects {
		if obj != state.Player {
			kept = append(kept, obj)
		}
	}
	state.Objects = kept

	state.Lives--
	state.GameState = GameStateDead
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

// bounceAsteroids handles collision between two asteroids.
func bounceAsteroids(a1, a2 *object.Asteroid, dist float64) {
	// Calculate collision normal (from a1 to a2)
	nx := (a2.X - a1.X) / dist
	ny := (a2.Y - a1.Y) / dist

	// Calculate relative velocity
	dvx := a1.VX - a2.VX
	dvy := a1.VY - a2.VY

	// Relative velocity along the collision normal
	dvn := dvx*nx + dvy*ny

	// Don't resolve if velocities are separating
	if dvn < 0 {
		return
	}

	// Use radius squared as mass (area-based mass)
	m1 := a1.Radius * a1.Radius
	m2 := a2.Radius * a2.Radius
	totalMass := m1 + m2

	// Calculate impulse scalar (elastic collision)
	impulse := 2 * dvn / totalMass

	// Apply impulse to velocities
	a1.VX -= impulse * m2 * nx
	a1.VY -= impulse * m2 * ny
	a2.VX += impulse * m1 * nx
	a2.VY += impulse * m1 * ny

	// Separate asteroids to prevent overlap
	overlap := (a1.Radius + a2.Radius) - dist
	if overlap > 0 {
		// Push each asteroid away proportionally to their mass ratio
		sep1 := overlap * m2 / totalMass
		sep2 := overlap * m1 / totalMass
		a1.X -= nx * sep1
		a1.Y -= ny * sep1
		a2.X += nx * sep2
		a2.Y += ny * sep2
	}
}
