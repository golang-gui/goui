package gui

import "github.com/golang-gui/goui/layout"

// WrapBox is an ordered, non-virtualized wrapping container.
// Children keep their measured sizes; their MainWeight is ignored.
type WrapBox struct {
	WidgetBase
	layout *layout.WrapLayout
}

// NewWrapBox defaults to MainStart and CrossDefault, with zero spacing and
// padding. Lines start at the cross-axis origin; CrossAlign acts within a line.
func NewWrapBox(direction layout.Direction) *WrapBox {
	box := &WrapBox{
		layout: layout.NewWrapLayout(direction),
	}
	box.SetLayoutManager(box.layout)
	return box
}

func (b *WrapBox) AddChild(child Widget) {
	b.WidgetBase.AddChild(b, child)
}

func (b *WrapBox) SetLayoutManager(manager layout.LayoutManager) {
	wrap, ok := manager.(*layout.WrapLayout)
	if !ok {
		return
	}
	b.layout = wrap
	b.WidgetBase.SetLayoutManager(manager)
}

func (b *WrapBox) Direction() layout.Direction {
	return b.layout.Direction
}

func (b *WrapBox) SetDirection(direction layout.Direction) {
	if b.layout.Direction == direction {
		return
	}
	b.layout.Direction = direction
	b.RequestLayout()
}

func (b *WrapBox) Spacing() float32 {
	return b.layout.Spacing
}

// SetSpacing sets the gap between children. Negative and non-finite values
// are treated as zero.
func (b *WrapBox) SetSpacing(spacing float32) {
	spacing = normalizeLayoutValue(spacing)
	if b.layout.Spacing == spacing {
		return
	}
	b.layout.Spacing = spacing
	b.RequestLayout()
}

// LineSpacing is the gap between rows (horizontal) or columns (vertical).
func (b *WrapBox) LineSpacing() float32 { return b.layout.LineSpacing }

// SetLineSpacing sets the gap between lines. Invalid values are treated as zero.
func (b *WrapBox) SetLineSpacing(spacing float32) {
	spacing = normalizeLayoutValue(spacing)
	if b.layout.LineSpacing == spacing {
		return
	}
	b.layout.LineSpacing = spacing
	b.RequestLayout()
}

func (b *WrapBox) Padding() float32 {
	return b.layout.Padding
}

// SetPadding sets the inner padding. Negative and non-finite values are
// treated as zero.
func (b *WrapBox) SetPadding(padding float32) {
	padding = normalizeLayoutValue(padding)
	if b.layout.Padding == padding {
		return
	}
	b.layout.Padding = padding
	b.RequestLayout()
}

func (b *WrapBox) MainAlign() layout.MainAlign {
	return b.layout.MainAlign
}

// SetMainAlign positions the children within each line, including the last.
func (b *WrapBox) SetMainAlign(align layout.MainAlign) {
	if b.layout.MainAlign == align {
		return
	}
	b.layout.MainAlign = align
	b.RequestLayout()
}

// CrossAlign returns the configured policy, including CrossDefault. Changing
// direction reinterprets CrossDefault without changing this stored value.
func (b *WrapBox) CrossAlign() layout.CrossAlign {
	return b.layout.CrossAlign
}

// SetCrossAlign aligns children within each line, not the block of all lines.
// Stretch fills that line's extent; Baseline applies only to horizontal rows.
func (b *WrapBox) SetCrossAlign(align layout.CrossAlign) {
	if b.layout.CrossAlign == align {
		return
	}
	b.layout.CrossAlign = align
	b.RequestLayout()
}

func (b *WrapBox) Snapshot() WidgetInfo {
	info := b.WidgetBase.Snapshot()
	info.Role = RoleWrapBox
	return info
}
