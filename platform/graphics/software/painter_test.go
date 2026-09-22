package software

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/srwiley/scanFT"
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
	if color.A != 0.5 || color.R != 0.5 || color.G != 0 || color.B != 0 {
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

func TestImageDestroyDuringActiveFrame(t *testing.T) {
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
	painter.DrawImage(graphics.Rect(0, 0, 1, 1), resource)
	assertPanics(t, func() { _ = resource.Update(image.NewRGBA(image.Rect(0, 0, 1, 1))) })
	resource.Destroy()
	resource.Destroy()
	native := resource.(*imageResource)
	if !native.destroyed || !native.pendingDestroy || native.bitmap.Pixels == nil || painter.(*Painter).pendingImages != 1 {
		t.Fatal("image must be invalidated immediately, with storage retained until End")
	}
	if err := resource.Update(image.NewRGBA(image.Rect(0, 0, 1, 1))); err == nil {
		t.Fatal("destroyed image accepted update")
	}
	assertPanics(t, func() { painter.DrawImage(graphics.Rect(0, 0, 1, 1), resource) })
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
			Color: graphics.ColorOf(color.NRGBA{R: 255, A: 128}), BlurRadius: 4,
		})
		p.DrawBoxShadow(graphics.Rect(10, 10, 20, 10), 0, graphics.BoxShadow{
			Color: graphics.ColorOf(color.NRGBA{B: 255, A: 128}), Offset: graphics.Point{X: 2}, BlurRadius: 4,
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

func (d *testDrawer) Transparent() bool {
	return false
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

type renderImageSurface struct{ presents int }

func (s *renderImageSurface) Transparent() bool      { return true }
func (s *renderImageSurface) Draw(image.Image) error { s.presents++; return nil }

func TestRenderImage(t *testing.T) {
	surface := &renderImageSurface{}
	backend, _ := NewPainter(surface)
	p := backend.(*Painter)
	defer p.Destroy()
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for i := 0; i < len(src.Pix); i += 4 {
		src.Pix[i], src.Pix[i+3] = 128, 128
	}
	native, err := p.NewImage(src)
	if err != nil {
		t.Fatal(err)
	}
	defer native.Destroy()
	// A real surface frame first: offscreen rendering must not overwrite it.
	p.Begin(20, 16, 1)
	p.Clear(graphics.RGB(0, 255, 0))
	p.End()
	for _, scale := range []float32{1, 2} {
		img, err := p.RenderImage(int(12*scale), int(10*scale), scale, func() {
			p.SetClipRect(graphics.Rect(2, 2, 5, 5))
			p.SetTransform(geometry.Translate(2, 2))
			p.DrawImage(graphics.Rect(0, 0, 8, 8), native)
			if _, err := p.RenderImage(1, 1, 1, func() {}); err == nil {
				t.Fatal("nested rendering accepted")
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		// Interior samples avoid antialiasing. RGBA is premultiplied.
		for _, tc := range []struct {
			x, y int
			want color.RGBA
		}{
			{0, 0, color.RGBA{}}, {3, 3, color.RGBA{R: 128, A: 128}}, {8, 3, color.RGBA{}},
		} {
			got := color.RGBAModel.Convert(img.At(tc.x*int(scale), tc.y*int(scale))).(color.RGBA)
			if got != tc.want {
				t.Fatalf("scale=%g at %d,%d: %v want %v", scale, tc.x, tc.y, got, tc.want)
			}
		}
		if p.bgra.Pix[1] != 255 || p.bgra.Rect.Dx() != 20 || p.scale != 1 {
			t.Fatal("surface target changed")
		}
		_, err = p.RenderImage(12, 10, 1, func() { p.Clear(graphics.RGB(0, 0, 255)) })
		if err != nil {
			t.Fatal(err)
		}
		if img.At(0, 0) != (color.RGBA{}) {
			t.Fatal("returned image aliases reusable storage")
		}
	}
	if surface.presents != 1 {
		t.Fatalf("offscreen rendering presented %d times", surface.presents)
	}
	p.Begin(20, 16, 1)
	p.Clear(graphics.Color{})
	p.DrawImage(graphics.Rect(0, 0, 2, 2), native)
	p.End()
	if p.bgra.Pix[2] != 128 || p.bgra.Pix[3] != 128 {
		t.Fatal("cached image no longer usable on surface")
	}
}

func TestRenderImagePanicAndImageLifetime(t *testing.T) {
	backend, _ := NewPainter(&renderImageSurface{})
	p := backend.(*Painter)
	defer p.Destroy()
	var retained graphics.Image
	func() {
		defer func() {
			if recover() != "paint failure" {
				t.Fatal("paint panic not propagated")
			}
		}()
		_, _ = p.RenderImage(8, 8, 1, func() {
			retained, _ = p.NewImage(image.NewRGBA(image.Rect(0, 0, 2, 2)))
			retired, _ := p.NewImage(image.NewRGBA(image.Rect(0, 0, 2, 2)))
			p.DrawImage(graphics.Rect(0, 0, 2, 2), retired)
			retired.Destroy()
			panic("paint failure")
		})
	}()
	if p.activeFrame || p.pendingImages != 0 || len(p.images) != 1 {
		t.Fatal("panic did not clean frame or removed retained image")
	}
	if _, err := p.RenderImage(8, 8, 1, func() { p.DrawImage(graphics.Rect(0, 0, 2, 2), retained) }); err != nil {
		t.Fatal(err)
	}
	retained.Destroy()
}

func TestRenderImageInvalidArguments(t *testing.T) {
	p, _ := NewPainter(&renderImageSurface{})
	defer p.Destroy()
	for _, tc := range []struct {
		w, h  int
		scale float32
	}{
		{0, 2, 1}, {2, -1, 1}, {16385, 1, 1}, {16384, 16384, 1}, {2, 2, 0}, {2, 2, -1}, {2, 2, float32(math.NaN())}, {2, 2, float32(math.Inf(1))},
	} {
		if _, err := p.RenderImage(tc.w, tc.h, tc.scale, func() { t.Fatal("invalid callback invoked") }); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
	if _, err := p.RenderImage(2, 2, 1, nil); err == nil {
		t.Fatal("nil callback accepted")
	}
	p.Begin(2, 2, 1)
	if _, err := p.RenderImage(2, 2, 1, func() {}); err == nil {
		t.Fatal("active frame accepted")
	}
	p.End()
}

type surface struct {
	transparent bool
	pixels      graphics.Bitmap
}

func (s *surface) Transparent() bool { return s.transparent }
func (s *surface) Draw(img image.Image) error {
	s.pixels = graphics.CopyToBitmap(img, graphics.PixelFormatBGRA, nil)
	return nil
}
func pixel(s *surface, x, y int) color.RGBA {
	r, g, b, a := s.pixels.GetPixel(x, y)
	return color.RGBA{r, g, b, a}
}
func reference(src, bg color.Color) color.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, 1, 1))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), image.NewUniform(src), image.Point{}, draw.Over)
	return dst.RGBAAt(0, 0)
}
func near(a, b color.RGBA) bool {
	av, bv := []uint8{a.R, a.G, a.B, a.A}, []uint8{b.R, b.G, b.B, b.A}
	for i, v := range av {
		d := int(v) - int(bv[i])
		if d < -1 || d > 1 {
			return false
		}
	}
	return true
}

// Compare full-coverage pixels with image/draw, independently of the backend.
func TestSoftwareContract(t *testing.T) {
	ink := color.NRGBA{R: 255, A: 128}
	c := graphics.ColorOf(ink)
	rect := graphics.Rect(4, 4, 16, 16)
	path := graphics.MoveTo(4, 4).LineTo(20, 4).LineTo(20, 20).LineTo(4, 20).Close()
	cases := []struct {
		name  string
		x, y  int
		paint func(graphics.Painter)
	}{
		{"Clear", 12, 12, func(p graphics.Painter) { p.Clear(c) }},
		{"FillRect", 12, 12, func(p graphics.Painter) { p.FillRect(rect, c) }},
		{"FillRoundRect", 12, 12, func(p graphics.Painter) { p.FillRoundRect(rect, 3, c) }},
		{"FillEllipse", 12, 12, func(p graphics.Painter) { p.FillEllipse(graphics.Point{X: 12, Y: 12}, 8, 8, c) }},
		{"FillPath", 12, 12, func(p graphics.Painter) { p.FillPath(path, c) }},
		{"DrawLine", 12, 12, func(p graphics.Painter) { p.DrawLine(graphics.Point{X: 4, Y: 12}, graphics.Point{X: 20, Y: 12}, 4, c) }},
		{"DrawRect", 12, 4, func(p graphics.Painter) { p.DrawRect(rect, 4, c) }},
		{"DrawRoundRect", 12, 4, func(p graphics.Painter) { p.DrawRoundRect(rect, 3, 4, c) }},
		{"DrawEllipse", 12, 4, func(p graphics.Painter) { p.DrawEllipse(graphics.Point{X: 12, Y: 12}, 8, 8, 4, c) }},
		{"DrawPath", 12, 4, func(p graphics.Painter) { p.DrawPath(path, 4, c) }},
		{"ConstantGradient", 12, 12, func(p graphics.Painter) {
			p.FillRect(rect, graphics.LinearGradient{Start: graphics.Point{X: 4}, End: graphics.Point{X: 20}, StartColor: c, EndColor: c})
		}},
		{"DegenerateGradient", 12, 12, func(p graphics.Painter) { p.FillRect(rect, graphics.LinearGradient{StartColor: c, EndColor: c}) }},
		{"ShadowClear", 12, 12, func(p graphics.Painter) { p.DrawBoxShadow(rect, 0, graphics.BoxShadow{Color: c}) }},
		{"ShadowSoftInterior", 12, 12, func(p graphics.Painter) { p.DrawBoxShadow(rect, 0, graphics.BoxShadow{Color: c, BlurRadius: 1}) }},
		{"DrawImage", 12, 12, func(p graphics.Painter) {
			src := image.NewNRGBA(image.Rect(0, 0, 16, 16))
			draw.Draw(src, src.Bounds(), image.NewUniform(ink), image.Point{}, draw.Src)
			img, e := p.NewImage(src)
			if e != nil {
				panic(e)
			}
			p.DrawImage(rect, img) /* released with painter after End */
		}},
	}
	for _, scale := range []float32{1, 2} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/%gx", tc.name, scale), func(t *testing.T) {
				s := &surface{transparent: true}
				p, e := NewPainter(s)
				if e != nil {
					t.Fatal(e)
				}
				defer p.Destroy()
				p.Begin(32*scale, 32*scale, scale)
				p.Clear(graphics.Color{})
				tc.paint(p)
				p.End()
				got, want := pixel(s, tc.x*int(scale), tc.y*int(scale)), reference(ink, color.Transparent)
				if !near(got, want) {
					t.Errorf("premultiplied RGBA got=%v want=%v", got, want)
				}
			})
		}
	}
}

func TestOpacityAndBackground(t *testing.T) {
	for _, transparent := range []bool{true, false} {
		for _, alpha := range []uint8{0, 64, 128, 192, 255} {
			ink := color.NRGBA{R: 255, A: alpha}
			s := &surface{transparent: transparent}
			p, e := NewPainter(s)
			if e != nil {
				t.Fatal(e)
			}
			p.Begin(16, 16, 1)
			p.Clear(graphics.RGB(255, 255, 255))
			p.FillRect(graphics.Rect(0, 0, 16, 16), graphics.ColorOf(ink))
			p.End()
			got, want := pixel(s, 8, 8), reference(ink, color.White)
			p.Destroy()
			t.Logf("surfaceTransparent=%v alpha=%d over white: got=%v want=%v", transparent, alpha, got, want)
			if !near(got, want) {
				t.Errorf("incorrect source-over")
			}
		}
	}
}

func TestVaryingGradient(t *testing.T) {
	// At t=0.5 interpolate premultiplied (64,0,0,64) and (0,0,192,192).
	for _, scale := range []float32{1, 2} {
		s := &surface{transparent: true}
		p, e := NewPainter(s)
		if e != nil {
			t.Fatal(e)
		}
		offset := .5 / scale
		p.Begin(32*scale, 32*scale, scale)
		p.Clear(graphics.Color{})
		p.FillRect(graphics.Rect(4, 4, 16, 16), graphics.LinearGradient{
			Start: graphics.Point{X: 4 + offset}, End: graphics.Point{X: 20 + offset},
			StartColor: graphics.ColorOf(color.NRGBA{R: 255, A: 64}), EndColor: graphics.ColorOf(color.NRGBA{B: 255, A: 192}),
		})
		p.End()
		got, want := pixel(s, 12*int(scale), 12*int(scale)), (color.RGBA{R: 32, B: 96, A: 128})
		p.Destroy()
		if !near(got, want) {
			t.Errorf("%gx gradient midpoint got=%v want=%v", scale, got, want)
		}
	}
}

func TestRawRasterizerReference(t *testing.T) {
	for _, bg := range []color.Color{color.Transparent, color.White} {
		dst := image.NewRGBA(image.Rect(0, 0, 1, 1))
		draw.Draw(dst, dst.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
		p := scanFT.NewRGBAPainter(dst)
		p.Op = draw.Over
		ink := color.NRGBA{R: 255, A: 128}
		p.SetColor(graphics.ColorOf(ink))
		p.Paint([]scanFT.Span{{Y: 0, X0: 0, X1: 1, Alpha: 65535}}, true, image.Rectangle{})
		got, want := dst.RGBAAt(0, 0), reference(ink, bg)
		if got != want {
			t.Errorf("raw scanner got=%v want=%v", got, want)
		} else {
			t.Logf("raw scanner matches image/draw: %v", got)
		}
	}
}

// A disjoint intersection must suppress drawing, not become scanFT's zero-size
// "no clip" sentinel. Text selection in the last cached paragraph exposed this:
// wrapped rows outside the viewport painted over the window's status bar.
func TestDisjointShapeClip(t *testing.T) {
	r := graphics.Rect(16, 16, 8, 8)
	start := graphics.Point{X: 16, Y: 16}
	center := graphics.Point{X: 20, Y: 20}
	path := graphics.MoveTo(16, 16).LineTo(24, 16).LineTo(24, 24).LineTo(16, 24).Close()
	red := graphics.RGBA(255, 0, 0, 128)
	for _, tc := range []struct {
		name string
		draw func(graphics.Painter, graphics.Image)
	}{
		{"rect", func(p graphics.Painter, _ graphics.Image) { p.FillRect(r, red) }},
		{"roundrect", func(p graphics.Painter, _ graphics.Image) { p.FillRoundRect(r, 2, red) }},
		{"ellipse", func(p graphics.Painter, _ graphics.Image) { p.FillEllipse(center, 4, 3, red) }},
		{"path", func(p graphics.Painter, _ graphics.Image) { p.FillPath(path, red) }},
		{"line", func(p graphics.Painter, _ graphics.Image) { p.DrawLine(start, center, 2, red) }},
		{"stroke-rect", func(p graphics.Painter, _ graphics.Image) { p.DrawRect(r, 2, red) }},
		{"stroke-roundrect", func(p graphics.Painter, _ graphics.Image) { p.DrawRoundRect(r, 2, 2, red) }},
		{"stroke-ellipse", func(p graphics.Painter, _ graphics.Image) { p.DrawEllipse(center, 4, 3, 2, red) }},
		{"stroke-path", func(p graphics.Painter, _ graphics.Image) { p.DrawPath(path, 2, red) }},
		{"shadow", func(p graphics.Painter, _ graphics.Image) {
			p.DrawBoxShadow(r, 2, graphics.BoxShadow{Color: red, BlurRadius: 2})
		}},
		{"gradient", func(p graphics.Painter, _ graphics.Image) {
			p.FillRect(r, graphics.LinearGradient{Start: start, End: center, StartColor: red, EndColor: graphics.RGB(0, 255, 0)})
		}},
		{"image", func(p graphics.Painter, img graphics.Image) { p.DrawImage(r, img) }},
	} {
		for _, scale := range []float32{1, 2} {
			for _, rotated := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%gx/rotated=%v", tc.name, scale, rotated), func(t *testing.T) {
					var d testDrawer
					p, err := NewPainter(&d)
					if err != nil {
						t.Fatal(err)
					}
					defer p.Destroy()
					src := image.NewRGBA(image.Rect(0, 0, 1, 1))
					src.SetRGBA(0, 0, color.RGBA{R: 128, A: 128})
					img, err := p.NewImage(src)
					if err != nil {
						t.Fatal(err)
					}
					defer img.Destroy()
					for _, clipped := range []bool{true, false} {
						p.Begin(48*scale, 48*scale, scale)
						p.Clear(graphics.RGB(0, 0, 255))
						if clipped {
							p.SetClipRect(graphics.Rect(2, 2, 6, 6))
						}
						if rotated {
							p.SetTransform(geometry.Translate(4, 4).Rotate(8).Scale(1.1, 1.2))
						}
						tc.draw(p, img)
						if clipped {
							// A skipped shape must not leave path segments or scanner
							// state that corrupt the next draw in the same frame.
							p.SetTransform(geometry.Identity())
							p.FillRect(graphics.Rect(2, 2, 2, 2), graphics.RGB(0, 255, 0))
						}
						p.End()
						// Outside the green control rectangle, no pixel may change.
						// No rasterization or color tolerance applies: the opaque blue
						// destination must remain byte-exact (premultiplied sRGB bytes).
						changed := 0
						for y := 0; y < int(48*scale); y++ {
							for x := 0; x < int(48*scale); x++ {
								want := color.RGBA{B: 255, A: 255}
								if clipped && x >= int(2*scale) && x < int(4*scale) && y >= int(2*scale) && y < int(4*scale) {
									want = color.RGBA{G: 255, A: 255}
								}
								if color.RGBAModel.Convert(d.result.At(x, y)) != want {
									changed++
								}
							}
						}
						if clipped && changed != 0 || !clipped && changed == 0 {
							t.Fatalf("clipped=%v: %d changed pixels", clipped, changed)
						}
					}
				})
			}
		}
	}
}

type lifetimeDrawer struct {
	transparent bool
	draw        func(image.Image) error
}

func (d *lifetimeDrawer) Transparent() bool          { return d.transparent }
func (d *lifetimeDrawer) Draw(img image.Image) error { return d.draw(img) }

func TestImageReleaseOnPresentFailure(t *testing.T) {
	for _, transparent := range []bool{false, true} {
		for _, panics := range []bool{false, true} {
			d := &lifetimeDrawer{transparent: transparent}
			native, err := NewPainter(d)
			if err != nil {
				t.Fatal(err)
			}
			p := native.(*Painter)
			img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 2, 2)))
			if err != nil {
				t.Fatal(err)
			}
			resource := img.(*imageResource)
			want := errors.New("present failed")
			d.draw = func(image.Image) error {
				if !resource.pendingDestroy || resource.bitmap.Pixels == nil {
					t.Fatal("resource released before presentation")
				}
				if panics {
					panic(want)
				}
				return want
			}
			p.Begin(2, 2, 1)
			img.Destroy()
			func() {
				defer func() {
					got, ok := recover().(error)
					if !ok || !errors.Is(got, want) {
						t.Fatalf("lost present failure: %v", got)
					}
				}()
				p.End()
			}()
			if p.activeFrame || p.pendingImages != 0 || len(p.images) != 0 || resource.owner != nil || resource.bitmap.Pixels != nil || resource.pendingDestroy {
				t.Fatal("failed End retained image or frame state")
			}
			img.Destroy()
			p.Destroy()
		}
	}
}

func TestRecordedImageSurvivesDestroyAndReplacement(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		d := &alphaDrawer{}
		p, err := NewPainter(d)
		if err != nil {
			t.Fatal(err)
		}
		p.Begin(16*scale, 8*scale, scale)
		p.Clear(graphics.Color{})
		for n, ink := range []color.RGBA{{R: 128, A: 128}, {B: 128, A: 128}} {
			src := image.NewRGBA(image.Rect(0, 0, 2, 2))
			for i := 0; i < len(src.Pix); i += 4 {
				copy(src.Pix[i:], []byte{ink.R, ink.G, ink.B, ink.A})
			}
			img, err := p.NewImage(src)
			if err != nil {
				t.Fatal(err)
			}
			p.DrawImage(graphics.Rect(float32(n*8), 0, 8, 8), img)
			img.Destroy()
			img.Destroy()
		}
		p.End()
		// Interior samples, exact premultiplied RGBA; no edge AA involved.
		for n, want := range []color.RGBA{{R: 128, A: 128}, {B: 128, A: 128}} {
			got := color.RGBAModel.Convert(d.pixels.At((n*8+4)*int(scale), 4*int(scale)))
			if got != want {
				t.Fatalf("scale %g: got %v want %v", scale, got, want)
			}
		}
		if len(p.(*Painter).images) != 0 {
			t.Fatal("retired images leaked")
		}
		p.Destroy()
	}
}

func TestBoxShadowPremultipliedPixels(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, blur := range []float32{0, 4} {
			for _, layers := range []int{1, 2} {
				t.Run(fmt.Sprintf("scale=%g/blur=%g/layers=%d", scale, blur, layers), func(t *testing.T) {
					d := &alphaDrawer{}
					p, err := NewPainter(d)
					if err != nil {
						t.Fatal(err)
					}
					defer p.Destroy()
					p.Begin(64*scale, 64*scale, scale)
					p.Clear(graphics.Color{})
					// These are premultiplied RGB values, as produced by ColorOf.
					ink := color.RGBA{R: 96, G: 48, B: 24, A: 128}
					for range layers {
						p.DrawBoxShadow(graphics.Rect(16, 16, 32, 32), 4, graphics.BoxShadow{
							Color: graphics.ColorOf(ink), BlurRadius: blur,
						})
					}
					p.End()
					want := ink
					if layers == 2 {
						want = color.RGBA{R: 144, G: 72, B: 36, A: 192}
					}
					assertColorNear(t, d.pixels.At(int(32*scale), int(32*scale)), want, 2)
					assertAlphaRange(t, d.pixels.At(0, 0), 0, 0)
					// Every feather pixel must preserve the premultiplied ratios.
					// Merely testing alpha or a black shadow cannot catch RGB*A twice.
					feather := 0
					for x := int(8 * scale); x < int(24*scale); x++ {
						r, g, b, a := d.pixels.At(x, int(32*scale)).RGBA()
						if a < 8*257 || a > 120*257 {
							continue
						}
						feather++
						assertColorNear(t, color.RGBA64{R: uint16(r), G: uint16(g), B: uint16(b), A: uint16(a)}, color.RGBA{
							R: uint8(a * 96 / 128 >> 8), G: uint8(a * 48 / 128 >> 8), B: uint8(a * 24 / 128 >> 8), A: uint8(a >> 8),
						}, 2)
					}
					if blur > 0 && feather == 0 {
						t.Fatal("soft shadow has no sampled feather pixels")
					}
				})
			}
		}
	}
}

type alphaDrawer struct{ pixels graphics.Bitmap }

func (*alphaDrawer) Transparent() bool { return true }
func (d *alphaDrawer) Draw(img image.Image) error {
	d.pixels = graphics.CopyToBitmap(img, graphics.PixelFormatBGRA, nil)
	return nil
}

func TestTransparentPresentation(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		d := &alphaDrawer{}
		p, err := NewPainter(d)
		if err != nil {
			t.Fatal(err)
		}
		p.Begin(8*scale, 8*scale, scale)
		p.Clear(graphics.Color{R: .5, G: .25, A: .5})
		p.End()
		// Premultiplied channels must reach the compositor without another RGB*A.
		got := d.pixels.Pixels[:4]
		if got[0] != 0 || got[1] < 63 || got[1] > 64 || got[2] < 127 || got[2] > 128 || got[3] < 127 || got[3] > 128 {
			t.Fatalf("scale %v: premultiplied BGRA = %v", scale, got)
		}
		p.Begin(8*scale, 8*scale, scale)
		// Transparent clear must remove the previous frame's content.
		p.Clear(graphics.Color{})
		p.End()
		if !bytes.Equal(d.pixels.Pixels, make([]byte, len(d.pixels.Pixels))) {
			t.Fatal("transparent clear retained RGB or previous-frame content")
		}
		p.Destroy()
	}
}
