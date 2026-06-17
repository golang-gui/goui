package graphics

import (
	"image"
	"image/color"
	"slices"
)

type PixelFormat int

const (
	PixelFormatUnknown PixelFormat = iota
	PixelFormatRGBA
	PixelFormatBGRA
	PixelFormatGray
)

func (f PixelFormat) BytesPerPixel() (bytes int) {
	if f == PixelFormatGray {
		return 1
	}
	return 4
}

func (f PixelFormat) BitsPerPixel() (bits int) {
	if f == PixelFormatGray {
		return 8
	}
	return 32
}

type Bitmap struct {
	X      int
	Y      int
	Width  int
	Height int
	Stride int
	Format PixelFormat
	Pixels []byte
}

func MakeBitmap(x, y, width, height int, format PixelFormat, buf []byte) Bitmap {
	stride := width * format.BytesPerPixel()
	byteSize := stride * height
	if cap(buf) < byteSize {
		buf = slices.Grow(buf, byteSize)
	}
	return Bitmap{
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
		Stride: stride,
		Format: format,
		Pixels: buf[:byteSize],
	}
}

func ToBitmap(src image.Image, dstFormat PixelFormat) (dst Bitmap, ok bool) {
	if bmp, ok := src.(Bitmap); ok {
		if bmp.Format == dstFormat {
			return bmp, true
		}
		return dst, false
	}

	switch img := src.(type) {
	case *image.RGBA:
		if dstFormat == PixelFormatRGBA {
			dst.X = img.Rect.Min.X
			dst.Y = img.Rect.Min.Y
			dst.Width = img.Rect.Dx()
			dst.Height = img.Rect.Dy()
			dst.Stride = img.Stride
			dst.Format = dstFormat
			dst.Pixels = img.Pix
			return dst, true
		}

	case *image.Gray:
		if dstFormat == PixelFormatGray {
			dst.X = img.Rect.Min.X
			dst.Y = img.Rect.Min.Y
			dst.Width = img.Rect.Dx()
			dst.Height = img.Rect.Dy()
			dst.Stride = img.Stride
			dst.Format = dstFormat
			dst.Pixels = img.Pix
			return dst, true
		}
	}

	return dst, false
}

func CopyToBitmap(src image.Image, dstFormat PixelFormat, buf []byte) (dst Bitmap) {
	if bmp, ok := src.(Bitmap); ok && bmp.Format == dstFormat {
		dst = bmp
		dst.Pixels = make([]byte, len(bmp.Pixels))
		copy(dst.Pixels, bmp.Pixels)
		return
	}

	bounds := src.Bounds()
	dst = MakeBitmap(bounds.Min.X, bounds.Min.Y, bounds.Dx(), bounds.Dy(), dstFormat, buf)

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

	case Bitmap:
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r, g, b, a := img.GetPixel(x, y)
				dst.SetPixel(x, y, r, g, b, a)
			}
		}
		return

	case *Bitmap:
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r, g, b, a := img.GetPixel(x, y)
				dst.SetPixel(x, y, r, g, b, a)
			}
		}
		return
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.Set(x, y, src.At(x, y))
		}
	}

	return
}

func (img Bitmap) GetPixel(x, y int) (r, g, b, a byte) {
	if image.Pt(x, y).In(img.Bounds()) {
		index := img.PixOffset(x, y)
		switch img.Format {
		case PixelFormatRGBA:
			r = img.Pixels[index]
			g = img.Pixels[index+1]
			b = img.Pixels[index+2]
			a = img.Pixels[index+3]
		case PixelFormatBGRA:
			b = img.Pixels[index]
			g = img.Pixels[index+1]
			r = img.Pixels[index+2]
			a = img.Pixels[index+3]
		case PixelFormatGray:
			r = img.Pixels[index]
			g, b, a = r, r, 255
		}
	}
	return
}

func (img Bitmap) SetPixel(x, y int, r, g, b, a byte) {
	if image.Pt(x, y).In(img.Bounds()) {
		index := img.PixOffset(x, y)
		switch img.Format {
		case PixelFormatRGBA:
			img.Pixels[index] = r
			img.Pixels[index+1] = g
			img.Pixels[index+2] = b
			img.Pixels[index+3] = a
		case PixelFormatBGRA:
			img.Pixels[index] = b
			img.Pixels[index+1] = g
			img.Pixels[index+2] = r
			img.Pixels[index+3] = a
		case PixelFormatGray:
			img.Pixels[index] = r
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
	return image.Rect(img.X, img.Y, img.X+img.Width, img.Y+img.Height)
}

func (img Bitmap) At(x, y int) color.Color {
	if image.Pt(x, y).In(img.Bounds()) {
		index := img.PixOffset(x, y)
		return img.toColor(img.Pixels[index:])
	}
	return color.RGBA{}
}

func (img Bitmap) Set(x, y int, c color.Color) {
	r, g, b, a := img.toPixel(c)
	img.SetPixel(x, y, r, g, b, a)
}

func (img Bitmap) SubImage(r image.Rectangle) image.Image {
	r = r.Intersect(img.Bounds())
	if r.Empty() {
		return nil
	}
	i := img.PixOffset(r.Min.X, r.Min.Y)
	return Bitmap{
		X:      r.Min.X,
		Y:      r.Min.Y,
		Width:  r.Dx(),
		Height: r.Dy(),
		Stride: img.Stride,
		Format: img.Format,
		Pixels: img.Pixels[i:],
	}
}

func (img Bitmap) PixOffset(x, y int) int {
	return (y-img.Y)*img.Stride + (x-img.X)*img.Format.BytesPerPixel()
}

func (img Bitmap) toPixel(c color.Color) (r, g, b, a byte) {
	if img.Format != PixelFormatGray {
		rgba := color.RGBAModel.Convert(c).(color.RGBA)
		return rgba.R, rgba.G, rgba.B, rgba.A
	}
	gray := color.GrayModel.Convert(c).(color.Gray)
	return gray.Y, gray.Y, gray.Y, 255
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
