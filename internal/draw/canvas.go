package draw

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
)

// Canvas is a drawing buffer with 2x vertical resolution using half-block characters.
// Supports scaling from logical coordinates to actual terminal pixels.
type Canvas struct {
	termWidth      int    // Actual terminal columns
	termHeight     int    // Actual terminal rows
	subPixelHeight int    // termHeight * 2
	pixels         []bool // Flat slice: [y * termWidth + x] - true if pixel is set

	// Scaling from logical to pixel coordinates
	logicalWidth  float64 // Target/logical width
	logicalHeight float64 // Target/logical height (in sub-pixels)
	scaleX        float64 // termWidth / logicalWidth
	scaleY        float64 // (termHeight*2) / logicalHeight

	// Reusable buffers to reduce allocations
	renderBuf       strings.Builder // Buffer for batching render output
	scaledBuf       []Point         // Reusable buffer for fillPolygon scaled points
	intersectionBuf []float64       // Reusable buffer for scanline intersections
	polygonBuf      []Point         // Reusable buffer for polygon point generation
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
	return &Canvas{
		termWidth:      termWidth,
		termHeight:     termHeight,
		subPixelHeight: subPixelHeight,
		pixels:         make([]bool, subPixelHeight*termWidth),
		logicalWidth:   logicalWidth,
		logicalHeight:  logicalHeight,
		scaleX:         float64(termWidth) / logicalWidth,
		scaleY:         float64(subPixelHeight) / logicalHeight,
	}
}

// Resize updates the canvas for new terminal dimensions while keeping logical size.
func (c *Canvas) Resize(termWidth, termHeight int) {
	subPixelHeight := termHeight * 2

	// Reallocate if size changed
	if termWidth != c.termWidth || termHeight != c.termHeight {
		c.pixels = make([]bool, subPixelHeight*termWidth)
		c.termWidth = termWidth
		c.termHeight = termHeight
		c.subPixelHeight = subPixelHeight
	}

	// Update scale factors
	c.scaleX = float64(termWidth) / c.logicalWidth
	c.scaleY = float64(subPixelHeight) / c.logicalHeight
}

// Clear resets all pixels in the canvas.
func (c *Canvas) Clear() {
	clear(c.pixels)
}

// setPixel sets a pixel at actual terminal coordinates (no scaling).
func (c *Canvas) setPixel(x, y int) {
	if x >= 0 && x < c.termWidth && y >= 0 && y < c.subPixelHeight {
		c.pixels[y*c.termWidth+x] = true
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
	// Reuse or grow scaled points buffer
	if cap(c.scaledBuf) < len(points) {
		c.scaledBuf = make([]Point, len(points))
	}
	scaled := c.scaledBuf[:len(points)]

	// Scale points to pixel coordinates
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

		// Reuse intersection buffer
		intersections := c.intersectionBuf[:0]

		// Find intersections with all edges
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

		// Store back in case it grew
		c.intersectionBuf = intersections

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

// maxChunkSize is the maximum bytes to write at once for optimal network flow.
// 1500 bytes matches typical MTU size for smooth SSH/network transmission.
const maxChunkSize = 1400

// Render outputs the canvas to the writer using half-block characters.
func (c *Canvas) Render(w io.Writer) {
	// Reset and pre-grow buffer for better performance
	c.renderBuf.Reset()
	c.renderBuf.Grow(c.termWidth * c.termHeight * 12) // Estimate ~12 bytes per cell

	for row := 0; row < c.termHeight; row++ {
		topY := row * 2
		bottomY := row*2 + 1
		topOffset := topY * c.termWidth
		bottomOffset := bottomY * c.termWidth

		for col := 0; col < c.termWidth; col++ {
			top := c.pixels[topOffset+col]
			bottom := bottomY < c.subPixelHeight && c.pixels[bottomOffset+col]

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

			fmt.Fprintf(&c.renderBuf, "\033[%d;%dH%c", row+1, col+1, ch)
		}
	}

	// Write output in chunks for optimal network flow
	data := c.renderBuf.String()
	for len(data) > 0 {
		chunk := data
		if len(chunk) > maxChunkSize {
			chunk = data[:maxChunkSize]
		}
		io.WriteString(w, chunk)
		data = data[len(chunk):]
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

// LogicalToTerminal converts logical coordinates to 1-based terminal position (col, row).
// This is useful for placing text overlays at positions matching canvas-drawn objects.
func (c *Canvas) LogicalToTerminal(x, y float64) (col, row int) {
	px := int(math.Round(x * c.scaleX))
	py := int(math.Round(y * c.scaleY))
	return px + 1, py/2 + 1
}

// BorrowPoints returns a reusable slice of Points with the given length.
// The returned slice is only valid until the next call to BorrowPoints.
// This avoids per-frame allocations for polygon rendering.
// Thread-safe as long as each goroutine uses its own Canvas instance.
func (c *Canvas) BorrowPoints(n int) []Point {
	if cap(c.polygonBuf) < n {
		c.polygonBuf = make([]Point, n)
	}
	return c.polygonBuf[:n]
}
