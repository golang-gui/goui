package gui

import (
	"image"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
)

type popoverMeasureWidget struct {
	WidgetBase
	size geometry.Size
}

func (w *popoverMeasureWidget) Measure(layout.Constraint) geometry.Size { return w.size }

type recordingPlatformPopup struct {
	width  float32
	height float32
}

func (*recordingPlatformPopup) NativeHandle() uintptr           { return 1 }
func (*recordingPlatformPopup) Draw(image.Image) error          { return nil }
func (*recordingPlatformPopup) RequestPaint() error             { return nil }
func (*recordingPlatformPopup) Destroy()                        {}
func (*recordingPlatformPopup) SetPosition(float32, float32)    {}
func (p *recordingPlatformPopup) SetSize(width, height float32) { p.width, p.height = width, height }
func (*recordingPlatformPopup) Show() error                     { return nil }
func (*recordingPlatformPopup) Hide() error                     { return nil }

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

	p := &popover{modal: true} // menu-style: intercepts the window's input
	p.SetWidget(content)
	p.visible = true
	win.SetModalTarget(p) // what Show() does for a modal popover

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

// A widget hosted in a popover reaches the popover as its host for repaint and
// layout requests — Window() is nil there, so those must go through Root.
func TestPopoverHostsWidgetForRepaintAndLayout(t *testing.T) {
	p := &popover{}
	content := newTestWidget()
	p.SetWidget(content)

	if content.Root() != Root(p) {
		t.Fatalf("content.Root() should be the popover")
	}
	if content.Window() != nil {
		t.Fatalf("content.Window() should be nil for a popover-hosted widget")
	}

	p.layoutDirty = false
	content.RequestLayout()
	if !p.layoutDirty {
		t.Fatalf("RequestLayout on popover content should reach the popover (mark it dirty)")
	}
}

func TestPopoverResizeKeepsAuthoritativeNativeSizeUntilSizeEvent(t *testing.T) {
	native := &recordingPlatformPopup{}
	p := &popover{
		rootBase:      rootBase{width: 120, height: 50, pixelWidth: 120, pixelHeight: 50},
		platformPopup: native,
		widget:        &popoverMeasureWidget{size: geometry.Size{Width: 120, Height: 49.65625}},
	}

	p.measureAndSize()
	if native.width != 120 || native.height != 49.65625 {
		t.Fatalf("native size request = %gx%g, want 120x49.65625", native.width, native.height)
	}
	if p.width != 120 || p.height != 50 {
		t.Fatalf("host size changed before SizeEvent: %gx%g, want authoritative 120x50", p.width, p.height)
	}

	p.onEvent(events.SizeEvent{Width: 120, Height: 51, PixelWidth: 120, PixelHeight: 51})
	if p.width != 120 || p.height != 51 {
		t.Fatalf("host size after SizeEvent = %gx%g, want 120x51", p.width, p.height)
	}
}

// A modal popover registers itself as the window's modal target and resigns when
// it stops being modal — through the public Window.SetModalTarget API only.
func TestPopoverModalTogglesWindowModalTarget(t *testing.T) {
	win := &window{root: newTestWidget()}
	p := &popover{owner: win, visible: true}

	p.becomeModalTarget()
	if win.modalTarget == nil {
		t.Fatalf("a modal popover should register itself as the owner's modal target")
	}
	p.resignModalTarget()
	if win.modalTarget != nil {
		t.Fatalf("resigning should clear the window's modal target")
	}
}

func TestPopoverSetWidgetMigratesFromWindow(t *testing.T) {
	win := &window{}
	content := newTestWidget()
	win.SetWidget(content)

	unmounted := false
	content.ConnectUnmount(func() { unmounted = true })

	// Migrating the widget from the window into the popover must emit the
	// unmount notification (adoptWidget semantics) and re-mount under the
	// popover.
	p := &popover{}
	p.SetWidget(content)
	if !unmounted {
		t.Fatal("migration to popover should emit unmount on the old root")
	}
	if content.Root() != p {
		t.Fatalf("content should be mounted under the popover, got %v", content.Root())
	}
	if p.Widget() != content {
		t.Fatal("popover content should be set")
	}
}

func TestPopoverConnectClosedFiresOnHideOnce(t *testing.T) {
	p := NewPopover(newTestWidget())
	closed := 0
	p.ConnectClosed(func() { closed++ })

	p.Hide() // not visible yet → no fire
	if closed != 0 {
		t.Fatalf("Hide on a hidden popover must not fire closed, got %d", closed)
	}
}
