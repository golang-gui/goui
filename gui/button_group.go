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
	radius, _ := s.Radius()
	radius = max(0, min(radius, rect.Width/2, rect.Height/2))
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
	p.DrawPath(buttonContour(rect.Inset(width/2), max(radius-width/2, 0), corners, omitEdge), width, stroke)
}

// A closed contour walks clockwise. Starting after the omitted edge produces
// an open three-sided stroke without clipping or repainting a sibling.
func buttonContour(rect geometry.Rectangle, radius float32, corners buttonCorners, omitEdge int) graphics.Path {
	x, y, right, bottom := rect.X, rect.Y, rect.X+rect.Width, rect.Y+rect.Height
	r := [4]float32{}
	for i := range r {
		if corners&(1<<i) != 0 {
			r[i] = radius
		}
	}
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
