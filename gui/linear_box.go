package gui

import "github.com/golang-gui/goui/layout"

type LinearBox struct {
	WidgetBase
	layout *layout.LinearLayout
}

func NewLinearBox(direction layout.Direction) *LinearBox {
	box := &LinearBox{
		layout: layout.NewLinearLayout(direction),
	}
	box.SetStyleName(styleNameBox)
	box.SetLayoutManager(box.layout)
	return box
}

func (b *LinearBox) AddChild(child Widget) {
	b.WidgetBase.AddChild(b, child)
}

func (b *LinearBox) SetLayoutManager(manager layout.LayoutManager) {
	linear, ok := manager.(*layout.LinearLayout)
	if !ok {
		return
	}
	b.layout = linear
	b.WidgetBase.SetLayoutManager(manager)
}

func (b *LinearBox) Direction() layout.Direction {
	return b.layout.Direction
}

func (b *LinearBox) SetDirection(direction layout.Direction) {
	if b.layout.Direction == direction {
		return
	}
	b.layout.Direction = direction
	b.RequestLayout()
}

func (b *LinearBox) Spacing() float32 {
	return b.layout.Spacing
}

func (b *LinearBox) SetSpacing(spacing float32) {
	if b.layout.Spacing == spacing {
		return
	}
	b.layout.Spacing = spacing
	b.RequestLayout()
}

func (b *LinearBox) MainAlign() layout.MainAlign {
	return b.layout.MainAlign
}

func (b *LinearBox) SetMainAlign(align layout.MainAlign) {
	if b.layout.MainAlign == align {
		return
	}
	b.layout.MainAlign = align
	b.RequestLayout()
}

func (b *LinearBox) CrossAlign() layout.CrossAlign {
	return b.layout.CrossAlign
}

func (b *LinearBox) SetCrossAlign(align layout.CrossAlign) {
	if b.layout.CrossAlign == align {
		return
	}
	b.layout.CrossAlign = align
	b.RequestLayout()
}

func (b *LinearBox) Snapshot() WidgetInfo {
	info := b.WidgetBase.Snapshot()
	if b.Direction() == layout.DirectionVertical {
		info.Role = RoleVBox
	} else {
		info.Role = RoleHBox
	}
	return info
}
