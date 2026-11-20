// Package pixelsnap aligns axis-aligned UI primitives to the device pixel grid.
// It leaves arbitrary paths and user rotation/scale/shear transforms untouched.
package pixelsnap

import (
	"image"
	"math"

	"github.com/goexlib/mathx"
	"github.com/golang-gui/goui/core/geometry"
)

// Clip returns the conservative device-pixel envelope of a window-local DIP
// rectangle. It is independent of the drawing transform. Callers distinguish
// an active empty clip from the all-zero rectangle used to reset clipping.
// Round the final logical intersection only, never its ancestors separately.
func Clip(rect geometry.Rectangle, scale float32) image.Rectangle {
	if rect.Width <= 0 || rect.Height <= 0 || scale <= 0 {
		return image.Rectangle{}
	}
	for _, v := range []float32{rect.X, rect.Y, rect.Width, rect.Height, scale} {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return image.Rectangle{}
		}
	}
	// Keep the same float32 edge arithmetic as the public geometry model.
	// Saturation avoids integer overflow for unbounded but finite coordinates.
	edge := func(v float32, upper bool) int {
		x := min(float64(math.MaxInt32), max(-float64(math.MaxInt32), float64(v)))
		if upper {
			return int(math.Ceil(x))
		}
		return int(math.Floor(x))
	}
	left, top := edge(rect.X*scale, false), edge(rect.Y*scale, false)
	right, bottom := edge((rect.X+rect.Width)*scale, true), edge((rect.Y+rect.Height)*scale, true)
	// Positive subpixel extents can collapse during float32 addition, but
	// still intersect the pixel beginning at an exactly integral lower edge.
	if right == left && left < math.MaxInt32 {
		right++
	}
	if bottom == top && top < math.MaxInt32 {
		bottom++
	}
	return image.Rectangle{Min: image.Pt(left, top), Max: image.Pt(right, bottom)}
}

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
