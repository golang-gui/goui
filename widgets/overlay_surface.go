package widgets

import (
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

// Reserve the shadow's three-sigma support inside the ordinary Widget clip.
// Symmetric reservations keep the content surface centered in its allocation.
func surfaceInsets(s style.Style) geometry.Insets {
	shadow, ok := s.Shadow()
	if !ok || shadow.Color == nil {
		return geometry.Insets{}
	}
	_, _, _, alpha := shadow.Color.RGBA()
	if alpha == 0 {
		return geometry.Insets{}
	}
	for _, v := range []float32{shadow.BlurRadius, shadow.SpreadRadius, shadow.Offset.X, shadow.Offset.Y} {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return geometry.Insets{}
		}
	}
	blur := float32(0)
	if shadow.BlurRadius > 0 {
		blur = max(.5, shadow.BlurRadius)
	}
	extent := 1.5*blur + shadow.SpreadRadius
	if math.IsInf(float64(extent), 0) {
		return geometry.Insets{}
	}
	pad := func(offset float32) float32 {
		return float32(math.Ceil(float64(max(0, extent+float32(math.Abs(float64(offset)))) + 1)))
	}
	x, y := pad(shadow.Offset.X), pad(shadow.Offset.Y)
	if math.IsInf(float64(x), 0) || math.IsInf(float64(y), 0) {
		return geometry.Insets{}
	}
	return geometry.Insets{Left: x, Right: x, Top: y, Bottom: y}
}

func paintSurface(p gui.Painter, rect geometry.Rectangle, s style.Style) {
	if shadow, ok := s.Shadow(); ok && shadow.Color != nil {
		radius, _ := s.Radius()
		p.DrawBoxShadow(rect, radius, graphics.BoxShadow{Color: graphics.ColorOf(shadow.Color),
			Offset: shadow.Offset, BlurRadius: shadow.BlurRadius, SpreadRadius: shadow.SpreadRadius})
	}
	gui.PaintStyledBox(p, rect, s)
}

func localPoint(widget gui.Widget, point geometry.Point) geometry.Point {
	for current := widget; current != nil; current = current.Parent() {
		point.X -= current.Rect().X
		point.Y -= current.Rect().Y
	}
	return point
}

func pointInRect(r geometry.Rectangle, p geometry.Point) bool {
	return p.X >= r.X && p.X < r.X+r.Width && p.Y >= r.Y && p.Y < r.Y+r.Height
}
