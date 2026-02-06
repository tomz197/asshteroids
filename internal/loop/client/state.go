package client

import (
	"time"

	"github.com/tomz197/asteroids/internal/draw"
	"github.com/tomz197/asteroids/internal/loop/config"
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

// NewClientState creates a new initialized client state.
func NewClientState() *ClientState {
	return &ClientState{
		GameState: GameStateStart,
		Lives:     config.InitialLives,
		Running:   true,
	}
}
