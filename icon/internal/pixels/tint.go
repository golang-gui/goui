package pixels

import (
	"image"

	"github.com/golang-gui/goui/gui"
)

// Tint colors src's alpha mask with a premultiplied foreground. It returns
// independent, origin-zero RGBA pixels and does not modify or retain src.
func Tint(src image.Image, foreground gui.Color) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			_, _, _, a := src.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			coverage := float32(a) / 65535
			o := dst.PixOffset(x, y)
			dst.Pix[o] = byte(foreground.R*coverage*255 + .5)
			dst.Pix[o+1] = byte(foreground.G*coverage*255 + .5)
			dst.Pix[o+2] = byte(foreground.B*coverage*255 + .5)
			dst.Pix[o+3] = byte(foreground.A*coverage*255 + .5)
		}
	}
	return dst
}
