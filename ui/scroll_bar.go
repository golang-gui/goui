package ui

import (
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/internal/identity"
	"github.com/golang-gui/goui/layout"
)

// ScrollHost is the capability required of an external scrollbar's target.
type ScrollHost interface {
	BindScrollBar(layout.Direction, gui.ScrollBarClient) signal.Handle
}

// ScrollBarView declares an independently laid out scrollbar associated with
// a persistent Target. Scroll state comes exclusively from the target host.
type ScrollBarView struct {
	ViewBase[ScrollBarView]
	target      string
	orientation layout.Direction
	thickness   float32
	thumb       float32
}

type scrollBarViewState struct {
	bar      *gui.ScrollBar
	host     ScrollHost
	axis     layout.Direction
	binding  signal.Handle
	revision uint64
	released bool
}

// ScrollBar creates a control for one axis of the target Widget ID. An empty
// targetID leaves it unbound with no range. Target can override this declaration.
// Built-in bars are hidden through the host's HScrollBarOptions or VScrollBarOptions.
func ScrollBar(orientation layout.Direction, targetID string) *ScrollBarView {
	if orientation != layout.DirectionHorizontal && orientation != layout.DirectionVertical {
		panic("ui: invalid scrollbar orientation")
	}
	v := &ScrollBarView{orientation: orientation, target: targetID}
	v.Self = v
	return v
}

// Target overrides the target Widget ID in this UI Root. The association follows
// local output replacement without rebuilding the bar. Empty clears it.
func (v *ScrollBarView) Target(id string) *ScrollBarView { v.target = id; return v }

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
	ctx.SetState(&scrollBarViewState{bar: b, axis: v.orientation})
	return b
}

func (v *ScrollBarView) Update(ctx BuildContext, widget gui.Widget) {
	b := widget.(*gui.ScrollBar)
	b.SetOrientation(v.orientation)
	b.SetThickness(v.thickness)
	b.SetThumbThickness(v.thumb)
	s := ctx.State().(*scrollBarViewState)
	BindTarget(ctx, "scroll-target", v.target, func(host ScrollHost) { s.setTarget(host, v.orientation) })
}

func (s *scrollBarViewState) setTarget(host ScrollHost, axis layout.Direction) {
	if s.released || identity.SamePointer(s.host, host) && s.axis == axis {
		return
	}
	revision := s.revision + 1
	s.disconnect()
	if s.revision != revision || s.released || s.bar.Destroyed() {
		return
	}
	s.host, s.axis = host, axis
	if host != nil {
		binding := host.BindScrollBar(axis, s.bar)
		if s.revision != revision || s.released || s.bar.Destroyed() {
			if binding != nil {
				binding.Disconnect()
			}
			return
		}
		s.binding = binding
	}
}

func (s *scrollBarViewState) disconnect() {
	s.revision++
	revision := s.revision
	binding := s.binding
	s.host, s.binding = nil, nil
	if binding != nil {
		binding.Disconnect()
	}
	if s.revision != revision || s.bar.Destroyed() {
		return
	}
	s.bar.SetMax(0)
	if s.revision != revision || s.bar.Destroyed() {
		return
	}
	s.bar.SetPageSize(0)
	if s.revision == revision && !s.bar.Destroyed() {
		s.bar.SetValue(0)
	}
}

func (*ScrollBarView) Unmount(ctx BuildContext, _ gui.Widget) {
	s := ctx.State().(*scrollBarViewState)
	s.released = true
	s.disconnect()
}
