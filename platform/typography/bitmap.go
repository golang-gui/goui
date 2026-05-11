package typography

import (
	"image"
	"image/color"
)

type TextBitmap struct {
	X      float32
	Y      float32
	Width  int
	Height int
	Stride int
	Pixels []byte // RGBA
}

func (bmp TextBitmap) ColorModel() color.Model {
	return color.RGBAModel
}

func (bmp TextBitmap) Bounds() image.Rectangle {
	return image.Rect(0, 0, bmp.Width, bmp.Height)
}

func (bmp TextBitmap) At(x, y int) color.Color {
	if image.Pt(x, y).In(bmp.Bounds()) {
		index := bmp.PixOffset(x, y)
		return color.RGBA{
			R: bmp.Pixels[index],
			G: bmp.Pixels[index+1],
			B: bmp.Pixels[index+2],
			A: bmp.Pixels[index+3],
		}
	}
	return nil
}

func (bmp TextBitmap) PixOffset(x, y int) int {
	return y*bmp.Stride + x*4
}
