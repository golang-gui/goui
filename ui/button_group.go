package ui

import (
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

// ButtonGroupView declares a connected strip without changing button behavior.
type ButtonGroupView struct {
	ViewBase[ButtonGroupView]
	direction layout.Direction
	children  []View
}

// ButtonGroup connects direct button children horizontally. Use Direction for
// a vertical strip. Spacing is always zero; each button retains its own style.
func ButtonGroup(children ...View) *ButtonGroupView {
	v := &ButtonGroupView{children: compactViews(children)}
	v.Self = v
	return v
}
func (v *ButtonGroupView) Direction(direction layout.Direction) *ButtonGroupView {
	v.direction = direction
	return v
}
func (v *ButtonGroupView) Children(children ...View) *ButtonGroupView {
	v.children = compactViews(children)
	return v
}
func (*ButtonGroupView) Mount(BuildContext) gui.Widget { return gui.NewButtonGroup() }
func (v *ButtonGroupView) Update(ctx BuildContext, widget gui.Widget) {
	group := widget.(*gui.ButtonGroup)
	group.SetDirection(v.direction)
	ctx.UpdateChildren(group, v.children)
}
func (*ButtonGroupView) Unmount(BuildContext, gui.Widget) {}
