package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/events"
)

type MotionInfo struct {
	Position  geometry.Point
	Modifiers events.Modifiers
}

type MotionEventController struct {
	phase                PropagationPhase
	isHover              bool
	containsHover        bool
	motion               signal.Signal1[MotionInfo]
	hoverChanged         signal.Signal1[bool]
	containsHoverChanged signal.Signal1[bool]
}

func NewMotionEventController() *MotionEventController {
	return &MotionEventController{
		phase: PhaseTarget,
	}
}

func (c *MotionEventController) Phase() PropagationPhase {
	return c.phase
}

func (c *MotionEventController) SetPhase(phase PropagationPhase) {
	c.phase = phase
}

func (c *MotionEventController) Reset() {
	c.setIsHover(false)
	c.setContainsHover(false)
}

func (c *MotionEventController) IsHover() bool {
	return c.isHover
}

func (c *MotionEventController) ContainsHover() bool {
	return c.containsHover
}

func (c *MotionEventController) ConnectMotion(fn func(MotionInfo)) signal.Handle {
	return c.motion.Connect(fn)
}

func (c *MotionEventController) ConnectHover(fn func(hovered bool)) signal.Handle {
	return c.hoverChanged.Connect(fn)
}

func (c *MotionEventController) ConnectContainsHover(fn func(hovered bool)) signal.Handle {
	return c.containsHoverChanged.Connect(fn)
}

func (c *MotionEventController) HandleEvent(ctx EventContext) {
	pointerEvent, ok := ctx.Event().(events.PointerEvent)
	if !ok || pointerEvent.EventType != events.PointerMove {
		return
	}

	position, ok := ctx.Position()
	if !ok {
		return
	}
	c.motion.Emit(MotionInfo{
		Position:  position,
		Modifiers: pointerEvent.Modifiers,
	})
}

func (c *MotionEventController) HandleCrossing(ctx CrossingContext) {
	if ctx.Type() != CrossingPointer {
		return
	}
	switch ctx.Mode() {
	case CrossingTarget:
		c.setIsHover(ctx.Direction() == CrossingEnter)
	case CrossingContains:
		c.setContainsHover(ctx.Direction() == CrossingEnter)
	}
}

func (c *MotionEventController) setIsHover(isHover bool) {
	if c.isHover == isHover {
		return
	}
	c.isHover = isHover
	c.hoverChanged.Emit(isHover)
}

func (c *MotionEventController) setContainsHover(containsHover bool) {
	if c.containsHover == containsHover {
		return
	}
	c.containsHover = containsHover
	c.containsHoverChanged.Emit(containsHover)
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

func (c *ClickEventController) HandleEvent(ctx EventContext) {
	pointerEvent, ok := ctx.Event().(events.PointerEvent)
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

func (c *ClickEventController) HandleCrossing(ctx CrossingContext) {
	if ctx.Type() != CrossingPointer || ctx.Mode() != CrossingContains || ctx.Direction() != CrossingLeave {
		return
	}
	position, hasPosition := ctx.Position()
	c.setPressed(&eventContext{
		position:         position,
		hasPosition:      hasPosition,
		explicitPosition: true,
	}, false)
}

func (c *ClickEventController) setPressed(ctx EventContext, pressed bool) {
	if c.pressed == pressed {
		return
	}
	c.pressed = pressed
	c.press.Emit(ctx, pressed)
}
