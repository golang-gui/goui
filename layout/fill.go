package layout

import "github.com/golang-gui/goui/core/geometry"

type FillLayout struct{}

func NewFillLayout() *FillLayout {
	return new(FillLayout)
}

func (l *FillLayout) Measure(children []Child, available geometry.Size) geometry.Size {
	var size geometry.Size
	for _, child := range children {
		if child == nil {
			continue
		}
		childSize := child.Measure(available)
		size.Width = max(size.Width, childSize.Width)
		size.Height = max(size.Height, childSize.Height)
	}
	return size
}

func (l *FillLayout) Arrange(children []Child, rect geometry.Rectangle) {
	for _, child := range children {
		if child != nil {
			child.Arrange(rect)
		}
	}
}
