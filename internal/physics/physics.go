// Package physics provides collision detection and distance utilities.
package physics

import "math"

// Distance calculates the Euclidean distance between two points.
func Distance(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	return math.Sqrt(dx*dx + dy*dy)
}

// DistanceSquared calculates the squared distance between two points.
// Use this when comparing distances to avoid the sqrt cost.
func DistanceSquared(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	return dx*dx + dy*dy
}

// PointInCircle checks if a point is within radius of a target position.
func PointInCircle(px, py, cx, cy, radius float64) bool {
	return DistanceSquared(px, py, cx, cy) <= radius*radius
}

// CirclesOverlap checks if two circles overlap.
func CirclesOverlap(x1, y1, r1, x2, y2, r2 float64) bool {
	minDist := r1 + r2
	return DistanceSquared(x1, y1, x2, y2) < minDist*minDist
}
