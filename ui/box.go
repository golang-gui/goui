package ui

import (
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

type BoxView struct {
	ViewBase[BoxView]
	direction layout.Direction
	spacing   float32
	children  []View
}

func HBox(children ...View) *BoxView {
	v := &BoxView{
		direction: layout.DirectionHorizontal,
		children:  compactViews(children),
	}
	v.Self = v
	return v
}

func VBox(children ...View) *BoxView {
	v := &BoxView{
		direction: layout.DirectionVertical,
		children:  compactViews(children),
	}
	v.Self = v
	return v
}

func (v *BoxView) Spacing(spacing float32) *BoxView {
	v.spacing = spacing
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
	ctx.UpdateChildren(box, v.children)
}

func (v *BoxView) Unmount(BuildContext, gui.Widget) {}
