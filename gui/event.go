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
	Widget() Widget
	SetWidget(Widget)
	HandleEvent(ctx *EventContext, event events.Event)
}

type EventContext struct {
	target  Widget
	current Widget
	phase   PropagationPhase
	stopped bool
}

func (c *EventContext) Target() Widget {
	return c.target
}

func (c *EventContext) Current() Widget {
	return c.current
}

func (c *EventContext) Phase() PropagationPhase {
	return c.phase
}

func (c *EventContext) StopPropagation() {
	c.stopped = true
}

func (c *EventContext) PropagationStopped() bool {
	return c.stopped
}

type EventDispatcher struct{}

func (d *EventDispatcher) DispatchEvent(window Window, event events.Event) error {
	if window == nil {
		return nil
	}

	root := window.Widget()
	if root == nil {
		return nil
	}

	target := d.target(root, event)
	if target == nil {
		return nil
	}

	path := widgetPath(root, target)
	if len(path) == 0 {
		return nil
	}

	ctx := &EventContext{
		target: target,
	}

	d.dispatchPhase(ctx, path[:len(path)-1], PhaseCapture, event)
	if ctx.PropagationStopped() {
		return nil
	}

	d.dispatchPhase(ctx, path[len(path)-1:], PhaseTarget, event)
	if ctx.PropagationStopped() {
		return nil
	}

	slices.Reverse(path[:len(path)-1])
	d.dispatchPhase(ctx, path[:len(path)-1], PhaseBubble, event)
	return nil
}

func (d *EventDispatcher) target(root Widget, event events.Event) Widget {
	switch event := event.(type) {
	case events.PointerEvent:
		return hitTest(root, event.Position)
	case events.WheelEvent:
		return hitTest(root, event.Position)
	case events.KeyEvent:
		// GUI focus is introduced in a later slice. Until then, key events are
		// delivered to the top-level widget so controllers can still be tested.
		return root
	default:
		return nil
	}
}

func (d *EventDispatcher) dispatchPhase(ctx *EventContext, widgets []Widget, phase PropagationPhase, event events.Event) {
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

func hitTest(widget Widget, point geometry.Point) Widget {
	if widget == nil || !widget.Visible() || !containsPoint(widget.Rect(), point) {
		return nil
	}

	localPoint := subtractPoint(point, widget.Rect().Pos)
	children := widget.Children()
	for i := len(children) - 1; i >= 0; i-- {
		if target := hitTest(children[i], localPoint); target != nil {
			return target
		}
	}
	return widget
}

func widgetPath(root, target Widget) []Widget {
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
