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
	view       *SplitView
	drag       *gui.DragEventController
	dragging   bool
	before     splitPreference
	grabOffset float32
}

func newSplitHandle(view *SplitView) *splitHandle {
	h := &splitHandle{view: view}
	h.SetFocusable(true)
	h.SetVisible(false)
	h.updateCursor()
	motion := gui.NewMotionEventController()
	h.AddEventController(motion)
	h.ConnectFocused(func(bool) { h.RequestPaint() })
	h.drag = gui.NewDragEventController()
	h.connectDrag()
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

func (h *splitHandle) connectDrag() {
	view, drag := h.view, h.drag
	drag.ConnectBegin(func(_ gui.EventContext, info gui.DragInfo) {
		if view.Destroyed() || !view.active() {
			drag.Reset()
			return
		}
		h.dragging, h.before = true, view.preference
		h.grabOffset = view.point(info.Position.Add(h.Rect().Pos)) - view.startSize
		h.SetState(style.Pressed, true)
		if host, ok := view.Root().(gui.EventTarget); ok {
			host.SetFocusedWidget(h)
		}
		if h.Destroyed() || view.Destroyed() || !view.active() || !h.dragging {
			drag.Reset()
			return
		}
		h.RequestPaint()
	})
	move := func(info gui.DragInfo) {
		if h.dragging {
			view.adjust(view.point(info.Position.Add(h.Rect().Pos)) - h.grabOffset)
		}
	}
	drag.ConnectUpdate(func(_ gui.EventContext, info gui.DragInfo) { move(info) })
	drag.ConnectEnd(func(_ gui.EventContext, info gui.DragInfo) {
		move(info)
		if h.Destroyed() {
			return
		}
		h.dragging = false
		h.SetState(style.Pressed, false)
		h.RequestPaint()
	})
	drag.ConnectCanceled(func(gui.GestureCancelReason) { h.cancel() })
}

func (h *splitHandle) cancel() {
	wasDragging := h.dragging
	h.dragging = false // Reset may recursively deliver ConnectCanceled.
	h.drag.Reset()
	if !wasDragging {
		return
	}
	h.SetState(style.Pressed, false)
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
func (h *splitHandle) paintState() style.State {
	state := h.State() &^ (style.Focused | style.FocusVisible | style.ContainsFocus)
	if !gui.IsEnabled(h) {
		return style.Disabled
	}
	if h.FocusVisible() {
		state |= style.Focused | style.FocusVisible
	}
	return state
}

func (v *SplitView) partStyle(part string, state style.State) style.Style {
	name := v.StyleName()
	if name == "" {
		name = "split-view"
	}
	return gui.ResolveWidgetStyle(v, name, part, state)
}

func (h *splitHandle) Paint(p gui.Painter) {
	v, state := h.view, h.paintState()
	main, cross := v.axis(h.Rect().Size), v.cross(h.Rect().Size)
	if main <= 0 || cross <= 0 {
		return
	}
	// The halves share one boundary and are painted once, without a common
	// background underneath. Transparent parts retain normal source-over rules.
	region := func(pos, extent float32) geometry.Rectangle {
		r := geometry.Rectangle{Size: v.size(extent, cross)}
		if v.direction == layout.DirectionVertical {
			r.Y = pos
		} else {
			r.X = pos
		}
		return r
	}
	for i, part := range []string{"handle-start", "handle-end"} {
		if ink, ok := v.partStyle(part, state).BackgroundColor(); ok && ink != nil {
			p.FillRect(region(float32(i)*main/2, main/2), graphics.ColorOf(ink))
		}
	}
	s := v.partStyle("separator", state)
	width, _ := s.BorderWidth()
	if !splitFinite(width) || width <= 0 {
		return
	}
	width = min(width, main)
	if ink, ok := s.ForegroundColor(); ok && ink != nil {
		p.FillRect(region((main-width)/2, width), graphics.ColorOf(ink))
	}
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
