package layout

import "github.com/golang-gui/goui/core/geometry"

type LinearLayout struct {
	Direction Direction
	Spacing   float32
}

func NewLinearLayout(direction Direction) *LinearLayout {
	return &LinearLayout{
		Direction: direction,
	}
}

func (l *LinearLayout) Measure(children []Child, available geometry.Size) geometry.Size {
	if len(children) == 0 {
		return geometry.Size{}
	}

	var size geometry.Size
	count := 0
	for _, child := range children {
		if child == nil {
			continue
		}
		childSize := child.Measure(available)
		if count > 0 {
			l.addSpacing(&size)
		}
		l.addChildSize(&size, childSize)
		count++
	}
	return size
}

func (l *LinearLayout) Arrange(children []Child, rect geometry.Rectangle) {
	var offset float32
	count := 0
	for _, child := range children {
		if child == nil {
			continue
		}
		if count > 0 {
			offset += l.Spacing
		}
		childSize := child.Measure(l.childAvailable(rect, offset))
		child.Arrange(l.childRect(rect, offset, childSize))
		offset += l.mainSize(childSize)
		count++
	}
}

func (l *LinearLayout) addSpacing(size *geometry.Size) {
	if l.Direction == DirectionVertical {
		size.Height += l.Spacing
		return
	}
	size.Width += l.Spacing
}

func (l *LinearLayout) addChildSize(size *geometry.Size, childSize geometry.Size) {
	if l.Direction == DirectionVertical {
		size.Width = max(size.Width, childSize.Width)
		size.Height += childSize.Height
		return
	}
	size.Width += childSize.Width
	size.Height = max(size.Height, childSize.Height)
}

func (l *LinearLayout) childRect(rect geometry.Rectangle, offset float32, childSize geometry.Size) geometry.Rectangle {
	if l.Direction == DirectionVertical {
		return geometry.Rect(rect.X, rect.Y+offset, rect.Width, childSize.Height)
	}
	return geometry.Rect(rect.X+offset, rect.Y, childSize.Width, rect.Height)
}

func (l *LinearLayout) childAvailable(rect geometry.Rectangle, offset float32) geometry.Size {
	if l.Direction == DirectionVertical {
		return geometry.Size{
			Width:  rect.Width,
			Height: max(0, rect.Height-offset),
		}
	}
	return geometry.Size{
		Width:  max(0, rect.Width-offset),
		Height: rect.Height,
	}
}

func (l *LinearLayout) mainSize(size geometry.Size) float32 {
	if l.Direction == DirectionVertical {
		return size.Height
	}
	return size.Width
}
