package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
)

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
