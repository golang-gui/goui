package directwrite

import (
	"bytes"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/win32/sdk/com"
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
		WordWrap: typography.WrapWordChar,
	}

	text := "abc中国中文👨‍👩‍👧‍👦 مشروع "
	layout, err := c.NewTextLayout(text, format, 200, 300)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()

	layout.SetAttribute(typography.TextAttribute{
		Start:  0,
		Length: len(text),
		Type:   typography.TextBgColor,
		Value:  graphics.RGBA(60, 0, 0, 255),
	})

	width, height := layout.Measure()
	t.Logf("%fx%f", width, height)

	lines, runs := layout.GetLineRuns()
	t.Logf("lines=%d runs=%d", len(lines), len(runs))

	bitmap, err := layout.(*TextLayout).Render(graphics.Color{R: 1, G: 1, B: 1, A: 1}, nil)
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
