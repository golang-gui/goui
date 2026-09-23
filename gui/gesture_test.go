package gui

import (
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
)

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
		click.ConnectClicked(func(EventContext) { clicks++ })
		parentDrag.ConnectBegin(func(geometry.Point, events.Modifiers) { begins++ })
		parentDrag.ConnectEnd(func(geometry.Point, events.Modifiers) { ends++ })
		win := &window{root: root}
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
	click.ConnectClick(func(_ EventContext, count int) { counts = append(counts, count) })
	win := &window{root: root}
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
	click.ConnectClick(func(EventContext, int) { results++; root.SetVisible(false) })
	click.ConnectClicked(func(EventContext) { compatibility++ })
	win := &window{root: root}
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
	click.ConnectClicked(func(EventContext) { clicked++ })
	win := &window{root: root}
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
	win := &window{}
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
	parent.ConnectBegin(func(geometry.Point, events.Modifiers) { counts[0]++ })
	first.ConnectBegin(func(geometry.Point, events.Modifiers) { counts[1]++ })
	second.ConnectBegin(func(geometry.Point, events.Modifiers) { counts[2]++ })
	win := &window{root: root}
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
	win := &window{root: root}
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: geometry.Point{X: 20, Y: 20}})
	if parent.accepted != 0 || parent.canceled != 1 || winner.accepted != 0 || winner.canceled != 1 {
		t.Fatalf("parent=%+v winner=%+v", parent, winner)
	}
}
