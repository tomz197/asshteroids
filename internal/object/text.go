package object

import (
	"fmt"
	"io"
)

// Text is a simple drawable text object.
// Coordinates are 1-based terminal positions.
type Text struct {
	X     int
	Y     int
	Value string
}

// Draw writes the text at its position using ANSI cursor movement.
func (t Text) Draw(w io.Writer) error {
	if t.Value == "" {
		return nil
	}
	x := t.X
	y := t.Y
	if x < 1 {
		x = 1
	}
	if y < 1 {
		y = 1
	}
	if _, err := fmt.Fprintf(w, "\033[%d;%dH%s", y, x, t.Value); err != nil {
		return err
	}
	return nil
}

// Update is a no-op for static text.
func (t Text) Update(ctx UpdateContext) (bool, error) {
	return false, nil
}
