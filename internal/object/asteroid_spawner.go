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

// SpawnProtectionTime is how long new asteroids are invulnerable.
const SpawnProtectionTime = 3.0

// Update spawns asteroids at random positions when the count drops.
func (s *AsteroidSpawner) Update(ctx UpdateContext) (bool, error) {
	if s.target == 0 {
		return false, nil
	}

	count := s.countActiveAsteroids(ctx)
	if count >= s.target {
		return false, nil
	}

	// Spawn large asteroids in batches when significantly below target
	// Each large asteroid counts as 4 (can split into 2 medium -> 4 small)
	const largeAsteroidValue = 4
	const batchThreshold = 12

	for s.target-count >= batchThreshold {
		asteroid := NewAsteroidRandom(ctx.Screen, AsteroidLarge, SpawnProtectionTime)
		ctx.Spawner.Spawn(asteroid)
		count += largeAsteroidValue
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
