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
	HandleEvent(ctx EventContext, event events.Event)
}

type EventContext interface {
	Target() Widget
	Current() Widget
	Phase() PropagationPhase
	StopPropagation()
	PropagationStopped() bool
}

type eventContext struct {
	target  Widget
	current Widget
	phase   PropagationPhase
	stopped bool
}

func (c *eventContext) Target() Widget {
	return c.target
}

func (c *eventContext) Current() Widget {
	return c.current
}

func (c *eventContext) Phase() PropagationPhase {
	return c.phase
}

func (c *eventContext) StopPropagation() {
	c.stopped = true
}

func (c *eventContext) PropagationStopped() bool {
	return c.stopped
}

type EventDispatcher struct {
	hoverPath []Widget
}

func (d *EventDispatcher) DispatchEvent(window Window, event events.Event) error {
	if window == nil {
		return nil
	}

	root := window.Widget()
	if root == nil {
		return nil
	}

	if pointerEvent, ok := event.(events.PointerEvent); ok {
		switch pointerEvent.EventType {
		case events.PointerEnter, events.PointerMove:
			d.updateHover(root, pointerEvent)
			if pointerEvent.EventType == events.PointerEnter {
				return nil
			}
		case events.PointerLeave:
			d.clearHover(pointerEvent)
			return nil
		}
	}

	target := d.target(window, root, event)
	if target == nil {
		return nil
	}

	path := widgetPath(root, target)
	if len(path) == 0 {
		return nil
	}

	ctx := &eventContext{
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

func (d *EventDispatcher) target(window Window, root Widget, event events.Event) Widget {
	switch event := event.(type) {
	case events.PointerEvent:
		target := hitTest(root, event.Position)
		if event.EventType == events.PointerDown {
			focusNearest(window, target)
		}
		return target
	case events.WheelEvent:
		return hitTest(root, event.Position)
	case events.KeyEvent:
		if focused := window.FocusedWidget(); focused != nil {
			return focused
		}
		return root
	default:
		return nil
	}
}

func focusNearest(window Window, target Widget) {
	for widget := target; widget != nil; widget = widget.Parent() {
		if widget.Focusable() {
			_ = window.SetFocusedWidget(widget)
			return
		}
	}
	_ = window.SetFocusedWidget(nil)
}

func (d *EventDispatcher) dispatchPhase(ctx *eventContext, widgets []Widget, phase PropagationPhase, event events.Event) {
	for _, widget := range widgets {
		ctx.current = widget
		ctx.phase = phase
		for _, controller := range widget.EventControllers() {
			if controller == nil || controller.Phase() != phase {
				continue
			}
			controller.HandleEvent(ctx, event)
			if ctx.PropagationStopped() {
				return
			}
		}
	}
}

func (d *EventDispatcher) updateHover(root Widget, event events.PointerEvent) {
	target := hitTest(root, event.Position)
	path := widgetPath(root, target)
	common := commonWidgetPrefix(d.hoverPath, path)

	leaveEvent := event
	leaveEvent.EventType = events.PointerLeave
	for i := len(d.hoverPath) - 1; i >= common; i-- {
		d.dispatchDirect(d.hoverPath[i], leaveEvent)
	}

	enterEvent := event
	enterEvent.EventType = events.PointerEnter
	for _, widget := range path[common:] {
		d.dispatchDirect(widget, enterEvent)
	}

	d.hoverPath = path
}

func (d *EventDispatcher) clearHover(event events.PointerEvent) {
	event.EventType = events.PointerLeave
	for i := len(d.hoverPath) - 1; i >= 0; i-- {
		d.dispatchDirect(d.hoverPath[i], event)
	}
	d.hoverPath = nil
}

func (d *EventDispatcher) dispatchDirect(widget Widget, event events.Event) {
	if widget == nil {
		return
	}

	ctx := &eventContext{
		target:  widget,
		current: widget,
		phase:   PhaseTarget,
	}
	for _, controller := range widget.EventControllers() {
		if controller == nil || controller.Phase() != PhaseTarget {
			continue
		}
		controller.HandleEvent(ctx, event)
		if ctx.PropagationStopped() {
			return
		}
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

func commonWidgetPrefix(a, b []Widget) int {
	count := min(len(a), len(b))
	for i := 0; i < count; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return count
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
