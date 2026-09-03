package software

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

type testDrawer struct {
	result image.Image
}

type testTextLayout struct {
	scale      float32
	bitmap     typography.TextBitmap
	rasterizes int
	changed    signal.Signal0
	destroyed  signal.Signal0
	didDestroy bool
}

func (l *testTextLayout) Destroy() {
	if l.didDestroy {
		return
	}
	l.didDestroy = true
	l.destroyed.Emit()
}
func (l *testTextLayout) Rasterize(scale float32, _ []byte) (typography.TextBitmap, error) {
	l.scale = scale
	l.rasterizes++
	return l.bitmap, nil
}
func (l *testTextLayout) ConnectChanged(fn func()) signal.Handle { return l.changed.Connect(fn) }
func (l *testTextLayout) ConnectDestroy(fn func()) signal.Handle { return l.destroyed.Connect(fn) }
func (*testTextLayout) Text() string                             { return "test" }
func (*testTextLayout) Format() typography.TextFormat            { return typography.TextFormat{} }
func (*testTextLayout) Size() (float32, float32)                 { return 0, 0 }
func (l *testTextLayout) SetSize(float32, float32)               { l.changed.Emit() }
func (l *testTextLayout) SetTextAlignment(typography.TextAlignment) {
	l.changed.Emit()
}
func (l *testTextLayout) SetWrapMode(typography.WrapMode) { l.changed.Emit() }
func (l *testTextLayout) SetTextFont(int, int, typography.FontInfo) {
	l.changed.Emit()
}
func (l *testTextLayout) SetTextColor(int, int, color.Color) { l.changed.Emit() }
func (l *testTextLayout) SetUnderline(int, int, bool)        { l.changed.Emit() }
func (l *testTextLayout) SetStrikethrough(int, int, bool)    { l.changed.Emit() }
func (*testTextLayout) MeasureSize() (float32, float32)      { return 0, 0 }
func (*testTextLayout) MeasureMetrics() ([]typography.TextLine, []typography.TextCluster) {
	return nil, nil
}

func TestLinearGradientPixels(t *testing.T) {
	img := renderGradient(t, graphics.LinearGradient{
		Start:      graphics.Point{X: 0.5},
		End:        graphics.Point{X: 9.5},
		StartColor: graphics.RGB(255, 0, 0),
		EndColor:   graphics.RGB(0, 0, 255),
	})

	assertColorNear(t, img.At(0, 0), color.RGBA{R: 255, A: 255}, 1)
	assertColorNear(t, img.At(9, 0), color.RGBA{B: 255, A: 255}, 1)
	assertColorNear(t, img.At(5, 0), color.RGBA{R: 113, B: 142, A: 255}, 2)
}

func TestLinearGradientTransformAndDegeneratePoint(t *testing.T) {
	var d testDrawer
	painter, err := NewPainter(&d)
	if err != nil {
		t.Fatal(err)
	}
	painter.Begin(20, 1, 1)
	painter.Clear(graphics.RGB(0, 0, 0))
	painter.SetTransform(geometry.Translate(5, 0))
	painter.FillRect(graphics.Rect(0, 0, 10, 1), graphics.LinearGradient{
		Start:      graphics.Point{},
		End:        graphics.Point{X: 10},
		StartColor: graphics.RGB(255, 0, 0),
		EndColor:   graphics.RGB(0, 0, 255),
	})
	painter.SetTransform(geometry.Identity())
	painter.FillRect(graphics.Rect(0, 0, 1, 1), graphics.LinearGradient{
		StartColor: graphics.RGB(0, 255, 0),
		EndColor:   graphics.RGB(0, 0, 255),
	})
	painter.End()

	assertColorNear(t, d.result.At(5, 0), color.RGBA{R: 242, B: 13, A: 255}, 3)
	assertColorNear(t, d.result.At(0, 0), color.RGBA{G: 255, A: 255}, 1)
}

func TestLinearGradientPremultipliedAlpha(t *testing.T) {
	color := interpolateGradientColor(graphics.RGBA(255, 0, 0, 255), graphics.RGBA(0, 0, 255, 0), 0.5)
	if color.A != 0.5 || color.R != 1 || color.G != 0 || color.B != 0 {
		t.Fatalf("unexpected premultiplied interpolation: %+v", color)
	}
}

func TestPainterTransformUsesLogicalCoordinatesAtHiDPI(t *testing.T) {
	var d testDrawer
	painter, err := NewPainter(&d)
	if err != nil {
		t.Fatal(err)
	}
	painter.Begin(40, 8, 2)
	painter.Clear(graphics.RGB(0, 0, 0))
	painter.SetTransform(geometry.Translate(3, 0))
	painter.FillRect(graphics.Rect(0, 0, 2, 2), graphics.RGB(255, 0, 0))
	painter.End()

	assertColorNear(t, d.result.At(5, 1), color.RGBA{A: 255}, 1)
	assertColorNear(t, d.result.At(6, 1), color.RGBA{R: 255, A: 255}, 1)
	assertColorNear(t, d.result.At(9, 3), color.RGBA{R: 255, A: 255}, 1)
	assertColorNear(t, d.result.At(10, 3), color.RGBA{A: 255}, 1)
}

func TestLinearGradientSharesHiDPITransformWithGeometry(t *testing.T) {
	var d testDrawer
	painter, err := NewPainter(&d)
	if err != nil {
		t.Fatal(err)
	}
	painter.Begin(40, 2, 2)
	painter.Clear(graphics.RGB(0, 0, 0))
	painter.SetTransform(geometry.Translate(3, 0))
	painter.FillRect(graphics.Rect(0, 0, 2, 1), graphics.LinearGradient{
		Start:      graphics.Point{},
		End:        graphics.Point{X: 2},
		StartColor: graphics.RGB(255, 0, 0),
		EndColor:   graphics.RGB(0, 0, 255),
	})
	painter.End()

	assertColorNear(t, d.result.At(6, 0), color.RGBA{R: 223, B: 32, A: 255}, 3)
	assertColorNear(t, d.result.At(9, 0), color.RGBA{R: 32, B: 223, A: 255}, 3)
}

func TestPainterRotatesRectGeometryInsteadOfBoundingBox(t *testing.T) {
	var d testDrawer
	painter, err := NewPainter(&d)
	if err != nil {
		t.Fatal(err)
	}
	painter.Begin(100, 100, 1)
	painter.Clear(graphics.RGB(0, 0, 0))
	painter.SetTransform(geometry.Translate(50, 50).Rotate(45))
	painter.FillRect(graphics.Rect(0, 0, 20, 10), graphics.RGB(255, 0, 0))
	painter.End()

	assertColorNear(t, d.result.At(60, 46), color.RGBA{R: 255, A: 255}, 1)
	assertColorNear(t, d.result.At(70, 37), color.RGBA{A: 255}, 1)
}

func TestStrokePrimitivesApplyWidthBeforeBuildingPath(t *testing.T) {
	tests := []struct {
		name string
		draw func(graphics.Painter)
	}{
		{
			name: "rectangle",
			draw: func(p graphics.Painter) {
				p.DrawRect(graphics.Rect(8, 6, 16, 21), 4, graphics.RGB(255, 255, 255))
			},
		},
		{
			name: "rounded rectangle",
			draw: func(p graphics.Painter) {
				p.DrawRoundRect(graphics.Rect(8, 6, 16, 21), 4, 4, graphics.RGB(255, 255, 255))
			},
		},
		{
			name: "ellipse",
			draw: func(p graphics.Painter) {
				p.DrawEllipse(graphics.Point{X: 16, Y: 16.5}, 8, 8, 4, graphics.RGB(255, 255, 255))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			img := renderPainter(t, 32, 33, 1, geometry.Identity(), graphics.Rectangle{}, test.draw)
			assertAlphaRange(t, img.At(5, 16), 0, 5)
			assertAlphaRange(t, img.At(6, 16), 250, 255)
			assertAlphaRange(t, img.At(9, 16), 250, 255)
			assertAlphaRange(t, img.At(10, 16), 0, 5)
		})
	}
}

func TestStrokeWidthDoesNotLeakBetweenDrawCalls(t *testing.T) {
	img := renderPainter(t, 48, 32, 1, geometry.Identity(), graphics.Rectangle{}, func(p graphics.Painter) {
		p.DrawRect(graphics.Rect(8, 6, 12, 20), 6, graphics.RGB(255, 255, 255))
		p.DrawRect(graphics.Rect(32, 6, 12, 20), 2, graphics.RGB(255, 255, 255))
	})

	assertAlphaRange(t, img.At(29, 16), 0, 5)
	assertAlphaRange(t, img.At(30, 16), 0, 5)
	assertAlphaRange(t, img.At(31, 16), 250, 255)
	assertAlphaRange(t, img.At(32, 16), 250, 255)
}

func TestStrokeWidthUsesPhysicalPixelsAtHiDPI(t *testing.T) {
	img := renderPainter(t, 48, 40, 2, geometry.Identity(), graphics.Rectangle{}, func(p graphics.Painter) {
		p.DrawRect(graphics.Rect(4.5, 3, 12, 14), 1, graphics.RGB(255, 255, 255))
	})

	assertAlphaRange(t, img.At(7, 20), 0, 5)
	assertAlphaRange(t, img.At(8, 20), 250, 255)
	assertAlphaRange(t, img.At(9, 20), 250, 255)
	assertAlphaRange(t, img.At(10, 20), 0, 5)
}

func TestDrawTextLayoutUsesTransformRasterScaleAndLogicalSize(t *testing.T) {
	pixels := make([]byte, 6*3*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i], pixels[i+3] = 255, 255
	}
	layout := &testTextLayout{bitmap: typography.TextBitmap{
		Width: 6, Height: 3, Stride: 6 * 4, Pixels: pixels,
	}}
	var d testDrawer
	painter, err := NewPainter(&d)
	if err != nil {
		t.Fatal(err)
	}

	painter.Begin(30, 20, 2)
	painter.Clear(graphics.Color{})
	painter.SetTransform(geometry.Translate(2, 2).Scale(1.5, 1.5))
	painter.DrawTextLayout(graphics.Point{}, layout)
	painter.End()

	if math.Abs(float64(layout.scale-3)) > 1e-5 {
		t.Fatalf("text raster scale = %v, want 3", layout.scale)
	}
	assertAlphaRange(t, d.result.At(9, 5), 250, 255)
	assertAlphaRange(t, d.result.At(10, 5), 0, 0)
}

func TestDrawTextLayoutCachesImageAcrossPositionsAndFrames(t *testing.T) {
	pixels := make([]byte, 4*2*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i], pixels[i+3] = 255, 255
	}
	layout := &testTextLayout{bitmap: typography.TextBitmap{
		Width: 4, Height: 2, Stride: 4 * 4, Pixels: pixels,
	}}
	var d testDrawer
	base, err := NewPainter(&d)
	if err != nil {
		t.Fatal(err)
	}
	painter := base.(*Painter)
	defer painter.Destroy()

	painter.Begin(40, 20, 1)
	painter.Clear(graphics.Color{})
	painter.DrawTextLayout(graphics.Point{}, layout)
	painter.DrawTextLayout(graphics.Point{X: 10}, layout)
	painter.End()

	painter.Begin(40, 20, 1)
	painter.Clear(graphics.Color{})
	painter.SetTransform(geometry.Translate(3, 2))
	painter.DrawTextLayout(graphics.Point{X: 4}, layout)
	painter.End()

	if layout.rasterizes != 1 {
		t.Fatalf("stable layout rasterized %d times, want 1", layout.rasterizes)
	}
	if len(painter.images) != 1 {
		t.Fatalf("cached image count = %d, want 1", len(painter.images))
	}

	layout.SetSize(20, 10)
	if len(painter.images) != 0 {
		t.Fatalf("changed layout retained %d images", len(painter.images))
	}
	painter.Begin(40, 20, 1)
	painter.DrawTextLayout(graphics.Point{}, layout)
	painter.End()
	if layout.rasterizes != 2 {
		t.Fatalf("changed layout rasterized %d times, want 2", layout.rasterizes)
	}

	layout.Destroy()
	if len(painter.images) != 0 {
		t.Fatalf("destroyed layout retained %d images", len(painter.images))
	}
}

func TestDrawTextLayoutDefersFrameInvalidationAndSupportsMultiplePainters(t *testing.T) {
	pixels := make([]byte, 2*2*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i+1], pixels[i+3] = 255, 255
	}
	layout := &testTextLayout{bitmap: typography.TextBitmap{
		Width: 2, Height: 2, Stride: 2 * 4, Pixels: pixels,
	}}
	var firstDrawer, secondDrawer testDrawer
	firstBase, _ := NewPainter(&firstDrawer)
	secondBase, _ := NewPainter(&secondDrawer)
	first := firstBase.(*Painter)
	second := secondBase.(*Painter)
	defer first.Destroy()
	defer second.Destroy()

	first.Begin(10, 10, 1)
	first.DrawTextLayout(graphics.Point{}, layout)
	oldImage, ok := first.textImages.Lookup(layout, 1)
	if !ok {
		t.Fatal("first painter did not cache the layout")
	}
	layout.SetTextAlignment(typography.TextAlignCenter)
	if !oldImage.(*imageResource).pendingDestroy {
		t.Fatal("frame-active invalidation destroyed or retained the old cache entry")
	}
	first.DrawTextLayout(graphics.Point{}, layout)
	if len(first.images) != 2 {
		t.Fatalf("active frame image count = %d, want old and replacement", len(first.images))
	}
	first.End()
	if !oldImage.(*imageResource).destroyed || len(first.images) != 1 {
		t.Fatal("pending text image was not released after End")
	}

	second.Begin(10, 10, 1)
	second.DrawTextLayout(graphics.Point{}, layout)
	second.End()
	if len(first.images) != 1 || len(second.images) != 1 {
		t.Fatalf("independent cache sizes = %d, %d, want 1, 1", len(first.images), len(second.images))
	}

	layout.Destroy()
	if len(first.images) != 0 || len(second.images) != 0 {
		t.Fatalf("layout destroy cache sizes = %d, %d, want 0, 0", len(first.images), len(second.images))
	}
}

func TestDrawBitmapRotationUsesBilinearSampling(t *testing.T) {
	bitmap := graphics.MakeBitmap(0, 0, 4, 4, graphics.PixelFormatRGBA, nil)
	for y := 0; y < 4; y++ {
		for x := 0; x < 2; x++ {
			bitmap.SetPixel(x, y, 255, 255, 255, 255)
		}
	}

	var d testDrawer
	base, err := NewPainter(&d)
	if err != nil {
		t.Fatal(err)
	}
	painter := base.(*Painter)
	painter.Begin(24, 24, 1)
	painter.Clear(graphics.Color{})
	painter.SetTransform(geometry.Translate(10.5, 10.5).Rotate(30).Translate(-2, -2))
	painter.drawBitmap(graphics.Rect(0, 0, 4, 4), bitmap)
	painter.End()

	// Device pixel (10,10) maps to local (2,2), exactly halfway between the
	// opaque and transparent source columns. Nearest-neighbor sampling returned
	// zero here; bilinear sampling preserves an intermediate coverage value.
	assertAlphaRange(t, d.result.At(10, 10), 120, 136)
}

func TestSampleBitmapBilinearInterpolatesPremultipliedFormats(t *testing.T) {
	tests := []struct {
		name   string
		format graphics.PixelFormat
		pixels []byte
	}{
		{
			name:   "rgba",
			format: graphics.PixelFormatRGBA,
			pixels: []byte{128, 0, 0, 128, 0, 0, 0, 0},
		},
		{
			name:   "bgra",
			format: graphics.PixelFormatBGRA,
			pixels: []byte{0, 0, 128, 128, 0, 0, 0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bitmap := graphics.Bitmap{
				Width: 2, Height: 1, Stride: 8, Format: tt.format, Pixels: tt.pixels,
			}
			got := sampleBitmapBilinear(bitmap, 0.5, 0)
			want := color.RGBA{B: 64, A: 64}
			if got != want {
				t.Fatalf("midpoint color = %+v, want %+v", got, want)
			}

			got = sampleBitmapBilinear(bitmap, -0.5, 0)
			want = color.RGBA{B: 128, A: 128}
			if got != want {
				t.Fatalf("clamped edge color = %+v, want %+v", got, want)
			}
		})
	}
}

func TestPainterCompositesTransparentBrushAndPreservesClip(t *testing.T) {
	var d testDrawer
	painter, err := NewPainter(&d)
	if err != nil {
		t.Fatal(err)
	}
	painter.Begin(6, 2, 1)
	painter.Clear(graphics.RGB(0, 0, 255))
	painter.SetClipRect(graphics.Rect(2, 0, 2, 2))
	painter.FillRect(graphics.Rect(0, 0, 6, 2), graphics.RGBA(255, 0, 0, 128))
	painter.End()

	assertColorNear(t, d.result.At(1, 0), color.RGBA{B: 255, A: 255}, 1)
	assertColorNear(t, d.result.At(2, 0), color.RGBA{R: 128, B: 127, A: 255}, 2)
	assertColorNear(t, d.result.At(4, 0), color.RGBA{B: 255, A: 255}, 1)
}

func TestImageResourceSnapshotsReusesAndCompositesPixels(t *testing.T) {
	var d testDrawer
	painter, err := NewPainter(&d)
	if err != nil {
		t.Fatal(err)
	}
	src := image.NewNRGBA(image.Rect(3, 4, 5, 5))
	src.SetNRGBA(3, 4, color.NRGBA{R: 255, A: 128})
	src.SetNRGBA(4, 4, color.NRGBA{G: 255, A: 255})
	resource, err := painter.NewImage(src)
	if err != nil {
		t.Fatal(err)
	}
	defer resource.Destroy()
	if width, height := resource.Size(); width != 2 || height != 1 {
		t.Fatalf("resource size = %dx%d, want 2x1", width, height)
	}

	// NewImage snapshots the source. Later mutations must not affect rendering.
	src.SetNRGBA(3, 4, color.NRGBA{B: 255, A: 255})
	painter.Begin(4, 2, 1)
	painter.Clear(graphics.RGB(0, 0, 255))
	painter.SetClipRect(graphics.Rect(0, 0, 3, 2))
	painter.DrawImage(graphics.Rect(0, 0, 4, 2), resource)
	painter.End()

	assertColorNear(t, d.result.At(0, 0), color.RGBA{R: 128, B: 127, A: 255}, 1)
	assertColorNear(t, d.result.At(2, 0), color.RGBA{G: 255, A: 255}, 1)
	assertColorNear(t, d.result.At(3, 0), color.RGBA{B: 255, A: 255}, 1)
}

func TestImageResourceRejectsWrongPainterAndDestroyedResource(t *testing.T) {
	var firstDrawer, secondDrawer testDrawer
	first, _ := NewPainter(&firstDrawer)
	second, _ := NewPainter(&secondDrawer)
	resource, err := first.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	if err != nil {
		t.Fatal(err)
	}

	second.Begin(1, 1, 1)
	assertPanics(t, func() { second.DrawImage(graphics.Rect(0, 0, 1, 1), resource) })
	second.End()

	resource.Destroy()
	first.Begin(1, 1, 1)
	assertPanics(t, func() { first.DrawImage(graphics.Rect(0, 0, 1, 1), resource) })
	first.End()
}

func TestImageAndPainterDestroyRejectActiveFrame(t *testing.T) {
	var d testDrawer
	painter, err := NewPainter(&d)
	if err != nil {
		t.Fatal(err)
	}
	resource, err := painter.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	if err != nil {
		t.Fatal(err)
	}

	painter.Begin(1, 1, 1)
	assertPanics(t, resource.Destroy)
	assertPanics(t, func() { _ = resource.Update(image.NewRGBA(image.Rect(0, 0, 1, 1))) })
	assertPanics(t, painter.Destroy)
	painter.End()

	resource.Destroy()
	if native := resource.(*imageResource); !native.destroyed || native.owner != nil {
		t.Fatal("image was not destroyed after the frame")
	}
}

func TestImageUpdateReusesStorageAndDetachesSource(t *testing.T) {
	var d testDrawer
	painter, err := NewPainter(&d)
	if err != nil {
		t.Fatal(err)
	}
	resource, err := painter.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	if err != nil {
		t.Fatal(err)
	}
	defer resource.Destroy()
	native := resource.(*imageResource)
	pixels := &native.bitmap.Pixels[0]

	src := image.NewNRGBA(image.Rect(4, 5, 5, 6))
	src.SetNRGBA(4, 5, color.NRGBA{G: 255, A: 255})
	if err := resource.Update(src); err != nil {
		t.Fatal(err)
	}
	if &native.bitmap.Pixels[0] != pixels {
		t.Fatal("same-size update replaced the software pixel storage")
	}
	src.SetNRGBA(4, 5, color.NRGBA{B: 255, A: 255})

	painter.Begin(1, 1, 1)
	painter.Clear(graphics.RGB(0, 0, 0))
	painter.DrawImage(graphics.Rect(0, 0, 1, 1), resource)
	painter.End()
	assertColorNear(t, d.result.At(0, 0), color.RGBA{G: 255, A: 255}, 1)

	if err := resource.Update(image.NewRGBA(image.Rect(0, 0, 2, 1))); err == nil {
		t.Fatal("size-changing update succeeded")
	}
}

func assertPanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

func BenchmarkDrawImageStatic1024(b *testing.B) {
	var d testDrawer
	painter, err := NewPainter(&d)
	if err != nil {
		b.Fatal(err)
	}
	src := image.NewRGBA(image.Rect(0, 0, 1024, 1024))
	for i := 3; i < len(src.Pix); i += 4 {
		src.Pix[i] = 255
	}
	resource, err := painter.NewImage(src)
	if err != nil {
		b.Fatal(err)
	}
	defer resource.Destroy()
	painter.Begin(1024, 1024, 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		painter.DrawImage(graphics.Rect(0, 0, 1024, 1024), resource)
	}
	b.StopTimer()
	painter.End()
}

func BenchmarkDrawImageRotated256(b *testing.B) {
	var d testDrawer
	painter, err := NewPainter(&d)
	if err != nil {
		b.Fatal(err)
	}
	src := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for i := 3; i < len(src.Pix); i += 4 {
		src.Pix[i] = 255
	}
	resource, err := painter.NewImage(src)
	if err != nil {
		b.Fatal(err)
	}
	defer resource.Destroy()

	painter.Begin(512, 512, 1)
	painter.SetTransform(geometry.Translate(256, 256).Rotate(15).Translate(-128, -128))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		painter.DrawImage(graphics.Rect(0, 0, 256, 256), resource)
	}
	b.StopTimer()
	painter.End()
}

func TestBoxShadowFullAndSingleBottomEdge(t *testing.T) {
	img := renderPainter(t, 40, 35, 1, geometry.Identity(), graphics.Rectangle{}, func(p graphics.Painter) {
		p.DrawBoxShadow(graphics.Rect(10, 10, 20, 10), 3, graphics.BoxShadow{
			Color:      graphics.RGBA(255, 0, 0, 255),
			BlurRadius: 4,
		})
	})
	assertAlphaRange(t, img.At(20, 15), 250, 255)
	assertAlphaRange(t, img.At(20, 9), 90, 160)
	assertAlphaRange(t, img.At(20, 5), 0, 4)
	assertAlphaRange(t, img.At(5, 15), 0, 4)
	assertAlphaRange(t, img.At(20, 3), 0, 0)

	img = renderPainter(t, 40, 35, 1, geometry.Identity(), graphics.Rectangle{}, func(p graphics.Painter) {
		blur := float32(4)
		rect := graphics.Rect(10, 10, 20, 10)
		p.DrawBoxShadow(rect, 2, graphics.BoxShadow{
			Color:        graphics.RGBA(0, 0, 0, 220),
			Offset:       graphics.Point{Y: blur},
			BlurRadius:   blur,
			SpreadRadius: -blur,
		})
		p.FillRect(rect, graphics.RGB(255, 255, 255))
	})
	assertAlphaRange(t, img.At(20, 22), 20, 210)
	assertAlphaRange(t, img.At(20, 8), 0, 2)
	assertAlphaRange(t, img.At(8, 15), 0, 7)
	assertAlphaRange(t, img.At(31, 15), 0, 7)
}

func TestBoxShadowCompositionClipTransformAndHiDPI(t *testing.T) {
	img := renderPainter(t, 60, 40, 1, geometry.Identity(), graphics.Rect(15, 0, 30, 40), func(p graphics.Painter) {
		p.DrawBoxShadow(graphics.Rect(10, 10, 20, 10), 0, graphics.BoxShadow{
			Color: graphics.RGBA(255, 0, 0, 128), BlurRadius: 4,
		})
		p.DrawBoxShadow(graphics.Rect(10, 10, 20, 10), 0, graphics.BoxShadow{
			Color: graphics.RGBA(0, 0, 255, 128), Offset: graphics.Point{X: 2}, BlurRadius: 4,
		})
	})
	assertColorNear(t, img.At(20, 15), color.RGBA{R: 85, B: 170, A: 192}, 3)
	assertAlphaRange(t, img.At(14, 15), 0, 2)
	assertAlphaRange(t, img.At(15, 15), 40, 200)

	img = renderPainter(t, 80, 50, 2, geometry.Translate(10, 10).Rotate(90), graphics.Rectangle{}, func(p graphics.Painter) {
		p.DrawBoxShadow(graphics.Rect(0, -4, 8, 4), 0, graphics.BoxShadow{
			Color: graphics.RGBA(0, 255, 0, 255), BlurRadius: 1,
		})
	})
	// Local (4,-2) rotates clockwise to (-2,-4), translates to (8,6),
	// then scales to device pixel (16,12).
	assertAlphaRange(t, img.At(16, 12), 240, 255)
	assertAlphaRange(t, img.At(5, 5), 0, 2)

	img = renderPainter(t, 50, 30, 1, geometry.Translate(5, 5).Scale(2, 1), graphics.Rectangle{}, func(p graphics.Painter) {
		p.DrawBoxShadow(graphics.Rect(0, 0, 5, 5), 0, graphics.BoxShadow{
			Color: graphics.RGBA(0, 255, 0, 255), BlurRadius: 1,
		})
	})
	assertAlphaRange(t, img.At(10, 7), 240, 255)
	assertAlphaRange(t, img.At(2, 7), 0, 2)
}

func TestBoxShadowFractionalHiDPI(t *testing.T) {
	img := renderPainter(t, 40, 30, 1.25, geometry.Identity(), graphics.Rectangle{}, func(p graphics.Painter) {
		p.DrawBoxShadow(graphics.Rect(8, 6, 8, 8), 3, graphics.BoxShadow{
			Color: graphics.RGBA(20, 40, 80, 200), BlurRadius: 2,
		})
	})
	assertAlphaRange(t, img.At(15, 12), 185, 200)
	assertAlphaRange(t, img.At(2, 12), 0, 0)
}

func TestBoxShadowTransparentColorAndContractedEmptyAreNoOps(t *testing.T) {
	img := renderPainter(t, 20, 20, 1, geometry.Identity(), graphics.Rectangle{}, func(p graphics.Painter) {
		p.DrawBoxShadow(graphics.Rect(4, 4, 8, 8), 2, graphics.BoxShadow{
			Color: graphics.RGBA(255, 0, 0, 0), BlurRadius: 4,
		})
		p.DrawBoxShadow(graphics.Rect(4, 4, 8, 8), 2, graphics.BoxShadow{
			Color: graphics.RGB(255, 0, 0), SpreadRadius: -4,
		})
	})
	assertAlphaRange(t, img.At(8, 8), 0, 2)
}

func renderPainter(t *testing.T, width, height int, scale float32, transform geometry.Transform, clip graphics.Rectangle, draw func(graphics.Painter)) image.Image {
	t.Helper()
	var d testDrawer
	painter, err := NewPainter(&d)
	if err != nil {
		t.Fatal(err)
	}
	painter.Begin(float32(width), float32(height), scale)
	painter.Clear(graphics.Color{})
	painter.SetTransform(transform)
	if clip != (graphics.Rectangle{}) {
		painter.SetClipRect(clip)
	}
	draw(painter)
	painter.End()
	return d.result
}

func assertAlphaRange(t *testing.T, actual color.Color, minAlpha, maxAlpha uint8) {
	t.Helper()
	_, _, _, alpha := actual.RGBA()
	got := uint8(alpha >> 8)
	if got < minAlpha || got > maxAlpha {
		t.Fatalf("alpha = %d, want [%d,%d]", got, minAlpha, maxAlpha)
	}
}

func renderGradient(t *testing.T, gradient graphics.LinearGradient) image.Image {
	t.Helper()
	var d testDrawer
	painter, err := NewPainter(&d)
	if err != nil {
		t.Fatal(err)
	}
	painter.Begin(10, 1, 1)
	painter.FillRect(graphics.Rect(0, 0, 10, 1), gradient)
	painter.End()
	return d.result
}

func assertColorNear(t *testing.T, actual color.Color, want color.RGBA, tolerance uint8) {
	t.Helper()
	r, g, b, a := actual.RGBA()
	got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	for _, value := range []struct{ got, want uint8 }{{got.R, want.R}, {got.G, want.G}, {got.B, want.B}, {got.A, want.A}} {
		if uint8(math.Abs(float64(value.got)-float64(value.want))) > tolerance {
			t.Fatalf("color = %+v, want %+v (tolerance %d)", got, want, tolerance)
		}
	}
}

func (d *testDrawer) Draw(img image.Image) error {
	d.result = img
	return nil
}

func TestPainter(t *testing.T) {
	var d testDrawer
	painter, err := NewPainter(&d)
	if err != nil {
		t.Fatal(err)
	}

	painter.Begin(800, 600, 2.0)
	painter.Clear(graphics.RGBA(90, 160, 200, 255))
	painter.FillRoundRect(graphics.Rect(50, 50, 100, 60), 12, graphics.RGBA(90, 50, 50, 255))
	painter.DrawPath(graphics.MoveTo(200, 50).QuadBezierTo(250, 100, 300, 50), 2, graphics.RGBA(100, 0, 0, 255))
	painter.DrawPath(graphics.MoveTo(310, 50).LineTo(360, 50).ArcTo(20, 20, 0, 0, 0, 380, 70), 2, graphics.RGBA(0, 100, 0, 255))
	painter.DrawEllipse(graphics.Point{480, 100}, 50, 50, 2, graphics.RGBA(50, 130, 60, 255))
	painter.FillEllipse(graphics.Point{480, 100}, 30, 30, graphics.RGBA(50, 50, 130, 255))
	painter.DrawLine(graphics.Point{480 - 50, 100}, graphics.Point{480 + 50, 100}, 2, graphics.RGB(130, 0, 0))
	painter.DrawLine(graphics.Point{480, 100 - 50}, graphics.Point{480, 100 + 50}, 2, graphics.RGB(130, 0, 0))
	painter.DrawRoundRect(graphics.Rect(430, 200, 260, 180), 12, 4, graphics.RGB(30, 100, 30))
	painter.DrawRect(graphics.Rect(450, 220, 220, 140), 4, graphics.RGB(30, 100, 30))
	painter.End()

	var buf bytes.Buffer
	err = png.Encode(&buf, d.result)
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile("output.png", buf.Bytes(), 0666)
}
