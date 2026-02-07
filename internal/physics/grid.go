package physics

import "math"

// SpatialGrid is a uniform grid for broad-phase collision detection in a wrapping world.
// Objects are inserted by position and index, then nearby objects can be queried
// in O(1) per cell via a 3x3 neighborhood lookup.
//
// Cell size must be >= the maximum interaction distance between any two
// colliding objects so that all potential collisions are found within
// the 3x3 neighborhood.
type SpatialGrid struct {
	cellSize    float64
	invCellSize float64 // 1 / cellSize (precomputed to avoid division)
	cols        int
	rows        int
	cells       []gridCell
}

// gridCell stores the indices of objects that fall within a grid cell.
// The slice is reused between frames (reset to [:0]) to avoid allocations.
type gridCell struct {
	items []int
}

// NewSpatialGrid creates a spatial grid covering the given world dimensions.
// cellSize should be >= the maximum collision distance for the objects being inserted.
func NewSpatialGrid(worldW, worldH, cellSize float64) *SpatialGrid {
	cols := int(math.Ceil(worldW / cellSize))
	rows := int(math.Ceil(worldH / cellSize))
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}

	cells := make([]gridCell, cols*rows)
	return &SpatialGrid{
		cellSize:    cellSize,
		invCellSize: 1.0 / cellSize,
		cols:        cols,
		rows:        rows,
		cells:       cells,
	}
}

// Clear removes all items from the grid without deallocating cell memory.
func (g *SpatialGrid) Clear() {
	for i := range g.cells {
		g.cells[i].items = g.cells[i].items[:0]
	}
}

// Insert adds an item (identified by index) at the given world position.
func (g *SpatialGrid) Insert(x, y float64, index int) {
	col, row := g.posToCell(x, y)
	idx := row*g.cols + col
	g.cells[idx].items = append(g.cells[idx].items, index)
}

// QueryAround calls fn for each item index in the 3x3 cell neighborhood
// around the given world position. Handles wrapping at world edges.
// If fn returns true, iteration stops early (useful for "find first" queries).
func (g *SpatialGrid) QueryAround(x, y float64, fn func(index int) bool) {
	col, row := g.posToCell(x, y)

	for dr := -1; dr <= 1; dr++ {
		r := row + dr
		if r < 0 {
			r += g.rows
		} else if r >= g.rows {
			r -= g.rows
		}

		rowOffset := r * g.cols

		for dc := -1; dc <= 1; dc++ {
			c := col + dc
			if c < 0 {
				c += g.cols
			} else if c >= g.cols {
				c -= g.cols
			}

			for _, itemIdx := range g.cells[rowOffset+c].items {
				if fn(itemIdx) {
					return
				}
			}
		}
	}
}

// posToCell converts world coordinates to grid cell coordinates.
// Clamps to valid range to handle edge cases with floating point.
func (g *SpatialGrid) posToCell(x, y float64) (col, row int) {
	col = int(x * g.invCellSize)
	if col < 0 {
		col = 0
	} else if col >= g.cols {
		col = g.cols - 1
	}

	row = int(y * g.invCellSize)
	if row < 0 {
		row = 0
	} else if row >= g.rows {
		row = g.rows - 1
	}

	return col, row
}
