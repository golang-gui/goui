package utils

import (
	"image/color"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/typography"
)

func CopyBitmap(src typography.TextBitmap, width, height int, buf []byte) (dst typography.TextBitmap) {
	dst.Width = min(width, src.Width)
	dst.Height = min(height, src.Height)
	dst.Stride = width * 4
	byteSize := dst.Stride * dst.Height
	if byteSize <= cap(buf) {
		dst.Pixels = buf[:byteSize]
	} else {
		dst.Pixels = make([]byte, byteSize)
	}
	for y := 0; y < dst.Height; y++ {
		dstOffset := dst.PixOffset(0, y)
		srcOffset := src.PixOffset(0, y)
		copy(dst.Pixels[dstOffset:dstOffset+dst.Stride], src.Pixels[srcOffset:])
	}
	return
}

// ReverseBitmap BGRA -> RGBA
func ReverseBitmap(bmp typography.TextBitmap) {
	pixels := cgo.GoSliceNTemp[color.RGBA](cgo.CSlice(bmp.Pixels), bmp.Width*bmp.Height)
	for i := range pixels {
		pixels[i].R, pixels[i].B = pixels[i].B, pixels[i].R
	}
}
