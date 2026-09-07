package gui

import "github.com/golang-gui/goui/layout"

type LinearBox struct {
	WidgetBase
	layout *layout.LinearLayout
}

// NewLinearBox creates a box with MainStart and CrossDefault alignment,
// no spacing, and no padding. CrossDefault centers rows and starts columns.
func NewLinearBox(direction layout.Direction) *LinearBox {
	box := &LinearBox{
		layout: layout.NewLinearLayout(direction),
	}
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

// SetSpacing sets the gap between children. Negative and non-finite values
// are treated as zero.
func (b *LinearBox) SetSpacing(spacing float32) {
	spacing = normalizeLayoutValue(spacing)
	if b.layout.Spacing == spacing {
		return
	}
	b.layout.Spacing = spacing
	b.RequestLayout()
}

func (b *LinearBox) Padding() float32 {
	return b.layout.Padding
}

// SetPadding sets the inner padding. Negative and non-finite values are
// treated as zero.
func (b *LinearBox) SetPadding(padding float32) {
	padding = normalizeLayoutValue(padding)
	if b.layout.Padding == padding {
		return
	}
	b.layout.Padding = padding
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

// CrossAlign returns the configured policy, including CrossDefault. Changing
// direction reinterprets CrossDefault without changing this stored value.
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
