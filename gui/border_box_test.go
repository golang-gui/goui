package gui

import (
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

func TestBorderBoxLayoutAndSnapshot(t *testing.T) {
	b := NewBorderBox()
	child := &baselineSizedWidget{measurement: layout.MeasuredWithBaseline(geometry.Size{Width: 40, Height: 20}, 15)}
	child.SetID("content")
	b.SetChild(child)
	if !b.Resizable() || b.ResizeWidth() != 6 || b.Focusable() {
		t.Fatal("incorrect constructor defaults")
	}
	m := b.Measure(layout.Unbounded())
	if m.Size != (geometry.Size{Width: 52, Height: 32}) || !m.HasBaseline || m.Baseline != 21 {
		t.Fatalf("content/inset/baseline: %+v", m)
	}
	b.Arrange(geometry.Rect(10, 20, 100, 80))
	if child.Rect() != geometry.Rect(6, 6, 88, 68) {
		t.Fatalf("child must fill safe rectangle: %v", child.Rect())
	}
	info := b.Snapshot()
	if info.Role != RoleBox || len(info.Children) != 1 || info.Children[0].ID != "content" || info.Children[0].Bounds != geometry.Rect(16, 26, 88, 68) || len(info.Actions) != 0 {
		t.Fatalf("snapshot lost content geometry or invented interaction: %+v", info)
	}
	b.SetResizable(false)
	if b.Measure(layout.Unbounded()) != m {
		t.Fatal("disabling resize changed layout")
	}
	b.SetChild(nil)
	if child.Parent() != nil || len(b.Children()) != 0 || b.Measure(layout.Unbounded()).Size != (geometry.Size{Width: 12, Height: 12}) {
		t.Fatal("child replacement/empty measurement failed")
	}
	for _, width := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
		b.SetResizeWidth(width)
		if b.ResizeWidth() != 0 {
			t.Fatal("invalid width retained")
		}
	}
	b.SetVisible(false)
	if b.Measure(layout.Unbounded()) != (layout.Measurement{}) {
		t.Fatal("hidden box measured")
	}
}

type borderConstraintChild struct {
	WidgetBase
	constraint layout.Constraint
}

func (c *borderConstraintChild) Measure(constraint layout.Constraint) layout.Measurement {
	c.constraint = constraint
	return layout.Measured(geometry.Size{Width: 30, Height: 20}).Constrain(constraint)
}

func TestBorderBoxConstraints(t *testing.T) {
	b := NewBorderBox()
	c := &borderConstraintChild{}
	b.SetChild(c)
	b.SetMinSize(geometry.Size{Width: 80})
	b.SetMaxSize(geometry.Size{Width: 120, Height: 100})
	m := b.Measure(layout.Loose(geometry.Size{Width: 200, Height: 200}))
	if m.Size != (geometry.Size{Width: 80, Height: 32}) || c.constraint.Min.Width != 68 || c.constraint.Max.Width != 108 {
		t.Fatalf("self constraints: m=%v child=%v", m, c.constraint)
	}
	b.SetMaxSize(geometry.Size{})
	b.Measure(layout.Unbounded())
	if c.constraint.Max != (geometry.Size{Width: layout.Inf, Height: layout.Inf}) {
		t.Fatalf("inset converted Inf to finite: %v", c.constraint)
	}
	b.Arrange(geometry.Rect(0, 0, 8, 10))
	if c.Rect() != geometry.Rect(4, 5, 0, 0) {
		t.Fatalf("tiny frame leaked content: %v", c.Rect())
	}
}

func TestBorderBoxEdgeRegions(t *testing.T) {
	b := NewBorderBox()
	b.Arrange(geometry.Rect(0, 0, 100, 80))
	tests := []struct {
		p    geometry.Point
		want ChromeRegion
	}{
		{geometry.Point{X: 2, Y: 40}, ChromeRegionLeft},
		{geometry.Point{X: 98, Y: 40}, ChromeRegionRight},
		{geometry.Point{X: 50, Y: 2}, ChromeRegionTop},
		{geometry.Point{X: 50, Y: 78}, ChromeRegionBottom},
		{geometry.Point{X: 2, Y: 12}, ChromeRegionTopLeft},
		{geometry.Point{X: 88, Y: 2}, ChromeRegionTopRight},
		{geometry.Point{X: 2, Y: 68}, ChromeRegionBottomLeft},
		{geometry.Point{X: 98, Y: 68}, ChromeRegionBottomRight},
		{geometry.Point{X: 6, Y: 6}, ChromeRegionDefault},
		{geometry.Point{X: 50, Y: 40}, ChromeRegionDefault},
		{geometry.Point{X: -1, Y: 0}, ChromeRegionDefault},
		{geometry.Point{X: 100, Y: 40}, ChromeRegionDefault},
	}
	for _, test := range tests {
		if got := b.edgeRegion(test.p); got != test.want {
			t.Errorf("%v: got %v want %v", test.p, got, test.want)
		}
	}
	b.Arrange(geometry.Rect(0, 0, 4, 4))
	if b.edgeRegion(geometry.Point{X: 3, Y: 3}) != ChromeRegionBottomRight {
		t.Fatal("small overlapping edges did not choose nearest corner")
	}
	b.Arrange(geometry.Rectangle{})
	if b.edgeRegion(geometry.Point{}) != ChromeRegionDefault {
		t.Fatal("empty box has a resize region")
	}
}

func borderWindowFixture(t *testing.T, nativeHit bool) (*window, *desktopTestWindow, *BorderBox) {
	t.Helper()
	native := &desktopTestWindow{chrome: platform.WindowChromeNone, state: WindowStateNormal}
	if !nativeHit {
		native.hitError = platform.ErrUnsupported
	}
	app := &application{platform: &chromeTestPlatform{native: native, creationNotifications: true}}
	result, err := app.NewWindow(&WindowOptions{Size: geometry.Size{Width: 100, Height: 80}, Chrome: WindowChromeNone})
	if err != nil {
		t.Fatal(err)
	}
	w := result.(*window)
	b := NewBorderBox()
	b.SetChild(newTestWidget())
	w.SetWidget(b)
	if err := w.Show(); err != nil {
		t.Fatal(err)
	}
	w.paint()
	t.Cleanup(w.Destroy)
	return w, native, b
}

func TestBorderBoxChromeAndLifecycle(t *testing.T) {
	for _, nativeHit := range []bool{true, false} {
		w, native, b := borderWindowFixture(t, nativeHit)
		point := geometry.Point{X: 98, Y: 40}
		if got := w.chrome.queryRegion(point); got != ChromeRegionRight {
			t.Fatalf("mounted before root assignment lost edge: %v", got)
		}
		_ = w.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point})
		if b.Cursor() != CursorResizeRight {
			t.Fatal("resize cursor missing")
		}
		b.SetCursor(CursorCrosshair)
		if b.Cursor() != CursorCrosshair {
			t.Fatal("explicit cursor override was ignored")
		}
		b.SetCursor(CursorDefault)
		w.dispatcher.captureTarget = b.Child()
		if b.Cursor() != CursorDefault {
			t.Fatal("capture kept a resize cursor")
		}
		w.dispatcher.captureTarget = nil
		b.SetVisible(false)
		if w.chrome.queryRegion(point) != ChromeRegionDefault || b.Cursor() != CursorDefault {
			t.Fatal("hidden box resized")
		}
		b.SetVisible(true)
		w.paint()
		_ = w.DispatchEvent(events.PointerEvent{EventType: events.PointerLeave, Position: point})
		if b.Cursor() != CursorDefault {
			t.Fatal("leaving window kept a resize cursor")
		}
		_ = w.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point})
		override := w.Chrome().ConnectQueryRegion(func(_ geometry.Point, r *ChromeRegion) { *r = ChromeRegionClient })
		if b.Cursor() != CursorDefault {
			t.Fatal("cursor ignored final query override")
		}
		override.Disconnect()
		b.SetEnabled(false)
		if w.chrome.queryRegion(point) != ChromeRegionDefault || b.Cursor() != CursorDefault {
			t.Fatal("disabled box resized")
		}
		b.SetEnabled(true)
		b.SetResizable(false)
		if w.chrome.queryRegion(point) != ChromeRegionDefault {
			t.Fatal("nonresizable box resized")
		}
		b.SetResizable(true)
		for _, state := range []WindowState{WindowStateMaximized, WindowStateFullscreen} {
			native.state = state
			if w.chrome.queryRegion(point) != ChromeRegionDefault || b.Cursor() != CursorDefault {
				t.Fatal("non-normal window resized")
			}
		}
		native.state = WindowStateNormal
		w.paint()
		_ = w.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: point, Button: events.PointerButtonLeft, Buttons: events.PointerButtonLeftDown})
		_ = w.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Position: point, Button: events.PointerButtonLeft})
		if nativeHit {
			if len(native.resizeRequests) != 0 || native.hitTest(point) != platform.WindowHitRight {
				t.Fatal("duplicate native resize path")
			}
		} else if len(native.resizeRequests) != 1 || native.resizeRequests[0] != platform.WindowEdgeRight || w.dispatcher.gesture != nil {
			t.Fatal("resize did not hand off through host gesture")
		}
		w.SetWidget(nil)
		if b.connection != nil || w.chrome.queryRegion(point) != ChromeRegionDefault {
			t.Fatal("unmount retained chrome connection")
		}
		w.SetWidget(b)
		w.paint()
		if w.chrome.queryRegion(point) != ChromeRegionRight {
			t.Fatal("remount did not restore resize")
		}
		w.SetWidget(nil)
		parent := NewLinearBox(layout.DirectionVertical)
		parent.AddChild(b)
		w.SetWidget(parent)
		w.paint()
		if w.chrome.queryRegion(point) != ChromeRegionDefault {
			t.Fatal("nested box resized host")
		}
	}
	for _, mode := range []WindowChromeMode{WindowChromeNative, WindowChromeIntegrated} {
		w, native := chromeFixture(t, false, true)
		w.chrome.info.Mode, native.chrome = mode, platform.WindowChrome(mode)
		b := NewBorderBox()
		w.SetWidget(b)
		w.paint()
		if w.chrome.queryRegion(geometry.Point{X: 1, Y: 100}) != ChromeRegionDefault {
			t.Fatal("non-None box resized host")
		}
	}
	// Popover is a different Root, not an ordinary Window. Mounting the box
	// must not find or subscribe to the popover's owner-window chrome.
	p := &popover{}
	b := NewBorderBox()
	p.SetWidget(b)
	if b.Root() != p || b.Window() != nil || b.connection != nil || b.resizeWindow() != nil {
		t.Fatal("popover decoration acquired ordinary-window interaction")
	}
	p.SetWidget(nil)
}

func TestBorderBoxStyledSoftwareClip(t *testing.T) {
	// Premultiplied RGBA, exact opaque/transparent interior samples; do not
	// assert a backend-specific AA curve on the rounded contour.
	useTestApplication(t, &application{style: style.Sheet(append(DefaultStyleRules(),
		style.Name("frame").BackgroundColor(color.White).BorderColor(color.Black).BorderWidth(1).Radius(24),
	)...)})
	for _, scale := range []float32{1, 2} {
		out := &transparentFrame{}
		p, err := software.NewPainter(out)
		if err != nil {
			t.Fatal(err)
		}
		b := NewBorderBox()
		b.SetStyleName("frame")
		child := newPainterTestWidget(func(p Painter) {
			p.SetClipRect(geometry.Rect(-100, -100, 400, 400))
			p.SetTransform(geometry.Translate(-20, -20))
			p.FillRect(geometry.Rect(0, 0, 200, 200), graphics.ColorOf(color.RGBA{R: 255, A: 255}))
		})
		b.SetChild(child)
		b.Arrange(geometry.Rect(0, 0, 80, 60))
		if b.contentInset() < 8 {
			t.Fatal("rounded safety inset not applied")
		}
		p.Begin(80*scale, 60*scale, scale)
		p.Clear(Color{})
		paintWidget(b, newPainter(p, geometry.Rect(0, 0, 80, 60), scale))
		p.End()
		for _, sample := range []struct {
			x, y int
			want color.RGBA
		}{
			{0, 0, color.RGBA{}}, {40, 4, color.RGBA{255, 255, 255, 255}}, {40, 30, color.RGBA{255, 0, 0, 255}},
		} {
			r, g, bl, a := out.image.At(sample.x*int(scale), sample.y*int(scale)).RGBA()
			got := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), uint8(a >> 8)}
			if got != sample.want {
				t.Fatalf("scale=%v pixel=(%d,%d) got %v want %v", scale, sample.x, sample.y, got, sample.want)
			}
		}
		p.Destroy()
	}
}
