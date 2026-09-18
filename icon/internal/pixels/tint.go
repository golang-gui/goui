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
	// RGBA and NRGBA store the same 8-bit alpha in the fourth byte. Read it
	// directly: image.Image.At boxes a color value for every pixel. Pix already
	// starts at Rect.Min, including for subimages; rows may have extra stride.
	switch src := src.(type) {
	case *image.RGBA:
		tintAlpha8(dst, src.Pix, src.Stride, foreground)
		return dst
	case *image.NRGBA:
		tintAlpha8(dst, src.Pix, src.Stride, foreground)
		return dst
	}
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

func tintAlpha8(dst *image.RGBA, src []byte, stride int, foreground gui.Color) {
	for y := 0; y < dst.Rect.Dy(); y++ {
		out := dst.Pix[y*dst.Stride : (y+1)*dst.Stride]
		in := src[y*stride : y*stride+len(out)]
		for x := 0; x < len(out); x += 4 {
			coverage := float32(in[x+3]) / 255
			out[x] = byte(foreground.R*coverage*255 + .5)
			out[x+1] = byte(foreground.G*coverage*255 + .5)
			out[x+2] = byte(foreground.B*coverage*255 + .5)
			out[x+3] = byte(foreground.A*coverage*255 + .5)
		}
	}
}
