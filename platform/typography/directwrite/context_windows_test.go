package directwrite

import (
	"bytes"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/win32/sdk/com"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func Test_TextLayout(t *testing.T) {
	com.Initialize(0)

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
		WordWrap:  typography.WrapWordChar,
		TextAlign: typography.TextAlignCenter,
		LineAlign: typography.LineAlignCenter,
	}

	text := "abc中国中文👨‍👩‍👧‍👦 مشروع "
	layout, err := c.NewTextLayout(text, format, 200, 50)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()

	x, y, width, height := layout.MeasureRect()
	t.Logf("%f-%f %fx%f", x, y, width, height)

	lines, clusters := layout.MeasureMetrics()
	t.Logf("lines=%d clusters=%d", len(lines), len(clusters))

	bitmap, err := c.DrawTextLayout(layout, color.White, nil)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	buf.Grow(bitmap.Width * bitmap.Height)
	err = png.Encode(&buf, bitmap)
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile("output.png", buf.Bytes(), 0666)
}
