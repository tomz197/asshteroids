package draw

import (
	"fmt"
	"io"
	"math"
	"sort"
)

// Canvas is a drawing buffer with 2x vertical resolution using half-block characters.
// Supports scaling from logical coordinates to actual terminal pixels.
type Canvas struct {
	termWidth  int      // Actual terminal columns
	termHeight int      // Actual terminal rows
	pixels     [][]bool // [subpixel_y][x] - true if pixel is set

	// Scaling from logical to pixel coordinates
	logicalWidth  float64 // Target/logical width
	logicalHeight float64 // Target/logical height (in sub-pixels)
	scaleX        float64 // termWidth / logicalWidth
	scaleY        float64 // (termHeight*2) / logicalHeight
}

// NewCanvas creates a canvas for the given terminal dimensions.
// The canvas has 2x vertical resolution (height*2 sub-pixels).
// No scaling is applied (1:1 mapping).
func NewCanvas(width, height int) *Canvas {
	return NewScaledCanvas(width, height, float64(width), float64(height*2))
}

// NewScaledCanvas creates a canvas that scales from logical coordinates to terminal pixels.
// logicalWidth/Height define the coordinate space used by game objects.
// termWidth/Height are the actual terminal dimensions.
func NewScaledCanvas(termWidth, termHeight int, logicalWidth, logicalHeight float64) *Canvas {
	subPixelHeight := termHeight * 2
	pixels := make([][]bool, subPixelHeight)
	for i := range pixels {
		pixels[i] = make([]bool, termWidth)
	}
	return &Canvas{
		termWidth:     termWidth,
		termHeight:    termHeight,
		pixels:        pixels,
		logicalWidth:  logicalWidth,
		logicalHeight: logicalHeight,
		scaleX:        float64(termWidth) / logicalWidth,
		scaleY:        float64(subPixelHeight) / logicalHeight,
	}
}

// Resize updates the canvas for new terminal dimensions while keeping logical size.
func (c *Canvas) Resize(termWidth, termHeight int) {
	subPixelHeight := termHeight * 2

	// Reallocate if size changed
	if termWidth != c.termWidth || termHeight != c.termHeight {
		c.pixels = make([][]bool, subPixelHeight)
		for i := range c.pixels {
			c.pixels[i] = make([]bool, termWidth)
		}
		c.termWidth = termWidth
		c.termHeight = termHeight
	}

	// Update scale factors
	c.scaleX = float64(termWidth) / c.logicalWidth
	c.scaleY = float64(subPixelHeight) / c.logicalHeight
}

// Clear resets all pixels in the canvas.
func (c *Canvas) Clear() {
	for y := range c.pixels {
		for x := range c.pixels[y] {
			c.pixels[y][x] = false
		}
	}
}

// setPixel sets a pixel at actual terminal coordinates (no scaling).
func (c *Canvas) setPixel(x, y int) {
	if x >= 0 && x < c.termWidth && y >= 0 && y < len(c.pixels) {
		c.pixels[y][x] = true
	}
}

// Set sets a pixel at logical coordinates (applies scaling).
func (c *Canvas) Set(x, y int) {
	px := int(math.Round(float64(x) * c.scaleX))
	py := int(math.Round(float64(y) * c.scaleY))
	c.setPixel(px, py)
}

// SetFloat sets a pixel using float logical coordinates (applies scaling).
func (c *Canvas) SetFloat(x, y float64) {
	px := int(math.Round(x * c.scaleX))
	py := int(math.Round(y * c.scaleY))
	c.setPixel(px, py)
}

// DrawLine draws a line on the canvas using Bresenham's algorithm.
// Coordinates are in logical space and get scaled to pixels.
func (c *Canvas) DrawLine(p1, p2 Point) {
	// Scale to pixel coordinates for drawing
	x1 := int(math.Round(p1.X * c.scaleX))
	y1 := int(math.Round(p1.Y * c.scaleY))
	x2 := int(math.Round(p2.X * c.scaleX))
	y2 := int(math.Round(p2.Y * c.scaleY))

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
		c.setPixel(x1, y1)

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

// DrawPolygon draws a polygon on the canvas.
// If filled is true, the interior is filled using scanline algorithm.
func (c *Canvas) DrawPolygon(points []Point, filled bool) {
	if len(points) < 3 {
		return
	}

	if filled {
		c.fillPolygon(points)
	}

	// Draw outline
	n := len(points)
	for i := 0; i < n; i++ {
		c.DrawLine(points[i], points[(i+1)%n])
	}
}

// fillPolygon fills a polygon using scanline algorithm.
// Works in pixel space for proper scaling.
func (c *Canvas) fillPolygon(points []Point) {
	// Scale points to pixel coordinates
	scaled := make([]Point, len(points))
	for i, p := range points {
		scaled[i] = Point{
			X: p.X * c.scaleX,
			Y: p.Y * c.scaleY,
		}
	}

	// Find bounding box in pixel space
	minY, maxY := scaled[0].Y, scaled[0].Y
	for _, p := range scaled {
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}

	yStart := int(math.Floor(minY))
	yEnd := int(math.Ceil(maxY))

	// Scanline fill in pixel space
	for y := yStart; y <= yEnd; y++ {
		scanY := float64(y) + 0.5

		// Find intersections with all edges
		var intersections []float64
		n := len(scaled)
		for i := 0; i < n; i++ {
			p1 := scaled[i]
			p2 := scaled[(i+1)%n]

			if (p1.Y <= scanY && p2.Y > scanY) || (p2.Y <= scanY && p1.Y > scanY) {
				t := (scanY - p1.Y) / (p2.Y - p1.Y)
				x := p1.X + t*(p2.X-p1.X)
				intersections = append(intersections, x)
			}
		}

		sort.Float64s(intersections)

		for i := 0; i+1 < len(intersections); i += 2 {
			xStart := int(math.Ceil(intersections[i]))
			xEnd := int(math.Floor(intersections[i+1]))
			for x := xStart; x <= xEnd; x++ {
				c.setPixel(x, y)
			}
		}
	}
}

// Render outputs the canvas to the writer using half-block characters.
func (c *Canvas) Render(w io.Writer) {
	for row := 0; row < c.termHeight; row++ {
		topY := row * 2
		bottomY := row*2 + 1

		for col := 0; col < c.termWidth; col++ {
			top := c.pixels[topY][col]
			bottom := bottomY < len(c.pixels) && c.pixels[bottomY][col]

			var ch rune
			switch {
			case top && bottom:
				ch = BlockFull
			case top:
				ch = BlockUpperHalf
			case bottom:
				ch = BlockLowerHalf
			default:
				continue // Skip empty cells
			}

			fmt.Fprintf(w, "\033[%d;%dH%c", row+1, col+1, ch)
		}
	}
}

// LogicalWidth returns the logical width (target resolution).
func (c *Canvas) LogicalWidth() float64 {
	return c.logicalWidth
}

// LogicalHeight returns the logical height (target resolution, in sub-pixels).
func (c *Canvas) LogicalHeight() float64 {
	return c.logicalHeight
}

// TerminalWidth returns the actual terminal column count.
func (c *Canvas) TerminalWidth() int {
	return c.termWidth
}

// TerminalHeight returns the actual terminal row count.
func (c *Canvas) TerminalHeight() int {
	return c.termHeight
}
