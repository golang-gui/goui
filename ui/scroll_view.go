package ui

import (
	"github.com/golang-gui/goui/gui"
)

type ScrollDirection = gui.ScrollDirection
type ScrollBarOptions = gui.ScrollBarOptions
type ScrollBarPolicy = gui.ScrollBarPolicy

const (
	ScrollBoth       = gui.ScrollBoth
	ScrollVertical   = gui.ScrollVertical
	ScrollHorizontal = gui.ScrollHorizontal
	ScrollBarAuto    = gui.ScrollBarAuto
	ScrollBarAlways  = gui.ScrollBarAlways
	ScrollBarHidden  = gui.ScrollBarHidden
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
	hScrollBar     ScrollBarOptions
	vScrollBar     ScrollBarOptions
}

type scrollViewState struct {
	view  *gui.ScrollView
	group ScrollGroupBinding
}

// ScrollView wraps content in a viewport. Its default MainWeight is 1;
// MainWeight(0) opts out of sharing a linear parent's remaining space.
// ScrollView(TextView()).Style("text-view") includes both scrollbars inside
// the editor frame. An unstyled ScrollView remains transparent.
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

// HScrollBarOptions configures the built-in horizontal bar. Omission restores
// default geometry and Auto visibility on the next update.
func (v *ScrollViewView) HScrollBarOptions(options ScrollBarOptions) *ScrollViewView {
	v.hScrollBar = options
	return v
}

// VScrollBarOptions configures the built-in vertical bar. Hidden retains
// scrolling without reserving a built-in bar or spacing.
func (v *ScrollViewView) VScrollBarOptions(options ScrollBarOptions) *ScrollViewView {
	v.vScrollBar = options
	return v
}

// Child sets the scrollable content (a ui.ListView or any widget tree).
func (v *ScrollViewView) Child(child View) *ScrollViewView {
	v.child = child
	return v
}

func (v *ScrollViewView) Mount(ctx BuildContext) gui.Widget {
	view := gui.NewScrollView()
	ctx.SetState(&scrollViewState{view: view})
	return view
}

func (v *ScrollViewView) Update(ctx BuildContext, widget gui.Widget) {
	s := ctx.State().(*scrollViewState)
	widget.(*gui.ScrollView).SetDirection(v.direction)
	widget.(*gui.ScrollView).SetDragAutoScroll(v.dragAutoScroll)
	widget.(*gui.ScrollView).SetHScrollBarOptions(v.hScrollBar)
	widget.(*gui.ScrollView).SetVScrollBarOptions(v.vScrollBar)
	ctx.UpdateChild(widget.(*gui.ScrollView), v.child)
	if ctx.State() == s && !widget.(*gui.ScrollView).Destroyed() {
		s.group.Update(ctx, s.view)
	}
}

func (v *ScrollViewView) Unmount(ctx BuildContext, _ gui.Widget) {
	ctx.State().(*scrollViewState).group.Disconnect()
}

func (v *ScrollViewView) Reparented(ctx BuildContext) {
	s := ctx.State().(*scrollViewState)
	s.group.Update(ctx, s.view)
}
