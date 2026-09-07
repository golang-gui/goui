// Package pixelsnap aligns axis-aligned UI primitives to the device pixel grid.
// It leaves arbitrary paths and user rotation/scale/shear transforms untouched.
package pixelsnap

import (
	"github.com/goexlib/mathx"
	"github.com/golang-gui/goui/core/geometry"
)

func canSnap(t geometry.Transform, scale float32) bool {
	return scale > 0 && t.A11 == 1 && t.A22 == 1 && t.A12 == 0 && t.A21 == 0
}

// Edge rounds a device-space edge and maps it back to local coordinates. Layout
// and font metrics can give widgets fractional DIP translations, so rounding
// only the local coordinate is insufficient.
func Edge(x, offset, scale float32) float32 {
	if scale <= 0 {
		return x
	}
	return mathx.Floor((x+offset)*scale+0.5)/scale - offset
}

func Rect(rect geometry.Rectangle, t geometry.Transform, scale float32) geometry.Rectangle {
	if !canSnap(t, scale) {
		return rect
	}
	left := Edge(rect.X, t.TX, scale)
	top := Edge(rect.Y, t.TY, scale)
	right := Edge(rect.X+rect.Width, t.TX, scale)
	bottom := Edge(rect.Y+rect.Height, t.TY, scale)
	return geometry.Rect(left, top, right-left, bottom-top)
}

func StrokeRect(rect geometry.Rectangle, width float32, t geometry.Transform, scale float32) geometry.Rectangle {
	if !canSnap(t, scale) || width <= 0 {
		return rect
	}
	// Both Direct2D and NanoVG center strokes on their path. Snap the outer
	// silhouette to the same edges as the fill/clip, then restore the centerline.
	// Odd physical widths need half-pixel centers; even widths need integers.
	// Preserve fractional widths: partial coverage of the inner edge is correct.
	outer := Rect(rect.Inset(-width/2), t, scale)
	if outer.Width < width || outer.Height < width {
		return rect
	}
	return outer.Inset(width / 2)
}

func Line(p0, p1 geometry.Point, width float32, t geometry.Transform, scale float32) (geometry.Point, geometry.Point) {
	if !canSnap(t, scale) || width <= 0 {
		return p0, p1
	}
	half := width / 2
	if p0.X == p1.X {
		p0.X = Edge(p0.X-half, t.TX, scale) + half
		p1.X = p0.X
	} else if p0.Y == p1.Y {
		p0.Y = Edge(p0.Y-half, t.TY, scale) + half
		p1.Y = p0.Y
	}
	return p0, p1
}
