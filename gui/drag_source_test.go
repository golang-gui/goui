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
			win := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{app: app, surface: &recordingPlatformPopup{}}}})
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
			click.ConnectClicked(func(_ EventContext, _ ClickInfo) {
				clicks++
			})
			drag.ConnectEnd(func(_ EventContext, _ DragInfo) {
				pointerEnds++
			})
			drag.ConnectCanceled(func(_ GestureCancelReason) {
				pointerCancels++
			})
			source.ConnectPrepare(func(*DragPrepare) { prepares++ })
			source.ConnectBegin(func() { begins++ })
			source.ConnectEnd(func(DragResult) { ends++ })
			var beginErr error
			drag.ConnectUpdate(func(_ EventContext, inputInfo DragInfo) {
				point := inputInfo.Position
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
	win := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{app: app, surface: &recordingPlatformPopup{}}}})
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	win.SetWidget(root)
	defer win.Destroy()
	drag, source := NewDragEventController(), NewDragSource()
	root.AddEventController(drag)
	ends, checks := 0, 0
	drag.ConnectEnd(func(_ EventContext, _ DragInfo) {
		ends++
	})
	drag.ConnectUpdate(func(_ EventContext, _ DragInfo) {
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

// 容器代表稳定的视图，子控件代表可回收行。所有手势／原生终态均经过Window.DispatchEvent。
// 原生替身只记录Cancel请求，直到显式终态才结束会话，不创建桌面窗口。
type dragLifetimeFixture struct {
	t            *testing.T
	win          *window
	app          *application
	root, view   *testWidget
	row          *testWidget
	source       *DragSource
	native       *gestureDragNative
	begins       int
	results      []DragResult
	preparedData *DragData
}

func newDragLifetimeFixture(t *testing.T, rowSource bool) *dragLifetimeFixture {
	t.Helper()
	f := &dragLifetimeFixture{t: t, root: newTestWidget(), view: newTestWidget(), row: newTestWidget(), source: NewDragSource(), native: new(gestureDragNative)}
	f.root.Arrange(geometry.Rect(0, 0, 240, 160))
	f.view.Arrange(geometry.Rect(10, 10, 200, 120))
	f.row.Arrange(geometry.Rect(0, 0, 200, 30))
	f.row.SetID("original-node")
	f.root.AddChild(f.view)
	f.view.AddChild(f.row)
	owner := f.view
	if rowSource {
		owner = f.row
	}
	owner.AddEventController(f.source)
	f.source.SetActions(DragMove)
	f.source.ConnectPrepare(func(r *DragPrepare) {
		f.preparedData = new(DragData)
		f.preparedData.SetText("original transport")
		f.preparedData.SetLocal("node", f.row.ID())
		r.Data = f.preparedData
	})
	f.source.ConnectBegin(func() { f.begins++ })
	f.source.ConnectEnd(func(r DragResult) { f.results = append(f.results, r) })
	f.app = &application{platform: &gestureDragPlatform{native: f.native}}
	f.win = testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{app: f.app, surface: &recordingPlatformPopup{}}}})
	f.win.SetWidget(f.root)
	f.native.onBegin = func(id uint64) { f.dispatch(events.DragSourceEvent{EventType: events.DragSourceBegin, ID: id}) }
	t.Cleanup(func() {
		if f.app.dragSession != nil {
			f.end(f.app.dragSession.id)
		}
		f.win.Destroy()
	})
	return f
}

func (f *dragLifetimeFixture) dispatch(e events.Event) {
	f.t.Helper()
	if err := f.win.DispatchEvent(e); err != nil {
		f.t.Fatal(err)
	}
}
func (f *dragLifetimeFixture) start() *guiDragSession {
	f.t.Helper()
	f.dispatch(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 20, Y: 20}, Button: events.PointerButtonLeft, Buttons: events.PointerButtonLeftDown})
	f.dispatch(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 36, Y: 20}, Buttons: events.PointerButtonLeftDown})
	if f.app.dragSession == nil || !f.source.Dragging() {
		f.t.Fatal("正常指针手势没有启动原生会话")
	}
	return f.app.dragSession
}
func (f *dragLifetimeFixture) end(id uint64) {
	f.dispatch(events.DragSourceEvent{EventType: events.DragSourceEnd, ID: id, Result: dragdrop.Result{Canceled: true}})
}
func (f *dragLifetimeFixture) drop(fn func(*DropRequest)) *testDragOffer {
	f.t.Helper()
	target := NewDropTarget(LocalFormat("node"))
	target.SetActions(DragMove)
	target.ConnectDrop(fn)
	f.root.AddEventController(target)
	offer := &testDragOffer{id: 8, sourceID: f.app.dragSession.id, formats: []dragdrop.Format{dragdrop.FormatLocalMarker}}
	for _, kind := range []events.EventType{events.DragEnter, events.DragDrop} {
		f.dispatch(events.DragOfferEvent{EventType: kind, Offer: offer, Position: geometry.Point{X: 36, Y: 20}, Actions: dragdrop.Move})
	}
	return offer
}

func TestDragSourceSessionBelongsToSourceWidget(t *testing.T) {
	for _, rowSource := range []bool{false, true} {
		name := "稳定视图源"
		if rowSource {
			name = "可回收行源负对照"
		}
		t.Run(name, func(t *testing.T) {
			f := newDragLifetimeFixture(t, rowSource)
			run := f.start()
			f.view.RemoveChild(f.row)
			f.row.SetID("recycled-node")
			f.view.AddChild(f.row)
			// 已启动的数据不重新读取复用行，也不受原Data容器后续写入影响。
			f.preparedData.SetText("changed transport")
			f.preparedData.SetLocal("node", "changed-node")
			id, ok := run.data.Local("node")
			text, textOK := run.data.Text()
			if !ok || id != "original-node" || !textOK || text != "original transport" || f.app.dragSession != run || len(f.results) != 0 || f.begins != 1 {
				t.Fatal("回收／复用污染了会话数据或提前结束会话")
			}
			if rowSource {
				if f.native.cancels == 0 {
					t.Fatal("负对照：直接挂在行上的源卸载后没有请求取消")
				}
				f.end(run.id)
				if len(f.results) != 1 || !f.results[0].Canceled {
					t.Fatal("负对照没有收到取消终态")
				}
				return
			}
			if f.native.cancels != 0 || run.widget != f.view || !f.view.Snapshot().DragDrop.Dragging || f.row.Snapshot().DragDrop != nil {
				t.Fatal("子行回收取消了视图会话，或拖拽语义错挂到复用行")
			}
			drops := 0
			offer := f.drop(func(r *DropRequest) {
				drops++
				if id, ok := r.Data.Local("node"); !ok || id != "original-node" {
					t.Fatal("Drop接收到复用行的数据")
				}
				r.Accepted = true
			})
			f.end(run.id) // 已确认的本地接收优先于原生取消结果。
			f.end(run.id)
			if drops != 1 || len(offer.finished) != 1 || offer.finished[0] != DragMove || len(f.results) != 1 || f.results[0].Action != DragMove || f.results[0].Canceled || !f.results[0].LocalDrop || f.app.dragSession != nil || f.source.run != nil || run.data != nil || f.view.Snapshot().DragDrop.Dragging {
				t.Fatal("行回收后的Drop／恰好一次终态／释放不符合契约")
			}
		})
	}
}

func TestDragSourceOwnerInvalidationWaitsForNativeEnd(t *testing.T) {
	for _, action := range []string{"卸载视图", "移除源控制器", "禁用源控制器"} {
		t.Run(action, func(t *testing.T) {
			f := newDragLifetimeFixture(t, false)
			run := f.start()
			switch action {
			case "卸载视图":
				f.root.RemoveChild(f.view)
			case "移除源控制器":
				f.view.RemoveEventController(f.source)
			case "禁用源控制器":
				f.source.SetEnabled(false)
			}
			if f.native.cancels == 0 || f.app.dragSession != run || run.data == nil || len(f.results) != 0 {
				t.Fatal("所属源失效没有请求取消，或原生终态前提前释放了会话")
			}
			f.end(run.id)
			f.end(run.id)
			if len(f.results) != 1 || !f.results[0].Canceled || f.results[0].Action != 0 || f.results[0].Err != nil || f.app.dragSession != nil || f.source.run != nil || run.data != nil || run.unmount != nil || f.source.Dragging() {
				t.Fatal("取消终态没有恰好一次通知并释放会话")
			}
			if f.view.Parent() == nil {
				f.root.AddChild(f.view)
			}
			if action == "移除源控制器" {
				f.view.AddEventController(f.source)
			}
			f.source.SetEnabled(true)
			next := f.start()
			f.end(run.id)
			if next.id == run.id || f.app.dragSession != next || f.native.begin != 2 || f.begins != 2 || len(f.results) != 1 {
				t.Fatal("后续拖拽没有启动，或迟到终态结束了新会话")
			}
			f.end(next.id)
			if len(f.results) != 2 || f.app.dragSession != nil {
				t.Fatal("下一会话没有释放")
			}
		})
	}
}

func TestDragSourceOwnerUnmountDuringLocalDrop(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		name := "拒绝后取消"
		if accepted {
			name = "接收后保留成功"
		}
		t.Run(name, func(t *testing.T) {
			f := newDragLifetimeFixture(t, false)
			run := f.start()
			offer := f.drop(func(r *DropRequest) {
				f.root.RemoveChild(f.view)
				if f.native.cancels != 0 || len(f.results) != 0 {
					t.Fatal("Drop回调返回前向原生取消或投递了End")
				}
				r.Accepted = accepted
			})
			if (f.native.cancels == 0) != accepted {
				t.Fatal("同步接收决定没有控制待取消请求")
			}
			f.end(run.id)
			want := DragAction(0)
			if accepted {
				want = DragMove
			}
			if len(offer.finished) != 1 || offer.finished[0] != want || len(f.results) != 1 || f.results[0].Canceled == accepted || f.results[0].Action != want || !f.results[0].LocalDrop || f.app.dragSession != nil {
				t.Fatal("所属视图卸载后的本地提交结果不正确")
			}
		})
	}
}
