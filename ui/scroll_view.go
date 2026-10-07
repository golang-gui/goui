package ui

import (
	"github.com/golang-gui/goui/gui"
)

type ScrollDirection = gui.ScrollDirection

const (
	ScrollBoth       = gui.ScrollBoth
	ScrollVertical   = gui.ScrollVertical
	ScrollHorizontal = gui.ScrollHorizontal
)

// ScrollViewView is the declarative wrapper of gui.ScrollView. It hosts a
// single scrollable child — either a gui.Scrollable (e.g. ui.ListView) or an
// ordinary widget tree (viewport mode). The child is a single content slot
// (gui.Bin), reconciled like ui.Button.
type ScrollViewView struct {
	ViewBase[ScrollViewView]
	child          View
	direction      ScrollDirection
	dragAutoScroll bool
}

// ScrollView wraps content in a viewport. Its default MainWeight is 1;
// MainWeight(0) opts out of sharing a linear parent's remaining space.
func ScrollView(child View) *ScrollViewView {
	v := &ScrollViewView{dragAutoScroll: true}
	v.Self = v
	v.child = child
	return v
}

func (v *ScrollViewView) DragAutoScroll(enabled bool) *ScrollViewView {
	v.dragAutoScroll = enabled
	return v
}

// Direction selects the scrolling axes. A single-axis viewport measures an
// ordinary child at its available cross-axis size, for example wrapping a
// form to the viewport width with ScrollVertical. The default is ScrollBoth.
func (v *ScrollViewView) Direction(direction ScrollDirection) *ScrollViewView {
	v.direction = direction
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
	widget.(*gui.ScrollView).SetDirection(v.direction)
	widget.(*gui.ScrollView).SetDragAutoScroll(v.dragAutoScroll)
	ctx.UpdateChild(widget.(*gui.ScrollView), v.child)
}

func (v *ScrollViewView) Unmount(BuildContext, gui.Widget) {}
