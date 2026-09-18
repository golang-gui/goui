package draw

import (
	"image"
	"image/color"
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/graphics"
)

func TestSourceImage(t *testing.T) {
	var _ gui.IconSource = &Source{}
	var drawnSize graphics.Size
	s := &Source{Draw: func(p graphics.Painter, size graphics.Size, c graphics.Color) {
		drawnSize = size
		p.SetClipRect(graphics.Rect(0, 0, size.Width/2, size.Height))
		p.FillRect(graphics.Rect(0, 0, size.Width, size.Height), c)
	}}
	for _, size := range []image.Point{{X: 16, Y: 16}, {X: 32, Y: 32}, {X: 24, Y: 40}, {X: 40, Y: 24}} {
		img := s.Image(size.X, size.Y, graphics.Color{R: .5, A: .5})
		if img == nil {
			t.Fatal("missing image")
		}
		if want := (graphics.Size{Width: float32(size.X), Height: float32(size.Y)}); drawnSize != want {
			t.Fatalf("Draw size = %v, want %v", drawnSize, want)
		}
		if want := image.Rect(0, 0, size.X, size.Y); img.Bounds() != want {
			t.Fatalf("image bounds = %v, want %v", img.Bounds(), want)
		}
		// Sample flat interiors, away from clip edges: exact premultiplied
		// RGBA, with half-opacity red on the left and transparent on the right.
		if got := color.RGBAModel.Convert(img.At(2, 2)); got != (color.RGBA{R: 128, A: 128}) {
			t.Fatal(got)
		}
		if got := color.RGBAModel.Convert(img.At(2, size.Y-2)); got != (color.RGBA{R: 128, A: 128}) {
			t.Fatal("height coverage:", got)
		}
		if got := color.RGBAModel.Convert(img.At(size.X-2, 2)); got != (color.RGBA{}) {
			t.Fatal("clip:", got)
		}
		// Subsequent draws and Painter.Destroy must not change retained pixels.
		if s.Image(size.X, size.Y, graphics.Color{B: 1, A: 1}) == nil {
			t.Fatal("missing image")
		}
		if got := color.RGBAModel.Convert(img.At(2, 2)); got != (color.RGBA{R: 128, A: 128}) {
			t.Fatal("retained output changed")
		}
	}
}
