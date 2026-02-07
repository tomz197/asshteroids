// Package config centralizes all tunable game parameters.
package config

import "time"

// View resolution - the visible viewport in logical units.
// Actual rendering scales to fit terminal size.
const (
	ViewWidth  = 120 // Logical viewport width
	ViewHeight = 80  // Logical viewport height (in sub-pixels, so 40 terminal rows)
)

// World dimensions - the total game area (larger than viewport).
// Ship stays centered while the camera follows it.
const (
	WorldWidth  = 400 // Total world width
	WorldHeight = 300 // Total world height
)

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
	MaxUsernameLength    = 16   // Maximum display length for player usernames
)

// Spawning
const (
	InitialAsteroidTarget = 250
)

// Shutdown
const (
	ShutdownDisplaySeconds = 10.0 // Seconds to show shutdown message before auto-disconnect
)

// Inactivity
const (
	InactivityWarnUser       = 90  // Seconds
	InactivityDisconnectUser = 120 // Seconds
)

// Client rendering
const (
	ClientTargetFPS       = 60
	ClientTargetFrameTime = time.Second / ClientTargetFPS
)

// Server tick rate
const (
	ServerTickRate = 60
	ServerTickTime = time.Second / ServerTickRate
)
