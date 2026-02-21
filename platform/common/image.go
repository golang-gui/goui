package common

import (
	"image"
	"image/color"

	"github.com/golang-gui/goui/platform/internal"
)

type Image interface {
	IsImage() internal.Tag
	Destroy()
	image.Image
	Set(x, y int, c color.Color)
}

type BGRAImage struct {
	Pix    []uint8
	Stride int
	Rect   image.Rectangle
}

func NewBGRAImage(r image.Rectangle) *BGRAImage {
	return &BGRAImage{
		Pix:    make([]byte, r.Dx()*r.Dy()*4),
		Stride: 4 * r.Dx(),
		Rect:   r,
	}
}

func ToBGRAImage(src image.Image) (dst *BGRAImage) {
	if img, ok := src.(*BGRAImage); ok {
		return img
	}
	bounds := src.Bounds()
	dst = NewBGRAImage(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.Set(x, y, src.At(x, y))
		}
	}
	return dst
}

func (img *BGRAImage) IsImage() internal.Tag {
	return 0
}

func (img *BGRAImage) Destroy() {

}

func (img *BGRAImage) ColorModel() color.Model {
	return BGRAModel
}

func (img *BGRAImage) Bounds() image.Rectangle {
	return img.Rect
}

func (img *BGRAImage) At(x, y int) color.Color {
	if !image.Pt(x, y).In(img.Rect) {
		return color.RGBA{}
	}
	i := img.PixOffset(x, y)
	s := img.Pix[i : i+4 : i+4]
	return color.RGBA{
		R: s[2],
		G: s[1],
		B: s[0],
		A: s[3],
	}
}

func (img *BGRAImage) Set(x, y int, c color.Color) {
	if !image.Pt(x, y).In(img.Rect) {
		return
	}
	i := img.PixOffset(x, y)
	c1 := color.RGBAModel.Convert(c).(color.RGBA)
	s := img.Pix[i : i+4 : i+4]
	s[0] = c1.B
	s[1] = c1.G
	s[2] = c1.R
	s[3] = c1.A
}

//func (img *BGRAImage) SubImage(r image.Rectangle) image.Image {
//	r = r.Intersect(img.Rect)
//	if r.Empty() {
//		return &BGRAImage{}
//	}
//	i := img.PixOffset(r.Min.X, r.Min.Y)
//	return &BGRAImage{
//		Pix:    img.Pix[i:],
//		Stride: img.Stride,
//		Rect:   r,
//		IsSub:  true,
//	}
//}

func (img *BGRAImage) PixOffset(x, y int) int {
	return (y-img.Rect.Min.Y)*img.Stride + (x-img.Rect.Min.X)*4
}

var BGRAModel = color.ModelFunc(bgraModel)

type BGRA struct {
	B, G, R, A uint8
}

func (c BGRA) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R)
	r |= r << 8

	g = uint32(c.G)
	g |= g << 8

	b = uint32(c.B)
	b |= b << 8

	a = uint32(c.A)
	a |= a << 8

	return
}

func bgraModel(c color.Color) color.Color {
	if _, ok := c.(BGRA); ok {
		return c
	}

	r, g, b, a := c.RGBA()
	return BGRA{
		B: uint8(b >> 8),
		G: uint8(g >> 8),
		R: uint8(r >> 8),
		A: uint8(a >> 8),
	}
}
