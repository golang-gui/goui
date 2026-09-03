package pango

import (
	"bytes"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/golang-gui/goui/platform/typography"
)

func Test_TextLayout(t *testing.T) {
	c, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Destroy()

	format := typography.TextFormat{
		Font: typography.FontInfo{
			Family: "Microsoft YaHei",
			Size:   32,
		},
		WrapMode:  typography.WrapWordChar,
		TextAlign: typography.TextAlignCenter,
	}

	text := "abc中国中文👨‍👩‍👧‍👦 مشروع "
	layout, err := c.NewTextLayout(text, format, 200, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()

	layout.SetTextFont(0, 3, typography.FontInfo{
		Family: "mono sans",
		Size:   24,
	})
	layout.SetTextColor(0, 3, color.RGBA{R: 160, A: 255})
	layout.SetUnderline(0, 1, true)
	layout.SetStrikethrough(1, 2, true)

	width, height := layout.MeasureSize()
	t.Logf("size=%fx%f", width, height)

	lines, clusters := layout.MeasureMetrics()
	t.Logf("lines=%d clusters=%d", len(lines), len(clusters))

	bitmap, err := layout.Rasterize(2.0, nil)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	buf.Grow(bitmap.Width * bitmap.Height)
	err = png.Encode(&buf, bitmap)
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile("text.png", buf.Bytes(), 0666)
}

func TestTextLayoutEmptyMeasureMetrics(t *testing.T) {
	c, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Destroy()

	format := typography.TextFormat{
		Font: typography.FontInfo{
			Family: "Microsoft YaHei",
			Size:   32,
		},
		WrapMode: typography.WrapWordChar,
	}

	layout, err := c.NewTextLayout("", format, 200, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()

	_, clusters := layout.MeasureMetrics()
	if len(clusters) != 0 {
		t.Fatalf("empty text should not produce clusters, got %d", len(clusters))
	}
	bitmap, err := layout.Rasterize(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bitmap.Width != 0 || bitmap.Height != 0 || len(bitmap.Pixels) != 0 {
		t.Fatalf("empty text raster = %dx%d/%d bytes", bitmap.Width, bitmap.Height, len(bitmap.Pixels))
	}
}

func TestTextLayoutsOwnRasterResourcesAndDoNotLeakPixels(t *testing.T) {
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	small, err := ctx.NewTextLayout("small", typography.TextFormat{
		Font: typography.FontInfo{Family: "sans", Size: 12},
	}, 100, 40)
	if err != nil {
		t.Fatal(err)
	}
	defer small.Destroy()
	smallNative := small.(*TextLayout)
	first, err := small.Rasterize(1, nil)
	if err != nil {
		t.Fatal(err)
	}

	large, err := ctx.NewTextLayout("a much larger layout used to grow the shared raster surface", typography.TextFormat{
		Font: typography.FontInfo{Family: "sans", Size: 48},
	}, 1200, 200)
	if err != nil {
		t.Fatal(err)
	}
	defer large.Destroy()
	largeNative := large.(*TextLayout)
	if smallNative.context.GObject == largeNative.context.GObject {
		t.Fatal("text layouts unexpectedly share a PangoContext")
	}
	smallWidth, smallHeight := smallNative.painter.width, smallNative.painter.height
	if _, err = large.Rasterize(2, nil); err != nil {
		t.Fatal(err)
	}
	if largeNative.painter.width <= 0 || largeNative.painter.height <= 0 {
		t.Fatal("large layout did not initialize its raster resources")
	}

	second, err := small.Rasterize(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertSameBitmap(t, first, second)
	if smallNative.painter.width != smallWidth || smallNative.painter.height != smallHeight {
		t.Fatal("rasterizing another layout changed the first layout's raster resources")
	}
}

func TestTextLayoutRasterScaleDoesNotAccumulate(t *testing.T) {
	c, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Destroy()

	format := typography.TextFormat{
		Font: typography.FontInfo{
			Family: "sans",
			Size:   18,
		},
		WrapMode:  typography.WrapNone,
		TextAlign: typography.TextAlignBegin,
		TextColor: color.Black,
	}

	layout, err := c.NewTextLayout("scale", format, 200, 80)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()

	first2x, err := layout.Rasterize(2, nil)
	if err != nil {
		t.Fatal(err)
	}

	// The layout-owned retained cairo context must start every rasterization with an
	// identity CTM, even when drawing the same layout again.
	layout.SetTextAlignment(format.TextAlign)
	second2x, err := layout.Rasterize(2, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertSameBitmap(t, first2x, second2x)

	actual1x, err := layout.Rasterize(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := c.NewTextLayout("scale", format, 200, 80)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Destroy()
	want1x, err := fresh.Rasterize(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertSameBitmap(t, want1x, actual1x)
}

func assertSameBitmap(t *testing.T, want, got typography.TextBitmap) {
	t.Helper()
	if want.Width != got.Width || want.Height != got.Height || want.Stride != got.Stride {
		t.Fatalf("bitmap dimensions differ: want=%dx%d/%d got=%dx%d/%d",
			want.Width, want.Height, want.Stride, got.Width, got.Height, got.Stride)
	}
	if !bytes.Equal(want.Pixels, got.Pixels) {
		t.Fatal("bitmap pixels differ")
	}
}
