package widgets

import (
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

func TestProgressBarParameters(t *testing.T) {
	p := NewProgressBar()
	if p.Value() != 0 || p.Shape() != ProgressLinear || p.Indeterminate() || p.Thickness() != 4 || p.Focusable() || len(p.EventControllers()) != 0 {
		t.Fatal("unexpected constructor defaults")
	}
	for _, tc := range []struct{ value, want float32 }{
		{-.5, 0}, {0, 0}, {.25, .25}, {1, 1}, {2, 1},
		{float32(math.NaN()), 0}, {float32(math.Inf(1)), 0}, {float32(math.Inf(-1)), 0},
	} {
		p.SetValue(tc.value)
		if p.Value() != tc.want {
			t.Fatalf("value %g became %g, want %g", tc.value, p.Value(), tc.want)
		}
	}
	p.SetIndeterminate(true)
	p.SetValue(.75)
	if !p.Indeterminate() {
		t.Fatal("SetValue changed mode")
	}
	p.SetIndeterminate(false)
	if p.Value() != .75 {
		t.Fatal("mode change lost value")
	}
	p.SetShape(ProgressCircular)
	p.SetShape(255)
	if p.Shape() != ProgressLinear {
		t.Fatal("invalid shape not normalized")
	}
	for _, value := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
		p.SetThickness(value)
		if p.Thickness() != 0 {
			t.Fatal("invalid thickness not normalized")
		}
	}
}

func TestProgressBarMeasurement(t *testing.T) {
	p := NewProgressBar()
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.AddChild(p)
	check := func(c layout.Constraint, want geometry.Size) {
		t.Helper()
		m := p.Measure(c)
		if m.Size != want || m.HasBaseline {
			t.Fatalf("measurement %+v, want %+v without baseline", m, want)
		}
	}
	check(layout.Unbounded(), geometry.Size{Width: 160, Height: 4})
	if box.Measure(layout.Unbounded()).Size != (geometry.Size{Width: 160, Height: 4}) {
		t.Fatal("parent did not measure the indicator")
	}
	p.SetThickness(8)
	check(layout.Unbounded(), geometry.Size{Width: 160, Height: 8})
	if box.Measure(layout.Unbounded()).Size != (geometry.Size{Width: 160, Height: 8}) {
		t.Fatal("thickness change did not invalidate the parent's child measurement cache")
	}
	p.SetValue(1)
	p.SetIndeterminate(true)
	check(layout.Unbounded(), geometry.Size{Width: 160, Height: 8})
	p.SetShape(ProgressCircular)
	check(layout.Unbounded(), geometry.Size{Width: 24, Height: 24})
	p.SetMinSize(geometry.Size{Width: 30})
	p.SetMaxSize(geometry.Size{Height: 16})
	check(layout.Unbounded(), geometry.Size{Width: 30, Height: 16})
	check(layout.Tight(geometry.Size{Width: 8, Height: 40}), geometry.Size{Width: 8, Height: 40})
	p.SetVisible(false)
	check(layout.Unbounded(), geometry.Size{})
}

func TestProgressBarSnapshot(t *testing.T) {
	p := NewProgressBar()
	p.SetID("download")
	p.SetValue(.25)
	p.SetShape(ProgressCircular)
	info := p.Snapshot()
	progress := info.Attributes[ProgressInfoKey].(ProgressInfo)
	if info.ID != "download" || info.Role != RoleProgressBar || info.Range != nil || len(info.Actions) != 0 ||
		progress.Shape != ProgressCircular || progress.Indeterminate || progress.Value == nil || *progress.Value != .25 {
		t.Fatalf("unexpected snapshot: %+v / %+v", info, progress)
	}
	*progress.Value = .8
	if p.Value() != .25 {
		t.Fatal("snapshot value aliases widget state")
	}
	p.SetIndeterminate(true)
	info = p.Snapshot()
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Attributes map[string]map[string]any `json:"attributes"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, present := decoded.Attributes[ProgressInfoKey]["value"]; present || decoded.Attributes[ProgressInfoKey]["indeterminate"] != true {
		t.Fatalf("indeterminate snapshot reports a fake value: %s", data)
	}
}

// Public GUI doubles: no native windows, scheduler internals or sleeping.
// Real timer activation/cancellation and destruction are asserted by the window
// examples; these tests exercise guards, failed starts and stale tick handling.
type progressTestApplication struct {
	gui.Application
	sheet  style.StyleSheet
	timers []*gui.Timer
}

func (a *progressTestApplication) StyleSheet() style.StyleSheet { return a.sheet }
func (a *progressTestApplication) NewTimer() *gui.Timer {
	timer := new(gui.Timer) // Public zero Timer deterministically rejects Start.
	a.timers = append(a.timers, timer)
	return timer
}

func useProgressApplication(t *testing.T, app *progressTestApplication) {
	t.Helper()
	old := gui.App
	gui.App = app
	t.Cleanup(func() { gui.App = old })
}

type progressTestRoot struct {
	widget          gui.Widget
	paints, layouts int
	visible         bool
}

func (r *progressTestRoot) Widget() gui.Widget  { return r.widget }
func (r *progressTestRoot) RequestPaint() error { r.paints++; return nil }
func (r *progressTestRoot) RequestLayout()      { r.layouts++ }
func (r *progressTestRoot) Visible() bool       { return r.visible }

type progressTestHost struct {
	gui.WidgetBase
	root *progressTestRoot
}

func (h *progressTestHost) Root() gui.Root { return h.root }
func newProgressTestHost() *progressTestHost {
	h := new(progressTestHost)
	h.root = &progressTestRoot{widget: h, visible: true}
	return h
}

func TestProgressBarAnimationLifecycle(t *testing.T) {
	app := new(progressTestApplication)
	useProgressApplication(t, app)
	host := newProgressTestHost()
	box := gui.NewLinearBox(layout.DirectionVertical)
	p := NewProgressBar()
	box.AddChild(p)
	host.AddChild(host, box)
	t.Cleanup(func() { host.RemoveChild(box) })
	p.Arrange(geometry.Rect(0, 0, 100, 20))
	if !p.mounted || len(app.timers) != 0 {
		t.Fatal("determinate widget allocated a timer or was not mounted")
	}
	p.SetIndeterminate(true)
	if p.timer == nil || len(app.timers) != 1 || !p.canAnimate() {
		t.Fatal("activity did not create one timer")
	}
	timer, started := p.timer, p.started
	p.SetIndeterminate(true)
	p.SetValue(.7)
	p.SetShape(ProgressCircular)
	p.Arrange(p.Rect())
	if p.timer != timer || len(app.timers) != 1 || p.started != started {
		t.Fatal("repeated declarations or shape change reset the clock")
	}
	host.root.paints, host.root.layouts = 0, 0
	p.tick()
	if host.root.paints != 1 || host.root.layouts != 0 || p.Value() != .7 || p.started != started {
		t.Fatal("tick changed layout, progress or clock")
	}
	box.SetVisible(false)
	host.root.paints = 0
	p.tick()
	if p.timer != nil || p.canAnimate() || host.root.paints != 0 {
		t.Fatal("hidden ancestor retained activity or repainted")
	}
	box.SetVisible(true)
	p.Arrange(p.Rect())
	if p.timer == nil || len(app.timers) != 2 {
		t.Fatal("layout did not restore activity after revealing subtree")
	}
	p.SetVisible(false)
	if p.timer != nil {
		t.Fatal("self-hide retained a timer")
	}
	p.SetVisible(true)
	p.SetThickness(0)
	if p.timer != nil {
		t.Fatal("zero thickness retained a timer")
	}
	p.SetThickness(4)
	p.Arrange(geometry.Rectangle{})
	if p.timer != nil {
		t.Fatal("zero allocation retained a timer")
	}
	p.Arrange(geometry.Rect(0, 0, 24, 24))
	// An external unmount listener must not resurrect activity while Root is
	// still available during the unmount signal.
	p.ConnectUnmount(func() {
		p.SetIndeterminate(false)
		p.SetIndeterminate(true)
	})
	box.RemoveChild(p)
	count := len(app.timers)
	p.tick() // A stale callback must be harmless after unmount.
	if p.timer != nil || p.mounted || p.canAnimate() || len(app.timers) != count {
		t.Fatal("unmount or stale tick resurrected activity")
	}
	other := newProgressTestHost()
	other.AddChild(other, p)
	t.Cleanup(func() { other.RemoveChild(p) })
	p.Arrange(p.Rect())
	if p.Root() != other.root || p.timer == nil || len(app.timers) != count+1 {
		t.Fatal("remount did not recreate activity for the new host")
	}
	p.SetIndeterminate(false)
	if p.Value() != .7 || p.timer != nil {
		t.Fatal("leaving activity lost progress or retained timer")
	}
}

func TestProgressBarAnimationFailureAndPopover(t *testing.T) {
	app := new(progressTestApplication)
	useProgressApplication(t, app)
	host := newProgressTestHost()
	p := NewProgressBar()
	host.AddChild(host, p)
	t.Cleanup(func() { host.RemoveChild(p) })
	p.SetIndeterminate(true)
	p.Arrange(geometry.Rect(0, 0, 100, 4))
	timer := p.timer
	host.root.paints = 0
	p.syncAnimation()
	if timer == nil || timer.Active() || p.timer != timer || len(app.timers) != 1 || !p.started.IsZero() || host.root.paints != 0 {
		t.Fatal("failed start retried or kept a moving phase")
	}
	host.root.visible = false
	p.tick()
	if p.timer != nil {
		t.Fatal("hidden host retained activity")
	}
	host.root.visible = true
	p.syncAnimation()
	if p.timer == nil || len(app.timers) != 2 {
		t.Fatal("revealed host did not retry")
	}
	popup := gui.NewPopover(nil, nil)
	popup.SetWidget(p)
	t.Cleanup(func() { popup.SetWidget(nil) })
	p.Arrange(p.Rect())
	p.tick()
	if p.timer != nil || !p.mounted || p.canAnimate() || len(app.timers) != 2 {
		t.Fatal("hidden popover retained activity")
	}
}

func TestProgressBarMotion(t *testing.T) {
	for _, tc := range []struct {
		elapsed time.Duration
		want    float32
	}{{0, -.3}, {450 * time.Millisecond, .025}, {900 * time.Millisecond, .35}, {1350 * time.Millisecond, .675}, {1800 * time.Millisecond, -.3}} {
		position, length := progressLinearMotion(tc.elapsed)
		if math.Abs(float64(position-tc.want)) > 1e-6 || length != .3 {
			t.Fatalf("linear motion at %s = %g/%g", tc.elapsed, position, length)
		}
	}
	previous := float32(-1)
	for ms := 0; ms < 1800; ms++ {
		position, length := progressLinearMotion(time.Duration(ms) * time.Millisecond)
		if position < previous || position < -.300001 || position > 1 || length != .3 {
			t.Fatal("busy segment reversed, changed length or escaped its sweep")
		}
		previous = position
	}
	// At both sides of the wrap, at most an infinitesimal clipped segment is
	// visible, not a full-size pill jumping back across the track.
	for _, elapsed := range []time.Duration{0, 1800*time.Millisecond - time.Nanosecond, 1800 * time.Millisecond, 1800*time.Millisecond + time.Nanosecond} {
		position, length := progressLinearMotion(elapsed)
		visible := max(0, min(1, position+length)-max(0, position))
		if visible > 1e-6 {
			t.Fatalf("visible jump at wrap: %s / %g", elapsed, visible)
		}
	}
	// A fixed quarter-circle rotates clockwise once every 1.4 seconds.
	for _, tc := range []struct {
		elapsed time.Duration
		start   float64
	}{
		{-time.Second, -math.Pi / 2}, {0, -math.Pi / 2},
		{350 * time.Millisecond, 0}, {700 * time.Millisecond, math.Pi / 2},
		{1050 * time.Millisecond, math.Pi}, {1400 * time.Millisecond, -math.Pi / 2},
		{1750 * time.Millisecond, 0}, {2800 * time.Millisecond, -math.Pi / 2},
	} {
		start, sweep := progressCircularMotion(tc.elapsed)
		if math.Abs(start-tc.start) > 1e-12 || sweep != math.Pi/2 {
			t.Fatalf("circular motion at %s = %g/%g, want %g/%g", tc.elapsed, start, sweep, tc.start, math.Pi/2)
		}
	}
	previousStart, _ := progressCircularMotion(0)
	for ms := 1; ms < 1400; ms++ {
		start, sweep := progressCircularMotion(time.Duration(ms) * time.Millisecond)
		if math.Abs(start-previousStart-2*math.Pi/1400) > 1e-12 || sweep != math.Pi/2 {
			t.Fatalf("circular motion stretched, reversed or changed speed at %dms", ms)
		}
		previousStart = start
	}
	start, sweep := progressCircularMotion(1400 * time.Millisecond)
	beforeStart, beforeSweep := progressCircularMotion(1400*time.Millisecond - time.Nanosecond)
	// Compare rotation modulo a full turn, not raw angle at the wrap.
	if math.Hypot(math.Cos(start)-math.Cos(beforeStart), math.Sin(start)-math.Sin(beforeStart)) > 1e-7 || math.Abs(sweep-beforeSweep) > 1e-7 {
		t.Fatal("circular loop has a visual discontinuity")
	}
}

func TestProgressBarCircularRotationPixels(t *testing.T) {
	useProgressApplication(t, &progressTestApplication{sheet: style.Sheet(style.Name("progress-bar").
		BackgroundColor(color.Transparent).ForegroundColor(color.NRGBA{R: 255, A: 128}))})
	p := NewProgressBar()
	p.SetShape(ProgressCircular)
	p.SetIndeterminate(true)
	p.Arrange(geometry.Rect(2, 2, 24, 24)) // Center (14,14); ring radii 8..12 DIP.
	// Sample mid-arc, away from caps and AA edges, in clockwise quadrant order.
	// Output is premultiplied sRGB RGBA8: NRGBA(255,0,0,128) is (128,0,0,128),
	// allowing one byte of coverage/compositing rounding. Other quadrants and
	// the center must remain transparent, not expand into a longer arc or disk.
	samples := []image.Point{{21, 7}, {21, 21}, {7, 21}, {7, 7}}
	for _, scale := range []float32{1, 2} {
		for _, tc := range []struct {
			elapsed  time.Duration
			quadrant int
		}{
			{0, 0}, {350 * time.Millisecond, 1}, {700 * time.Millisecond, 2},
			{1050 * time.Millisecond, 3}, {1400 * time.Millisecond, 0},
			{1750 * time.Millisecond, 1}, {2800 * time.Millisecond, 0},
		} {
			w := &progressPaintWidget{paint: func(painter gui.Painter) { p.paint(painter, tc.elapsed) }}
			w.Arrange(p.Rect())
			img := renderProgressTest(t, w, scale)
			for quadrant, point := range samples {
				got := color.RGBAModel.Convert(img.At(point.X*int(scale), point.Y*int(scale))).(color.RGBA)
				want := color.RGBA{}
				if quadrant == tc.quadrant {
					want = color.RGBA{R: 128, A: 128}
				}
				if math.Abs(float64(got.R)-float64(want.R)) > 1 || math.Abs(float64(got.A)-float64(want.A)) > 1 || got.G != 0 || got.B != 0 {
					t.Fatalf("rotation at %s / %gx, quadrant %d = %v, want %v", tc.elapsed, scale, quadrant, got, want)
				}
			}
			if _, _, _, a := img.At(14*int(scale), 14*int(scale)).RGBA(); a != 0 {
				t.Fatal("circular activity filled its center")
			}
			for y := 0; y < img.Bounds().Dy(); y++ {
				for x := 0; x < img.Bounds().Dx(); x++ {
					got := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
					if got.A > 129 || got.R > got.A || got.G != 0 || got.B != 0 {
						t.Fatalf("rotating arc composited twice at %s / %gx (%d,%d) = %v", tc.elapsed, scale, x, y, got)
					}
				}
			}
		}
	}
}

// This narrow adapter tests the control's Paint against Software. GUI's
// traversal / state isolation have their own tests; no copy of traversal here.
type progressPixelPainter struct {
	gui.Painter
	native graphics.Painter
}

type progressPixelSurface struct{ pixels *image.RGBA }

func (*progressPixelSurface) Transparent() bool { return true }
func (s *progressPixelSurface) Draw(img image.Image) error {
	s.pixels = image.NewRGBA(img.Bounds())
	draw.Draw(s.pixels, s.pixels.Bounds(), img, img.Bounds().Min, draw.Src)
	return nil
}

func (p *progressPixelPainter) FillRoundRect(rect geometry.Rectangle, radius float32, brush graphics.Brush) {
	p.native.FillRoundRect(rect, radius, brush)
}
func (p *progressPixelPainter) FillPath(path graphics.Path, brush graphics.Brush) {
	p.native.FillPath(path, brush)
}
func (p *progressPixelPainter) SetClipRect(rect geometry.Rectangle) { p.native.SetClipRect(rect) }
func (p *progressPixelPainter) SetTransform(transform geometry.Transform) {
	p.native.SetTransform(transform)
}

type progressPaintWidget struct {
	gui.WidgetBase
	paint func(gui.Painter)
}

func (w *progressPaintWidget) Paint(painter gui.Painter) { w.paint(painter) }

func renderProgressTest(t *testing.T, widget gui.Widget, scale float32) image.Image {
	t.Helper()
	out := new(progressPixelSurface)
	backend, err := software.NewPainter(out)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	backend.Begin(80*scale, 60*scale, scale)
	backend.Clear(graphics.Color{})
	backend.SetClipRect(widget.Rect().Intersect(geometry.Rect(0, 0, 80, 60)))
	backend.SetTransform(geometry.Translate(widget.Rect().X, widget.Rect().Y))
	widget.Paint(&progressPixelPainter{native: backend})
	backend.End()
	return out.pixels
}

func TestProgressBarSoftwarePixels(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	useProgressApplication(t, &progressTestApplication{sheet: style.Sheet(style.Name("progress-bar").
		BackgroundColor(blue).ForegroundColor(red).Radius(2))})
	for _, scale := range []float32{1, 2} {
		pixel := func(img image.Image, x, y int) color.RGBA {
			return color.RGBAModel.Convert(img.At(int(float32(x)*scale), int(float32(y)*scale))).(color.RGBA)
		}
		p := NewProgressBar()
		p.Arrange(geometry.Rect(2, 2, 60, 20))
		for _, value := range []float32{0, .25, 1} {
			p.SetValue(value)
			img := renderProgressTest(t, p, scale)
			want := blue
			if value > 0 {
				want = red
			}
			if pixel(img, 6, 11) != want || pixel(img, 6, 5).A != 0 {
				t.Fatalf("linear centered fill at %g / %gx", value, scale)
			}
			want = blue
			if value == 1 {
				want = red
			}
			if pixel(img, 40, 11) != want {
				t.Fatal("linear proportional fill is wrong")
			}
		}
		p.SetShape(ProgressCircular)
		p.Arrange(geometry.Rect(2, 2, 40, 24)) // Center at (22,14), radius 12.
		for _, value := range []float32{0, .25, .5, 1} {
			p.SetValue(value)
			img := renderProgressTest(t, p, scale)
			if pixel(img, 22, 14).A != 0 || pixel(img, 3, 3).A != 0 {
				t.Fatal("circle was filled in or stretched to allocation")
			}
			for _, sample := range []struct {
				x, y int
				red  bool
			}{{29, 7, value >= .25}, {29, 21, value >= .5}, {15, 21, value == 1}, {15, 7, value == 1}} {
				want := blue
				if sample.red {
					want = red
				}
				if got := pixel(img, sample.x, sample.y); got != want {
					t.Fatalf("circle %g / %gx pixel (%d,%d) = %v, want %v", value, scale, sample.x, sample.y, got, want)
				}
			}
		}
		p.SetIndeterminate(true) // Unmounted: deterministic static activity pose.
		img := renderProgressTest(t, p, scale)
		if pixel(img, 25, 4) != red || pixel(img, 15, 21) != blue {
			t.Fatal("circular activity is not a partial arc")
		}
		p.SetShape(ProgressLinear)
		p.Arrange(geometry.Rect(2, 2, 60, 20))
		img = renderProgressTest(t, p, scale)
		if pixel(img, 6, 11) != red || pixel(img, 40, 11) != blue {
			t.Fatal("linear activity is not a partial segment")
		}
		p.SetThickness(0)
		img = renderProgressTest(t, p, scale)
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
					t.Fatal("zero thickness still paints")
				}
			}
		}
		// Excess thickness is limited to the tiny allocation. With a zero
		// inner radius, a completed circular indicator becomes a solid disk.
		p.SetIndeterminate(false)
		p.SetShape(ProgressCircular)
		p.SetThickness(100)
		p.SetValue(1)
		p.Arrange(geometry.Rect(2, 2, 4, 4))
		img = renderProgressTest(t, p, scale)
		if pixel(img, 4, 4) != red || pixel(img, 7, 4).A != 0 {
			t.Fatal("tiny circular indicator did not clamp its thickness / bounds")
		}
	}
}

func TestProgressBarSoftwareAlphaAndClip(t *testing.T) {
	// Software output is premultiplied sRGB RGBA8. Away from AA boundaries,
	// foreground NRGBA(255,0,0,128) must be RGBA(128,0,0,128), +/-1 for rounding.
	useProgressApplication(t, &progressTestApplication{sheet: style.Sheet(style.Name("progress-bar").
		BackgroundColor(color.Transparent).ForegroundColor(color.NRGBA{R: 255, A: 128}).Radius(2))})
	for _, scale := range []float32{1, 2} {
		p := NewProgressBar()
		p.SetShape(ProgressCircular)
		p.Arrange(geometry.Rect(2, 2, 24, 24))
		for _, value := range []float32{.001, .25, .5, .94, .99, 1} {
			p.SetValue(value)
			img := renderProgressTest(t, p, scale)
			painted := false
			for y := 0; y < img.Bounds().Dy(); y++ {
				for x := 0; x < img.Bounds().Dx(); x++ {
					c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
					if c.A > 129 || c.R > c.A || c.G != 0 || c.B != 0 {
						t.Fatalf("round cap / ring composited twice: %g / %gx (%d,%d)=%v", value, scale, x, y, c)
					}
					painted = painted || c.A >= 127
				}
			}
			if !painted {
				t.Fatal("arc did not paint a fully covered pixel")
			}
			if value == 1 {
				for _, pt := range []image.Point{{14, 4}, {24, 14}, {14, 24}, {4, 14}} {
					c := color.RGBAModel.Convert(img.At(pt.X*int(scale), pt.Y*int(scale))).(color.RGBA)
					if c.A < 127 {
						t.Fatalf("full ring has a seam at %v: %v", pt, c)
					}
				}
			}
		}
		// Draw under a local 90-degree transform and a layout-space clip. A
		// wrapper's Paint delegates only its own content; no child traversal.
		p.SetValue(1)
		p.Arrange(geometry.Rect(0, 0, 40, 8))
		p.SetShape(ProgressLinear)
		parent := &progressPaintWidget{paint: func(painter gui.Painter) {
			painter.SetClipRect(geometry.Rect(0, 0, 20, 30))
			painter.SetTransform(geometry.Transform{A12: -1, A21: 2, TX: 8, TY: 8})
			p.Paint(painter)
		}}
		parent.Arrange(geometry.Rect(0, 0, 60, 40))
		img := renderProgressTest(t, parent, scale)
		if _, _, _, a := img.At(4*int(scale), 12*int(scale)).RGBA(); a == 0 {
			t.Fatal("transformed bar did not paint at its independent expected position")
		}
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				if _, _, _, a := img.At(x, y).RGBA(); a != 0 && (x >= 20*int(scale) || y >= 30*int(scale)) {
					t.Fatal("progress escaped the local rectangular clip")
				}
			}
		}
	}
}

func TestProgressBarLinearSweepPixels(t *testing.T) {
	useProgressApplication(t, &progressTestApplication{sheet: style.Sheet(style.Name("progress-bar").
		BackgroundColor(color.Transparent).ForegroundColor(color.NRGBA{R: 255, A: 128}).Radius(2))})
	p := NewProgressBar()
	p.SetIndeterminate(true)
	p.Arrange(geometry.Rect(2, 2, 60, 20))
	for _, scale := range []float32{1, 2} {
		for _, tc := range []struct {
			elapsed         time.Duration
			inside, outside int
		}{
			{0, -1, 30},
			{150 * time.Millisecond, 3, 10},
			{900 * time.Millisecond, 25, 20},
			{1650 * time.Millisecond, 58, 54},
			{1800*time.Millisecond - time.Nanosecond, -1, 30},
			{1800 * time.Millisecond, -1, 30},
		} {
			w := &progressPaintWidget{paint: func(painter gui.Painter) { p.paint(painter, tc.elapsed) }}
			w.Arrange(p.Rect())
			img := renderProgressTest(t, w, scale)
			if tc.inside >= 0 {
				got := color.RGBAModel.Convert(img.At(tc.inside*int(scale), 11*int(scale))).(color.RGBA)
				if got != (color.RGBA{R: 128, A: 128}) {
					t.Fatalf("sweep at %s / %gx: inside=%v", tc.elapsed, scale, got)
				}
			}
			if _, _, _, a := img.At(tc.outside*int(scale), 11*int(scale)).RGBA(); a != 0 {
				t.Fatalf("sweep at %s / %gx painted outside its expected segment", tc.elapsed, scale)
			}
			for y := 0; y < img.Bounds().Dy(); y++ {
				for x := 0; x < img.Bounds().Dx(); x++ {
					c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
					if c.A > 129 || c.R > c.A || c.A != 0 && (x < 2*int(scale) || x >= 62*int(scale)) || tc.inside < 0 && c.A != 0 {
						t.Fatalf("sweep escaped clip / composited twice / jumped at wrap: %s / %gx (%d,%d)=%v", tc.elapsed, scale, x, y, c)
					}
				}
			}
		}
	}
}
