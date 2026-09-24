package ui

import (
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/optional"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

// HeaderBarView binds gui.HeaderBar. GUI owns layout, controls avoidance and
// automatic drag classification; this view only reconciles content and options.
type HeaderBarView struct {
	ViewBase[HeaderBarView]
	child      View
	padding    optional.Optional[float32]
	captionIDs []string
}

type headerBarState struct {
	initPadding float32
	captionIDs  []string
	drag        signal.Handle
}

// HeaderBar hosts one optional child, without inserting a title or controls.
// Unset size and padding modifiers preserve the GUI constructor defaults.
func HeaderBar(child View) *HeaderBarView {
	v := &HeaderBarView{child: child}
	v.Self = v
	return v
}

func (v *HeaderBarView) Child(child View) *HeaderBarView {
	v.child = child
	return v
}

// Padding sets inner padding in DIP. Unset restores the GUI default; negative
// and non-finite values are normalized by gui.HeaderBar.
func (v *HeaderBarView) Padding(padding float32) *HeaderBarView {
	v.padding.SetValue(padding)
	return v
}

// Captions treats widgets with these IDs as caption regions when they are the
// actual hit target. It does not match ancestors or other HeaderBars. A match
// explicitly overrides the automatic classification, even for interactive
// widgets. Repeated calls replace the list; an empty list leaves only the
// automatic classification. IDs are not window title text.
func (v *HeaderBarView) Captions(ids ...string) *HeaderBarView {
	v.captionIDs = slices.Clone(ids)
	return v
}

func (v *HeaderBarView) Build() View { return v }

func (v *HeaderBarView) Mount(ctx BuildContext) gui.Widget {
	header := gui.NewHeaderBar()
	state := &headerBarState{initPadding: header.Padding()}
	state.drag = header.ConnectDragRegion(func(point geometry.Point, drag *bool) {
		state.extendDragRegion(header, point, drag)
	})
	ctx.SetState(state)
	return header
}

func (v *HeaderBarView) Update(ctx BuildContext, widget gui.Widget) {
	header := widget.(*gui.HeaderBar)
	state := ctx.State().(*headerBarState)
	state.captionIDs = v.captionIDs
	if v.padding.HasValue() {
		header.SetPadding(v.padding.Value())
	} else {
		header.SetPadding(state.initPadding)
	}
	ctx.UpdateChild(header, v.child)
}

func (v *HeaderBarView) Unmount(ctx BuildContext, _ gui.Widget) {
	state, _ := ctx.State().(*headerBarState)
	if state != nil {
		if state.drag != nil {
			state.drag.Disconnect()
			state.drag = nil
		}
		state.captionIDs = nil
	}
}

func (s *headerBarState) extendDragRegion(header *gui.HeaderBar, point geometry.Point, drag *bool) {
	if *drag || len(s.captionIDs) == 0 {
		return
	}
	// GUI has already excluded covering siblings and window controls before
	// emitting this query. Pick uses the same HeaderBar-local coordinates.
	if target := gui.Pick(header, point); target != nil {
		if id := target.ID(); id != "" && slices.Contains(s.captionIDs, id) {
			*drag = true
		}
	}
}
