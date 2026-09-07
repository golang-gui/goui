package layout

import "github.com/golang-gui/goui/core/geometry"

type FillLayout struct{}

func NewFillLayout() *FillLayout {
	return new(FillLayout)
}

func (l *FillLayout) Measure(children []Child, c Constraint) Measurement {
	inner := Loose(c.Max)
	var measured Measurement
	for _, child := range children {
		if child == nil {
			continue
		}
		childMeasured := child.Measure(inner)
		measured.Width = max(measured.Width, childMeasured.Width)
		measured.Height = max(measured.Height, childMeasured.Height)
		if !measured.HasBaseline && childMeasured.HasBaseline {
			measured.Baseline = childMeasured.Baseline
			measured.HasBaseline = true
		}
	}
	return measured.Constrain(c)
}

func (l *FillLayout) Arrange(children []Child, rect geometry.Rectangle) {
	for _, child := range children {
		if child != nil {
			child.Arrange(rect)
		}
	}
}
