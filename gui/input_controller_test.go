package gui

import (
	"testing"

	"github.com/golang-gui/goui/platform/events"
)

func TestHoverEventControllerTracksPointerEnterAndLeave(t *testing.T) {
	controller := NewHoverEventController()
	enters := 0
	leaves := 0
	controller.ConnectEnter(func() {
		enters++
	})
	controller.ConnectLeave(func() {
		leaves++
	})

	controller.HandleEvent(new(eventContext), events.PointerEvent{EventType: events.PointerEnter})
	controller.HandleEvent(new(eventContext), events.PointerEvent{EventType: events.PointerEnter})
	if !controller.Hovered() || enters != 1 || leaves != 0 {
		t.Fatalf("unexpected hover enter state: hovered=%v enters=%d leaves=%d", controller.Hovered(), enters, leaves)
	}

	controller.HandleEvent(new(eventContext), events.PointerEvent{EventType: events.PointerLeave})
	controller.HandleEvent(new(eventContext), events.PointerEvent{EventType: events.PointerLeave})
	if controller.Hovered() || enters != 1 || leaves != 1 {
		t.Fatalf("unexpected hover leave state: hovered=%v enters=%d leaves=%d", controller.Hovered(), enters, leaves)
	}
}

func TestClickEventControllerTracksPressAndClick(t *testing.T) {
	controller := NewClickEventController()
	var pressed []bool
	clicks := 0
	controller.ConnectPressed(func(ctx EventContext, value bool) {
		pressed = append(pressed, value)
	})
	controller.ConnectClicked(func(ctx EventContext) {
		clicks++
	})

	down := new(eventContext)
	controller.HandleEvent(down, events.PointerEvent{
		EventType: events.PointerDown,
		Button:    events.PointerButtonLeft,
	})
	if !controller.Pressed() || down.PropagationStopped() {
		t.Fatalf("pointer down did not press controller: pressed=%v stopped=%v", controller.Pressed(), down.PropagationStopped())
	}

	up := new(eventContext)
	controller.HandleEvent(up, events.PointerEvent{
		EventType: events.PointerUp,
		Button:    events.PointerButtonLeft,
	})
	if controller.Pressed() || up.PropagationStopped() || clicks != 1 {
		t.Fatalf("pointer up did not click: pressed=%v stopped=%v clicks=%d", controller.Pressed(), up.PropagationStopped(), clicks)
	}
	if len(pressed) != 2 || !pressed[0] || pressed[1] {
		t.Fatalf("unexpected pressed calls: %v", pressed)
	}
}

func TestClickEventControllerSignalsCanStopPropagation(t *testing.T) {
	controller := NewClickEventController()
	controller.ConnectPressed(func(ctx EventContext, pressed bool) {
		if pressed {
			ctx.StopPropagation()
		}
	})
	controller.ConnectClicked(func(ctx EventContext) {
		ctx.StopPropagation()
	})

	down := new(eventContext)
	controller.HandleEvent(down, events.PointerEvent{
		EventType: events.PointerDown,
		Button:    events.PointerButtonLeft,
	})
	if !down.PropagationStopped() {
		t.Fatal("pressed signal did not stop propagation")
	}

	up := new(eventContext)
	controller.HandleEvent(up, events.PointerEvent{
		EventType: events.PointerUp,
		Button:    events.PointerButtonLeft,
	})
	if !up.PropagationStopped() {
		t.Fatal("clicked signal did not stop propagation")
	}
}

func TestClickEventControllerIgnoresOtherButtons(t *testing.T) {
	controller := NewClickEventController()
	clicks := 0
	controller.ConnectClicked(func(ctx EventContext) {
		clicks++
	})

	ctx := new(eventContext)
	controller.HandleEvent(ctx, events.PointerEvent{
		EventType: events.PointerDown,
		Button:    events.PointerButtonRight,
	})
	if controller.Pressed() || ctx.PropagationStopped() {
		t.Fatal("right button should not press default click controller")
	}

	controller.HandleEvent(new(eventContext), events.PointerEvent{
		EventType: events.PointerUp,
		Button:    events.PointerButtonRight,
	})
	if clicks != 0 {
		t.Fatalf("unexpected right button clicks: %d", clicks)
	}
}

func TestClickEventControllerDefaultsToBubblePhaseAndCanSetPhase(t *testing.T) {
	controller := NewClickEventController()
	if controller.Phase() != PhaseBubble {
		t.Fatalf("unexpected default phase: %v", controller.Phase())
	}

	controller.SetPhase(PhaseTarget)
	if controller.Phase() != PhaseTarget {
		t.Fatalf("unexpected phase: %v", controller.Phase())
	}
}

func TestClickEventControllerResetClearsPress(t *testing.T) {
	controller := NewClickEventController()
	var pressed []bool
	controller.ConnectPressed(func(ctx EventContext, value bool) {
		pressed = append(pressed, value)
	})

	controller.HandleEvent(new(eventContext), events.PointerEvent{
		EventType: events.PointerDown,
		Button:    events.PointerButtonLeft,
	})
	controller.Reset()

	if controller.Pressed() {
		t.Fatal("reset did not clear pressed state")
	}
	if len(pressed) != 1 || !pressed[0] {
		t.Fatalf("unexpected pressed calls: %v", pressed)
	}
}
