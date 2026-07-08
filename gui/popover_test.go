package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
)

// absOrigin sums each widget's parent-relative rect origin up the parent chain.
func TestPopoverAbsOrigin(t *testing.T) {
	root := newTestWidget()
	parent := newTestWidget()
	anchor := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 200, 200))
	parent.Arrange(geometry.Rect(10, 20, 100, 100))
	anchor.Arrange(geometry.Rect(5, 7, 40, 30))
	root.AddChild(parent)
	parent.AddChild(anchor)

	got := absOrigin(anchor)
	if got.X != 15 || got.Y != 27 { // 5+10+0, 7+20+0
		t.Fatalf("absOrigin = %+v, want (15, 27)", got)
	}
}

// The window forwards its own input to an open popover (§7): keyboard is
// forwarded, an outside click / Esc / focus loss requests dismissal, and the
// window's own widget tree is not reached while a popover is open.
func TestWindowForwardsToPopover(t *testing.T) {
	winRoot := newTestWidget()
	winRoot.Arrange(geometry.Rect(0, 0, 100, 100))
	var winCalls []string
	winRoot.AddEventController(newRecordingController("win", PhaseTarget, &winCalls, nil))
	win := &window{root: winRoot}

	content := newTestWidget()
	content.Arrange(geometry.Rect(0, 0, 60, 40))
	var popCalls []string
	content.AddEventController(newRecordingController("pop", PhaseTarget, &popCalls, nil))

	p := &popover{modal: true} // menu-style: occupies the window's input slot
	p.SetWidget(content)
	p.visible = true
	win.popover = p

	dismisses := 0
	p.dismissRequest.Connect(func() { dismisses++ })

	// Outside click (the owner only ever receives clicks outside the popover).
	win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 5, Y: 5}})
	if dismisses != 1 {
		t.Fatalf("PointerDown: dismisses=%d, want 1", dismisses)
	}
	if len(winCalls) != 0 {
		t.Fatalf("PointerDown should be swallowed, winCalls=%v", winCalls)
	}

	// Esc requests dismissal.
	win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyEscape})
	if dismisses != 2 {
		t.Fatalf("Esc: dismisses=%d, want 2", dismisses)
	}

	// A non-Esc key is forwarded to the popover's content, not the window's tree.
	win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyEnter})
	if dismisses != 2 {
		t.Fatalf("non-Esc key must not dismiss, dismisses=%d", dismisses)
	}
	if len(popCalls) == 0 {
		t.Fatalf("non-Esc key should reach popover content, popCalls empty")
	}
	if len(winCalls) != 0 {
		t.Fatalf("non-Esc key should not reach window tree, winCalls=%v", winCalls)
	}

	// Focus loss requests dismissal.
	win.DispatchEvent(events.FocusEvent{Focused: false})
	if dismisses != 3 {
		t.Fatalf("FocusEvent{false}: dismisses=%d, want 3", dismisses)
	}
}

// setPopover superseding a previous popover requests the old one's dismissal.
func TestWindowSetPopoverSupersedes(t *testing.T) {
	win := &window{root: newTestWidget()}

	old := &popover{visible: true}
	oldDismissed := 0
	old.dismissRequest.Connect(func() { oldDismissed++ })
	win.setPopover(old)

	newP := &popover{visible: true}
	win.setPopover(newP)

	if oldDismissed != 1 {
		t.Fatalf("old popover should get a dismiss request when superseded, got %d", oldDismissed)
	}
	if win.popover != newP {
		t.Fatalf("window should track the new popover")
	}
}
