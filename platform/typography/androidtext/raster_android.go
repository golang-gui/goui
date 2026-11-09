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
	"github.com/golang-gui/goui/platform/android/libs/jni"
	"github.com/golang-gui/goui/platform/android/sdk"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/utils"
)

// textRaster owns one reusable software bitmap and Canvas per TextLayout.
// Java batch layout never owns or replaces these drawing resources.
type textRaster struct {
	bitmap        sdk.Bitmap
	canvas        sdk.Canvas
	width, height int
}

func (r *textRaster) destroy(e jni.Env) (err error) {
	if r.canvas != 0 {
		err = r.canvas.SetBitmap(e, 0)
		e.DeleteGlobalRef(jni.Object(r.canvas))
		r.canvas = 0
	}
	if r.bitmap != 0 {
		if recycleErr := r.bitmap.Recycle(e); err == nil {
			err = recycleErr
		}
		e.DeleteGlobalRef(jni.Object(r.bitmap))
		r.bitmap = 0
	}
	r.width, r.height = 0, 0
	return
}
func (r *textRaster) ensure(e jni.Env, w, h int) error {
	if r.bitmap != 0 && r.width >= w && r.height >= h {
		return nil
	}
	config, err := sdk.BitmapARGB8888(e)
	if err != nil {
		return err
	}
	bitmap, err := sdk.CreateBitmap(e, int32(w), int32(h), config)
	if err != nil {
		return err
	}
	if err = bitmap.SetDensity(e, sdk.BitmapDensityNone); err != nil {
		_ = bitmap.Recycle(e)
		return err
	}
	ref, err := retain(e, jni.Object(bitmap))
	if err != nil {
		_ = bitmap.Recycle(e)
		return err
	}
	next := sdk.Bitmap(ref)
	if r.canvas == 0 {
		local, createErr := sdk.NewCanvas(e)
		if createErr == nil {
			var ref jni.Object
			ref, createErr = retain(e, jni.Object(local))
			r.canvas = sdk.Canvas(ref)
		}
		if createErr != nil {
			_ = next.Recycle(e)
			e.DeleteGlobalRef(jni.Object(next))
			return createErr
		}
	}
	if err = r.canvas.SetBitmap(e, next); err != nil {
		// A failed Canvas mutation must not leave a reusable but unknown binding.
		_ = r.destroy(e)
		_ = next.Recycle(e)
		e.DeleteGlobalRef(jni.Object(next))
		return err
	}
	previous := r.bitmap
	r.bitmap, r.width, r.height = next, w, h
	if previous != 0 {
		err = previous.Recycle(e)
		e.DeleteGlobalRef(jni.Object(previous))
	}
	return err
}
func (r *textRaster) draw(e jni.Env, layout layoutPeer, scale float32, region image.Rectangle, w, h int) (result error) {
	if result = r.ensure(e, w, h); result != nil {
		return
	}
	if result = r.bitmap.EraseColor(e, 0); result != nil {
		return
	}
	if _, result = r.canvas.Save(e); result != nil {
		return
	}
	defer func() {
		if err := r.canvas.Restore(e); err != nil {
			if result == nil {
				result = err
			}
			_ = r.destroy(e)
		}
	}()
	if _, result = r.canvas.ClipRect(e, 0, 0, float32(w), float32(h)); result != nil {
		return
	}
	if result = r.canvas.Translate(e, -float32(region.Min.X), -float32(region.Min.Y)); result != nil {
		return
	}
	if result = r.canvas.Scale(e, scale/nativeUnits, scale/nativeUnits); result != nil {
		return
	}
	result = layout.draw(e, r.canvas)
	return
}

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
	if err = t.raster.draw(e, t.native, scale, region, w, h); err != nil {
		return typography.TextBitmap{}, err
	}
	obj := jni.Object(t.raster.bitmap)
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
