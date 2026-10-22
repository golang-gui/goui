package gui

import (
	"errors"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/events"
)

// 通过正常 Window.DispatchEvent 验证已获胜手势的接管，不伪造参与者状态。
func TestDragSourceHandoff(t *testing.T) {
	for _, mode := range []string{"async", "sync", "startup-error"} {
		t.Run(mode, func(t *testing.T) {
			native := new(gestureDragNative)
			app := &application{platform: &gestureDragPlatform{native: native}}
			win := &window{rootBase: rootBase{app: app, surface: &recordingPlatformPopup{}}}
			root := newTestWidget()
			root.Arrange(geometry.Rect(0, 0, 100, 100))
			win.SetWidget(root)
			defer win.Destroy()
			click, drag := NewClickEventController(), NewDragEventController()
			drag.SetThreshold(4)
			root.AddEventController(click)
			root.AddEventController(drag)
			source := NewDragSource() // 不注册第二个手势参与者
			source.SetActions(DragMove)
			data := new(DragData)
			data.SetLocal("page", root)
			clicks, pointerEnds, pointerCancels, prepares, begins, ends := 0, 0, 0, 0, 0, 0
			click.ConnectClicked(func(EventContext) { clicks++ })
			drag.ConnectEnd(func(geometry.Point, events.Modifiers) { pointerEnds++ })
			drag.ConnectCancel(func() { pointerCancels++ })
			source.ConnectPrepare(func(*DragPrepare) { prepares++ })
			source.ConnectBegin(func() { begins++ })
			source.ConnectEnd(func(DragResult) { ends++ })
			var beginErr error
			drag.ConnectUpdate(func(point geometry.Point, _ events.Modifiers) {
				if point.X < 40 {
					return
				}
				beginErr = source.Begin(drag, data, DragPreview{})
			})
			native.onBegin = func(id uint64) {
				if drag.Dragging() || win.dispatcher.gesture != nil || win.dispatcher.captureTarget != nil {
					t.Fatal("GUI pointer routing still active inside native Begin")
				}
				if app.dragSession == nil || source.run != app.dragSession {
					t.Fatal("session not installed before native Begin")
				}
				if mode == "startup-error" {
					return
				}
				_ = win.DispatchEvent(events.DragSourceEvent{EventType: events.DragSourceBegin, ID: id})
				if mode == "sync" {
					_ = win.DispatchEvent(events.DragSourceEvent{EventType: events.DragSourceEnd, ID: id, Result: dragdrop.Result{Canceled: true}})
				}
			}
			if mode != "async" {
				native.beginErr = errors.New("native startup/completion error")
			}
			dispatch := func(kind events.EventType, x float32) {
				t.Helper()
				if err := win.DispatchEvent(events.PointerEvent{EventType: kind, Button: events.PointerButtonLeft,
					Buttons: events.PointerButtonLeftDown, Position: geometry.Point{X: x, Y: 20}}); err != nil {
					t.Fatal(err)
				}
			}
			if source.Begin(drag, data, DragPreview{}) == nil {
				t.Fatal("started outside a drag")
			}
			dispatch(events.PointerDown, 20)
			dispatch(events.PointerMove, 30) // 赢得栏内手势
			if !drag.Dragging() || native.begin != 0 {
				t.Fatal("local drag must start first")
			}
			if source.Begin(drag, data, DragPreview{}) == nil {
				t.Fatal("handoff from outside Update was accepted")
			}
			dispatch(events.PointerMove, 40) // Update 内接管
			if (beginErr != nil) != (mode == "startup-error") || native.begin != 1 {
				t.Fatalf("Begin=%v, calls=%d", beginErr, native.begin)
			}
			if mode == "async" {
				data.SetLocal("page", nil)
				if value, _ := app.dragSession.data.Local("page"); value != root {
					t.Fatal("drag data not frozen")
				}
				id := app.dragSession.id
				source.Cancel()
				if native.cancels != 1 {
					t.Fatal("unattached source cannot cancel")
				}
				_ = win.DispatchEvent(events.DragSourceEvent{EventType: events.DragSourceEnd, ID: id, Result: dragdrop.Result{Canceled: true}})
				_ = win.DispatchEvent(events.DragSourceEvent{EventType: events.DragSourceEnd, ID: id})
			}
			dispatch(events.PointerUp, 40)
			want := 1
			if mode == "startup-error" {
				want = 0
			}
			if begins != want || ends != want || prepares != 0 || clicks != 0 || pointerEnds != 0 || pointerCancels != 0 || source.run != nil || app.dragSession != nil {
				t.Fatalf("begin/end=%d/%d prepare=%d click=%d pointer=%d/%d session=%p", begins, ends, prepares, clicks, pointerEnds, pointerCancels, app.dragSession)
			}
		})
	}
}

func TestDragHandoffValidationPreservesGesture(t *testing.T) {
	native := new(gestureDragNative)
	app := &application{platform: &gestureDragPlatform{native: native}}
	win := &window{rootBase: rootBase{app: app, surface: &recordingPlatformPopup{}}}
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	win.SetWidget(root)
	defer win.Destroy()
	drag, source := NewDragEventController(), NewDragSource()
	root.AddEventController(drag)
	ends, checks := 0, 0
	drag.ConnectEnd(func(geometry.Point, events.Modifiers) { ends++ })
	drag.ConnectUpdate(func(geometry.Point, events.Modifiers) {
		checks++
		if source.Begin(drag, nil, DragPreview{}) == nil || !drag.Dragging() {
			t.Fatal("invalid data consumed local gesture")
		}
	})
	for _, kind := range []events.EventType{events.PointerDown, events.PointerMove, events.PointerUp} {
		_ = win.DispatchEvent(events.PointerEvent{EventType: kind, Button: events.PointerButtonLeft,
			Buttons: events.PointerButtonLeftDown, Position: geometry.Point{X: 10, Y: 10}})
	}
	if checks != 1 || ends != 1 || native.begin != 0 {
		t.Fatalf("checks=%d local ends=%d native starts=%d", checks, ends, native.begin)
	}
}
