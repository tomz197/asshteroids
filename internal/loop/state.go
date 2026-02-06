package loop

import (
	"time"

	"github.com/tomz197/asteroids/internal/draw"
	"github.com/tomz197/asteroids/internal/object"
)

// GameState represents the current game phase for a client.
type GameState int

const (
	GameStateStart    GameState = iota // Title screen
	GameStatePlaying                   // Active gameplay
	GameStateDead                      // Player died, show restart prompt
	GameStateShutdown                  // Server is shutting down
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

// ClientState holds per-player state (input, score, camera, etc.).
// Each client has their own instance, managed by the Client.
type ClientState struct {
	Input          object.Input
	View           object.Screen     // Viewport dimensions (can vary per client)
	Camera         object.Camera     // Camera position (follows this client's player)
	GameState      GameState         // This client's game phase
	Player         *object.User      // Reference to this client's ship (from server)
	Score          int               // This client's score
	Lives          int               // This client's remaining lives
	InvincibleTime float64           // Remaining invincibility time in seconds
	termSizeFunc   draw.TermSizeFunc // Function to get terminal size
	Running        bool              // Client loop running
	delta          time.Duration     // Frame delta time (client-side)
	shutdownTimer  float64           // Countdown before auto-disconnect on shutdown
	isInactive     bool              // Whether the client is in inactive warning state
}

// NewWorldState creates a new initialized world state.
func NewWorldState() *WorldState {
	return &WorldState{
		Objects: []object.Object{},
	}
}

// NewClientState creates a new initialized client state.
func NewClientState() *ClientState {
	return &ClientState{
		GameState: GameStateStart,
		Lives:     InitialLives,
		Running:   true,
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
