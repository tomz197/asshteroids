package draw

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// ChunkWriter accumulates text for terminal output and writes in chunks for optimal
// network flow (e.g. over SSH). Use MoveCursor, WriteString, WriteRune to accumulate,
// then Flush to write to the underlying writer. Implements io.Writer for Canvas.Render.
type ChunkWriter struct {
	buf    strings.Builder
	w      io.Writer
	offCol int
	offRow int
}

// NewChunkWriter creates a ChunkWriter that writes to w. offsetCol and offsetRow
// are added to all MoveCursor coordinates (for canvas centering).
func NewChunkWriter(w io.Writer, offsetCol, offsetRow int) *ChunkWriter {
	return &ChunkWriter{w: w, offCol: offsetCol, offRow: offsetRow}
}

// SetOffset updates the cursor offset (e.g. after terminal resize).
func (cw *ChunkWriter) SetOffset(offsetCol, offsetRow int) {
	cw.offCol = offsetCol
	cw.offRow = offsetRow
}

// MoveCursor appends an ANSI cursor position sequence. col and row are 1-based
// canvas coordinates; offset is applied automatically.
func (cw *ChunkWriter) MoveCursor(col, row int) {
	cw.buf.WriteString("\033[")
	cw.buf.WriteString(strconv.Itoa(row + cw.offRow))
	cw.buf.WriteByte(';')
	cw.buf.WriteString(strconv.Itoa(col + cw.offCol))
	cw.buf.WriteByte('H')
}

// Write implements io.Writer for use with Canvas.Render and other writers.
func (cw *ChunkWriter) Write(p []byte) (n int, err error) {
	return cw.buf.Write(p)
}

// WriteString appends a string to the buffer.
func (cw *ChunkWriter) WriteString(s string) {
	cw.buf.WriteString(s)
}

// WriteAt writes a string at a specific position. col and row are 1-based canvas coordinates; offset is applied automatically.
func (cw *ChunkWriter) WriteAt(col, row int, s string) {
	cw.MoveCursor(col, row)
	cw.buf.WriteString(s)
}

// WriteRune appends a rune to the buffer.
func (cw *ChunkWriter) WriteRune(r rune) {
	cw.buf.WriteRune(r)
}

// Ensure ChunkWriter satisfies io.Writer.
var _ io.Writer = (*ChunkWriter)(nil)

// Flush writes the accumulated buffer to the underlying writer in chunks,
// then resets the buffer. Uses the same chunk size as Canvas.Render.
func (cw *ChunkWriter) Flush() error {
	data := cw.buf.String()
	cw.buf.Reset()
	for len(data) > 0 {
		chunk := data
		if len(chunk) > maxChunkSize {
			chunk = data[:maxChunkSize]
		}
		if _, err := io.WriteString(cw.w, chunk); err != nil {
			return err
		}
		data = data[len(chunk):]
	}
	return nil
}

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
