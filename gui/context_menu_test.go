package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
)

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
			c.ConnectRequest(func(EventContext) { calls++ })
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
	c.ConnectRequest(func(EventContext) { calls++ })
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
	local.ConnectRequest(func(ctx EventContext) {
		localCalls++
		if p, ok := ctx.Position(); ok && p != (geometry.Point{X: 5, Y: 7}) {
			t.Fatal(p)
		}
		if consume {
			ctx.StopPropagation()
		}
	})
	parent.ConnectRequest(func(EventContext) { parentCalls++ })
	w := &window{}
	w.SetWidget(root)
	e := events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonRight, Position: geometry.Point{X: 15, Y: 27}}
	_ = w.DispatchEvent(e)
	consume = true
	_ = w.DispatchEvent(e)
	if localCalls != 2 || parentCalls != 1 {
		t.Fatal(localCalls, parentCalls)
	}
	w.SetFocusedWidget(child)
	local.ConnectRequest(func(ctx EventContext) {
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
