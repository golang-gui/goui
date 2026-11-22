package gui

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

func TestDragDataFrozenRepresentations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.txt")
	files := []string{path}
	bits := []byte{1, 2, 3}
	local := &struct{ Number int }{Number: 4}
	data := new(DragData)
	data.SetFiles(files)
	data.SetBytes("application/x-goui-test", bits)
	data.SetLocal("record", local)
	files[0] = "changed"
	bits[0] = 9
	if err := data.validate(); err != nil {
		t.Fatal(err)
	}
	frozen := data.freeze()
	data.SetFiles([]string{"other"})
	data.SetBytes("application/x-goui-test", []byte{7})
	data.SetLocal("record", nil)
	gotFiles, ok := frozen.Files()
	if !ok || !reflect.DeepEqual(gotFiles, []string{path}) {
		t.Fatalf("frozen files %v %t", gotFiles, ok)
	}
	gotBytes, ok := frozen.Bytes("application/x-goui-test")
	if !ok || !bytes.Equal(gotBytes, []byte{1, 2, 3}) {
		t.Fatalf("frozen bytes %v %t", gotBytes, ok)
	}
	value, ok := frozen.Local("record")
	if !ok || value != local {
		t.Fatalf("frozen local identity %v %t", value, ok)
	}
	if got := frozen.selected(LocalFormat("record")); !got.contains(LocalFormat("record")) || got.contains(DragFormatFiles) {
		t.Fatalf("selected local leaked representations: %v", got.formats())
	}
}

func TestDragDataRejectsInvalidLocalAndEmpty(t *testing.T) {
	if LocalFormat("") != "" || LocalFormat("x\x00y") != "" {
		t.Fatal("invalid local names accepted")
	}
	if err := (new(DragData)).validate(); err == nil {
		t.Fatal("empty data accepted")
	}
	data := new(DragData)
	data.SetLocal("bad\x00name", 1)
	if err := data.validate(); err == nil {
		t.Fatal("invalid local name accepted")
	}
	data = new(DragData)
	data.SetLocal("nil-value", nil)
	if err := data.validate(); err != nil {
		t.Fatal(err)
	}
	if value, ok := data.Local("nil-value"); !ok || value != nil {
		t.Fatalf("nil value and absent confused: %v %t", value, ok)
	}
}

// --- 拖拽源、手势交接与会话生命周期 ---

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

// --- 拖放派发与提交 ---

type testDragOffer struct {
	id       uint64
	sourceID uint64
	onFinish func()
	formats  []dragdrop.Format
	read     dragdrop.Format
	finished []dragdrop.Action
}

func TestDragMotionLocalIsScopedToMatchingSessionAndFormat(t *testing.T) {
	app := &application{}
	win := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{app: app}}})
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	win.SetWidget(root)
	defer win.Destroy()
	payload := new(int)
	data := new(DragData)
	data.SetLocal("page", payload)
	data.SetLocal("other", new(int))
	app.dragSession = &guiDragSession{app: app, id: 42, data: data, source: NewDragSource()}
	target := NewDropTarget(LocalFormat("page"))
	root.AddEventController(target)
	var held *DragMotion
	calls := 0
	inspect := func(e *DragMotion) {
		calls++
		held = e
		if got, ok := e.Local("page"); !ok || got != payload {
			t.Fatal("matching local payload unavailable")
		}
		if _, ok := e.Local("other"); ok {
			t.Fatal("unselected format exposed")
		}
	}
	target.ConnectEnter(inspect)
	target.ConnectMotion(inspect)
	for _, id := range []uint64{42, 42, 99} {
		offer := &testDragOffer{id: 7, sourceID: id, formats: []dragdrop.Format{dragdrop.FormatLocalMarker}}
		_ = win.DispatchEvent(events.DragOfferEvent{EventType: events.DragMotion, Offer: offer, Position: geometry.Point{X: 10, Y: 10}, Actions: dragdrop.Copy})
		if held != nil {
			if _, ok := held.Local("page"); ok {
				t.Fatal("expired request retained local access")
			}
		}
	}
	if calls != 2 {
		t.Fatalf("foreign offer was exposed, callbacks=%d", calls)
	}
}

func (o *testDragOffer) ID() uint64                   { return o.id }
func (o *testDragOffer) SourceID() uint64             { return o.sourceID }
func (o *testDragOffer) Formats() []dragdrop.Format   { return slices.Clone(o.formats) }
func (o *testDragOffer) Read(f dragdrop.Format) error { o.read = f; return nil }
func (o *testDragOffer) Finish(a dragdrop.Action) error {
	o.finished = append(o.finished, a)
	if o.onFinish != nil {
		o.onFinish()
	}
	return nil
}

func TestLocalDropCommitSurvivesReentrantEnd(t *testing.T) {
	for _, stage := range []string{"drop", "leave", "finish", "source-destroy", "source-destroy-after-drop"} {
		for _, accepted := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/accepted=%t", stage, accepted), func(t *testing.T) {
				app := &application{}
				win := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{app: app}}})
				root := newTestWidget()
				root.Arrange(geometry.Rect(0, 0, 100, 100))
				win.SetWidget(root)
				source := NewDragSource()
				run := &guiDragSession{app: app, id: 42, source: source, data: new(DragData)}
				app.dragSession = run
				source.run = run
				var results []DragResult
				source.ConnectEnd(func(result DragResult) { results = append(results, result) })
				var sourceWindow *window
				end := func() {
					event := events.DragSourceEvent{EventType: events.DragSourceEnd, ID: run.id, Result: dragdrop.Result{Canceled: true}}
					if sourceWindow != nil {
						// 即使源已销毁，终态仍须经过原宿主的正常事件通道。
						_ = sourceWindow.DispatchEvent(event)
					} else {
						app.dispatchDragSourceEvent(event)
					}
					if run.delivering && len(results) != 0 {
						t.Fatal("End was emitted before synchronous Drop/Finish returned")
					}
				}
				if stage == "source-destroy" || stage == "source-destroy-after-drop" {
					sourceWindow = testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{app: app}}})
					sourceWindow.drag.native = &gestureDragNative{onDestroy: end}
					defer sourceWindow.Destroy()
				}
				target := NewDropTarget(DragFormatText)
				root.AddEventController(target)
				target.ConnectDrop(func(e *DropRequest) {
					if stage == "drop" {
						end()
					}
					if stage == "source-destroy" {
						sourceWindow.Destroy()
					}
					e.Accepted = accepted
				})
				if stage == "leave" {
					target.ConnectLeave(end)
				}
				offer := &testDragOffer{id: 7, sourceID: run.id, formats: []dragdrop.Format{dragdrop.FormatText}}
				if stage == "finish" {
					offer.onFinish = end
				}
				for _, kind := range []events.EventType{events.DragEnter, events.DragDrop} {
					if err := win.DispatchEvent(events.DragOfferEvent{EventType: kind, Offer: offer, Position: geometry.Point{X: 10, Y: 10}, Actions: dragdrop.Copy}); err != nil {
						t.Fatal(err)
					}
				}
				data := new(dragdrop.Data)
				data.SetText("committed payload")
				if err := win.DispatchEvent(events.DragDataEvent{OfferID: offer.id, Format: dragdrop.FormatText, Data: data}); err != nil {
					t.Fatal(err)
				}
				if stage == "source-destroy-after-drop" {
					if app.dragSession != run || len(results) != 0 {
						t.Fatal("原生结束前提前释放了会话")
					}
					sourceWindow.Destroy()
				}
				want := DragAction(0)
				if accepted {
					want = DragCopy
				}
				if sourceWindow != nil && len(results) != 1 {
					t.Fatal("源窗口销毁后未通过正常事件通道收到终态")
				}
				run.end(DragResult{Canceled: true}) // duplicate terminal callbacks are ignored
				if len(results) != 1 || !results[0].LocalDrop || results[0].Action != want || results[0].Canceled == accepted || results[0].Validate() != nil || app.dragSession != nil || source.run != nil || run.data != nil || !slices.Equal(offer.finished, []dragdrop.Action{want}) {
					t.Fatalf("results=%+v finish=%v session=%p data=%p", results, offer.finished, app.dragSession, run.data)
				}
				if sourceWindow != nil {
					// 通过正常指针手势开始下一会话；旧终态不得清除新会话。
					native := new(gestureDragNative)
					win.drag.native = native
					next := NewDragSource()
					next.ConnectPrepare(func(request *DragPrepare) {
						request.Data = new(DragData)
						request.Data.SetText("next session")
					})
					root.AddEventController(next)
					for _, event := range []events.PointerEvent{
						{EventType: events.PointerDown, Position: geometry.Point{X: 20, Y: 20}},
						{EventType: events.PointerMove, Position: geometry.Point{X: 40, Y: 20}},
					} {
						event.Button, event.Buttons = events.PointerButtonLeft, events.PointerButtonLeftDown
						if err := win.DispatchEvent(event); err != nil {
							t.Fatal(err)
						}
					}
					if native.begin != 1 || app.dragSession == nil || next.run != app.dragSession {
						t.Fatal("源窗口关闭后的下一次拖拽未启动")
					}
					id := app.dragSession.id
					_ = sourceWindow.DispatchEvent(events.DragSourceEvent{EventType: events.DragSourceEnd, ID: run.id})
					if app.dragSession == nil || app.dragSession.id != id || len(results) != 1 {
						t.Fatal("迟到终态结束了新会话或重复通知旧源")
					}
					_ = win.DispatchEvent(events.DragSourceEvent{EventType: events.DragSourceEnd, ID: id, Result: dragdrop.Result{Canceled: true}})
					if app.dragSession != nil || next.run != nil {
						t.Fatal("下一会话未正常释放")
					}
				}
			})
		}
	}
}

func TestDragResultLocalDropIsDeliveryNotHover(t *testing.T) {
	for _, tc := range []struct {
		name          string
		drop, foreign bool
	}{{"hover only", false, false}, {"local rejected", true, false}, {"foreign rejected", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			app := &application{}
			win := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{app: app}}})
			root := newTestWidget()
			root.Arrange(geometry.Rect(0, 0, 100, 100))
			win.SetWidget(root)
			defer win.Destroy()
			source := NewDragSource()
			run := &guiDragSession{app: app, id: 42, source: source, data: new(DragData)}
			app.dragSession = run
			var result DragResult
			source.ConnectEnd(func(r DragResult) { result = r })
			offer := &testDragOffer{id: 7, sourceID: 42, formats: []dragdrop.Format{dragdrop.FormatText}}
			if tc.foreign {
				offer.sourceID = 0
			}
			kinds := []events.EventType{events.DragEnter}
			if tc.drop {
				kinds = append(kinds, events.DragDrop)
			}
			kinds = append(kinds, events.DragLeave)
			for _, kind := range kinds {
				// No compatible target: a delivered local Drop still counts.
				if err := win.DispatchEvent(events.DragOfferEvent{EventType: kind, Offer: offer, Position: geometry.Point{X: 10, Y: 10}, Actions: dragdrop.Move}); err != nil {
					t.Fatal(err)
				}
			}
			run.end(DragResult{PositionValid: true, Position: geometry.Point{X: 12, Y: 34}})
			if result.LocalDrop != (tc.drop && !tc.foreign) || result.Action != 0 || result.Canceled || !result.PositionValid {
				t.Fatalf("incorrect end facts: %+v", result)
			}
		})
	}
}

func TestDragResultForwardsFinalLocalTargetWithoutInventingDrop(t *testing.T) {
	for _, local := range []bool{false, true} {
		app := &application{}
		source := NewDragSource()
		app.dragSession = &guiDragSession{app: app, id: 42, source: source, data: new(DragData)}
		calls := 0
		source.ConnectEnd(func(r DragResult) {
			calls++
			if r.LocalTarget != local || r.LocalDrop || r.Action != 0 || r.Canceled ||
				!r.PositionValid || r.Position != (geometry.Point{X: 12, Y: 34}) || r.Validate() != nil {
				t.Fatalf("incorrect terminal facts: %+v", r)
			}
		})
		app.dispatchDragSourceEvent(events.DragSourceEvent{EventType: events.DragSourceEnd, ID: 42,
			Result: dragdrop.Result{LocalTarget: local, PositionValid: true, Position: geometry.Point{X: 12, Y: 34}}})
		if calls != 1 || app.dragSession != nil {
			t.Fatalf("calls=%d session retained=%t", calls, app.dragSession != nil)
		}
	}
}

func TestDropTargetDispatchThroughWindowAndSemanticState(t *testing.T) {
	win := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{app: &application{}}}})
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 300, 200))
	child := newTestWidget()
	child.Arrange(geometry.Rect(20, 30, 120, 80))
	root.AddChild(child)
	win.SetWidget(root)
	target := NewDropTarget(DragFormatFiles, DragFormatText)
	child.AddEventController(target)
	var calls []string
	target.ConnectEnter(func(e *DragMotion) {
		if e.Position != (geometry.Point{X: 10, Y: 20}) || e.Format != DragFormatFiles {
			t.Errorf("enter: %+v", *e)
		}
		calls = append(calls, "enter")
	})
	target.ConnectMotion(func(*DragMotion) { calls = append(calls, "motion") })
	target.ConnectDrop(func(e *DropRequest) {
		files, ok := e.Data.Files()
		if !ok || !slices.Equal(files, []string{"/tmp/drop.txt"}) || e.Position != (geometry.Point{X: 10, Y: 20}) {
			t.Errorf("drop: %+v, files=%q", *e, files)
		}
		e.Accepted = true
		calls = append(calls, "drop")
	})
	target.ConnectLeave(func() { calls = append(calls, "leave") })
	offer := &testDragOffer{id: 1, formats: []dragdrop.Format{dragdrop.FormatText, dragdrop.FormatFiles}}
	point := geometry.Point{X: 30, Y: 50}
	reply := dragdrop.Action(0)
	for _, typ := range []events.EventType{events.DragEnter, events.DragMotion, events.DragDrop} {
		e := events.DragOfferEvent{EventType: typ, Offer: offer, Position: point,
			Actions: dragdrop.Copy, Suggested: dragdrop.Copy, ActionReply: &reply}
		if err := win.DispatchEvent(e); err != nil {
			t.Fatal(err)
		}
	}
	if reply != dragdrop.Copy || offer.read != dragdrop.FormatFiles || len(offer.finished) != 0 {
		t.Fatalf("negotiation reply=%d read=%s finished=%v", reply, offer.read, offer.finished)
	}
	if info := child.Snapshot().DragDrop; info == nil || !info.DropActive || !child.State().Has(style.DropActive) || info.TargetActions != DragCopy {
		t.Fatalf("active target snapshot: %+v", info)
	}
	data := new(dragdrop.Data)
	data.SetFiles([]string{"/tmp/drop.txt"})
	if err := win.DispatchEvent(events.DragDataEvent{OfferID: offer.id, Format: offer.read, Data: data}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(calls, []string{"enter", "motion", "motion", "drop", "leave"}) {
		t.Fatalf("callbacks: %q", calls)
	}
	if !slices.Equal(offer.finished, []dragdrop.Action{dragdrop.Copy}) || child.Snapshot().DragDrop.DropActive || child.State().Has(style.DropActive) {
		t.Fatalf("result=%v snapshot=%+v", offer.finished, child.Snapshot().DragDrop)
	}
}

func TestDropTargetRemovedBeforeReadFinishesRejects(t *testing.T) {
	win := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{app: &application{}}}})
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 200, 100))
	win.SetWidget(root)
	target := NewDropTarget(DragFormatText)
	root.AddEventController(target)
	drops := 0
	target.ConnectDrop(func(*DropRequest) { drops++ })
	offer := &testDragOffer{id: 2, formats: []dragdrop.Format{dragdrop.FormatText}}
	for _, typ := range []events.EventType{events.DragEnter, events.DragDrop} {
		if err := win.DispatchEvent(events.DragOfferEvent{EventType: typ, Offer: offer,
			Position: geometry.Point{X: 5, Y: 5}, Actions: dragdrop.Copy}); err != nil {
			t.Fatal(err)
		}
	}
	root.RemoveEventController(target)
	data := new(dragdrop.Data)
	data.SetText("late")
	if err := win.DispatchEvent(events.DragDataEvent{OfferID: offer.id, Format: dragdrop.FormatText, Data: data}); err != nil {
		t.Fatal(err)
	}
	if drops != 0 || !slices.Equal(offer.finished, []dragdrop.Action{0}) {
		t.Fatalf("late data delivered: drops=%d finish=%v", drops, offer.finished)
	}
}

func TestDropTargetHostCancellationFinishesPendingRead(t *testing.T) {
	win := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{app: &application{}}}})
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	win.SetWidget(root)
	target := NewDropTarget(DragFormatText)
	root.AddEventController(target)
	offer := &testDragOffer{id: 17, formats: []dragdrop.Format{dragdrop.FormatText}}
	for _, kind := range []events.EventType{events.DragEnter, events.DragDrop} {
		_ = win.DispatchEvent(events.DragOfferEvent{EventType: kind, Offer: offer,
			Position: geometry.Point{X: 10, Y: 10}, Actions: dragdrop.Copy})
	}
	if offer.read != dragdrop.FormatText {
		t.Fatal("drop did not start reading")
	}
	win.cancelInput(GestureHostClosed)
	if !slices.Equal(offer.finished, []dragdrop.Action{0}) || target.active || root.State().Has(style.DropActive) {
		t.Fatalf("pending offer was not rejected: finish=%v active=%v", offer.finished, target.active)
	}
	data := new(dragdrop.Data)
	data.SetText("late")
	_ = win.DispatchEvent(events.DragDataEvent{OfferID: offer.id, Format: offer.read, Data: data})
	if !slices.Equal(offer.finished, []dragdrop.Action{0}) {
		t.Fatalf("late data finished twice: %v", offer.finished)
	}
}

type gestureDragPlatform struct {
	platform.Platform
	native   *gestureDragNative
	failures int
	attempts int
}

func (p *gestureDragPlatform) NewDragDrop(platform.Surface) (platform.DragDrop, error) {
	p.attempts++
	if p.failures > 0 {
		p.failures--
		return nil, errors.New("temporary drag service failure")
	}
	return p.native, nil
}

func TestDragServiceRetriesAfterControllerConfigurationChanges(t *testing.T) {
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 40, 40))
	platformMock := &gestureDragPlatform{native: new(gestureDragNative), failures: 2}
	win := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{app: &application{platform: platformMock}, surface: &recordingPlatformPopup{}}}})
	win.SetWidget(root)
	defer win.Destroy()
	source := NewDragSource()
	root.AddEventController(source)
	if win.drag.native != nil || platformMock.attempts != 2 {
		t.Fatalf("first creation failures: attempts=%d", platformMock.attempts)
	}
	source.SetActions(DragMove)
	if win.drag.native == nil || platformMock.attempts != 3 {
		t.Fatalf("configuration did not retry: attempts=%d", platformMock.attempts)
	}
}

type gestureDragNative struct {
	feedback  dragdrop.Feedback
	begin     int
	cancels   int
	onBegin   func(uint64)
	onDestroy func()
	beginErr  error
}

func (*gestureDragNative) SetFormats([]dragdrop.Format) error { return nil }
func (n *gestureDragNative) Begin(id uint64, _ *dragdrop.Data, _ dragdrop.Action, _ dragdrop.Preview, feedback dragdrop.Feedback) error {
	n.feedback = feedback
	n.begin++
	if n.onBegin != nil {
		n.onBegin(id)
	}
	return n.beginErr
}
func (n *gestureDragNative) Cancel() { n.cancels++ }
func (n *gestureDragNative) Destroy() {
	if fn := n.onDestroy; fn != nil {
		n.onDestroy = nil
		fn()
	}
}

func TestDragSourceFeedbackIsPerSessionAndPreservesCancellation(t *testing.T) {
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	source := NewDragSource()
	root.AddEventController(source)
	source.ConnectPrepare(func(request *DragPrepare) {
		request.Data = new(DragData)
		request.Data.SetText("drag")
	})
	native := new(gestureDragNative)
	win := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{app: &application{platform: &gestureDragPlatform{native: native}}, surface: &recordingPlatformPopup{}}}})
	win.SetWidget(root)
	defer win.Destroy()
	var ended DragResult
	ends := 0
	source.ConnectEnd(func(result DragResult) { ended = result; ends++ })
	native.onBegin = func(id uint64) {
		source.SetFeedback(DragFeedback{}) // affects only future sessions
		_ = win.DispatchEvent(events.DragSourceEvent{EventType: events.DragSourceBegin, ID: id})
		_ = win.DispatchEvent(events.DragSourceEvent{EventType: events.DragSourceEnd, ID: id, Result: dragdrop.Result{Canceled: true}})
	}
	for i, feedback := range []DragFeedback{
		{NeutralOutsideTargets: true, DisableReturnAnimation: true},
		{NeutralOutsideTargets: true},
		{DisableReturnAnimation: true},
		{},
	} {
		source.SetFeedback(feedback)
		_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: geometry.Point{X: 20, Y: 20}})
		_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Buttons: events.PointerButtonLeftDown, Position: geometry.Point{X: 30, Y: 20}})
		want := dragdrop.Feedback{NeutralOutsideTargets: feedback.NeutralOutsideTargets, DisableReturnAnimation: feedback.DisableReturnAnimation}
		if native.begin != i+1 || native.feedback != want || ends != i+1 || !ended.Canceled || ended.Action != 0 || ended.Err != nil {
			t.Fatalf("begin=%d feedback=%+v want=%+v ends=%d result=%+v", native.begin, native.feedback, want, ends, ended)
		}
	}
}

func TestGestureDragSourceNilPrepareFallsBackToAncestor(t *testing.T) {
	root, child := newTestWidget(), newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	child.Arrange(geometry.Rect(10, 10, 40, 40))
	root.AddChild(child)
	outer, inner, click := NewDragSource(), NewDragSource(), NewClickEventController()
	root.AddEventController(outer)
	child.AddEventController(inner)
	child.AddEventController(click)
	platformNative := new(gestureDragNative)
	app := &application{platform: &gestureDragPlatform{native: platformNative}}
	win := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{app: app, surface: &recordingPlatformPopup{}}}})
	win.SetWidget(root)
	defer win.Destroy()
	prepared, clicked, began, ended := 0, 0, 0, 0
	inner.ConnectPrepare(func(*DragPrepare) { prepared++ })
	outer.ConnectPrepare(func(request *DragPrepare) {
		prepared++
		data := new(DragData)
		data.SetText("drag")
		request.Data = data
	})
	click.ConnectClicked(func(_ EventContext, _ ClickInfo) {
		clicked++
	})
	outer.ConnectBegin(func() { began++ })
	outer.ConnectEnd(func(DragResult) { ended++ })
	platformNative.onBegin = func(id uint64) {
		_ = win.DispatchEvent(events.DragSourceEvent{EventType: events.DragSourceBegin, ID: id})
		_ = win.DispatchEvent(events.DragSourceEvent{EventType: events.DragSourceEnd, ID: id})
	}
	platformNative.beginErr = errors.New("native returned an error after synchronous completion")
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: geometry.Point{X: 20, Y: 20}})
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Buttons: events.PointerButtonLeftDown, Position: geometry.Point{X: 30, Y: 20}})
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: geometry.Point{X: 30, Y: 20}})
	if prepared != 2 || platformNative.begin != 1 || began != 1 || ended != 1 || clicked != 0 || click.Pressed() {
		t.Fatalf("prepared=%d native=%d begin=%d end=%d click=%d pressed=%v", prepared, platformNative.begin, began, ended, clicked, click.Pressed())
	}
}

// --- 拖动观察与自动滚动 ---

func TestDragObserverTracksSubtreeIndependentlyOfAcceptance(t *testing.T) {
	root, a, b := newTestWidget(), newTestWidget(), newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 200, 100))
	a.Arrange(geometry.Rect(10, 10, 60, 50))
	b.Arrange(geometry.Rect(80, 10, 60, 50))
	root.AddChild(a)
	root.AddChild(b)
	parent, child := NewDragMotionEventController(), NewDragMotionEventController()
	root.AddEventController(parent)
	a.AddEventController(child)
	enters, motions, leaves, childLeaves := 0, 0, 0, 0
	parent.ConnectEnter(func(p geometry.Point) {
		enters++
		if p != (geometry.Point{X: 15, Y: 20}) {
			t.Fatal(p)
		}
	})
	parent.ConnectMotion(func(geometry.Point) { motions++ })
	parent.ConnectLeave(func() { leaves++ })
	child.ConnectLeave(func() { childLeaves++ })
	target := NewDropTarget(DragFormatText)
	a.AddEventController(target)
	win := testWindowWithSurface(&window{})
	win.SetWidget(root)
	defer win.Destroy()
	offer := &testDragOffer{id: 1, formats: []dragdrop.Format{dragdrop.FormatText}}
	reply := dragdrop.Action(0)
	for _, point := range []geometry.Point{{X: 15, Y: 20}, {X: 90, Y: 20}} {
		_ = win.DispatchEvent(events.DragOfferEvent{EventType: events.DragMotion, Position: point, Offer: offer, Actions: dragdrop.Copy, ActionReply: &reply})
	}
	if enters != 1 || motions != 1 || leaves != 0 || childLeaves != 1 || reply != 0 || win.dragMotion.event.ActionReply != nil {
		t.Fatal(enters, motions, leaves, childLeaves, reply)
	}
	_ = win.DispatchEvent(events.DragOfferEvent{EventType: events.DragLeave, Offer: offer})
	if leaves != 1 || parent.active || win.dragMotion.event.Offer != nil {
		t.Fatal("leave retained observation")
	}
}

func TestDragObserverReentrantDestroyStopsRemainingObservation(t *testing.T) {
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	first, second := NewDragMotionEventController(), NewDragMotionEventController()
	root.AddEventController(first)
	root.AddEventController(second)
	win := testWindowWithSurface(&window{})
	win.SetWidget(root)
	first.ConnectEnter(func(geometry.Point) { win.Destroy() })
	calls := 0
	first.ConnectEnter(func(geometry.Point) { calls++ })
	second.ConnectEnter(func(geometry.Point) { calls++ })
	_ = win.DispatchEvent(events.DragOfferEvent{EventType: events.DragEnter, Offer: &testDragOffer{id: 1}, Position: geometry.Point{X: 10, Y: 10}})
	if calls != 0 || win.dragMotion.event.Offer != nil || first.active {
		t.Fatal("destroyed host kept observing")
	}
}

func TestDragAutoScrollStationaryPointerRenegotiatesAndStops(t *testing.T) {
	sv := NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 100, Height: 1000}}
	sv.SetChild(content)
	var targets []*DropTarget
	for i := 0; i < 10; i++ {
		row := newTestWidget()
		row.Arrange(geometry.Rect(0, float32(i*100), 112, 100))
		content.WidgetBase.AddChild(content, row)
		target := NewDropTarget(DragFormatText)
		row.AddEventController(target)
		targets = append(targets, target)
	}
	win := testWindowWithSurface(&window{})
	win.SetWidget(sv)
	defer win.Destroy()
	sv.Measure(layout.Tight(geometry.Size{Width: 120, Height: 100}))
	sv.Arrange(geometry.Rect(0, 0, 120, 100))
	offer := &testDragOffer{id: 1, formats: []dragdrop.Format{dragdrop.FormatText}}
	reply := dragdrop.Action(0)
	_ = win.DispatchEvent(events.DragOfferEvent{EventType: events.DragEnter, Offer: offer, Position: geometry.Point{X: 50, Y: 95}, Actions: dragdrop.Copy, ActionReply: &reply})
	if win.dragTarget.target != targets[0] || reply != dragdrop.Copy {
		t.Fatal("initial target")
	}
	reply = dragdrop.Link // old synchronous native reply must not be touched again
	win.dragMotion.last = time.Unix(0, 0)
	win.advanceDragScroll(time.Unix(0, 0).Add(100 * time.Millisecond))
	if sv.ScrollY() < 40 || win.dragTarget.target != targets[1] || reply != dragdrop.Link {
		t.Fatal(sv.ScrollY(), win.dragTarget.target, reply)
	}
	sv.SetDragAutoScroll(false)
	before := sv.ScrollY()
	win.advanceDragScroll(time.Unix(0, 0).Add(200 * time.Millisecond))
	if sv.ScrollY() != before {
		t.Fatal("disabled scroll advanced")
	}
	_ = win.DispatchEvent(events.DragOfferEvent{EventType: events.DragDrop, Offer: offer, Position: geometry.Point{X: 50, Y: 95}, Actions: dragdrop.Copy})
	if win.dragMotion.event.Offer != nil || sv.dragMotion.active {
		t.Fatal("drop retained scrolling")
	}
}

func TestDragAutoScrollNestedAxesAndFractionalBoundary(t *testing.T) {
	outer, inner := NewScrollView(), NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 120, Height: 1000}}
	outer.SetChild(content)
	content.WidgetBase.AddChild(content, inner)
	inner.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 1000.5}})
	win := testWindowWithSurface(&window{})
	win.SetWidget(outer)
	defer win.Destroy()
	outer.Measure(layout.Tight(geometry.Size{Width: 160, Height: 100}))
	outer.Arrange(geometry.Rect(0, 0, 160, 100))
	inner.Measure(layout.Tight(geometry.Size{Width: 120, Height: 100}))
	inner.Arrange(geometry.Rect(0, 0, 120, 100))
	_ = win.DispatchEvent(events.DragOfferEvent{EventType: events.DragEnter, Offer: &testDragOffer{id: 1}, Position: geometry.Point{X: 50, Y: 95}})
	_, y, _, _ := win.dragScrollAxes()
	if y != inner {
		t.Fatal("inner must scroll first")
	}
	inner.SetScrollY(2000)
	_, y, _, _ = win.dragScrollAxes()
	if y != outer {
		t.Fatal("outer should take over at inner boundary", inner.ScrollY())
	}
	outer.SetDragAutoScroll(false)
	_, y, _, _ = win.dragScrollAxes()
	if y != nil {
		t.Fatal("fractional extent kept boundary scrolling")
	}
}

func TestDragScrollVelocity(t *testing.T) {
	for _, tc := range []struct{ p, w, want float32 }{{-1, 100, 0}, {0, 100, -480}, {16, 100, -240}, {50, 100, 0}, {84, 100, 240}, {100, 100, 0}, {0, 0, 0}} {
		if got := dragScrollVelocity(tc.p, tc.w); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}

func TestDragAutoScrollRespectsNestedDirections(t *testing.T) {
	outer, inner := NewScrollView(), NewScrollView()
	outer.SetDirection(ScrollVertical)
	inner.SetDirection(ScrollHorizontal)
	content := &mockWidget{size: geometry.Size{Width: 200, Height: 1000}}
	outer.SetChild(content)
	content.WidgetBase.AddChild(content, inner)
	inner.SetChild(&extentScrollContent{size: geometry.Size{Width: 500, Height: 500}})
	win := testWindowWithSurface(&window{})
	win.SetWidget(outer)
	defer win.Destroy()
	outer.Arrange(geometry.Rect(0, 0, 200, 100))
	inner.Arrange(geometry.Rect(0, 0, 120, 100))
	_ = win.DispatchEvent(events.DragOfferEvent{EventType: events.DragEnter, Offer: &testDragOffer{id: 1}, Position: geometry.Point{X: 115, Y: 85}})
	x, y, _, _ := win.dragScrollAxes()
	if x != inner || y != outer {
		t.Fatalf("nested axis ownership: x=%p y=%p, want inner=%p outer=%p", x, y, inner, outer)
	}
}
