package coretext

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
			Family: ".AppleSystemUIFont",
			Size:   32,
		},
		WrapMode:  typography.WrapWordChar,
		TextAlign: typography.TextAlignCenter,
		TextColor: color.RGBA{B: 160, A: 255},
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

func TestTextLayoutRasterScaleDoesNotAccumulate(t *testing.T) {
	c, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Destroy()

	format := typography.TextFormat{
		Font: typography.FontInfo{
			Family: ".AppleSystemUIFont",
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

	// The layout-owned retained CGContext must start every rasterization with an
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

func TestTextLayoutsOwnRasterResources(t *testing.T) {
	c, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Destroy()

	format := typography.TextFormat{
		Font:      typography.FontInfo{Family: ".AppleSystemUIFont", Size: 18},
		WrapMode:  typography.WrapNone,
		TextAlign: typography.TextAlignBegin,
		TextColor: color.Black,
	}
	firstLayout, err := c.NewTextLayout("first", format, 100, 40)
	if err != nil {
		t.Fatal(err)
	}
	defer firstLayout.Destroy()
	secondLayout, err := c.NewTextLayout("a much larger second layout", format, 600, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer secondLayout.Destroy()

	first := firstLayout.(*TextLayout)
	second := secondLayout.(*TextLayout)
	want, err := first.Rasterize(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	firstWidth, firstHeight := first.painter.width, first.painter.height
	if _, err = second.Rasterize(2, nil); err != nil {
		t.Fatal(err)
	}
	if second.painter.width <= 0 || second.painter.height <= 0 {
		t.Fatal("second layout did not initialize its raster resources")
	}
	got, err := first.Rasterize(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertSameBitmap(t, want, got)
	if first.painter.width != firstWidth || first.painter.height != firstHeight {
		t.Fatal("rasterizing another layout changed the first layout's raster resources")
	}
}

func TestTextLayoutBitmapRespectsSize(t *testing.T) {
	c, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Destroy()

	format := typography.TextFormat{
		Font:      typography.FontInfo{Family: ".AppleSystemUIFont", Size: 32},
		WrapMode:  typography.WrapNone,
		TextAlign: typography.TextAlignBegin,
		TextColor: color.Black,
	}
	layout, err := c.NewTextLayout("a long line that exceeds its bounds", format, 40, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()

	bitmap, err := layout.Rasterize(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bitmap.Width > 40 || bitmap.Height > 20 {
		t.Fatalf("bitmap exceeds layout size: got %dx%d, max 40x20", bitmap.Width, bitmap.Height)
	}
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
