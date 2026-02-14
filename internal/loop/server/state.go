package server

import (
	"time"

	"github.com/tomz197/asteroids/internal/object"
	"github.com/tomz197/asteroids/internal/physics"
)

// TopScoreEntry represents a single entry on the leaderboard.
type TopScoreEntry struct {
	Username string
	Score    int
	clientID int // Used for deterministic tie-break when scores are equal
}

// WorldState holds shared game state (objects, world bounds, timing).
// This is managed by the Server and shared across all clients via snapshots.
type WorldState struct {
	Objects       []object.Object
	toSpawn       []object.Object // Objects to add after current update cycle
	Screen        object.Screen   // Used for update context (world bounds)
	World         object.Screen   // World dimensions (total game area)
	Delta         time.Duration   // Frame delta time
	AsteroidCount int             // Weighted asteroid count maintained incrementally

	// Reusable caches for collision detection (avoids allocations)
	projectileCache []*object.Projectile
	asteroidCache   []*object.Asteroid

	// Spatial grids for broad-phase collision detection (reused each frame)
	asteroidGrid   *physics.SpatialGrid
	projectileGrid *physics.SpatialGrid
}

// WorldSnapshot is an immutable snapshot of the world state for rendering.
type WorldSnapshot struct {
	Objects     []object.Object
	UserObjects []*object.User
	Players     int
	World       object.Screen
	Delta       time.Duration
	TopScores   []TopScoreEntry // Top N scores for leaderboard display
}

// collisionGridCellSize is the cell size for the spatial hash grids.
// Must be >= the largest collision distance (two large asteroids: 5.0 + 5.0 = 10.0).
const collisionGridCellSize = 10.0

// NewWorldState creates a new initialized world state.
func NewWorldState() *WorldState {
	return &WorldState{
		Objects: []object.Object{},
	}
}

// InitGrids creates the spatial grids for broad-phase collision detection.
// Must be called after World dimensions are set.
func (w *WorldState) InitGrids() {
	worldW := float64(w.World.Width)
	worldH := float64(w.World.Height)
	w.asteroidGrid = physics.NewSpatialGrid(worldW, worldH, collisionGridCellSize)
	w.projectileGrid = physics.NewSpatialGrid(worldW, worldH, collisionGridCellSize)
}

// asteroidWeight returns the weighted count for an asteroid by size.
// Large=4 (splits into 2 medium), Medium=2 (splits into 2 small), Small=1.
// Returns 0 for non-asteroid objects.
func asteroidWeight(obj object.Object) int {
	a, ok := obj.(*object.Asteroid)
	if !ok || a.Destroyed {
		return 0
	}
	switch a.Size {
	case object.AsteroidLarge:
		return 4
	case object.AsteroidMedium:
		return 2
	case object.AsteroidSmall:
		return 1
	default:
		return 0
	}
}

// AddObject adds an object to the game world.
func (w *WorldState) AddObject(obj object.Object) {
	w.Objects = append(w.Objects, obj)
	w.AsteroidCount += asteroidWeight(obj)
}

// RemoveObject decrements the asteroid count for a removed object.
// Call this when removing an object that was tracked via AddObject.
func (w *WorldState) RemoveObject(obj object.Object) {
	w.AsteroidCount -= asteroidWeight(obj)
}

// Spawn queues an object to be added after the current update cycle.
// Implements object.Spawner interface.
func (w *WorldState) Spawn(obj object.Object) {
	w.toSpawn = append(w.toSpawn, obj)
}

// FlushSpawned adds all queued objects to the game and clears the queue.
func (w *WorldState) FlushSpawned() {
	for _, obj := range w.toSpawn {
		w.AsteroidCount += asteroidWeight(obj)
	}
	w.Objects = append(w.Objects, w.toSpawn...)
	w.toSpawn = w.toSpawn[:0]
}
