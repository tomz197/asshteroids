package loop

import (
	"fmt"
	"io"

	"github.com/tomz197/asteroids/internal/draw"
	"github.com/tomz197/asteroids/internal/input"
	"github.com/tomz197/asteroids/internal/object"
)

// updateStartState handles the start screen state.
func updateStartState(state *State) {
	if state.Input.Space || state.Input.Enter {
		startGame(state)
	}
}

// updateDeadState handles the death screen state.
func updateDeadState(state *State) {
	// Update particles for death explosion effect
	ctx := state.UpdateContext()
	kept := state.Objects[:0]
	for _, obj := range state.Objects {
		// Only update particles
		if _, isParticle := obj.(*object.Particle); isParticle {
			remove, _ := obj.Update(ctx)
			if !remove {
				kept = append(kept, obj)
			}
		} else if _, isAsteroid := obj.(*object.Asteroid); isAsteroid {
			kept = append(kept, obj) // Keep asteroids visible but frozen
		}
	}
	state.Objects = kept
	state.FlushSpawned()

	if state.Input.Space || state.Input.Enter {
		startGame(state)
	}
}

// startGame initializes a new game or respawns player.
func startGame(state *State) {
	input.ResetKeyInput(state.InputStream)
	if state.GameState == GameStateStart || state.Lives <= 0 {
		// Full restart
		state.Objects = state.Objects[:0]
		state.toSpawn = state.toSpawn[:0]
		state.Score = 0
		state.Lives = 3

	} else {
		// Respawn - keep asteroids, remove particles
		kept := state.Objects[:0]
		for _, obj := range state.Objects {
			if _, isParticle := obj.(*object.Particle); !isParticle {
				kept = append(kept, obj)
			}
		}
		state.Objects = kept
	}

	state.AddObject(object.NewAsteroidSpawner(30))

	// Create player at center
	player := object.NewUser(float64(targetWidth/2), float64(targetHeight/2))
	state.Player = player
	state.AddObject(player)

	// Grant invincibility for 3 seconds
	state.InvincibleTime = 3.0

	state.GameState = GameStatePlaying
}

// drawUI draws the game UI overlay.
func drawUI(state *State, w io.Writer, canvas *draw.Canvas) {
	termWidth := canvas.TerminalWidth()
	termHeight := canvas.TerminalHeight()
	centerX := termWidth / 2
	centerY := termHeight / 2

	switch state.GameState {
	case GameStateStart:
		drawStartScreen(w, centerX, centerY)
	case GameStatePlaying:
		drawPlayingHUD(state, w, termWidth)
	case GameStateDead:
		drawDeadScreen(state, w, centerX, centerY)
	}
}

// drawStartScreen draws the title screen.
func drawStartScreen(w io.Writer, centerX, centerY int) {
	title := "A S T E R O I D S"
	draw.MoveCursor(w, centerX-len(title)/2, centerY-2)
	fmt.Fprint(w, title)

	subtitle := "Press SPACE to Start"
	draw.MoveCursor(w, centerX-len(subtitle)/2, centerY+1)
	fmt.Fprint(w, subtitle)

	controls := "Controls: A/D or Arrows to rotate, W or Up to thrust, SPACE to shoot, Q to quit"
	draw.MoveCursor(w, centerX-len(controls)/2, centerY+4)
	fmt.Fprint(w, controls)
}

// drawPlayingHUD draws the in-game HUD (score, lives).
func drawPlayingHUD(state *State, w io.Writer, termWidth int) {
	// Score display
	scoreText := fmt.Sprintf("Score: %d", state.Score)
	draw.MoveCursor(w, 2, 1)
	fmt.Fprint(w, scoreText)

	// Lives display
	livesText := fmt.Sprintf("Lives: %d", state.Lives)
	draw.MoveCursor(w, termWidth-len(livesText)-1, 1)
	fmt.Fprint(w, livesText)
}

// drawDeadScreen draws the death/game over screen.
func drawDeadScreen(state *State, w io.Writer, centerX, centerY int) {
	var title string
	if state.Lives > 0 {
		title = "YOU DIED"
	} else {
		title = "GAME OVER"
	}
	draw.MoveCursor(w, centerX-len(title)/2, centerY-2)
	fmt.Fprint(w, title)

	scoreText := fmt.Sprintf("Score: %d", state.Score)
	draw.MoveCursor(w, centerX-len(scoreText)/2, centerY)
	fmt.Fprint(w, scoreText)

	var prompt string
	if state.Lives > 0 {
		prompt = fmt.Sprintf("Lives remaining: %d - Press SPACE to continue", state.Lives)
	} else {
		prompt = "Press SPACE to Restart"
	}
	draw.MoveCursor(w, centerX-len(prompt)/2, centerY+2)
	fmt.Fprint(w, prompt)
}
