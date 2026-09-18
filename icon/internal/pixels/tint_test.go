package pixels

import (
	"image"
	"image/color"
	"testing"

	"github.com/golang-gui/goui/gui"
)

func TestTintPremultipliedMask(t *testing.T) {
	src := image.NewNRGBA(image.Rect(7, 9, 9, 10))
	src.SetNRGBA(7, 9, color.NRGBA{G: 255, A: 128})
	dst := Tint(src, gui.Color{R: .5, A: .5})
	if dst.Bounds() != image.Rect(0, 0, 2, 1) {
		t.Fatal("origin was not normalized")
	}
	// 128/255 coverage times 0.5 premultiplied foreground rounds to 64/255.
	if dst.RGBAAt(0, 0) != (color.RGBA{R: 64, A: 64}) || dst.RGBAAt(1, 0) != (color.RGBA{}) {
		t.Fatal("incorrect mask/alpha")
	}
	if src.NRGBAAt(7, 9) != (color.NRGBA{G: 255, A: 128}) {
		t.Fatal("input pixels were modified")
	}
}
