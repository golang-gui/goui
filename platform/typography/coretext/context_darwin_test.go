package coretext

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"

	"github.com/golang-gui/goui/platform/typography"
)

type testDrawable struct {
	result image.Image
}

func (d *testDrawable) Draw(img image.Image) error {
	d.result = img
	return nil
}

func Test_TextLayout(t *testing.T) {
	runtime.LockOSThread()

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
	}

	text := "abc中国中文👨‍👩‍👧‍👦 مشروع "
	layout, err := c.NewTextLayout(text, format, 500, 200)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()

	x, y, width, height := layout.MeasureRect()
	t.Logf("%f-%f %fx%f", x, y, width, height)

	lines, clusters := layout.MeasureMetrics()
	t.Logf("lines=%d clusters=%d", len(lines), len(clusters))

	var drawable testDrawable
	painter, err := software.NewPainter(&drawable, c)
	if err != nil {
		t.Fatal(err)
	}

	painter.Begin(400, 300)
	painter.Clear(graphics.RGB(180, 180, 180))
	painter.DrawRect(graphics.Rect(x, y, width, height), 1, graphics.RGB(0, 160, 0))
	painter.DrawTextLayout(graphics.Point{}, layout, graphics.RGB(0, 0, 60))
	for _, cluster := range clusters {
		rect := graphics.Rect(cluster.X, cluster.Y, cluster.Width, cluster.Height)
		painter.DrawRect(rect, 1, graphics.RGB(160, 0, 0))
	}
	//for _, line := range lines {
	//	//p0 := geometry.Point{line.X, line.Y}
	//	//p1 := geometry.Point{line.X + line.Width, line.Y}
	//	//painter.DrawLine(p0, p1, 1, graphics.RGB(160, 0, 0))
	//	rect := graphics.Rect(line.X, line.Y, line.Width, line.Height)
	//	painter.DrawRect(rect, 1, graphics.RGB(160, 0, 0))
	//}

	painter.End()

	//bitmap, err := c.DrawTextLayout(layout, color.White, nil)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//t.Logf("bitmap=%dx%d", bitmap.Width, bitmap.Height)

	var buf bytes.Buffer
	err = png.Encode(&buf, drawable.result)
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile("output.png", buf.Bytes(), 0666)
}
