package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

type splitHandle struct {
	gui.WidgetBase
	view              *SplitView
	drag              *gui.DragEventController
	hovered, dragging bool
	before            splitPreference
	grabOffset        float32
}

func newSplitHandle(view *SplitView) *splitHandle {
	h := &splitHandle{view: view}
	h.SetStyleName("split-handle")
	h.SetFocusable(true)
	h.SetVisible(false)
	h.updateCursor()
	motion := gui.NewMotionEventController()
	motion.ConnectHover(func(hovered bool) { h.hovered = hovered; h.RequestPaint() })
	h.AddEventController(motion)
	h.ConnectFocused(func(bool) { h.RequestPaint() })
	h.drag = gui.NewDragEventController()
	h.drag.ConnectBegin(func(point geometry.Point, _ events.Modifiers) {
		if view.Destroyed() || !view.active() {
			h.drag.Reset()
			return
		}
		h.dragging, h.before = true, view.preference
		h.grabOffset = view.point(point.Add(h.Rect().Pos)) - view.startSize
		h.RequestPaint()
	})
	h.drag.ConnectUpdate(func(point geometry.Point, _ events.Modifiers) { h.move(point) })
	h.drag.ConnectEnd(func(point geometry.Point, _ events.Modifiers) {
		h.move(point)
		if h.Destroyed() {
			return
		}
		h.dragging = false
		h.RequestPaint()
	})
	h.drag.ConnectCancel(h.cancel)
	h.AddEventController(h.drag)
	h.ConnectUnmount(h.cancel)
	key := gui.NewKeyEventController()
	key.ConnectKeyDown(h.keyDown)
	h.AddEventController(key)
	return h
}
func (h *splitHandle) updateCursor() {
	cursor := gui.CursorResizeHorizontal
	if h.view.direction == layout.DirectionVertical {
		cursor = gui.CursorResizeVertical
	}
	h.SetCursor(cursor)
}
func (h *splitHandle) move(point geometry.Point) {
	if h.dragging {
		h.view.adjust(h.view.point(point.Add(h.Rect().Pos)) - h.grabOffset)
	}
}
func (h *splitHandle) cancel() {
	if !h.dragging {
		return
	}
	h.dragging = false // Reset may recursively deliver ConnectCancel.
	h.drag.Reset()
	v := h.view
	if v.Destroyed() || h.Destroyed() {
		return
	}
	v.preference = h.before
	if v.active() {
		a := v.available()
		position := splitAllocate(a, v.preference, 0, 0, v.start.limits(v), v.end.limits(v))
		changed := position != v.startSize
		v.startSize, v.endSize = position, a-position
		v.RequestLayout()
		if changed {
			v.emitResize()
		}
	} else {
		v.RequestLayout()
	}
	if !h.Destroyed() {
		h.RequestPaint()
	}
}
func (h *splitHandle) keyDown(ctx gui.EventContext, event events.KeyEvent) {
	v := h.view
	if v.Destroyed() || !v.active() {
		return
	}
	if (gui.KeyGesture{Key: gui.KeyEscape}).Matches(event) && h.dragging {
		ctx.StopPropagation()
		event.PreventDefault()
		h.cancel()
		return
	}
	decrease, increase := gui.KeyArrowLeft, gui.KeyArrowRight
	if v.direction == layout.DirectionVertical {
		decrease, increase = gui.KeyArrowUp, gui.KeyArrowDown
	}
	for _, modifiers := range []gui.KeyModifiers{0, gui.ModShift} {
		step := float32(1)
		if modifiers != 0 {
			step = 10
		}
		for _, action := range []struct {
			key   gui.Key
			value float32
		}{
			{decrease, v.startSize - step}, {increase, v.startSize + step},
		} {
			if (gui.KeyGesture{Key: action.key, Modifiers: modifiers}).Matches(event) {
				ctx.StopPropagation()
				event.PreventDefault()
				v.adjust(action.value)
				return
			}
		}
	}
	lo, hi := v.limits(v.available())
	for _, action := range []struct {
		key   gui.Key
		value float32
	}{{gui.KeyHome, lo}, {gui.KeyEnd, hi}} {
		if (gui.KeyGesture{Key: action.key}).Matches(event) {
			ctx.StopPropagation()
			event.PreventDefault()
			v.adjust(action.value)
			return
		}
	}
}
func (h *splitHandle) Paint(p gui.Painter) {
	const width float32 = 1
	state := style.Normal
	switch {
	case h.dragging:
		state = style.Pressed
	case h.hovered:
		state = style.Hovered
	}
	// Keyboard focus does not keep the pointer's visual feedback active.
	s := gui.ResolveStyle(h.StyleName(), "", state)
	ink, ok := s.ForegroundColor()
	if !ok || ink == nil {
		return
	}
	// Use the real gap center, even when the hit rectangle is clipped near an
	// edge. A centered line avoids the half-pixel inset of an outlined shape.
	center := h.view.startSize + .5 - h.view.point(h.Rect().Pos)
	var rect geometry.Rectangle
	if h.view.direction == layout.DirectionVertical {
		rect = geometry.Rect(0, center-width/2, h.Rect().Width, width)
	} else {
		rect = geometry.Rect(center-width/2, 0, width, h.Rect().Height)
	}
	p.FillRect(rect, graphics.ColorOf(ink))
}
func (h *splitHandle) Snapshot() gui.WidgetInfo {
	info := h.WidgetBase.Snapshot()
	info.Role = RoleSeparator
	info.Enabled = info.Enabled && h.view.active()
	lo, hi := h.view.limits(h.view.available())
	info.Range = &gui.RangeInfo{Value: h.view.startSize, Min: lo, Max: hi, Direction: h.view.direction}
	if info.Enabled {
		info.Actions = []gui.Action{gui.ActionFocus}
	}
	return info
}
