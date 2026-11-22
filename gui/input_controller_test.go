package gui

import (
	"reflect"
	"slices"
	"testing"
	"time"

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
	controller.ConnectMotion(func(_ EventContext, info MotionInfo) {
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
	controller.ConnectPressedChanged(func(value bool) {
		pressed = append(pressed, value)
	})
	controller.ConnectClicked(func(ctx EventContext, _ ClickInfo) {
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
	controller.ConnectPressed(func(ctx EventContext, _ ClickInfo) { ctx.StopPropagation() })
	controller.ConnectClicked(func(ctx EventContext, _ ClickInfo) {
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
	controller.ConnectClicked(func(ctx EventContext, _ ClickInfo) {
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
	controller.ConnectPressedChanged(func(value bool) {
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
	if !slices.Equal(pressed, []bool{true, false}) {
		t.Fatalf("unexpected pressed calls: %v", pressed)
	}
}

func TestClickEventControllerCancelPointerReleasesWithoutClick(t *testing.T) {
	controller := NewClickEventController()
	var pressed []bool
	clicks := 0
	controller.ConnectPressedChanged(func(value bool) { pressed = append(pressed, value) })
	controller.ConnectClicked(func(_ EventContext, _ ClickInfo) {
		clicks++
	})
	controller.HandleEvent(&eventContext{event: events.PointerEvent{
		EventType: events.PointerDown, Button: events.PointerButtonLeft,
	}})
	controller.CancelPointer()
	controller.HandleEvent(&eventContext{event: events.PointerEvent{
		EventType: events.PointerUp, Button: events.PointerButtonLeft,
	}})
	if controller.Pressed() || clicks != 0 || !slices.Equal(pressed, []bool{true, false}) {
		t.Fatalf("cancel: pressed=%t clicks=%d notifications=%v", controller.Pressed(), clicks, pressed)
	}
}

func TestClickEventControllerResetsPressOnPointerContainsLeave(t *testing.T) {
	controller := NewClickEventController()
	var pressed []bool
	controller.ConnectPressedChanged(func(value bool) {
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

func TestDragEventControllerLifecycle(t *testing.T) {
	controller := NewDragEventController()
	var begins, updates, ends []geometry.Point
	var beginMods, updateMods, endMods []events.Modifiers
	controller.ConnectBegin(func(_ EventContext, inputInfo DragInfo) {
		p := inputInfo.Position
		m := inputInfo.Modifiers
		begins = append(begins, p)
		beginMods = append(beginMods, m)
	})
	controller.ConnectUpdate(func(_ EventContext, inputInfo DragInfo) {
		p := inputInfo.Position
		m := inputInfo.Modifiers
		updates = append(updates, p)
		updateMods = append(updateMods, m)
	})
	controller.ConnectEnd(func(_ EventContext, inputInfo DragInfo) {
		p := inputInfo.Position
		m := inputInfo.Modifiers
		ends = append(ends, p)
		endMods = append(endMods, m)
	})

	widget := newTestWidget()
	widget.Arrange(geometry.Rect(0, 0, 100, 100))

	// PointerDown starts the drag.
	down := &eventContext{
		current: widget,
		event: events.PointerEvent{
			EventType: events.PointerDown,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: 20, Y: 30},
			Modifiers: events.ModifierShift,
		},
	}
	controller.HandleEvent(down)

	if !controller.Dragging() {
		t.Fatal("drag should be active after pointer down")
	}
	if !controller.CapturingPointer() {
		t.Fatal("controller should report capturing pointer after pointer down")
	}
	if len(begins) != 1 {
		t.Fatalf("expected 1 begin, got %d", len(begins))
	}
	if begins[0] != (geometry.Point{X: 20, Y: 30}) {
		t.Fatalf("unexpected begin point: %+v", begins[0])
	}
	if beginMods[0] != events.ModifierShift {
		t.Fatalf("unexpected begin modifiers: %v", beginMods[0])
	}

	// PointerMove emits update.
	move := &eventContext{
		current: widget,
		event: events.PointerEvent{
			EventType: events.PointerMove,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: 50, Y: 60},
			Modifiers: events.ModifierControl,
		},
	}
	controller.HandleEvent(move)

	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	if updates[0] != (geometry.Point{X: 50, Y: 60}) {
		t.Fatalf("unexpected update point: %+v", updates[0])
	}
	if updateMods[0] != events.ModifierControl {
		t.Fatalf("unexpected update modifiers: %v", updateMods[0])
	}

	// PointerUp ends the drag.
	up := &eventContext{
		current: widget,
		event: events.PointerEvent{
			EventType: events.PointerUp,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: 80, Y: 90},
			Modifiers: events.ModifierAlt,
		},
	}
	controller.HandleEvent(up)

	if controller.Dragging() {
		t.Fatal("drag should be inactive after pointer up")
	}
	if len(ends) != 1 {
		t.Fatalf("expected 1 end, got %d", len(ends))
	}
	if ends[0] != (geometry.Point{X: 80, Y: 90}) {
		t.Fatalf("unexpected end point: %+v", ends[0])
	}
	if endMods[0] != events.ModifierAlt {
		t.Fatalf("unexpected end modifiers: %v", endMods[0])
	}
}

func TestDragEventControllerIgnoresOtherButtons(t *testing.T) {
	controller := NewDragEventController()
	begins := 0
	controller.ConnectBegin(func(_ EventContext, _ DragInfo) {
		begins++
	})

	controller.HandleEvent(&eventContext{
		event: events.PointerEvent{
			EventType: events.PointerDown,
			Button:    events.PointerButtonRight,
		},
	})

	if controller.Dragging() || begins != 0 {
		t.Fatal("right button should not start a left-button drag")
	}
}

func TestDragEventControllerIgnoresCrossing(t *testing.T) {
	controller := NewDragEventController()

	controller.HandleEvent(&eventContext{
		event: events.PointerEvent{
			EventType: events.PointerDown,
			Button:    events.PointerButtonLeft,
		},
	})
	if !controller.Dragging() {
		t.Fatal("pointer down should start drag")
	}

	// CrossingLeave should not cancel the drag.
	controller.HandleCrossing(&crossingContext{
		crossingType: CrossingPointer,
		mode:         CrossingContains,
		direction:    CrossingLeave,
	})

	if !controller.Dragging() {
		t.Fatal("crossing leave should not cancel drag")
	}
}

func TestDragEventControllerReset(t *testing.T) {
	controller := NewDragEventController()

	controller.HandleEvent(&eventContext{
		event: events.PointerEvent{
			EventType: events.PointerDown,
			Button:    events.PointerButtonLeft,
		},
	})
	if !controller.Dragging() {
		t.Fatal("pointer down should start drag")
	}

	controller.Reset()

	if controller.Dragging() {
		t.Fatal("reset should clear dragging state")
	}
}

func TestDragEventControllerDefaultsToBubblePhase(t *testing.T) {
	controller := NewDragEventController()
	if controller.Phase() != PhaseBubble {
		t.Fatalf("unexpected default phase: %v", controller.Phase())
	}

	controller.SetPhase(PhaseTarget)
	if controller.Phase() != PhaseTarget {
		t.Fatalf("unexpected phase: %v", controller.Phase())
	}
}

func TestDragEventControllerSetButtonResets(t *testing.T) {
	controller := NewDragEventController()

	controller.HandleEvent(&eventContext{
		event: events.PointerEvent{
			EventType: events.PointerDown,
			Button:    events.PointerButtonLeft,
		},
	})
	if !controller.Dragging() {
		t.Fatal("pointer down should start drag")
	}

	controller.SetButton(events.PointerButtonRight)

	if controller.Dragging() {
		t.Fatal("SetButton should reset dragging state")
	}
	if controller.Button() != events.PointerButtonRight {
		t.Fatalf("unexpected button: %v", controller.Button())
	}
}

// --- 输入信息与连续点击 ---

// 真实和模拟输入均经过窗口分发；子控件有两级偏移，不能以原始宿主坐标冒充局部坐标。
func inputInfoFixture(t *testing.T) (*window, *testWidget, func(events.PointerEvent)) {
	t.Helper()
	root, parent, child := newTestWidget(), newTestWidget(), newTestWidget()
	root.AddChild(parent)
	parent.AddChild(child)
	w := testWindowWithSurface(&window{})
	w.SetWidget(root)
	root.Arrange(geometry.Rect(0, 0, 400, 300))
	parent.Arrange(geometry.Rect(20, 30, 300, 240))
	child.Arrange(geometry.Rect(10, 15, 200, 120))
	t.Cleanup(w.Destroy)
	return w, child, func(e events.PointerEvent) {
		t.Helper()
		if err := w.DispatchEvent(e); err != nil {
			t.Fatal(err)
		}
	}
}

func TestClickInfoImmediateCountAndTerminalPhases(t *testing.T) {
	_, child, dispatch := inputInfoFixture(t)
	click := NewClickEventController()
	child.AddEventController(click)
	now := time.Unix(100, 0)
	click.now = func() time.Time { return now }
	var phases []string
	var presses, releases, completions []ClickInfo
	click.ConnectPressedChanged(func(pressed bool) {
		if pressed {
			phases = append(phases, "state-on")
		} else {
			phases = append(phases, "state-off")
		}
	})
	click.ConnectPressed(func(ctx EventContext, info ClickInfo) {
		phases = append(phases, "press")
		presses = append(presses, info)
		if p, ok := ctx.Position(); !ok || p != info.Position {
			t.Fatal("按下信息不在当前控件坐标中")
		}
	})
	click.ConnectReleased(func(_ EventContext, info ClickInfo) {
		phases = append(phases, "release")
		releases = append(releases, info)
	})
	click.ConnectClicked(func(_ EventContext, info ClickInfo) {
		phases = append(phases, "click")
		completions = append(completions, info)
	})
	for count := 1; count <= 4; count++ {
		dispatch(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 35, Y: 51}, Button: events.PointerButtonLeft, Buttons: events.PointerButtonLeftDown, Modifiers: events.ModifierShift})
		want := ClickInfo{Position: geometry.Point{X: 5, Y: 6}, Button: events.PointerButtonLeft, Buttons: events.PointerButtonLeftDown, Modifiers: events.ModifierShift, Count: count}
		if len(presses) != count || presses[count-1] != want || len(completions) != count-1 {
			t.Fatalf("按下阶段次数/时机不正确：%+v %+v", presses, completions)
		}
		dispatch(events.PointerEvent{EventType: events.PointerUp, Position: geometry.Point{X: 37, Y: 53}, Button: events.PointerButtonLeft, Modifiers: events.ModifierControl})
		want.Position, want.Buttons, want.Modifiers = geometry.Point{X: 7, Y: 8}, 0, events.ModifierControl
		if len(releases) != count || len(completions) != count || releases[count-1] != want || completions[count-1] != want {
			t.Fatalf("释放/完成没有沿用次数或没有使用当前输入：%+v %+v", releases, completions)
		}
		now = now.Add(100 * time.Millisecond)
	}
	want := []string{"state-on", "press", "state-off", "release", "click"}
	for i := 0; i < 4; i++ {
		if !reflect.DeepEqual(phases[i*5:i*5+5], want) {
			t.Fatal(phases)
		}
	}
}

func TestClickReleasedOutsideIsNotCanceledOrClicked(t *testing.T) {
	_, child, dispatch := inputInfoFixture(t)
	c := NewClickEventController()
	child.AddEventController(c)
	released, canceled, clicked := 0, 0, 0
	c.ConnectReleased(func(_ EventContext, info ClickInfo) {
		released++
		if info.Position.X != 270 {
			t.Fatal(info)
		}
	})
	c.ConnectCanceled(func(GestureCancelReason) { canceled++ })
	c.ConnectClicked(func(EventContext, ClickInfo) { clicked++ })
	dispatch(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: geometry.Point{X: 35, Y: 51}})
	dispatch(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 300, Y: 51}, Buttons: events.PointerButtonLeftDown})
	dispatch(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: geometry.Point{X: 300, Y: 51}})
	if released != 1 || canceled != 0 || clicked != 0 || c.Pressed() {
		t.Fatalf("移出释放混淆了完成与取消：%d/%d/%d", released, canceled, clicked)
	}
}

func TestClickResetDuringPressStopsOldCallbacks(t *testing.T) {
	_, child, dispatch := inputInfoFixture(t)
	c := NewClickEventController()
	child.AddEventController(c)
	later, released, clicked, canceled := 0, 0, 0, 0
	c.ConnectPressed(func(EventContext, ClickInfo) { c.Reset() })
	c.ConnectPressed(func(EventContext, ClickInfo) { later++ })
	c.ConnectReleased(func(EventContext, ClickInfo) { released++ })
	c.ConnectClicked(func(EventContext, ClickInfo) { clicked++ })
	c.ConnectCanceled(func(reason GestureCancelReason) {
		canceled++
		if reason != GestureInterrupted {
			t.Fatal(reason)
		}
	})
	for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
		dispatch(events.PointerEvent{EventType: kind, Button: events.PointerButtonLeft, Position: geometry.Point{X: 35, Y: 51}})
	}
	if later != 0 || released != 0 || clicked != 0 || canceled != 1 || c.Pressed() {
		t.Fatalf("Reset 后继续旧输入通知：%d/%d/%d/%d", later, released, clicked, canceled)
	}
}

func TestClickResetDuringReleaseDoesNotInventCancel(t *testing.T) {
	_, child, dispatch := inputInfoFixture(t)
	c := NewClickEventController()
	child.AddEventController(c)
	later, clicked, canceled := 0, 0, 0
	c.ConnectReleased(func(EventContext, ClickInfo) { c.Reset() })
	c.ConnectReleased(func(EventContext, ClickInfo) { later++ })
	c.ConnectClicked(func(EventContext, ClickInfo) { clicked++ })
	c.ConnectCanceled(func(GestureCancelReason) { canceled++ })
	for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
		dispatch(events.PointerEvent{EventType: kind, Button: events.PointerButtonLeft, Position: geometry.Point{X: 35, Y: 51}})
	}
	if later != 0 || clicked != 0 || canceled != 0 {
		t.Fatalf("已释放的输入重复终止或继续激活：%d/%d/%d", later, clicked, canceled)
	}
}

func TestThirdPressCanContinueAsDrag(t *testing.T) {
	_, child, dispatch := inputInfoFixture(t)
	click, drag := NewClickEventController(), NewDragEventController()
	drag.SetThreshold(4)
	child.AddEventController(click)
	child.AddEventController(drag)
	now := time.Unix(100, 0)
	click.now = func() time.Time { return now }
	unit, completed, canceled, updates, ends := 0, 0, 0, 0, 0
	click.ConnectPressed(func(_ EventContext, info ClickInfo) { unit = info.Count })
	click.ConnectClicked(func(EventContext, ClickInfo) { completed++ })
	click.ConnectCanceled(func(reason GestureCancelReason) {
		canceled++
		if reason != GestureLost {
			t.Fatal(reason)
		}
	})
	drag.ConnectBegin(func(_ EventContext, info DragInfo) {
		if unit != 3 || info.Position != (geometry.Point{X: 5, Y: 6}) || info.Modifiers != events.ModifierShift {
			t.Fatal(unit, info)
		}
	})
	drag.ConnectUpdate(func(_ EventContext, info DragInfo) {
		updates++
		if unit != 3 || info.Position != (geometry.Point{X: 25, Y: 6}) || info.Modifiers != events.ModifierAlt {
			t.Fatal(unit, info)
		}
	})
	drag.ConnectEnd(func(_ EventContext, info DragInfo) {
		ends++
		if info.Modifiers != events.ModifierControl {
			t.Fatal(info)
		}
	})
	for i := 0; i < 3; i++ {
		dispatch(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 35, Y: 51}, Button: events.PointerButtonLeft, Buttons: events.PointerButtonLeftDown, Modifiers: events.ModifierShift})
		if unit != i+1 || completed != i {
			t.Fatal("必须在释放之前取得本次按下次数", unit, completed)
		}
		if i < 2 {
			dispatch(events.PointerEvent{EventType: events.PointerUp, Position: geometry.Point{X: 35, Y: 51}, Button: events.PointerButtonLeft})
			now = now.Add(100 * time.Millisecond)
		}
	}
	dispatch(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 55, Y: 51}, Buttons: events.PointerButtonLeftDown, Modifiers: events.ModifierAlt})
	dispatch(events.PointerEvent{EventType: events.PointerUp, Position: geometry.Point{X: 55, Y: 51}, Button: events.PointerButtonLeft, Modifiers: events.ModifierControl})
	if completed != 2 || canceled != 1 || updates != 1 || ends != 1 || drag.Dragging() {
		t.Fatalf("三击拖选丢失阶段或生成了点击：%d/%d/%d/%d", completed, canceled, updates, ends)
	}
}

func TestTypedMotionWheelAndContextMenu(t *testing.T) {
	w, child, dispatch := inputInfoFixture(t)
	motion, wheel, menu := NewMotionEventController(), NewWheelEventController(), NewContextMenuEventController()
	menu.goos = "linux"
	child.SetFocusable(true)
	child.AddEventController(motion)
	child.AddEventController(wheel)
	child.AddEventController(menu)
	var m MotionInfo
	var scroll WheelInfo
	var requests []ContextMenuInfo
	motion.ConnectMotion(func(_ EventContext, info MotionInfo) { m = info })
	wheel.ConnectScroll(func(_ EventContext, info WheelInfo) { scroll = info })
	menu.ConnectRequest(func(ctx EventContext, info ContextMenuInfo) { requests = append(requests, info); ctx.StopPropagation() })
	point := geometry.Point{X: 35, Y: 51}
	local := geometry.Point{X: 5, Y: 6}
	dispatch(events.PointerEvent{EventType: events.PointerMove, Position: point, Buttons: events.PointerButtonLeftDown, Modifiers: events.ModifierAlt})
	if m.Position != local || m.Buttons != events.PointerButtonLeftDown || m.Modifiers != events.ModifierAlt {
		t.Fatal(m)
	}
	if err := w.DispatchEvent(events.WheelEvent{Position: point, DeltaX: 2, DeltaY: 3, Mode: events.WheelDeltaLine, Buttons: events.PointerButtonRightDown, Modifiers: events.ModifierControl}); err != nil {
		t.Fatal(err)
	}
	if scroll != (WheelInfo{Position: local, DeltaX: 2, DeltaY: 3, Mode: events.WheelDeltaLine, Buttons: events.PointerButtonRightDown, Modifiers: events.ModifierControl}) {
		t.Fatal(scroll)
	}
	dispatch(events.PointerEvent{EventType: events.PointerDown, Position: point, Button: events.PointerButtonRight, Modifiers: events.ModifierShift})
	w.SetFocusedWidget(child)
	if err := w.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyF10, Modifiers: events.ModifierShift}); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || requests[0] != (ContextMenuInfo{Position: local, HasPosition: true, Modifiers: events.ModifierShift}) || requests[1] != (ContextMenuInfo{Modifiers: events.ModifierShift}) {
		t.Fatal(requests)
	}
}

func TestClickSequenceTimeDistanceAndButtonBoundaries(t *testing.T) {
	t0 := time.Unix(100, 0)
	for _, tc := range []struct {
		name   string
		dt     time.Duration
		point  geometry.Point
		button events.PointerButton
		want   int
	}{
		{"边界内", 500 * time.Millisecond, geometry.Point{X: 4}, events.PointerButtonLeft, 2},
		{"超时", 501 * time.Millisecond, geometry.Point{}, events.PointerButtonLeft, 1},
		{"回退", -time.Millisecond, geometry.Point{}, events.PointerButtonLeft, 1},
		{"超距", time.Millisecond, geometry.Point{X: 5}, events.PointerButtonLeft, 1},
		{"次键", time.Millisecond, geometry.Point{}, events.PointerButtonRight, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var s clickSequence
			if s.press(t0, geometry.Point{}, events.PointerButtonLeft) != 1 {
				t.Fatal("首次不是1")
			}
			if got := s.press(t0.Add(tc.dt), tc.point, tc.button); got != tc.want {
				t.Fatal(got, tc.want)
			}
			s.reset()
			if s.press(t0, geometry.Point{}, events.PointerButtonLeft) != 1 {
				t.Fatal("复位没有清空序列")
			}
		})
	}
}

func TestInputControllerButtonChangePreservesLaterCallbackChoice(t *testing.T) {
	for _, drag := range []bool{false, true} {
		t.Run(map[bool]string{false: "Click", true: "Drag"}[drag], func(t *testing.T) {
			_, child, dispatch := inputInfoFixture(t)
			var set func(events.PointerButton)
			var get func() events.PointerButton
			if drag {
				c := NewDragEventController()
				child.AddEventController(c)
				set, get = c.SetButton, c.Button
				c.ConnectCanceled(func(GestureCancelReason) { c.SetButton(events.PointerButtonMiddle) })
			} else {
				c := NewClickEventController()
				child.AddEventController(c)
				set, get = c.SetButton, c.Button
				c.ConnectCanceled(func(GestureCancelReason) { c.SetButton(events.PointerButtonMiddle) })
			}
			dispatch(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 35, Y: 51}, Button: events.PointerButtonLeft})
			set(events.PointerButtonRight)
			if get() != events.PointerButtonMiddle {
				t.Fatal("较早的 setter 覆盖了取消回调里较晚的选择", get())
			}
		})
	}
}

// --- 上下文菜单手势 ---

// Platform classification coverage moved here from widgets/tab_bar_test.go:
// TreeView, TabBar and editors now share this interpreter.
func TestContextMenuPointer(t *testing.T) {
	for _, goos := range []string{"linux", "windows", "darwin"} {
		if !contextMenuPointer(events.PointerEvent{Button: events.PointerButtonRight}, goos) {
			t.Fatal(goos)
		}
		if got := contextMenuPointer(events.PointerEvent{Button: events.PointerButtonLeft, Modifiers: events.ModifierControl}, goos); got != (goos == "darwin") {
			t.Fatal(goos, got)
		}
		if contextMenuPointer(events.PointerEvent{Button: events.PointerButtonLeft}, goos) {
			t.Fatal(goos)
		}
	}
}

func TestContextMenuTimingAndCancellation(t *testing.T) {
	for _, goos := range []string{"linux", "windows", "darwin"} {
		t.Run(goos, func(t *testing.T) {
			c := NewContextMenuEventController()
			c.goos = goos
			calls := 0
			c.ConnectRequest(func(_ EventContext, _ ContextMenuInfo) {
				calls++
			})
			e := events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonRight}
			c.HandleEvent(&eventContext{event: e})
			want := 1
			if goos == "windows" {
				want = 0
			}
			if calls != want {
				t.Fatalf("press calls=%d want=%d", calls, want)
			}
			e.EventType = events.PointerUp
			c.HandleEvent(&eventContext{event: e})
			if calls != 1 {
				t.Fatalf("release calls=%d", calls)
			}
			c.HandleEvent(&eventContext{event: e})
			if calls != 1 {
				t.Fatal("unmatched release requested a menu")
			}
		})
	}
	c := NewContextMenuEventController()
	c.goos = "windows"
	calls := 0
	c.ConnectRequest(func(_ EventContext, _ ContextMenuInfo) {
		calls++
	})
	for _, cancel := range []func(){c.Reset, func() { c.HandleCrossing(&crossingContext{direction: CrossingLeave}) }, func() {
		c.HandleEvent(&eventContext{event: events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 100}}})
	}} {
		c.HandleEvent(&eventContext{event: events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonRight}})
		cancel()
		c.HandleEvent(&eventContext{event: events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonRight}})
	}
	if calls != 0 {
		t.Fatal("canceled sequence requested a menu")
	}
}

func TestContextMenuBubblesAndKeyboardHasNoPosition(t *testing.T) {
	root, child := newTestWidget(), newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	child.Arrange(geometry.Rect(10, 20, 40, 40))
	root.AddChild(child)
	child.SetFocusable(true)
	parent, local := NewContextMenuEventController(), NewContextMenuEventController()
	parent.goos, local.goos = "linux", "linux"
	root.AddEventController(parent)
	child.AddEventController(local)
	parentCalls, localCalls := 0, 0
	consume := false
	local.ConnectRequest(func(ctx EventContext, _ ContextMenuInfo) {
		localCalls++
		if p, ok := ctx.Position(); ok && p != (geometry.Point{X: 5, Y: 7}) {
			t.Fatal(p)
		}
		if consume {
			ctx.StopPropagation()
		}
	})
	parent.ConnectRequest(func(_ EventContext, _ ContextMenuInfo) {
		parentCalls++
	})
	w := testWindowWithSurface(&window{})
	w.SetWidget(root)
	e := events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonRight, Position: geometry.Point{X: 15, Y: 27}}
	_ = w.DispatchEvent(e)
	consume = true
	_ = w.DispatchEvent(e)
	if localCalls != 2 || parentCalls != 1 {
		t.Fatal(localCalls, parentCalls)
	}
	w.SetFocusedWidget(child)
	local.ConnectRequest(func(ctx EventContext, _ ContextMenuInfo) {
		if _, ok := ctx.Position(); ok {
			t.Fatal("keyboard request has a position")
		}
	})
	k := events.KeyEvent{EventType: events.KeyDown, Key: events.KeyF10, Modifiers: events.ModifierShift}
	_ = w.DispatchEvent(k)
	k.Repeat = true
	_ = w.DispatchEvent(k)
	if localCalls != 3 || parentCalls != 1 {
		t.Fatal(localCalls, parentCalls)
	}
}

// --- 点击与拖动的手势竞争 ---

func TestGestureThresholdMoveDeliveredAfterBegin(t *testing.T) {
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	win := testWindowWithSurface(&window{})
	win.SetWidget(root)
	drag := NewDragEventController()
	drag.SetThreshold(4)
	root.AddEventController(drag)
	var points []geometry.Point
	drag.ConnectBegin(func(_ EventContext, inputInfo DragInfo) {
		p := inputInfo.Position
		points = append(points, p)
	})
	drag.ConnectUpdate(func(_ EventContext, inputInfo DragInfo) {
		p := inputInfo.Position
		points = append(points, p)
	})
	start, far := geometry.Point{X: 10, Y: 10}, geometry.Point{X: 200, Y: 150}
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft,
		Buttons: events.PointerButtonLeftDown, Position: start})
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Buttons: events.PointerButtonLeftDown, Position: far})
	if len(points) != 2 || points[0] != start || points[1] != far {
		t.Fatalf("threshold crossing lost its position: %v", points)
	}
}

func TestGestureClickAndParentDrag(t *testing.T) {
	for _, drag := range []bool{false, true} {
		root, child := newTestWidget(), newTestWidget()
		root.Arrange(geometry.Rect(0, 0, 100, 100))
		child.Arrange(geometry.Rect(10, 10, 40, 40))
		root.AddChild(child)
		click := NewClickEventController()
		child.AddEventController(click)
		parentDrag := NewDragEventController()
		parentDrag.SetThreshold(4)
		root.AddEventController(parentDrag)
		clicks, begins, ends := 0, 0, 0
		click.ConnectClicked(func(_ EventContext, _ ClickInfo) {
			clicks++
		})
		parentDrag.ConnectBegin(func(_ EventContext, _ DragInfo) {
			begins++
		})
		parentDrag.ConnectEnd(func(_ EventContext, _ DragInfo) {
			ends++
		})
		win := testWindowWithSurface(&window{surfaceBase: surfaceBase{root: root}})
		point := geometry.Point{X: 20, Y: 20}
		_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: point})
		if !click.Pressed() || begins != 0 {
			t.Fatal("press should remain undecided")
		}
		if drag {
			point.X += 8
			_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Buttons: events.PointerButtonLeftDown, Position: point})
			if click.Pressed() || begins != 1 {
				t.Fatal("drag did not cancel click and begin")
			}
		}
		_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: point})
		if drag && (clicks != 0 || begins != 1 || ends != 1) {
			t.Fatalf("drag result: clicks=%d begin=%d end=%d", clicks, begins, ends)
		}
		if !drag && (clicks != 1 || begins != 0 || ends != 0) {
			t.Fatalf("click result: clicks=%d begin=%d end=%d", clicks, begins, ends)
		}
	}
}

func TestGestureSecondClickCanBecomeDrag(t *testing.T) {
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	click, drag := NewClickEventController(), NewDragEventController()
	drag.SetThreshold(4)
	root.AddEventController(click)
	root.AddEventController(drag)
	now := time.Unix(100, 0)
	click.now = func() time.Time { return now }
	var counts []int
	click.ConnectClicked(func(_ EventContext, inputInfo ClickInfo) {
		count := inputInfo.Count
		counts = append(counts, count)
	})
	win := testWindowWithSurface(&window{surfaceBase: surfaceBase{root: root}})
	press := func(x float32) {
		_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: geometry.Point{X: x, Y: 20}})
	}
	release := func(x float32) {
		_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: geometry.Point{X: x, Y: 20}})
	}
	press(20)
	release(20)
	now = now.Add(100 * time.Millisecond)
	press(20)
	release(20)
	if len(counts) != 2 || counts[0] != 1 || counts[1] != 2 {
		t.Fatalf("click counts = %v", counts)
	}
	now = now.Add(100 * time.Millisecond)
	press(20)
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 30, Y: 20}, Buttons: events.PointerButtonLeftDown})
	release(30)
	if len(counts) != 2 || drag.Dragging() {
		t.Fatalf("second press did not become a finished drag: %v", counts)
	}
	now = now.Add(100 * time.Millisecond)
	press(20)
	release(20)
	if len(counts) != 3 || counts[2] != 1 {
		t.Fatalf("drag left a stale double-click candidate: %v", counts)
	}
}

func TestGestureClickStopsAfterResultCallbackInvalidatesWidget(t *testing.T) {
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 40, 40))
	click := NewClickEventController()
	root.AddEventController(click)
	results, compatibility := 0, 0
	click.ConnectClicked(func(_ EventContext, _ ClickInfo) {
		results++
		root.SetVisible(false)
	})
	click.ConnectClicked(func(_ EventContext, _ ClickInfo) {
		compatibility++
	})
	win := testWindowWithSurface(&window{surfaceBase: surfaceBase{root: root}})
	point := geometry.Point{X: 10, Y: 10}
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: point})
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: point})
	if results != 1 || compatibility != 0 {
		t.Fatalf("invalidated widget received stale callback: result=%d compatibility=%d", results, compatibility)
	}
}

func TestGestureLostObservedButtonCancelsPress(t *testing.T) {
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 40, 40))
	click := NewClickEventController()
	root.AddEventController(click)
	clicked := 0
	click.ConnectClicked(func(_ EventContext, _ ClickInfo) {
		clicked++
	})
	win := testWindowWithSurface(&window{surfaceBase: surfaceBase{root: root}})
	point := geometry.Point{X: 10, Y: 10}
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft,
		Buttons: events.PointerButtonLeftDown, Position: point})
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point})
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: point})
	if click.Pressed() || clicked != 0 {
		t.Fatalf("lost release caused a click: pressed=%v clicked=%d", click.Pressed(), clicked)
	}
}

func TestGestureUnmountCancelsPressImmediately(t *testing.T) {
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 40, 40))
	click := NewClickEventController()
	root.AddEventController(click)
	win := testWindowWithSurface(&window{})
	win.SetWidget(root)
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown,
		Button: events.PointerButtonLeft, Position: geometry.Point{X: 10, Y: 10}})
	if !click.Pressed() {
		t.Fatal("press was not established")
	}
	win.SetWidget(nil)
	if click.Pressed() || win.dispatcher.gesture != nil {
		t.Fatal("unmount left a live pointer sequence")
	}
}

func TestGestureSameEventUsesDepthThenRegistrationOrder(t *testing.T) {
	root, child := newTestWidget(), newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	child.Arrange(geometry.Rect(10, 10, 50, 50))
	root.AddChild(child)
	parent, first, second := NewDragEventController(), NewDragEventController(), NewDragEventController()
	second.SetPhase(PhaseCapture) // Delivered first, but registered after first.
	root.AddEventController(parent)
	child.AddEventController(first)
	child.AddEventController(second)
	counts := [3]int{}
	parent.ConnectBegin(func(_ EventContext, _ DragInfo) {
		counts[0]++
	})
	first.ConnectBegin(func(_ EventContext, _ DragInfo) {
		counts[1]++
	})
	second.ConnectBegin(func(_ EventContext, _ DragInfo) {
		counts[2]++
	})
	win := testWindowWithSurface(&window{surfaceBase: surfaceBase{root: root}})
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: geometry.Point{X: 20, Y: 20}})
	if counts != [3]int{0, 1, 0} {
		t.Fatalf("winner = %v", counts)
	}
}

type gestureProbe struct {
	EventControllerBase
	part     *GestureParticipation
	accepted int
	canceled int
	onCancel func()
}

func (p *gestureProbe) HandleEvent(ctx EventContext) {
	if p.part = JoinGesture(ctx); p.part != nil {
		p.part.Claim()
	}
}
func (p *gestureProbe) GestureAccepted(EventContext) { p.accepted++ }
func (p *gestureProbe) GestureCanceled(GestureCancelReason) {
	p.canceled++
	if p.onCancel != nil {
		p.onCancel()
	}
}

func TestGestureWinnerInvalidatedByLoserCancellation(t *testing.T) {
	root, child := newTestWidget(), newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	child.Arrange(geometry.Rect(10, 10, 40, 40))
	root.AddChild(child)
	parent := &gestureProbe{EventControllerBase: NewEventControllerBase(PhaseCapture)}
	winner := &gestureProbe{EventControllerBase: NewEventControllerBase(PhaseTarget)}
	parent.onCancel = func() { child.SetVisible(false) }
	root.AddEventController(parent)
	child.AddEventController(winner)
	win := testWindowWithSurface(&window{surfaceBase: surfaceBase{root: root}})
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: geometry.Point{X: 20, Y: 20}})
	if parent.accepted != 0 || parent.canceled != 1 || winner.accepted != 0 || winner.canceled != 1 {
		t.Fatalf("parent=%+v winner=%+v", parent, winner)
	}
}
