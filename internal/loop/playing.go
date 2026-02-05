package loop

import (
	"github.com/tomz197/asteroids/internal/object"
	"github.com/tomz197/asteroids/internal/physics"
)

// updatePlayingState handles the playing game state (legacy single-player).
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

// collectCollidables extracts projectiles and asteroids from the object list.
func collectCollidables(objects []object.Object) ([]*object.Projectile, []*object.Asteroid) {
	var projectiles []*object.Projectile
	var asteroids []*object.Asteroid

	for _, obj := range objects {
		switch o := obj.(type) {
		case *object.Projectile:
			projectiles = append(projectiles, o)
		case *object.Asteroid:
			asteroids = append(asteroids, o)
		}
	}
	return projectiles, asteroids
}

// checkCollisions detects and handles all collisions between objects (legacy single-player).
func checkCollisions(state *State) {
	projectiles, asteroids := collectCollidables(state.Objects)

	checkProjectileAsteroidCollisions(state, projectiles, asteroids)
	checkProjectileProjectileCollisions(projectiles)
	checkAsteroidAsteroidCollisions(asteroids)

	// Player collisions only if vulnerable
	if state.Player != nil && state.GameState == GameStatePlaying && state.InvincibleTime <= 0 {
		if checkPlayerCollisions(state, projectiles, asteroids) {
			return // Player died, skip remaining checks
		}
	}
}

// checkProjectileAsteroidCollisions handles projectile hits on asteroids.
func checkProjectileAsteroidCollisions(state *State, projectiles []*object.Projectile, asteroids []*object.Asteroid) {
	for _, p := range projectiles {
		if p.IsDestroyed() {
			continue
		}
		for _, a := range asteroids {
			if a.IsDestroyed() || a.IsProtected() {
				continue
			}
			if physics.PointInCircle(p.X, p.Y, a.X, a.Y, a.GetRadius()) {
				p.MarkDestroyed()
				a.MarkDestroyed()
				state.Score += asteroidScore(a.Size)
			}
		}
	}
}

// asteroidScore returns the score for destroying an asteroid of the given size.
func asteroidScore(size object.AsteroidSize) int {
	switch size {
	case object.AsteroidLarge:
		return ScoreLargeAsteroid
	case object.AsteroidMedium:
		return ScoreMediumAsteroid
	case object.AsteroidSmall:
		return ScoreSmallAsteroid
	default:
		return 0
	}
}

// checkProjectileProjectileCollisions handles projectile-projectile collisions.
func checkProjectileProjectileCollisions(projectiles []*object.Projectile) {
	for i := 0; i < len(projectiles); i++ {
		p1 := projectiles[i]
		if p1.IsDestroyed() {
			continue
		}
		for j := i + 1; j < len(projectiles); j++ {
			p2 := projectiles[j]
			if p2.IsDestroyed() {
				continue
			}
			if physics.CirclesOverlap(p1.X, p1.Y, object.ProjectileRadius, p2.X, p2.Y, object.ProjectileRadius) {
				p1.MarkDestroyed()
				p2.MarkDestroyed()
			}
		}
	}
}

// checkAsteroidAsteroidCollisions handles bouncing between asteroids.
func checkAsteroidAsteroidCollisions(asteroids []*object.Asteroid) {
	for i := 0; i < len(asteroids); i++ {
		a1 := asteroids[i]
		if a1.IsDestroyed() {
			continue
		}
		for j := i + 1; j < len(asteroids); j++ {
			a2 := asteroids[j]
			if a2.IsDestroyed() {
				continue
			}
			dist := physics.Distance(a1.X, a1.Y, a2.X, a2.Y)
			minDist := a1.GetRadius() + a2.GetRadius()
			if dist < minDist && dist > 0 {
				bounceAsteroids(a1, a2, dist)
			}
		}
	}
}

// checkPlayerCollisions checks if the player collides with projectiles or asteroids.
// Returns true if the player was killed. (Legacy single-player)
func checkPlayerCollisions(state *State, projectiles []*object.Projectile, asteroids []*object.Asteroid) bool {
	px, py := state.Player.GetPosition()
	pr := state.Player.GetRadius()

	// Check projectile hits
	for _, p := range projectiles {
		if p.IsDestroyed() {
			continue
		}
		if physics.PointInCircle(p.X, p.Y, px, py, pr) {
			p.MarkDestroyed()
			killPlayer(state)
			return true
		}
	}

	// Check asteroid collisions
	for _, a := range asteroids {
		if a.IsDestroyed() || a.IsProtected() {
			continue
		}
		if physics.CirclesOverlap(px, py, pr, a.X, a.Y, a.GetRadius()) {
			killPlayer(state)
			return true
		}
	}

	return false
}

// killPlayer handles player death (legacy single-player).
func killPlayer(state *State) {
	if state.Player == nil {
		return
	}

	// Spawn death explosion
	x, y := state.Player.GetPosition()
	object.SpawnExplosion(x, y, 20, 25.0, 1.0, &state.WorldState)

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

// bounceAsteroids handles elastic collision between two asteroids.
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
