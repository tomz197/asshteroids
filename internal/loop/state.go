package loop

import (
	"time"

	"github.com/tomz197/asteroids/internal/draw"
	"github.com/tomz197/asteroids/internal/input"
	"github.com/tomz197/asteroids/internal/object"
)

// GameState represents the current game phase for a client.
type GameState int

const (
	GameStateStart   GameState = iota // Title screen
	GameStatePlaying                  // Active gameplay
	GameStateDead                     // Player died, show restart prompt
)

// WorldState holds shared game state (objects, world bounds, timing).
// This is shared across all clients in a multiplayer scenario.
type WorldState struct {
	Objects []object.Object
	toSpawn []object.Object // Objects to add after current update cycle
	Screen  object.Screen   // Used for update context (world bounds)
	World   object.Screen   // World dimensions (total game area)
	Delta   time.Duration   // Frame delta time
	Running bool            // Game loop running
}

// ClientState holds per-player state (input, score, camera, etc.).
// Each player/client has their own instance.
type ClientState struct {
	Input          object.Input
	InputStream    *input.Stream
	View           object.Screen     // Viewport dimensions (can vary per client)
	Camera         object.Camera     // Camera position (follows this client's player)
	GameState      GameState         // This client's game phase
	Player         *object.User      // Reference to this client's ship
	Score          int               // This client's score
	Lives          int               // This client's remaining lives
	InvincibleTime float64           // Remaining invincibility time in seconds
	termSizeFunc   draw.TermSizeFunc // Function to get terminal size
}

// State holds all game state, combining world and client state.
// For single-player, this contains one ClientState.
// For future multiplayer, WorldState would be shared while each client has its own ClientState.
type State struct {
	WorldState
	ClientState
}

// NewWorldState creates a new initialized world state.
func NewWorldState() *WorldState {
	return &WorldState{
		Objects: []object.Object{},
		Running: true,
	}
}

// NewClientState creates a new initialized client state.
func NewClientState() *ClientState {
	return &ClientState{
		GameState: GameStateStart,
		Lives:     3,
	}
}

// NewState creates a new initialized game state (world + single client).
func NewState() *State {
	return &State{
		WorldState:  *NewWorldState(),
		ClientState: *NewClientState(),
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

// UpdateContext creates an UpdateContext from the current state.
// Uses the client's input but the world's objects and bounds.
func (s *State) UpdateContext() object.UpdateContext {
	return object.UpdateContext{
		Delta:   s.Delta,
		Input:   s.Input,
		Screen:  s.Screen,
		Spawner: &s.WorldState,
		Objects: s.Objects,
	}
}
