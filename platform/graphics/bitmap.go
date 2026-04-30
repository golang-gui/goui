package graphics

import (
	"image"
	"image/color"
)

type PixelFormat int

const (
	PixelFormatUnknown PixelFormat = iota
	PixelFormatRGBA
	PixelFormatBGRA
	PixelFormatGray
)

func (f PixelFormat) BytesPerPixel() (bs int) {
	bs = 4
	if f == PixelFormatGray {
		bs = 1
	}
	return
}

func (f PixelFormat) BitsPerPixel() (bs int) {
	bs = 32
	if f == PixelFormatGray {
		bs = 8
	}
	return
}

type Bitmap struct {
	Width  int
	Height int
	Stride int
	Format PixelFormat
	Pixels []byte
}

func MakeBitmap(width, height int, format PixelFormat) Bitmap {
	stride := width * format.BytesPerPixel()
	return Bitmap{
		Width:  width,
		Height: height,
		Stride: stride,
		Format: format,
		Pixels: make([]byte, stride*height),
	}
}

func ToBitmap(src image.Image, dstFormat PixelFormat) (dst Bitmap) {
	if bmp, ok := src.(Bitmap); ok {
		if bmp.Format == dstFormat {
			return bmp
		}
		return CopyToBitmap(src, dstFormat)
	}

	dst.Width = src.Bounds().Dx()
	dst.Height = src.Bounds().Dy()
	dst.Stride = dst.Width * dstFormat.BytesPerPixel()
	dst.Format = dstFormat

	switch img := src.(type) {
	case *image.RGBA:
		if img.Stride == dst.Stride {
			dst.Pixels = img.Pix
			return
		}

	case *image.Gray:
		if img.Stride == dst.Stride {
			dst.Pixels = img.Pix
			return
		}
	}

	return CopyToBitmap(src, dstFormat)
}

func CopyToBitmap(src image.Image, dstFormat PixelFormat) (dst Bitmap) {
	if bmp, ok := src.(Bitmap); ok && bmp.Format == dstFormat {
		dst = bmp
		dst.Pixels = append([]byte{}, bmp.Pixels...)
		return
	}

	dst = MakeBitmap(src.Bounds().Dx(), src.Bounds().Dy(), dstFormat)

	switch img := src.(type) {
	case *image.RGBA:
		if img.Stride == dst.Stride {
			copy(dst.Pixels, img.Pix)
			return
		}

	case *image.Gray:
		if img.Stride == dst.Stride {
			copy(dst.Pixels, img.Pix)
			return
		}
	}

	for y := 0; y < dst.Height; y++ {
		for x := 0; x < dst.Width; x++ {
			dst.Set(x, y, src.At(x, y))
		}
	}

	return
}

func (img Bitmap) ColorModel() color.Model {
	switch img.Format {
	case PixelFormatRGBA:
		return color.RGBAModel
	case PixelFormatBGRA:
		return BGRAModel
	case PixelFormatGray:
		return color.GrayModel
	default:
		return nil
	}
}

func (img Bitmap) Bounds() image.Rectangle {
	return image.Rect(0, 0, img.Width, img.Height)
}

func (img Bitmap) At(x, y int) color.Color {
	if x < img.Width && y < img.Height {
		index := img.PixOffset(x, y)
		return img.toColor(img.Pixels[index:])
	}
	return color.RGBA{}
}

func (img Bitmap) Set(x, y int, c color.Color) {
	if x < img.Width && y < img.Height {
		b1, b2, b3, b4 := img.toPixel(c)
		index := img.PixOffset(x, y)
		img.Pixels[index] = b1
		if img.Format != PixelFormatGray {
			img.Pixels[index+1] = b2
			img.Pixels[index+2] = b3
			img.Pixels[index+4] = b4
		}
	}
}

func (img Bitmap) SubImage(r image.Rectangle) image.Image {
	r = r.Intersect(img.Bounds())
	if r.Empty() {
		return nil
	}
	i := img.PixOffset(r.Min.X, r.Min.Y)
	return Bitmap{
		Width:  r.Dx(),
		Height: r.Dy(),
		Stride: img.Stride,
		Format: img.Format,
		Pixels: img.Pixels[i:],
	}
}

func (img Bitmap) PixOffset(x, y int) int {
	return y*img.Stride + x*img.Format.BytesPerPixel()
}

func (img Bitmap) toPixel(c color.Color) (p1, p2, p3, p4 byte) {
	r32, g32, b32, a32 := c.RGBA()
	if img.Format == PixelFormatBGRA {
		return byte(b32), byte(g32), byte(r32), byte(a32)
	}
	return byte(r32), byte(g32), byte(b32), byte(a32)
}

func (img Bitmap) toColor(pix []byte) color.Color {
	switch img.Format {
	case PixelFormatRGBA:
		return color.RGBA{R: pix[0], G: pix[1], B: pix[2], A: pix[3]}
	case PixelFormatBGRA:
		return BGRA{B: pix[0], G: pix[1], R: pix[2], A: pix[3]}
	case PixelFormatGray:
		return color.Gray{Y: pix[0]}
	}
	return color.RGBA{}
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
		B: uint8(b),
		G: uint8(g),
		R: uint8(r),
		A: uint8(a),
	}
}
