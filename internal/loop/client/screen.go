package client

import (
	"fmt"
	"time"

	"github.com/tomz197/asteroids/internal/draw"
	"github.com/tomz197/asteroids/internal/loop/config"
	"github.com/tomz197/asteroids/internal/object"
)

// drawFrame draws the current frame.
func (c *Client) drawFrame() error {
	draw.ClearScreen(c.writer)
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

	// Draw usernames above other players' ships
	c.drawPlayerNames(snapshot.UserObjects, snapshot.World)

	// Draw UI overlay
	c.drawUI()

	return nil
}

// drawUI draws the game UI overlay.
func (c *Client) drawUI() {
	termWidth := c.canvas.TerminalWidth()
	termHeight := c.canvas.TerminalHeight()
	centerX := termWidth / 2
	centerY := termHeight / 2

	if c.state.isInactive {
		c.drawInactivityScreen(centerX, centerY)
		return
	}

	switch c.state.GameState {
	case GameStateShutdown:
		c.drawShutdownScreen(centerX, centerY)
	case GameStatePlaying:
		c.drawPlayingHUD(termWidth, termHeight)
	case GameStateStart:
		c.drawStartScreen(centerX, centerY)
	case GameStateDead:
		c.drawDeadScreen(centerX, centerY)
	}
}

// drawInactivityScreen draws the inactivity warning screen.
func (c *Client) drawInactivityScreen(centerX, centerY int) {
	title := "INACTIVITY WARNING"
	draw.MoveCursor(c.writer, centerX-len(title)/2, centerY-2)
	fmt.Fprint(c.writer, title)

	msg := fmt.Sprintf(
		"You have been inactive for too long. You will be disconnected in %d seconds.",
		int(config.InactivityDisconnectUser-time.Since(c.lastInput).Seconds()),
	)
	draw.MoveCursor(c.writer, centerX-len(msg)/2, centerY)
	fmt.Fprint(c.writer, msg)

	hint := "Press any key to continue"
	draw.MoveCursor(c.writer, centerX-len(hint)/2, centerY+2)
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
		draw.MoveCursor(c.writer, centerX-titleWidth/2, titleStartY+i)
		fmt.Fprint(c.writer, line)
	}

	// Subtitle
	subtitle := "~ Multiplayer Asteroids over SSH ~"
	draw.MoveCursor(c.writer, centerX-len(subtitle)/2, titleStartY+len(titleArt)+1)
	fmt.Fprint(c.writer, subtitle)

	// Controls section
	controlsY := titleStartY + len(titleArt) + 3
	controlHeader := "Controls"
	draw.MoveCursor(c.writer, centerX-len(controlHeader)/2, controlsY)
	fmt.Fprint(c.writer, controlHeader)

	controlLines := []string{
		"W / Up  . . . . Thrust",
		"A D / < >  . .  Rotate",
		"SPACE  . . . . . Shoot",
		"Q  . . . . . . .  Quit",
	}
	for i, line := range controlLines {
		draw.MoveCursor(c.writer, centerX-len(line)/2, controlsY+1+i)
		fmt.Fprint(c.writer, line)
	}

	// Blinking start prompt
	if time.Now().UnixMilli()/600%2 == 0 {
		prompt := ">>  Press SPACE to Start  <<"
		draw.MoveCursor(c.writer, centerX-len(prompt)/2, controlsY+len(controlLines)+2)
		fmt.Fprint(c.writer, prompt)
	}

	// GitHub link (OSC 8 clickable hyperlink)
	ghURL := "https://github.com/tomz197/asshteroids"
	ghLabel := "Click to view on github"
	ghLine := fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", ghURL, ghLabel)
	draw.MoveCursor(c.writer, centerX-len(ghLabel)/2, controlsY+len(controlLines)+4)
	fmt.Fprint(c.writer, ghLine)
	ghLabel2 := "github.com/tomz197/asshteroids"
	ghLine2 := fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", ghURL, ghLabel2)
	draw.MoveCursor(c.writer, centerX-len(ghLabel2)/2, controlsY+len(controlLines)+5)
	fmt.Fprint(c.writer, ghLine2)
}

// drawPlayingHUD draws the in-game HUD.
func (c *Client) drawPlayingHUD(termWidth, termHeight int) {
	snapshot := c.server.GetSnapshot()
	// Score display (top left)
	scoreText := fmt.Sprintf("Score: %d", c.state.Score)
	draw.MoveCursor(c.writer, 2, 1)
	fmt.Fprint(c.writer, scoreText)

	// Lives display (top right)
	livesText := fmt.Sprintf("Lives: %d", c.state.Lives)
	draw.MoveCursor(c.writer, termWidth-len(livesText)-1, 1)
	fmt.Fprint(c.writer, livesText)

	// Live players (bottom right)
	livePlayersText := fmt.Sprintf("Players: %d", snapshot.Players)
	draw.MoveCursor(c.writer, termWidth-len(livePlayersText)-1, termHeight)
	fmt.Fprint(c.writer, livePlayersText)

	// Coordinates display (bottom left)
	if c.state.Player != nil {
		px, py := c.state.Player.GetPosition()
		coordText := fmt.Sprintf("X:%.0f Y:%.0f", px, py)
		draw.MoveCursor(c.writer, 2, termHeight)
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
		draw.MoveCursor(c.writer, centerX-titleWidth/2, titleStartY+i)
		fmt.Fprint(c.writer, line)
	}

	// Score
	scoreText := fmt.Sprintf("Score: %d", c.state.Score)
	draw.MoveCursor(c.writer, centerX-len(scoreText)/2, titleStartY+len(titleArt)+1)
	fmt.Fprint(c.writer, scoreText)

	// Lives or game over info
	if c.state.Lives > 0 {
		livesText := fmt.Sprintf("Lives remaining: %d", c.state.Lives)
		draw.MoveCursor(c.writer, centerX-len(livesText)/2, titleStartY+len(titleArt)+3)
		fmt.Fprint(c.writer, livesText)
	}

	// Blinking prompt
	if time.Now().UnixMilli()/600%2 == 0 {
		var prompt string
		if c.state.Lives > 0 {
			prompt = ">>  Press SPACE to Continue  <<"
		} else {
			prompt = ">>  Press SPACE to Restart  <<"
		}
		draw.MoveCursor(c.writer, centerX-len(prompt)/2, titleStartY+len(titleArt)+5)
		fmt.Fprint(c.writer, prompt)
	}
}

// drawShutdownScreen draws the server shutdown notification screen.
func (c *Client) drawShutdownScreen(centerX, centerY int) {
	title := "SERVER SHUTTING DOWN"
	draw.MoveCursor(c.writer, centerX-len(title)/2, centerY-3)
	fmt.Fprint(c.writer, title)

	msg1 := "The server is restarting for maintenance."
	draw.MoveCursor(c.writer, centerX-len(msg1)/2, centerY-1)
	fmt.Fprint(c.writer, msg1)

	msg2 := "Please reconnect in a moment."
	draw.MoveCursor(c.writer, centerX-len(msg2)/2, centerY)
	fmt.Fprint(c.writer, msg2)

	remaining := int(c.state.shutdownTimer) + 1
	countdown := fmt.Sprintf("Disconnecting in %d seconds...", remaining)
	draw.MoveCursor(c.writer, centerX-len(countdown)/2, centerY+2)
	fmt.Fprint(c.writer, countdown)

	hint := "Press Q to disconnect now"
	draw.MoveCursor(c.writer, centerX-len(hint)/2, centerY+4)
	fmt.Fprint(c.writer, hint)
}

// drawPlayerNames draws usernames above other players' ships.
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
			if col < 1 {
				col = 1
			}
			if col+len(user.Username) > termWidth {
				continue
			}

			draw.MoveCursor(c.writer, col, row)
			fmt.Fprint(c.writer, user.Username)
		}
	}
}
