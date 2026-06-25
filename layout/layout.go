package layout

import "github.com/golang-gui/goui/core/geometry"

type Child interface {
	Measure(available geometry.Size) geometry.Size
	Arrange(rect geometry.Rectangle)
}

type LayoutManager interface {
	Measure(children []Child, available geometry.Size) geometry.Size
	Arrange(children []Child, rect geometry.Rectangle)
}

type Direction int

const (
	DirectionHorizontal Direction = iota
	DirectionVertical
)
