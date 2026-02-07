package server

import (
	"github.com/tomz197/asteroids/internal/loop/config"
	"github.com/tomz197/asteroids/internal/object"
	"github.com/tomz197/asteroids/internal/physics"
)

// collectCollidables extracts projectiles and asteroids from the object list.
// Uses pre-allocated slices to avoid allocations.
func collectCollidables(objects []object.Object, projectiles *[]*object.Projectile, asteroids *[]*object.Asteroid) {
	*projectiles = (*projectiles)[:0]
	*asteroids = (*asteroids)[:0]

	for _, obj := range objects {
		switch o := obj.(type) {
		case *object.Projectile:
			*projectiles = append(*projectiles, o)
		case *object.Asteroid:
			*asteroids = append(*asteroids, o)
		}
	}
}

// populateGrids clears and re-inserts all collidables into the spatial grids.
func populateGrids(
	asteroids []*object.Asteroid,
	projectiles []*object.Projectile,
	asteroidGrid *physics.SpatialGrid,
	projectileGrid *physics.SpatialGrid,
) {
	asteroidGrid.Clear()
	for i, a := range asteroids {
		asteroidGrid.Insert(a.X, a.Y, i)
	}

	projectileGrid.Clear()
	for i, p := range projectiles {
		projectileGrid.Insert(p.X, p.Y, i)
	}
}

// asteroidScore returns the score for destroying an asteroid of the given size.
func asteroidScore(size object.AsteroidSize) int {
	switch size {
	case object.AsteroidLarge:
		return config.ScoreLargeAsteroid
	case object.AsteroidMedium:
		return config.ScoreMediumAsteroid
	case object.AsteroidSmall:
		return config.ScoreSmallAsteroid
	default:
		return 0
	}
}

// checkProjectileProjectileCollisions handles projectile-projectile collisions
// using the spatial grid to limit checks to nearby projectiles.
func checkProjectileProjectileCollisions(projectiles []*object.Projectile, grid *physics.SpatialGrid) {
	for i, p1 := range projectiles {
		if p1.IsDestroyed() {
			continue
		}
		grid.QueryAround(p1.X, p1.Y, func(j int) bool {
			if j <= i {
				return false // Skip self and already-checked pairs
			}
			p2 := projectiles[j]
			if p2.IsDestroyed() {
				return false
			}
			if physics.CirclesOverlap(p1.X, p1.Y, object.ProjectileRadius, p2.X, p2.Y, object.ProjectileRadius) {
				p1.MarkDestroyed()
				p2.MarkDestroyed()
				return true // p1 is destroyed, stop checking
			}
			return false
		})
	}
}

// checkAsteroidAsteroidCollisions handles bouncing between asteroids
// using the spatial grid to limit checks to nearby asteroids.
func checkAsteroidAsteroidCollisions(asteroids []*object.Asteroid, grid *physics.SpatialGrid) {
	for i, a1 := range asteroids {
		if a1.IsDestroyed() {
			continue
		}
		grid.QueryAround(a1.X, a1.Y, func(j int) bool {
			if j <= i {
				return false // Skip self and already-checked pairs
			}
			a2 := asteroids[j]
			if a2.IsDestroyed() {
				return false
			}
			dist := physics.Distance(a1.X, a1.Y, a2.X, a2.Y)
			minDist := a1.GetRadius() + a2.GetRadius()
			if dist < minDist && dist > 0 {
				bounceAsteroids(a1, a2, dist)
			}
			return false
		})
	}
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
