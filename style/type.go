package style

import (
	"image/color"

	"github.com/golang-gui/goui/core/geometry"
)

// Shadow describes an outer box shadow. Offset, BlurRadius and SpreadRadius
// use local logical units (DIP). Positive spread expands the contour; negative
// spread contracts it. Non-positive blur produces a clear silhouette.
//
// A nil or fully transparent Color disables the shadow. A shadow is a style
// value, not a drawing command: its consumer owns normalization, painting and
// any space reserved outside the body. Ordinary widgets do not automatically
// paint shadows outside their bounds.
type Shadow struct {
	Color        color.Color
	Offset       geometry.Point
	BlurRadius   float32
	SpreadRadius float32
}
