package client

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tomz197/asteroids/internal/draw"
	"github.com/tomz197/asteroids/internal/loop/config"
	"github.com/tomz197/asteroids/internal/loop/server"
	"github.com/tomz197/asteroids/internal/object"
)

// drawFrame draws the current frame.
func (c *Client) drawFrame() error {
	// On game state, inactivity, or chat transitions, do a full terminal clear
	// so UI elements from the previous state don't persist on screen.
	stateChanged := c.state.GameState != c.state.prevGameState
	inactiveChanged := c.state.isInactive != c.state.wasInactive
	chatOpenChanged := c.state.ChatOpen != c.state.prevChatOpen
	if stateChanged || inactiveChanged || chatOpenChanged {
		c.chunkWriter.WriteString("\033[H\033[2J")
		c.canvas.ForceRedraw()
		c.state.prevGameState = c.state.GameState
		c.state.wasInactive = c.state.isInactive
		c.state.prevChatOpen = c.state.ChatOpen
	}

	c.canvas.Clear()

	// Get world snapshot
	snapshot := c.server.GetSnapshot()

	// Create draw context
	ctx := object.DrawContext{
		Canvas: c.canvas,
		Writer: c.chunkWriter,
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
	c.canvas.Render(c.chunkWriter)

	// Draw border when terminal exceeds max render resolution
	c.canvas.RenderBorder(c.chunkWriter)

	// Draw usernames above other players' ships
	c.drawPlayerNames(snapshot.UserObjects, snapshot.World)

	// Draw UI overlay
	c.drawUI(snapshot)

	// Draw chat (overlays all screens)
	c.drawChat(snapshot)

	return c.chunkWriter.Flush()
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
		c.drawStartScreen(centerX, centerY, snapshot)
	case GameStateDead:
		c.drawDeadScreen(centerX, centerY)
	}
}

// chatHistoryLines is the number of chat history lines to display when chat is closed.
const chatHistoryLines = 6

// chatHistoryLinesActive is the number of chat history lines when chat input is open.
const chatHistoryLinesActive = 12

// chatWidth is the fixed width of the chat column (narrow so game remains visible).
const chatWidth = 40

// drawChat draws the chat history and input box. Overlays all screens.
// Uses a narrow column so asteroids remain visible. Pads lines to clear artefacts.
func (c *Client) drawChat(snapshot *server.WorldSnapshot) {
	termHeight := c.canvas.TerminalHeight()
	cw := c.chunkWriter

	messages := snapshot.ChatMessages
	if messages == nil {
		messages = []server.ChatMessage{}
	}

	msgRows := chatHistoryLines
	if c.state.ChatOpen {
		msgRows = chatHistoryLinesActive
	}
	displayCount := msgRows
	if len(messages) < displayCount {
		displayCount = len(messages)
	}
	displayMessages := messages
	if len(messages) > msgRows {
		start := len(messages) - msgRows
		if start < 0 {
			start = 0
		}
		displayMessages = messages[start:]
		displayCount = msgRows
	}

	// Layout: when chat open, input and hint use 2 rows at bottom
	inputRow := termHeight - 1
	hintRow := termHeight
	historyStart := termHeight - msgRows
	if c.state.ChatOpen {
		historyStart = termHeight - msgRows - 2
		inputRow = termHeight - 1
		hintRow = termHeight
	}

	// Draw messages (wrap to multiple lines if needed)
	var allLines []string
	for i := 0; i < displayCount; i++ {
		m := displayMessages[i]
		fullLine := truncate(m.Username, 12) + ": " + m.Text
		allLines = append(allLines, wrapText(fullLine, chatWidth)...)
	}
	// Take last N lines to fit in available rows (newest at bottom)
	lineStart := 0
	if len(allLines) > msgRows {
		lineStart = len(allLines) - msgRows
	}
	for i := 0; i < msgRows && lineStart+i < len(allLines); i++ {
		row := historyStart + i
		if row >= 1 && row <= termHeight {
			line := allLines[lineStart+i]
			c.canvas.MarkTextDirty(2, row, len(line))
			cw.WriteAt(2, row, line)
		}
	}

	if c.state.ChatOpen {
		hint := "ESC to close, Enter to send"
		cw.WriteAt(2, hintRow, hint)

		prompt := "> " + c.state.ChatInput
		if utf8.RuneCountInString(prompt) > config.MaxChatMessageLength {
			prompt = truncate(prompt, config.MaxChatMessageLength)
		}
		cw.WriteAt(2, inputRow, prompt)
		c.canvas.MarkTextDirty(2, inputRow, chatWidth)
		c.canvas.MarkTextDirty(2, hintRow, chatWidth)
	} else {
		hint := "Press C to strat chatting"
		cw.WriteAt(2, hintRow, hint)
	}
}

// drawInactivityScreen draws the inactivity warning screen.
func (c *Client) drawInactivityScreen(centerX, centerY int) {
	cw := c.chunkWriter
	title := "INACTIVITY WARNING"
	cw.WriteAt(centerX-len(title)/2, centerY-2, title)

	msg := fmt.Sprintf(
		"You have been inactive for too long. You will be disconnected in %d seconds.",
		int(config.InactivityDisconnectUser-time.Since(c.lastInput).Seconds()),
	)
	cw.WriteAt(centerX-len(msg)/2, centerY, msg)

	hint := "Press any key to continue"
	cw.WriteAt(centerX-len(hint)/2, centerY+2, hint)
}

// drawStartScreen draws the title screen.
func (c *Client) drawStartScreen(centerX, centerY int, snapshot *server.WorldSnapshot) {
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
	cw := c.chunkWriter
	titleStartY := centerY - 7
	for i, line := range titleArt {
		cw.WriteAt(centerX-titleWidth/2, titleStartY+i, line)
	}

	// Subtitle
	subtitle := "~ Multiplayer Asteroids over SSH ~"
	cw.WriteAt(centerX-len(subtitle)/2, titleStartY+len(titleArt)+1, subtitle)

	// Controls section
	controlsY := titleStartY + len(titleArt) + 3
	controlHeader := "Controls"
	cw.WriteAt(centerX-len(controlHeader)/2, controlsY, controlHeader)

	controlLines := []string{
		"W / Up  . . . . Thrust",
		"A D / < >  . .  Rotate",
		"SPACE  . . . . . Shoot",
		"C  . . . . . . . Chat",
		"Q  . . . . . . .  Quit",
	}
	for i, line := range controlLines {
		cw.WriteAt(centerX-len(line)/2, controlsY+1+i, line)
	}

	// Blinking start prompt
	if time.Now().UnixMilli()/600%2 == 0 {
		prompt := ">>  Press SPACE to Start  <<"
		cw.WriteAt(centerX-len(prompt)/2, controlsY+len(controlLines)+2, prompt)
	}

	// Top scores (right of controls)
	c.drawTopScores(cw, centerX+22, controlsY, snapshot.TopScores)

	// GitHub link (OSC 8 clickable hyperlink)
	ghURL := "https://github.com/tomz197/asshteroids"
	ghLabel := "Click to view on github"
	ghLine := fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", ghURL, ghLabel)
	cw.WriteAt(centerX-len(ghLabel)/2, controlsY+len(controlLines)+4, ghLine)
	ghLabel2 := "github.com/tomz197/asshteroids"
	ghLine2 := fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", ghURL, ghLabel2)
	cw.WriteAt(centerX-len(ghLabel2)/2, controlsY+len(controlLines)+5, ghLine2)
}

// drawTopScores draws the top scores leaderboard at the given position.
func (c *Client) drawTopScores(cw *draw.ChunkWriter, col, row int, topScores []server.TopScoreEntry) {
	if len(topScores) == 0 {
		return
	}
	header := "Top Scores"
	cw.WriteAt(col, row, header)
	for i, e := range topScores {
		line := fmt.Sprintf("#%-2d %-12s %6d", i+1, truncate(e.Username, 12), e.Score)
		cw.WriteAt(col, row+1+i, line)
	}
}

// wrapText splits s into lines of at most maxWidth runes.
func wrapText(s string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{s}
	}
	var lines []string
	for len(s) > 0 {
		if utf8.RuneCountInString(s) <= maxWidth {
			lines = append(lines, s)
			break
		}
		n := 0
		cut := len(s) // fallback if inner loop never breaks
		for i := range s {
			if n == maxWidth {
				cut = i
				break
			}
			n++
		}
		lines = append(lines, s[:cut])
		s = strings.TrimLeft(s[cut:], " ")
	}
	return lines
}

// truncate shortens s to at most maxLen runes.
func truncate(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	n := 0
	for i := range s {
		if n == maxLen {
			return s[:i]
		}
		n++
	}
	return s
}

// drawPlayingHUD draws the in-game HUD.
// Text fields use fixed-width formatting so shrinking values don't leave
// residual characters on screen (since we no longer clear every frame).
func (c *Client) drawPlayingHUD(termWidth, termHeight int, snapshot *server.WorldSnapshot) {
	cw := c.chunkWriter
	// Score display (top left) — left-aligned, padded to 8 digits
	scoreText := fmt.Sprintf("Score: %-8d", c.state.Score)
	cw.WriteAt(2, 1, scoreText)

	// Top scores (left, below score) — show top 5 in-game to save space
	top5 := snapshot.TopScores
	if len(top5) > 5 {
		top5 = top5[:5]
	}
	c.drawTopScores(cw, 2, 3, top5)

	// Lives display (top right)
	livesText := fmt.Sprintf("Lives: %-3d", c.state.Lives)
	cw.WriteAt(termWidth-len(livesText)-1, 1, livesText)

	// Minimap (top right, below lives)
	minimapStartCol := termWidth - minimapWidth - 3
	minimapStartRow := 3
	if c.state.Player != nil {
		c.drawMinimap(termWidth, termHeight, snapshot)
	}

	// Coordinates display (under minimap)
	if c.state.Player != nil && minimapStartCol >= 1 && minimapStartRow+minimapHeight+2 <= termHeight {
		px, py := c.state.Player.GetPosition()
		coordText := fmt.Sprintf("X:%-5.0f Y:%-5.0f", px, py)
		cw.WriteAt(minimapStartCol, minimapStartRow+minimapHeight+2, coordText)
	}

	// Live players (bottom right)
	livePlayersText := fmt.Sprintf("Players: %-4d", snapshot.Players)
	cw.WriteAt(termWidth-len(livePlayersText)-1, termHeight, livePlayersText)
}

// drawMinimap draws a small overview of the world showing the local player and others.
// Uses half-block characters (▀▄█) for 2x vertical resolution. Self is bright cyan, others dim.
func (c *Client) drawMinimap(termWidth, termHeight int, snapshot *server.WorldSnapshot) {
	worldW := float64(snapshot.World.Width)
	worldH := float64(snapshot.World.Height)
	if worldW <= 0 || worldH <= 0 {
		return
	}

	// Build minimap grid: 0=empty, 1=other, 2=self (self overwrites)
	grid := &c.state.minimapGrid
	*grid = [minimapSubRows][minimapWidth]byte{} // Clear

	// Map all players to grid cells (2x vertical resolution)
	for _, user := range snapshot.UserObjects {
		x, y := user.GetPosition()
		col := int(x / worldW * float64(minimapWidth))
		subRow := int(y / worldH * float64(minimapSubRows))
		if col < 0 {
			col = 0
		}
		if col >= minimapWidth {
			col = minimapWidth - 1
		}
		if subRow < 0 {
			subRow = 0
		}
		if subRow >= minimapSubRows {
			subRow = minimapSubRows - 1
		}
		if user == c.state.Player {
			grid[subRow][col] = 2 // Self
		} else if grid[subRow][col] == 0 {
			grid[subRow][col] = 1 // Other (don't overwrite self)
		}
	}

	// Position: top-right, below lives
	startCol := termWidth - minimapWidth - 3 // border + padding
	startRow := 3
	if startCol < 1 || startRow+minimapHeight+1 > termHeight {
		return // Not enough space
	}

	// Accumulate minimap output for chunked write
	cw := c.chunkWriter
	cw.WriteAt(startCol, startRow, "┌"+strings.Repeat("─", minimapWidth)+"┐")
	c.canvas.MarkTextDirty(startCol, startRow, minimapWidth+2)

	// Each terminal row combines 2 sub-rows via half-block characters (▀▄█)
	for termRow := 0; termRow < minimapHeight; termRow++ {
		cw.WriteAt(startCol, startRow+1+termRow, "│")
		curColor := ""
		for col := 0; col < minimapWidth; col++ {
			top := grid[termRow*2][col]
			bot := grid[termRow*2+1][col]
			topFilled := top != 0
			botFilled := bot != 0
			isSelf := top == 2 || bot == 2
			wantColor := draw.ColorReset // Default color for others
			if isSelf {
				wantColor = draw.ColorBrightCyan // Bright cyan for current player
			}
			var r rune
			switch {
			case topFilled && botFilled:
				r = draw.BlockFull
			case topFilled && !botFilled:
				r = draw.BlockUpperHalf
			case !topFilled && botFilled:
				r = draw.BlockLowerHalf
			default:
				r = ' '
			}
			if r != ' ' {
				if curColor != wantColor {
					cw.WriteString(wantColor)
					curColor = wantColor
				}
			} else if curColor != "" {
				cw.WriteString(draw.ColorReset)
				curColor = ""
			}
			cw.WriteRune(r)
		}
		if curColor != "" {
			cw.WriteString(draw.ColorReset)
		}
		cw.WriteString("│")
		c.canvas.MarkTextDirty(startCol, startRow+1+termRow, minimapWidth+2)
	}

	cw.WriteAt(startCol, startRow+1+minimapHeight, "└"+strings.Repeat("─", minimapWidth)+"┘")
	c.canvas.MarkTextDirty(startCol, startRow+1+minimapHeight, minimapWidth+2)

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
	cw := c.chunkWriter
	titleStartY := centerY - 6
	for i, line := range titleArt {
		cw.WriteAt(centerX-titleWidth/2, titleStartY+i, line)
	}

	// Killed by (when killed by another player)
	offset := 0
	if c.state.KilledBy != "" {
		killedByText := fmt.Sprintf("Killed by %s", c.state.KilledBy)
		cw.WriteAt(centerX-len(killedByText)/2, titleStartY+len(titleArt)+offset, killedByText)
		offset++
	}

	// Score
	scoreText := fmt.Sprintf("Score: %d", c.state.Score)
	cw.WriteAt(centerX-len(scoreText)/2, titleStartY+len(titleArt)+offset+1, scoreText)

	// Lives or game over info
	if c.state.Lives > 0 {
		livesText := fmt.Sprintf("Lives remaining: %d", c.state.Lives)
		cw.WriteAt(centerX-len(livesText)/2, titleStartY+len(titleArt)+3, livesText)
	}

	// Respawn countdown or prompt
	if c.state.RespawnTimeRemaining > 0 {
		countdown := fmt.Sprintf("Respawn in %.1f seconds...", c.state.RespawnTimeRemaining)
		cw.WriteAt(centerX-len(countdown)/2, titleStartY+len(titleArt)+5, countdown)
	} else if time.Now().UnixMilli()/600%2 == 0 {
		var prompt string
		if c.state.Lives > 0 {
			prompt = ">>  Press SPACE to Continue  <<"
		} else {
			prompt = ">>  Press SPACE to Restart  <<"
		}
		cw.WriteAt(centerX-len(prompt)/2, titleStartY+len(titleArt)+5, prompt)
	}
	escapeHint := "ESC to return to menu"
	cw.WriteAt(centerX-len(escapeHint)/2, titleStartY+len(titleArt)+7, escapeHint)
}

// drawShutdownScreen draws the server shutdown notification screen.
func (c *Client) drawShutdownScreen(centerX, centerY int) {
	cw := c.chunkWriter
	title := "SERVER SHUTTING DOWN"
	cw.WriteAt(centerX-len(title)/2, centerY-3, title)

	msg1 := "The server is restarting for maintenance."
	cw.WriteAt(centerX-len(msg1)/2, centerY-1, msg1)

	msg2 := "Please reconnect in a moment."
	cw.WriteAt(centerX-len(msg2)/2, centerY, msg2)

	remaining := int(c.state.shutdownTimer) + 1
	countdown := fmt.Sprintf("Disconnecting in %d seconds...", remaining)
	cw.WriteAt(centerX-len(countdown)/2, centerY+2, countdown)

	hint := "Press Q to disconnect now"
	cw.WriteAt(centerX-len(hint)/2, centerY+4, hint)
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

			c.chunkWriter.WriteAt(col, row, user.Username)

			// Mark these cells dirty so the canvas cleans them up next frame
			c.canvas.MarkTextDirty(col, row, len(user.Username))
		}
	}
}
