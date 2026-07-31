package geometry

import (
	"math"

	"github.com/goexlib/mathx"
)

// Transform is a 2D affine transform stored as a row-major 3×2 matrix.
//
//	| A11 A12 TX |
//	| A21 A22 TY |
type Transform struct {
	A11, A12 float32 // row 1
	A21, A22 float32 // row 2
	TX, TY   float32 // translation
}

// Identity returns the identity transform.
func Identity() Transform {
	return Transform{A11: 1, A22: 1}
}

// Translate returns a translation transform.
func Translate(dx, dy float32) Transform {
	return Transform{A11: 1, A22: 1, TX: dx, TY: dy}
}

// Scale returns a scale transform.
func Scale(sx, sy float32) Transform {
	return Transform{A11: sx, A22: sy}
}

// Rotate returns a rotation transform (degrees, positive = clockwise in Y-down coords).
func Rotate(degrees float32) Transform {
	rad := degrees * math.Pi / 180
	c := mathx.Cos(rad)
	s := mathx.Sin(rad)
	return Transform{A11: c, A12: -s, A21: s, A22: c}
}

// Multiply returns t * o (apply o first, then t).
func (t Transform) Multiply(o Transform) Transform {
	return Transform{
		A11: t.A11*o.A11 + t.A12*o.A21,
		A12: t.A11*o.A12 + t.A12*o.A22,
		A21: t.A21*o.A11 + t.A22*o.A21,
		A22: t.A21*o.A12 + t.A22*o.A22,
		TX:  t.A11*o.TX + t.A12*o.TY + t.TX,
		TY:  t.A21*o.TX + t.A22*o.TY + t.TY,
	}
}

// Translate returns a new transform with translation applied in local space after t.
func (t Transform) Translate(dx, dy float32) Transform {
	return t.Multiply(Translate(dx, dy))
}

// Scale returns a new transform with scaling applied in local space after t.
func (t Transform) Scale(sx, sy float32) Transform {
	return t.Multiply(Scale(sx, sy))
}

// Rotate returns a new transform with rotation applied in local space after t.
func (t Transform) Rotate(degrees float32) Transform {
	return t.Multiply(Rotate(degrees))
}

// TransformPoint applies the transform to a point.
func (t Transform) TransformPoint(p Point) Point {
	return Point{
		X: t.A11*p.X + t.A12*p.Y + t.TX,
		Y: t.A21*p.X + t.A22*p.Y + t.TY,
	}
}

// Inverse returns the inverse of t. If t is non-invertible (determinant ≈ 0),
// it returns Identity.
func (t Transform) Inverse() Transform {
	det := t.A11*t.A22 - t.A12*t.A21
	if det > -1e-12 && det < 1e-12 {
		return Identity()
	}
	invDet := 1.0 / det
	return Transform{
		A11: t.A22 * invDet,
		A12: -t.A12 * invDet,
		A21: -t.A21 * invDet,
		A22: t.A11 * invDet,
		TX:  (t.A12*t.TY - t.A22*t.TX) * invDet,
		TY:  (t.A21*t.TX - t.A11*t.TY) * invDet,
	}
}
