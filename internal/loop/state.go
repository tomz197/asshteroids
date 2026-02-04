package loop

import (
	"time"

	"github.com/tomz197/asteroids/internal/draw"
	"github.com/tomz197/asteroids/internal/input"
	"github.com/tomz197/asteroids/internal/object"
)

// GameState represents the current game phase.
type GameState int

const (
	GameStateStart   GameState = iota // Title screen
	GameStatePlaying                  // Active gameplay
	GameStateDead                     // Player died, show restart prompt
)

// State holds all game state.
type State struct {
	Objects        []object.Object
	toSpawn        []object.Object // Objects to add after current update cycle
	Screen         object.Screen   // Used for update context (world bounds)
	View           object.Screen   // Viewport dimensions
	World          object.Screen   // World dimensions (total game area)
	Camera         object.Camera   // Camera position (follows player)
	Input          object.Input
	Delta          time.Duration
	InputStream    *input.Stream
	Running        bool
	GameState      GameState
	Player         *object.User // Reference to player ship
	Score          int
	Lives          int
	InvincibleTime float64           // Remaining invincibility time in seconds
	termSizeFunc   draw.TermSizeFunc // Function to get terminal size
}

// NewState creates a new initialized game state.
func NewState() *State {
	return &State{
		Objects:   []object.Object{},
		Running:   true,
		GameState: GameStateStart,
		Lives:     3,
	}
}

// AddObject adds an object to the game.
func (s *State) AddObject(obj object.Object) {
	s.Objects = append(s.Objects, obj)
}

// Spawn queues an object to be added after the current update cycle.
// Implements object.Spawner interface.
func (s *State) Spawn(obj object.Object) {
	s.toSpawn = append(s.toSpawn, obj)
}

// FlushSpawned adds all queued objects to the game and clears the queue.
func (s *State) FlushSpawned() {
	s.Objects = append(s.Objects, s.toSpawn...)
	s.toSpawn = s.toSpawn[:0]
}

// UpdateContext creates an UpdateContext from the current state.
func (s *State) UpdateContext() object.UpdateContext {
	return object.UpdateContext{
		Delta:   s.Delta,
		Input:   s.Input,
		Screen:  s.Screen,
		Spawner: s,
		Objects: s.Objects,
	}
}
