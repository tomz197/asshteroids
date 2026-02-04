// Package draw provides terminal drawing primitives and a scaled canvas.
package draw

// Point represents a 2D coordinate.
type Point struct {
	X, Y float64
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

// Shades are characters from lightest to darkest.
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

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
