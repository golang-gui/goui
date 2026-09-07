package ui

import (
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

type BoxView struct {
	ViewBase[BoxView]
	direction  layout.Direction
	spacing    float32
	padding    float32
	mainAlign  layout.MainAlign
	crossAlign layout.CrossAlign
	children   []View
}

// HBox arranges children horizontally. By default it packs from the start and
// centers children vertically (CrossDefault), with no spacing or padding.
func HBox(children ...View) *BoxView {
	v := &BoxView{
		direction: layout.DirectionHorizontal,
		children:  compactViews(children),
	}
	v.Self = v
	return v
}

// VBox arranges children vertically. By default it packs from the start and
// aligns children to the left (CrossDefault), with no spacing or padding.
func VBox(children ...View) *BoxView {
	v := &BoxView{
		direction: layout.DirectionVertical,
		children:  compactViews(children),
	}
	v.Self = v
	return v
}

// Spacing sets the gap between children. Negative and non-finite values are
// treated as zero by the widget.
func (v *BoxView) Spacing(spacing float32) *BoxView {
	v.spacing = spacing
	return v
}

// Padding sets the box's inner padding (a layout scalar, not style).
// Negative and non-finite values are treated as zero by the widget.
func (v *BoxView) Padding(padding float32) *BoxView {
	v.padding = padding
	return v
}

// MainAlign sets how children are packed along the main axis (the flow
// direction): Start / Center / End / SpaceBetween. Container-level.
func (v *BoxView) MainAlign(align layout.MainAlign) *BoxView {
	v.mainAlign = align
	return v
}

// CrossAlign sets how each child sits on the cross axis. CrossDefault (also
// used when unset) means Center in HBox and Start in VBox. CrossBaseline applies
// to horizontal boxes and falls back to Start in vertical boxes. CrossStretch
// fills the cross axis; the other policies preserve the child's natural size.
func (v *BoxView) CrossAlign(align layout.CrossAlign) *BoxView {
	v.crossAlign = align
	return v
}

func (v *BoxView) Children(children ...View) *BoxView {
	v.children = compactViews(children)
	return v
}

func (v *BoxView) Build() View {
	return v
}

func (v *BoxView) Mount(BuildContext) gui.Widget {
	return gui.NewLinearBox(v.direction)
}

func (v *BoxView) Update(ctx BuildContext, widget gui.Widget) {
	box := widget.(*gui.LinearBox)
	box.SetDirection(v.direction)
	box.SetSpacing(v.spacing)
	box.SetPadding(v.padding)
	box.SetMainAlign(v.mainAlign)
	box.SetCrossAlign(v.crossAlign)
	ctx.UpdateChildren(box, v.children)
}

func (v *BoxView) Unmount(BuildContext, gui.Widget) {}
