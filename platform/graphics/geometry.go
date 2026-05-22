package graphics

import "github.com/golang-gui/goui/core/geometry"

type (
	Point     = geometry.Point
	Pos       = geometry.Pos
	Size      = geometry.Size
	Rectangle = geometry.Rectangle
)

func Rect(x, y, w, h float32) Rectangle {
	return Rectangle{
		Pos: Point{
			X: x,
			Y: y,
		},
		Size: Size{
			Width:  w,
			Height: h,
		},
	}
}
