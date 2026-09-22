package ui

import (
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

// WrapView reconciles a gui.WrapBox and its ordered child views.
type WrapView struct {
	ViewBase[WrapView]
	direction   layout.Direction
	spacing     float32
	lineSpacing float32
	padding     float32
	mainAlign   layout.MainAlign
	crossAlign  layout.CrossAlign
	children    []View
}

// HWrap arranges children horizontally, wrapping into rows at finite width.
// It packs from the start and centers children within each row (CrossDefault),
// with no spacing or padding. Child MainWeight is ignored.
func HWrap(children ...View) *WrapView {
	v := &WrapView{
		direction: layout.DirectionHorizontal,
		children:  compactViews(children),
	}
	v.Self = v
	return v
}

// VWrap arranges children vertically, wrapping into columns at finite height.
// It packs from the start and aligns children to the left within each column
// (CrossDefault), with no spacing or padding. Child MainWeight is ignored.
func VWrap(children ...View) *WrapView {
	v := &WrapView{
		direction: layout.DirectionVertical,
		children:  compactViews(children),
	}
	v.Self = v
	return v
}

// Spacing sets the gap between children. Negative and non-finite values are
// treated as zero by the widget.
func (v *WrapView) Spacing(spacing float32) *WrapView {
	v.spacing = spacing
	return v
}

// LineSpacing sets the gap between rows or columns, independently of Spacing.
func (v *WrapView) LineSpacing(spacing float32) *WrapView {
	v.lineSpacing = spacing
	return v
}

// Padding sets the box's inner padding (a layout scalar, not style).
// Negative and non-finite values are treated as zero by the widget.
func (v *WrapView) Padding(padding float32) *WrapView {
	v.padding = padding
	return v
}

// MainAlign sets how children are packed along the main axis (the flow
// direction) within each line: Start / Center / End / SpaceBetween.
func (v *WrapView) MainAlign(align layout.MainAlign) *WrapView {
	v.mainAlign = align
	return v
}

// CrossAlign sets how each child sits on the cross axis. CrossDefault (also
// used when unset) means Center in HWrap and Start in VWrap. CrossBaseline applies
// to horizontal boxes and falls back to Start in vertical boxes. CrossStretch
// fills the line's cross extent; the other policies preserve the child's natural size.
func (v *WrapView) CrossAlign(align layout.CrossAlign) *WrapView {
	v.crossAlign = align
	return v
}

func (v *WrapView) Children(children ...View) *WrapView {
	v.children = compactViews(children)
	return v
}

func (v *WrapView) Build() View {
	return v
}

func (v *WrapView) Mount(BuildContext) gui.Widget {
	return gui.NewWrapBox(v.direction)
}

func (v *WrapView) Update(ctx BuildContext, widget gui.Widget) {
	box := widget.(*gui.WrapBox)
	box.SetDirection(v.direction)
	box.SetSpacing(v.spacing)
	box.SetLineSpacing(v.lineSpacing)
	box.SetPadding(v.padding)
	box.SetMainAlign(v.mainAlign)
	box.SetCrossAlign(v.crossAlign)
	ctx.UpdateChildren(box, v.children)
}

func (v *WrapView) Unmount(BuildContext, gui.Widget) {}
