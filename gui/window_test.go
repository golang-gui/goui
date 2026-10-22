package gui

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"reflect"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
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

type windowCursorRecorder struct{ shape platform.CursorShape }

func (c *windowCursorRecorder) SetShape(shape platform.CursorShape) { c.shape = shape }
func (*windowCursorRecorder) Destroy()                              {}

func TestWindowCapturedDragOwnsCursor(t *testing.T) {
	for _, end := range []string{"release", "cancel", "hide", "disable", "unmount", "focus-loss", "native-takeover"} {
		t.Run(end, func(t *testing.T) {
			root, handle, other := newTestWidget(), newTestWidget(), newTestWidget()
			root.Arrange(geometry.Rect(0, 0, 300, 100))
			handle.Arrange(geometry.Rect(10, 0, 8, 100))
			other.Arrange(geometry.Rect(50, 0, 200, 100))
			handle.SetCursor(CursorResizeHorizontal)
			other.SetCursor(CursorText)
			root.AddChild(handle)
			root.AddChild(other)
			drag := NewDragEventController()
			handle.AddEventController(drag)
			motion := NewMotionEventController()
			other.AddEventController(motion)
			cursor := new(windowCursorRecorder)
			win := &window{cursor: cursor}
			win.SetWidget(root)
			t.Cleanup(win.Destroy)
			pointer := func(kind events.EventType, x float32, down bool) {
				t.Helper()
				buttons := events.PointerButtons(0)
				if down {
					buttons = events.PointerButtonLeftDown
				}
				if err := win.DispatchEvent(events.PointerEvent{EventType: kind, Position: geometry.Point{X: x, Y: 30}, Button: events.PointerButtonLeft, Buttons: buttons}); err != nil {
					t.Fatal(err)
				}
			}
			pointer(events.PointerMove, 14, false)
			pointer(events.PointerDown, 14, true)
			// 大幅移动，且不执行布局：模拟快速拖动及尺寸达到限制后越过手柄。
			pointer(events.PointerMove, 200, true)
			if win.dispatcher.captureTarget != handle || !motion.Hover() {
				t.Fatal("drag capture and live hover must remain independent")
			}
			if cursor.shape != platform.CursorResizeHorizontal {
				t.Fatalf("captured cursor=%v, want horizontal resize", cursor.shape)
			}
			win.applyCursor() // A paint-side cursor refresh must not restore hover early.
			if cursor.shape != platform.CursorResizeHorizontal {
				t.Fatal("paint refresh overrode captured cursor")
			}
			switch end {
			case "release":
				pointer(events.PointerUp, 200, false)
			case "cancel":
				drag.Reset()
			case "hide":
				handle.SetVisible(false)
			case "disable":
				handle.SetEnabled(false)
			case "unmount":
				root.RemoveChild(handle)
			case "focus-loss":
				_ = win.DispatchEvent(events.FocusEvent{Focused: false})
			case "native-takeover":
				win.dispatcher.gesture.takeOver()
			}
			win.applyCursor()
			if win.dispatcher.captureTarget != nil || cursor.shape != platform.CursorText {
				t.Fatalf("%s left capture/cursor: %v %v", end, win.dispatcher.captureTarget, cursor.shape)
			}
		})
	}
}

func TestWindowStyleNameOnlyInvalidatesOwnPaint(t *testing.T) {
	win, other := &window{}, &window{}
	root := newTestWidget()
	root.SetStyleName("content-style")
	win.SetWidget(root)
	win.paintDirty, win.layoutDirty = false, false
	before := win.Snapshot()
	win.SetStyleName("special-window")
	if win.StyleName() != "special-window" || !win.paintDirty || win.layoutDirty {
		t.Fatal("window style must request paint, not change layout")
	}
	if other.StyleName() != "" || other.paintDirty || root.StyleName() != "content-style" || !reflect.DeepEqual(win.Snapshot(), before) {
		t.Fatal("window style leaked to other windows, child styles or semantic geometry")
	}
	win.paintDirty = false
	win.SetStyleName("special-window")
	if win.paintDirty {
		t.Fatal("setting the same window style must be a no-op")
	}
	win.SetStyleName("")
	if win.StyleName() != "" || !win.paintDirty {
		t.Fatal("empty style name must restore the default background")
	}
	win.Destroy()
	win.paintDirty = false
	win.SetStyleName("after-destroy")
	if win.StyleName() != "" || win.paintDirty {
		t.Fatal("destroyed window accepted a style change")
	}
}

// 物理像素采样远离圆角和抗锯齿：x=2 DIP、y=30 DIP 位于窗口背景边缘，
// 不在 5 DIP 的控件绘制区内；(30,30) 位于辅助窗的实色内容中。输出按预乘
// RGBA 精确比较，不涉及字体。分别覆盖应用 1x/2x 的独立背景、多窗口和重绘。
func TestWindowBackgroundStyleIsIsolated(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			rules := func(background color.Color) style.StyleSheet {
				return style.Sheet(
					style.Name("window").BackgroundColor(background),
					// These geometry fields must not override GUI-owned chrome.
					style.Name("transparent-window").BackgroundColor(color.Transparent).Radius(40).BorderWidth(20),
					style.Name("dark-window").BackgroundColor(color.Gray{32}),
				)
			}
			app := &application{style: rules(color.Gray{245})}
			useTestApplication(t, app)
			makeWindow := func(framed bool) (*window, *transparentFrame) {
				output := &transparentFrame{}
				p, err := software.NewPainter(output)
				if err != nil {
					t.Fatal(err)
				}
				w := &window{rootBase: rootBase{width: 80, height: 60,
					pixelWidth: 80 * scale, pixelHeight: 60 * scale, transparent: true, painter: p}, clientChrome: framed}
				w.SetWidget(newPainterTestWidget(func(p Painter) {
					p.FillRect(geometry.Rect(16, 16, 48, 28), graphics.RGB(12, 34, 56))
				}))
				t.Cleanup(w.Destroy)
				return w, output
			}
			pixel := func(output *transparentFrame, x, y float32) color.RGBA {
				return color.RGBAModel.Convert(output.image.At(int(x*scale), int(y*scale))).(color.RGBA)
			}
			main, mainOutput := makeWindow(true)
			aux, auxOutput := makeWindow(false)
			main.paint()
			aux.paint()
			if got := pixel(mainOutput, 2, 30); got != (color.RGBA{245, 245, 245, 255}) {
				t.Fatalf("default main background: %v", got)
			}
			aux.SetStyleName("transparent-window")
			aux.paint()
			main.paint()
			if got := pixel(auxOutput, 2, 30); got != (color.RGBA{}) {
				t.Fatalf("auxiliary window did not clear its own background: %v", got)
			}
			if got := pixel(auxOutput, 30, 30); got != (color.RGBA{12, 34, 56, 255}) {
				t.Fatalf("transparent background changed content painting: %v", got)
			}
			if got := pixel(mainOutput, 2, 30); got != (color.RGBA{245, 245, 245, 255}) || main.frameInset() != 5 {
				t.Fatalf("auxiliary style changed the main background or chrome: %v", got)
			}
			aux.SetStyleName("dark-window")
			aux.paint()
			if pixel(auxOutput, 2, 30) != (color.RGBA{32, 32, 32, 255}) ||
				aux.decorationPalette(true).frame.Y != 90 || main.decorationPalette(true).frame.Y != 165 {
				t.Fatal("decoration palettes must follow each window's own background")
			}
			aux.SetStyleName("")
			aux.paint()
			if got := pixel(auxOutput, 2, 30); got != (color.RGBA{245, 245, 245, 255}) {
				t.Fatalf("empty style did not restore the default: %v", got)
			}
			aux.SetStyleName("transparent-window")
			app.style = rules(color.Gray{40})
			main.paint()
			aux.paint()
			if pixel(mainOutput, 2, 30) != (color.RGBA{40, 40, 40, 255}) || pixel(auxOutput, 2, 30) != (color.RGBA{}) {
				t.Fatal("application theme update lost independent background selection")
			}
			another, anotherOutput := makeWindow(false)
			another.SetStyleName("transparent-window")
			aux.Destroy()
			main.paint()
			another.paint()
			if pixel(mainOutput, 2, 30) != (color.RGBA{40, 40, 40, 255}) || pixel(anotherOutput, 2, 30) != (color.RGBA{}) {
				t.Fatal("closing one transparent window changed other windows")
			}
		})
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
func (p *testGraphicsPainter) RenderImage(int, int, float32, func()) (image.Image, error) {
	return nil, errors.New("test painter does not rasterize")
}

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
	windowReference                                   platform.Window
	windowPosition                                    geometry.Point
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

func (w *desktopTestWindow) SetPosition(reference platform.Window, position geometry.Point) error {
	w.windowReference, w.windowPosition = reference, position
	return w.commandError
}

func (w *desktopTestWindow) Position(reference platform.Window) (geometry.Point, error) {
	w.windowReference = reference
	return w.windowPosition, w.queryError
}

func (*desktopTestWindow) WorkAreaAt(geometry.Point) (geometry.Rectangle, error) {
	return geometry.Rectangle{}, platform.ErrUnsupported
}

func TestWindowPositionDelegation(t *testing.T) {
	native, reference := &desktopTestWindow{}, &desktopTestWindow{}
	w, ref := &window{platformWindow: native}, &window{platformWindow: reference}
	p := geometry.Point{X: -20, Y: 40}
	for _, relativeTo := range []Window{nil, ref, w} {
		if err := w.SetPosition(relativeTo, p); err != nil {
			t.Fatal(err)
		}
		var want platform.Window
		if relativeTo != nil {
			want = relativeTo.PlatformWindow()
		}
		if native.windowReference != want || native.windowPosition != p {
			t.Fatal("GUI changed reference or coordinates")
		}
	}
	failure := errors.New("native positioning failed")
	native.commandError = failure
	if err := w.SetPosition(nil, p); err != failure {
		t.Fatalf("lost native error: %v", err)
	}
	// Destroy callbacks run before releasing the native window; no use is valid
	// once destruction starts, even while the native handle is still present.
	ref.destroyed = true
	if err := w.SetPosition(ref, p); !errors.Is(err, platform.ErrUnavailable) {
		t.Fatalf("accepted destroyed reference: %v", err)
	}
	if err := w.SetPosition(&window{}, p); !errors.Is(err, platform.ErrUnavailable) {
		t.Fatalf("nil reference handle became work-area positioning: %v", err)
	}
	w.destroyed = true
	if err := w.SetPosition(nil, p); !errors.Is(err, platform.ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := w.WorkAreaAt(p); !errors.Is(err, platform.ErrUnavailable) {
		t.Fatal(err)
	}
	w = &window{platformWindow: &chromeTestWindow{}}
	if err := w.SetPosition(nil, p); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := w.WorkAreaAt(p); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatal(err)
	}
}

func TestWindowObservedPosition(t *testing.T) {
	native := &desktopTestWindow{windowPosition: geometry.Point{X: -6, Y: -28}}
	w := &window{platformWindow: native}
	ref := &window{platformWindow: &desktopTestWindow{}}
	for _, relativeTo := range []Window{nil, ref, w} {
		got, err := w.Position(relativeTo)
		if err != nil || got != native.windowPosition {
			t.Fatalf("observation changed: %v, %v", got, err)
		}
		var want platform.Window
		if relativeTo != nil {
			want = relativeTo.PlatformWindow()
		}
		if native.windowReference != want {
			t.Fatal("reference changed")
		}
	}
	// Observations change without a GUI SetPosition call.
	native.windowPosition = geometry.Point{X: 500, Y: 200}
	if got, err := w.Position(nil); err != nil || got != native.windowPosition {
		t.Fatal("returned a cached request", got, err)
	}
	for _, err := range []error{platform.ErrUnavailable, platform.ErrUnsupported, errors.New("native query failed")} {
		native.queryError = err
		if _, got := w.Position(nil); got != err {
			t.Fatal("native error changed", got)
		}
	}
	native.queryError = nil
	ref.destroyed = true // native object still present during destroy callbacks
	var nilRef *window
	for _, reference := range []Window{ref, &window{}, nilRef} {
		if _, err := w.Position(reference); !errors.Is(err, platform.ErrUnavailable) {
			t.Fatal("accepted unavailable reference", err)
		}
	}
	w.destroyed = true
	if _, err := w.Position(nil); !errors.Is(err, platform.ErrUnavailable) {
		t.Fatal("accepted destroyed target", err)
	}
	w = &window{platformWindow: &chromeTestWindow{}}
	if _, err := w.Position(nil); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatal("accepted non-desktop host", err)
	}
}

func TestWindowWorkAreaDelegation(t *testing.T) {
	native := &placementWindow{area: geometry.Rect(-80, -40, 1000, 700)}
	w := &window{platformWindow: native}
	p := geometry.Point{X: 200, Y: -10}
	area, err := w.WorkAreaAt(p)
	if err != nil || area != native.area || len(native.points) != 1 || native.points[0] != p {
		t.Fatalf("query changed: %v %v %v", area, err, native.points)
	}
	native.err = errors.New("observation failed")
	if _, err := w.WorkAreaAt(p); err != native.err {
		t.Fatalf("lost native error: %v", err)
	}
}

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

type transparentFrame struct{ image image.Image }

func (*transparentFrame) Transparent() bool            { return true }
func (f *transparentFrame) Draw(img image.Image) error { f.image = img; return nil }

// 不额外添加 Padding；根布局、滚动条和 Snapshot 都必须位于四边缩放区域内侧。
func TestIntegratedContentLayoutAvoidsResizeEdges(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			w, native := chromeFixture(t, false, false)
			w.clientChrome = true
			w.pixelWidth, w.pixelHeight = w.width*scale, w.height*scale
			w.Chrome().ConnectQueryRegion(w.chrome.queryFrameRegion)
			content := newTestWidget()
			content.SetMinSize(geometry.Size{Width: 1000, Height: 1000})
			scroll := NewScrollView()
			scroll.SetChild(content)
			w.SetWidget(scroll)
			for _, state := range []WindowState{WindowStateNormal, WindowStateMaximized, WindowStateFullscreen, WindowStateNormal} {
				native.state = state
				_ = w.DispatchEvent(events.StateEvent{State: state})
				w.paint()
				inset := float32(0)
				if state == WindowStateNormal {
					inset = float32(math.Ceil(float64(5*scale))) / scale
				}
				want := geometry.Rect(inset, inset, w.width-2*inset, w.height-2*inset)
				if scroll.Rect() != want || w.Snapshot().Widget.Bounds != want {
					t.Fatalf("state=%v: root/snapshot=%v/%v, want %v", state, scroll.Rect(), w.Snapshot().Widget.Bounds, want)
				}
				for _, bar := range []*ScrollBar{scroll.vbar, scroll.hbar} {
					r := bar.windowRect()
					if !bar.Visible() || r.Intersect(want) != r || hitTest(scroll, r.Center()) != bar {
						t.Fatalf("state=%v: scrollbar painted/hit outside content: %v", state, r)
					}
					if bar == scroll.vbar && (r.Width != 8 || r.X+r.Width != want.X+want.Width) ||
						bar == scroll.hbar && (r.Height != 8 || r.Y+r.Height != want.Y+want.Height) {
						t.Fatalf("scrollbar is not full-width and flush with content: %v", r)
					}
				}
				if state == WindowStateNormal {
					for _, p := range []geometry.Point{{X: 2, Y: 100}, {X: w.width - 2, Y: 100}, {X: 100, Y: 2}, {X: 100, Y: w.height - 2}} {
						if hitTest(scroll, p) != nil || w.chrome.queryRegion(p) < ChromeRegionTop {
							t.Fatalf("resize edge reached Widget content: %v", p)
						}
					}
				}
			}
			// 普通原生 Integrated、Native 和 None 都不因相同样式或透明能力而内缩。
			w.clientChrome = false
			w.RequestLayout()
			w.paint()
			if scroll.Rect() != geometry.Rect(0, 0, w.width, w.height) {
				t.Fatal("GUI resize inset leaked to native/None hosts")
			}
		})
	}
}

func TestIntegratedAutomaticMinimumIncludesResizeEdges(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		w, native := chromeFixture(t, false, false)
		w.clientChrome = true
		w.minSizeApplied = false
		native.minimums = nil
		content := newTestWidget()
		content.SetMinSize(geometry.Size{Width: 120, Height: 60})
		w.SetWidget(content)
		if explicit {
			w.SetMinSize(geometry.Size{Width: 200, Height: 100})
		}
		w.updateMinSize()
		want := geometry.Size{Width: 130, Height: 70}
		if explicit {
			want = geometry.Size{Width: 200, Height: 100}
		}
		if !reflect.DeepEqual(native.minimums, []geometry.Size{want}) {
			t.Fatalf("explicit=%v: minimums=%v, want %v", explicit, native.minimums, want)
		}
		w.updateMinSize()
		if len(native.minimums) != 1 {
			t.Fatal("unchanged decoration reapplied the WM hint")
		}
		// 分数缩放只重算占用边缘；显式窗口最小尺寸不受影响。
		w.pixelWidth = w.width * 1.25
		w.updateMinSize()
		if !explicit {
			if got := native.minimums[len(native.minimums)-1]; got != (geometry.Size{Width: 131.2, Height: 71.2}) {
				t.Fatalf("fractional scale hint: %v", got)
			}
			native.state = WindowStateMaximized
			w.updateMinSize()
			if got := native.minimums[len(native.minimums)-1]; got != (geometry.Size{Width: 120, Height: 60}) {
				t.Fatalf("maximized hint retained floating edges: %v", got)
			}
		} else if len(native.minimums) != 1 {
			t.Fatal("scale overrode the explicit minimum")
		}
	}
}

// 在胶囊滑块中段采样，避开圆角 AA；不透明预乘 RGBA 应精确覆盖完整的
// 8 DIP 宽度（1x=8、2x=16 物理像素），紧邻外侧仍为窗口自己的白色背景。
func TestIntegratedScrollbarPixelsKeepFullWidth(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			useTestApplication(t, &application{style: style.Sheet(
				style.Name("window").BackgroundColor(color.White),
				style.Name("scroll-view").Part("thumb").BackgroundColor(color.RGBA{R: 40, G: 100, B: 200, A: 255}).Radius(4),
			)})
			w, _ := chromeFixture(t, false, false)
			w.clientChrome, w.transparent = true, true
			w.pixelWidth, w.pixelHeight = w.width*scale, w.height*scale
			output := &transparentFrame{}
			p, err := software.NewPainter(output)
			if err != nil {
				t.Fatal(err)
			}
			w.painter = p
			child := newTestWidget()
			child.SetMinSize(geometry.Size{Width: 1000, Height: 1000})
			scroll := NewScrollView()
			scroll.SetChild(child)
			w.SetWidget(scroll)
			w.paint()
			want := color.RGBA{40, 100, 200, 255}
			for _, bar := range []*ScrollBar{scroll.vbar, scroll.hbar} {
				thumb := bar.thumbRect().Translate(bar.windowRect().Pos)
				start, end := int(thumb.X*scale), int((thumb.X+thumb.Width)*scale)
				fixed := int(thumb.Center().Y * scale)
				if bar == scroll.hbar {
					start, end = int(thumb.Y*scale), int((thumb.Y+thumb.Height)*scale)
					fixed = int(thumb.Center().X * scale)
				}
				if end-start != int(8*scale) {
					t.Fatalf("thumb pixel width=%d", end-start)
				}
				for n := start; n <= end; n++ {
					x, y := n, fixed
					if bar == scroll.hbar {
						x, y = fixed, n
					}
					got := color.RGBAModel.Convert(output.image.At(x, y)).(color.RGBA)
					expected := want
					if n == end {
						expected = color.RGBA{255, 255, 255, 255}
					}
					if got != expected {
						t.Fatalf("bar=%v pixel=(%d,%d): %v, want %v", bar.Orientation(), x, y, got, expected)
					}
				}
			}
		})
	}
}

func TestWindowFrameClipMatchesContentLayout(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		w, native := chromeFixture(t, false, false)
		w.clientChrome, w.transparent = true, true
		w.pixelWidth, w.pixelHeight = w.width*scale, w.height*scale
		output := &transparentFrame{}
		p, err := software.NewPainter(output)
		if err != nil {
			t.Fatal(err)
		}
		w.painter = p
		content := newPainterTestWidget(func(p Painter) {
			// A custom Widget cannot cancel the structural frame clip.
			p.SetClipRect(geometry.Rect(-100, -100, 10000, 10000))
			p.Save()
			p.SetTransform(geometry.Translate(-20, -20))
			p.FillRect(geometry.Rect(0, 0, 10000, 10000), graphics.RGB(40, 100, 200))
			p.Restore()
		})
		w.SetWidget(content)
		w.paint()
		_, inset := w.frameStyle()
		bounds := geometry.Rect(inset, inset, w.width-2*inset, w.height-2*inset)
		if content.Rect() != bounds || content.base().windowRect() != bounds {
			t.Fatalf("content layout differs from frame clip: %v", content.Rect())
		}
		if hitTest(content, geometry.Point{X: 2, Y: 100}) != nil || hitTest(content, geometry.Point{X: 20, Y: 100}) != content {
			t.Fatal("Widget hit bounds differ from its safe allocation")
		}
		if inset < 5 || math.Abs(float64(inset*scale)-math.Round(float64(inset*scale))) > 1e-5 {
			t.Fatalf("unsafe pixel alignment: %v at %v", inset, scale)
		}
		pixel := func(x, y int) color.RGBA {
			return color.RGBAModel.Convert(output.image.At(x, y)).(color.RGBA)
		}
		if got := pixel(0, 0); got.A != 0 {
			t.Fatalf("corner not transparent: %v", got)
		}
		if got := pixel(int(2*scale), int(100*scale)); got != (color.RGBA{255, 255, 255, 255}) {
			t.Fatalf("Widget painted over frame at %vx: %v", scale, got)
		}
		if got := pixel(int(20*scale), int(100*scale)); got != (color.RGBA{40, 100, 200, 255}) {
			t.Fatalf("content missing: %v", got)
		}
		if got := pixel(0, int(100*scale)); got == (color.RGBA{255, 255, 255, 255}) || got.A == 0 {
			t.Fatalf("border inherited content clip: %v", got)
		}
		for _, state := range []WindowState{WindowStateMaximized, WindowStateFullscreen, WindowStateNormal} {
			native.state = state
			_ = w.DispatchEvent(events.StateEvent{State: state})
			w.paint()
			_, inset = w.frameStyle()
			want := geometry.Rect(inset, inset, w.width-2*inset, w.height-2*inset)
			if content.Rect() != want {
				t.Fatalf("state=%v: content=%v, want %v", state, content.Rect(), want)
			}
			if state == WindowStateNormal {
				if pixel(0, 0).A != 0 || inset == 0 {
					t.Fatal("normal frame not restored")
				}
			} else if pixel(0, 0) != (color.RGBA{40, 100, 200, 255}) || inset != 0 {
				t.Fatal("maximized/fullscreen retained frame clip")
			}
		}
		w.clientChrome = false
		w.RequestLayout()
		w.paint()
		if pixel(0, 0) != (color.RGBA{40, 100, 200, 255}) {
			t.Fatal("frame leaked to native/None")
		}
	}
}

func TestFrameStructuralClipIncludesChildrenAndDecorations(t *testing.T) {
	backend := &recordingPainterBackend{}
	root := rootBase{width: 100, height: 80, pixelWidth: 100, pixelHeight: 80, painter: backend, layoutDirty: true}
	full := geometry.Rect(0, 0, 100, 80)
	draw := func(p Painter) {
		p.SetClipRect(full)
		p.FillRect(full, graphics.RGB(255, 0, 0))
	}
	parent := &painterTestContainer{}
	child := newPainterTestWidget(draw)
	parent.AddChild(child)
	root.layoutFrame(parent)
	child.Arrange(full)
	decoration := newPainterTestWidget(draw)
	decoration.Arrange(full)
	root.drawDecoratedFrame(parent, style.Style{}, style.Style{}, 5, decoration)
	if len(backend.fills) != 2 {
		t.Fatalf("fills: %v", backend.fills)
	}
	for _, fill := range backend.fills {
		if fill.clip != geometry.Rect(5, 5, 90, 70) {
			t.Fatalf("escaped structural clip: %v", fill.clip)
		}
	}
	if backend.clip != (geometry.Rectangle{}) || backend.transform != geometry.Identity() {
		t.Fatal("frame state not restored")
	}
	// An empty safe area suppresses drawing, never disables clipping and leaks.
	backend.fills = nil
	root.drawDecoratedFrame(parent, style.Style{}, style.Style{}, 60, decoration)
	if len(backend.fills) != 0 {
		t.Fatal("empty safe area drew content")
	}
}
