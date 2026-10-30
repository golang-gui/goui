package ui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

type MenuButtonGroupView struct {
	ViewBase[MenuButtonGroupView]
	spacing    float32
	padding    geometry.Insets
	mainAlign  layout.MainAlign
	crossAlign layout.CrossAlign
	children   []View
}

// MenuButtonGroup arranges children horizontally. Descendant menu buttons
// automatically switch within the nearest group; ordinary buttons stay independent.
func MenuButtonGroup(children ...View) *MenuButtonGroupView {
	v := &MenuButtonGroupView{children: compactViews(children)}
	v.Self = v
	return v
}

// Spacing sets the gap between children. Negative and non-finite values are
// treated as zero by the widget.
func (v *MenuButtonGroupView) Spacing(spacing float32) *MenuButtonGroupView {
	v.spacing = spacing
	return v
}

// Padding sets equal inner padding on every edge (a layout field, not style).
// Negative and non-finite values are treated as zero by the widget.
func (v *MenuButtonGroupView) Padding(padding float32) *MenuButtonGroupView {
	v.padding = geometry.UniformInsets(padding)
	return v
}

// Paddings sets the per-edge inner padding (a layout field, not style).
// Negative and non-finite edges are treated as zero by the widget.
func (v *MenuButtonGroupView) Paddings(padding geometry.Insets) *MenuButtonGroupView {
	v.padding = padding
	return v
}

// MainAlign sets how children are packed along the main axis (the flow
// direction): Start / Center / End / SpaceBetween. Container-level.
func (v *MenuButtonGroupView) MainAlign(align layout.MainAlign) *MenuButtonGroupView {
	v.mainAlign = align
	return v
}

// CrossAlign sets vertical alignment within the horizontal group. CrossDefault
// centers children; CrossStretch fills the height and CrossBaseline aligns text.
func (v *MenuButtonGroupView) CrossAlign(align layout.CrossAlign) *MenuButtonGroupView {
	v.crossAlign = align
	return v
}

func (v *MenuButtonGroupView) Children(children ...View) *MenuButtonGroupView {
	v.children = compactViews(children)
	return v
}

func (v *MenuButtonGroupView) Mount(BuildContext) gui.Widget {
	return gui.NewMenuButtonGroup()
}

func (v *MenuButtonGroupView) Update(ctx BuildContext, widget gui.Widget) {
	box := widget.(*gui.MenuButtonGroup)
	box.SetSpacing(v.spacing)
	box.SetPadding(v.padding)
	box.SetMainAlign(v.mainAlign)
	box.SetCrossAlign(v.crossAlign)
	ctx.UpdateChildren(box, v.children)
}

func (v *MenuButtonGroupView) Unmount(BuildContext, gui.Widget) {}
