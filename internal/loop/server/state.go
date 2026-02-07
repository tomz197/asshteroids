package server

import (
	"time"

	"github.com/tomz197/asteroids/internal/object"
)

// WorldState holds shared game state (objects, world bounds, timing).
// This is managed by the Server and shared across all clients via snapshots.
type WorldState struct {
	Objects []object.Object
	toSpawn []object.Object // Objects to add after current update cycle
	Screen  object.Screen   // Used for update context (world bounds)
	World   object.Screen   // World dimensions (total game area)
	Delta   time.Duration   // Frame delta time

	// Reusable caches for collision detection (avoids allocations)
	projectileCache []*object.Projectile
	asteroidCache   []*object.Asteroid
}

// WorldSnapshot is an immutable snapshot of the world state for rendering.
type WorldSnapshot struct {
	Objects     []object.Object
	UserObjects []*object.User
	Players     int
	World       object.Screen
	Delta       time.Duration
}

// NewWorldState creates a new initialized world state.
func NewWorldState() *WorldState {
	return &WorldState{
		Objects: []object.Object{},
	}
}

// AddObject adds an object to the game world.
func (w *WorldState) AddObject(obj object.Object) {
	w.Objects = append(w.Objects, obj)
}

// Spawn queues an object to be added after the current update cycle.
// Implements object.Spawner interface.
func (w *WorldState) Spawn(obj object.Object) {
	w.toSpawn = append(w.toSpawn, obj)
}

// FlushSpawned adds all queued objects to the game and clears the queue.
func (w *WorldState) FlushSpawned() {
	w.Objects = append(w.Objects, w.toSpawn...)
	w.toSpawn = w.toSpawn[:0]
}
