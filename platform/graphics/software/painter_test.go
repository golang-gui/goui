package software

import (
	"bytes"
	"github.com/golang-gui/goui/platform/graphics"
	"image"
	"image/png"
	"os"
	"testing"
)

type testDrawer struct {
	result image.Image
}

func (d *testDrawer) Draw(img image.Image) error {
	d.result = img
	return nil
}

func TestPainter(t *testing.T) {
	var d testDrawer
	painter, err := NewPainter(&d, nil)
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
