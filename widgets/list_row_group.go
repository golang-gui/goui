package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
)

// ListRowGroup stacks directly contained rows into one visually connected
// column. It changes only layout and decoration: rows keep their own styles,
// states, focus and activation, and the group adds no selection or action.
// Rows are stacked without a gap; only the group's outer corners stay rounded
// and neighbours share a single border line. A ListRow paints that geometry
// itself, so nested containers and non-row children stay independent.
type ListRowGroup struct {
	gui.WidgetBase
	uniformRows bool
}

// NewListRowGroup creates a vertical group with no spacing or padding and
// stretched cross-axis alignment. Hidden children leave no gap and do not keep
// a seam; the visible first and last rows own the outer contour.
func NewListRowGroup() *ListRowGroup {
	g := &ListRowGroup{}
	g.SetLayoutManager(listRowGroupLayout{group: g, LinearLayout: layout.LinearLayout{
		Direction: layout.DirectionVertical, CrossAlign: layout.CrossStretch,
	}})
	return g
}

// UniformRows reports whether visible direct ListRow children share the tallest
// row's natural height at the available width. The default is false.
func (g *ListRowGroup) UniformRows() bool { return g.uniformRows }

// SetUniformRows enables equal natural row heights without changing the rows'
// size properties. Enabled rows ignore MainWeight; other children keep their
// normal layout. Disabling it restores the ordinary per-child layout.
func (g *ListRowGroup) SetUniformRows(uniform bool) {
	if g.Destroyed() || g.uniformRows == uniform {
		return
	}
	g.uniformRows = uniform
	g.RequestLayout()
}

func (g *ListRowGroup) AddChild(child gui.Widget) { g.WidgetBase.AddChild(g, child) }

func (g *ListRowGroup) Snapshot() gui.WidgetInfo {
	info := g.WidgetBase.Snapshot()
	info.Role = gui.RoleVBox
	return info
}

// Paint draws only the optional group decoration; rows paint their own boxes.
// No default rule exists for "list-row-group": the connected look comes from
// the rows themselves, so a group surface would paint underneath them.
func (g *ListRowGroup) Paint(p gui.Painter) { gui.PaintWidgetStyle(p, g, "list-row-group") }

// listRowCorners selects the corners that stay rounded on one connected row.
type listRowCorners uint8

const (
	listRowTopLeft listRowCorners = 1 << iota
	listRowTopRight
	listRowBottomRight
	listRowBottomLeft
	listRowAllCorners = listRowTopLeft | listRowTopRight | listRowBottomRight | listRowBottomLeft
)

// listRowSegment is one row's place in a connected column. Grouped means the
// row shares an edge with a row neighbour, so it must drop the corners and the
// seam border that neighbour no longer repaints.
type listRowSegment struct {
	corners listRowCorners
	shared  bool // the next visible row owns the bottom edge
	grouped bool
}

// complete restores a closed outline for decorations that must not open, such
// as the keyboard focus ring. The exposed outer corners are unchanged.
func (s listRowSegment) complete() listRowSegment {
	s.shared = false
	return s
}

// radius zeroes the corners a neighbour already owns, then fits the remainder
// into the row rectangle.
func (s listRowSegment) radius(r geometry.CornerRadius, size geometry.Size) geometry.CornerRadius {
	if s.grouped {
		if s.corners&listRowTopLeft == 0 {
			r.TopLeft = 0
		}
		if s.corners&listRowTopRight == 0 {
			r.TopRight = 0
		}
		if s.corners&listRowBottomRight == 0 {
			r.BottomRight = 0
		}
		if s.corners&listRowBottomLeft == 0 {
			r.BottomLeft = 0
		}
	}
	return r.Normalize(size)
}

// sides removes the bottom border of a row whose seam belongs to the next row.
// An unset BorderSides means all edges, as everywhere else in the style layer.
func (s listRowSegment) sides(v style.Style) style.BorderSides {
	sides, set := v.BorderSides()
	if !set {
		sides = style.BorderAll
	}
	if s.grouped && s.shared {
		sides &^= style.BorderBottom
	}
	return sides
}

// listRowSegmentOf derives a row's geometry from the actual visible children on
// every paint. There is no membership registry to invalidate after hiding,
// reordering or removing rows, and no state is stored on either side.
//
// Only a directly adjacent visible row connects. Any other visible child ends
// the chain, so rows separated by unrelated content keep a complete outline
// instead of omitting a border that nothing would repaint. Hidden children take
// no layout space, so hiding a middle row joins its neighbours rather than
// leaving a seam gap.
func listRowSegmentOf(row *ListRow) listRowSegment {
	standalone := listRowSegment{corners: listRowAllCorners}
	group, ok := row.Parent().(*ListRowGroup)
	if !ok || group == nil || row.Destroyed() {
		return standalone
	}
	var visible []gui.Widget
	for _, child := range group.Children() {
		if child.Visible() && !widgetDestroyed(child) {
			visible = append(visible, child)
		}
	}
	index := -1
	for i, child := range visible {
		if child == row {
			index = i
			break
		}
	}
	if index < 0 {
		return standalone
	}
	var top, bottom bool
	if index > 0 {
		_, top = visible[index-1].(*ListRow)
	}
	if index+1 < len(visible) {
		_, bottom = visible[index+1].(*ListRow)
	}
	if !top && !bottom {
		return standalone
	}
	segment := listRowSegment{grouped: true, shared: bottom}
	if !top {
		segment.corners |= listRowTopLeft | listRowTopRight
	}
	if !bottom {
		segment.corners |= listRowBottomLeft | listRowBottomRight
	}
	return segment
}

func widgetDestroyed(widget gui.Widget) bool {
	dead, ok := widget.(interface{ Destroyed() bool })
	return ok && dead.Destroyed()
}

// paintListRowBox paints one resolved row style with the connected geometry.
// The segment decides corners and the owned seam; background, border and focus
// rules still come entirely from the row's own resolved style.
func paintListRowBox(p gui.Painter, rect geometry.Rectangle, v style.Style, segment listRowSegment) {
	if !segment.grouped {
		gui.PaintStyledBox(p, rect, v)
		return
	}
	radius, _ := v.CornerRadius()
	painted := style.Rule{Style: v}.CornerRadius(segment.radius(radius, rect.Size)).BorderSides(segment.sides(v)).Style
	gui.PaintStyledBox(p, rect, painted)
}

type listRowGroupLayout struct {
	layout.LinearLayout
	group *ListRowGroup
}

func (l listRowGroupLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	return l.LinearLayout.Measure(l.uniformChildren(children, c), c)
}

func (l listRowGroupLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	l.LinearLayout.Arrange(l.uniformChildren(children, layout.Tight(rect.Size)), rect)
}

func (l listRowGroupLayout) uniformChildren(children []layout.Child, c layout.Constraint) []layout.Child {
	if !l.group.uniformRows || len(children) == 0 {
		return children
	}
	// Match LinearLayout's CrossStretch measurement width, including a finite
	// minimum on an otherwise unbounded axis. Height stays natural, independent
	// of the group's allocation or an individual row's MainWeight.
	width := c.Max.Width
	if width >= layout.Inf {
		width = c.Min.Width
	}
	if width < layout.Inf && (width > 0 || c.Max.Width == 0) {
		c.Min.Width, c.Max.Width = width, width
	}
	c.Min.Height, c.Max.Height = 0, layout.Inf
	items := append([]layout.Child(nil), children...)
	index, height := 0, float32(0)
	// WidgetBase supplies only visible children, in this same order. Keep its
	// layout.Child adapters so measurement still uses the GUI cache and styles.
	for _, widget := range l.group.Children() {
		if !widget.Visible() {
			continue
		}
		if index >= len(items) {
			break
		}
		if _, ok := widget.(*ListRow); ok {
			m := children[index].Measure(c)
			height = max(height, m.Height)
			items[index] = uniformListRowChild{Child: children[index], measured: m}
		}
		index++
	}
	for i, item := range items {
		if row, ok := item.(uniformListRowChild); ok {
			row.measured.Height = height
			items[i] = row
		}
	}
	return items
}

// This allocation adapter is local to one layout pass. It never changes the
// row's persistent min/max sizes, weight or own layout manager.
type uniformListRowChild struct {
	layout.Child
	measured layout.Measurement
}

func (c uniformListRowChild) Measure(constraint layout.Constraint) layout.Measurement {
	return c.measured.Constrain(constraint)
}

func (uniformListRowChild) MainWeight() float32 { return 0 }
