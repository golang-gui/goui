package graphics

// BoxShadow describes an outer shadow for a rectangle or rounded rectangle.
// All values use the same local logical coordinate space as the rectangle.
// Draw the shadow before the source shape when the source should cover the
// portion of the softened mask that falls inside it.
type BoxShadow struct {
	Color        Color
	Offset       Point
	BlurRadius   float32
	SpreadRadius float32
}
