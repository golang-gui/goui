package gui

import (
	"math"
	"time"

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
	phase        PropagationPhase
	button       events.PointerButton
	pressed      bool
	gesture      *GestureParticipation
	now          func() time.Time
	downAt       time.Time
	downPosition geometry.Point
	lastDown     time.Time
	lastPosition geometry.Point
	second       bool
	press        signal.Signal2[EventContext, bool]
	clicked      signal.Signal1[EventContext]
	click        signal.Signal2[EventContext, int]
}

func NewClickEventController() *ClickEventController {
	return &ClickEventController{
		phase:  PhaseBubble,
		button: events.PointerButtonLeft,
		now:    time.Now,
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
	gesture := c.gesture
	c.gesture = nil
	gesture.Reject()
	c.pressed = false
	c.lastDown = time.Time{}
	c.second = false
}

func (c *ClickEventController) GestureAccepted(ctx EventContext) {
	gesture := c.gesture
	c.gesture = nil
	c.setPressed(ctx, false)
	if gesture != nil && !gesture.sequence.valid(gesture) {
		return
	}
	c.emitClick(ctx, gesture)
}

func (c *ClickEventController) GestureCanceled(GestureCancelReason) {
	gesture := c.gesture
	c.gesture = nil
	ctx := EventContext(&eventContext{})
	if gesture != nil {
		ctx = &eventContext{event: gesture.sequence.event, current: gesture.widget}
	}
	c.setPressed(ctx, false)
	if c.second {
		c.lastDown = time.Time{}
	}
	c.second = false
}

func (c *ClickEventController) emitClick(ctx EventContext, gesture *GestureParticipation) {
	count := 1
	if c.second {
		count = 2
		c.lastDown = time.Time{}
	} else {
		c.lastDown, c.lastPosition = c.downAt, c.downPosition
	}
	c.second = false
	c.click.Emit(ctx, count)
	if gesture != nil && !gesture.sequence.valid(gesture) {
		return
	}
	c.clicked.Emit(ctx)
}

// CancelPointer releases visual/semantic press state but never emits Clicked.
func (c *ClickEventController) CancelPointer(ctx EventContext) {
	c.setPressed(ctx, false)
}

func (c *ClickEventController) ConnectPressed(fn func(ctx EventContext, pressed bool)) signal.Handle {
	return c.press.Connect(fn)
}

func (c *ClickEventController) ConnectClicked(fn func(ctx EventContext)) signal.Handle {
	return c.clicked.Connect(fn)
}

// ConnectClick reports one or two completed clicks. The first click is
// delivered immediately; the second is recognized only on its valid release.
func (c *ClickEventController) ConnectClick(fn func(ctx EventContext, count int)) signal.Handle {
	return c.click.Connect(fn)
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
		now := time.Now()
		if c.now != nil {
			now = c.now()
		}
		c.second = !c.lastDown.IsZero() && now.Sub(c.lastDown) >= 0 && now.Sub(c.lastDown) <= gestureClickInterval &&
			!gestureMoved(pointerEvent.Position, c.lastPosition, gestureDefaultDistance)
		c.downAt, c.downPosition = now, pointerEvent.Position
		c.gesture = JoinGesture(ctx)
		c.setPressed(ctx, true)
	case events.PointerUp:
		if pointerEvent.Button != c.button {
			return
		}
		if c.pressed {
			if c.gesture != nil {
				if widget := c.gesture.widget; widget != nil {
					point := widgetLocalPoint(widget, pointerEvent.Position)
					if containsPoint(geometry.Rect(0, 0, widget.Rect().Width, widget.Rect().Height), point) {
						c.gesture.Claim()
					} else {
						c.gesture.Reject()
					}
				}
				return
			}
			c.setPressed(ctx, false)
			c.emitClick(ctx, nil)
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
	phase          PropagationPhase
	button         events.PointerButton
	dragging       bool
	gesture        *GestureParticipation
	threshold      float32
	start          geometry.Point
	startWindow    geometry.Point
	startModifiers events.Modifiers
	begin          signal.Signal2[geometry.Point, events.Modifiers]
	update         signal.Signal2[geometry.Point, events.Modifiers]
	end            signal.Signal2[geometry.Point, events.Modifiers]
	cancel         signal.Signal0
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

// SetThreshold sets the distance in DIP required before a drag starts.
// Zero preserves immediate dragging on PointerDown.
func (c *DragEventController) SetThreshold(distance float32) {
	if distance < 0 || math.IsNaN(float64(distance)) || math.IsInf(float64(distance), 0) {
		distance = 0
	}
	c.threshold = distance
}

func (c *DragEventController) Reset() {
	gesture := c.gesture
	c.gesture = nil
	gesture.Reject()
	c.dragging = false
}

func (c *DragEventController) GestureAccepted(EventContext) {
	c.dragging = true
	c.begin.Emit(c.start, c.startModifiers)
}

func (c *DragEventController) GestureCanceled(GestureCancelReason) {
	wasDragging := c.dragging
	c.dragging = false
	c.gesture = nil
	if wasDragging {
		c.cancel.Emit()
	}
}

// CapturingPointer reports whether this controller is currently dragging.
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

func (c *DragEventController) ConnectCancel(fn func()) signal.Handle { return c.cancel.Connect(fn) }

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
		if g := JoinGesture(ctx); g != nil {
			c.gesture = g
			c.start, c.startWindow, c.startModifiers = position, pointerEvent.Position, pointerEvent.Modifiers
			if c.threshold == 0 {
				g.Claim()
			}
			return
		}
		c.dragging = true
		c.begin.Emit(position, pointerEvent.Modifiers)

	case events.PointerMove:
		if c.gesture != nil && !c.dragging && gestureMoved(pointerEvent.Position, c.startWindow, c.threshold) {
			c.gesture.Claim()
			return
		}
		if !c.dragging {
			return
		}
		position, ok := ctx.Position()
		if !ok {
			return
		}
		c.update.Emit(position, pointerEvent.Modifiers)

	case events.PointerUp:
		if c.gesture != nil && !c.dragging {
			c.gesture.Reject()
			return
		}
		if pointerEvent.Button != c.button || !c.dragging {
			return
		}
		position, ok := ctx.Position()
		if !ok {
			return
		}
		c.dragging = false
		c.gesture = nil
		c.end.Emit(position, pointerEvent.Modifiers)
	}
}

func (c *DragEventController) HandleCrossing(ctx CrossingContext) {}
