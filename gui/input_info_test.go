package gui

import (
	"reflect"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
)

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
