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
	p := &popover{modal: true, visible: true}
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
	p := &popover{rootBase: rootBase{width: 100, height: 100}, modal: true, visible: true}
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
		{"point", geometry.Rect(10, 20, 0, 0), geometry.Size{30, 20}, PopoverPlacementPoint, popoverInsets{}, geometry.Point{10, 20}},
		{"point slide", geometry.Rect(90, 90, 0, 0), geometry.Size{30, 20}, PopoverPlacementPoint, popoverInsets{}, geometry.Point{70, 80}},
		{"point top left", geometry.Rect(-30, -20, 0, 0), geometry.Size{30, 20}, PopoverPlacementPoint, popoverInsets{}, geometry.Point{}},
		{"below left", geometry.Rect(10, 10, 20, 10), geometry.Size{30, 20}, PopoverPlacementBottom, popoverInsets{}, geometry.Point{10, 20}},
		{"below right", geometry.Rect(80, 10, 20, 10), geometry.Size{30, 20}, PopoverPlacementBottom, popoverInsets{}, geometry.Point{70, 20}},
		{"above left", geometry.Rect(10, 85, 20, 10), geometry.Size{30, 20}, PopoverPlacementBottom, popoverInsets{}, geometry.Point{10, 65}},
		{"above right", geometry.Rect(80, 85, 20, 10), geometry.Size{30, 20}, PopoverPlacementBottom, popoverInsets{}, geometry.Point{70, 65}},
		{"slide before shrink", geometry.Rect(40, 40, 20, 20), geometry.Size{90, 90}, PopoverPlacementBottom, popoverInsets{}, geometry.Point{10, 10}},
		{"shadow body alignment", geometry.Rect(10, 10, 20, 10), geometry.Size{38, 30}, PopoverPlacementBottom, popoverInsets{3, 4, 5, 6}, geometry.Point{7, 16}},
		{"shadow containment flips", geometry.Rect(10, 70, 20, 10), geometry.Size{38, 30}, PopoverPlacementBottom, popoverInsets{3, 4, 5, 6}, geometry.Point{7, 46}},
		{"actual rounded oversized", geometry.Rect(90, 90, 0, 0), geometry.Size{100.5, 100.5}, PopoverPlacementPoint, popoverInsets{}, geometry.Point{}},
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
	if got := s.position(geometry.Size{50, 40}, popoverInsets{}); got != (geometry.Point{-100, -50}) {
		t.Fatal("negative workarea lost", got)
	}
	if limit, err := s.bodyLimit(popoverInsets{10, 20, 30, 40}); err != nil || limit != (geometry.Size{260, 140}) {
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
	p.sizes = append(p.sizes, geometry.Size{w, h})
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
	p.SetWidget(&popoverMeasureWidget{size: geometry.Size{30, 20}})
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
	p.SetPosition(geometry.Point{5, 5})
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	settlePlacement(t, p)
	n := plat.natives[0]
	if p.requestedPosition != (geometry.Point{70, 80}) || w.points[0] != (geometry.Point{85, 85}) {
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
	if len(n.positions) <= positions || p.requestedPosition != (geometry.Point{50, 50}) || p.Position() != (geometry.Point{5, 5}) {
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
	if p.requestedPosition != (geometry.Point{49.5, 49.5}) || len(w.points) != queries || len(n.sizes) != requests {
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
	if p.requestedPosition != (geometry.Point{80, 80}) {
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
	p.SetWidget(&popoverMeasureWidget{size: geometry.Size{1000, 2000}}) // deliberately ignores constraints
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if plat.created[0] != (geometry.Size{200, 150}) || p.insets != (popoverInsets{16, 16, 16, 16}) {
		t.Fatal(plat.created, p.insets)
	}
	settlePlacement(t, p)
	if p.widget.Rect().Size != (geometry.Size{168, 118}) {
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
	p.SetWidget(&popoverMeasureWidget{size: geometry.Size{50, 40}})
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
	if w.points[0] != (geometry.Point{90, 85}) {
		t.Fatal("rectangle reference is not anchor center", w.points)
	}
	if plat.created[0].Height != 180 || pm.maxHeight != 0 || !pm.content.sv.vScrollable() || pm.content.sv.hScrollable() {
		t.Fatal(plat.created, pm.content.sv.contentWidth)
	}
	if want := float32(40 + 2*menuItemPadding + 2*menuContentPadding + scrollbarWidth + menuScrollbarGap); menu.width != want {
		t.Fatal(menu.width, want)
	}
	r := pm.content.list.items[0].(*menuItemRow)
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
