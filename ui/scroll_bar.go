package ui

import (
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

// ScrollBarView declares an independently laid out scrollbar associated with
// the nearest ScrollGroup. Scroll state comes exclusively from the group's host.
type ScrollBarView struct {
	ViewBase[ScrollBarView]
	orientation layout.Direction
	thickness   float32
	thumb       float32
}

type scrollBarViewState struct{ member *scrollGroupMember }

// ScrollBar creates a control for one axis of the nearest ScrollGroup's host.
// Outside a group it has no range. Built-in bars are hidden explicitly through
// the host's HScrollBarOptions or VScrollBarOptions.
func ScrollBar(orientation layout.Direction) *ScrollBarView {
	if orientation != layout.DirectionHorizontal && orientation != layout.DirectionVertical {
		panic("ui: invalid scrollbar orientation")
	}
	v := &ScrollBarView{orientation: orientation}
	v.Self = v
	return v
}

func (v *ScrollBarView) Orientation(orientation layout.Direction) *ScrollBarView {
	if orientation != layout.DirectionHorizontal && orientation != layout.DirectionVertical {
		panic("ui: invalid scrollbar orientation")
	}
	v.orientation = orientation
	return v
}

func (v *ScrollBarView) Thickness(thickness float32) *ScrollBarView {
	v.thickness = thickness
	return v
}

func (v *ScrollBarView) ThumbThickness(thickness float32) *ScrollBarView {
	v.thumb = thickness
	return v
}

func (v *ScrollBarView) Mount(ctx BuildContext) gui.Widget {
	b := gui.NewScrollBar(v.orientation)
	ctx.SetState(&scrollBarViewState{member: &scrollGroupMember{bar: b, axis: v.orientation}})
	return b
}

func (v *ScrollBarView) Update(ctx BuildContext, widget gui.Widget) {
	b := widget.(*gui.ScrollBar)
	b.SetOrientation(v.orientation)
	b.SetThickness(v.thickness)
	b.SetThumbThickness(v.thumb)
	v.applyGroup(ctx)
}

func (v *ScrollBarView) applyGroup(ctx BuildContext) {
	m := ctx.State().(*scrollBarViewState).member
	g, _ := FindAncestor[*scrollGroupState](ctx)
	if m.group == g && m.axis == v.orientation {
		return
	}
	m.Disconnect()
	m.axis = v.orientation
	if g != nil {
		g.add(m)
	}
}

func (*ScrollBarView) Unmount(ctx BuildContext, _ gui.Widget) {
	ctx.State().(*scrollBarViewState).member.Disconnect()
}

func (v *ScrollBarView) Reparented(ctx BuildContext) { v.applyGroup(ctx) }

var _ ReparentedView = (*ScrollBarView)(nil)
