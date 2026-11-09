package androidtext

import (
	"fmt"
	"image"
	"math"
	"runtime"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/internal/diagnostic"
	"github.com/golang-gui/goui/platform/android/libs/bitmap"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/utils"
)

func (t *TextLayout) Rasterize(scale float32, region image.Rectangle, buf []byte) (typography.TextBitmap, error) {
	if scale <= 0 || math.IsInf(float64(scale), 0) || math.IsNaN(float64(scale)) {
		return typography.TextBitmap{}, fmt.Errorf("androidtext: invalid raster scale %g", scale)
	}
	w, h, err := utils.RasterDimensions(region)
	if err != nil || w == 0 || h == 0 {
		return typography.TextBitmap{}, err
	}
	for _, v := range []int{region.Min.X, region.Min.Y} {
		if int64(v) < math.MinInt32 || int64(v) > math.MaxInt32 {
			return typography.TextBitmap{}, fmt.Errorf("androidtext: raster origin is outside native integer range")
		}
	}
	e := frame()
	defer e.PopLocalFrame(0)
	obj, err := t.native.raster(e, scale/nativeUnits, int32(region.Min.X), int32(region.Min.Y), int32(w), int32(h))
	if err != nil {
		return typography.TextBitmap{}, err
	}
	info, status := bitmap.GetInfo(e, obj)
	if err = e.Exception(); err != nil {
		return typography.TextBitmap{}, err
	}
	if status != 0 {
		return typography.TextBitmap{}, fmt.Errorf("androidtext: bitmap info %d", status)
	}
	if info.Format != bitmap.RGBA8888 || info.Flags&3 != 0 || int(info.Width) < w || int(info.Height) < h || uint64(info.Stride) < uint64(w)*4 {
		return typography.TextBitmap{}, fmt.Errorf("androidtext: unexpected bitmap format %+v", info)
	}
	pixels, status := bitmap.LockPixels(e, obj)
	if err = e.Exception(); err != nil {
		return typography.TextBitmap{}, err
	}
	if status != 0 {
		return typography.TextBitmap{}, fmt.Errorf("androidtext: lock bitmap %d", status)
	}
	locked := true
	defer func() {
		if locked {
			bitmap.UnlockPixels(e, obj)
		}
	}()
	if cap(buf) < w*h*4 {
		buf = make([]byte, w*h*4)
	} else {
		buf = buf[:w*h*4]
	}
	for y := 0; y < h; y++ {
		copy(buf[y*w*4:(y+1)*w*4], unsafe.Slice((*byte)(cgo.Pointer(pixels+uintptr(y)*uintptr(info.Stride))), w*4))
	}
	status = bitmap.UnlockPixels(e, obj)
	locked = false
	runtime.KeepAlive(obj)
	if err = e.Exception(); err != nil {
		return typography.TextBitmap{}, err
	}
	if status != 0 {
		return typography.TextBitmap{}, fmt.Errorf("androidtext: unlock bitmap %d", status)
	}
	diagnostic.Record(diagnostic.TextRasterize, 1)
	return typography.TextBitmap{Width: w, Height: h, Stride: w * 4, Pixels: buf}, nil
}
