package gui

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/events"
)

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
	win := &window{rootBase: rootBase{app: app}}
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
	for _, stage := range []string{"drop", "leave", "finish"} {
		for _, accepted := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/accepted=%t", stage, accepted), func(t *testing.T) {
				app := &application{}
				win := &window{rootBase: rootBase{app: app}}
				root := newTestWidget()
				root.Arrange(geometry.Rect(0, 0, 100, 100))
				win.SetWidget(root)
				source := NewDragSource()
				run := &guiDragSession{app: app, id: 42, source: source, data: new(DragData)}
				app.dragSession = run
				var results []DragResult
				source.ConnectEnd(func(result DragResult) { results = append(results, result) })
				end := func() {
					app.dispatchDragSourceEvent(events.DragSourceEvent{EventType: events.DragSourceEnd, ID: run.id, Result: dragdrop.Result{Canceled: true}})
					if len(results) != 0 {
						t.Fatal("End was emitted before synchronous Drop/Finish returned")
					}
				}
				target := NewDropTarget(DragFormatText)
				root.AddEventController(target)
				target.ConnectDrop(func(e *DropRequest) {
					if stage == "drop" {
						end()
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
				want := DragAction(0)
				if accepted {
					want = DragCopy
				}
				run.end(DragResult{Canceled: true}) // duplicate terminal callbacks are ignored
				if len(results) != 1 || !results[0].LocalDrop || results[0].Action != want || results[0].Canceled == accepted || results[0].Validate() != nil || app.dragSession != nil || run.data != nil || !slices.Equal(offer.finished, []dragdrop.Action{want}) {
					t.Fatalf("results=%+v finish=%v session=%p data=%p", results, offer.finished, app.dragSession, run.data)
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
			win := &window{rootBase: rootBase{app: app}}
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
	win := &window{rootBase: rootBase{app: &application{}}}
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
	if info := child.Snapshot().DragDrop; info == nil || !info.DropActive || info.TargetActions != DragCopy {
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
	if !slices.Equal(offer.finished, []dragdrop.Action{dragdrop.Copy}) || child.Snapshot().DragDrop.DropActive {
		t.Fatalf("result=%v snapshot=%+v", offer.finished, child.Snapshot().DragDrop)
	}
}

func TestDropTargetRemovedBeforeReadFinishesRejects(t *testing.T) {
	win := &window{rootBase: rootBase{app: &application{}}}
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
	win := &window{rootBase: rootBase{app: &application{}}}
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
	if !slices.Equal(offer.finished, []dragdrop.Action{0}) || target.active {
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
	win := &window{rootBase: rootBase{app: &application{platform: platformMock}, surface: &recordingPlatformPopup{}}}
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
	feedback dragdrop.Feedback
	begin    int
	cancels  int
	onBegin  func(uint64)
	beginErr error
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
func (*gestureDragNative) Destroy()  {}

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
	win := &window{rootBase: rootBase{app: &application{platform: &gestureDragPlatform{native: native}}, surface: &recordingPlatformPopup{}}}
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
	win := &window{rootBase: rootBase{app: app, surface: &recordingPlatformPopup{}}}
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
	click.ConnectClicked(func(EventContext) { clicked++ })
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
