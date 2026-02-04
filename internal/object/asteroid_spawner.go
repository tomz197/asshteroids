package object

// AsteroidSpawner keeps the asteroid population at a target level.
type AsteroidSpawner struct {
	target int
}

// NewAsteroidSpawner creates a spawner with a target asteroid count.
func NewAsteroidSpawner(target int) *AsteroidSpawner {
	if target < 0 {
		target = 0
	}
	return &AsteroidSpawner{
		target: target,
	}
}

// Update spawns asteroids at the edges when the count drops.
func (s *AsteroidSpawner) Update(ctx UpdateContext) (bool, error) {
	if s.target == 0 {
		return false, nil
	}

	count := s.countActiveAsteroids(ctx)
	if count >= s.target {
		return false, nil
	}

	for s.target-count > 5 {
		var size AsteroidSize
		if s.target-count > 4 {
			size = AsteroidLarge
			count += 4
		} else if s.target-count > 2 {
			size = AsteroidMedium
			count += 2
		} else {
			size = AsteroidSmall
			count += 1
		}

		asteroid := NewAsteroidAtEdge(ctx.Screen, size)
		ctx.Spawner.Spawn(asteroid)
	}
	return false, nil
}

// Draw is a no-op; spawner is not visible.
func (s *AsteroidSpawner) Draw(_ DrawContext) error {
	return nil
}

func (s *AsteroidSpawner) countActiveAsteroids(ctx UpdateContext) int {
	total := 0
	for _, obj := range ctx.Objects {
		if asteroid, ok := obj.(*Asteroid); ok && !asteroid.Destroyed {
			switch asteroid.Size {
			case AsteroidLarge:
				total += 4
			case AsteroidMedium:
				total += 2
			case AsteroidSmall:
				total += 1
			}
		}
	}
	return total
}
