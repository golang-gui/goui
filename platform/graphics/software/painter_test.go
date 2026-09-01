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
	"github.com/golang-gui/goui/platform/graphics"
)

type testDrawer struct {
	result image.Image
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
	painter, err := NewPainter(&d, nil)
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
	painter, err := NewPainter(&d, nil)
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
	painter, err := NewPainter(&d, nil)
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
	painter, err := NewPainter(&d, nil)
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

func TestPainterCompositesTransparentBrushAndPreservesClip(t *testing.T) {
	var d testDrawer
	painter, err := NewPainter(&d, nil)
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

func TestBoxShadowFullAndSingleBottomEdge(t *testing.T) {
	img := renderShadow(t, 40, 35, 1, geometry.Identity(), graphics.Rectangle{}, func(p graphics.Painter) {
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

	img = renderShadow(t, 40, 35, 1, geometry.Identity(), graphics.Rectangle{}, func(p graphics.Painter) {
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
	img := renderShadow(t, 60, 40, 1, geometry.Identity(), graphics.Rect(15, 0, 30, 40), func(p graphics.Painter) {
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

	img = renderShadow(t, 80, 50, 2, geometry.Translate(10, 10).Rotate(90), graphics.Rectangle{}, func(p graphics.Painter) {
		p.DrawBoxShadow(graphics.Rect(0, -4, 8, 4), 0, graphics.BoxShadow{
			Color: graphics.RGBA(0, 255, 0, 255), BlurRadius: 1,
		})
	})
	// Local (4,-2) rotates clockwise to (-2,-4), translates to (8,6),
	// then scales to device pixel (16,12).
	assertAlphaRange(t, img.At(16, 12), 240, 255)
	assertAlphaRange(t, img.At(5, 5), 0, 2)

	img = renderShadow(t, 50, 30, 1, geometry.Translate(5, 5).Scale(2, 1), graphics.Rectangle{}, func(p graphics.Painter) {
		p.DrawBoxShadow(graphics.Rect(0, 0, 5, 5), 0, graphics.BoxShadow{
			Color: graphics.RGBA(0, 255, 0, 255), BlurRadius: 1,
		})
	})
	assertAlphaRange(t, img.At(10, 7), 240, 255)
	assertAlphaRange(t, img.At(2, 7), 0, 2)
}

func TestBoxShadowFractionalHiDPI(t *testing.T) {
	img := renderShadow(t, 40, 30, 1.25, geometry.Identity(), graphics.Rectangle{}, func(p graphics.Painter) {
		p.DrawBoxShadow(graphics.Rect(8, 6, 8, 8), 3, graphics.BoxShadow{
			Color: graphics.RGBA(20, 40, 80, 200), BlurRadius: 2,
		})
	})
	assertAlphaRange(t, img.At(15, 12), 185, 200)
	assertAlphaRange(t, img.At(2, 12), 0, 0)
}

func TestBoxShadowTransparentColorAndContractedEmptyAreNoOps(t *testing.T) {
	img := renderShadow(t, 20, 20, 1, geometry.Identity(), graphics.Rectangle{}, func(p graphics.Painter) {
		p.DrawBoxShadow(graphics.Rect(4, 4, 8, 8), 2, graphics.BoxShadow{
			Color: graphics.RGBA(255, 0, 0, 0), BlurRadius: 4,
		})
		p.DrawBoxShadow(graphics.Rect(4, 4, 8, 8), 2, graphics.BoxShadow{
			Color: graphics.RGB(255, 0, 0), SpreadRadius: -4,
		})
	})
	assertAlphaRange(t, img.At(8, 8), 0, 2)
}

func renderShadow(t *testing.T, width, height int, scale float32, transform geometry.Transform, clip graphics.Rectangle, draw func(graphics.Painter)) image.Image {
	t.Helper()
	var d testDrawer
	painter, err := NewPainter(&d, nil)
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
	painter, err := NewPainter(&d, nil)
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
