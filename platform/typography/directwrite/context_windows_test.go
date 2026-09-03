package directwrite

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
		TextColor: color.White,
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
	t.Logf("%fx%f", width, height)

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

func TestTextLayoutsOwnRasterResources(t *testing.T) {
	c, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Destroy()

	format := typography.TextFormat{
		Font:      typography.FontInfo{Family: "Microsoft YaHei", Size: 18},
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
	if want.Width != got.Width || want.Height != got.Height || want.Stride != got.Stride || !bytes.Equal(want.Pixels, got.Pixels) {
		t.Fatal("rasterizing another layout changed the first layout's bitmap")
	}
	if first.painter.width != firstWidth || first.painter.height != firstHeight {
		t.Fatal("rasterizing another layout changed the first layout's raster resources")
	}
}
