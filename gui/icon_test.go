package gui

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

func TestIconMeasurement(t *testing.T) {
	i := NewIcon(func(Painter, geometry.Rectangle, Color) {})
	check := func(c layout.Constraint, want geometry.Size) {
		t.Helper()
		if got := measureWidget(i, c); got.Size != want || got.HasBaseline {
			t.Fatalf("measurement = %+v, want size %+v without baseline", got, want)
		}
	}
	check(layout.Unbounded(), geometry.Size{Width: 16, Height: 16})
	i.SetSize(24)
	check(layout.Unbounded(), geometry.Size{Width: 24, Height: 24})
	i.SetMinSize(geometry.Size{Width: 30, Height: 12})
	i.SetMaxSize(geometry.Size{Width: 40, Height: 20})
	check(layout.Unbounded(), geometry.Size{Width: 30, Height: 20})
	check(layout.Tight(geometry.Size{Width: 8, Height: 50}), geometry.Size{Width: 8, Height: 50})
	i.SetMinSize(geometry.Size{})
	for _, size := range []float32{-1, float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
		i.SetSize(size)
		if i.Size() != 0 {
			t.Fatalf("invalid size not normalized: %v", i.Size())
		}
		check(layout.Unbounded(), geometry.Size{})
	}
	i.SetSize(16)
	i.SetDrawFunc(nil)
	check(layout.Unbounded(), geometry.Size{})
	i.SetDrawFunc(func(Painter, geometry.Rectangle, Color) {})
	check(layout.Unbounded(), geometry.Size{Width: 16, Height: 16})
	i.SetVisible(false)
	check(layout.Unbounded(), geometry.Size{})
}

func TestIconPaintBoundsAndReplacement(t *testing.T) {
	var got geometry.Rectangle
	calls, value := 0, 0
	draw := func(v int) IconDrawFunc {
		return func(_ Painter, rect geometry.Rectangle, _ Color) { calls++; value = v; got = rect }
	}
	i := NewIcon(draw(1))
	i.Arrange(geometry.Rect(100, 200, 40, 20))
	i.Paint(nil)
	if got != geometry.Rect(10, 0, 20, 20) || value != 1 {
		t.Fatalf("local centered rect: %+v, value %d", got, value)
	}
	i.SetDrawFunc(draw(2))
	i.Arrange(geometry.Rect(100, 200, 20, 40))
	i.Paint(nil)
	if got != geometry.Rect(0, 10, 20, 20) || value != 2 {
		t.Fatalf("replacement: %+v, value %d", got, value)
	}
	i.SetVisible(false)
	i.Paint(nil)
	i.SetVisible(true)
	i.Arrange(geometry.Rectangle{})
	i.Paint(nil)
	i.SetDrawFunc(nil)
	i.Arrange(geometry.Rect(0, 0, 20, 20))
	i.Paint(nil)
	if calls != 2 {
		t.Fatalf("hidden/empty/nil content was painted: %d", calls)
	}
	i.SetID("search-icon")
	info := i.Snapshot()
	if info.Role != RoleImage || info.ID != "search-icon" || i.Focusable() || len(info.Actions) != 0 {
		t.Fatalf("unexpected icon semantics: %+v", info)
	}
}

func TestIconForegroundAndStyleLifecycle(t *testing.T) {
	app := &application{}
	useTestApplication(t, app)
	win := &window{}
	app.windows = []*window{win}
	var got Color
	calls := 0
	i := NewIcon(func(_ Painter, _ geometry.Rectangle, c Color) { got = c; calls++ })
	win.SetWidget(i)
	defer win.SetWidget(nil)
	i.Arrange(geometry.Rect(0, 0, 20, 20))
	paintStyleTestWidget(i)
	if got != graphics.ColorOf(color.Black) {
		t.Fatalf("fallback color: %+v", got)
	}
	for _, tc := range []struct {
		name string
		rule style.Rule
		want Color
	}{
		{"missing", style.Name("other").ForegroundColor(color.White), Color{}},
		{"nil", style.Name("icon").ForegroundColor(nil), Color{}},
		{"transparent", style.Name("icon").ForegroundColor(color.Transparent), Color{}},
		{"alpha", style.Name("icon").ForegroundColor(color.NRGBA{R: 255, A: 128}), Color{R: 128.0 / 255, A: 128.0 / 255}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app.SetStyleSheet(style.Sheet(tc.rule))
			before := calls
			paintStyleTestWidget(i)
			if got != tc.want || calls != before+1 {
				t.Fatalf("color %+v, want %+v; calls %d", got, tc.want, calls)
			}
		})
	}
	red, blue := color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}
	app.SetStyleSheet(style.Sheet(style.Name("icon").ForegroundColor(red), style.Name("alternate").ForegroundColor(blue)))
	i.SetStyleName("alternate")
	paintStyleTestWidget(i)
	if got != graphics.ColorOf(blue) {
		t.Fatal("style-name change retained foreground")
	}
	i.SetVisible(false)
	app.SetStyleSheet(style.Sheet(style.Name("alternate").ForegroundColor(red)))
	i.SetVisible(true)
	paintStyleTestWidget(i)
	if got != graphics.ColorOf(red) {
		t.Fatal("reshow retained foreground")
	}
	win.SetWidget(nil)
	app.SetStyleSheet(style.Sheet(style.Name("alternate").ForegroundColor(blue)))
	win.SetWidget(i)
	paintStyleTestWidget(i)
	if got != graphics.ColorOf(blue) {
		t.Fatal("remount retained foreground")
	}
}

func TestIconParentStyleIsolation(t *testing.T) {
	red, blue := color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}
	useTestApplication(t, &application{style: style.Sheet(
		style.Name("icon").ForegroundColor(red), style.Name("alternate").ForegroundColor(blue),
		style.Name("parent").ForegroundColor(color.White),
		style.Name("parent").State(style.Hovered).ForegroundColor(color.Black),
	)})
	var got []Color
	draw := func(_ Painter, _ geometry.Rectangle, c Color) { got = append(got, c) }
	i, other := NewIcon(draw), NewIcon(draw)
	button := NewButton()
	button.SetStyleName("parent")
	button.SetChild(i)
	other.SetStyleName("alternate")
	for _, icon := range []*Icon{i, other} {
		icon.Arrange(geometry.Rect(0, 0, 16, 16))
		icon.Paint(nil)
	}
	if len(got) != 2 || got[0] != graphics.ColorOf(red) || got[1] != graphics.ColorOf(blue) {
		t.Fatalf("shared draw or parent leaked color: %+v", got)
	}
}

type iconNoImageBackend struct {
	graphics.Painter
	images int
}

func (p *iconNoImageBackend) NewImage(image.Image) (graphics.Image, error) {
	p.images++
	panic("Icon allocated an image")
}

func TestIconSoftwarePixels(t *testing.T) {
	useTestApplication(t, &application{style: style.Sheet(style.Name("icon").ForegroundColor(color.RGBA{R: 255, A: 255}))})
	for _, scale := range []float32{1, 2} {
		out := &transparentFrame{}
		backend, err := software.NewPainter(out)
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer backend.Destroy()
			spy := &iconNoImageBackend{Painter: backend}
			p := newPainter(spy, geometry.Rect(0, 0, 40, 20))
			i := NewIcon(func(p Painter, rect geometry.Rectangle, c Color) {
				p.Save()
				defer p.Restore()
				p.SetClipRect(geometry.Rect(0, 0, 8, 16))
				p.SetTransform(geometry.Translate(2, 0))
				p.FillRect(rect, c)
			})
			i.Arrange(geometry.Rect(2, 2, 16, 16))
			other := NewIcon(func(p Painter, r geometry.Rectangle, c Color) { p.FillRect(r, c) })
			other.Arrange(geometry.Rect(22, 2, 16, 16))
			backend.Begin(40*scale, 20*scale, scale)
			backend.Clear(Color{})
			paintWidget(i, p)
			paintWidget(other, p)
			backend.End()
			for _, point := range []struct {
				x, y int
				want color.RGBA
			}{
				{3, 4, color.RGBA{}}, {5, 4, color.RGBA{R: 255, A: 255}},
				{11, 4, color.RGBA{}}, {23, 4, color.RGBA{R: 255, A: 255}},
				{37, 16, color.RGBA{R: 255, A: 255}}, {39, 19, color.RGBA{}},
			} {
				got := color.RGBAModel.Convert(out.image.At(point.x*int(scale), point.y*int(scale))).(color.RGBA)
				if got != point.want {
					t.Fatalf("scale %v pixel (%d,%d): %v want %v", scale, point.x, point.y, got, point.want)
				}
			}
			if spy.images != 0 {
				t.Fatal("unexpected image upload")
			}
		}()
	}
}

// Verify Icon uses the existing brush path without introducing an additional
// alpha conversion. Both paths must match the absolute premultiplied color.
func TestIconSoftwareMatchesDirectTranslucentDrawing(t *testing.T) {
	c := color.NRGBA{R: 255, G: 100, A: 128}
	useTestApplication(t, &application{style: style.Sheet(style.Name("icon").ForegroundColor(c))})
	out := &transparentFrame{}
	backend, err := software.NewPainter(out)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	i := NewIcon(func(p Painter, r geometry.Rectangle, fg Color) { p.FillRect(r, fg) })
	i.Arrange(geometry.Rect(0, 0, 8, 8))
	backend.Begin(16, 8, 1)
	backend.Clear(Color{})
	paintWidget(i, newPainter(backend, geometry.Rect(0, 0, 16, 8)))
	backend.FillRect(graphics.Rect(8, 0, 8, 8), graphics.ColorOf(c))
	backend.End()
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			got := color.RGBAModel.Convert(out.image.At(x, y))
			want := color.RGBAModel.Convert(out.image.At(x+8, y))
			if got != color.RGBAModel.Convert(c) {
				t.Fatalf("incorrect premultiplied icon pixel: %v", got)
			}
			if got != want {
				t.Fatalf("Icon changed translucent brush at (%d,%d): %v want %v", x, y, got, want)
			}
		}
	}
}

func TestIconInvalidation(t *testing.T) {
	i := NewIcon(func(Painter, geometry.Rectangle, Color) {})
	win := &window{}
	win.SetWidget(i)
	defer win.SetWidget(nil)
	win.layoutDirty, win.paintDirty = false, false
	i.SetSize(20)
	if !win.layoutDirty || !win.paintDirty {
		t.Fatal("size must request layout and paint")
	}
	win.layoutDirty, win.paintDirty = false, false
	i.SetSize(20)
	if win.layoutDirty || win.paintDirty {
		t.Fatal("unchanged size invalidated")
	}
	i.SetDrawFunc(func(Painter, geometry.Rectangle, Color) {})
	if win.layoutDirty || !win.paintDirty {
		t.Fatal("replacement must repaint without remeasurement")
	}
	win.layoutDirty, win.paintDirty = false, false
	i.SetDrawFunc(nil)
	if !win.layoutDirty || !win.paintDirty {
		t.Fatal("removing content must remeasure")
	}
}
