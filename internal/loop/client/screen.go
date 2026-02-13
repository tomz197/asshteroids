package client

import (
	"fmt"
	"time"

	"github.com/tomz197/asteroids/internal/draw"
	"github.com/tomz197/asteroids/internal/loop/config"
	"github.com/tomz197/asteroids/internal/loop/server"
	"github.com/tomz197/asteroids/internal/object"
)

// moveCursor moves the cursor to a position relative to the canvas area.
// x and y are 1-based coordinates within the canvas; the canvas offset is applied automatically.
func (c *Client) moveCursor(x, y int) {
	draw.MoveCursor(c.writer, x+c.canvas.OffsetCol(), y+c.canvas.OffsetRow())
}

// drawFrame draws the current frame.
func (c *Client) drawFrame() error {
	// On game state or inactivity transitions, do a full terminal clear
	// so UI elements from the previous state don't persist on screen.
	stateChanged := c.state.GameState != c.state.prevGameState
	inactiveChanged := c.state.isInactive != c.state.wasInactive
	if stateChanged || inactiveChanged {
		draw.ClearScreen(c.writer)
		c.canvas.ForceRedraw()
		c.state.prevGameState = c.state.GameState
		c.state.wasInactive = c.state.isInactive
	}

	c.canvas.Clear()

	// Get world snapshot
	snapshot := c.server.GetSnapshot()

	// Create draw context
	ctx := object.DrawContext{
		Canvas: c.canvas,
		Writer: c.writer,
		Camera: c.state.Camera,
		View:   c.state.View,
		World:  snapshot.World,
	}

	// Draw all objects from snapshot
	for _, obj := range snapshot.Objects {
		// Skip drawing player when blinking (invincible)
		if obj == c.state.Player && !object.ShouldRenderBlink(c.state.InvincibleTime, config.PlayerBlinkFrequency) {
			continue
		}
		if err := obj.Draw(ctx); err != nil {
			return err
		}
	}

	// Render canvas to terminal
	c.canvas.Render(c.writer)

	// Draw border when terminal exceeds max render resolution
	c.canvas.RenderBorder(c.writer)

	// Draw usernames above other players' ships
	c.drawPlayerNames(snapshot.UserObjects, snapshot.World)

	// Draw UI overlay
	c.drawUI(snapshot)

	return nil
}

// drawUI draws the game UI overlay.
func (c *Client) drawUI(snapshot *server.WorldSnapshot) {
	termWidth := c.canvas.TerminalWidth()
	termHeight := c.canvas.TerminalHeight()
	centerX := termWidth / 2
	centerY := termHeight / 2

	if c.state.GameState == GameStateShutdown {
		c.drawShutdownScreen(centerX, centerY)
		return
	}

	if c.state.isInactive {
		c.drawInactivityScreen(centerX, centerY)
		return
	}

	switch c.state.GameState {
	case GameStatePlaying:
		c.drawPlayingHUD(termWidth, termHeight, snapshot)
	case GameStateStart:
		c.drawStartScreen(centerX, centerY)
	case GameStateDead:
		c.drawDeadScreen(centerX, centerY)
	}
}

// drawInactivityScreen draws the inactivity warning screen.
func (c *Client) drawInactivityScreen(centerX, centerY int) {
	title := "INACTIVITY WARNING"
	c.moveCursor(centerX-len(title)/2, centerY-2)
	fmt.Fprint(c.writer, title)

	msg := fmt.Sprintf(
		"You have been inactive for too long. You will be disconnected in %d seconds.",
		int(config.InactivityDisconnectUser-time.Since(c.lastInput).Seconds()),
	)
	c.moveCursor(centerX-len(msg)/2, centerY)
	fmt.Fprint(c.writer, msg)

	hint := "Press any key to continue"
	c.moveCursor(centerX-len(hint)/2, centerY+2)
	fmt.Fprint(c.writer, hint)
}

// drawStartScreen draws the title screen.
func (c *Client) drawStartScreen(centerX, centerY int) {
	// ASCII art title (figlet "small" font)
	titleArt := []string{
		`    _   ___ ___ _  _ _____ ___ ___  ___ ___ ___  ___  `,
		`   /_\ / __/ __| || |_   _| __| _ \/ _ \_ _|   \/ __| `,
		`  / _ \\__ \__ \ __ | | | | _||   / (_) | || |) \__ \ `,
		` /_/ \_\___/___/_||_| |_| |___|_|_\\___/___|___/|___/ `,
		`                                                      `,
	}

	// Find max width for centering
	titleWidth := 0
	for _, line := range titleArt {
		if len(line) > titleWidth {
			titleWidth = len(line)
		}
	}

	// Draw title art centered
	titleStartY := centerY - 7
	for i, line := range titleArt {
		c.moveCursor(centerX-titleWidth/2, titleStartY+i)
		fmt.Fprint(c.writer, line)
	}

	// Subtitle
	subtitle := "~ Multiplayer Asteroids over SSH ~"
	c.moveCursor(centerX-len(subtitle)/2, titleStartY+len(titleArt)+1)
	fmt.Fprint(c.writer, subtitle)

	// Controls section
	controlsY := titleStartY + len(titleArt) + 3
	controlHeader := "Controls"
	c.moveCursor(centerX-len(controlHeader)/2, controlsY)
	fmt.Fprint(c.writer, controlHeader)

	controlLines := []string{
		"W / Up  . . . . Thrust",
		"A D / < >  . .  Rotate",
		"SPACE  . . . . . Shoot",
		"Q  . . . . . . .  Quit",
	}
	for i, line := range controlLines {
		c.moveCursor(centerX-len(line)/2, controlsY+1+i)
		fmt.Fprint(c.writer, line)
	}

	// Blinking start prompt
	if time.Now().UnixMilli()/600%2 == 0 {
		prompt := ">>  Press SPACE to Start  <<"
		c.moveCursor(centerX-len(prompt)/2, controlsY+len(controlLines)+2)
		fmt.Fprint(c.writer, prompt)
	}

	// GitHub link (OSC 8 clickable hyperlink)
	ghURL := "https://github.com/tomz197/asshteroids"
	ghLabel := "Click to view on github"
	ghLine := fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", ghURL, ghLabel)
	c.moveCursor(centerX-len(ghLabel)/2, controlsY+len(controlLines)+4)
	fmt.Fprint(c.writer, ghLine)
	ghLabel2 := "github.com/tomz197/asshteroids"
	ghLine2 := fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", ghURL, ghLabel2)
	c.moveCursor(centerX-len(ghLabel2)/2, controlsY+len(controlLines)+5)
	fmt.Fprint(c.writer, ghLine2)
}

// drawPlayingHUD draws the in-game HUD.
// Text fields use fixed-width formatting so shrinking values don't leave
// residual characters on screen (since we no longer clear every frame).
func (c *Client) drawPlayingHUD(termWidth, termHeight int, snapshot *server.WorldSnapshot) {
	// Score display (top left) — left-aligned, padded to 8 digits
	scoreText := fmt.Sprintf("Score: %-8d", c.state.Score)
	c.moveCursor(2, 1)
	fmt.Fprint(c.writer, scoreText)

	// Lives display (top right)
	livesText := fmt.Sprintf("Lives: %-3d", c.state.Lives)
	c.moveCursor(termWidth-len(livesText)-1, 1)
	fmt.Fprint(c.writer, livesText)

	// Live players (bottom right)
	livePlayersText := fmt.Sprintf("Players: %-4d", snapshot.Players)
	c.moveCursor(termWidth-len(livePlayersText)-1, termHeight)
	fmt.Fprint(c.writer, livePlayersText)

	// Coordinates display (bottom left)
	if c.state.Player != nil {
		px, py := c.state.Player.GetPosition()
		coordText := fmt.Sprintf("X:%-5.0f Y:%-5.0f", px, py)
		c.moveCursor(2, termHeight)
		fmt.Fprint(c.writer, coordText)
	}
}

// drawDeadScreen draws the death/game over screen.
func (c *Client) drawDeadScreen(centerX, centerY int) {
	var titleArt []string
	if c.state.Lives > 0 {
		titleArt = []string{
			` __   _____  _   _   ___ ___ ___ ___   `,
			` \ \ / / _ \| | | | |   \_ _| __|   \  `,
			`  \ V / (_) | |_| | | |) | || _|| |) | `,
			`   |_| \___/ \___/  |___/___|___|___/  `,
			`                                       `,
		}
	} else {
		titleArt = []string{
			`   ___   _   __  __ ___    _____   _____ ___  `,
			`  / __| /_\ |  \/  | __|  / _ \ \ / / __| _ \ `,
			` | (_ |/ _ \| |\/| | _|  | (_) \ V /| _||   / `,
			`  \___/_/ \_\_|  |_|___|  \___/ \_/ |___|_|_\ `,
			`                                              `,
		}
	}

	// Find max width for centering
	titleWidth := 0
	for _, line := range titleArt {
		if len(line) > titleWidth {
			titleWidth = len(line)
		}
	}

	// Draw title art
	titleStartY := centerY - 6
	for i, line := range titleArt {
		c.moveCursor(centerX-titleWidth/2, titleStartY+i)
		fmt.Fprint(c.writer, line)
	}

	// Score
	scoreText := fmt.Sprintf("Score: %d", c.state.Score)
	c.moveCursor(centerX-len(scoreText)/2, titleStartY+len(titleArt)+1)
	fmt.Fprint(c.writer, scoreText)

	// Lives or game over info
	if c.state.Lives > 0 {
		livesText := fmt.Sprintf("Lives remaining: %d", c.state.Lives)
		c.moveCursor(centerX-len(livesText)/2, titleStartY+len(titleArt)+3)
		fmt.Fprint(c.writer, livesText)
	}

	// Respawn countdown or prompt
	if c.state.RespawnTimeRemaining > 0 {
		countdown := fmt.Sprintf("Respawn in %.1f seconds...", c.state.RespawnTimeRemaining)
		c.moveCursor(centerX-len(countdown)/2, titleStartY+len(titleArt)+5)
		fmt.Fprint(c.writer, countdown)
	} else if time.Now().UnixMilli()/600%2 == 0 {
		var prompt string
		if c.state.Lives > 0 {
			prompt = ">>  Press SPACE to Continue  <<"
		} else {
			prompt = ">>  Press SPACE to Restart  <<"
		}
		c.moveCursor(centerX-len(prompt)/2, titleStartY+len(titleArt)+5)
		fmt.Fprint(c.writer, prompt)
	}
}

// drawShutdownScreen draws the server shutdown notification screen.
func (c *Client) drawShutdownScreen(centerX, centerY int) {
	title := "SERVER SHUTTING DOWN"
	c.moveCursor(centerX-len(title)/2, centerY-3)
	fmt.Fprint(c.writer, title)

	msg1 := "The server is restarting for maintenance."
	c.moveCursor(centerX-len(msg1)/2, centerY-1)
	fmt.Fprint(c.writer, msg1)

	msg2 := "Please reconnect in a moment."
	c.moveCursor(centerX-len(msg2)/2, centerY)
	fmt.Fprint(c.writer, msg2)

	remaining := int(c.state.shutdownTimer) + 1
	countdown := fmt.Sprintf("Disconnecting in %d seconds...", remaining)
	c.moveCursor(centerX-len(countdown)/2, centerY+2)
	fmt.Fprint(c.writer, countdown)

	hint := "Press Q to disconnect now"
	c.moveCursor(centerX-len(hint)/2, centerY+4)
	fmt.Fprint(c.writer, hint)
}

// drawPlayerNames draws usernames above other players' ships.
// Marks the drawn cells as dirty so the canvas overwrites them next frame,
// preventing stale name text from persisting when ships move.
func (c *Client) drawPlayerNames(userObjects []*object.User, world object.Screen) {
	termWidth := c.canvas.TerminalWidth()
	termHeight := c.canvas.TerminalHeight()

	for _, user := range userObjects {
		if user == c.state.Player || user.Username == "" {
			continue
		}

		// Get screen positions (handles world wrapping)
		positions := object.WorldToScreen(user.X, user.Y, c.state.Camera, c.state.View, world)
		for i := 0; i < positions.Count; i++ {
			pos := positions.Positions[i]

			// Convert logical position to terminal coordinates, offset above the ship
			col, row := c.canvas.LogicalToTerminal(pos.X, pos.Y-user.Size-2)

			// Center the username horizontally
			col -= len(user.Username) / 2

			// Clamp to screen bounds
			if row < 1 || row > termHeight {
				continue
			}
			if col < 1 || col+len(user.Username) > termWidth {
				continue
			}

			c.moveCursor(col, row)
			fmt.Fprint(c.writer, user.Username)

			// Mark these cells dirty so the canvas cleans them up next frame
			c.canvas.MarkTextDirty(col, row, len(user.Username))
		}
	}
}
