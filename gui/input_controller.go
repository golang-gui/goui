package gui

import (
	"math"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/events"
)

// MotionInfo carries a pointer move in widget-local DIP.
type MotionInfo struct {
	Buttons   events.PointerButtons
	Position  geometry.Point
	Modifiers events.Modifiers
}

type MotionEventController struct {
	phase                PropagationPhase
	hover                bool
	containsHover        bool
	motion               signal.Signal2[inputNotification, MotionInfo]
	generation           uint64
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
	c.generation++
	c.setHover(false)
	c.setContainsHover(false)
}

func (c *MotionEventController) Hover() bool {
	return c.hover
}

func (c *MotionEventController) ContainsHover() bool {
	return c.containsHover
}

func (c *MotionEventController) ConnectMotion(fn func(EventContext, MotionInfo)) signal.Handle {
	return c.motion.Connect(func(n inputNotification, info MotionInfo) {
		if n.valid() {
			fn(n.ctx, info)
		}
	})
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
	epoch := c.generation
	c.motion.Emit(newInputNotification(ctx, func() bool { return c.generation == epoch }), MotionInfo{
		Buttons:   pointerEvent.Buttons,
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

// ClickEventController reports immediate presses and completed clicks through
// separate signals. Consecutive presses do not delay an earlier click or claim
// gesture ownership. All callbacks run synchronously on the GUI thread.
type ClickEventController struct {
	phase       PropagationPhase
	button      events.PointerButton
	pressed     bool
	active      bool
	released    bool
	gesture     *GestureParticipation
	now         func() time.Time
	sequence    clickSequence
	pressInfo   ClickInfo
	releaseInfo ClickInfo
	generation  uint64
	press       signal.Signal2[inputNotification, ClickInfo]
	release     signal.Signal2[inputNotification, ClickInfo]
	clicked     signal.Signal2[inputNotification, ClickInfo]
	state       signal.Signal2[uint64, bool]
	canceled    signal.Signal2[uint64, GestureCancelReason]
}

func NewClickEventController() *ClickEventController {
	return &ClickEventController{phase: PhaseBubble, button: events.PointerButtonLeft, now: time.Now}
}

func (c *ClickEventController) Phase() PropagationPhase         { return c.phase }
func (c *ClickEventController) SetPhase(phase PropagationPhase) { c.phase = phase }
func (c *ClickEventController) Button() events.PointerButton    { return c.button }
func (c *ClickEventController) Pressed() bool                   { return c.pressed }
func (c *ClickEventController) SetButton(button events.PointerButton) {
	if c.button != button {
		epoch := c.generation
		c.Reset()
		if c.generation != epoch+1 {
			return
		}
		c.button = button
	}
}

func (c *ClickEventController) Reset() { c.cancel(GestureInterrupted) }

func (c *ClickEventController) cancel(reason GestureCancelReason) {
	if g := c.gesture; g != nil && g.sequence.active && g.state != gestureEnded {
		g.sequence.cancelMember(g, reason)
		return
	}
	c.GestureCanceled(reason)
}

func (c *ClickEventController) GestureCanceled(reason GestureCancelReason) {
	wasActive := c.active
	c.generation++
	epoch := c.generation
	c.gesture, c.active, c.released = nil, false, false
	c.sequence.reset()
	c.setPressed(false)
	if wasActive && c.generation == epoch {
		c.canceled.Emit(epoch, reason)
	}
}

func (c *ClickEventController) notification(ctx EventContext, epoch uint64, g *GestureParticipation) inputNotification {
	var serial uint64
	if g != nil {
		serial = g.sequence.serial
	}
	return newInputNotification(ctx, func() bool {
		return c.generation == epoch && (g == nil || g.sequence.valid(g) && g.sequence.serial == serial)
	})
}

func (c *ClickEventController) GestureAccepted(ctx EventContext) {
	if !c.released {
		return
	}
	g, info, epoch := c.gesture, c.releaseInfo, c.generation
	c.gesture, c.released = nil, false
	c.clicked.Emit(c.notification(ctx, epoch, g), info)
}

// CancelPointer cancels the active press without inventing a release input.
func (c *ClickEventController) CancelPointer() { c.cancel(GestureInterrupted) }

// ConnectPressed reports a real press immediately, before gesture arbitration.
// A press does not promise a later click or gesture ownership.
func (c *ClickEventController) ConnectPressed(fn func(EventContext, ClickInfo)) signal.Handle {
	return c.press.Connect(func(n inputNotification, info ClickInfo) {
		if n.valid() {
			fn(n.ctx, info)
		}
	})
}

// ConnectReleased reports a real matching release of an active press. It may
// complete without a click, for example when released outside the control.
// Cancellation does not emit Released; each press has at most one terminal
// release or cancellation notification.
func (c *ClickEventController) ConnectReleased(fn func(EventContext, ClickInfo)) signal.Handle {
	return c.release.Connect(func(n inputNotification, info ClickInfo) {
		if n.valid() {
			fn(n.ctx, info)
		}
	})
}

// ConnectClicked reports a successful completed click. Count matches its press,
// including counts above two. Earlier clicks are delivered without waiting for
// a possible later double or triple click.
func (c *ClickEventController) ConnectClicked(fn func(EventContext, ClickInfo)) signal.Handle {
	return c.clicked.Connect(func(n inputNotification, info ClickInfo) {
		if n.valid() {
			fn(n.ctx, info)
		}
	})
}

// ConnectPressedChanged observes visual press state, including crossing and
// Reset. It carries no input context: a state change need not be a native input.
func (c *ClickEventController) ConnectPressedChanged(fn func(bool)) signal.Handle {
	return c.state.Connect(func(epoch uint64, pressed bool) {
		if c.generation == epoch && c.pressed == pressed {
			fn(pressed)
		}
	})
}

// ConnectCanceled observes interruption before release. Reason comes from the
// existing gesture/lifecycle path; no synthetic pointer input is reported.
func (c *ClickEventController) ConnectCanceled(fn func(GestureCancelReason)) signal.Handle {
	return c.canceled.Connect(func(epoch uint64, reason GestureCancelReason) {
		if c.generation == epoch {
			fn(reason)
		}
	})
}

func (c *ClickEventController) HandleEvent(ctx EventContext) {
	e, ok := ctx.Event().(events.PointerEvent)
	if !ok {
		return
	}
	switch e.EventType {
	case events.PointerDown:
		if e.Button != c.button || c.button == events.PointerButtonLeft && !primaryPointer(e) {
			c.sequence.reset()
			return
		}
		position, ok := ctx.Position()
		if !ok {
			return
		}
		if c.active || c.released {
			c.Reset()
		}
		c.generation++
		epoch := c.generation
		now := time.Now()
		if c.now != nil {
			now = c.now()
		}
		c.pressInfo = ClickInfo{Position: position, Button: e.Button, Buttons: e.Buttons,
			Modifiers: e.Modifiers, Count: c.sequence.press(now, e.Position, e.Button)}
		c.active, c.released = true, false
		c.gesture = JoinGesture(ctx)
		n := c.notification(ctx, epoch, c.gesture)
		c.setPressed(true)
		if n.valid() {
			c.press.Emit(n, c.pressInfo)
		}
	case events.PointerUp:
		if e.Button != c.button || !c.active {
			return
		}
		position, ok := ctx.Position()
		if !ok {
			c.cancel(GestureInterrupted)
			return
		}
		g, epoch := c.gesture, c.generation
		n := c.notification(ctx, epoch, g)
		eligible := c.pressed
		if ec, ok := ctx.(*eventContext); ok && ec.current != nil {
			eligible = eligible && containsPoint(geometry.Rect(0, 0, ec.current.Rect().Width, ec.current.Rect().Height), position)
		}
		info := ClickInfo{Position: position, Button: e.Button, Buttons: e.Buttons,
			Modifiers: e.Modifiers, Count: c.pressInfo.Count}
		c.active = false
		c.setPressed(false)
		if !n.valid() {
			return
		}
		c.release.Emit(n, info)
		if !n.valid() {
			return
		}
		if !eligible {
			c.sequence.reset()
			c.gesture = nil
			g.Reject()
			return
		}
		if g != nil {
			c.releaseInfo, c.released = info, true
			g.Claim()
		} else {
			c.clicked.Emit(n, info)
		}
	case events.PointerLeave:
		c.setPressed(false)
	}
}

func (c *ClickEventController) HandleCrossing(ctx CrossingContext) {
	if ctx.Type() == CrossingPointer && ctx.Mode() == CrossingContains && ctx.Direction() == CrossingLeave {
		c.setPressed(false)
	}
}

func (c *ClickEventController) setPressed(pressed bool) {
	if c.pressed == pressed {
		return
	}
	c.pressed = pressed
	c.state.Emit(c.generation, pressed)
}

type KeyEventController struct {
	phase      PropagationPhase
	keyDown    signal.Signal2[inputNotification, events.KeyEvent]
	generation uint64
	keyUp      signal.Signal2[inputNotification, events.KeyEvent]
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

func (c *KeyEventController) Reset() { c.generation++ }

func (c *KeyEventController) ConnectKeyDown(fn func(ctx EventContext, event events.KeyEvent)) signal.Handle {
	return c.keyDown.Connect(func(n inputNotification, event events.KeyEvent) {
		if n.valid() {
			fn(n.ctx, event)
		}
	})
}

func (c *KeyEventController) ConnectKeyUp(fn func(ctx EventContext, event events.KeyEvent)) signal.Handle {
	return c.keyUp.Connect(func(n inputNotification, event events.KeyEvent) {
		if n.valid() {
			fn(n.ctx, event)
		}
	})
}

func (c *KeyEventController) HandleEvent(ctx EventContext) {
	keyEvent, ok := ctx.Event().(events.KeyEvent)
	if !ok {
		return
	}

	epoch := c.generation
	n := newInputNotification(ctx, func() bool { return c.generation == epoch })
	switch keyEvent.EventType {
	case events.KeyDown:
		c.keyDown.Emit(n, keyEvent)
	case events.KeyUp:
		c.keyUp.Emit(n, keyEvent)
	}
}

func (c *KeyEventController) HandleCrossing(ctx CrossingContext) {}

type WheelEventController struct {
	phase      PropagationPhase
	scroll     signal.Signal2[inputNotification, WheelInfo]
	generation uint64
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

func (c *WheelEventController) Reset() { c.generation++ }

func (c *WheelEventController) ConnectScroll(fn func(EventContext, WheelInfo)) signal.Handle {
	return c.scroll.Connect(func(n inputNotification, info WheelInfo) {
		if n.valid() {
			fn(n.ctx, info)
		}
	})
}

func (c *WheelEventController) HandleEvent(ctx EventContext) {
	wheelEvent, ok := ctx.Event().(events.WheelEvent)
	if !ok {
		return
	}
	position, ok := ctx.Position()
	if !ok {
		return
	}
	epoch := c.generation
	c.scroll.Emit(newInputNotification(ctx, func() bool { return c.generation == epoch }), WheelInfo{Position: position, DeltaX: wheelEvent.DeltaX, DeltaY: wheelEvent.DeltaY, Mode: wheelEvent.Mode, Buttons: wheelEvent.Buttons, Modifiers: wheelEvent.Modifiers})
}

func (c *WheelEventController) HandleCrossing(ctx CrossingContext) {}

// DragEventController tracks a pointer drag using the existing gesture
// arbitration. Begin describes the original press; Update and End describe the
// current input. All positions are widget-local DIP. Cancellation is separate
// from End and never invents pointer coordinates.
type DragEventController struct {
	phase          PropagationPhase
	button         events.PointerButton
	dragging       bool
	updating       bool
	gesture        *GestureParticipation
	threshold      float32
	start          geometry.Point
	startWindow    geometry.Point
	startModifiers events.Modifiers
	startButtons   events.PointerButtons
	generation     uint64
	begin          signal.Signal2[inputNotification, DragInfo]
	update         signal.Signal2[inputNotification, DragInfo]
	end            signal.Signal2[inputNotification, DragInfo]
	cancel         signal.Signal2[uint64, GestureCancelReason]
}

func NewDragEventController() *DragEventController {
	return &DragEventController{phase: PhaseBubble, button: events.PointerButtonLeft}
}
func (c *DragEventController) Phase() PropagationPhase         { return c.phase }
func (c *DragEventController) SetPhase(phase PropagationPhase) { c.phase = phase }
func (c *DragEventController) Button() events.PointerButton    { return c.button }
func (c *DragEventController) SetButton(button events.PointerButton) {
	if c.button != button {
		epoch := c.generation
		c.Reset()
		if c.generation != epoch+1 {
			return
		}
		c.button = button
	}
}
func (c *DragEventController) Dragging() bool { return c.dragging }

// SetThreshold sets the distance in DIP required before a drag starts.
// Zero preserves immediate dragging on PointerDown.
func (c *DragEventController) SetThreshold(distance float32) {
	if distance < 0 || math.IsNaN(float64(distance)) || math.IsInf(float64(distance), 0) {
		distance = 0
	}
	c.threshold = distance
}
func (c *DragEventController) Reset() {
	if g := c.gesture; g != nil && g.sequence.active && g.state != gestureEnded {
		g.sequence.cancelMember(g, GestureInterrupted)
		return
	}
	c.GestureCanceled(GestureInterrupted)
}
func (c *DragEventController) notification(ctx EventContext) inputNotification {
	epoch, g := c.generation, c.gesture
	var serial uint64
	if g != nil {
		serial = g.sequence.serial
	}
	return newInputNotification(ctx, func() bool {
		return c.generation == epoch && (g == nil || g.sequence.valid(g) && g.sequence.serial == serial)
	})
}
func (c *DragEventController) startInfo() DragInfo {
	return DragInfo{Position: c.start, Button: c.button, Buttons: c.startButtons, Modifiers: c.startModifiers}
}
func (c *DragEventController) GestureAccepted(ctx EventContext) {
	g := c.gesture
	c.dragging = true
	n := c.notification(ctx)
	c.begin.Emit(n, c.startInfo())
	// Preserve the threshold-crossing displacement in the same dispatch.
	if n.valid() && c.gesture == g && c.dragging {
		if e, ok := ctx.Event().(events.PointerEvent); ok && e.EventType == events.PointerMove {
			if position, valid := ctx.Position(); valid {
				c.emitUpdate(ctx, position, e)
			}
		}
	}
}
func (c *DragEventController) emitUpdate(ctx EventContext, position geometry.Point, e events.PointerEvent) {
	c.updating = true
	defer func() { c.updating = false }()
	c.update.Emit(c.notification(ctx), DragInfo{Position: position, Button: c.button, Buttons: e.Buttons, Modifiers: e.Modifiers})
}
func (c *DragEventController) GestureCanceled(reason GestureCancelReason) {
	wasDragging := c.dragging
	c.generation++
	epoch := c.generation
	c.dragging, c.gesture = false, nil
	if wasDragging {
		c.cancel.Emit(epoch, reason)
	}
}
func (c *DragEventController) CapturingPointer() bool { return c.dragging }
func (c *DragEventController) ConnectBegin(fn func(EventContext, DragInfo)) signal.Handle {
	return c.begin.Connect(func(n inputNotification, info DragInfo) {
		if n.valid() {
			fn(n.ctx, info)
		}
	})
}
func (c *DragEventController) ConnectUpdate(fn func(EventContext, DragInfo)) signal.Handle {
	return c.update.Connect(func(n inputNotification, info DragInfo) {
		if n.valid() {
			fn(n.ctx, info)
		}
	})
}
func (c *DragEventController) ConnectEnd(fn func(EventContext, DragInfo)) signal.Handle {
	return c.end.Connect(func(n inputNotification, info DragInfo) {
		if n.valid() {
			fn(n.ctx, info)
		}
	})
}
func (c *DragEventController) ConnectCanceled(fn func(GestureCancelReason)) signal.Handle {
	return c.cancel.Connect(func(epoch uint64, reason GestureCancelReason) {
		if c.generation == epoch {
			fn(reason)
		}
	})
}
func (c *DragEventController) HandleEvent(ctx EventContext) {
	e, ok := ctx.Event().(events.PointerEvent)
	if !ok {
		return
	}
	switch e.EventType {
	case events.PointerDown:
		if e.Button != c.button || c.button == events.PointerButtonLeft && !primaryPointer(e) {
			return
		}
		position, ok := ctx.Position()
		if !ok {
			return
		}
		c.generation++
		c.start, c.startWindow, c.startModifiers, c.startButtons = position, e.Position, e.Modifiers, e.Buttons
		if g := JoinGesture(ctx); g != nil {
			c.gesture = g
			if c.threshold == 0 {
				g.Claim()
			}
			return
		}
		c.dragging = true
		c.begin.Emit(c.notification(ctx), c.startInfo())
	case events.PointerMove:
		if c.gesture != nil && !c.dragging && gestureMoved(e.Position, c.startWindow, c.threshold) {
			c.gesture.Claim()
			return
		}
		if c.dragging {
			if position, ok := ctx.Position(); ok {
				c.emitUpdate(ctx, position, e)
			}
		}
	case events.PointerUp:
		if e.Button != c.button {
			return
		}
		if c.gesture != nil && !c.dragging {
			c.gesture.Reject()
			return
		}
		if !c.dragging {
			return
		}
		position, ok := ctx.Position()
		if !ok {
			return
		}
		n := c.notification(ctx)
		c.dragging, c.gesture = false, nil
		c.end.Emit(n, DragInfo{Position: position, Button: c.button, Buttons: e.Buttons, Modifiers: e.Modifiers})
	}
}
func (c *DragEventController) HandleCrossing(ctx CrossingContext) {}
