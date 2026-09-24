package gui

import (
	"errors"
	"slices"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/events"
)

type testDragOffer struct {
	id       uint64
	formats  []dragdrop.Format
	read     dragdrop.Format
	finished []dragdrop.Action
}

func (o *testDragOffer) ID() uint64                   { return o.id }
func (o *testDragOffer) SourceID() uint64             { return 0 }
func (o *testDragOffer) Formats() []dragdrop.Format   { return slices.Clone(o.formats) }
func (o *testDragOffer) Read(f dragdrop.Format) error { o.read = f; return nil }
func (o *testDragOffer) Finish(a dragdrop.Action) error {
	o.finished = append(o.finished, a)
	return nil
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
	begin    int
	onBegin  func(uint64)
	beginErr error
}

func (*gestureDragNative) SetFormats([]dragdrop.Format) error { return nil }
func (n *gestureDragNative) Begin(id uint64, _ *dragdrop.Data, _ dragdrop.Action, _ dragdrop.Preview) error {
	n.begin++
	if n.onBegin != nil {
		n.onBegin(id)
	}
	return n.beginErr
}
func (*gestureDragNative) Cancel()  {}
func (*gestureDragNative) Destroy() {}

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
