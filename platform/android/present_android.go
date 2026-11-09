package android

import (
	"fmt"
	"image"
	"unsafe"

	"github.com/goexlib/cgo"
	native "github.com/golang-gui/goui/platform/android/libs/android"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/graphics"
)

func present(window uintptr, img image.Image) (result error) {
	pixels, ok := graphics.ToBitmap(img, graphics.PixelFormatRGBA)
	if !ok {
		pixels = graphics.CopyToBitmap(img, graphics.PixelFormatRGBA, nil)
	}
	var buffer native.WindowBuffer
	if native.NativeWindowLock(window, &buffer, nil) != 0 {
		return common.ErrUnavailable
	}
	defer func() {
		if native.NativeWindowUnlockAndPost(window) != 0 && result == nil {
			result = common.ErrUnavailable
		}
	}()
	if buffer.Bits == 0 || buffer.Width <= 0 || buffer.Height <= 0 || int64(buffer.Stride)*int64(buffer.Height) > int64(^uint(0)>>1)/4 || buffer.Stride < buffer.Width || (buffer.Format != 1 && buffer.Format != 2) {
		return fmt.Errorf("android: invalid native window buffer")
	}
	dst := unsafe.Slice((*byte)(cgo.Pointer(buffer.Bits)), int(buffer.Stride)*int(buffer.Height)*4)
	columns, rows := min(pixels.Width, int(buffer.Width)), min(pixels.Height, int(buffer.Height))
	for y := 0; y < int(buffer.Height); y++ {
		row := dst[y*int(buffer.Stride)*4:][:int(buffer.Width)*4]
		clear(row)
		if y < rows {
			copy(row[:columns*4], pixels.Pixels[y*pixels.Stride:][:columns*4])
		}
		for x := 0; x < int(buffer.Width); x++ {
			row[x*4+3] = 255
		}
	}
	return nil
}
