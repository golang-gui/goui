package gui

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

type chromeTestWindow struct {
	platform.Window
	destroyed bool
	handler   platform.EventHandler
}

func (w *chromeTestWindow) Title() string       { return "" }
func (w *chromeTestWindow) RequestPaint() error { return nil }
func (w *chromeTestWindow) Show() error         { return nil }
func (w *chromeTestWindow) RequestClose() error {
	if w.handler != nil {
		w.handler(events.CloseEvent{})
	}
	return nil
}
func (w *chromeTestWindow) Destroy() { w.destroyed = true }

type chromeTestPlatform struct {
	platform.Platform
	options                     WindowOptions
	history                     []WindowOptions
	created                     int
	native                      platform.Window
	creationNotifications       bool
	creationError, painterError error
	unsupportedIntegrated       bool
}

func (p *chromeTestPlatform) NewWindow(size geometry.Size, handler platform.EventHandler, options platform.WindowOptions) (platform.Window, error) {
	p.options = WindowOptions{Size: size, Chrome: WindowChromeMode(options.Chrome), Transparent: options.Transparent}
	p.history = append(p.history, p.options)
	p.created++
	if p.creationError != nil {
		return nil, p.creationError
	}
	if p.unsupportedIntegrated && options.Chrome == platform.WindowChromeIntegrated {
		return nil, platform.ErrUnsupported
	}
	switch native := p.native.(type) {
	case *desktopTestWindow:
		native.handler = handler
	case *chromeTestWindow:
		native.handler = handler
	}
	if p.creationNotifications {
		handler(events.SizeEvent{Width: size.Width, Height: size.Height, PixelWidth: size.Width * 2, PixelHeight: size.Height * 2})
		handler(events.StateEvent{State: WindowStateHidden})
	}
	return p.native, nil
}
func (p *chromeTestPlatform) NewPainter(platform.Surface) (graphics.Painter, error) {
	return nil, p.painterError
}
func (*chromeTestPlatform) NewInputMethod(platform.Window, platform.InputMethodHandler) (platform.InputMethod, error) {
	return nil, platform.ErrUnsupported
}
func (*chromeTestPlatform) NewCursor(platform.Window) (platform.Cursor, error) {
	return nil, platform.ErrUnsupported
}

func chromeFixture(t *testing.T, nativeButtons, nativeHit bool) (*window, *desktopTestWindow) {
	t.Helper()
	native := &desktopTestWindow{chrome: platform.WindowChromeIntegrated, state: WindowStateNormal,
		nativeButtons: nativeButtons, controls: geometry.Rect(12, 8, 56, 14)}
	if !nativeHit {
		native.hitError = platform.ErrUnsupported
	}
	app := &application{platform: &chromeTestPlatform{native: native, creationNotifications: true}}
	result, err := app.NewWindow(&WindowOptions{Size: geometry.Size{Width: 640, Height: 400}, Chrome: WindowChromeIntegrated})
	if err != nil {
		t.Fatal(err)
	}
	win := result.(*window)
	win.SetWidget(newTestWidget())
	win.paint()
	native.positions = nil
	t.Cleanup(win.Destroy)
	return win, native
}

func chromeInfo(chrome WindowChrome) (info ChromeInfo) {
	h := chrome.ConnectInfo(func(v ChromeInfo) { info = v })
	h.Disconnect()
	return
}

func TestWindowCreationOptionsAndService(t *testing.T) {
	for _, mode := range []WindowChromeMode{WindowChromeNative, WindowChromeNone} {
		plat := &chromeTestPlatform{native: &chromeTestWindow{}, creationNotifications: true}
		app := &application{platform: plat}
		options := WindowOptions{Size: geometry.Size{Width: 320, Height: 200}, Chrome: mode}
		win, err := app.NewWindow(&options)
		if err != nil {
			t.Fatal(err)
		}
		if plat.options != options || win.Chrome() == nil || win.Chrome() != win.Chrome() {
			t.Fatal("lost creation/service contract")
		}
		if info := chromeInfo(win.Chrome()); info.Mode != mode || info.Enabled {
			t.Fatalf("wrong fallback info: %+v", info)
		}
		if win.Widget() != nil {
			t.Fatal("injected hidden root")
		}
		root := NewLinearBox(layout.DirectionVertical)
		win.SetWidget(root)
		if win.Widget() != root {
			t.Fatal("wrapped user root")
		}
		if win.(*window).width != 320 || win.(*window).pixelWidth != 640 {
			t.Fatal("lost creation-time size event")
		}
		win.Destroy()
	}
}

func TestNoneChromeResizeRegions(t *testing.T) {
	for _, nativeHit := range []bool{false, true} {
		native := &desktopTestWindow{chrome: platform.WindowChromeNone, state: WindowStateNormal}
		if !nativeHit {
			native.hitError = platform.ErrUnsupported
		}
		app := &application{platform: &chromeTestPlatform{native: native, creationNotifications: true}}
		result, err := app.NewWindow(&WindowOptions{Chrome: WindowChromeNone, Transparent: true})
		if err != nil {
			t.Fatal(err)
		}
		win := result.(*window)
		t.Cleanup(win.Destroy)
		root := newTestWidget()
		win.SetWidget(root)
		win.paint()
		if !win.chrome.info.Enabled || win.chrome.info.Controls != ChromeControlsNone || win.controls != nil {
			t.Fatal("None must enable regions without adding controls")
		}
		region := ChromeRegionTop
		win.Chrome().ConnectQueryRegion(func(_ geometry.Point, r *ChromeRegion) { *r = region })
		clicks := 0
		click := NewClickEventController()
		click.ConnectClicked(func(EventContext) { clicks++ })
		root.AddEventController(click)
		hits := []platform.WindowHit{platform.WindowHitTop, platform.WindowHitBottom, platform.WindowHitLeft, platform.WindowHitRight,
			platform.WindowHitTopLeft, platform.WindowHitTopRight, platform.WindowHitBottomLeft, platform.WindowHitBottomRight}
		for i, hit := range hits {
			region = ChromeRegionTop + ChromeRegion(i)
			if nativeHit {
				if got := native.hitTest(geometry.Point{X: 20, Y: 20}); got != hit {
					t.Fatalf("hit %v != %v", got, hit)
				}
			} else {
				clickAt(win, geometry.Point{X: 20, Y: 20})
				if len(native.resizeRequests) != i+1 || native.resizeRequests[i] != platform.WindowEdge(i) {
					t.Fatal(native.resizeRequests)
				}
			}
		}
		if clicks != 0 || click.Pressed() || win.dispatcher.captureTarget != nil {
			t.Fatal("resize started a widget click")
		}
		if nativeHit && len(native.resizeRequests) != 0 {
			t.Fatal("duplicate native operation")
		}
		if !nativeHit {
			native.commandError = platform.ErrUnavailable
			clickAt(win, geometry.Point{X: 20, Y: 20})
			if clicks != 1 {
				t.Fatal("failed resize swallowed input")
			}
			native.commandError = nil
			native.onMove = win.Destroy
			clickAt(win, geometry.Point{X: 20, Y: 20})
			if !win.destroyed || clicks != 1 {
				t.Fatal("resize destruction was unsafe")
			}
		}
	}
}

func TestWindowCreationFallbackAndErrors(t *testing.T) {
	for _, failure := range []error{nil, errors.New("creation failure")} {
		plat := &chromeTestPlatform{native: &chromeTestWindow{}, unsupportedIntegrated: true, creationError: failure}
		app := &application{platform: plat}
		win, err := app.NewWindow(&WindowOptions{Chrome: WindowChromeIntegrated})
		if failure != nil {
			if win != nil || !errors.Is(err, failure) || plat.created != 1 {
				t.Fatal("retried real creation error")
			}
			continue
		}
		if err != nil || plat.created != 2 || chromeInfo(win.Chrome()).Enabled {
			t.Fatalf("fallback: %v, %+v", err, plat.history)
		}
		if plat.history[0].Chrome != WindowChromeIntegrated || plat.history[1] != defaultWindowOptions {
			t.Fatal("incorrect fallback options")
		}
		win.Destroy()
	}
	winNative := &desktopTestWindow{chrome: platform.WindowChromeIntegrated}
	plat := &chromeTestPlatform{native: winNative, painterError: platform.ErrUnsupported}
	app := &application{platform: plat}
	if win, err := app.NewWindow(&WindowOptions{Chrome: WindowChromeIntegrated}); win != nil || err == nil || plat.created != 1 || !winNative.destroyed || winNative.hitTest != nil {
		t.Fatal("painter failure retried or leaked native callback")
	}
}

func TestWindowDefaultOptionsAndInvalidCreation(t *testing.T) {
	plat := &chromeTestPlatform{native: &chromeTestWindow{}}
	app := &application{platform: plat}
	win, err := app.NewWindow(nil)
	if err != nil || plat.options != defaultWindowOptions {
		t.Fatal("native defaults changed")
	}
	win.Destroy()
	for _, value := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
		if _, err := app.NewWindow(&WindowOptions{Size: geometry.Size{Width: value}}); err == nil || plat.created != 1 {
			t.Fatal("invalid size allocated window")
		}
	}
}

func TestChromeOrderedQueriesAndHandles(t *testing.T) {
	win, native := chromeFixture(t, false, true)
	var order []int
	p := geometry.Point{X: 20, Y: 20}
	first := win.Chrome().ConnectQueryRegion(func(got geometry.Point, r *ChromeRegion) {
		if got != p {
			t.Fatal("changed client DIP")
		}
		order = append(order, 1)
		*r = ChromeRegionCaption
	})
	second := win.Chrome().ConnectQueryRegion(func(_ geometry.Point, r *ChromeRegion) { order = append(order, 2); *r = ChromeRegionClient })
	third := win.Chrome().ConnectQueryRegion(func(_ geometry.Point, r *ChromeRegion) { order = append(order, 3); *r = ChromeRegionMaximize })
	if native.hitTest(p) != platform.WindowHitMaximize || !reflect.DeepEqual(order, []int{1, 2, 3}) {
		t.Fatal("query short-circuited or reordered")
	}
	third.Block()
	if native.hitTest(p) != platform.WindowHitClient {
		t.Fatal("blocking failed")
	}
	third.Unblock()
	third.Disconnect()
	third.Disconnect()
	second.Disconnect()
	if native.hitTest(p) != platform.WindowHitCaption {
		t.Fatal("independent disconnect failed")
	}
	first.Disconnect()
	if native.hitTest(p) != platform.WindowHitDefault {
		t.Fatal("empty query overwrote native edges")
	}
}

func TestChromeReentrancyAndDestroy(t *testing.T) {
	win, native := chromeFixture(t, false, true)
	service := win.Chrome()
	query := native.hitTest
	service.ConnectQueryRegion(func(p geometry.Point, r *ChromeRegion) {
		if query(p) != platform.WindowHitDefault {
			t.Fatal("reentrant query did not defer")
		}
		*r = ChromeRegionCaption
	})
	if query(geometry.Point{}) != platform.WindowHitCaption {
		t.Fatal("outer query lost")
	}
	win.ConnectDestroy(func() {
		if native.hitTest != nil {
			t.Error("native callback retained during destroy")
		}
	})
	win.Destroy()
	if service != win.Chrome() || query(geometry.Point{}) != platform.WindowHitDefault {
		t.Fatal("stale service")
	}
	called := false
	service.ConnectInfo(func(ChromeInfo) { called = true })
	if called {
		t.Fatal("post-destroy callback")
	}
}

func TestChromeNativeInfoAndLayout(t *testing.T) {
	win, native := chromeFixture(t, true, false)
	service := win.Chrome()
	var observations []ChromeInfo
	service.ConnectInfo(func(info ChromeInfo) { observations = append(observations, info) })
	if len(observations) != 1 || observations[0].Controls != ChromeControlsNative {
		t.Fatal("missing initial native info")
	}
	height := float32(14) // intrinsic height: zero Y is a valid centered origin
	position := geometry.Point{X: 12}
	binding := service.ConnectQueryControls(func(r *ChromeControls) { r.Height = height })
	win.paint()
	win.paint()
	if len(native.positions) != 1 || !native.positions[0].HasPosition {
		t.Fatalf("duplicate/lost origin request: %v", native.positions)
	}
	if len(observations) != 2 || observations[1].ControlsBounds.Pos != position {
		t.Fatal("actual position was not observed")
	}
	position = geometry.Point{X: 12, Y: 18}
	height = 50
	native.positionError = platform.ErrUnavailable
	win.paint()
	win.paint()
	if len(native.positions) != 2 || win.chrome.info.ControlsBounds.Pos == position {
		t.Fatal("failure fabricated geometry or retried on every paint")
	}
	native.positionError = nil
	win.chrome.nativeChanged()
	win.paint()
	if len(native.positions) != 3 || win.chrome.info.ControlsBounds.Pos != position {
		t.Fatal("native change did not permit retry")
	}
	binding.Disconnect()
	win.paint()
	if last := native.positions[len(native.positions)-1]; last.HasPosition {
		t.Fatal("disconnect did not restore default native placement")
	}
	observed := win.chrome.info
	count := len(observations)
	native.queryError = platform.ErrUnavailable
	win.chrome.refresh()
	if win.chrome.info != observed || len(observations) != count {
		t.Fatal("temporary native failure changed the public observation")
	}
}

func TestChromeNotificationDestroySkipsLaterCallbacks(t *testing.T) {
	win, native := chromeFixture(t, true, false)
	armed := false
	win.Chrome().ConnectInfo(func(ChromeInfo) {
		if armed {
			win.Destroy()
		}
	})
	later := 0
	win.Chrome().ConnectInfo(func(ChromeInfo) {
		if armed {
			later++
		}
	})
	armed = true
	native.controls.Width++
	win.chrome.refresh()
	if later != 0 || !native.destroyed {
		t.Fatal("notification accessed destroyed owner")
	}
}

func TestChromeInvalidHeightAndNativeSizeChanges(t *testing.T) {
	for _, nativeButtons := range []bool{false, true} {
		win, native := chromeFixture(t, nativeButtons, !nativeButtons)
		height := float32(80)
		win.Chrome().ConnectQueryControls(func(result *ChromeControls) { result.Height = height })
		win.paint()
		valid := chromeInfo(win.Chrome())
		count := len(native.positions)
		for _, invalid := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
			height = invalid
			win.paint()
			if chromeInfo(win.Chrome()) != valid || len(native.positions) != count {
				t.Fatal("invalid height changed geometry or reached native positioning")
			}
		}
		height = 80
		if nativeButtons {
			native.controls.Height = 20
			win.chrome.nativeChanged()
			win.paint()
			if native.controls.Y != 30 {
				t.Fatal("native size change was not re-centered")
			}
		}
	}
}

func TestChromeLayoutSignalOrderAndDestruction(t *testing.T) {
	win, native := chromeFixture(t, true, false)
	win.Chrome().ConnectQueryControls(func(r *ChromeControls) {
		r.Height = 50
	})
	last := win.Chrome().ConnectQueryControls(func(r *ChromeControls) {
		r.Height = 54
	})
	win.paint()
	if native.controls.Pos != (geometry.Point{X: 12, Y: 20}) {
		t.Fatal("layout signals were not last-writer-wins")
	}
	last.Disconnect()
	win.Chrome().ConnectQueryControls(func(*ChromeControls) { win.Destroy() })
	called := false
	win.Chrome().ConnectQueryControls(func(*ChromeControls) { called = true })
	count := len(native.positions)
	win.paint()
	if called || len(native.positions) != count {
		t.Fatal("layout used destroyed native window")
	}
}

func TestChromeMoveCaptureFailureAndReentrantDestroy(t *testing.T) {
	win, native := chromeFixture(t, false, false)
	win.Chrome().ConnectQueryRegion(func(_ geometry.Point, r *ChromeRegion) { *r = ChromeRegionCaption })
	win.dispatcher.captureTarget = win.Widget()
	clickAt(win, geometry.Point{X: 20, Y: 20})
	if native.moveRequests != 0 {
		t.Fatal("move stole existing GUI capture")
	}
	native.commandError = platform.ErrUnavailable
	click := NewClickEventController()
	clicked := 0
	click.ConnectClicked(func(EventContext) { clicked++ })
	win.Widget().AddEventController(click)
	clickAt(win, geometry.Point{X: 20, Y: 20})
	if clicked != 1 || native.moveRequests != 1 {
		t.Fatal("failed move swallowed ordinary input")
	}
	native.commandError = nil
	native.onMove = win.Destroy
	clickAt(win, geometry.Point{X: 20, Y: 20})
	if !win.destroyed || clicked != 1 {
		t.Fatal("reentrant native drag used destroyed Widget tree")
	}
}

func TestChromeMoveUsesEventController(t *testing.T) {
	for _, nativeHit := range []bool{true, false} {
		win, native := chromeFixture(t, false, nativeHit)
		win.Chrome().ConnectQueryRegion(func(_ geometry.Point, r *ChromeRegion) { *r = ChromeRegionCaption })
		clicks := 0
		click := NewClickEventController()
		click.ConnectClicked(func(EventContext) { clicks++ })
		win.Widget().AddEventController(click)
		for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
			_ = win.DispatchEvent(events.PointerEvent{EventType: kind, Button: events.PointerButtonLeft, Position: geometry.Point{X: 20, Y: 20}})
		}
		if nativeHit {
			if native.moveRequests != 0 {
				t.Fatal("Windows took duplicate move path")
			}
		} else if native.moveRequests != 1 || clicks != 0 || click.Pressed() || win.dispatcher.captureTarget != nil {
			t.Fatal("native move started a GUI click/capture")
		}
		win.Destroy()
	}
}

func TestDisabledChromeIsInert(t *testing.T) {
	win := &window{platformWindow: &chromeTestWindow{}}
	defer win.Destroy()
	service := win.Chrome()
	info := chromeInfo(service)
	if info.Enabled || info.Controls != ChromeControlsNone {
		t.Fatal("disabled service is not empty")
	}
	called := false
	connections := signal.Handles{
		service.ConnectQueryRegion(func(geometry.Point, *ChromeRegion) { called = true }),
		service.ConnectQueryControls(func(*ChromeControls) { called = true }),
	}
	win.chrome.queryRegion(geometry.Point{})
	win.chrome.beforeLayout()
	win.chrome.afterLayout()
	connections.Disconnect()
	if called {
		t.Fatal("disabled service emitted native queries")
	}
}

func TestWindowChromeKeepsCloseVeto(t *testing.T) {
	native := &chromeTestWindow{}
	win := &window{platformWindow: native}
	h := win.ConnectCloseRequest(func(allow *bool) { *allow = false })
	_ = win.DispatchEvent(events.CloseEvent{})
	if native.destroyed {
		t.Fatal("close veto bypassed")
	}
	h.Disconnect()
	_ = win.DispatchEvent(events.CloseEvent{})
	if !native.destroyed {
		t.Fatal("accepted close ignored")
	}
}
