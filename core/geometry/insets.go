package geometry

// Insets gives the distance inward from each edge of a rectangle. Values use
// the same coordinate system as the rectangle they apply to.
type Insets struct {
	Left   float32
	Top    float32
	Right  float32
	Bottom float32
}
