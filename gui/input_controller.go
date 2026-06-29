package gui

import (
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/events"
)

type HoverEventController struct {
	hovered bool
	enter   signal.Signal0
	leave   signal.Signal0
}

func NewHoverEventController() *HoverEventController {
	return new(HoverEventController)
}

func (c *HoverEventController) Phase() PropagationPhase {
	return PhaseTarget
}

func (c *HoverEventController) Reset() {
	c.setHovered(false)
}

func (c *HoverEventController) Hovered() bool {
	return c.hovered
}

func (c *HoverEventController) ConnectEnter(fn func()) signal.Handle {
	return c.enter.Connect(fn)
}

func (c *HoverEventController) ConnectLeave(fn func()) signal.Handle {
	return c.leave.Connect(fn)
}

func (c *HoverEventController) HandleEvent(ctx EventContext, event events.Event) {
	pointerEvent, ok := event.(events.PointerEvent)
	if !ok {
		return
	}

	switch pointerEvent.EventType {
	case events.PointerEnter:
		c.setHovered(true)
	case events.PointerLeave:
		c.setHovered(false)
	}
}

func (c *HoverEventController) setHovered(hovered bool) {
	if c.hovered == hovered {
		return
	}
	c.hovered = hovered
	if hovered {
		c.enter.Emit()
	} else {
		c.leave.Emit()
	}
}

type ClickEventController struct {
	phase   PropagationPhase
	button  events.PointerButton
	pressed bool
	press   signal.Signal2[EventContext, bool]
	clicked signal.Signal1[EventContext]
}

func NewClickEventController() *ClickEventController {
	return &ClickEventController{
		phase:  PhaseBubble,
		button: events.PointerButtonLeft,
	}
}

func (c *ClickEventController) Phase() PropagationPhase {
	return c.phase
}

func (c *ClickEventController) SetPhase(phase PropagationPhase) {
	c.phase = phase
}

func (c *ClickEventController) Button() events.PointerButton {
	return c.button
}

func (c *ClickEventController) SetButton(button events.PointerButton) {
	if c.button == button {
		return
	}
	c.Reset()
	c.button = button
}

func (c *ClickEventController) Pressed() bool {
	return c.pressed
}

func (c *ClickEventController) Reset() {
	c.pressed = false
}

func (c *ClickEventController) ConnectPressed(fn func(ctx EventContext, pressed bool)) signal.Handle {
	return c.press.Connect(fn)
}

func (c *ClickEventController) ConnectClicked(fn func(ctx EventContext)) signal.Handle {
	return c.clicked.Connect(fn)
}

func (c *ClickEventController) HandleEvent(ctx EventContext, event events.Event) {
	pointerEvent, ok := event.(events.PointerEvent)
	if !ok {
		return
	}

	switch pointerEvent.EventType {
	case events.PointerDown:
		if pointerEvent.Button != c.button {
			return
		}
		c.setPressed(ctx, true)
	case events.PointerUp:
		if pointerEvent.Button != c.button {
			return
		}
		if c.pressed {
			c.setPressed(ctx, false)
			c.clicked.Emit(ctx)
		}
	case events.PointerLeave:
		c.setPressed(ctx, false)
	}
}

func (c *ClickEventController) setPressed(ctx EventContext, pressed bool) {
	if c.pressed == pressed {
		return
	}
	c.pressed = pressed
	c.press.Emit(ctx, pressed)
}
