package draw

import (
	"fmt"
	"io"
	"math"
	"os"
	"sort"

	"golang.org/x/term"
)

// Point represents a 2D coordinate.
type Point struct {
	X, Y float64
}

// Shade characters from lightest to darkest.
// Use these to render different intensities in the terminal.
var Shades = []rune{' ', '░', '▒', '▓', '█'}

// ShadeLevel returns a shade character for a value between 0.0 (empty) and 1.0 (solid).
func ShadeLevel(intensity float64) rune {
	if intensity <= 0 {
		return Shades[0]
	}
	if intensity >= 1 {
		return Shades[len(Shades)-1]
	}
	idx := int(intensity * float64(len(Shades)-1))
	return Shades[idx]
}

// Block characters for drawing.
const (
	BlockFull      = '█'
	BlockLight     = '░'
	BlockMedium    = '▒'
	BlockDark      = '▓'
	BlockEmpty     = ' '
	BlockUpperHalf = '▀'
	BlockLowerHalf = '▄'
	BlockLeftHalf  = '▌'
	BlockRightHalf = '▐'
)

// Screen holds terminal dimensions.
type Screen struct {
	Width   int
	Height  int
	CenterX int
	CenterY int
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

// TerminalSize returns the terminal width and height.
func TerminalSize() (Screen, error) {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return Screen{}, err
	}
	return Screen{Width: width, Height: height, CenterX: width / 2, CenterY: height / 2}, nil
}

// DrawChar draws a single character at the given position.
func DrawChar(w io.Writer, x, y int, ch rune) {
	if x >= 1 && y >= 1 {
		fmt.Fprintf(w, "\033[%d;%dH%c", y, x, ch)
	}
}

// DrawLine draws a line between two points using Bresenham's algorithm.
func DrawLine(w io.Writer, p1, p2 Point, ch rune) {
	x1, y1 := int(math.Round(p1.X)), int(math.Round(p1.Y))
	x2, y2 := int(math.Round(p2.X)), int(math.Round(p2.Y))

	dx := abs(x2 - x1)
	dy := abs(y2 - y1)

	sx := 1
	if x1 > x2 {
		sx = -1
	}
	sy := 1
	if y1 > y2 {
		sy = -1
	}

	err := dx - dy

	for {
		DrawChar(w, x1, y1, ch)

		if x1 == x2 && y1 == y2 {
			break
		}

		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x1 += sx
		}
		if e2 < dx {
			err += dx
			y1 += sy
		}
	}
}

// DrawPolygon draws a polygon from a slice of points.
// If filled is true, the polygon interior is filled; otherwise only the outline is drawn.
func DrawPolygon(w io.Writer, points []Point, ch rune, filled bool) {
	if len(points) < 3 {
		return
	}

	if filled {
		drawFilledPolygon(w, points, ch)
	}

	drawPolygonOutline(w, points, ch)
}

// drawPolygonOutline draws only the edges of the polygon.
func drawPolygonOutline(w io.Writer, points []Point, ch rune) {
	n := len(points)
	for i := 0; i < n; i++ {
		DrawLine(w, points[i], points[(i+1)%n], ch)
	}
}

// drawFilledPolygon fills a polygon using scanline algorithm.
func drawFilledPolygon(w io.Writer, points []Point, ch rune) {
	if len(points) < 3 {
		return
	}

	// Find bounding box
	minY, maxY := points[0].Y, points[0].Y
	for _, p := range points {
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}

	yStart := int(math.Floor(minY))
	yEnd := int(math.Ceil(maxY))

	// Scanline fill
	for y := yStart; y <= yEnd; y++ {
		scanY := float64(y) + 0.5 // Sample at pixel center

		// Find intersections with all edges
		var intersections []float64
		n := len(points)
		for i := 0; i < n; i++ {
			p1 := points[i]
			p2 := points[(i+1)%n]

			// Check if edge crosses this scanline
			if (p1.Y <= scanY && p2.Y > scanY) || (p2.Y <= scanY && p1.Y > scanY) {
				// Calculate x intersection
				t := (scanY - p1.Y) / (p2.Y - p1.Y)
				x := p1.X + t*(p2.X-p1.X)
				intersections = append(intersections, x)
			}
		}

		// Sort intersections
		sort.Float64s(intersections)

		// Fill between pairs of intersections
		for i := 0; i+1 < len(intersections); i += 2 {
			xStart := int(math.Ceil(intersections[i]))
			xEnd := int(math.Floor(intersections[i+1]))
			for x := xStart; x <= xEnd; x++ {
				DrawChar(w, x, y, ch)
			}
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
