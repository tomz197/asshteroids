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
