package graphics

import (
	"math"

	"github.com/golang-gui/goui/core/geometry"
)

// RoundRectPath constructs a closed circular-corner outline in local logical
// coordinates. Empty or non-finite bounds produce an empty path.
func RoundRectPath(rect Rectangle, radius geometry.CornerRadius) Path {
	if !validRoundRect(rect) {
		return Path{}
	}
	radius = radius.Normalize(rect.Size)
	p := Path{data: make([]float32, 0, 48)}
	p.data = append(p.data, float32(PathMoveTo), rect.X+radius.TopLeft, rect.Y)
	return roundRectContour(p, rect, radius, false).Close()
}

// RoundRectBorderPath constructs the filled region between an outer rounded
// rectangle and its inset contour. It preserves destination pixels in the
// center, including when the border is translucent. A collapsed inner contour
// yields the entire outer shape. Width is measured inward from rect.
func RoundRectBorderPath(rect Rectangle, radius geometry.CornerRadius, width float32) Path {
	if !validRoundRect(rect) || !(width > 0) || math.IsInf(float64(width), 0) {
		return Path{}
	}
	radius = radius.Normalize(rect.Size)
	if width >= min(rect.Width, rect.Height)/2 {
		return RoundRectPath(rect, radius)
	}
	inner := rect.Inset(width)
	innerRadius := radius.Inset(width).Normalize(inner.Size)
	p := Path{data: make([]float32, 0, 96)}
	x, y := rect.X+radius.TopLeft, rect.Y
	p.data = append(p.data, float32(PathMoveTo), x, y)
	p = roundRectContour(p, rect, radius, false)
	p = p.Close()
	p.data = append(p.data, float32(PathMoveTo), inner.X+innerRadius.TopLeft, inner.Y, float32(PathHole), 0, 0)
	p = roundRectContour(p, inner, innerRadius, true)
	return p.Close()
}

func validRoundRect(r Rectangle) bool {
	return r.Width > 0 && r.Height > 0 &&
		!math.IsNaN(float64(r.X)) && !math.IsNaN(float64(r.Y)) &&
		!math.IsInf(float64(r.X), 0) && !math.IsInf(float64(r.Y), 0) &&
		!math.IsInf(float64(r.Width), 0) && !math.IsInf(float64(r.Height), 0)
}

func roundRectContour(p Path, rect Rectangle, r geometry.CornerRadius, reverse bool) Path {
	x, y, right, bottom := rect.X, rect.Y, rect.X+rect.Width, rect.Y+rect.Height
	const k float32 = 0.5522847498307936
	curve := func(radius, a, b, c, d, ex, ey float32) {
		if radius > 0 {
			p = p.BezierTo(a, b, c, d, ex, ey)
		} else {
			p = p.LineTo(ex, ey)
		}
	}
	if !reverse {
		p = p.LineTo(right-r.TopRight, y)
		curve(r.TopRight, right-r.TopRight*(1-k), y, right, y+r.TopRight*(1-k), right, y+r.TopRight)
		p = p.LineTo(right, bottom-r.BottomRight)
		curve(r.BottomRight, right, bottom-r.BottomRight*(1-k), right-r.BottomRight*(1-k), bottom, right-r.BottomRight, bottom)
		p = p.LineTo(x+r.BottomLeft, bottom)
		curve(r.BottomLeft, x+r.BottomLeft*(1-k), bottom, x, bottom-r.BottomLeft*(1-k), x, bottom-r.BottomLeft)
		p = p.LineTo(x, y+r.TopLeft)
		curve(r.TopLeft, x, y+r.TopLeft*(1-k), x+r.TopLeft*(1-k), y, x+r.TopLeft, y)
	} else {
		curve(r.TopLeft, x+r.TopLeft*(1-k), y, x, y+r.TopLeft*(1-k), x, y+r.TopLeft)
		p = p.LineTo(x, bottom-r.BottomLeft)
		curve(r.BottomLeft, x, bottom-r.BottomLeft*(1-k), x+r.BottomLeft*(1-k), bottom, x+r.BottomLeft, bottom)
		p = p.LineTo(right-r.BottomRight, bottom)
		curve(r.BottomRight, right-r.BottomRight*(1-k), bottom, right, bottom-r.BottomRight*(1-k), right, bottom-r.BottomRight)
		p = p.LineTo(right, y+r.TopRight)
		curve(r.TopRight, right, y+r.TopRight*(1-k), right-r.TopRight*(1-k), y, right-r.TopRight, y)
		p = p.LineTo(x+r.TopLeft, y)
	}
	return p
}
