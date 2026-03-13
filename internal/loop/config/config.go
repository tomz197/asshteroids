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
	WorldHeight = 400 // Total world height
)

// Scoring
const (
	ScoreLargeAsteroid  = 20
	ScoreMediumAsteroid = 50
	ScoreSmallAsteroid  = 100
	ScorePlayerKill     = 1000
	TopScoresCount      = 5 // Number of top scores to track and display
)

// Player
const (
	InitialLives         = 3
	InvincibilityTime    = 3 * time.Second
	RespawnTimeout       = 3 * time.Second
	PlayerBlinkFrequency = 10.0 // Hz
	MaxUsernameLength    = 16   // Maximum display length for player usernames
)

// Spawning
const (
	InitialAsteroidTarget = 250
)

// Shutdown
const (
	ShutdownDisplayTime = 10 * time.Second
)

// Inactivity
const (
	InactivityWarnUser       = 90  // Seconds
	InactivityDisconnectUser = 120 // Seconds
)

// Chat
const (
	MaxChatMessageLength = 128 // Maximum characters per chat message
	MaxChatHistory       = 50  // Messages kept in server buffer
)

// Maximum terminal render resolution.
// If the user's terminal is larger, the render area is centered with a border.
const (
	MaxTermWidth  = 240 // Maximum terminal columns for rendering
	MaxTermHeight = 80  // Maximum terminal rows for rendering
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
