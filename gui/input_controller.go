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
	hover                bool
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
	c.setHover(false)
	c.setContainsHover(false)
}

func (c *MotionEventController) Hover() bool {
	return c.hover
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
		c.setHover(ctx.Direction() == CrossingEnter)
	case CrossingContains:
		c.setContainsHover(ctx.Direction() == CrossingEnter)
	}
}

func (c *MotionEventController) setHover(hover bool) {
	if c.hover == hover {
		return
	}
	c.hover = hover
	c.hoverChanged.Emit(hover)
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

type KeyEventController struct {
	phase   PropagationPhase
	keyDown signal.Signal2[EventContext, events.KeyEvent]
	keyUp   signal.Signal2[EventContext, events.KeyEvent]
}

func NewKeyEventController() *KeyEventController {
	return &KeyEventController{
		phase: PhaseTarget,
	}
}

func (c *KeyEventController) Phase() PropagationPhase {
	return c.phase
}

func (c *KeyEventController) SetPhase(phase PropagationPhase) {
	c.phase = phase
}

func (c *KeyEventController) Reset() {}

func (c *KeyEventController) ConnectKeyDown(fn func(ctx EventContext, event events.KeyEvent)) signal.Handle {
	return c.keyDown.Connect(fn)
}

func (c *KeyEventController) ConnectKeyUp(fn func(ctx EventContext, event events.KeyEvent)) signal.Handle {
	return c.keyUp.Connect(fn)
}

func (c *KeyEventController) HandleEvent(ctx EventContext) {
	keyEvent, ok := ctx.Event().(events.KeyEvent)
	if !ok {
		return
	}

	switch keyEvent.EventType {
	case events.KeyDown:
		c.keyDown.Emit(ctx, keyEvent)
	case events.KeyUp:
		c.keyUp.Emit(ctx, keyEvent)
	}
}

func (c *KeyEventController) HandleCrossing(ctx CrossingContext) {}

type WheelEventController struct {
	phase  PropagationPhase
	scroll signal.Signal2[EventContext, events.WheelEvent]
}

func NewWheelEventController() *WheelEventController {
	return &WheelEventController{
		phase: PhaseBubble,
	}
}

func (c *WheelEventController) Phase() PropagationPhase {
	return c.phase
}

func (c *WheelEventController) SetPhase(phase PropagationPhase) {
	c.phase = phase
}

func (c *WheelEventController) Reset() {}

func (c *WheelEventController) ConnectScroll(fn func(ctx EventContext, event events.WheelEvent)) signal.Handle {
	return c.scroll.Connect(fn)
}

func (c *WheelEventController) HandleEvent(ctx EventContext) {
	wheelEvent, ok := ctx.Event().(events.WheelEvent)
	if !ok {
		return
	}
	c.scroll.Emit(ctx, wheelEvent)
}

func (c *WheelEventController) HandleCrossing(ctx CrossingContext) {}

// DragEventController tracks a pointer drag gesture: button down → moves →
// button up. On PointerDown it calls CapturePointer() so subsequent moves
// arrive even outside the widget bounds. The begin signal receives the start
// point; update and end signals receive the current point — all in
// widget-local coordinates. The controller ignores crossing events; drag
// state is managed entirely by PointerDown/Move/Up.
type DragEventController struct {
	phase    PropagationPhase
	button   events.PointerButton
	dragging bool
	begin    signal.Signal2[geometry.Point, events.Modifiers]
	update   signal.Signal2[geometry.Point, events.Modifiers]
	end      signal.Signal2[geometry.Point, events.Modifiers]
}

func NewDragEventController() *DragEventController {
	return &DragEventController{
		phase:  PhaseBubble,
		button: events.PointerButtonLeft,
	}
}

func (c *DragEventController) Phase() PropagationPhase {
	return c.phase
}

func (c *DragEventController) SetPhase(phase PropagationPhase) {
	c.phase = phase
}

func (c *DragEventController) Button() events.PointerButton {
	return c.button
}

func (c *DragEventController) SetButton(button events.PointerButton) {
	if c.button == button {
		return
	}
	c.Reset()
	c.button = button
}

func (c *DragEventController) Dragging() bool {
	return c.dragging
}

func (c *DragEventController) Reset() {
	c.dragging = false
}

// CapturingPointer satisfies PointerCapturer. The DragEventController captures
// the pointer for the duration of a drag: from PointerDown (dragging=true)
// until PointerUp or Reset (dragging=false). The dispatcher queries this after
// each dispatch to decide whether to route subsequent pointer events to the
// controller's widget.
func (c *DragEventController) CapturingPointer() bool {
	return c.dragging
}

func (c *DragEventController) ConnectBegin(fn func(geometry.Point, events.Modifiers)) signal.Handle {
	return c.begin.Connect(fn)
}

func (c *DragEventController) ConnectUpdate(fn func(geometry.Point, events.Modifiers)) signal.Handle {
	return c.update.Connect(fn)
}

func (c *DragEventController) ConnectEnd(fn func(geometry.Point, events.Modifiers)) signal.Handle {
	return c.end.Connect(fn)
}

func (c *DragEventController) HandleEvent(ctx EventContext) {
	pointerEvent, ok := ctx.Event().(events.PointerEvent)
	if !ok {
		return
	}

	switch pointerEvent.EventType {
	case events.PointerDown:
		if pointerEvent.Button != c.button {
			return
		}
		position, ok := ctx.Position()
		if !ok {
			return
		}
		c.dragging = true
		c.begin.Emit(position, pointerEvent.Modifiers)

	case events.PointerMove:
		if !c.dragging {
			return
		}
		position, ok := ctx.Position()
		if !ok {
			return
		}
		c.update.Emit(position, pointerEvent.Modifiers)

	case events.PointerUp:
		if pointerEvent.Button != c.button || !c.dragging {
			return
		}
		position, ok := ctx.Position()
		if !ok {
			return
		}
		c.dragging = false
		c.end.Emit(position, pointerEvent.Modifiers)
	}
}

func (c *DragEventController) HandleCrossing(ctx CrossingContext) {}
