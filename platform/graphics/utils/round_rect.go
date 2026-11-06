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

// RoundRectInsetPath constructs the rounded contour remaining after insetting
// each edge independently. Outer circular radii become elliptical inner radii.
// Insets must be finite and nonnegative, with all nonzero values equal. Invalid
// or collapsed bounds produce an empty path.
func RoundRectInsetPath(rect graphics.Rectangle, radius geometry.CornerRadius, insets geometry.Insets) graphics.Path {
	if !validRoundRect(rect) || !validRoundRectInsets(insets) {
		return graphics.Path{}
	}
	radius = radius.Normalize(rect.Size)
	inner, corners := insetRoundRect(rect, radius, insets)
	if !validRoundRect(inner) {
		return graphics.Path{}
	}
	p := graphics.Path{}.Grow(48).MoveTo(inner.X+corners[0].X, inner.Y)
	p = ellipseRectContour(p, inner, corners).Close()
	if crossesOppositeCorner(rect, radius, insets) {
		p = intersectRoundRect(p, rect, radius)
	}
	return p
}

// RoundRectBorderInsetsPath constructs the filled difference between the outer
// rounded rectangle and a contour inset independently on each edge. Zero inset
// omits an edge; adjoining rounded edges taper naturally to an omitted edge.
// Insets must be finite and nonnegative, with all nonzero values equal.
// All-zero insets produce an empty path;
// a collapsed inner contour yields the whole outer shape. Fill the path once
// so translucent joins do not accumulate alpha.
func RoundRectBorderInsetsPath(rect graphics.Rectangle, radius geometry.CornerRadius, insets geometry.Insets) graphics.Path {
	if !validRoundRect(rect) || !validRoundRectInsets(insets) || insets == (geometry.Insets{}) {
		return graphics.Path{}
	}
	if insets == geometry.UniformInsets(insets.Left) {
		return RoundRectBorderPath(rect, radius, insets.Left)
	}
	radius = radius.Normalize(rect.Size)
	inner, corners := insetRoundRect(rect, radius, insets)
	if !validRoundRect(inner) {
		return RoundRectPath(rect, radius)
	}
	// Uniform insets returned above, so at least one edge is omitted here.
	p := openRoundRectBorder(rect, radius, inner, corners, insets)
	if crossesOppositeCorner(rect, radius, insets) {
		p = intersectRoundRect(p, rect, radius)
	}
	return p
}

func crossesOppositeCorner(rect graphics.Rectangle, r geometry.CornerRadius, i geometry.Insets) bool {
	return i.Top > rect.Height-max(r.BottomLeft, r.BottomRight) ||
		i.Bottom > rect.Height-max(r.TopLeft, r.TopRight) ||
		i.Left > rect.Width-max(r.TopRight, r.BottomRight) ||
		i.Right > rect.Width-max(r.TopLeft, r.BottomLeft)
}

// A very thick selected edge can reach a corner beyond its own two arcs. Clip
// these uncommon contours to the convex outer silhouette. Ordinary borders
// retain cubic curves; the degenerate case uses an inscribed polygon with
// <=1/64 DIP curve flatness, without a bitmap, clip state or extra draw calls.
func intersectRoundRect(path graphics.Path, rect graphics.Rectangle, radius geometry.CornerRadius) graphics.Path {
	outer := roundRectPolygons(RoundRectPath(rect, radius))[0]
	result := graphics.Path{}
	cross := func(a, b, p geometry.Point) float64 {
		return float64(b.X-a.X)*float64(p.Y-a.Y) - float64(b.Y-a.Y)*float64(p.X-a.X)
	}
	for _, polygon := range roundRectPolygons(path) {
		var scratch []geometry.Point
		for i, a := range outer {
			if len(polygon) == 0 {
				break
			}
			b := outer[(i+1)%len(outer)]
			clipped := scratch[:0]
			previous := polygon[len(polygon)-1]
			before := cross(a, b, previous)
			for _, current := range polygon {
				after := cross(a, b, current)
				if (before >= 0) != (after >= 0) {
					t := float32(before / (before - after))
					clipped = append(clipped, geometry.Point{X: previous.X + (current.X-previous.X)*t, Y: previous.Y + (current.Y-previous.Y)*t})
				}
				if after >= 0 {
					clipped = append(clipped, current)
				}
				previous, before = current, after
			}
			polygon, scratch = clipped, polygon
		}
		if len(polygon) < 3 {
			continue
		}
		result = result.MoveTo(polygon[0].X, polygon[0].Y)
		for _, point := range polygon[1:] {
			result = result.LineTo(point.X, point.Y)
		}
		result = result.Close()
	}
	return result
}

func roundRectPolygons(path graphics.Path) [][]geometry.Point {
	var contours [][]geometry.Point
	var points []geometry.Point
	appendPoint := func(p geometry.Point) {
		if len(points) == 0 || points[len(points)-1] != p {
			points = append(points, p)
		}
	}
	var flatten func(geometry.Point, geometry.Point, geometry.Point, geometry.Point, int)
	flatten = func(a, b, c, d geometry.Point, depth int) {
		const tolerance = 1.0 / 64
		dx, dy := float64(d.X-a.X), float64(d.Y-a.Y)
		length := math.Hypot(dx, dy)
		flat := math.Max(math.Abs(dx*float64(b.Y-a.Y)-dy*float64(b.X-a.X)), math.Abs(dx*float64(c.Y-a.Y)-dy*float64(c.X-a.X)))
		if depth == 16 || flat <= tolerance*length {
			appendPoint(d)
			return
		}
		mid := func(p, q geometry.Point) geometry.Point {
			return geometry.Point{X: (p.X + q.X) / 2, Y: (p.Y + q.Y) / 2}
		}
		ab, bc, cd := mid(a, b), mid(b, c), mid(c, d)
		abc, bcd := mid(ab, bc), mid(bc, cd)
		center := mid(abc, bcd)
		flatten(a, ab, abc, center, depth+1)
		flatten(center, bcd, cd, d, depth+1)
	}
	path.Range(func(op graphics.PathOperation, args []float32) bool {
		switch op {
		case graphics.PathMoveTo:
			points = nil
			appendPoint(geometry.Point{X: args[0], Y: args[1]})
		case graphics.PathLineTo:
			appendPoint(geometry.Point{X: args[0], Y: args[1]})
		case graphics.PathBezierTo:
			flatten(points[len(points)-1], geometry.Point{X: args[0], Y: args[1]}, geometry.Point{X: args[2], Y: args[3]}, geometry.Point{X: args[4], Y: args[5]}, 0)
		case graphics.PathClose:
			if len(points) > 1 && points[len(points)-1] == points[0] {
				points = points[:len(points)-1]
			}
			contours = append(contours, points)
		}
		return false
	})
	return contours
}

type roundRectArc struct{ start, first, second, end geometry.Point }

func ellipseRectArcs(rect graphics.Rectangle, r [4]geometry.Point) [4]roundRectArc {
	x, y, right, bottom := rect.X, rect.Y, rect.X+rect.Width, rect.Y+rect.Height
	const k float32 = 0.5522847498307936
	return [4]roundRectArc{
		{geometry.Point{X: x, Y: y + r[0].Y}, geometry.Point{X: x, Y: y + r[0].Y*(1-k)}, geometry.Point{X: x + r[0].X*(1-k), Y: y}, geometry.Point{X: x + r[0].X, Y: y}},
		{geometry.Point{X: right - r[1].X, Y: y}, geometry.Point{X: right - r[1].X*(1-k), Y: y}, geometry.Point{X: right, Y: y + r[1].Y*(1-k)}, geometry.Point{X: right, Y: y + r[1].Y}},
		{geometry.Point{X: right, Y: bottom - r[2].Y}, geometry.Point{X: right, Y: bottom - r[2].Y*(1-k)}, geometry.Point{X: right - r[2].X*(1-k), Y: bottom}, geometry.Point{X: right - r[2].X, Y: bottom}},
		{geometry.Point{X: x + r[3].X, Y: bottom}, geometry.Point{X: x + r[3].X*(1-k), Y: bottom}, geometry.Point{X: x, Y: bottom - r[3].Y*(1-k)}, geometry.Point{X: x, Y: bottom - r[3].Y}},
	}
}

func (a roundRectArc) append(p graphics.Path, reverse bool) graphics.Path {
	if reverse {
		return p.BezierTo(a.second.X, a.second.Y, a.first.X, a.first.Y, a.start.X, a.start.Y)
	}
	return p.BezierTo(a.first.X, a.first.Y, a.second.X, a.second.Y, a.end.X, a.end.Y)
}

// Omitted sides connect the inner and outer boundaries. Represent each run of
// selected edges as its own closed contour instead of coincident solid/hole
// segments: touching contours have ambiguous AA fringes in stencil renderers.
func openRoundRectBorder(rect graphics.Rectangle, radius geometry.CornerRadius, inner graphics.Rectangle, corners [4]geometry.Point, insets geometry.Insets) graphics.Path {
	outer := ellipseRectArcs(rect, [4]geometry.Point{
		{X: radius.TopLeft, Y: radius.TopLeft}, {X: radius.TopRight, Y: radius.TopRight},
		{X: radius.BottomRight, Y: radius.BottomRight}, {X: radius.BottomLeft, Y: radius.BottomLeft},
	})
	inside := ellipseRectArcs(inner, corners)
	edges := [4]float32{insets.Top, insets.Right, insets.Bottom, insets.Left}
	p := graphics.Path{}.Grow(96)
	for start := 0; start < 4; start++ {
		if edges[start] == 0 || edges[(start+3)%4] > 0 {
			continue
		}
		end := start
		for edges[(end+1)%4] > 0 {
			end++
		}
		a := outer[start]
		p = a.append(p.MoveTo(a.start.X, a.start.Y), false)
		for edge := start; edge <= end; edge++ {
			a = outer[(edge+1)%4]
			p = a.append(p.LineTo(a.start.X, a.start.Y), false)
		}
		a = inside[(end+1)%4]
		p = a.append(p.LineTo(a.end.X, a.end.Y), true)
		for edge := end; edge >= start; edge-- {
			a = inside[edge%4]
			p = a.append(p.LineTo(a.end.X, a.end.Y), true)
		}
		p = p.Close()
	}
	return p
}

func validRoundRectInsets(i geometry.Insets) bool {
	var width float32
	for _, n := range [4]float32{i.Left, i.Top, i.Right, i.Bottom} {
		if !(n >= 0) || math.IsInf(float64(n), 0) {
			return false
		}
		if n > 0 {
			if width > 0 && n != width {
				return false
			}
			width = n
		}
	}
	return true
}

func insetRoundRect(rect graphics.Rectangle, radius geometry.CornerRadius, i geometry.Insets) (graphics.Rectangle, [4]geometry.Point) {
	inner := rect.Insets(i.Left, i.Top, i.Right, i.Bottom)
	r := [4]geometry.Point{
		{X: max(0, radius.TopLeft-i.Left), Y: max(0, radius.TopLeft-i.Top)},
		{X: max(0, radius.TopRight-i.Right), Y: max(0, radius.TopRight-i.Top)},
		{X: max(0, radius.BottomRight-i.Right), Y: max(0, radius.BottomRight-i.Bottom)},
		{X: max(0, radius.BottomLeft-i.Left), Y: max(0, radius.BottomLeft-i.Bottom)},
	}
	// A zero ellipse axis is a square corner. Normalize the remaining radii
	// together so adjacent arcs cannot cross when the insets consume space.
	for n, c := range r {
		if c.X == 0 || c.Y == 0 {
			r[n] = geometry.Point{}
		}
	}
	scale := float32(1)
	for _, pair := range [4][2]float32{
		{inner.Width, r[0].X + r[1].X}, {inner.Width, r[3].X + r[2].X},
		{inner.Height, r[0].Y + r[3].Y}, {inner.Height, r[1].Y + r[2].Y},
	} {
		if pair[1] > 0 {
			scale = min(scale, max(0, pair[0])/pair[1])
		}
	}
	for n := range r {
		r[n].X *= scale
		r[n].Y *= scale
	}
	return inner, r
}

// Corners are clockwise from top-left.
func ellipseRectContour(p graphics.Path, rect graphics.Rectangle, r [4]geometry.Point) graphics.Path {
	arcs := ellipseRectArcs(rect, r)
	for n := 0; n < 4; n++ {
		a := arcs[(n+1)%4]
		p = a.append(p.LineTo(a.start.X, a.start.Y), false)
	}
	return p
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
