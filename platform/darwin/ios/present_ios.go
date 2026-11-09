package ios

import (
	"fmt"
	"image"
	"sync"

	cf "github.com/golang-gui/goui/platform/darwin/frameworks/core_foundation"
	cg "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	qc "github.com/golang-gui/goui/platform/darwin/frameworks/quartzcore"
	"github.com/golang-gui/goui/platform/graphics"
)

// pendingImage owns at most one copied CGImage between drawing and main-thread
// submission. The native host clears it before releasing or replacing its View.
type pendingImage struct {
	mu         sync.Mutex
	pending    cg.CGImageRef
	generation uint64
	handle     uintptr
	scheduled  bool
}

func (h *nativeState) clearImage() {
	h.image.mu.Lock()
	if h.image.pending != 0 {
		cg.CGImageRelease(h.image.pending)
		h.image.pending = 0
	}
	h.image.mu.Unlock()
}
func (h *nativeState) present(handle uintptr, generation uint64, img image.Image) error {
	bitmap, ok := graphics.ToBitmap(img, graphics.PixelFormatRGBA)
	if !ok {
		bitmap = graphics.CopyToBitmap(img, graphics.PixelFormatRGBA, nil)
	}
	data := cf.CFDataCreate(0, bitmap.Pixels)
	if data == 0 {
		return fmt.Errorf("ios: allocate image data")
	}
	defer cf.CFRelease(cf.CFTypeRef(data))
	provider := cg.CGDataProviderCreateWithCFData(data)
	if provider == 0 {
		return fmt.Errorf("ios: allocate image provider")
	}
	defer cg.CGDataProviderRelease(provider)
	space := cg.CGColorSpaceCreateDeviceRGB()
	if space == 0 {
		return fmt.Errorf("ios: allocate color space")
	}
	defer cg.CGColorSpaceRelease(space)
	// Premultiplied RGBA bytes, big-endian component order. CFData copied the input.
	image := cg.CGImageCreate(uint(bitmap.Width), uint(bitmap.Height), 8, 32, uint(bitmap.Stride), space, cg.CGBitmapInfo(0x4001), provider, nil, false, 0)
	if image == 0 {
		return fmt.Errorf("ios: create image")
	}
	h.image.mu.Lock()
	if h.image.pending != 0 {
		cg.CGImageRelease(h.image.pending)
	}
	h.image.pending = image
	h.image.handle = handle
	h.image.generation = generation
	schedule := !h.image.scheduled
	h.image.scheduled = true
	h.image.mu.Unlock()
	if schedule {
		iosApplication.loop.Load().Post(h.commitImage)
	}
	return nil
}
func (h *nativeState) commitImage() {
	h.image.mu.Lock()
	img, handle, generation := h.image.pending, h.image.handle, h.image.generation
	h.image.pending = 0
	h.image.scheduled = false
	h.image.mu.Unlock()
	if img == 0 {
		return
	}
	defer cg.CGImageRelease(img)
	if h.bound != nil && h.view.ID != 0 && uintptr(h.view.ID) == handle && h.generation == generation {
		qc.CATransactionClassId.Begin()
		qc.CATransactionClassId.SetDisableActions(true)
		Cast[qc.CALayer](h.view.Layer()).SetContents(ID(img))
		qc.CATransactionClassId.Commit()
	}
}
