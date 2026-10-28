package geometry

// Insets gives the distance inward from each edge of a rectangle. Values use
// the same coordinate system as the rectangle they apply to. Left and Right are
// physical horizontal edges, not writing-direction start and end.
type Insets struct {
	Left   float32
	Top    float32
	Right  float32
	Bottom float32
}

// UniformInsets returns insets with the same value on every edge.
func UniformInsets(n float32) Insets {
	return Insets{Left: n, Top: n, Right: n, Bottom: n}
}

// Horizontal returns the total inset along the horizontal axis.
func (i Insets) Horizontal() float32 { return i.Left + i.Right }

// Vertical returns the total inset along the vertical axis.
func (i Insets) Vertical() float32 { return i.Top + i.Bottom }
