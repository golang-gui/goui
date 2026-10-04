package gui

import (
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/events"
)

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
	win := &window{}
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
	win := &window{}
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
	win := &window{}
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
	win := &window{}
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
