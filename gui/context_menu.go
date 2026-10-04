package gui

import (
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/events"
)

// ContextMenuEventController interprets pointer and keyboard requests. It does
// not select items or show a menu. A listener consumes the request with
// StopPropagation. Position is widget-local DIP for pointer requests and absent
// for keyboard requests. The controller defaults to Bubble phase.
type ContextMenuEventController struct {
	EventControllerBase
	goos       string
	armed      bool
	press      geometry.Point
	button     events.PointerButton
	request    signal.Signal2[EventContext, uint64]
	generation uint64
}

func NewContextMenuEventController() *ContextMenuEventController {
	return &ContextMenuEventController{EventControllerBase: NewEventControllerBase(PhaseBubble), goos: runtime.GOOS}
}

func (c *ContextMenuEventController) ConnectRequest(fn func(EventContext)) signal.Handle {
	return c.request.Connect(func(ctx EventContext, generation uint64) {
		if generation != c.generation {
			return
		}
		if e, ok := ctx.(*eventContext); ok && e.alive != nil && !e.alive() {
			return
		}
		fn(ctx)
	})
}

func (c *ContextMenuEventController) Reset()           { c.armed = false; c.generation++ }
func (c *ContextMenuEventController) setWidget(Widget) { c.Reset() }

func contextMenuPointer(e events.PointerEvent, goos string) bool {
	return e.Button == events.PointerButtonRight || goos == "darwin" &&
		e.Button == events.PointerButtonLeft && e.Modifiers&events.ModifierControl != 0
}

func primaryPointer(e events.PointerEvent) bool {
	return e.Button == events.PointerButtonLeft && !contextMenuPointer(e, runtime.GOOS)
}

func (c *ContextMenuEventController) HandleEvent(ctx EventContext) {
	switch e := ctx.Event().(type) {
	case events.PointerEvent:
		switch e.EventType {
		case events.PointerDown:
			c.Reset()
			if !contextMenuPointer(e, c.goos) {
				return
			}
			if c.goos == "windows" {
				c.armed, c.press, c.button = true, e.Position, e.Button
				return
			}
			c.request.Emit(ctx, c.generation)
		case events.PointerMove:
			if c.armed && gestureMoved(e.Position, c.press, gestureDefaultDistance) {
				c.Reset()
			}
		case events.PointerUp:
			armed := c.armed && e.Button == c.button && !gestureMoved(e.Position, c.press, gestureDefaultDistance)
			c.Reset()
			if armed {
				c.request.Emit(ctx, c.generation)
			}
		case events.PointerLeave:
			c.Reset()
		}
	case events.KeyEvent:
		if e.EventType == events.KeyDown && !e.Repeat && (e.Key == events.KeyF10 && e.Modifiers == events.ModifierShift || e.Key == events.KeyMenu && e.Modifiers == 0) {
			c.request.Emit(ctx, c.generation)
			if ctx.PropagationStopped() {
				e.PreventDefault()
			}
		}
	}
}

func (c *ContextMenuEventController) HandleCrossing(ctx CrossingContext) {
	if ctx.Direction() == CrossingLeave {
		c.Reset()
	}
}
