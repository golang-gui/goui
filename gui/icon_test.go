package gui

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

func TestIconMeasurement(t *testing.T) {
	i := NewIcon(iconTestDraw(func(Painter, geometry.Rectangle, Color) {}))
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
	i.SetSource(nil)
	check(layout.Unbounded(), geometry.Size{})
	i.SetSource(iconTestDraw(func(Painter, geometry.Rectangle, Color) {}))
	check(layout.Unbounded(), geometry.Size{Width: 16, Height: 16})
	i.SetVisible(false)
	check(layout.Unbounded(), geometry.Size{})
}

func TestIconPaintBoundsAndReplacement(t *testing.T) {
	var got geometry.Rectangle
	calls, value := 0, 0
	draw := func(v int) iconTestDraw {
		return func(_ Painter, rect geometry.Rectangle, _ Color) { calls++; value = v; got = rect }
	}
	i := NewIcon(draw(1))
	i.Arrange(geometry.Rect(100, 200, 40, 20))
	paintIconTest(i)
	if got != geometry.Rect(0, 0, 20, 20) || value != 1 {
		t.Fatalf("local centered rect: %+v, value %d", got, value)
	}
	i.SetSource(draw(2))
	i.Arrange(geometry.Rect(100, 200, 20, 40))
	paintIconTest(i)
	if got != geometry.Rect(0, 0, 20, 20) || value != 2 {
		t.Fatalf("replacement: %+v, value %d", got, value)
	}
	i.SetVisible(false)
	paintIconTest(i)
	i.SetVisible(true)
	i.Arrange(geometry.Rectangle{})
	paintIconTest(i)
	i.SetSource(nil)
	i.Arrange(geometry.Rect(0, 0, 20, 20))
	paintIconTest(i)
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
	i := NewIcon(iconTestDraw(func(_ Painter, _ geometry.Rectangle, c Color) { got = c; calls++ }))
	win.SetWidget(i)
	defer win.SetWidget(nil)
	i.Arrange(geometry.Rect(0, 0, 20, 20))
	paintIconTest(i)
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
			paintIconTest(i)
			if got != tc.want || calls != before+1 {
				t.Fatalf("color %+v, want %+v; calls %d", got, tc.want, calls)
			}
		})
	}
	red, blue := color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}
	app.SetStyleSheet(style.Sheet(style.Name("icon").ForegroundColor(red), style.Name("alternate").ForegroundColor(blue)))
	i.SetStyleName("alternate")
	paintIconTest(i)
	if got != graphics.ColorOf(blue) {
		t.Fatal("style-name change retained foreground")
	}
	i.SetVisible(false)
	app.SetStyleSheet(style.Sheet(style.Name("alternate").ForegroundColor(red)))
	i.SetVisible(true)
	paintIconTest(i)
	if got != graphics.ColorOf(red) {
		t.Fatal("reshow retained foreground")
	}
	win.SetWidget(nil)
	app.SetStyleSheet(style.Sheet(style.Name("alternate").ForegroundColor(blue)))
	win.SetWidget(i)
	paintIconTest(i)
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
	draw := iconTestDraw(func(_ Painter, _ geometry.Rectangle, c Color) { got = append(got, c) })
	i, other := NewIcon(draw), NewIcon(draw)
	button := NewButton()
	button.SetStyleName("parent")
	button.SetChild(i)
	other.SetStyleName("alternate")
	for _, icon := range []*Icon{i, other} {
		icon.Arrange(geometry.Rect(0, 0, 16, 16))
		paintIconTest(icon)
	}
	if len(got) != 2 || got[0] != graphics.ColorOf(red) || got[1] != graphics.ColorOf(blue) {
		t.Fatalf("shared draw or parent leaked color: %+v", got)
	}
}

type iconUploadBackend struct {
	graphics.Painter
	images int
}

func (p *iconUploadBackend) NewImage(src image.Image) (graphics.Image, error) {
	p.images++
	return p.Painter.NewImage(src)
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
			spy := &iconUploadBackend{Painter: backend}
			p := newPainter(spy, geometry.Rect(0, 0, 40, 20), scale)
			i := NewIcon(iconTestDraw(func(p Painter, rect geometry.Rectangle, c Color) {
				p.Save()
				defer p.Restore()
				p.SetClipRect(geometry.Rect(0, 0, 8*scale, 16*scale))
				p.SetTransform(geometry.Translate(2*scale, 0))
				p.FillRect(rect, c)
			}))
			i.Arrange(geometry.Rect(2, 2, 16, 16))
			other := NewIcon(iconTestDraw(func(p Painter, r geometry.Rectangle, c Color) { p.FillRect(r, c) }))
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
			if spy.images != 2 {
				t.Fatal("each icon should upload one image")
			}
		}()
	}
}

// Verify offscreen Source rendering and Icon's image upload do not introduce
// another alpha conversion. Both paths must match absolute premultiplied color.
func TestIconSoftwareMatchesDirectTranslucentDrawing(t *testing.T) {
	c := color.NRGBA{R: 255, G: 100, A: 128}
	useTestApplication(t, &application{style: style.Sheet(style.Name("icon").ForegroundColor(c))})
	out := &transparentFrame{}
	backend, err := software.NewPainter(out)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	i := NewIcon(iconTestDraw(func(p Painter, r geometry.Rectangle, fg Color) { p.FillRect(r, fg) }))
	i.Arrange(geometry.Rect(0, 0, 8, 8))
	backend.Begin(16, 8, 1)
	backend.Clear(Color{})
	paintWidget(i, newPainter(backend, geometry.Rect(0, 0, 16, 8), 1))
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
	i := NewIcon(iconTestDraw(func(Painter, geometry.Rectangle, Color) {}))
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
	i.SetSource(iconTestDraw(func(Painter, geometry.Rectangle, Color) {}))
	if win.layoutDirty || !win.paintDirty {
		t.Fatal("replacement must repaint without remeasurement")
	}
	win.layoutDirty, win.paintDirty = false, false
	i.SetSource(nil)
	if !win.layoutDirty || !win.paintDirty {
		t.Fatal("removing content must remeasure")
	}
}

// This test-only adapter migrates the old direct-drawing assertions to offscreen
// Source.Image. Production custom drawing uses the independent icon/draw package.
type iconTestDraw func(Painter, geometry.Rectangle, Color)

func (f iconTestDraw) Image(w, h int, c Color) image.Image {
	out := &transparentFrame{}
	p, err := software.NewPainter(out)
	if err != nil {
		panic(err)
	}
	defer p.Destroy()
	gp := newPainter(p, geometry.Rect(0, 0, float32(w), float32(h)), 1)
	p.Begin(float32(w), float32(h), 1)
	func() {
		defer p.End()
		p.Clear(Color{})
		bounds := geometry.Rect(0, 0, float32(w), float32(h))
		widget := newPainterTestWidget(func(p Painter) { f(p, bounds, c) })
		widget.Arrange(bounds)
		paintWidget(widget, gp)
	}()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), out.image, out.image.Bounds().Min, draw.Src)
	return img
}
func paintIconTest(i *Icon) {
	p := new(iconCacheBackend)
	iconFrame(p, 1, func(gp *painter) { i.Paint(gp) })
	i.clearImageCache()
}

type testIconImage func(int, int, Color) image.Image

func (f testIconImage) Image(w, h int, c Color) image.Image { return f(w, h, c) }

type iconCacheBackend struct {
	recordingPainterBackend
	active       bool
	images       []*iconCachedImage
	draws        int
	rect         geometry.Rectangle
	err          error
	placeholders int
}

type iconCachedImage struct {
	testNativeImage
	owner    *iconCacheBackend
	src      image.Image
	released bool
}

func (p *iconCacheBackend) Begin(_, _, _ float32) { p.active = true }
func (p *iconCacheBackend) End() {
	p.active = false
	for _, img := range p.images {
		if img.destroyed {
			img.released = true
		}
	}
}
func (p *iconCacheBackend) NewImage(src image.Image) (graphics.Image, error) {
	if p.err != nil {
		return nil, p.err
	}
	n := &iconCachedImage{testNativeImage: *newTestNativeImage(src), owner: p, src: src}
	p.images = append(p.images, n)
	return n, nil
}

func (p *iconCacheBackend) RenderImage(int, int, float32, func()) (image.Image, error) {
	return nil, errors.New("iconCacheBackend does not rasterize")
}
func (p *iconCacheBackend) DrawImage(rect geometry.Rectangle, img graphics.Image) {
	n := img.(*iconCachedImage)
	if n.owner != p || n.destroyed {
		panic("invalid native image owner/lifetime")
	}
	p.draws++
	p.rect = rect
}
func (p *iconCacheBackend) DrawRect(rect geometry.Rectangle, stroke float32, brush graphics.Brush) {
	p.placeholders++
}

func (n *iconCachedImage) Destroy() {
	n.destroyed = true
	if !n.owner.active {
		n.released = true
	}
}

func iconFrame(p *iconCacheBackend, scale float32, draw func(*painter)) {
	gp := newPainter(p, geometry.Rect(0, 0, 200, 200), scale)
	p.Begin(200*scale, 200*scale, scale)
	defer p.End()
	draw(gp)
}

func solidIconSource(counter *int) testIconImage {
	return testIconImage(func(w, h int, c Color) image.Image {
		*counter++
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				img.SetRGBA(x, y, color.RGBA{R: byte(c.R*128 + .5), G: byte(c.G*128 + .5), B: byte(c.B*128 + .5), A: byte(c.A*128 + .5)})
			}
		}
		return img
	})
}

func TestIconImageCacheAndColor(t *testing.T) {
	useTestApplication(t, &application{style: style.Sheet(style.Name("icon").ForegroundColor(color.NRGBA{R: 255, A: 128}))})
	rasterizations := 0
	source := solidIconSource(&rasterizations)
	i := NewIcon(source)
	i.Arrange(geometry.Rect(0, 0, 20, 20))
	defer i.clearImageCache()
	p := new(iconCacheBackend)
	for j := 0; j < 8; j++ {
		iconFrame(p, 1, func(gp *painter) { i.Paint(gp) })
	}
	if rasterizations != 1 || len(p.images) != 1 || p.draws != 8 {
		t.Fatalf("cache miss: rasters=%d uploads=%d draws=%d", rasterizations, len(p.images), p.draws)
	}
	// 128/255 source alpha × 128/255 premultiplied red -> 64/255 RGBA.
	if got := color.RGBAModel.Convert(p.images[0].src.At(10, 10)); got != (color.RGBA{R: 64, A: 64}) {
		t.Fatalf("premultiplied mask = %v", got)
	}
	i.SetStyleName("other")
	App.SetStyleSheet(style.Sheet(style.Name("other").ForegroundColor(color.NRGBA{B: 255, A: 128})))
	iconFrame(p, 1, func(gp *painter) {
		i.Paint(gp)
		if p.images[0].released {
			t.Fatal("retired image released inside frame")
		}
	})
	if rasterizations != 2 || len(p.images) != 2 || !p.images[0].destroyed || !p.images[0].released {
		t.Fatal("foreground was not sent to Source or old image leaked")
	}

	iconFrame(p, 2, func(gp *painter) { i.Paint(gp) })
	if rasterizations != 3 || p.images[2].width != 40 {
		t.Fatal("HiDPI did not render")
	}
	iconFrame(p, 2, func(gp *painter) { gp.SetTransform(geometry.Rotate(30).Scale(2, 3)); i.Paint(gp) })
	if p.images[3].width < 120 || p.images[3].width > 121 {
		t.Fatal("transform stretch ignored")
	}
	i.SetSource(source)
	iconFrame(p, 2, func(gp *painter) { i.Paint(gp) })
	if rasterizations != 5 {
		t.Fatal("explicit SetSource must refresh even the same function")
	}

}

func TestIconSourcesEmptyAndClosureIdentity(t *testing.T) {
	i := NewIcon(nil)
	if i.Source() != nil || i.Measure(layout.Unbounded()).Size != (geometry.Size{}) {
		t.Fatal("nil source is not empty")
	}
	value := 0
	makeDraw := func(n int) iconTestDraw { return func(Painter, geometry.Rectangle, Color) { value = n } }
	i.SetSource(makeDraw(1))
	i.Arrange(geometry.Rect(0, 0, 20, 20))
	paintIconTest(i)
	i.SetSource(makeDraw(2))
	paintIconTest(i)
	if value != 2 {
		t.Fatal("closure not replaced")
	}
}

func TestIconSourceRefreshReusesOnlyUnchangedPixels(t *testing.T) {
	bitmap := image.NewRGBA(image.Rect(0, 0, 16, 16))
	calls := 0
	source := testIconImage(func(int, int, Color) image.Image { calls++; return bitmap })
	i := NewIcon(source)
	defer i.clearImageCache()
	i.Arrange(geometry.Rect(0, 0, 16, 16))
	p := new(iconCacheBackend)
	paint := func() { iconFrame(p, 1, func(gp *painter) { i.Paint(gp) }) }
	paint()
	for j := 0; j < 3; j++ {
		i.SetSource(source)
		if p.images[0].destroyed {
			t.Fatal("SetSource discarded the reusable image before fetching pixels")
		}
		paint()
	}
	if calls != 4 || len(p.images) != 1 || p.draws != 4 {
		t.Fatalf("refresh/reuse: calls=%d uploads=%d draws=%d", calls, len(p.images), p.draws)
	}
	paint()
	if calls != 4 {
		t.Fatal("ordinary repaint must retain the request cache")
	}

	// The same Source can supply new immutable pixels after explicit refresh.
	bitmap = image.NewRGBA(image.Rect(0, 0, 16, 16))
	i.SetSource(source)
	p.err = errors.New("replacement upload")
	paint()
	if calls != 5 || !p.images[0].released || p.draws != 5 || p.placeholders != 1 {
		t.Fatal("failed replacement drew stale content or retained the old resource")
	}
	p.err = nil
	paint()
	if calls != 5 || len(p.images) != 2 || p.images[1].src != bitmap || p.draws != 6 {
		t.Fatal("replacement retry did not upload the cached new pixels")
	}

	i.SetSource(testIconImage(func(int, int, Color) image.Image { return nil }))
	paint()
	if !p.images[1].released || p.draws != 6 || p.placeholders != 2 {
		t.Fatal("missing replacement must release the image and never draw stale content")
	}
	i.SetSource(source)
	paint()
	i.SetSource(source) // Pending refresh must not prevent immediate nil cleanup.
	i.SetSource(nil)
	if len(p.images) != 3 || !p.images[2].released {
		t.Fatal("nil source must release pending reuse candidates immediately")
	}
}

func TestIconImageSharingUnmountAndFrameReplacement(t *testing.T) {
	n := 0
	s := solidIconSource(&n)
	a, b := NewIcon(s), NewIcon(s)
	first, second := &window{}, &window{}
	first.SetWidget(a)
	second.SetWidget(b)
	a.Arrange(geometry.Rect(0, 0, 16, 16))
	b.Arrange(geometry.Rect(0, 0, 32, 32))
	p, q := new(iconCacheBackend), new(iconCacheBackend)
	iconFrame(p, 1, func(gp *painter) { a.Paint(gp) })
	iconFrame(q, 1, func(gp *painter) { b.Paint(gp) })
	if n != 2 || p.images[0].width != 16 || q.images[0].width != 32 {
		t.Fatal("shared source polluted size/owner")
	}
	// Reattachment releases the first owner's image; the same source is safe
	// to render through a different painter afterwards.
	second.SetWidget(a)
	if !p.images[0].released || !q.images[0].released {
		t.Fatal("unmount did not release images")
	}
	iconFrame(q, 1, func(gp *painter) {
		a.Paint(gp)
		a.SetSource(iconTestDraw(func(Painter, geometry.Rectangle, Color) {}))
		if q.images[1].destroyed {
			t.Fatal("pending refresh discarded image before comparing new pixels")
		}
		a.Paint(gp)
		if q.images[1].released {
			t.Fatal("source replacement released in frame")
		}
	})
	if !q.images[1].destroyed || !q.images[1].released {
		t.Fatal("replacement leaked image")
	}
	a.SetSource(s) // Unmount must release even before a pending refresh paints.
	second.SetWidget(nil)
	if !q.images[2].released {
		t.Fatal("unmount leaked the pending reuse candidate")
	}
}

func TestIconMissingOutputCacheAndRefresh(t *testing.T) {
	calls := 0
	source := testIconImage(func(int, int, Color) image.Image { calls++; return nil })
	i := NewIcon(source)
	i.Arrange(geometry.Rect(0, 0, 16, 16))
	p := new(iconCacheBackend)
	for j := 0; j < 3; j++ {
		iconFrame(p, 1, func(gp *painter) { i.Paint(gp) })
	}
	if calls != 1 || p.draws != 0 || p.placeholders != 3 {
		t.Fatal("missing result was not cached/drawn")
	}
	iconFrame(p, 2, func(gp *painter) { i.Paint(gp) })
	if calls != 2 {
		t.Fatal("new size did not request pixels")
	}
	i.SetSource(source)
	iconFrame(p, 2, func(gp *painter) { i.Paint(gp) })
	if calls != 3 {
		t.Fatal("SetSource did not refresh missing result")
	}
	i.SetSource(nil)
	iconFrame(p, 2, func(gp *painter) { i.Paint(gp) })
	if p.placeholders != 5 {
		t.Fatal("nil source must not draw a placeholder")
	}
}

func TestIconImageUploadRetryAndInvalidOutput(t *testing.T) {
	n := 0
	i := NewIcon(solidIconSource(&n))
	defer i.clearImageCache()
	i.Arrange(geometry.Rect(0, 0, 16, 16))
	p := &iconCacheBackend{err: errors.New("upload")}
	iconFrame(p, 1, func(gp *painter) { i.Paint(gp) })
	if p.placeholders != 1 || p.draws != 0 {
		t.Fatal("upload failure must draw placeholder")
	}
	p.err = nil
	iconFrame(p, 1, func(gp *painter) { i.Paint(gp) })
	if n != 1 || len(p.images) != 1 || p.draws != 1 {
		t.Fatal("upload retry regenerated pixels or failed")
	}
	old := p.images[0]
	calls := 0
	i.SetSource(testIconImage(func(int, int, Color) image.Image { calls++; return image.NewRGBA(image.Rectangle{}) }))
	for j := 0; j < 2; j++ {
		iconFrame(p, 1, func(gp *painter) { i.Paint(gp) })
	}
	if calls != 1 || p.placeholders != 3 || !old.released {
		t.Fatal("empty output must be cached; old image must be released")
	}
	iconFrame(p, 1000, func(gp *painter) { i.Paint(gp) })
	if calls != 1 || p.placeholders != 4 {
		t.Fatal("oversized request reached source")
	}
}

func TestPainterRasterScaleAndFrameCleanup(t *testing.T) {
	p := newPainter(new(recordingPainterBackend), geometry.Rect(0, 0, 10, 10), 2)
	p.SetTransform(geometry.Translate(200, 300).Rotate(20).Scale(-2, 3))
	if math.Abs(float64(p.PixelScale()-6)) > 1e-5 {
		t.Fatal(p.PixelScale())
	}
	p.SetTransform(geometry.Scale(0, 0))
	if p.PixelScale() != 0 {
		t.Fatal("degenerate transform")
	}
	// Exercise scale propagation and End in the actual root frame.
	backend := new(iconCacheBackend)
	root := rootBase{painter: backend, width: 10, height: 10, pixelWidth: 20, pixelHeight: 20}
	ran := false
	w := newPainterTestWidget(func(p Painter) {
		if p.PixelScale() != 2 {
			t.Fatal("root scale not propagated")
		}
		ran = true
	})
	w.Arrange(geometry.Rect(0, 0, 10, 10))
	root.drawFrame(w, style.Style{})
	if !ran || backend.active {
		t.Fatal("root did not finish frame")
	}
}

func TestIconImageSoftwarePixels(t *testing.T) {
	useTestApplication(t, &application{style: style.Sheet(style.Name("icon").ForegroundColor(color.NRGBA{R: 255, A: 128}))})
	for _, scale := range []float32{1, 2} {
		out := &transparentFrame{}
		backend, err := software.NewPainter(out)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		i := NewIcon(solidIconSource(&n))
		i.Arrange(geometry.Rect(4, 4, 8, 8))
		gp := newPainter(backend, geometry.Rect(0, 0, 20, 20), scale)
		backend.Begin(20*scale, 20*scale, scale)
		backend.Clear(Color{})
		paintWidget(i, gp)
		backend.End()
		for _, sample := range []struct {
			x, y int
			want color.RGBA
		}{{6, 6, color.RGBA{R: 64, A: 64}}, {2, 2, color.RGBA{}}} {
			got := color.RGBAModel.Convert(out.image.At(sample.x*int(scale), sample.y*int(scale))).(color.RGBA)
			if got != sample.want {
				t.Fatalf("scale %v pixel: %v != %v", scale, got, sample.want)
			}
		}
		i.clearImageCache()
		backend.Destroy()
	}
}

func TestIconMissingPlaceholderPixels(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		for _, ink := range []color.NRGBA{{R: 255, A: 128}, {B: 255, A: 128}} {
			useTestApplication(t, &application{style: style.Sheet(style.Name("icon").ForegroundColor(ink))})
			out := &transparentFrame{}
			base, err := software.NewPainter(out)
			if err != nil {
				t.Fatal(err)
			}
			func() {
				defer base.Destroy()
				spy := &iconUploadBackend{Painter: base}
				i := NewIcon(testIconImage(func(int, int, Color) image.Image { return nil }))
				i.Arrange(geometry.Rect(0, 0, 16, 16))
				gp := newPainter(spy, geometry.Rect(0, 0, 40, 40), scale)
				base.Begin(40*scale, 40*scale, scale)
				base.Clear(Color{})
				// Explicit clockwise matrix in Y-down coordinates; keep the pixel
				// expectation independent of the rotation helper's sign convention.
				gp.SetTransform(geometry.Transform{A11: .70710678, A12: -.70710678, A21: .70710678, A22: .70710678, TX: 20, TY: 8})
				gp.SetClipRect(geometry.Rect(0, 0, 20, 40))
				i.Paint(gp)
				base.End()
				if spy.images != 0 {
					t.Fatal("placeholder tried to upload another image")
				}
				visible := 0
				for y := 0; y < 40*int(scale); y++ {
					for x := 0; x < 40*int(scale); x++ {
						c := color.RGBAModel.Convert(out.image.At(x, y)).(color.RGBA)
						if c.A == 0 {
							continue
						}
						visible++
						// The 45-degree rotation fits in [8,32] x [9,31]; explicit
						// clipping removes the right half. RGB is premultiplied:
						// pure red/blue must equal alpha even at AA fringes.
						if x >= 20*int(scale) || x < 8*int(scale) || y < 9*int(scale) || y >= 31*int(scale) {
							t.Fatalf("placeholder escaped clip/transform at %d,%d", x, y)
						}
						if ink.R != 0 && (c.R != c.A || c.G != 0 || c.B != 0) ||
							ink.B != 0 && (c.B != c.A || c.R != 0 || c.G != 0) {
							t.Fatal("placeholder did not use premultiplied style color", c)
						}
					}
				}
				if visible == 0 {
					t.Fatal("missing content was invisible")
				}
			}()
		}
	}
}

func TestIconTransparentPixelsAreNotMissing(t *testing.T) {
	i := NewIcon(testIconImage(func(w, h int, _ Color) image.Image { return image.NewRGBA(image.Rect(0, 0, w, h)) }))
	defer i.clearImageCache()
	i.Arrange(geometry.Rect(0, 0, 16, 16))
	p := new(iconCacheBackend)
	iconFrame(p, 1, func(gp *painter) { i.Paint(gp) })
	if p.draws != 1 || p.placeholders != 0 {
		t.Fatal("transparent content replaced by placeholder")
	}
}
