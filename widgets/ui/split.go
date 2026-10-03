package ui

import (
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// SplitViewView coordinates two ordinary Bin targets. Without a sizing
// declaration user adjustments survive rebuilding; with one, OnResize should
// update the application's sizing value before RequestUpdate (controlled use).
type SplitViewView struct {
	baseui.ViewBase[SplitViewView]
	direction  layout.Direction
	start, end baseui.View
	sizing     uint8
	value      float32
	onResize   func(float32, float32)
}

// HSplit divides space left/right; VSplit divides it top/bottom.
func HSplit(start, end baseui.View) *SplitViewView {
	return newSplitView(layout.DirectionHorizontal, start, end)
}
func VSplit(start, end baseui.View) *SplitViewView {
	return newSplitView(layout.DirectionVertical, start, end)
}
func newSplitView(direction layout.Direction, start, end baseui.View) *SplitViewView {
	v := &SplitViewView{direction: direction, start: start, end: end}
	v.Self = v
	return v
}
func (v *SplitViewView) Start(child baseui.View) *SplitViewView { v.start = child; return v }
func (v *SplitViewView) End(child baseui.View) *SplitViewView   { v.end = child; return v }

// StartSize, EndSize and Ratio are mutually exclusive: the last setter wins.
func (v *SplitViewView) StartSize(size float32) *SplitViewView { v.sizing, v.value = 1, size; return v }
func (v *SplitViewView) EndSize(size float32) *SplitViewView   { v.sizing, v.value = 2, size; return v }
func (v *SplitViewView) Ratio(ratio float32) *SplitViewView    { v.sizing, v.value = 3, ratio; return v }
func (v *SplitViewView) OnResize(fn func(start, end float32)) *SplitViewView {
	v.onResize = fn
	return v
}
func (v *SplitViewView) Build() baseui.View { return v }

type splitSlot struct {
	view  *widgets.SplitView
	start bool
}

func (s *splitSlot) SetChild(child gui.Widget) {
	if s.start {
		s.view.SetStartChild(child)
	} else {
		s.view.SetEndChild(child)
	}
}

type splitState struct {
	start, end *splitSlot
	connection signal.Handle
	onResize   func(float32, float32)
}

func (v *SplitViewView) Mount(ctx baseui.BuildContext) gui.Widget {
	view := widgets.NewSplitView(v.direction)
	state := &splitState{start: &splitSlot{view: view, start: true}, end: &splitSlot{view: view}}
	state.connection = view.ConnectResize(func(start, end float32) {
		if state.onResize != nil {
			state.onResize(start, end)
		}
	})
	ctx.SetState(state)
	return view
}
func (v *SplitViewView) Update(ctx baseui.BuildContext, widget gui.Widget) {
	view, state := widget.(*widgets.SplitView), ctx.State().(*splitState)
	state.onResize = v.onResize
	view.SetDirection(v.direction)
	switch v.sizing {
	case 1:
		view.SetStartSize(v.value)
	case 2:
		view.SetEndSize(v.value)
	case 3:
		view.SetRatio(v.value)
	}
	ctx.UpdateChild(state.start, v.start)
	ctx.UpdateChild(state.end, v.end)
}
func (*SplitViewView) Unmount(ctx baseui.BuildContext, _ gui.Widget) {
	state := ctx.State().(*splitState)
	state.connection.Disconnect()
	state.onResize = nil
}
