package gui

import (
	"image"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
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
	win.ConnectFocusChanged(func(focused bool) {
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
		painter:     painter,
		width:       320,
		height:      240,
		pixelWidth:  640,
		pixelHeight: 480,
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
		id:     "main",
		title:  "Main",
		width:  320,
		height: 240,
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

func (w *layoutPassWidget) Measure(available geometry.Size) geometry.Size {
	w.measures++
	w.measuredAvailable = available
	return available
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

func (p *testGraphicsPainter) Begin(width, height, scale float32) {
	p.begins++
}

func (p *testGraphicsPainter) End() {
	p.ends++
}

func (p *testGraphicsPainter) SetClipRect(rect graphics.Rectangle) {}

func (p *testGraphicsPainter) Clear(color graphics.Color) {}

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

func (p *testGraphicsPainter) DrawImage(rect graphics.Rectangle, img image.Image) {}
