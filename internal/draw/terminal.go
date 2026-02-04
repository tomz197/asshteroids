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

// TerminalSize returns the terminal width and height using the default size function.
// Height is in sub-pixel units (2x terminal rows) for use with Canvas.
func TerminalSize() (Screen, error) {
	return TerminalSizeWith(DefaultTermSizeFunc)
}

// TerminalSizeWith returns terminal size using the provided size function.
func TerminalSizeWith(sizeFunc TermSizeFunc) (Screen, error) {
	width, height, err := sizeFunc()
	if err != nil {
		return Screen{}, err
	}
	// Return height as sub-pixel height (2x terminal rows)
	subPixelHeight := height * 2
	return Screen{
		Width:   width,
		Height:  subPixelHeight,
		CenterX: width / 2,
		CenterY: subPixelHeight / 2,
	}, nil
}

// TerminalSizeRaw returns the actual terminal dimensions without sub-pixel scaling.
func TerminalSizeRaw() (width, height int, err error) {
	return DefaultTermSizeFunc()
}

// TerminalSizeRawWith returns actual terminal dimensions using the provided size function.
func TerminalSizeRawWith(sizeFunc TermSizeFunc) (width, height int, err error) {
	return sizeFunc()
}

// DrawChar draws a single character at the given position (1-based coordinates).
func DrawChar(w io.Writer, x, y int, ch rune) {
	if x >= 1 && y >= 1 {
		fmt.Fprintf(w, "\033[%d;%dH%c", y, x, ch)
	}
}
