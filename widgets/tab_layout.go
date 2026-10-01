package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
)

// Arrow space is reserved only after crossing the minimum-width threshold.
// It never feeds back into the threshold, so repeated layout cannot oscillate
// between scroll and shrink. In overflow, widths stay at the minimum.
func allocateTabWidth(count int, available, minimum, maximum float32) (float32, bool) {
	if count == 0 {
		return 0, false
	}
	gaps := float32(count-1) * tabSpacing
	if available < float32(count)*minimum+gaps {
		return minimum, true
	}
	return min(maximum, (available-gaps)/float32(count)), false
}

// Measure intrinsic line height before applying the parent's limits. In
// particular, the item's 32 DIP minimum is not a tight height for its text.
type tabItemLayout struct{}

func (tabItemLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	var size geometry.Size
	for _, child := range children {
		m := child.Measure(layout.Unbounded())
		size.Width = max(size.Width, m.Width)
		size.Height = max(size.Height, m.Height)
	}
	return layout.Measured(c.Clamp(size))
}

func (tabItemLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	if len(children) == 0 {
		return
	}
	children[0].Arrange(geometry.Rect(0, 0, rect.Width, rect.Height))
	if len(children) > 1 {
		close := children[1]
		width := max(0, rect.Width-4)
		m := close.Measure(layout.Loose(geometry.Size{Width: width, Height: rect.Height}))
		width = max(0, width-m.Width)
		close.Arrange(geometry.Rect(width, (rect.Height-m.Height)/2, m.Width, m.Height))
	}
}

// Horizontal padding is 10 DIP and vertical padding is 4 DIP. A weighted label
// gives up width to the icon and close button, but keeps its natural line height.
type tabBodyLayout struct{ page *TabPage }

func (l tabBodyLayout) closeSpace() float32 {
	if l.page.closable {
		return 24 + 4 // close target plus trailing space, even while hidden
	}
	return 0
}

func tabBodyRow() layout.LinearLayout {
	return layout.LinearLayout{Direction: layout.DirectionHorizontal, CrossAlign: layout.CrossCenter, Padding: 4, Spacing: 6}
}

func (l tabBodyLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	inner := c
	extra := 12 + l.closeSpace()
	inner.Min.Width = max(0, inner.Min.Width-extra)
	if inner.Max.Width < layout.Inf {
		inner.Max.Width = max(0, inner.Max.Width-extra)
	}
	row := tabBodyRow()
	m := row.Measure(children, inner)
	m.Width += extra
	return m.Constrain(c)
}

func (l tabBodyLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	row := tabBodyRow()
	row.Arrange(children, geometry.Rect(min(6, rect.Width), 0, max(0, rect.Width-12-l.closeSpace()), rect.Height))
}
