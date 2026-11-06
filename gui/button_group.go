package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
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
	if focus {
		omitEdge = -1
	}
	sides := styledBorderSides(s)
	if omitEdge == 1 {
		sides &^= style.BorderRight
	} else if omitEdge == 2 {
		sides &^= style.BorderBottom
	}
	paintStyledBox(p, rect, s, radius, sides)
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
