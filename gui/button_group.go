package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

// ButtonGroup arranges directly contained buttons into a connected strip.
// It changes only layout and decoration, not selection or activation behavior.
// Button, MenuButton and button-appearance CheckButton support the geometry;
// custom buttons can opt in with PaintButtonBox and PaintButtonFocus.
type ButtonGroup struct {
	WidgetBase
	layout *layout.LinearLayout
}

// NewButtonGroup creates a horizontal strip with zero spacing and stretched
// cross-axis alignment. Hidden children do not occupy a segment.
func NewButtonGroup() *ButtonGroup {
	g := &ButtonGroup{layout: layout.NewLinearLayout(layout.DirectionHorizontal)}
	g.layout.CrossAlign = layout.CrossStretch
	g.SetLayoutManager(g.layout)
	return g
}

func (g *ButtonGroup) AddChild(child Widget)       { g.WidgetBase.AddChild(g, child) }
func (g *ButtonGroup) Direction() layout.Direction { return g.layout.Direction }
func (g *ButtonGroup) SetDirection(direction layout.Direction) {
	if direction != layout.DirectionHorizontal && direction != layout.DirectionVertical || g.layout.Direction == direction {
		return
	}
	g.layout.Direction = direction
	g.RequestLayout()
}
func (g *ButtonGroup) Snapshot() WidgetInfo {
	info := g.WidgetBase.Snapshot()
	info.Role = RoleHBox
	if g.Direction() == layout.DirectionVertical {
		info.Role = RoleVBox
	}
	return info
}
func (g *ButtonGroup) Paint(p Painter) { PaintWidgetStyle(p, g, "button-group") }

// PaintButtonBox paints a button's own resolved style. Within a ButtonGroup it
// rounds only exposed corners and omits the trailing shared border. The next
// segment owns that seam. It does not paint children or resolve their styles.
func PaintButtonBox(p Painter, widget Widget, s style.Style) {
	paintButtonBox(p, widget, s, false)
}

// PaintButtonFocus paints a complete focus decoration, including shared edges,
// using the same exposed corners as PaintButtonBox. Call only when appropriate
// for the control's focus policy; this helper does not query or set focus.
func PaintButtonFocus(p Painter, widget Widget, s style.Style) {
	paintButtonBox(p, widget, s, true)
}

type buttonCorners uint8

const (
	buttonTopLeft buttonCorners = 1 << iota
	buttonTopRight
	buttonBottomRight
	buttonBottomLeft
	buttonAllCorners = buttonTopLeft | buttonTopRight | buttonBottomRight | buttonBottomLeft
)

// The geometry is derived from actual visible child order on every paint.
// There is no member registry to invalidate after hiding or reordering children.
func buttonSegment(widget Widget) (corners buttonCorners, omitEdge int, grouped bool) {
	group, ok := widget.Parent().(*ButtonGroup)
	if !ok {
		return buttonAllCorners, -1, false
	}
	var first, last Widget
	for _, child := range group.Children() {
		if child.Visible() && !child.base().destroyed {
			if first == nil {
				first = child
			}
			last = child
		}
	}
	if first == nil {
		return buttonAllCorners, -1, false
	}
	if group.Direction() == layout.DirectionHorizontal {
		if widget == first {
			corners |= buttonTopLeft | buttonBottomLeft
		}
		if widget == last {
			corners |= buttonTopRight | buttonBottomRight
		}
		omitEdge = 1 // right
	} else {
		if widget == first {
			corners |= buttonTopLeft | buttonTopRight
		}
		if widget == last {
			corners |= buttonBottomLeft | buttonBottomRight
		}
		omitEdge = 2 // bottom
	}
	if widget == last {
		omitEdge = -1
	}
	return corners, omitEdge, true
}

func paintButtonBox(p Painter, widget Widget, s style.Style, focus bool) {
	rect := geometry.Rectangle{Size: widget.Rect().Size}
	corners, omitEdge, grouped := buttonSegment(widget)
	if !grouped {
		PaintStyledBox(p, rect, s)
		return
	}
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	radius, _ := s.CornerRadius()
	radius = buttonCornerRadius(radius, corners).Normalize(rect.Size)
	if bg, ok := s.BackgroundColor(); ok && bg != nil {
		fill := graphics.ColorOf(bg)
		if fill.A > 0 {
			p.FillPath(buttonContour(rect, radius, corners, -1), fill)
		}
	}
	width, _ := s.BorderWidth()
	border, ok := s.BorderColor()
	if !ok || border == nil || width <= 0 {
		return
	}
	stroke := graphics.ColorOf(border)
	if stroke.A == 0 {
		return
	}
	if width >= min(rect.Width, rect.Height)/2 {
		p.FillPath(buttonContour(rect, radius, corners, -1), stroke)
		return
	}
	if focus {
		omitEdge = -1
	}
	if omitEdge < 0 {
		p.FillPath(graphics.RoundRectBorderPath(rect, buttonCornerRadius(radius, corners), width), stroke)
	} else {
		outer := buttonContour(rect, radius, corners, omitEdge)
		innerRect := rect.Inset(width)
		inner := buttonContourReverse(innerRect, radius.Inset(width).Normalize(innerRect.Size), corners, omitEdge)
		inner.Range(func(op graphics.PathOperation, args []float32) bool {
			switch op {
			case graphics.PathMoveTo, graphics.PathLineTo:
				outer = outer.LineTo(args[0], args[1])
			case graphics.PathBezierTo:
				outer = outer.BezierTo(args[0], args[1], args[2], args[3], args[4], args[5])
			}
			return false
		})
		p.FillPath(outer.Close(), stroke)
	}
}

// A closed contour walks clockwise. Starting after the omitted edge produces
// an open three-sided stroke without clipping or repainting a sibling.
func buttonContour(rect geometry.Rectangle, radius geometry.CornerRadius, corners buttonCorners, omitEdge int) graphics.Path {
	x, y, right, bottom := rect.X, rect.Y, rect.X+rect.Width, rect.Y+rect.Height
	radius = buttonCornerRadius(radius, corners).Normalize(rect.Size)
	r := [4]float32{radius.TopLeft, radius.TopRight, radius.BottomRight, radius.BottomLeft}
	starts := [4]geometry.Point{{X: x, Y: y + r[0]}, {X: right - r[1], Y: y}, {X: right, Y: bottom - r[2]}, {X: x + r[3], Y: bottom}}
	ends := [4]geometry.Point{{X: x + r[0], Y: y}, {X: right, Y: y + r[1]}, {X: right - r[2], Y: bottom}, {X: x, Y: bottom - r[3]}}
	const k float32 = 0.55228475
	controls := [4][2]geometry.Point{
		{{X: x, Y: y + r[0]*(1-k)}, {X: x + r[0]*(1-k), Y: y}},
		{{X: right - r[1]*(1-k), Y: y}, {X: right, Y: y + r[1]*(1-k)}},
		{{X: right, Y: bottom - r[2]*(1-k)}, {X: right - r[2]*(1-k), Y: bottom}},
		{{X: x + r[3]*(1-k), Y: bottom}, {X: x, Y: bottom - r[3]*(1-k)}},
	}
	start := 0
	if omitEdge >= 0 {
		start = (omitEdge + 1) % 4
	}
	path := graphics.MoveTo(starts[start].X, starts[start].Y)
	for step := 0; step < 4; step++ {
		i := (start + step) % 4
		if step > 0 {
			path = path.LineTo(starts[i].X, starts[i].Y)
		}
		if r[i] > 0 {
			path = path.BezierTo(controls[i][0].X, controls[i][0].Y, controls[i][1].X, controls[i][1].Y, ends[i].X, ends[i].Y)
		}
	}
	if omitEdge < 0 {
		path = path.Close()
	}
	return path
}

func buttonCornerRadius(r geometry.CornerRadius, corners buttonCorners) geometry.CornerRadius {
	if corners&buttonTopLeft == 0 {
		r.TopLeft = 0
	}
	if corners&buttonTopRight == 0 {
		r.TopRight = 0
	}
	if corners&buttonBottomRight == 0 {
		r.BottomRight = 0
	}
	if corners&buttonBottomLeft == 0 {
		r.BottomLeft = 0
	}
	return r
}

// Reverse the open contour to close the border around the omitted shared edge.
func buttonContourReverse(rect geometry.Rectangle, radius geometry.CornerRadius, corners buttonCorners, omitEdge int) graphics.Path {
	path := buttonContour(rect, radius, corners, omitEdge)
	type segment struct {
		op   graphics.PathOperation
		from geometry.Point
		args [6]float32
	}
	segments := make([]segment, 0, 8)
	var current geometry.Point
	path.Range(func(op graphics.PathOperation, args []float32) bool {
		if op == graphics.PathMoveTo {
			current = geometry.Point{X: args[0], Y: args[1]}
			return false
		}
		part := segment{op: op, from: current}
		copy(part.args[:], args)
		segments = append(segments, part)
		current = geometry.Point{X: args[len(args)-2], Y: args[len(args)-1]}
		return false
	})
	result := graphics.MoveTo(current.X, current.Y)
	for i := len(segments) - 1; i >= 0; i-- {
		part := segments[i]
		if part.op == graphics.PathBezierTo {
			result = result.BezierTo(part.args[2], part.args[3], part.args[0], part.args[1], part.from.X, part.from.Y)
		} else {
			result = result.LineTo(part.from.X, part.from.Y)
		}
	}
	return result
}
