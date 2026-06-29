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
		"root-capture current=root target=target phase=0 type=7",
		"parent-capture current=parent target=target phase=0 type=7",
		"target-capture current=target target=target phase=0 type=7",
		"target current=target target=target phase=1 type=7",
		"target-bubble current=target target=target phase=2 type=7",
		"parent-bubble current=parent target=target phase=2 type=7",
		"root-bubble current=root target=target phase=2 type=7",
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
	parent.AddEventController(newRecordingController("parent-capture", PhaseCapture, &calls, func(ctx *EventContext) {
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
		"root-capture current=root target=target phase=0 type=7",
		"parent-capture current=parent target=target phase=0 type=7",
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
		"top current=top target=top phase=1 type=7",
	})

	top.SetVisible(false)
	calls = nil

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"bottom current=bottom target=bottom phase=1 type=7",
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
		"child current=child target=child phase=1 type=9",
	})
}

func TestEventDispatcherSynthesizesWidgetHoverEvents(t *testing.T) {
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
		"root current=root target=root phase=1 type=4",
		"first current=first target=first phase=1 type=4",
		"first current=first target=first phase=1 type=6",
	})

	calls = nil
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 60, Y: 10},
	}); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"first current=first target=first phase=1 type=5",
		"second current=second target=second phase=1 type=4",
		"second current=second target=second phase=1 type=6",
	})

	calls = nil
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerLeave,
		Position:  geometry.Point{X: 60, Y: 10},
	}); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"second current=second target=second phase=1 type=5",
		"root current=root target=root phase=1 type=5",
	})
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
		"child current=child target=child phase=1 type=10",
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
		"root current=root target=root phase=1 type=10",
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
	handle func(ctx *EventContext)
}

func newRecordingController(name string, phase PropagationPhase, calls *[]string, handle func(ctx *EventContext)) *recordingController {
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

func (c *recordingController) HandleEvent(ctx *EventContext, event events.Event) {
	*c.calls = append(*c.calls, fmt.Sprintf(
		"%s current=%s target=%s phase=%d type=%d",
		c.name,
		widgetID(ctx.Current()),
		widgetID(ctx.Target()),
		ctx.Phase(),
		event.Type(),
	))
	if c.handle != nil {
		c.handle(ctx)
	}
}

func widgetID(widget Widget) string {
	if widget == nil {
		return "<nil>"
	}
	if widget.ID() == "" {
		return "<unnamed>"
	}
	return widget.ID()
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
