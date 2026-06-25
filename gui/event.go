package gui

import "github.com/golang-gui/goui/platform/events"

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
	target      Widget
	current     Widget
	phase       PropagationPhase
	propagation bool
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
	c.propagation = true
}

func (c *EventContext) PropagationStopped() bool {
	return c.propagation
}

type EventDispatcher struct{}

func (d *EventDispatcher) DispatchEvent(window Window, event events.Event) error {
	// Full widget-tree propagation is implemented in the next slice. Keeping
	// this method now lets Window and Application expose the stable entry point.
	return nil
}
