package ui

import (
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
)

type BoxView struct {
	name      string
	hidden    bool
	direction layout.Direction
	spacing   float32
	children  []View
	rules     []style.Rule
}

func HBox(children ...View) BoxView {
	return BoxView{
		direction: layout.DirectionHorizontal,
		children:  compactViews(children),
	}
}

func VBox(children ...View) BoxView {
	return BoxView{
		direction: layout.DirectionVertical,
		children:  compactViews(children),
	}
}

func (v BoxView) Name(name string) BoxView {
	v.name = name
	return v
}

func (v BoxView) Hidden(hidden bool) BoxView {
	v.hidden = hidden
	return v
}

func (v BoxView) Spacing(spacing float32) BoxView {
	v.spacing = spacing
	return v
}

func (v BoxView) Children(children ...View) BoxView {
	v.children = compactViews(children)
	return v
}

func (v BoxView) Style(rules ...style.Rule) BoxView {
	v.rules = rules
	return v
}

func (v BoxView) Build() View {
	return v
}

func (v BoxView) Mount(BuildContext) gui.Widget {
	return gui.NewLinearBox(v.direction)
}

func (v BoxView) Update(ctx BuildContext, widget gui.Widget) {
	box := widget.(*gui.LinearBox)
	box.SetID(v.name)
	box.SetVisible(!v.hidden)
	box.SetStyleRules(v.rules...)
	box.SetDirection(v.direction)
	box.SetSpacing(v.spacing)
	ctx.UpdateChildren(box, v.children)
}

func (v BoxView) Unmount(BuildContext, gui.Widget) {}
