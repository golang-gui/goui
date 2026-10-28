package gui

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

type popoverMeasureWidget struct {
	WidgetBase
	size      geometry.Size
	onMeasure func()
}

func TestPopoverPlacementConfiguration(t *testing.T) {
	root, anchor := newTestWidget(), newTestWidget()
	root.AddChild(anchor)
	win := &window{}
	win.SetWidget(root)
	anchor.Arrange(geometry.Rect(30, 40, 160, 32))
	p := NewPopover(anchor, nil).(*popover)
	if p.Placement() != PopoverPlacementPoint {
		t.Fatal("default placement is not Point")
	}
	pos := geometry.Point{X: 5, Y: 6}
	p.SetPosition(pos)
	p.SetPlacement(PopoverPlacementBottom)
	snapshot, err := p.queryPlacement()
	if err != nil || snapshot.placement != PopoverPlacementBottom || snapshot.anchor != geometry.Rect(30, 40, 160, 32) || p.Position() != pos {
		t.Fatalf("anchor placement: %v %v", snapshot, err)
	}
	p.SetPosition(geometry.Point{X: 7, Y: 8})
	if p.Placement() != PopoverPlacementBottom {
		t.Fatal("SetPosition changed placement")
	}
	p.SetPosition(pos)
	p.SetPlacement(PopoverPlacementPoint)
	snapshot, err = p.queryPlacement()
	if err != nil || snapshot.placement != PopoverPlacementPoint || snapshot.anchor != geometry.Rect(35, 46, 0, 0) {
		t.Fatalf("point placement: %v %v", snapshot, err)
	}
	p.SetPlacement(PopoverPlacementBottom)
	p.SetPlacement(PopoverPlacement(255))
	if p.Placement() != PopoverPlacementPoint || p.Position() != pos || p.platformPopup != nil {
		t.Fatal("invalid enum normalization changed position or created a popup")
	}
	p.Destroy()
	win.Destroy()
}

func TestWindowPopoversFollowVisibleLifetime(t *testing.T) {
	p, native, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
	w := app.windows[0]
	if len(w.Popovers()) != 0 {
		t.Fatal("unshown popover was enumerated")
	}
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if got := w.Popovers(); len(got) != 1 || got[0] != p {
		t.Fatalf("shown popovers=%v", got)
	} else {
		got[0] = nil
		if w.Popovers()[0] != p {
			t.Fatal("Popovers exposed the internal slice")
		}
	}
	if err := p.Show(); err != nil || len(w.Popovers()) != 1 {
		t.Fatal("repeated Show registered the same object twice")
	}
	p.Hide()
	if len(w.Popovers()) != 0 {
		t.Fatal("hidden popover retained by window")
	}
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	other := &window{platformWindow: native}
	other.SetWidget(p.Anchor()) // unmount releases the old native surface
	if len(w.Popovers()) != 0 {
		t.Fatal("anchor migration left an old window entry")
	}
	if err := p.Show(); err != nil || len(other.Popovers()) != 1 {
		t.Fatalf("new owner did not register popover: %v", err)
	}
	p.Destroy()
	if len(other.Popovers()) != 0 {
		t.Fatal("destroyed popover retained by window")
	}
}

func TestWindowPopoversRemoveBeforeClosedCallbacks(t *testing.T) {
	p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
	w := app.windows[0]
	p.ConnectClosed(func() {
		if len(w.Popovers()) != 0 {
			t.Fatal("closed callback observed stale visible popovers")
		}
	})
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	p.Anchor().base().detachRoot(p.Anchor())
	if len(w.Popovers()) != 0 {
		t.Fatal("anchor unmount retained popup")
	}
}

type failedShowPopup struct{ platform.Popup }

func (*failedShowPopup) Show() error { return errors.New("native Show failed") }

type failedShowPlatform struct{ *placementPlatform }

func (p *failedShowPlatform) NewPopup(owner platform.Window, size geometry.Size, handler platform.EventHandler, options platform.PopupOptions) (platform.Popup, error) {
	native, err := p.placementPlatform.NewPopup(owner, size, handler, options)
	if err != nil {
		return nil, err
	}
	return &failedShowPopup{native}, nil
}

func TestWindowPopoversExcludeFailedShowAndOwnerDestruction(t *testing.T) {
	p, _, plat, app := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
	w := app.windows[0]
	app.platform = &failedShowPlatform{plat}
	if err := p.Show(); err == nil || len(w.Popovers()) != 0 || p.Visible() {
		t.Fatal("failed native Show registered a visible popup")
	}
	p.releaseNative()
	app.platform = plat
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	w.Destroy()
	if len(w.Popovers()) != 0 || p.Visible() {
		t.Fatal("owner destruction retained a shown popup")
	}
}

func TestPopoverTabFocusAndModalIsolation(t *testing.T) {
	ownerRoot := newTestWidget()
	ownerRoot.SetFocusable(true)
	win := &window{}
	win.SetWidget(ownerRoot)
	win.SetFocusedWidget(ownerRoot)
	root, first, second := newTestWidget(), newTestWidget(), newTestWidget()
	first.SetFocusable(true)
	second.SetFocusable(true)
	root.AddChild(first)
	root.AddChild(second)
	p := &popover{modal: true, autoDismiss: true, visible: true}
	p.SetWidget(root)
	win.SetModalTarget(p)
	var changes []bool
	first.ConnectFocused(func(focused bool) { changes = append(changes, focused) })
	for _, want := range []Widget{first, second, first} {
		handled := false
		e := shortcutPress(events.KeyTab, 0)
		e.Handled = &handled
		_ = win.DispatchEvent(e)
		if !handled || p.FocusedWidget() != want || !want.Focused() || !root.ContainsFocus() || win.FocusedWidget() != ownerRoot {
			t.Fatalf("modal Tab: focus=%p want=%p handled=%v", p.FocusedWidget(), want, handled)
		}
	}
	root.RemoveChild(first)
	if p.FocusedWidget() != nil || first.Focused() || root.ContainsFocus() {
		t.Fatal("popover detach retained focus")
	}
	if len(changes) != 4 || !changes[0] || changes[1] || !changes[2] || changes[3] {
		t.Fatalf("focus signals=%v, want [true false true false]", changes)
	}
	win.SetModalTarget(nil)
	p.SetFocusedWidget(second)
	p.Hide()
	if p.FocusedWidget() != nil || second.Focused() || root.ContainsFocus() {
		t.Fatal("hiding popover retained logical focus")
	}
}

func TestPopoverFocusVisibleIsolationAndReset(t *testing.T) {
	ownerRoot, popupRoot := newTestWidget(), newTestWidget()
	for _, widget := range []*testWidget{ownerRoot, popupRoot} {
		widget.SetFocusable(true)
		widget.Arrange(geometry.Rect(0, 0, 100, 100))
	}
	win := &window{}
	win.SetWidget(ownerRoot)
	p := &popover{rootBase: rootBase{width: 100, height: 100}, modal: true, autoDismiss: true, visible: true}
	p.SetWidget(popupRoot)
	t.Cleanup(func() { p.SetWidget(nil); win.SetWidget(nil) })
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
	win.SetModalTarget(p)
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
	if !ownerRoot.FocusVisible() || !popupRoot.FocusVisible() {
		t.Fatal("modal navigation changed the owner's input mode")
	}
	_ = p.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 5, Y: 5}, Button: events.PointerButtonLeft})
	if !ownerRoot.FocusVisible() || popupRoot.FocusVisible() || !popupRoot.Focused() {
		t.Fatal("popup pointer mode leaked to owner or changed logical focus")
	}
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
	// Even a click in the transparent popup corner clears its own hint before
	// requesting dismissal; applications need not hide it synchronously.
	_ = p.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: -1, Y: -1}})
	if popupRoot.FocusVisible() || !ownerRoot.FocusVisible() {
		t.Fatal("popup outside press did not independently clear hints")
	}
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
	p.ConnectDismissRequest(func() { p.Hide(); win.SetModalTarget(nil) })
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown})
	if ownerRoot.FocusVisible() || !ownerRoot.Focused() || popupRoot.FocusVisible() || p.FocusedWidget() != nil {
		t.Fatal("owner outside press or popup Hide retained keyboard hints")
	}
	// Reopening and programmatically focusing starts without the old popup mode.
	p.visible = true
	p.SetFocusedWidget(popupRoot)
	if popupRoot.FocusVisible() {
		t.Fatal("reopened popup inherited the hidden session's keyboard mode")
	}
}

func (w *popoverMeasureWidget) Measure(layout.Constraint) layout.Measurement {
	if w.onMeasure != nil {
		w.onMeasure()
	}
	return layout.Measured(w.size)
}

type recordingPlatformPopup struct {
	width  float32
	height float32
}

func (*recordingPlatformPopup) NativeHandle() uintptr           { return 1 }
func (*recordingPlatformPopup) Transparent() bool               { return false }
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

	p := &popover{modal: true, autoDismiss: true} // 明确启用菜单式关闭，Modal 只指定输入独占。
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
	p := NewPopover(newTestWidget(), nil)
	closed := 0
	p.ConnectClosed(func() { closed++ })

	p.Hide() // not visible yet → no fire
	if closed != 0 {
		t.Fatalf("Hide on a hidden popover must not fire closed, got %d", closed)
	}
}

func TestPopoverPlacementGeometry(t *testing.T) {
	for _, tc := range []struct {
		name      string
		anchor    geometry.Rectangle
		size      geometry.Size
		placement PopoverPlacement
		insets    popoverInsets
		want      geometry.Point
	}{
		{"point", geometry.Rect(10, 20, 0, 0), geometry.Size{Width: 30, Height: 20}, PopoverPlacementPoint, popoverInsets{}, geometry.Point{X: 10, Y: 20}},
		{"point slide", geometry.Rect(90, 90, 0, 0), geometry.Size{Width: 30, Height: 20}, PopoverPlacementPoint, popoverInsets{}, geometry.Point{X: 70, Y: 80}},
		{"point top left", geometry.Rect(-30, -20, 0, 0), geometry.Size{Width: 30, Height: 20}, PopoverPlacementPoint, popoverInsets{}, geometry.Point{}},
		{"below left", geometry.Rect(10, 10, 20, 10), geometry.Size{Width: 30, Height: 20}, PopoverPlacementBottom, popoverInsets{}, geometry.Point{X: 10, Y: 20}},
		{"below right", geometry.Rect(80, 10, 20, 10), geometry.Size{Width: 30, Height: 20}, PopoverPlacementBottom, popoverInsets{}, geometry.Point{X: 70, Y: 20}},
		{"above left", geometry.Rect(10, 85, 20, 10), geometry.Size{Width: 30, Height: 20}, PopoverPlacementBottom, popoverInsets{}, geometry.Point{X: 10, Y: 65}},
		{"above right", geometry.Rect(80, 85, 20, 10), geometry.Size{Width: 30, Height: 20}, PopoverPlacementBottom, popoverInsets{}, geometry.Point{X: 70, Y: 65}},
		{"slide before shrink", geometry.Rect(40, 40, 20, 20), geometry.Size{Width: 90, Height: 90}, PopoverPlacementBottom, popoverInsets{}, geometry.Point{X: 10, Y: 10}},
		{"shadow body alignment", geometry.Rect(10, 10, 20, 10), geometry.Size{Width: 38, Height: 30}, PopoverPlacementBottom, popoverInsets{3, 4, 5, 6}, geometry.Point{X: 7, Y: 16}},
		{"shadow containment flips", geometry.Rect(10, 70, 20, 10), geometry.Size{Width: 38, Height: 30}, PopoverPlacementBottom, popoverInsets{3, 4, 5, 6}, geometry.Point{X: 7, Y: 46}},
		{"actual rounded oversized", geometry.Rect(90, 90, 0, 0), geometry.Size{Width: 100.5, Height: 100.5}, PopoverPlacementPoint, popoverInsets{}, geometry.Point{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := popoverPlacementSnapshot{anchor: tc.anchor, placement: tc.placement, workArea: geometry.Rect(0, 0, 100, 100), constrained: true}
			if got := s.position(tc.size, tc.insets); got != tc.want {
				t.Fatalf("%v, want %v", got, tc.want)
			}
		})
	}
	s := popoverPlacementSnapshot{workArea: geometry.Rect(-100, -50, 300, 200)}
	s.constrained = true
	s.anchor = geometry.Rect(-120, -70, 0, 0)
	if got := s.position(geometry.Size{Width: 50, Height: 40}, popoverInsets{}); got != (geometry.Point{X: -100, Y: -50}) {
		t.Fatal("negative workarea lost", got)
	}
	if limit, err := s.bodyLimit(popoverInsets{10, 20, 30, 40}); err != nil || limit != (geometry.Size{Width: 260, Height: 140}) {
		t.Fatal(limit, err)
	}
	if _, err := s.bodyLimit(popoverInsets{200, 0, 100, 0}); err == nil {
		t.Fatal("accepted no body space")
	}
}

type placementWindow struct {
	desktopTestWindow
	area    geometry.Rectangle
	err     error
	points  []geometry.Point
	onQuery func()
}

func (w *placementWindow) WorkAreaAt(p geometry.Point) (geometry.Rectangle, error) {
	w.points = append(w.points, p)
	if w.onQuery != nil {
		w.onQuery()
	}
	return w.area, w.err
}

type placementNative struct {
	recordingPlatformPopup
	handler   platform.EventHandler
	positions []geometry.Point
	sizes     []geometry.Size
	syncSize  bool
	onSize    func()
	destroyed bool
	shows     int
	paints    int
}

func (p *placementNative) Show() error         { p.shows++; return nil }
func (p *placementNative) RequestPaint() error { p.paints++; return nil }
func (p *placementNative) SetPosition(x, y float32) {
	p.positions = append(p.positions, geometry.Point{X: x, Y: y})
}
func (p *placementNative) SetSize(w, h float32) {
	p.sizes = append(p.sizes, geometry.Size{Width: w, Height: h})
	if p.onSize != nil {
		p.onSize()
	}
	if p.syncSize {
		p.handler(events.SizeEvent{Width: w, Height: h, PixelWidth: w, PixelHeight: h})
	}
}
func (p *placementNative) Destroy() { p.destroyed = true }

type placementPlatform struct {
	platform.Platform
	natives        []*placementNative
	created        []geometry.Size
	syncSize       bool
	transparentErr error
	options        []platform.PopupOptions
}

func (p *placementPlatform) NewPopup(_ platform.Window, size geometry.Size, handler platform.EventHandler, options platform.PopupOptions) (platform.Popup, error) {
	p.options = append(p.options, options)
	if options.Transparent && p.transparentErr != nil {
		return nil, p.transparentErr
	}
	n := &placementNative{handler: handler, syncSize: p.syncSize}
	p.natives, p.created = append(p.natives, n), append(p.created, size)
	if p.syncSize {
		handler(events.SizeEvent{Width: size.Width, Height: size.Height, PixelWidth: size.Width, PixelHeight: size.Height})
	}
	return n, nil
}
func (*placementPlatform) NewPainter(platform.Surface) (graphics.Painter, error) {
	return &recordingPainterBackend{}, nil
}

func placementFixture(t *testing.T, area geometry.Rectangle) (*popover, *placementWindow, *placementPlatform, *application) {
	t.Helper()
	native := &placementWindow{area: area}
	plat := &placementPlatform{syncSize: true}
	app := &application{platform: plat}
	useTestApplication(t, app)
	w := &window{platformWindow: native}
	app.windows = []*window{w}
	anchor := NewButton()
	w.SetWidget(anchor)
	anchor.Arrange(geometry.Rect(80, 80, 20, 10))
	p := NewPopover(anchor, nil).(*popover)
	p.SetWidget(&popoverMeasureWidget{size: geometry.Size{Width: 30, Height: 20}})
	t.Cleanup(p.Destroy)
	return p, native, plat, app
}

func settlePlacement(t *testing.T, p *popover) {
	t.Helper()
	for range 8 {
		p.paint()
		if !p.layoutDirty {
			return
		}
	}
	t.Fatal("popup layout did not settle")
}

func TestPopoverPlacementVisibleUpdates(t *testing.T) {
	p, _, plat, _ := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
	p.SetModal(true)
	p.SetAutoDismiss(true)
	p.SetPosition(geometry.Point{X: 5, Y: 6})
	p.widget.SetFocusable(true)
	closed := 0
	p.ConnectClosed(func() { closed++ })
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	settlePlacement(t, p)
	p.SetFocusedWidget(p.widget)
	n := plat.natives[0]
	positions, queries := len(n.positions), len(plat.created)
	p.SetPlacement(PopoverPlacementBottom)
	if len(n.positions) != positions || len(plat.created) != queries {
		t.Fatal("setter synchronously moved or recreated popup")
	}
	settlePlacement(t, p)
	if p.requestedPosition != (geometry.Point{X: 80, Y: 90}) {
		t.Fatal("Bottom did not use the complete anchor", p.requestedPosition)
	}
	paints := n.paints
	p.SetPosition(geometry.Point{X: 21, Y: 22})
	if p.layoutDirty || n.paints != paints || p.Placement() != PopoverPlacementBottom {
		t.Fatal("ignored Position scheduled layout or changed placement")
	}
	p.SetPlacement(PopoverPlacementPoint)
	p.SetPosition(geometry.Point{X: 23, Y: 24})
	positions = len(n.positions)
	settlePlacement(t, p)
	if len(n.positions) != positions+1 || p.requestedPosition != (geometry.Point{X: 103, Y: 104}) {
		t.Fatal("batched Point configuration did not position once", n.positions)
	}
	if len(plat.created) != 1 || n.shows != 1 || closed != 0 || !p.Visible() || !p.Modal() || p.FocusedWidget() != p.widget || p.owner.(*window).modalTarget != p {
		t.Fatal("placement update changed surface, visibility, modal state or focus")
	}
	paints = n.paints
	p.SetPlacement(p.Placement())
	p.SetPosition(p.Position())
	if p.layoutDirty || n.paints != paints {
		t.Fatal("equal setters requested layout")
	}
}

func TestPopoverPlacementSurvivesFailureAndHide(t *testing.T) {
	p, w, plat, _ := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
	pos := geometry.Point{X: 5, Y: 6}
	p.SetPosition(pos)
	p.SetPlacement(PopoverPlacementBottom)
	w.err = platform.ErrUnavailable
	if err := p.Show(); !errors.Is(err, platform.ErrUnavailable) || len(plat.created) != 0 {
		t.Fatal("query failure created a surface", err)
	}
	if p.Placement() != PopoverPlacementBottom || p.Position() != pos {
		t.Fatal("failed Show changed configuration")
	}
	w.err = nil
	for range 2 {
		if err := p.Show(); err != nil {
			t.Fatal(err)
		}
		settlePlacement(t, p)
		if p.Placement() != PopoverPlacementBottom || p.Position() != pos || p.requestedPosition != (geometry.Point{X: 80, Y: 90}) {
			t.Fatal("Show changed configuration or placement")
		}
		p.Hide()
	}
	if len(plat.created) != 1 {
		t.Fatal("Hide/Show unnecessarily recreated popup")
	}
}

func TestPopoverPlacementChangedDuringMeasure(t *testing.T) {
	for _, visible := range []bool{false, true} {
		for _, field := range []string{"placement", "position"} {
			t.Run(fmt.Sprintf("visible=%t/%s", visible, field), func(t *testing.T) {
				p, _, plat, _ := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
				if visible {
					if err := p.Show(); err != nil {
						t.Fatal(err)
					}
					settlePlacement(t, p)
				}
				content := p.widget.(*popoverMeasureWidget)
				content.onMeasure = func() {
					content.onMeasure = nil
					if field == "placement" {
						p.SetPlacement(PopoverPlacementBottom)
					} else {
						p.SetPosition(geometry.Point{X: 17, Y: 18})
					}
				}
				if visible {
					n := plat.natives[0]
					positions := len(n.positions)
					p.RequestLayout()
					p.paint()
					if len(n.positions) != positions || !p.layoutDirty {
						t.Fatal("stale solve moved surface or consumed new request")
					}
					settlePlacement(t, p)
				} else {
					if err := p.Show(); err == nil || len(plat.created) != 0 {
						t.Fatal("stale solve created surface", err)
					}
					if err := p.Show(); err != nil {
						t.Fatal(err)
					}
					settlePlacement(t, p)
				}
				want := geometry.Point{X: 97, Y: 98}
				if field == "placement" {
					want = geometry.Point{X: 80, Y: 90}
				}
				if p.requestedPosition != want {
					t.Fatal(p.requestedPosition, want)
				}
			})
		}
	}
}

func TestMenuPlacementFallbackAndModeSwitch(t *testing.T) {
	for _, placement := range []PopoverPlacement{PopoverPlacementPoint, PopoverPlacementBottom} {
		t.Run(fmt.Sprint(placement), func(t *testing.T) {
			p, _, plat, app := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
			app.typo = &styleTypography{}
			app.style = textStyleSheet(10, color.Black)
			plat.transparentErr = platform.ErrUnsupported
			pm := NewPopoverMenu(p.anchor)
			m := NewMenu()
			m.Append("Text", nil)
			pm.SetMenu(m)
			t.Cleanup(func() {
				if pm.popover != nil {
					pm.popover.Destroy()
				}
			})
			pos := geometry.Point{X: 5, Y: 6}
			if err := pm.show(placement, pos); err != nil {
				t.Fatal(err)
			}
			menu := pm.popover.(*popover)
			settlePlacement(t, menu)
			if menu.Transparent() || menu.Placement() != placement || menu.Position() != pos || len(plat.options) != 2 || !plat.options[0].Transparent || plat.options[1].Transparent {
				t.Fatal("opaque retry changed placement request", plat.options)
			}
			if err := pm.show(PopoverPlacementBottom, geometry.Point{}); err != nil {
				t.Fatal(err)
			}
			settlePlacement(t, menu)
			if menu.requestedPosition != (geometry.Point{X: 80, Y: 90}) {
				t.Fatal(menu.requestedPosition)
			}
			if err := pm.ShowAt(pos); err != nil {
				t.Fatal(err)
			}
			settlePlacement(t, menu)
			if menu.Placement() != PopoverPlacementPoint || menu.requestedPosition != (geometry.Point{X: 85, Y: 86}) || len(plat.created) != 1 {
				t.Fatal("ShowAt retained rectangle placement or recreated surface", menu.requestedPosition)
			}
		})
	}
}

func TestPopoverPlacementRefreshAndRounding(t *testing.T) {
	p, w, plat, _ := placementFixture(t, geometry.Rect(0, 0, 100, 100))
	p.SetPosition(geometry.Point{X: 5, Y: 5})
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	settlePlacement(t, p)
	n := plat.natives[0]
	if p.requestedPosition != (geometry.Point{X: 70, Y: 80}) || w.points[0] != (geometry.Point{X: 85, Y: 85}) {
		t.Fatal(p.requestedPosition, w.points)
	}
	queries := len(w.points)
	for range 4 {
		p.paint()
	}
	if len(w.points) != queries {
		t.Fatal("ordinary paint queried work area")
	}
	positions := len(n.positions)
	p.Hide()
	// Owner moved: native workarea shifts in owner-local coordinates.
	w.area = geometry.Rect(-20, -30, 100, 100)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if len(n.positions) <= positions || p.requestedPosition != (geometry.Point{X: 50, Y: 50}) || p.Position() != (geometry.Point{X: 5, Y: 5}) {
		t.Fatal("stale or cumulative placement", n.positions, p.Position())
	}
	positions = len(n.positions)
	p.Hide()
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if len(n.positions) != positions+1 {
		t.Fatal("Show deduped unchanged local position")
	}
	// Asynchronous rounded size: reposition without issuing another size request.
	n.syncSize = false
	queries, requests := len(w.points), len(n.sizes)
	p.onEvent(events.SizeEvent{Width: 30.5, Height: 20.5, PixelWidth: 61, PixelHeight: 41})
	if p.requestedPosition != (geometry.Point{X: 49.5, Y: 49.5}) || len(w.points) != queries || len(n.sizes) != requests {
		t.Fatal("rounding feedback", p.requestedPosition)
	}
	settlePlacement(t, p)
	if len(n.sizes) != requests || p.width != 30.5 {
		t.Fatal("rounded size overwritten or requested repeatedly")
	}
}

func TestPopoverPlacementErrorsAreNotTransparencyFailures(t *testing.T) {
	for _, queryErr := range []error{platform.ErrUnavailable, errors.New("native failure")} {
		p, w, plat, _ := placementFixture(t, geometry.Rect(0, 0, 100, 100))
		w.err = queryErr
		p.transparent = true
		err := p.Show()
		var creation *popoverCreationError
		if !errors.Is(err, queryErr) || errors.As(err, &creation) || len(plat.created) != 0 {
			t.Fatal(err, plat.created)
		}
	}
	p, w, plat, _ := placementFixture(t, geometry.Rect(0, 0, 100, 100))
	w.err = platform.ErrUnsupported
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if p.requestedPosition != (geometry.Point{X: 80, Y: 80}) {
		t.Fatal("unsupported used fabricated area")
	}
	w.err = nil
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	settlePlacement(t, p)
	n := plat.natives[0]
	positions, sizes := len(n.positions), len(n.sizes)
	body, insets := p.bodyRect(), p.insets
	w.err = platform.ErrUnavailable
	p.RequestLayout()
	p.paint()
	if len(n.positions) != positions || len(n.sizes) != sizes || p.bodyRect() != body || p.insets != insets {
		t.Fatal("failed update partially committed")
	}
	if err := p.Show(); !errors.Is(err, platform.ErrUnavailable) {
		t.Fatal(err)
	}
	w.err, w.area = nil, geometry.Rectangle{}
	if err := p.Show(); !errors.Is(err, platform.ErrUnavailable) {
		t.Fatal("accepted invalid workarea", err)
	}
}

func TestPopoverPlacementConstrainsBeforeCreation(t *testing.T) {
	p, _, plat, app := placementFixture(t, geometry.Rect(0, 0, 200, 150))
	p.transparent = true
	app.style = style.Sheet(style.Name("popover").Shadow(style.Shadow{Color: color.Black, BlurRadius: 10}))
	p.SetWidget(&popoverMeasureWidget{size: geometry.Size{Width: 1000, Height: 2000}}) // deliberately ignores constraints
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if plat.created[0] != (geometry.Size{Width: 200, Height: 150}) || p.insets != (popoverInsets{16, 16, 16, 16}) {
		t.Fatal(plat.created, p.insets)
	}
	settlePlacement(t, p)
	if p.widget.Rect().Size != (geometry.Size{Width: 168, Height: 118}) {
		t.Fatal(p.widget.Rect())
	}
	p.Hide()
	app.style = style.Sheet(style.Name("popover").Shadow(style.Shadow{Color: color.Black, BlurRadius: 1000}))
	if err := p.Show(); err == nil {
		t.Fatal("accepted shadow larger than workarea")
	}
}

func TestPopoverPlacementCancelledByCallbacks(t *testing.T) {
	for _, cancel := range []string{"hide", "destroy", "unmount"} {
		t.Run(cancel, func(t *testing.T) {
			p, w, plat, _ := placementFixture(t, geometry.Rect(0, 0, 100, 100))
			w.onQuery = func() {
				switch cancel {
				case "hide":
					p.Hide()
				case "destroy":
					p.Destroy()
				case "unmount":
					p.anchor.Window().SetWidget(nil)
				}
			}
			if err := p.Show(); err == nil || len(plat.created) != 0 {
				t.Fatal("created after cancellation", err)
			}
		})
	}
	p, _, plat, _ := placementFixture(t, geometry.Rect(0, 0, 100, 100))
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	n := plat.natives[0]
	n.onSize = p.Destroy
	p.SetWidget(&popoverMeasureWidget{size: geometry.Size{Width: 50, Height: 40}})
	if !n.destroyed {
		t.Fatal("resize callback did not destroy")
	}
	p.paint() // must not use released painter
}

func TestMenuPlacementScreenLimitAndInput(t *testing.T) {
	p, w, plat, app := placementFixture(t, geometry.Rect(0, 0, 240, 180))
	app.typo = &styleTypography{}
	app.style = textStyleSheet(10, color.Black)
	pm := NewPopoverMenu(p.anchor)
	t.Cleanup(func() {
		if pm.popover != nil {
			pm.popover.Destroy()
		}
	})
	m := NewMenu()
	activated := 0
	for range 30 {
		m.Append("Text", func() { activated++ })
	}
	pm.SetMenu(m)
	if err := pm.show(PopoverPlacementBottom, geometry.Point{}); err != nil {
		t.Fatal(err)
	}
	menu := pm.popover.(*popover)
	settlePlacement(t, menu)
	if w.points[0] != (geometry.Point{X: 90, Y: 85}) {
		t.Fatal("rectangle reference is not anchor center", w.points)
	}
	if plat.created[0].Height != 180 || pm.maxHeight != 0 || !pm.content.sv.vScrollable() || pm.content.sv.hScrollable() {
		t.Fatal(plat.created, pm.content.sv.contentWidth)
	}
	if menu.width != 160 {
		t.Fatalf("short scrollable menu width = %v, want default minimum 160", menu.width)
	}
	r := pm.content.list.items[0].(*menuItemRow)
	if r.trailingGap != menuScrollbarGap {
		t.Fatal("minimum width lost the menu/scrollbar gap", r.trailingGap)
	}
	position := r.base().windowRect().Center()
	for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
		menu.DispatchEvent(events.PointerEvent{EventType: kind, Position: position, Button: events.PointerButtonLeft})
	}
	if activated != 1 || pm.Visible() {
		t.Fatal("placement changed local input coordinates")
	}
	// A short menu too tall for either side still fits the whole display.
	short := NewMenu()
	for range 4 {
		short.Append("Text", nil)
	}
	pm.SetMenu(short)
	if err := pm.show(PopoverPlacementBottom, geometry.Point{}); err != nil {
		t.Fatal(err)
	}
	settlePlacement(t, menu)
	if menu.bodyRect().Height != 4*menuItemMinHeight+2*menuContentPadding || pm.content.sv.vScrollable() {
		t.Fatal("premature side-based height limit", menu.bodyRect())
	}
}

func TestMenuPlacementMinimumWidth(t *testing.T) {
	for _, tt := range []struct {
		name      string
		label     string
		areaWidth float32
		shadow    bool
		bodyWidth float32
		surfaceW  float32
	}{
		{"short Chinese", "复制", 240, false, 160, 160},
		{"narrow work area", "复制", 100, false, 100, 100},
		{"shadow excluded", "复制", 240, true, 160, 192},
		{"narrow with shadow", "Copy", 100, true, 68, 100},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, _, plat, app := placementFixture(t, geometry.Rect(0, 0, tt.areaWidth, 180))
			app.typo, app.style = &styleTypography{}, textStyleSheet(10, color.Black)
			if tt.shadow {
				app.style = style.Sheet(
					style.Name(styleNameMenuItemText).FontFamily("Test Sans").FontSize(10).ForegroundColor(color.Black),
					style.Name(styleNameMenu).Shadow(style.Shadow{Color: color.Black, BlurRadius: 10}),
				)
			}
			pm := NewPopoverMenu(p.anchor)
			t.Cleanup(func() {
				if pm.popover != nil {
					pm.popover.Destroy()
				}
			})
			model := NewMenu()
			model.Append(tt.label, nil)
			pm.SetMenu(model)
			if err := pm.ShowAt(geometry.Point{}); err != nil {
				t.Fatal(err)
			}
			menu := pm.popover.(*popover)
			settlePlacement(t, menu)
			if menu.bodyRect().Size != (geometry.Size{Width: tt.bodyWidth, Height: 40}) || plat.created[0].Width != tt.surfaceW {
				t.Fatalf("body = %v, created surface = %v", menu.bodyRect(), plat.created[0])
			}
			if pm.content.sv.hScrollable() || pm.content.sv.vScrollable() {
				t.Fatal("preferred minimum caused unnecessary scrollbars")
			}
			row := pm.content.list.items[0].(*menuItemRow)
			info := row.Snapshot()
			if pm.content.Snapshot().Role != RoleMenu || info.Role != RoleMenuItem || info.Text != tt.label || info.Bounds != row.base().windowRect() {
				t.Fatal("minimum width changed menu semantics", info)
			}
			if row.Rect().Width != tt.bodyWidth-2*menuContentPadding {
				t.Fatal("row did not occupy the wider body", row.Rect())
			}
			// A general Popover keeps its own content size; it must not inherit menu defaults.
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			settlePlacement(t, p)
			if p.bodyRect().Size != (geometry.Size{Width: 30, Height: 20}) {
				t.Fatal("menu minimum leaked into general Popover", p.bodyRect())
			}
		})
	}
}

func TestMenuPlacementLabelChangesResizeBody(t *testing.T) {
	p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
	app.typo, app.style = &styleTypography{}, textStyleSheet(10, color.Black)
	pm := NewPopoverMenu(p.anchor)
	t.Cleanup(func() {
		if pm.popover != nil {
			pm.popover.Destroy()
		}
	})
	model := NewMenu()
	item := model.Append("复制", nil)
	pm.SetMenu(model)
	if err := pm.ShowAt(geometry.Point{}); err != nil {
		t.Fatal(err)
	}
	menu := pm.popover.(*popover)
	for _, tt := range []struct {
		label string
		width float32
	}{
		{"复制", 160},
		{"A very long menu item", 238},
		{"Copy", 160},
	} {
		item.SetLabel(tt.label)
		settlePlacement(t, menu)
		if menu.bodyRect().Size != (geometry.Size{Width: tt.width, Height: 40}) {
			t.Fatalf("%q: body = %v, want %vx40", tt.label, menu.bodyRect(), tt.width)
		}
		if pm.content.sv.hScrollable() || pm.content.sv.vScrollable() {
			t.Fatal("content change caused unnecessary scrollbars")
		}
	}
}

func TestMenuPlacementHorizontalOverflow(t *testing.T) {
	p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 100, 100))
	app.typo, app.style = &styleTypography{}, textStyleSheet(10, color.Black)
	model := NewMenu()
	for range 8 {
		model.Append("A very long menu item", nil)
	}
	content := newMenuContent(model, 0, nil)
	p.SetWidget(content)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	settlePlacement(t, p)
	if p.width != 100 || p.height != 100 || !content.sv.hScrollable() || !content.sv.vScrollable() {
		t.Fatal("oversized menu not reachable by scrolling")
	}
	content.sv.SetScrollX(10000)
	content.sv.SetScrollY(10000)
	settlePlacement(t, p)
	if content.sv.scrollX <= 0 || content.sv.scrollY <= 0 {
		t.Fatal("cannot scroll overflow")
	}
	for _, widget := range content.list.items {
		row := widget.(*menuItemRow)
		if want := float32(len("A very long menu item")) * 10; row.label.Rect().Width < want {
			t.Fatalf("horizontal scroll clips text tail: label width %v, want >= %v", row.label.Rect().Width, want)
		}
	}
}

func TestPopoverPlacementRejectsInvalidPoint(t *testing.T) {
	p, _, plat, _ := placementFixture(t, geometry.Rect(0, 0, 100, 100))
	p.SetPosition(geometry.Point{X: float32(math.Inf(1))})
	if err := p.Show(); err == nil || len(plat.created) != 0 {
		t.Fatal(err)
	}
}

// 非模态自动关闭不抢键盘，外部按下关闭后仍沿原窗口事件路径完成一次点击。
func TestPopoverModelessDismissalAndExplicitKeys(t *testing.T) {
	p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
	w := app.windows[0]
	anchor := p.anchor
	w.SetWidget(nil)
	root := newTestWidget()
	root.AddChild(anchor)
	other := NewButton()
	root.AddChild(other)
	w.SetWidget(root)
	root.Arrange(geometry.Rect(0, 0, 400, 300))
	anchor.Arrange(geometry.Rect(20, 20, 100, 40))
	other.Arrange(geometry.Rect(200, 20, 100, 40))
	t.Cleanup(w.Destroy)
	keys, ownerKeys, clicks, dismissed := 0, 0, 0, 0
	key := NewKeyEventController()
	key.ConnectKeyDown(func(EventContext, events.KeyEvent) { keys++ })
	p.Widget().AddEventController(key)
	p.Widget().SetFocusable(true)
	ownerKey := NewKeyEventController()
	ownerKey.ConnectKeyDown(func(EventContext, events.KeyEvent) { ownerKeys++ })
	anchor.AddEventController(ownerKey)
	other.ConnectClicked(func() { clicks++ })
	p.ConnectDismissRequest(func() { dismissed++; p.Hide() })
	show := func() {
		t.Helper()
		if err := p.Show(); err != nil {
			t.Fatal(err)
		}
		p.SetFocusedWidget(p.Widget())
	}
	if p.AutoDismiss() || p.Modal() {
		t.Fatal("默认策略不是被动非模态面")
	}
	p.SetAutoDismiss(true)
	show()
	w.SetFocusedWidget(anchor)
	if err := w.DispatchEvent(shortcutPress(events.KeyQ, 0)); err != nil {
		t.Fatal(err)
	}
	if ownerKeys != 1 || keys != 0 {
		t.Fatal("非模态面自动抢走了键盘")
	}
	if err := p.DispatchEvent(shortcutPress(events.KeyQ, 0)); err != nil {
		t.Fatal(err)
	}
	if keys != 1 {
		t.Fatal("显式键盘转发没有进入普通控制器")
	}
	press := func(x float32) {
		t.Helper()
		for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
			if err := w.DispatchEvent(events.PointerEvent{EventType: kind, Position: geometry.Point{X: x, Y: 30}, Button: events.PointerButtonLeft}); err != nil {
				t.Fatal(err)
			}
		}
	}
	press(40)
	if !p.Visible() || dismissed != 0 {
		t.Fatal("锚点被当作外部点击")
	}
	press(220)
	if p.Visible() || clicks != 1 || dismissed != 1 {
		t.Fatal("外部关闭吞掉、重复或阻止了原点击", clicks, dismissed)
	}
	if err := p.DispatchEvent(shortcutPress(events.KeyQ, 0)); err == nil {
		t.Fatal("隐藏面仍接受键盘")
	}
	show()
	handled := false
	e := shortcutPress(events.KeyEscape, 0)
	e.Handled = &handled
	if err := w.DispatchEvent(e); err != nil {
		t.Fatal(err)
	}
	if p.Visible() || dismissed != 2 || !handled {
		t.Fatal("Esc 没有关闭并消费")
	}
	p.SetAutoDismiss(false)
	p.SetModal(true)
	show()
	if err := w.DispatchEvent(shortcutPress(events.KeyEscape, 0)); err != nil {
		t.Fatal(err)
	}
	if !p.Visible() || dismissed != 2 || keys != 2 {
		t.Fatal("持久模态面的 Esc 没有交给内容", keys, dismissed)
	}
	p.SetAutoDismiss(true)
	if err := w.DispatchEvent(events.FocusEvent{Focused: false}); err != nil {
		t.Fatal(err)
	}
	if p.Visible() || dismissed != 3 {
		t.Fatal("失焦没有请求关闭")
	}
}

// 原生面被替换后，旧面的延迟释放不能撤销新的模态登记。
func TestPopoverResignKeepsReplacementTarget(t *testing.T) {
	w := &window{}
	old, next := &popover{owner: w}, &popover{owner: w}
	w.SetModalTarget(next)
	old.resignModalTarget()
	if w.modalTarget != next {
		t.Fatal("旧面清除了替换目标")
	}
	next.resignModalTarget()
	if w.modalTarget != nil {
		t.Fatal("当前目标没有正确撤销")
	}
}

func TestPopoverModelessDismissalReentry(t *testing.T) {
	for _, action := range []string{"new surface", "new root", "destroy"} {
		t.Run(action, func(t *testing.T) {
			p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
			w := app.windows[0]
			w.SetWidget(nil)
			root := newTestWidget()
			root.AddChild(p.anchor)
			button := NewButton()
			root.AddChild(button)
			w.SetWidget(root)
			root.Arrange(geometry.Rect(0, 0, 400, 300))
			p.anchor.Arrange(geometry.Rect(0, 0, 80, 40))
			button.Arrange(geometry.Rect(150, 0, 100, 40))
			t.Cleanup(w.Destroy)
			p.SetAutoDismiss(true)
			var replacement Popover
			p.ConnectDismissRequest(func() {
				p.Hide()
				switch action {
				case "new surface":
					replacement = NewPopover(p.anchor, nil)
					replacement.SetWidget(&popoverMeasureWidget{size: geometry.Size{Width: 20, Height: 20}})
					if err := replacement.Show(); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(replacement.Destroy)
				case "new root":
					w.SetWidget(button)
					button.Arrange(geometry.Rect(0, 0, 400, 300))
				case "destroy":
					w.Destroy()
				}
			})
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			if err := w.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 180, Y: 20}, Button: events.PointerButtonLeft}); err != nil {
				t.Fatal(err)
			}
			if button.pressed {
				t.Fatal("旧事件泄漏给关闭回调改变后的界面")
			}
			if replacement != nil && !replacement.Visible() {
				t.Fatal("新面被原始关闭事件再次关闭")
			}
		})
	}
}

func TestPopoverReopenedDuringFocusReset(t *testing.T) {
	p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
	p.SetModal(true)
	p.SetAutoDismiss(true)
	p.Widget().SetFocusable(true)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	p.SetFocusedWidget(p.Widget())
	reopen, closed := true, 0
	p.ConnectClosed(func() { closed++ })
	p.Widget().ConnectFocused(func(focused bool) {
		if !focused && reopen {
			reopen = false
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
		}
	})
	p.Hide()
	w := app.windows[0]
	if !p.Visible() || w.modalTarget != p || len(w.Popovers()) != 1 || closed != 1 {
		t.Fatal("旧 Hide 清理了回调重新显示的面", p.Visible(), closed)
	}
	p.Hide()
	if p.Visible() || w.modalTarget != nil || closed != 2 {
		t.Fatal("重新显示后的正常关闭没有完成")
	}
}

// 登记模态输入会取消 owner 手势，取消回调可能打开另一表面。
func TestPopoverModalShowReentryDoesNotOwnReplacement(t *testing.T) {
	p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
	w := app.windows[0]
	w.focused = true
	w.root.Arrange(geometry.Rect(0, 0, 100, 60))
	replacement := NewPopover(p.anchor, nil).(*popover)
	replacement.SetModal(true)
	replacement.SetAutoDismiss(true)
	replacement.SetWidget(&popoverMeasureWidget{size: geometry.Size{Width: 20, Height: 20}})
	t.Cleanup(replacement.Destroy)
	var click *ClickEventController
	for _, controller := range p.anchor.EventControllers() {
		if c, ok := controller.(*ClickEventController); ok {
			click = c
			break
		}
	}
	if click == nil {
		t.Fatal("anchor controller missing")
	}
	reentered := false
	click.ConnectPressed(func(_ EventContext, pressed bool) {
		if !pressed && !reentered {
			reentered = true
			if err := replacement.Show(); err != nil {
				t.Fatal(err)
			}
		}
	})
	if err := w.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 20, Y: 20}, Button: events.PointerButtonLeft}); err != nil {
		t.Fatal(err)
	}
	if !click.Pressed() || reentered || w.dispatcher.gesture == nil {
		t.Fatalf("无效的按下初态：pressed=%v reentered=%v", click.Pressed(), reentered)
	}
	p.SetModal(true)
	p.SetAutoDismiss(true)
	if err := p.Show(); err == nil {
		t.Fatalf("Show 在登记回调替换输入目标后仍报告成功：reentered=%v current=%p expected=%p", reentered, w.modalTarget, replacement)
	}
	if !reentered || !replacement.Visible() || w.modalTarget != replacement || p.Visible() {
		t.Fatal("失效的 Show 覆盖或保留了旧表面")
	}
}

func TestPopoverDismissalSkipsReopenedPresentation(t *testing.T) {
	p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
	p.SetAutoDismiss(true)
	p.ConnectDismissRequest(p.Hide)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	next := NewPopover(p.anchor, nil)
	next.SetAutoDismiss(true)
	next.SetWidget(&popoverMeasureWidget{size: geometry.Size{Width: 20, Height: 20}})
	t.Cleanup(next.Destroy)
	next.ConnectDismissRequest(func() {
		next.Hide()
		p.Hide()
		if err := p.Show(); err != nil {
			t.Fatal(err)
		}
	})
	if err := next.Show(); err != nil {
		t.Fatal(err)
	}
	if err := app.windows[0].DispatchEvent(events.FocusEvent{Focused: false}); err != nil {
		t.Fatal(err)
	}
	if !p.Visible() || next.Visible() {
		t.Fatal("原事件关闭了回调重新显示的面")
	}
}

func TestPopoverCannotOpenFromOwnerDestroy(t *testing.T) {
	p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
	called := false
	app.windows[0].ConnectDestroy(func() {
		called = true
		if err := p.Show(); err == nil {
			t.Fatal("销毁中的 owner 仍创建了原生面")
		}
	})
	app.windows[0].Destroy()
	if !called || p.Visible() || p.platformPopup != nil {
		t.Fatal("销毁期间重入 Show 遗留资源")
	}
}

// 原生透明面仍接收阴影区域内的指针；非模态 Bottom 不得挡住锚点按钮。
func TestPopoverModelessAnchorExcludesShadow(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		anchorY                    float32
		constrained                bool
		wantTop, wantBottom, wantY float32
	}{
		{"below", 40, true, 0, 24, 72},
		{"above", 160, true, 16, 0, 104},
		{"unconstrained", 40, false, 0, 24, 72},
		{"overlapping", 80, true, 16, 24, 64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			height := float32(40)
			if tc.name == "overlapping" {
				height = 180
			}
			snapshot := popoverPlacementSnapshot{anchor: geometry.Rect(30, tc.anchorY, 80, 32), placement: PopoverPlacementBottom, workArea: geometry.Rect(0, 0, 300, 220), constrained: tc.constrained}
			i := snapshot.trimAnchorShadow(geometry.Size{Width: 100, Height: height}, popoverInsets{20, 16, 20, 24})
			if i.top != tc.wantTop || i.bottom != tc.wantBottom {
				t.Fatal("阴影裁剪错误", i)
			}
			pos := snapshot.position(geometry.Size{Width: 140, Height: height + i.top + i.bottom}, i)
			if tc.name == "overlapping" {
				if snapshot.anchorSide != 0 {
					t.Fatal("超高内容改变原滑移规则")
				}
				return
			}
			if pos.Y != tc.wantY {
				t.Fatal("主体没有贴住锚点", pos)
			}
			if snapshot.anchorSide > 0 && pos.Y < tc.anchorY+32 || snapshot.anchorSide < 0 && pos.Y+height+i.top+i.bottom > tc.anchorY {
				t.Fatal("原生表面挡住锚点", pos, i)
			}
			// 向外取整多半像素后仍不侵入锚点。
			rounded := snapshot.position(geometry.Size{Width: 140.5, Height: height + i.top + i.bottom + .5}, i)
			if snapshot.anchorSide > 0 && rounded.Y < tc.anchorY+32 || snapshot.anchorSide < 0 && rounded.Y+height+i.top+i.bottom+.5 > tc.anchorY+.5 {
				t.Fatal("原生取整改变锚点关系", rounded)
			}
		})
	}
}

func TestPopoverModelessNativeShadowDoesNotCoverAnchor(t *testing.T) {
	for _, above := range []bool{false, true} {
		t.Run(fmt.Sprint(above), func(t *testing.T) {
			p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 400, 300))
			p.transparent = true
			app.style = style.Sheet(style.Name("popover").Shadow(style.Shadow{Color: color.Black, BlurRadius: 16, Offset: geometry.Point{Y: 4}}))
			anchor := geometry.Rect(80, 40, 60, 32)
			if above {
				anchor.Y = 240
			}
			p.anchor.Arrange(anchor)
			p.SetPlacement(PopoverPlacementBottom)
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			settlePlacement(t, p)
			if above {
				if p.insets.bottom != 0 || p.requestedPosition.Y+p.height > anchor.Y {
					t.Fatal("原生阴影挡住上方菜单的锚点", p.requestedPosition, p.insets, p.height)
				}
			} else if p.insets.top != 0 || p.requestedPosition.Y < anchor.Y+anchor.Height {
				t.Fatal("原生阴影挡住下方菜单的锚点", p.requestedPosition, p.insets)
			}
		})
	}
}

func TestPopoverNativeTerminationPreservesContent(t *testing.T) {
	for _, inFrame := range []bool{false, true} {
		t.Run(fmt.Sprint(inFrame), func(t *testing.T) {
			p, _, plat, _ := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
			p.SetModal(true)
			content := p.Widget()
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			settlePlacement(t, p)
			old := plat.natives[0]
			backend := &lifecyclePainter{}
			p.painter = backend
			closed := 0
			p.ConnectClosed(func() { closed++ })
			terminate := func() { old.handler(events.DestroyEvent{}) }
			if inFrame {
				backend.onBegin = terminate
				p.paint()
			} else {
				terminate()
			}
			old.handler(events.DestroyEvent{})
			old.handler(events.PaintEvent{})
			if p.Visible() || p.platformPopup != nil || p.painter != nil || p.destroyed || content.base().destroyed || p.Widget() != content || closed != 1 || !old.destroyed {
				t.Fatalf("终止后状态错误: visible=%v destroyed=%v closed=%d", p.Visible(), p.destroyed, closed)
			}
			want := []string{"destroy"}
			if inFrame {
				want = []string{"begin", "end", "destroy"}
			}
			if fmt.Sprint(backend.calls) != fmt.Sprint(want) {
				t.Fatalf("calls=%v want=%v", backend.calls, want)
			}
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			if len(plat.natives) != 2 || p.Widget() != content || !p.Visible() {
				t.Fatal("内容没有在新原生表面中复用")
			}
			old.handler(events.DestroyEvent{})
			old.handler(events.PaintEvent{})
			if !p.Visible() || closed != 1 || p.platformPopup != plat.natives[1] {
				t.Fatal("旧表面的事件污染了重新打开的 Popover")
			}
			p.Destroy()
			if p.Widget() != nil || content.Root() != nil || closed != 2 {
				t.Fatal("最终 Destroy 没有解除内容的挂载")
			}
		})
	}
}
