package gui

import (
	"fmt"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
)

func TestEventDispatcherDispatchesPointerEventThroughThreePhases(t *testing.T) {
	root := newTestWidget()
	parent := newTestWidget()
	target := newTestWidget()
	root.SetID("root")
	parent.SetID("parent")
	target.SetID("target")
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	parent.Arrange(geometry.Rect(10, 10, 80, 80))
	target.Arrange(geometry.Rect(5, 5, 30, 30))
	root.AddChild(parent)
	parent.AddChild(target)

	var calls []string
	root.AddEventController(newRecordingController("root-capture", PhaseCapture, &calls, nil))
	parent.AddEventController(newRecordingController("parent-capture", PhaseCapture, &calls, nil))
	target.AddEventController(newRecordingController("target-capture", PhaseCapture, &calls, nil))
	target.AddEventController(newRecordingController("target", PhaseTarget, &calls, nil))
	target.AddEventController(newRecordingController("target-bubble", PhaseBubble, &calls, nil))
	parent.AddEventController(newRecordingController("parent-bubble", PhaseBubble, &calls, nil))
	root.AddEventController(newRecordingController("root-bubble", PhaseBubble, &calls, nil))

	win := &window{root: root}
	event := events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 20, Y: 20},
	}

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"root-capture phase=0 type=6",
		"parent-capture phase=0 type=6",
		"target-capture phase=0 type=6",
		"target phase=1 type=6",
		"target-bubble phase=2 type=6",
		"parent-bubble phase=2 type=6",
		"root-bubble phase=2 type=6",
	}
	assertStrings(t, calls, want)
}

func TestEventDispatcherStopsPropagation(t *testing.T) {
	root := newTestWidget()
	parent := newTestWidget()
	target := newTestWidget()
	root.SetID("root")
	parent.SetID("parent")
	target.SetID("target")
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	parent.Arrange(geometry.Rect(10, 10, 80, 80))
	target.Arrange(geometry.Rect(5, 5, 30, 30))
	root.AddChild(parent)
	parent.AddChild(target)

	var calls []string
	root.AddEventController(newRecordingController("root-capture", PhaseCapture, &calls, nil))
	parent.AddEventController(newRecordingController("parent-capture", PhaseCapture, &calls, func(ctx EventContext) {
		ctx.StopPropagation()
	}))
	target.AddEventController(newRecordingController("target", PhaseTarget, &calls, nil))
	root.AddEventController(newRecordingController("root-bubble", PhaseBubble, &calls, nil))

	win := &window{root: root}
	event := events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 20, Y: 20},
	}

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"root-capture phase=0 type=6",
		"parent-capture phase=0 type=6",
	}
	assertStrings(t, calls, want)
}

func TestEventDispatcherHitTestUsesLastVisibleChild(t *testing.T) {
	root := newTestWidget()
	bottom := newTestWidget()
	top := newTestWidget()
	root.SetID("root")
	bottom.SetID("bottom")
	top.SetID("top")
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	bottom.Arrange(geometry.Rect(10, 10, 40, 40))
	top.Arrange(geometry.Rect(10, 10, 40, 40))
	root.AddChild(bottom)
	root.AddChild(top)

	var calls []string
	bottom.AddEventController(newRecordingController("bottom", PhaseTarget, &calls, nil))
	top.AddEventController(newRecordingController("top", PhaseTarget, &calls, nil))

	win := &window{root: root}
	event := events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 20, Y: 20},
	}

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"top phase=1 type=6",
	})

	top.SetVisible(false)
	calls = nil

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"bottom phase=1 type=6",
	})
}

func TestEventDispatcherDispatchesWheelByPosition(t *testing.T) {
	root := newTestWidget()
	child := newTestWidget()
	root.SetID("root")
	child.SetID("child")
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	child.Arrange(geometry.Rect(10, 10, 40, 40))
	root.AddChild(child)

	var calls []string
	child.AddEventController(newRecordingController("child", PhaseTarget, &calls, nil))

	win := &window{root: root}
	event := events.WheelEvent{
		Position: geometry.Point{X: 20, Y: 20},
		DeltaY:   -1,
		Mode:     events.WheelDeltaLine,
	}

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"child phase=1 type=8",
	})
}

func TestEventContextPositionIsLocalToCurrentWidget(t *testing.T) {
	root := newTestWidget()
	child := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	child.Arrange(geometry.Rect(10, 20, 40, 40))
	root.AddChild(child)

	var positions []geometry.Point
	child.AddEventController(newRecordingController("child", PhaseTarget, new([]string), func(ctx EventContext) {
		position, ok := ctx.Position()
		if !ok {
			t.Fatal("pointer event should provide a position")
		}
		positions = append(positions, position)
	}))

	win := &window{root: root}
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 15, Y: 27},
	}); err != nil {
		t.Fatal(err)
	}

	if len(positions) != 1 || positions[0] != (geometry.Point{X: 5, Y: 7}) {
		t.Fatalf("unexpected local positions: %+v", positions)
	}
}

func TestEventContextPositionIsAbsentForKeyEvent(t *testing.T) {
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))

	root.AddEventController(newRecordingController("root", PhaseTarget, new([]string), func(ctx EventContext) {
		if _, ok := ctx.Position(); ok {
			t.Fatal("key event should not provide a position")
		}
	}))

	win := &window{root: root}
	if err := win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown}); err != nil {
		t.Fatal(err)
	}
}

func TestEventDispatcherDoesNotPropagatePointerCrossingAsPlatformEvents(t *testing.T) {
	root := newTestWidget()
	first := newTestWidget()
	second := newTestWidget()
	root.SetID("root")
	first.SetID("first")
	second.SetID("second")
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	first.Arrange(geometry.Rect(0, 0, 40, 40))
	second.Arrange(geometry.Rect(50, 0, 40, 40))
	root.AddChild(first)
	root.AddChild(second)

	var calls []string
	root.AddEventController(newRecordingController("root", PhaseTarget, &calls, nil))
	first.AddEventController(newRecordingController("first", PhaseTarget, &calls, nil))
	second.AddEventController(newRecordingController("second", PhaseTarget, &calls, nil))

	win := &window{root: root}
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 10, Y: 10},
	}); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"first phase=1 type=5",
	})

	calls = nil
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 60, Y: 10},
	}); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"second phase=1 type=5",
	})

	calls = nil
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerLeave,
		Position:  geometry.Point{X: 60, Y: 10},
	}); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, nil)
}

func TestEventDispatcherUpdatesMotionHoverStates(t *testing.T) {
	root := newTestWidget()
	parent := newTestWidget()
	child := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	parent.Arrange(geometry.Rect(10, 10, 80, 80))
	child.Arrange(geometry.Rect(10, 10, 20, 20))
	root.AddChild(parent)
	parent.AddChild(child)

	parentMotion := NewMotionEventController()
	childMotion := NewMotionEventController()
	parent.AddEventController(parentMotion)
	child.AddEventController(childMotion)

	win := &window{root: root}
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 15, Y: 15},
	}); err != nil {
		t.Fatal(err)
	}
	if !parentMotion.Hover() || !parentMotion.ContainsHover() {
		t.Fatalf("parent should be directly hovered: is=%v contains=%v", parentMotion.Hover(), parentMotion.ContainsHover())
	}
	if childMotion.Hover() || childMotion.ContainsHover() {
		t.Fatalf("child should not be hovered: is=%v contains=%v", childMotion.Hover(), childMotion.ContainsHover())
	}

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 25, Y: 25},
	}); err != nil {
		t.Fatal(err)
	}
	if parentMotion.Hover() || !parentMotion.ContainsHover() {
		t.Fatalf("parent should contain hover through child: is=%v contains=%v", parentMotion.Hover(), parentMotion.ContainsHover())
	}
	if !childMotion.Hover() || !childMotion.ContainsHover() {
		t.Fatalf("child should be directly hovered: is=%v contains=%v", childMotion.Hover(), childMotion.ContainsHover())
	}

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 95, Y: 95},
	}); err != nil {
		t.Fatal(err)
	}
	if parentMotion.Hover() || parentMotion.ContainsHover() || childMotion.Hover() || childMotion.ContainsHover() {
		t.Fatalf("hover states were not cleared: parent is=%v contains=%v child is=%v contains=%v",
			parentMotion.Hover(), parentMotion.ContainsHover(), childMotion.Hover(), childMotion.ContainsHover())
	}
}

func TestEventDispatcherHoverStateIgnoresStoppedPointerMove(t *testing.T) {
	root := newTestWidget()
	child := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	child.Arrange(geometry.Rect(10, 10, 40, 40))
	root.AddChild(child)

	motion := NewMotionEventController()
	child.AddEventController(motion)
	child.AddEventController(newRecordingController("stop", PhaseTarget, new([]string), func(ctx EventContext) {
		ctx.StopPropagation()
	}))

	win := &window{root: root}
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 20, Y: 20},
	}); err != nil {
		t.Fatal(err)
	}
	if !motion.Hover() || !motion.ContainsHover() {
		t.Fatalf("hover state should update before propagation stop: is=%v contains=%v", motion.Hover(), motion.ContainsHover())
	}
}

func TestEventDispatcherDispatchesKeyToFocusedWidget(t *testing.T) {
	root := newTestWidget()
	child := newTestWidget()
	root.SetID("root")
	child.SetID("child")
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	child.Arrange(geometry.Rect(10, 10, 40, 40))
	root.AddChild(child)
	child.SetFocusable(true)

	var calls []string
	root.AddEventController(newRecordingController("root", PhaseTarget, &calls, nil))
	child.AddEventController(newRecordingController("child", PhaseTarget, &calls, nil))

	win := &window{}
	win.SetWidget(root)
	win.SetFocusedWidget(child)
	event := events.KeyEvent{
		EventType: events.KeyDown,
		Key:       events.KeyA,
	}

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"child phase=1 type=9",
	})
}

func TestEventDispatcherDispatchesKeyToRootWithoutFocusedWidget(t *testing.T) {
	root := newTestWidget()
	child := newTestWidget()
	root.SetID("root")
	child.SetID("child")
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	child.Arrange(geometry.Rect(10, 10, 40, 40))
	root.AddChild(child)

	var calls []string
	root.AddEventController(newRecordingController("root", PhaseTarget, &calls, nil))
	child.AddEventController(newRecordingController("child", PhaseTarget, &calls, nil))

	win := &window{root: root}
	event := events.KeyEvent{
		EventType: events.KeyDown,
		Key:       events.KeyA,
	}

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"root phase=1 type=9",
	})
}

func TestEventDispatcherDispatchesFocusCrossing(t *testing.T) {
	root := newTestWidget()
	parent := newTestWidget()
	child := newTestWidget()
	root.SetID("root")
	parent.SetID("parent")
	child.SetID("child")
	child.SetFocusable(true)
	root.AddChild(parent)
	parent.AddChild(child)

	var calls []string
	root.AddEventController(newCrossingRecordingController("root", &calls))
	parent.AddEventController(newCrossingRecordingController("parent", &calls))
	child.AddEventController(newCrossingRecordingController("child", &calls))

	win := &window{}
	win.SetWidget(root)
	if !win.SetFocusedWidget(child) {
		t.Fatal("focusable child should accept focus")
	}

	assertStrings(t, calls, []string{
		"root type=1 mode=1 direction=0 pos=false",
		"parent type=1 mode=1 direction=0 pos=false",
		"child type=1 mode=1 direction=0 pos=false",
		"child type=1 mode=0 direction=0 pos=false",
	})
	if root.Focused() || !root.ContainsFocus() || parent.Focused() || !parent.ContainsFocus() || !child.Focused() || !child.ContainsFocus() {
		t.Fatalf("unexpected focus state: root=%v/%v parent=%v/%v child=%v/%v",
			root.Focused(), root.ContainsFocus(), parent.Focused(), parent.ContainsFocus(), child.Focused(), child.ContainsFocus())
	}

	calls = nil
	win.SetFocusedWidget(nil)
	assertStrings(t, calls, []string{
		"child type=1 mode=0 direction=1 pos=false",
		"child type=1 mode=1 direction=1 pos=false",
		"parent type=1 mode=1 direction=1 pos=false",
		"root type=1 mode=1 direction=1 pos=false",
	})
}

func TestEventDispatcherPointerDownFocusesNearestFocusableWidget(t *testing.T) {
	root := newTestWidget()
	parent := newTestWidget()
	child := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	parent.Arrange(geometry.Rect(10, 10, 80, 80))
	child.Arrange(geometry.Rect(5, 5, 30, 30))
	root.AddChild(parent)
	parent.AddChild(child)
	parent.SetFocusable(true)

	win := &window{}
	win.SetWidget(root)

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 20, Y: 20},
	}); err != nil {
		t.Fatal(err)
	}

	if win.FocusedWidget() != parent {
		t.Fatalf("pointer down focused %v, want parent", win.FocusedWidget())
	}
	if !parent.Focused() {
		t.Fatal("parent focused state was not set")
	}

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 1, Y: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if win.FocusedWidget() != nil {
		t.Fatalf("non-focusable target should clear focus, got %v", win.FocusedWidget())
	}
	if parent.Focused() {
		t.Fatal("parent focused state was not cleared")
	}
}

func TestEventDispatcherIgnoresEventsWithoutTarget(t *testing.T) {
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))

	var calls []string
	root.AddEventController(newRecordingController("root", PhaseTarget, &calls, nil))

	win := &window{root: root}
	event := events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 120, Y: 20},
	}

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("unexpected calls: %v", calls)
	}
}

type recordingController struct {
	name   string
	phase  PropagationPhase
	calls  *[]string
	handle func(ctx EventContext)
}

func newRecordingController(name string, phase PropagationPhase, calls *[]string, handle func(ctx EventContext)) *recordingController {
	return &recordingController{
		name:   name,
		phase:  phase,
		calls:  calls,
		handle: handle,
	}
}

func (c *recordingController) Phase() PropagationPhase {
	return c.phase
}

func (c *recordingController) Reset() {}

func (c *recordingController) HandleEvent(ctx EventContext) {
	*c.calls = append(*c.calls, fmt.Sprintf(
		"%s phase=%d type=%d",
		c.name,
		c.phase,
		ctx.Event().Type(),
	))
	if c.handle != nil {
		c.handle(ctx)
	}
}

func (c *recordingController) HandleCrossing(ctx CrossingContext) {}

type crossingRecordingController struct {
	name  string
	calls *[]string
}

func newCrossingRecordingController(name string, calls *[]string) *crossingRecordingController {
	return &crossingRecordingController{name: name, calls: calls}
}

func (c *crossingRecordingController) Phase() PropagationPhase {
	return PhaseTarget
}

func (c *crossingRecordingController) Reset() {}

func (c *crossingRecordingController) HandleEvent(ctx EventContext) {}

func (c *crossingRecordingController) HandleCrossing(ctx CrossingContext) {
	_, hasPosition := ctx.Position()
	*c.calls = append(*c.calls, fmt.Sprintf(
		"%s type=%d mode=%d direction=%d pos=%v",
		c.name,
		ctx.Type(),
		ctx.Mode(),
		ctx.Direction(),
		hasPosition,
	))
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected call count:\ngot  %v\nwant %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected call %d:\ngot  %q\nwant %q\nall got: %v", i, got[i], want[i], got)
		}
	}
}
