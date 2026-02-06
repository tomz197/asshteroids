package draw

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// TermSizeFunc is a function that returns the terminal dimensions.
type TermSizeFunc func() (width, height int, err error)

// DefaultTermSizeFunc returns terminal size from os.Stdout.
var DefaultTermSizeFunc TermSizeFunc = func() (int, int, error) {
	return term.GetSize(int(os.Stdout.Fd()))
}

// ClearScreen clears the terminal and moves cursor to top-left.
func ClearScreen(w io.Writer) {
	fmt.Fprint(w, "\033[H\033[2J")
}

// HideCursor hides the terminal cursor.
func HideCursor(w io.Writer) {
	fmt.Fprint(w, "\033[?25l")
}

// ShowCursor shows the terminal cursor.
func ShowCursor(w io.Writer) {
	fmt.Fprint(w, "\033[?25h")
}

// MoveCursor moves cursor to a specific position (1-based).
func MoveCursor(w io.Writer, x, y int) {
	fmt.Fprintf(w, "\033[%d;%dH", y, x)
}

// TerminalSizeRawWith returns actual terminal dimensions using the provided size function.
func TerminalSizeRawWith(sizeFunc TermSizeFunc) (width, height int, err error) {
	return sizeFunc()
}
