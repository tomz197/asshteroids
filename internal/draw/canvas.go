package draw

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

// ANSI color codes for terminal output.
const (
	ColorReset = "\033[0m"

	// Standard foreground colors (30–37)
	ColorBlack   = "\033[30m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorWhite   = "\033[37m"

	// Bright foreground colors (90–97)
	ColorBrightBlack   = "\033[90m"
	ColorBrightRed     = "\033[91m"
	ColorBrightGreen   = "\033[92m"
	ColorBrightYellow  = "\033[93m"
	ColorBrightBlue    = "\033[94m"
	ColorBrightMagenta = "\033[95m"
	ColorBrightCyan    = "\033[96m"
	ColorBrightWhite   = "\033[97m"

	// Semantic colors for UI elements
	ColorDim = "\033[2m" // Dimmed text
)

// cellState represents the visual state of a terminal cell for double-buffering.
type cellState byte

const (
	cellEmpty cellState = iota // ' '
	cellUpper                  // '▀'
	cellLower                  // '▄'
	cellFull                   // '█'
)

// prevCells packing: low 2 bits = cell state, bit 2 = dirty from MarkTextDirty.
const (
	cellStateMask = 0x03
	cellDirtyBit  = 0x04
)

// Canvas is a drawing buffer with 2x vertical resolution using half-block characters.
// Supports scaling from logical coordinates to actual terminal pixels.
// Uses double-buffering to only write cells that changed between frames,
// eliminating the need for full-screen clearing and reducing SSH bandwidth.
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

	// Offset for centering the render area when terminal is larger than max resolution.
	// These are 0-based terminal offsets (columns/rows to skip).
	offsetCol int
	offsetRow int

	// Double-buffering: track previous frame's cell states to render only diffs.
	// prevCells packs state (low 2 bits) and dirty flag (bit 2) in one byte per cell.
	prevCells   []byte // Packed: cellStateMask = state, cellDirtyBit = externally dirtied
	forceRedraw bool   // Force all cells to be re-rendered next frame

	// Reusable buffers to reduce allocations
	renderBuf       strings.Builder // Buffer for batching render output
	numBuf          [20]byte        // Scratch buffer for integer-to-string conversion
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
	totalCells := termWidth * termHeight
	return &Canvas{
		termWidth:      termWidth,
		termHeight:     termHeight,
		subPixelHeight: subPixelHeight,
		pixels:         make([]bool, subPixelHeight*termWidth),
		logicalWidth:   logicalWidth,
		logicalHeight:  logicalHeight,
		scaleX:         float64(termWidth) / logicalWidth,
		scaleY:         float64(subPixelHeight) / logicalHeight,
		prevCells:      make([]byte, totalCells),
		forceRedraw:    true, // First frame must render everything
	}
}

// Resize updates the canvas for new terminal dimensions while keeping logical size.
// Forces a full redraw on the next Render call when the size actually changes.
func (c *Canvas) Resize(termWidth, termHeight int) {
	subPixelHeight := termHeight * 2

	// Reallocate if size changed
	if termWidth != c.termWidth || termHeight != c.termHeight {
		totalCells := termWidth * termHeight
		c.pixels = make([]bool, subPixelHeight*termWidth)
		c.prevCells = make([]byte, totalCells)
		c.forceRedraw = true
		c.termWidth = termWidth
		c.termHeight = termHeight
		c.subPixelHeight = subPixelHeight
	}

	// Update scale factors
	c.scaleX = float64(termWidth) / c.logicalWidth
	c.scaleY = float64(subPixelHeight) / c.logicalHeight
}

// SetOffset sets the column and row offset for centering the canvas.
// Offsets are 0-based terminal positions: the canvas starts at (offsetCol+1, offsetRow+1).
func (c *Canvas) SetOffset(col, row int) {
	c.offsetCol = col
	c.offsetRow = row
}

// OffsetCol returns the column offset used for centering.
func (c *Canvas) OffsetCol() int {
	return c.offsetCol
}

// OffsetRow returns the row offset used for centering.
func (c *Canvas) OffsetRow() int {
	return c.offsetRow
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

// Render outputs the canvas to the chunk writer using half-block characters.
// Uses double-buffering: only cells that changed since the previous frame
// (or were externally dirtied via MarkTextDirty) are written. Empty cells
// that were previously filled are overwritten with spaces, eliminating
// the need for full-screen clearing and reducing SSH bandwidth.
func (c *Canvas) Render(cw *ChunkWriter) {
	c.renderBuf.Reset()
	minCap := c.termWidth * c.termHeight * 4
	if c.renderBuf.Cap() < minCap {
		c.renderBuf.Grow(minCap - c.renderBuf.Cap())
	}

	force := c.forceRedraw
	c.forceRedraw = false

	for row := 0; row < c.termHeight; row++ {
		topY := row * 2
		bottomY := row*2 + 1
		topOffset := topY * c.termWidth
		bottomOffset := bottomY * c.termWidth
		rowBase := row * c.termWidth

		for col := 0; col < c.termWidth; col++ {
			top := c.pixels[topOffset+col]
			bottom := bottomY < c.subPixelHeight && c.pixels[bottomOffset+col]

			var current cellState
			switch {
			case top && bottom:
				current = cellFull
			case top:
				current = cellUpper
			case bottom:
				current = cellLower
			default:
				current = cellEmpty
			}

			cellIdx := rowBase + col
			packed := c.prevCells[cellIdx]
			prev := cellState(packed & cellStateMask)
			dirty := packed&cellDirtyBit != 0
			c.prevCells[cellIdx] = byte(current)

			if !force && !dirty && current == prev {
				continue // No change, skip this cell
			}

			// Write ANSI cursor position + character
			c.renderBuf.WriteString("\033[")
			c.renderBuf.Write(strconv.AppendInt(c.numBuf[:0], int64(row+1+c.offsetRow), 10))
			c.renderBuf.WriteByte(';')
			c.renderBuf.Write(strconv.AppendInt(c.numBuf[:0], int64(col+1+c.offsetCol), 10))
			c.renderBuf.WriteByte('H')

			switch current {
			case cellFull:
				c.renderBuf.WriteRune(BlockFull)
			case cellUpper:
				c.renderBuf.WriteRune(BlockUpperHalf)
			case cellLower:
				c.renderBuf.WriteRune(BlockLowerHalf)
			case cellEmpty:
				c.renderBuf.WriteByte(' ')
			}
		}
	}

	// Clear dirty bits for next frame (cells we skipped retain state but not dirty)
	for i := range c.prevCells {
		c.prevCells[i] &= cellStateMask
	}

	cw.WriteString(c.renderBuf.String())
}

// RenderBorder draws a box border around the canvas area when the terminal
// exceeds the max render resolution on either axis.
// Draws horizontal borders when there is vertical offset, vertical borders
// when there is horizontal offset, and corners when both are present.
func (c *Canvas) RenderBorder(cw *ChunkWriter) {
	hasH := c.offsetCol >= 1 // Room for left/right vertical bars
	hasV := c.offsetRow >= 1 // Room for top/bottom horizontal bars

	// Border positions (1-based terminal coordinates)
	left := c.offsetCol
	right := c.offsetCol + c.termWidth + 1
	top := c.offsetRow
	bottom := c.offsetRow + c.termHeight + 1

	c.renderBuf.Reset()
	c.renderBuf.Grow((c.termWidth+2)*2 + c.termHeight*2*12) // Estimate buffer size

	hLine := strings.Repeat("─", c.termWidth)

	if hasV {
		// Top border
		if hasH {
			// Full top: ┌───┐
			c.writeCSI(&c.renderBuf, top, left)
			c.renderBuf.WriteString("┌")
			c.renderBuf.WriteString(hLine)
			c.renderBuf.WriteString("┐")
		} else {
			// Top without corners: ───
			c.writeCSI(&c.renderBuf, top, c.offsetCol+1)
			c.renderBuf.WriteString(hLine)
		}

		// Bottom border
		if hasH {
			// Full bottom: └───┘
			c.writeCSI(&c.renderBuf, bottom, left)
			c.renderBuf.WriteString("└")
			c.renderBuf.WriteString(hLine)
			c.renderBuf.WriteString("┘")
		} else {
			// Bottom without corners: ───
			c.writeCSI(&c.renderBuf, bottom, c.offsetCol+1)
			c.renderBuf.WriteString(hLine)
		}
	}

	if hasH {
		// Side borders: │ ... │
		startRow := top + 1
		endRow := bottom
		if !hasV {
			// No horizontal borders, side bars span full canvas height
			startRow = c.offsetRow + 1
			endRow = c.offsetRow + c.termHeight + 1
		}
		for row := startRow; row < endRow; row++ {
			c.writeCSI(&c.renderBuf, row, left)
			c.renderBuf.WriteString("│")
			c.writeCSI(&c.renderBuf, row, right)
			c.renderBuf.WriteString("│")
		}
	}

	cw.WriteString(c.renderBuf.String())
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

// ForceRedraw marks the canvas so the next Render call writes every cell,
// regardless of whether it changed. Use after a full terminal clear or resize.
func (c *Canvas) ForceRedraw() {
	c.forceRedraw = true
}

// MarkTextDirty marks terminal cells as externally modified (e.g. by UI text overlays).
// col and row are 1-based coordinates within the canvas area (matching moveCursor).
// width is the number of characters that were drawn.
// The next Render call will overwrite these cells with the correct canvas content,
// effectively cleaning up text drawn on top of the canvas.
func (c *Canvas) MarkTextDirty(col, row, width int) {
	r := row - 1 // Convert 1-based to 0-based
	c0 := col - 1
	if r < 0 || r >= c.termHeight {
		return
	}
	base := r * c.termWidth
	for i := 0; i < width; i++ {
		ci := c0 + i
		if ci >= 0 && ci < c.termWidth {
			c.prevCells[base+ci] |= cellDirtyBit
		}
	}
}

// writeCSI writes an ANSI CSI cursor position sequence (\033[row;colH) to the builder.
// Uses the canvas numBuf to avoid allocations.
func (c *Canvas) writeCSI(buf *strings.Builder, row, col int) {
	buf.WriteString("\033[")
	buf.Write(strconv.AppendInt(c.numBuf[:0], int64(row), 10))
	buf.WriteByte(';')
	buf.Write(strconv.AppendInt(c.numBuf[:0], int64(col), 10))
	buf.WriteByte('H')
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
