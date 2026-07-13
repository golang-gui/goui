package layout

import "github.com/golang-gui/goui/core/geometry"

type FillLayout struct{}

func NewFillLayout() *FillLayout {
	return new(FillLayout)
}

func (l *FillLayout) Measure(children []Child, c Constraint) geometry.Size {
	inner := Loose(c.Max)
	var size geometry.Size
	for _, child := range children {
		if child == nil {
			continue
		}
		childSize := child.Measure(inner)
		size.Width = max(size.Width, childSize.Width)
		size.Height = max(size.Height, childSize.Height)
	}
	return c.Clamp(size)
}

func (l *FillLayout) Arrange(children []Child, rect geometry.Rectangle) {
	for _, child := range children {
		if child != nil {
			child.Arrange(rect)
		}
	}
}
