package geometry

import "math"

// CornerRadius holds circular radii in physical corner order. Values use the
// coordinate system of the rectangle; they are independent of text direction.
type CornerRadius struct {
	TopLeft     float32
	TopRight    float32
	BottomRight float32
	BottomLeft  float32
}

// UniformCornerRadius sets all four corners to radius.
func UniformCornerRadius(radius float32) CornerRadius {
	return CornerRadius{radius, radius, radius, radius}
}

// Uniform reports whether all corners have the same radius.
func (r CornerRadius) Uniform() bool {
	return r.TopLeft == r.TopRight && r.TopLeft == r.BottomRight && r.TopLeft == r.BottomLeft
}

// Max returns the largest nonnegative, finite radius, useful for conservative
// rectangular content insets. It does not approximate the outline for painting.
func (r CornerRadius) Max() float32 {
	return max(cornerValue(r.TopLeft), cornerValue(r.TopRight), cornerValue(r.BottomRight), cornerValue(r.BottomLeft))
}

// Normalize removes negative/non-finite values and scales all corners together
// until adjacent radii fit their edge. Empty/non-finite sizes yield zero radii.
func (r CornerRadius) Normalize(size Size) CornerRadius {
	if !(size.Width > 0 && size.Height > 0) || math.IsInf(float64(size.Width), 0) || math.IsInf(float64(size.Height), 0) {
		return CornerRadius{}
	}
	if r.Uniform() {
		return UniformCornerRadius(min(cornerValue(r.TopLeft), size.Width/2, size.Height/2))
	}
	r = CornerRadius{cornerValue(r.TopLeft), cornerValue(r.TopRight), cornerValue(r.BottomRight), cornerValue(r.BottomLeft)}
	scale := float64(1)
	limit := func(length, a, b float32) {
		if sum := float64(a) + float64(b); sum > 0 {
			scale = min(scale, float64(length)/sum)
		}
	}
	limit(size.Width, r.TopLeft, r.TopRight)
	limit(size.Width, r.BottomLeft, r.BottomRight)
	limit(size.Height, r.TopLeft, r.BottomLeft)
	limit(size.Height, r.TopRight, r.BottomRight)
	return CornerRadius{float32(float64(r.TopLeft) * scale), float32(float64(r.TopRight) * scale), float32(float64(r.BottomRight) * scale), float32(float64(r.BottomLeft) * scale)}
}

// Inset subtracts distance from each radius, clamping at zero. Normalize the
// result against the inset rectangle before constructing a contour.
func (r CornerRadius) Inset(distance float32) CornerRadius {
	return CornerRadius{max(0, r.TopLeft-distance), max(0, r.TopRight-distance), max(0, r.BottomRight-distance), max(0, r.BottomLeft-distance)}
}

// Contains reports whether point is inside the rounded rectangle. The bounds
// use half-open edges: left/top inclusive, right/bottom exclusive.
func (r CornerRadius) Contains(rect Rectangle, point Point) bool {
	if !(point.X >= rect.X && point.Y >= rect.Y && point.X < rect.X+rect.Width && point.Y < rect.Y+rect.Height) {
		return false
	}
	r = r.Normalize(rect.Size)
	x, y := point.X-rect.X, point.Y-rect.Y
	inCorner := func(cx, cy, radius float32) bool {
		dx, dy := float64(x-cx), float64(y-cy)
		return dx*dx+dy*dy <= float64(radius)*float64(radius)
	}
	if x < r.TopLeft && y < r.TopLeft && !inCorner(r.TopLeft, r.TopLeft, r.TopLeft) {
		return false
	}
	if x > rect.Width-r.TopRight && y < r.TopRight && !inCorner(rect.Width-r.TopRight, r.TopRight, r.TopRight) {
		return false
	}
	if x > rect.Width-r.BottomRight && y > rect.Height-r.BottomRight && !inCorner(rect.Width-r.BottomRight, rect.Height-r.BottomRight, r.BottomRight) {
		return false
	}
	if x < r.BottomLeft && y > rect.Height-r.BottomLeft && !inCorner(r.BottomLeft, rect.Height-r.BottomLeft, r.BottomLeft) {
		return false
	}
	return true
}

func cornerValue(v float32) float32 {
	if v <= 0 || math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
		return 0
	}
	return v
}
