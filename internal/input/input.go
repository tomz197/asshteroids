package input

import (
	"bufio"
	"time"
)

// keyHoldDuration is how long a key is considered "held" after its last press.
const keyHoldDuration = 30 * time.Millisecond

// Input represents the current frame's input state.
type Input struct {
	Quit      bool
	Left      bool
	Right     bool
	UpLeft    bool
	UpRight   bool
	Up        bool
	Down      bool
	Space     bool
	Enter     bool
	Backspace bool
	Delete    bool
	Escape    bool
	Number    int
	Pressed   []byte
}

// keyState tracks the last time each key was pressed.
type keyState struct {
	quit      time.Time
	left      time.Time
	right     time.Time
	upLeft    time.Time
	upRight   time.Time
	up        time.Time
	down      time.Time
	space     time.Time
	enter     time.Time
	backspace time.Time
	delete_   time.Time
	escape    time.Time
	number    time.Time
	numberVal int
}

// Stream delivers input bytes via a channel and tracks key state for combinations.
type Stream struct {
	ch    chan byte
	state keyState
}

// StartStream spawns a goroutine that reads from r and sends bytes to the stream.
func StartStream(r *bufio.Reader) *Stream {
	s := &Stream{
		ch:    make(chan byte, 128),
		state: keyState{numberVal: -1},
	}
	go func() {
		for {
			b, err := r.ReadByte()
			if err != nil {
				close(s.ch)
				return
			}
			s.ch <- b
		}
	}()
	return s
}

// ReadInput drains all available bytes from the stream (non-blocking).
// Handles escape sequences for arrow keys and accumulates all pressed keys.
// Uses key state persistence to allow detecting simultaneous key combinations.
func ReadInput(s *Stream) Input {
	now := time.Now()
	var buf []byte

	// Drain all available bytes
	for {
		select {
		case b, ok := <-s.ch:
			if !ok {
				break
			}
			buf = append(buf, b)
		default:
			goto parse
		}
	}

parse:
	// Parse the collected bytes and update key state timestamps
	for i := 0; i < len(buf); i++ {
		b := buf[i]

		// Check for escape sequences (arrow keys, etc.)
		if b == '\x1b' && i+2 < len(buf) && buf[i+1] == '[' {
			// CSI sequence: ESC [ <code>
			switch buf[i+2] {
			case 'A': // Up arrow
				s.state.up = now
				i += 2
				continue
			case 'B': // Down arrow
				s.state.down = now
				i += 2
				continue
			case 'C': // Right arrow
				s.state.right = now
				i += 2
				continue
			case 'D': // Left arrow
				s.state.left = now
				i += 2
				continue
			}
		}

		// Single byte handling - update key state
		applyByteToState(&s.state, b, now)
	}

	// Build input from key state - keys are "pressed" if seen within hold duration
	input := Input{
		Quit:      now.Sub(s.state.quit) < keyHoldDuration,
		Left:      now.Sub(s.state.left) < keyHoldDuration,
		Right:     now.Sub(s.state.right) < keyHoldDuration,
		UpLeft:    now.Sub(s.state.upLeft) < keyHoldDuration,
		UpRight:   now.Sub(s.state.upRight) < keyHoldDuration,
		Up:        now.Sub(s.state.up) < keyHoldDuration,
		Down:      now.Sub(s.state.down) < keyHoldDuration,
		Space:     now.Sub(s.state.space) < keyHoldDuration,
		Enter:     now.Sub(s.state.enter) < keyHoldDuration,
		Backspace: now.Sub(s.state.backspace) < keyHoldDuration,
		Delete:    now.Sub(s.state.delete_) < keyHoldDuration,
		Escape:    now.Sub(s.state.escape) < keyHoldDuration,
		Number:    -1,
		Pressed:   buf,
	}

	// Number is only set if recently pressed
	if now.Sub(s.state.number) < keyHoldDuration {
		input.Number = s.state.numberVal
	}

	return input
}

// applyByteToState updates the key state timestamps based on the pressed byte.
func applyByteToState(state *keyState, b byte, now time.Time) {
	switch b {
	case 'q', 'Q':
		state.quit = now
	case 'a', 'A', 'j', 'J':
		state.left = now
	case 'd', 'D', 'l', 'L':
		state.right = now
	case 'w', 'W', 'i', 'I':
		state.up = now
	case 's', 'S', 'k', 'K':
		state.down = now
	case 'u', 'U':
		state.upLeft = now
	case 'o', 'O':
		state.upRight = now
	case ' ':
		state.space = now
	case '\n', '\r':
		state.enter = now
	case '\b':
		state.backspace = now
	case '\x7f':
		state.delete_ = now
	case '\x1b':
		state.escape = now
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		state.number = now
		state.numberVal = int(b - '0')
	}
}
