package loop

// Game configuration constants.
// All tunable game parameters are centralized here for easy adjustment.

// Scoring
const (
	ScoreLargeAsteroid  = 20
	ScoreMediumAsteroid = 50
	ScoreSmallAsteroid  = 100
)

// Player
const (
	InitialLives         = 3
	InvincibilitySeconds = 3.0
	PlayerBlinkFrequency = 10.0 // Hz
)

// Spawning
const (
	InitialAsteroidTarget = 250
)

// Shutdown
const (
	shutdownDisplaySeconds = 10.0 // Seconds to show shutdown message before auto-disconnect
)

// Inactivity
const (
	InactivityWarnUser       = 90  // Seconds
	InactivityDisconnectUser = 120 // Seconds
)
