package gui

import (
	"errors"
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
	size geometry.Size
}

func (w *popoverMeasureWidget) Measure(layout.Constraint) layout.Measurement {
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
		rectangle bool
		insets    popoverInsets
		want      geometry.Point
	}{
		{"point", geometry.Rect(10, 20, 0, 0), geometry.Size{30, 20}, false, popoverInsets{}, geometry.Point{10, 20}},
		{"point slide", geometry.Rect(90, 90, 0, 0), geometry.Size{30, 20}, false, popoverInsets{}, geometry.Point{70, 80}},
		{"point top left", geometry.Rect(-30, -20, 0, 0), geometry.Size{30, 20}, false, popoverInsets{}, geometry.Point{}},
		{"below left", geometry.Rect(10, 10, 20, 10), geometry.Size{30, 20}, true, popoverInsets{}, geometry.Point{10, 20}},
		{"below right", geometry.Rect(80, 10, 20, 10), geometry.Size{30, 20}, true, popoverInsets{}, geometry.Point{70, 20}},
		{"above left", geometry.Rect(10, 85, 20, 10), geometry.Size{30, 20}, true, popoverInsets{}, geometry.Point{10, 65}},
		{"above right", geometry.Rect(80, 85, 20, 10), geometry.Size{30, 20}, true, popoverInsets{}, geometry.Point{70, 65}},
		{"slide before shrink", geometry.Rect(40, 40, 20, 20), geometry.Size{90, 90}, true, popoverInsets{}, geometry.Point{10, 10}},
		{"shadow body alignment", geometry.Rect(10, 10, 20, 10), geometry.Size{38, 30}, true, popoverInsets{3, 4, 5, 6}, geometry.Point{7, 16}},
		{"shadow containment flips", geometry.Rect(10, 70, 20, 10), geometry.Size{38, 30}, true, popoverInsets{3, 4, 5, 6}, geometry.Point{7, 46}},
		{"actual rounded oversized", geometry.Rect(90, 90, 0, 0), geometry.Size{100.5, 100.5}, false, popoverInsets{}, geometry.Point{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := popoverPlacement{anchor: tc.anchor, rectangle: tc.rectangle, workArea: geometry.Rect(0, 0, 100, 100), constrained: true}
			if got := s.position(tc.size, tc.insets); got != tc.want {
				t.Fatalf("%v, want %v", got, tc.want)
			}
		})
	}
	s := popoverPlacement{workArea: geometry.Rect(-100, -50, 300, 200)}
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
}

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
	natives  []*placementNative
	created  []geometry.Size
	syncSize bool
}

func (p *placementPlatform) NewPopup(_ platform.Window, size geometry.Size, handler platform.EventHandler, _ platform.PopupOptions) (platform.Popup, error) {
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
	if err := pm.show(geometry.Point{}, true); err != nil {
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
	if err := pm.show(geometry.Point{}, true); err != nil {
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
