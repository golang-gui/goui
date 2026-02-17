package win32

import (
	"image"
	"image/color"
	"image/draw"
)

type BGRAImage struct {
	Pix    []uint8
	Stride int
	Rect   image.Rectangle
}

func NewBGRAImage(r image.Rectangle) *BGRAImage {
	//if r.Dx()%2 != 0 {
	//	r.Max.X++
	//}

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
	draw.Draw(dst, bounds, src, bounds.Min, draw.Over)
	return dst
}

func (img *BGRAImage) ColorModel() color.Model {
	return color.RGBAModel
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

func (img *BGRAImage) SubImage(r image.Rectangle) image.Image {
	r = r.Intersect(img.Rect)
	if r.Empty() {
		return &BGRAImage{}
	}
	i := img.PixOffset(r.Min.X, r.Min.Y)
	return &BGRAImage{
		Pix:    img.Pix[i:],
		Stride: img.Stride,
		Rect:   r,
	}
}

func (img *BGRAImage) PixOffset(x, y int) int {
	return (y-img.Rect.Min.Y)*img.Stride + (x-img.Rect.Min.X)*4
}
