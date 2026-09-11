package gui

import (
	"errors"
	"image"
	"reflect"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

func TestWindowSetWidget(t *testing.T) {
	win := &window{}
	root := newTestWidget()

	win.SetWidget(root)

	if win.Widget() != root {
		t.Fatal("window root widget was not set")
	}
	if root.Window() != win {
		t.Fatal("root widget window was not set")
	}

	win.SetWidget(nil)
	if root.Window() != nil {
		t.Fatal("old root widget window was not cleared")
	}
}

func TestWindowRequestPaintWithoutPlatformWindow(t *testing.T) {
	win := &window{}

	if err := win.RequestPaint(); err != nil {
		t.Fatal(err)
	}
}

func TestWindowDispatchEventHandlesSize(t *testing.T) {
	win := &window{}

	// SizeEvent carries both logical (DIP) and physical (pixel) size; scale is
	// derived as PixelWidth/Width (here 2x).
	if err := win.DispatchEvent(events.SizeEvent{
		Width: 320, Height: 240,
		PixelWidth: 640, PixelHeight: 480,
	}); err != nil {
		t.Fatal(err)
	}
	if win.width != 320 || win.height != 240 {
		t.Fatalf("unexpected logical size: %gx%g", win.width, win.height)
	}
	if win.pixelWidth != 640 || win.pixelHeight != 480 {
		t.Fatalf("unexpected physical size: %gx%g", win.pixelWidth, win.pixelHeight)
	}
	if !win.layoutDirty || !win.paintDirty {
		t.Fatal("size event did not request layout and paint")
	}
}

func TestWindowDispatchEventHandlesFocus(t *testing.T) {
	win := &window{}
	var calls []bool
	win.ConnectFocus(func(focused bool) {
		calls = append(calls, focused)
	})

	if err := win.DispatchEvent(events.FocusEvent{Focused: true}); err != nil {
		t.Fatal(err)
	}
	if !win.Focused() {
		t.Fatal("window focus was not set")
	}

	if err := win.DispatchEvent(events.FocusEvent{Focused: true}); err != nil {
		t.Fatal(err)
	}
	if err := win.DispatchEvent(events.FocusEvent{Focused: false}); err != nil {
		t.Fatal(err)
	}
	if win.Focused() {
		t.Fatal("window focus was not cleared")
	}

	if len(calls) != 2 || !calls[0] || calls[1] {
		t.Fatalf("unexpected focus changed calls: %v", calls)
	}
}

func TestWindowSetFocusedWidgetValidatesTarget(t *testing.T) {
	win := &window{}
	root := newTestWidget()
	child := newTestWidget()
	root.AddChild(child)
	win.SetWidget(root)

	if win.SetFocusedWidget(child) {
		t.Fatal("non-focusable widget should not be focused")
	}
	child.SetFocusable(true)
	if !win.SetFocusedWidget(child) {
		t.Fatal("focusable child should be focused")
	}
	if win.FocusedWidget() != child || !child.Focused() || !child.ContainsFocus() || !root.ContainsFocus() {
		t.Fatal("focused widget state was not set")
	}

	other := newTestWidget()
	other.SetFocusable(true)
	if win.SetFocusedWidget(other) {
		t.Fatal("unmounted widget should not be focused")
	}

	child.SetVisible(false)
	if win.SetFocusedWidget(child) {
		t.Fatal("hidden widget should not be focused")
	}
}

func TestWindowPaintPerformsPendingLayoutBeforePainting(t *testing.T) {
	painter := new(testGraphicsPainter)
	win := &window{
		rootBase: rootBase{
			painter:     painter,
			width:       320,
			height:      240,
			pixelWidth:  640,
			pixelHeight: 480,
		},
	}
	root := newLayoutPassWidget()
	win.SetWidget(root)

	win.layoutDirty = false
	win.paintDirty = false

	root.RequestLayout()

	if !win.layoutDirty {
		t.Fatal("request layout did not mark window layout dirty")
	}
	if !win.paintDirty {
		t.Fatal("request layout did not request paint")
	}

	if err := win.DispatchEvent(events.PaintEvent{}); err != nil {
		t.Fatal(err)
	}

	if root.measures != 1 {
		t.Fatalf("expected one measure before paint, got %d", root.measures)
	}
	if root.arranges != 1 {
		t.Fatalf("expected one arrange before paint, got %d", root.arranges)
	}
	if root.paints != 1 {
		t.Fatalf("expected one paint, got %d", root.paints)
	}
	if root.measuredAvailable != (geometry.Size{Width: 320, Height: 240}) {
		t.Fatalf("unexpected measure available size: %+v", root.measuredAvailable)
	}
	if root.arrangedRect != geometry.Rect(0, 0, 320, 240) {
		t.Fatalf("unexpected arranged rect: %+v", root.arrangedRect)
	}
	if win.layoutDirty {
		t.Fatal("layout dirty was not cleared after paint")
	}
	if win.paintDirty {
		t.Fatal("paint dirty was not cleared after paint")
	}
	if painter.begins != 1 || painter.ends != 1 {
		t.Fatalf("unexpected painter calls: begin=%d end=%d", painter.begins, painter.ends)
	}
}

func TestWindowCloseRequestCanPreventDestroy(t *testing.T) {
	win := &window{}
	destroyed := false

	win.ConnectCloseRequest(func(allow *bool) {
		*allow = false
	})
	win.ConnectDestroy(func() {
		destroyed = true
	})

	if err := win.DispatchEvent(events.CloseEvent{}); err != nil {
		t.Fatal(err)
	}

	if destroyed {
		t.Fatal("destroy signal fired after close request was prevented")
	}
	if win.destroyed {
		t.Fatal("window was destroyed after close request was prevented")
	}
}

func TestWindowCloseRequestAllowsDestroy(t *testing.T) {
	win := &window{}
	destroyed := false

	win.ConnectDestroy(func() {
		destroyed = true
	})

	if err := win.DispatchEvent(events.CloseEvent{}); err != nil {
		t.Fatal(err)
	}

	if !destroyed {
		t.Fatal("destroy signal did not fire")
	}
	if !win.destroyed {
		t.Fatal("window was not destroyed")
	}
}

func TestWindowDestroyDestroysRootWidget(t *testing.T) {
	var calls []string
	win := &window{}
	root := newLifecycleWidget("root", &calls)
	child := newLifecycleWidget("child", &calls)
	root.AddChild(child)
	win.SetWidget(root)

	calls = nil
	win.Destroy()

	assertStrings(t, calls, []string{
		"child unmount",
		"root unmount",
	})
	if win.Widget() != nil {
		t.Fatal("destroyed window still has a root widget")
	}
}

func TestWindowSnapshot(t *testing.T) {
	win := &window{
		id:    "main",
		title: "Main",
		rootBase: rootBase{
			width:  320,
			height: 240,
		},
	}
	root := newTestWidget()
	root.SetID("root")
	win.SetWidget(root)

	info := win.Snapshot()

	if info.ID != "main" || info.Title != "Main" {
		t.Fatalf("unexpected window info: %+v", info)
	}
	if info.Widget.ID != "root" {
		t.Fatalf("unexpected root widget info: %+v", info.Widget)
	}
}

type layoutPassWidget struct {
	WidgetBase
	measures          int
	arranges          int
	paints            int
	measuredAvailable geometry.Size
	arrangedRect      geometry.Rectangle
}

func newLayoutPassWidget() *layoutPassWidget {
	return new(layoutPassWidget)
}

func (w *layoutPassWidget) Measure(c layout.Constraint) layout.Measurement {
	w.measures++
	w.measuredAvailable = c.Max
	return layout.Measured(c.Max)
}

func (w *layoutPassWidget) Arrange(rect geometry.Rectangle) {
	w.arranges++
	w.arrangedRect = rect
	w.WidgetBase.Arrange(rect)
}

func (w *layoutPassWidget) Paint(p Painter) {
	w.paints++
}

type testGraphicsPainter struct {
	begins int
	ends   int
}

func (p *testGraphicsPainter) Name() string { return "test" }

func (p *testGraphicsPainter) Destroy() {}

func (p *testGraphicsPainter) NewImage(src image.Image) (graphics.Image, error) {
	return newTestNativeImage(src), nil
}

func (p *testGraphicsPainter) Begin(width, height, scale float32) {
	p.begins++
}

func (p *testGraphicsPainter) End() {
	p.ends++
}

func (p *testGraphicsPainter) SetClipRect(rect graphics.Rectangle) {}

func (p *testGraphicsPainter) Clear(color graphics.Color) {}

func (p *testGraphicsPainter) DrawBoxShadow(rect graphics.Rectangle, radius float32, shadow graphics.BoxShadow) {
}

func (p *testGraphicsPainter) FillRect(rect graphics.Rectangle, brush graphics.Brush) {}

func (p *testGraphicsPainter) FillRoundRect(rect graphics.Rectangle, radius float32, brush graphics.Brush) {
}

func (p *testGraphicsPainter) FillEllipse(center graphics.Point, xRadius, yRadius float32, brush graphics.Brush) {
}

func (p *testGraphicsPainter) FillPath(path graphics.Path, brush graphics.Brush) {}

func (p *testGraphicsPainter) DrawLine(p0, p1 graphics.Point, strokeWidth float32, brush graphics.Brush) {
}

func (p *testGraphicsPainter) DrawRect(rect graphics.Rectangle, strokeWidth float32, brush graphics.Brush) {
}

func (p *testGraphicsPainter) DrawRoundRect(rect graphics.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
}

func (p *testGraphicsPainter) DrawEllipse(center graphics.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush) {
}

func (p *testGraphicsPainter) DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush) {
}

func (p *testGraphicsPainter) DrawTextLayout(origin graphics.Point, layout typography.TextLayout) {}

func (p *testGraphicsPainter) DrawImage(rect graphics.Rectangle, img graphics.Image) {}
func (p *testGraphicsPainter) SetTransform(matrix geometry.Transform)                {}

type recordedControlsPosition struct {
	Position    geometry.Point
	HasPosition bool
}

type desktopTestWindow struct {
	chromeTestWindow
	chrome                                            platform.WindowChrome
	state                                             WindowState
	controls                                          geometry.Rectangle
	nativeButtons                                     bool
	queryError, commandError, hitError, positionError error
	requests                                          []WindowState
	minimums                                          []geometry.Size
	hitTest                                           func(geometry.Point) platform.WindowHit
	moveRequests                                      int
	resizeRequests                                    []platform.WindowEdge
	positions                                         []recordedControlsPosition
	onMove                                            func()
	onState                                           func()
}

func (w *desktopTestWindow) Chrome() platform.WindowChrome { return w.chrome }
func (w *desktopTestWindow) State() WindowState            { return w.state }
func (w *desktopTestWindow) RequestState(state WindowState) error {
	w.requests = append(w.requests, state)
	if w.onState != nil {
		w.onState()
	}
	return w.commandError
}
func (w *desktopTestWindow) ControlsRect() (geometry.Rectangle, error) {
	return w.controls, w.queryError
}
func (w *desktopTestWindow) SetControlsPosition(p *geometry.Point) error {
	if !w.nativeButtons {
		return platform.ErrUnsupported
	}
	var v recordedControlsPosition
	if p != nil {
		v = recordedControlsPosition{Position: *p, HasPosition: true}
	}
	w.positions = append(w.positions, v)
	if w.positionError != nil {
		return w.positionError
	}
	if p != nil {
		w.controls.Pos = *p
	} else {
		w.controls.Pos = geometry.Point{X: 12, Y: 8}
	}
	return nil
}
func (w *desktopTestWindow) SetMinSize(width, height float32) {
	w.minimums = append(w.minimums, geometry.Size{Width: width, Height: height})
}
func (w *desktopTestWindow) SetHitTest(f func(geometry.Point) platform.WindowHit) error {
	if w.hitError != nil {
		return w.hitError
	}
	w.hitTest = f
	return nil
}
func (w *desktopTestWindow) BeginMove() error {
	w.moveRequests++
	if w.onMove != nil {
		w.onMove()
	}
	return w.commandError
}
func (w *desktopTestWindow) BeginResize(edge platform.WindowEdge) error {
	w.resizeRequests = append(w.resizeRequests, edge)
	if w.onMove != nil {
		w.onMove()
	}
	return w.commandError
}

var _ platform.DesktopWindow = (*desktopTestWindow)(nil)

func TestDesktopRequestsDoNotPredictState(t *testing.T) {
	native := &desktopTestWindow{state: WindowStateNormal}
	win := &window{platformWindow: native}
	defer win.Destroy()
	var changes []WindowState
	win.ConnectState(func(s WindowState) { changes = append(changes, s) })
	targets := []WindowState{WindowStateMinimized, WindowStateHidden, WindowStateMaximized, WindowStateFullscreen, WindowStateNormal}
	for _, state := range targets {
		win.RequestState(state)
		if win.State() != WindowStateNormal {
			t.Fatal("request predicted observation")
		}
	}
	if !reflect.DeepEqual(native.requests, targets) || len(changes) != 0 {
		t.Fatal("request manufactured state event")
	}
	for _, err := range []error{platform.ErrUnsupported, platform.ErrUnavailable, errors.New("native failure")} {
		native.commandError = err
		win.RequestState(WindowStateMaximized)
		if win.State() != WindowStateNormal || len(changes) != 0 {
			t.Fatal("failed request changed observation")
		}
	}
	native.state = WindowStateMaximized
	_ = win.DispatchEvent(events.StateEvent{State: native.state})
	if !reflect.DeepEqual(changes, []WindowState{WindowStateMaximized}) {
		t.Fatal("lost actual state")
	}
	win.Destroy()
	count := len(native.requests)
	win.RequestState(WindowStateNormal)
	if len(native.requests) != count || win.State() != WindowStateUnknown {
		t.Fatal("dead window request")
	}
}

func TestNonDesktopWindowNeedsNoDesktopImplementation(t *testing.T) {
	win := &window{platformWindow: &chromeTestWindow{}}
	defer win.Destroy()
	if win.State() != WindowStateUnknown || win.Chrome() == nil {
		t.Fatal("invalid non-desktop contract")
	}
	var info ChromeInfo
	win.Chrome().ConnectInfo(func(v ChromeInfo) { info = v })
	if info.Enabled {
		t.Fatal("invented integration")
	}
	win.RequestState(WindowStateNormal)
	win.SetMinSize(geometry.Size{Width: 100, Height: 50})
	win.updateMinSize()
}

func TestAutomaticMinimumOnlyAppliesToDesktop(t *testing.T) {
	size := geometry.Size{Width: 120, Height: 40}
	child := &countingMeasureWidget{size: size}
	win := &window{platformWindow: &chromeTestWindow{}, root: child}
	win.updateMinSize()
	if child.measures != 0 {
		t.Fatal("non-desktop host measured desktop minimum")
	}
	if got := measureWidget(child, layout.Unbounded()).Size; got != size {
		t.Fatal("widget minimum lost")
	}
	win.Destroy()
	native := &desktopTestWindow{}
	child = &countingMeasureWidget{size: size}
	win = &window{platformWindow: native, root: child}
	win.updateMinSize()
	win.updateMinSize()
	if child.measures != 1 || !reflect.DeepEqual(native.minimums, []geometry.Size{size}) {
		t.Fatal("derived minimum changed")
	}
	win.Destroy()
}
