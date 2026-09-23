package gui

import (
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
)

type PropagationPhase int

const (
	PhaseCapture PropagationPhase = iota
	PhaseTarget
	PhaseBubble
)

type EventController interface {
	Phase() PropagationPhase
	Reset()
	HandleEvent(ctx EventContext)
	HandleCrossing(ctx CrossingContext)
}

type EventContext interface {
	Event() events.Event
	Position() (geometry.Point, bool)
	StopPropagation()
	PropagationStopped() bool
}

type CrossingType int

const (
	CrossingPointer CrossingType = iota
	CrossingFocus
)

type CrossingMode int

const (
	CrossingTarget CrossingMode = iota
	CrossingContains
)

type CrossingDirection int

const (
	CrossingEnter CrossingDirection = iota
	CrossingLeave
)

type CrossingContext interface {
	Type() CrossingType
	Mode() CrossingMode
	Direction() CrossingDirection
	Position() (geometry.Point, bool)
}

type EventControllerBase struct {
	phase PropagationPhase
}

func NewEventControllerBase(phase PropagationPhase) EventControllerBase {
	return EventControllerBase{phase: phase}
}

func (b *EventControllerBase) Phase() PropagationPhase {
	return b.phase
}

func (b *EventControllerBase) SetPhase(phase PropagationPhase) {
	b.phase = phase
}

func (b *EventControllerBase) Reset() {}

func (b *EventControllerBase) HandleEvent(ctx EventContext) {}

func (b *EventControllerBase) HandleCrossing(ctx CrossingContext) {}

type eventContext struct {
	dispatcher       *EventDispatcher
	host             EventTarget
	controller       EventController
	alive            func() bool
	event            events.Event
	target           Widget
	current          Widget
	position         geometry.Point
	hasPosition      bool
	explicitPosition bool
	stopped          bool
}

func (c *eventContext) Event() events.Event {
	return c.event
}

func (c *eventContext) Position() (geometry.Point, bool) {
	if c.explicitPosition {
		return c.position, c.hasPosition
	}
	return eventLocalPosition(c.current, c.event)
}

func (c *eventContext) StopPropagation() {
	c.stopped = true
}

func (c *eventContext) PropagationStopped() bool {
	return c.stopped
}

type crossingContext struct {
	crossingType CrossingType
	mode         CrossingMode
	direction    CrossingDirection
	position     geometry.Point
	hasPosition  bool
}

func (c *crossingContext) Type() CrossingType {
	return c.crossingType
}

func (c *crossingContext) Mode() CrossingMode {
	return c.mode
}

func (c *crossingContext) Direction() CrossingDirection {
	return c.direction
}

func (c *crossingContext) Position() (geometry.Point, bool) {
	return c.position, c.hasPosition
}

type EventDispatcher struct {
	shortcuts *ShortcutController
	// Optional window-owned decoration tree, above the application content.
	// Both trees use this dispatcher's hover, focus, capture and controllers.
	decoration Widget
	// Optional window-owned controller, dispatched before Widget controllers.
	// Popovers do not install one.
	hostController EventController
	hoverPath      []Widget
	focusPath      []Widget
	captureTarget  Widget // derived from the accepted gesture; used by chrome queries
	gesture        *gestureSequence
	suppressUp     events.PointerButton // native takeover may consume the matching release
}

func (d *EventDispatcher) cancelInput(reason GestureCancelReason) {
	if d.gesture != nil {
		d.gesture.cancel(reason)
	}
	d.captureTarget = nil
	d.suppressUp = events.PointerButtonNone
}

// EventTarget is a widget-tree host the dispatcher propagates events into — a
// window or a popover. It exposes only what propagation needs, keeping the
// dispatcher independent of the concrete host type.
type EventTarget interface {
	Widget() Widget
	FocusedWidget() Widget
	SetFocusedWidget(Widget) bool
}

func (d *EventDispatcher) DispatchEvent(host EventTarget, event events.Event) error {
	if host == nil {
		return nil
	}

	root := host.Widget()
	var sequence *gestureSequence
	if pointer, ok := event.(events.PointerEvent); ok {
		if pointer.EventType == events.PointerDown {
			if d.gesture != nil {
				d.gesture.cancel(GestureInterrupted)
			}
			d.suppressUp = events.PointerButtonNone
			d.captureTarget = nil
			sequence = &gestureSequence{dispatcher: d, host: host, button: pointer.Button,
				buttonsObserved: pointer.Buttons&pointerButtonMask(pointer.Button) != 0,
				event:           pointer, active: true, delivering: true, serial: 1}
			d.gesture = sequence
		} else if d.gesture != nil {
			sequence = d.gesture
			sequence.event = pointer
			sequence.serial++
			sequence.delivering = true
			sequence.revalidate()
			if pointer.EventType == events.PointerMove && sequence.active {
				if pointer.Buttons&pointerButtonMask(sequence.button) != 0 {
					sequence.buttonsObserved = true
				} else if sequence.buttonsObserved {
					sequence.cancel(GestureInterrupted)
				}
			}
		}
		if pointer.EventType == events.PointerUp && pointer.Button == d.suppressUp {
			d.suppressUp = events.PointerButtonNone
			return nil
		}
		defer func() {
			d.deliverGestureRemainder(sequence)
			d.finishGesture(sequence, pointer)
		}()
	}
	if root == nil && d.decoration == nil {
		if d.shortcuts != nil {
			d.shortcuts.HandleEvent(&eventContext{event: event})
		}
		return nil
	}

	if _, ok := event.(events.FocusEvent); ok {
		if e := event.(events.FocusEvent); !e.Focused && d.gesture != nil {
			d.gesture.cancel(GestureInterrupted)
		}
		d.updateFocus(d.treeRoot(root, host.FocusedWidget()), host.FocusedWidget())
		return nil
	}

	if pointerEvent, ok := event.(events.PointerEvent); ok {
		switch pointerEvent.EventType {
		case events.PointerEnter, events.PointerMove, events.PointerDown, events.PointerUp:
			d.updateHover(root, pointerEvent)
			if pointerEvent.EventType == events.PointerEnter {
				return nil
			}
		case events.PointerLeave:
			d.clearHover(pointerEvent)
			return nil
		}
	}

	target := d.target(host, root, event)
	if target == nil {
		return nil
	}

	root = d.treeRoot(root, target)
	path := widgetPath(root, target)
	if len(path) == 0 {
		return nil
	}

	ctx := &eventContext{
		dispatcher: d,
		host:       host,
		event:      event,
		target:     target,
		alive: func() bool {
			return liveRoot(root) != nil && !target.base().destroyed && visibleInTree(target) &&
				(host.Widget() == root || d.decoration == root) && len(widgetPath(root, target)) != 0
		},
	}
	if pointer, ok := event.(events.PointerEvent); ok && sequence != nil && pointer.EventType == events.PointerDown {
		sequence.path = slices.Clone(path)
	}
	if d.hostController != nil {
		ctx.controller = d.hostController
		ctx.current = nil
		d.hostController.HandleEvent(ctx)
		if ctx.PropagationStopped() {
			return nil
		}
		if liveRoot(root) == nil || (host.Widget() != root && d.decoration != root) {
			return nil
		}
	}

	d.dispatchShortcuts(ctx, PhaseCapture)
	if ctx.PropagationStopped() {
		return nil
	}
	d.dispatchPhase(ctx, path, PhaseCapture, event)
	if ctx.PropagationStopped() {
		return nil
	}

	d.dispatchPhase(ctx, path[len(path)-1:], PhaseTarget, event)
	if ctx.PropagationStopped() {
		return nil
	}
	d.dispatchShortcuts(ctx, PhaseTarget)
	if ctx.PropagationStopped() {
		return nil
	}

	slices.Reverse(path)
	d.dispatchPhase(ctx, path, PhaseBubble, event)
	if !ctx.PropagationStopped() {
		d.dispatchShortcuts(ctx, PhaseBubble)
	}
	return nil
}

func (d *EventDispatcher) dispatchShortcuts(ctx *eventContext, phase PropagationPhase) {
	if ctx.alive != nil && !ctx.alive() {
		ctx.StopPropagation()
		return
	}
	if d.shortcuts != nil && d.shortcuts.Phase() == phase {
		d.shortcuts.HandleEvent(ctx)
	}
}

func (d *EventDispatcher) target(host EventTarget, root Widget, event events.Event) Widget {
	if d.gesture != nil && d.gesture.active && len(d.gesture.members) != 0 {
		if pe, ok := event.(events.PointerEvent); ok && (pe.EventType == events.PointerMove || pe.EventType == events.PointerUp) {
			for i := len(d.gesture.path) - 1; i >= 0; i-- {
				if visibleInTree(d.gesture.path[i]) {
					return d.gesture.path[i]
				}
			}
		}
	}
	switch event := event.(type) {
	case events.PointerEvent:
		target := d.pick(root, event.Position)
		if event.EventType == events.PointerDown {
			focusNearest(host, target)
		}
		return target
	case events.WheelEvent:
		return d.pick(root, event.Position)
	case events.KeyEvent:
		if focused := host.FocusedWidget(); focused != nil {
			return focused
		}
		return root
	default:
		return nil
	}
}

// focusNearest moves input focus to the nearest focusable widget in target's
// parent chain (the pointer-down target).
//
// If no focusable widget is found, the current focus is left unchanged. A
// click on a non-focusable surface — empty space, a :focusable=FALSE control
// like a menu-bar button, a label wrapper — does not steal or clear focus.
// This mirrors GTK4 (:focusable=FALSE means the widget and its descendants
// cannot take focus, so a click there does not move it) and Flutter ("there is
// always a primary focus": tapping a non-focusable element does not unfocus).
func focusNearest(host EventTarget, target Widget) {
	for widget := target; widget != nil; widget = widget.Parent() {
		if widget.Focusable() {
			_ = host.SetFocusedWidget(widget)
			return
		}
	}
}

func (d *EventDispatcher) dispatchPhase(ctx *eventContext, widgets []Widget, phase PropagationPhase, event events.Event) {
	for _, widget := range widgets {
		if ctx.alive != nil && !ctx.alive() {
			ctx.StopPropagation()
			return
		}
		if widget.base().destroyed {
			ctx.StopPropagation()
			return
		}
		ctx.current = widget
		for _, controller := range slices.Clone(widget.EventControllers()) {
			if !slices.Contains(widget.EventControllers(), controller) {
				continue
			}
			if controller == nil || controller.Phase() != phase {
				continue
			}
			ctx.controller = controller
			if s := d.gesture; s != nil {
				for _, p := range s.members {
					if p.controller == controller && p.widget == widget {
						p.delivered = s.serial
					}
				}
			}
			controller.HandleEvent(ctx)
			if ctx.alive != nil && !ctx.alive() {
				ctx.StopPropagation()
			}
			if ctx.PropagationStopped() {
				return
			}
		}
	}
}

func (d *EventDispatcher) updateHover(root Widget, event events.PointerEvent) {
	target := d.pick(root, event.Position)
	path := widgetPath(d.treeRoot(root, target), target)
	d.updatePointerHoverPath(path, event)
}

func (d *EventDispatcher) updatePointerHoverPath(path []Widget, event events.PointerEvent) {
	d.updateCrossingPath(CrossingPointer, d.hoverPath, path, event.Position, true)
	d.hoverPath = path
}

func (d *EventDispatcher) updateFocus(root, target Widget) {
	d.updateFocusPath(widgetPath(root, target))
}

func (d *EventDispatcher) updateFocusPath(path []Widget) {
	d.updateCrossingPath(CrossingFocus, d.focusPath, path, geometry.Point{}, false)
	d.focusPath = path
}

func (d *EventDispatcher) updateCrossingPath(crossingType CrossingType, oldPath, newPath []Widget, position geometry.Point, hasPosition bool) {
	oldTarget := pathTarget(oldPath)
	newTarget := pathTarget(newPath)

	if oldTarget != newTarget {
		d.notifyCrossing(oldTarget, crossingType, CrossingTarget, CrossingLeave, position, hasPosition)
	}

	for i := len(oldPath) - 1; i >= 0; i-- {
		if !slices.Contains(newPath, oldPath[i]) {
			d.notifyCrossing(oldPath[i], crossingType, CrossingContains, CrossingLeave, position, hasPosition)
		}
	}

	for _, widget := range newPath {
		if !slices.Contains(oldPath, widget) {
			d.notifyCrossing(widget, crossingType, CrossingContains, CrossingEnter, position, hasPosition)
		}
	}

	if oldTarget != newTarget {
		d.notifyCrossing(newTarget, crossingType, CrossingTarget, CrossingEnter, position, hasPosition)
	}
}

func (d *EventDispatcher) clearHover(event events.PointerEvent) {
	d.updatePointerHoverPath(nil, event)
}

func (d *EventDispatcher) notifyCrossing(widget Widget, crossingType CrossingType, mode CrossingMode, direction CrossingDirection, position geometry.Point, hasPosition bool) {
	if widget == nil {
		return
	}

	ctx := &crossingContext{
		crossingType: crossingType,
		mode:         mode,
		direction:    direction,
		hasPosition:  hasPosition,
	}
	if hasPosition {
		ctx.position = widgetLocalPoint(widget, position)
	}
	widget.base().handleCrossing(ctx)
	for _, controller := range widget.EventControllers() {
		if controller == nil {
			continue
		}
		controller.HandleCrossing(ctx)
	}
}

func hitTest(widget Widget, point geometry.Point) Widget {
	if widget == nil {
		return nil
	}
	return Pick(widget, subtractPoint(point, widget.Rect().Pos))
}

func (d *EventDispatcher) pick(content Widget, point geometry.Point) Widget {
	if target := hitTest(d.decoration, point); target != nil {
		return target
	}
	return hitTest(content, point)
}

func (d *EventDispatcher) treeRoot(content, target Widget) Widget {
	if target != nil && d.decoration != nil && target.base().isDescendant(target, d.decoration) {
		return d.decoration
	}
	return content
}

func widgetPath(root, target Widget) []Widget {
	if root == nil || target == nil {
		return nil
	}
	var path []Widget
	for widget := target; widget != nil; widget = widget.Parent() {
		path = append(path, widget)
		if widget == root {
			slices.Reverse(path)
			return path
		}
	}
	return nil
}

func containsPoint(rect geometry.Rectangle, point geometry.Point) bool {
	return point.X >= rect.X &&
		point.Y >= rect.Y &&
		point.X < rect.X+rect.Width &&
		point.Y < rect.Y+rect.Height
}

func subtractPoint(p, q geometry.Point) geometry.Point {
	return geometry.Point{
		X: p.X - q.X,
		Y: p.Y - q.Y,
	}
}

func pathTarget(path []Widget) Widget {
	if len(path) == 0 {
		return nil
	}
	return path[len(path)-1]
}

func eventLocalPosition(widget Widget, event events.Event) (geometry.Point, bool) {
	switch event := event.(type) {
	case events.PointerEvent:
		return widgetLocalPoint(widget, event.Position), true
	case events.WheelEvent:
		return widgetLocalPoint(widget, event.Position), true
	default:
		return geometry.Point{}, false
	}
}

func widgetLocalPoint(widget Widget, point geometry.Point) geometry.Point {
	if widget == nil {
		return point
	}
	rect := widget.base().windowRect()
	return geometry.Point{
		X: point.X - rect.X,
		Y: point.Y - rect.Y,
	}
}
