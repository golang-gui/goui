package utils

import (
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
)

// RoundRectPath constructs a closed circular-corner outline in local logical
// coordinates. Empty or non-finite bounds produce an empty path.
func RoundRectPath(rect graphics.Rectangle, radius geometry.CornerRadius) graphics.Path {
	if !validRoundRect(rect) {
		return graphics.Path{}
	}
	radius = radius.Normalize(rect.Size)
	p := graphics.Path{}.Grow(48).MoveTo(rect.X+radius.TopLeft, rect.Y)
	return roundRectContour(p, rect, radius, false).Close()
}

// RoundRectBorderPath constructs the filled region between an outer rounded
// rectangle and its inset contour. It preserves destination pixels in the
// center, including when the border is translucent. A collapsed inner contour
// yields the entire outer shape. Width is measured inward from rect.
func RoundRectBorderPath(rect graphics.Rectangle, radius geometry.CornerRadius, width float32) graphics.Path {
	if !validRoundRect(rect) || !(width > 0) || math.IsInf(float64(width), 0) {
		return graphics.Path{}
	}
	radius = radius.Normalize(rect.Size)
	if width >= min(rect.Width, rect.Height)/2 {
		return RoundRectPath(rect, radius)
	}
	inner := rect.Inset(width)
	innerRadius := radius.Inset(width).Normalize(inner.Size)
	p := graphics.Path{}.Grow(96)
	x, y := rect.X+radius.TopLeft, rect.Y
	p = p.MoveTo(x, y)
	p = roundRectContour(p, rect, radius, false)
	p = p.Close().MoveTo(inner.X+innerRadius.TopLeft, inner.Y).Hole()
	p = roundRectContour(p, inner, innerRadius, true)
	return p.Close()
}

func validRoundRect(r graphics.Rectangle) bool {
	return r.Width > 0 && r.Height > 0 &&
		!math.IsNaN(float64(r.X)) && !math.IsNaN(float64(r.Y)) &&
		!math.IsInf(float64(r.X), 0) && !math.IsInf(float64(r.Y), 0) &&
		!math.IsInf(float64(r.Width), 0) && !math.IsInf(float64(r.Height), 0)
}

func roundRectContour(p graphics.Path, rect graphics.Rectangle, r geometry.CornerRadius, reverse bool) graphics.Path {
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
