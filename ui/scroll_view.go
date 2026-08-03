package ui

import (
	"github.com/golang-gui/goui/gui"
)

// ScrollViewView is the declarative wrapper of gui.ScrollView. It hosts a
// single scrollable child — either a gui.Scrollable (e.g. ui.ListView) or an
// ordinary widget tree (viewport mode). The child is a single content slot
// (gui.Bin), reconciled like ui.Button.
type ScrollViewView struct {
	ViewBase[ScrollViewView]
	child View
}

// ScrollView creates an empty scroll view. Set the content with Child.
func ScrollView(child View) *ScrollViewView {
	v := &ScrollViewView{}
	v.Self = v
	v.child = child
	return v
}

// Child sets the scrollable content (a ui.ListView or any widget tree).
func (v *ScrollViewView) Child(child View) *ScrollViewView {
	v.child = child
	return v
}

func (v *ScrollViewView) Build() View {
	return v
}

func (v *ScrollViewView) Mount(BuildContext) gui.Widget {
	return gui.NewScrollView()
}

func (v *ScrollViewView) Update(ctx BuildContext, widget gui.Widget) {
	if v.child == nil {
		ctx.UpdateChildren(widget, nil)
	} else {
		ctx.UpdateChildren(widget, []View{v.child})
	}
}

func (v *ScrollViewView) Unmount(BuildContext, gui.Widget) {}
