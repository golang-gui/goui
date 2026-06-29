package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
)

func TestMotionEventControllerTracksHoverStates(t *testing.T) {
	controller := NewMotionEventController()
	var hover []bool
	var containsHover []bool
	controller.ConnectHover(func(value bool) {
		hover = append(hover, value)
	})
	controller.ConnectContainsHover(func(value bool) {
		containsHover = append(containsHover, value)
	})

	controller.HandleCrossing(&crossingContext{crossingType: CrossingPointer, mode: CrossingContains, direction: CrossingEnter})
	controller.HandleCrossing(&crossingContext{crossingType: CrossingPointer, mode: CrossingTarget, direction: CrossingEnter})
	controller.HandleCrossing(&crossingContext{crossingType: CrossingPointer, mode: CrossingTarget, direction: CrossingEnter})
	if !controller.Hover() || !controller.ContainsHover() {
		t.Fatalf("unexpected hover state: is=%v contains=%v", controller.Hover(), controller.ContainsHover())
	}

	controller.Reset()
	if controller.Hover() || controller.ContainsHover() {
		t.Fatalf("reset did not clear hover state: is=%v contains=%v", controller.Hover(), controller.ContainsHover())
	}
	if len(hover) != 2 || !hover[0] || hover[1] {
		t.Fatalf("unexpected hover calls: %v", hover)
	}
	if len(containsHover) != 2 || !containsHover[0] || containsHover[1] {
		t.Fatalf("unexpected contains hover calls: %v", containsHover)
	}
}

func TestMotionEventControllerEmitsMotionInfo(t *testing.T) {
	controller := NewMotionEventController()
	widget := newTestWidget()
	widget.Arrange(geometry.Rect(10, 20, 30, 40))
	var motions []MotionInfo
	controller.ConnectMotion(func(info MotionInfo) {
		motions = append(motions, info)
	})

	controller.HandleEvent(&eventContext{current: widget, event: events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 15, Y: 27},
		Modifiers: events.ModifierShift,
	}})

	if len(motions) != 1 {
		t.Fatalf("expected one motion, got %d", len(motions))
	}
	if motions[0].Position != (geometry.Point{X: 5, Y: 7}) {
		t.Fatalf("unexpected local position: %+v", motions[0].Position)
	}
	if motions[0].Modifiers != events.ModifierShift {
		t.Fatalf("unexpected modifiers: %v", motions[0].Modifiers)
	}
}

func TestMotionEventControllerDefaultsToTargetPhaseAndCanSetPhase(t *testing.T) {
	controller := NewMotionEventController()
	if controller.Phase() != PhaseTarget {
		t.Fatalf("unexpected default phase: %v", controller.Phase())
	}

	controller.SetPhase(PhaseBubble)
	if controller.Phase() != PhaseBubble {
		t.Fatalf("unexpected phase: %v", controller.Phase())
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

	down := &eventContext{event: events.PointerEvent{
		EventType: events.PointerDown,
		Button:    events.PointerButtonLeft,
	}}
	controller.HandleEvent(down)
	if !controller.Pressed() || down.PropagationStopped() {
		t.Fatalf("pointer down did not press controller: pressed=%v stopped=%v", controller.Pressed(), down.PropagationStopped())
	}

	up := &eventContext{event: events.PointerEvent{
		EventType: events.PointerUp,
		Button:    events.PointerButtonLeft,
	}}
	controller.HandleEvent(up)
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

	down := &eventContext{event: events.PointerEvent{
		EventType: events.PointerDown,
		Button:    events.PointerButtonLeft,
	}}
	controller.HandleEvent(down)
	if !down.PropagationStopped() {
		t.Fatal("pressed signal did not stop propagation")
	}

	up := &eventContext{event: events.PointerEvent{
		EventType: events.PointerUp,
		Button:    events.PointerButtonLeft,
	}}
	controller.HandleEvent(up)
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

	ctx := &eventContext{event: events.PointerEvent{
		EventType: events.PointerDown,
		Button:    events.PointerButtonRight,
	}}
	controller.HandleEvent(ctx)
	if controller.Pressed() || ctx.PropagationStopped() {
		t.Fatal("right button should not press default click controller")
	}

	controller.HandleEvent(&eventContext{event: events.PointerEvent{
		EventType: events.PointerUp,
		Button:    events.PointerButtonRight,
	}})
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

	controller.HandleEvent(&eventContext{event: events.PointerEvent{
		EventType: events.PointerDown,
		Button:    events.PointerButtonLeft,
	}})
	controller.Reset()

	if controller.Pressed() {
		t.Fatal("reset did not clear pressed state")
	}
	if len(pressed) != 1 || !pressed[0] {
		t.Fatalf("unexpected pressed calls: %v", pressed)
	}
}

func TestClickEventControllerResetsPressOnPointerContainsLeave(t *testing.T) {
	controller := NewClickEventController()
	var pressed []bool
	controller.ConnectPressed(func(ctx EventContext, value bool) {
		pressed = append(pressed, value)
	})

	controller.HandleEvent(&eventContext{event: events.PointerEvent{
		EventType: events.PointerDown,
		Button:    events.PointerButtonLeft,
	}})
	controller.HandleCrossing(&crossingContext{crossingType: CrossingPointer, mode: CrossingContains, direction: CrossingLeave})

	if controller.Pressed() {
		t.Fatal("pointer contains leave did not clear pressed state")
	}
	if len(pressed) != 2 || !pressed[0] || pressed[1] {
		t.Fatalf("unexpected pressed calls: %v", pressed)
	}
}

func TestKeyEventControllerEmitsKeyDownAndKeyUp(t *testing.T) {
	controller := NewKeyEventController()
	var downs []events.KeyEvent
	var ups []events.KeyEvent
	controller.ConnectKeyDown(func(ctx EventContext, event events.KeyEvent) {
		downs = append(downs, event)
	})
	controller.ConnectKeyUp(func(ctx EventContext, event events.KeyEvent) {
		ups = append(ups, event)
	})

	down := events.KeyEvent{EventType: events.KeyDown, Key: events.KeyA, Modifiers: events.ModifierShift}
	up := events.KeyEvent{EventType: events.KeyUp, Key: events.KeyA}
	controller.HandleEvent(&eventContext{event: down})
	controller.HandleEvent(&eventContext{event: up})

	if len(downs) != 1 || downs[0] != down {
		t.Fatalf("unexpected key down calls: %v", downs)
	}
	if len(ups) != 1 || ups[0] != up {
		t.Fatalf("unexpected key up calls: %v", ups)
	}
}

func TestKeyEventControllerSignalsCanStopPropagation(t *testing.T) {
	controller := NewKeyEventController()
	controller.ConnectKeyDown(func(ctx EventContext, event events.KeyEvent) {
		ctx.StopPropagation()
	})

	ctx := &eventContext{event: events.KeyEvent{EventType: events.KeyDown, Key: events.KeyA}}
	controller.HandleEvent(ctx)

	if !ctx.PropagationStopped() {
		t.Fatal("key down signal did not stop propagation")
	}
}

func TestKeyEventControllerDefaultsToTargetPhaseAndCanSetPhase(t *testing.T) {
	controller := NewKeyEventController()
	if controller.Phase() != PhaseTarget {
		t.Fatalf("unexpected default phase: %v", controller.Phase())
	}

	controller.SetPhase(PhaseBubble)
	if controller.Phase() != PhaseBubble {
		t.Fatalf("unexpected phase: %v", controller.Phase())
	}
}

func TestKeyEventControllerIgnoresNonKeyEvents(t *testing.T) {
	controller := NewKeyEventController()
	calls := 0
	controller.ConnectKeyDown(func(ctx EventContext, event events.KeyEvent) {
		calls++
	})
	controller.ConnectKeyUp(func(ctx EventContext, event events.KeyEvent) {
		calls++
	})

	controller.HandleEvent(&eventContext{event: events.PointerEvent{EventType: events.PointerDown}})

	if calls != 0 {
		t.Fatalf("unexpected key calls: %d", calls)
	}
}
