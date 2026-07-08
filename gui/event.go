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
	hoverPath []Widget
	focusPath []Widget
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
	if root == nil {
		return nil
	}

	if _, ok := event.(events.FocusEvent); ok {
		d.updateFocus(root, host.FocusedWidget())
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

	path := widgetPath(root, target)
	if len(path) == 0 {
		return nil
	}

	ctx := &eventContext{
		event:  event,
		target: target,
	}

	d.dispatchPhase(ctx, path, PhaseCapture, event)
	if ctx.PropagationStopped() {
		return nil
	}

	d.dispatchPhase(ctx, path[len(path)-1:], PhaseTarget, event)
	if ctx.PropagationStopped() {
		return nil
	}

	slices.Reverse(path)
	d.dispatchPhase(ctx, path, PhaseBubble, event)
	return nil
}

func (d *EventDispatcher) target(host EventTarget, root Widget, event events.Event) Widget {
	switch event := event.(type) {
	case events.PointerEvent:
		target := hitTest(root, event.Position)
		if event.EventType == events.PointerDown {
			focusNearest(host, target)
		}
		return target
	case events.WheelEvent:
		return hitTest(root, event.Position)
	case events.KeyEvent:
		if focused := host.FocusedWidget(); focused != nil {
			return focused
		}
		return root
	default:
		return nil
	}
}

func focusNearest(host EventTarget, target Widget) {
	for widget := target; widget != nil; widget = widget.Parent() {
		if widget.Focusable() {
			_ = host.SetFocusedWidget(widget)
			return
		}
	}
	_ = host.SetFocusedWidget(nil)
}

func (d *EventDispatcher) dispatchPhase(ctx *eventContext, widgets []Widget, phase PropagationPhase, event events.Event) {
	for _, widget := range widgets {
		ctx.current = widget
		for _, controller := range widget.EventControllers() {
			if controller == nil || controller.Phase() != phase {
				continue
			}
			controller.HandleEvent(ctx)
			if ctx.PropagationStopped() {
				return
			}
		}
	}
}

func (d *EventDispatcher) updateHover(root Widget, event events.PointerEvent) {
	target := hitTest(root, event.Position)
	path := widgetPath(root, target)
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
	if widget == nil || !widget.Visible() || !containsPoint(widget.Rect(), point) {
		return nil
	}

	localPoint := subtractPoint(point, widget.Rect().Pos)
	if container, ok := widget.(Container); ok {
		children := container.Children()
		for i := len(children) - 1; i >= 0; i-- {
			if target := hitTest(children[i], localPoint); target != nil {
				return target
			}
		}
	}
	return widget
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
