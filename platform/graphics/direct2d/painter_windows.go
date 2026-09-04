package direct2d

import (
	"fmt"
	"image"
	"unsafe"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/internal/boxshadow"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/directwrite"

	"github.com/golang-gui/goui/platform/windows/sdk/com"
	"github.com/golang-gui/goui/platform/windows/sdk/d2d1"
	"github.com/golang-gui/goui/platform/windows/sdk/d3d11"
	"github.com/golang-gui/goui/platform/windows/sdk/dxgi"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"

	"github.com/goexlib/mathx"
)

type Painter struct {
	hwnd         uintptr
	factory      *d2d1.Factory1
	d3dDevice    *d3d11.Device
	dxgiDevice   *dxgi.Device
	d2dDevice    *d2d1.Device
	swapChain    *dxgi.SwapChain1
	target       *d2d1.Bitmap1
	render       *d2d1.DeviceContext
	shadowRender *d2d1.DeviceContext
	shadowEffect *d2d1.Effect
	shadowBrush  *d2d1.SolidColorBrush
	shadowCache  []shadowCacheEntry
	shadowClock  uint64

	colorBrush           *d2d1.SolidColorBrush
	color                d2d1.ColorF
	linearStops          *d2d1.GradientStopCollection
	linearBrush          *d2d1.LinearGradientBrush
	linearStart          graphics.Color
	linearEnd            graphics.Color
	hasLinear            bool
	rect                 d2d1.RectF
	roundRect            d2d1.RoundRect
	ellipse              d2d1.Ellipse
	clip                 d2d1.RectF
	images               map[*imageResource]struct{}
	width                uint32
	height               uint32
	scale                float32
	transform            geometry.Transform
	matrix               d2d1.Matrix3x2F
	activeFrame          bool
	deferredFrame        bool
	occluded             bool
	swapChainFlags       uint32
	frameLatencyWaitable winapi.HANDLE
}

type imageResource struct {
	owner     *Painter
	width     int
	height    int
	pixels    graphics.Bitmap
	bitmap    *d2d1.Bitmap
	destroyed bool
}

func (i *imageResource) Size() (width, height int) {
	if i == nil {
		return 0, 0
	}
	return i.width, i.height
}

func (i *imageResource) Update(src image.Image) error {
	if i == nil || i.destroyed || i.owner == nil {
		return fmt.Errorf("direct2d: update destroyed image")
	}
	return i.owner.updateImage(i, src)
}

func (i *imageResource) Destroy() {
	if i == nil || i.destroyed || i.owner == nil {
		return
	}
	i.owner.destroyImage(i)
}

const shadowCacheCapacity = 16

// GOUI paints complete frames in response to window messages. Present without
// a sync interval so a newer frame can replace queued work; the waitable swap
// chain below performs pacing before Direct2D starts rendering.
const frameSyncInterval uint32 = 0

const frameWaitTimeoutMillis winapi.DWORD = 1

type shadowCacheKey struct{ Width, Height, Radius float32 }
type shadowCacheEntry struct {
	key  shadowCacheKey
	list *d2d1.CommandList
	age  uint64
}

type NativeWindow interface {
	NativeHandle() uintptr
}

func NewPainter(win NativeWindow) (_ graphics.Painter, err error) {
	p := new(Painter)
	p.hwnd = win.NativeHandle()
	p.images = make(map[*imageResource]struct{})

	p.factory, err = d2d1.CreateFactory[d2d1.Factory1](d2d1.D2D1_FACTORY_TYPE_SINGLE_THREADED, d2d1.IID_ID2D1Factory1, nil)
	if err != nil {
		return nil, fmt.Errorf("create d2d 1.1 factory: %v", err)
	}
	if err = p.createDeviceResources(); err != nil {
		p.Destroy()
		return nil, err
	}

	return p, nil
}

func (p *Painter) createDeviceResources() (err error) {
	levels := []d3d11.FeatureLevel{
		d3d11.D3D_FEATURE_LEVEL_11_1,
		d3d11.D3D_FEATURE_LEVEL_11_0,
		d3d11.D3D_FEATURE_LEVEL_10_1,
		d3d11.D3D_FEATURE_LEVEL_10_0,
	}
	var hr com.HRESULT
	for _, driver := range []d3d11.DriverType{d3d11.D3D_DRIVER_TYPE_HARDWARE, d3d11.D3D_DRIVER_TYPE_WARP} {
		var immediate *d3d11.DeviceContext
		p.d3dDevice, _, immediate, hr = d3d11.CreateDevice(driver, d3d11.D3D11_CREATE_DEVICE_BGRA_SUPPORT, levels)
		if hr == com.HRESULT(-2147024809) { // E_INVALIDARG: Windows 7 does not accept feature level 11.1.
			if immediate != nil {
				immediate.Release()
				immediate = nil
			}
			if p.d3dDevice != nil {
				p.d3dDevice.Release()
				p.d3dDevice = nil
			}
			p.d3dDevice, _, immediate, hr = d3d11.CreateDevice(driver, d3d11.D3D11_CREATE_DEVICE_BGRA_SUPPORT, levels[1:])
		}
		if immediate != nil {
			immediate.Release()
		}
		if hr.Succeeded() {
			break
		}
		if p.d3dDevice != nil {
			p.d3dDevice.Release()
			p.d3dDevice = nil
		}
	}
	if hr.Failed() || p.d3dDevice == nil {
		return fmt.Errorf("create D3D11 BGRA device: %v", hr)
	}

	var unknown *com.Unknown
	if hr = p.d3dDevice.QueryInterface(dxgi.IID_IDXGIDevice, &unknown); hr.Failed() {
		p.releaseDeviceResources()
		return fmt.Errorf("query IDXGIDevice: %v", hr)
	}
	p.dxgiDevice = (*dxgi.Device)(unsafe.Pointer(unknown))
	if p.d2dDevice, hr = p.factory.CreateDevice(p.dxgiDevice); hr.Failed() {
		p.releaseDeviceResources()
		return fmt.Errorf("create Direct2D device: %v", hr)
	}
	if p.render, hr = p.d2dDevice.CreateDeviceContext(d2d1.D2D1_DEVICE_CONTEXT_OPTIONS_NONE); hr.Failed() {
		p.releaseDeviceResources()
		return fmt.Errorf("create Direct2D device context: %v", hr)
	}
	if p.shadowRender, hr = p.d2dDevice.CreateDeviceContext(d2d1.D2D1_DEVICE_CONTEXT_OPTIONS_NONE); hr.Failed() {
		p.releaseDeviceResources()
		return fmt.Errorf("create Direct2D shadow context: %v", hr)
	}

	adapter, hr := p.dxgiDevice.GetAdapter()
	if hr.Failed() {
		p.releaseDeviceResources()
		return fmt.Errorf("get DXGI adapter: %v", hr)
	}
	defer adapter.Release()
	unknown = nil
	if hr = adapter.GetParent(dxgi.IID_IDXGIFactory2, &unknown); hr.Failed() {
		p.releaseDeviceResources()
		return fmt.Errorf("get IDXGIFactory2: %v", hr)
	}
	dxgiFactory := (*dxgi.Factory2)(unsafe.Pointer(unknown))
	defer dxgiFactory.Release()
	var frameLatencyErr error
	for _, desc := range swapChainCandidates() {
		p.swapChain, hr = dxgiFactory.CreateSwapChainForHwnd(&p.d3dDevice.Unknown, p.hwnd, &desc)
		if hr.Failed() {
			continue
		}
		p.swapChainFlags = desc.Flags
		if desc.Flags&dxgi.DXGI_SWAP_CHAIN_FLAG_FRAME_LATENCY_WAITABLE_OBJECT != 0 {
			if err := p.configureFrameLatency(); err != nil {
				frameLatencyErr = err
				p.swapChain.Release()
				p.swapChain = nil
				p.swapChainFlags = 0
				continue
			}
		}
		break
	}
	if p.swapChain == nil {
		p.releaseDeviceResources()
		if frameLatencyErr != nil {
			return fmt.Errorf("create waitable DXGI swap chain: %w", frameLatencyErr)
		}
		return fmt.Errorf("create DXGI swap chain: %v", hr)
	}

	if p.colorBrush, hr = p.render.CreateSolidColorBrush(&p.color, nil); hr.Failed() {
		p.releaseDeviceResources()
		return fmt.Errorf("create solid color brush: %v", hr)
	}
	opaque := d2d1.ColorF{R: 1, G: 1, B: 1, A: 1}
	if p.shadowBrush, hr = p.shadowRender.CreateSolidColorBrush(&opaque, nil); hr.Failed() {
		p.releaseDeviceResources()
		return fmt.Errorf("create shadow mask brush: %v", hr)
	}
	if p.shadowEffect, hr = p.render.CreateEffect(d2d1.CLSID_D2D1Shadow); hr.Failed() {
		p.releaseDeviceResources()
		return fmt.Errorf("create Direct2D shadow effect: %v", hr)
	}
	return nil
}

func swapChainCandidates() [5]dxgi.SwapChainDesc1 {
	base := dxgi.SwapChainDesc1{
		Format:      dxgi.DXGI_FORMAT_B8G8R8A8_UNORM,
		SampleDesc:  dxgi.SampleDesc{Count: 1},
		BufferUsage: dxgi.DXGI_USAGE_RENDER_TARGET_OUTPUT,
		BufferCount: 2,
		Scaling:     dxgi.DXGI_SCALING_NONE,
		SwapEffect:  dxgi.DXGI_SWAP_EFFECT_FLIP_SEQUENTIAL,
		AlphaMode:   dxgi.DXGI_ALPHA_MODE_UNSPECIFIED,
		Flags:       dxgi.DXGI_SWAP_CHAIN_FLAG_FRAME_LATENCY_WAITABLE_OBJECT,
	}

	// DXGI_SCALING_NONE prevents DWM from stretching the last presented
	// buffer while the HWND and swap-chain sizes briefly differ during a live
	// resize. Prefer a frame-latency waitable swap chain so painting can be
	// paced before Direct2D acquires a back buffer. Keep non-waitable flip-model
	// configurations for Windows 8, then the legacy blt-model fallback.
	// SCALING_NONE is not valid with DXGI_SWAP_EFFECT_DISCARD.
	flipStretch := base
	flipStretch.Scaling = dxgi.DXGI_SCALING_STRETCH
	flipNoWait := base
	flipNoWait.Flags = 0
	flipStretchNoWait := flipStretch
	flipStretchNoWait.Flags = 0
	legacy := flipStretchNoWait
	legacy.BufferCount = 1
	legacy.SwapEffect = dxgi.DXGI_SWAP_EFFECT_DISCARD
	return [5]dxgi.SwapChainDesc1{base, flipStretch, flipNoWait, flipStretchNoWait, legacy}
}

func (p *Painter) configureFrameLatency() error {
	var unknown *com.Unknown
	if hr := p.swapChain.QueryInterface(dxgi.IID_IDXGISwapChain2, &unknown); hr.Failed() {
		return fmt.Errorf("query IDXGISwapChain2: %v", hr)
	}
	swapChain2 := (*dxgi.SwapChain2)(unsafe.Pointer(unknown))
	defer swapChain2.Release()
	if hr := swapChain2.SetMaximumFrameLatency(1); hr.Failed() {
		return fmt.Errorf("set maximum frame latency: %v", hr)
	}
	handle := swapChain2.GetFrameLatencyWaitableObject()
	if handle == 0 {
		return fmt.Errorf("get frame latency waitable object: null handle")
	}
	p.frameLatencyWaitable = winapi.HANDLE(handle)
	return nil
}

func (p *Painter) createTarget(scale float32) com.HRESULT {
	surface, hr := p.swapChain.GetBuffer(0, dxgi.IID_IDXGISurface)
	if hr.Failed() {
		return hr
	}
	defer surface.Release()
	dpi := 96 * scale
	props := d2d1.BitmapProperties1{
		PixelFormat: d2d1.PixelFormat{Format: dxgi.DXGI_FORMAT_B8G8R8A8_UNORM, AlphaMode: d2d1.D2D1_ALPHA_MODE_IGNORE},
		DpiX:        dpi, DpiY: dpi,
		BitmapOptions: d2d1.D2D1_BITMAP_OPTIONS_TARGET | d2d1.D2D1_BITMAP_OPTIONS_CANNOT_DRAW,
	}
	p.target, hr = p.render.CreateBitmapFromDxgiSurface(surface, &props)
	if hr.Succeeded() {
		p.render.SetTarget((*d2d1.Image)(unsafe.Pointer(p.target)))
	}
	return hr
}

func (p *Painter) releaseTarget() {
	if p.target == nil {
		return
	}
	p.render.SetTarget(nil)
	p.target.Release()
	p.target = nil
}

func isDeviceLost(hr com.HRESULT) bool {
	code := int32(hr)
	return code == d2d1.D2DERR_RECREATE_TARGET || code == dxgi.DXGI_ERROR_DEVICE_REMOVED || code == dxgi.DXGI_ERROR_DEVICE_RESET
}

func (p *Painter) handleDeviceFailure(hr com.HRESULT) {
	if isDeviceLost(hr) {
		p.releaseDeviceResources()
		return
	}
	p.releaseTarget()
}

func (p *Painter) Name() string {
	return "Direct2D"
}

func (p *Painter) Destroy() {
	if p.activeFrame {
		panic("direct2d: destroy painter during active frame")
	}
	p.destroyAllImages()
	p.releaseDeviceResources()
	if p.factory != nil {
		p.factory.Release()
		p.factory = nil
	}
}

func (p *Painter) releaseDeviceResources() {
	p.activeFrame = false
	p.deferredFrame = false
	p.occluded = false
	p.releaseImageNatives()
	p.releaseShadowResources()
	if p.linearBrush != nil {
		p.linearBrush.Release()
		p.linearBrush = nil
	}
	if p.linearStops != nil {
		p.linearStops.Release()
		p.linearStops = nil
	}
	if p.colorBrush != nil {
		p.colorBrush.Release()
		p.colorBrush = nil
	}
	if p.target != nil {
		if p.render != nil {
			p.render.SetTarget(nil)
		}
		p.target.Release()
		p.target = nil
	}
	if p.shadowRender != nil {
		p.shadowRender.Release()
		p.shadowRender = nil
	}
	if p.render != nil {
		p.render.Release()
		p.render = nil
	}
	p.closeFrameLatencyWaitable()
	if p.swapChain != nil {
		p.swapChain.Release()
		p.swapChain = nil
	}
	p.swapChainFlags = 0
	if p.d2dDevice != nil {
		p.d2dDevice.Release()
		p.d2dDevice = nil
	}
	if p.dxgiDevice != nil {
		p.dxgiDevice.Release()
		p.dxgiDevice = nil
	}
	if p.d3dDevice != nil {
		p.d3dDevice.Release()
		p.d3dDevice = nil
	}
	p.width, p.height = 0, 0
}

func (p *Painter) closeFrameLatencyWaitable() {
	if p.frameLatencyWaitable == 0 {
		return
	}
	winapi.CloseHandle(p.frameLatencyWaitable)
	p.frameLatencyWaitable = 0
}

func (p *Painter) NewImage(src image.Image) (graphics.Image, error) {
	if src == nil {
		return nil, fmt.Errorf("direct2d: create image from nil source")
	}
	bounds := src.Bounds()
	if bounds.Empty() {
		return nil, fmt.Errorf("direct2d: create empty image")
	}
	if p.render == nil {
		if err := p.createDeviceResources(); err != nil {
			return nil, fmt.Errorf("direct2d: create image device: %w", err)
		}
	}
	pixels := graphics.CopyToBitmap(src, graphics.PixelFormatBGRA, nil)
	bitmap, hr := p.createImageBitmap(pixels)
	if hr.Failed() {
		if isDeviceLost(hr) {
			p.handleDeviceFailure(hr)
		}
		return nil, fmt.Errorf("direct2d: create native image: %v", hr)
	}
	img := &imageResource{
		owner:  p,
		width:  bounds.Dx(),
		height: bounds.Dy(),
		pixels: pixels,
		bitmap: bitmap,
	}
	p.images[img] = struct{}{}
	return img, nil
}

func (p *Painter) updateImage(img *imageResource, src image.Image) error {
	if p.activeFrame {
		panic("direct2d: update image during active frame")
	}
	if img == nil || img.destroyed || img.owner != p {
		return fmt.Errorf("direct2d: update invalid image")
	}
	if src == nil {
		return fmt.Errorf("direct2d: update image from nil source")
	}
	bounds := src.Bounds()
	if bounds.Dx() != img.width || bounds.Dy() != img.height {
		return fmt.Errorf("direct2d: update image size %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), img.width, img.height)
	}
	if p.render == nil {
		if err := p.createDeviceResources(); err != nil {
			return fmt.Errorf("direct2d: create image device for update: %w", err)
		}
	}
	pixels := graphics.CopyToBitmap(src, graphics.PixelFormatBGRA, img.pixels.Pixels)
	if img.bitmap == nil {
		bitmap, hr := p.createImageBitmap(pixels)
		if hr.Failed() {
			if isDeviceLost(hr) {
				p.handleDeviceFailure(hr)
			}
			return fmt.Errorf("direct2d: recreate native image for update: %v", hr)
		}
		img.bitmap = bitmap
	} else if hr := img.bitmap.CopyFromMemory(nil, pixels.Pixels, pixels.Stride); hr.Failed() {
		if isDeviceLost(hr) {
			p.handleDeviceFailure(hr)
		}
		return fmt.Errorf("direct2d: update native image: %v", hr)
	}
	img.pixels = pixels
	return nil
}

func (p *Painter) createImageBitmap(bitmap graphics.Bitmap) (*d2d1.Bitmap, com.HRESULT) {
	size := d2d1.SizeU{Width: uint32(bitmap.Width), Height: uint32(bitmap.Height)}
	props := d2d1.BitmapProperties{
		PixelFormat: d2d1.PixelFormat{
			Format:    dxgi.DXGI_FORMAT_B8G8R8A8_UNORM,
			AlphaMode: d2d1.D2D1_ALPHA_MODE_PREMULTIPLIED,
		},
		DpiX: 96,
		DpiY: 96,
	}
	return p.render.CreateBitmap(size, bitmap.Pixels, bitmap.Stride, &props)
}

func (p *Painter) destroyImage(img *imageResource) {
	if img == nil || img.destroyed || img.owner != p {
		return
	}
	if p.activeFrame {
		panic("direct2d: destroy image during active frame")
	}
	delete(p.images, img)
	bitmap := img.bitmap
	img.owner = nil
	img.bitmap = nil
	img.pixels.Pixels = nil
	img.destroyed = true
	if bitmap != nil {
		bitmap.Release()
	}
}

func (p *Painter) releaseImageNatives() {
	for img := range p.images {
		if img.bitmap != nil {
			img.bitmap.Release()
			img.bitmap = nil
		}
	}
}

func (p *Painter) destroyAllImages() {
	p.releaseImageNatives()
	for img := range p.images {
		img.owner = nil
		img.pixels.Pixels = nil
		img.destroyed = true
	}
	clear(p.images)
}

func (p *Painter) Begin(width, height, scale float32) {
	p.activeFrame = false
	p.deferredFrame = false
	if width <= 0 || height <= 0 {
		return
	}
	if p.render == nil {
		if err := p.createDeviceResources(); err != nil {
			return
		}
	}
	if p.occluded {
		// DXGI_PRESENT_TEST is intended for leaving the idle state. Avoid
		// rebuilding a complete frame until DWM reports that the HWND is visible
		// again.
		if !p.processPresentResult(p.swapChain.Present(0, dxgi.DXGI_PRESENT_TEST)) {
			return
		}
	}
	if !p.acquireFrameSlot() {
		return
	}
	w, h := uint32(width), uint32(height)
	scaleChanged := p.scale != 0 && p.scale != scale
	targetChanged := p.target == nil || p.width != w || p.height != h || scaleChanged
	if targetChanged {
		p.releaseTarget()
		if p.width != w || p.height != h {
			// ResizeBuffers must receive the same waitable-object flag that was
			// used to create the swap chain.
			if hr := p.swapChain.ResizeBuffers(0, w, h, dxgi.DXGI_FORMAT_UNKNOWN, p.swapChainFlags); hr.Failed() {
				p.handleDeviceFailure(hr)
				return
			}
		}
		p.width, p.height, p.scale = w, h, scale
		if hr := p.createTarget(scale); hr.Failed() {
			p.handleDeviceFailure(hr)
			return
		}
		if scaleChanged {
			p.releaseShadowCache()
		}
	}
	dpi := 96 * scale
	p.render.SetDpi(dpi, dpi)
	p.render.BeginDraw()
	p.scale = scale
	p.activeFrame = true
	// Reset transform to identity at the start of each frame.
	p.SetTransform(geometry.Identity())
}

func (p *Painter) acquireFrameSlot() bool {
	if p.frameLatencyWaitable == 0 {
		return true
	}

	switch winapi.WaitForSingleObjectEx(p.frameLatencyWaitable, frameWaitTimeoutMillis, winapi.FALSE) {
	case winapi.WAIT_OBJECT_0:
		return true
	case winapi.WAIT_TIMEOUT:
		// WM_PAINT runs on the UI thread, so do not block it for a complete
		// refresh interval. Coalesce another paint and render once DXGI has a
		// back-buffer slot instead of letting EndDraw absorb the queue stall.
		p.deferredFrame = true
		return false
	default:
		// A failed wait must not make the window permanently blank. Drop the
		// pacing handle and continue with the compatible non-waiting path.
		p.closeFrameLatencyWaitable()
		return true
	}
}

func (p *Painter) End() {
	if !p.activeFrame {
		if p.deferredFrame {
			p.deferredFrame = false
			_ = winapi.InvalidateRect(winapi.HWND(p.hwnd), nil, winapi.FALSE)
		}
		return
	}
	// Defensive: pop any clip the GUI layer forgot to restore. D2D requires
	// the clip stack to be balanced before EndDraw; an unbalanced stack fails
	// the draw and leaves the window blank.
	p.SetClipRect(graphics.Rectangle{})
	hr := p.render.EndDraw(nil, nil)
	p.activeFrame = false
	if hr.Failed() {
		p.handleDeviceFailure(hr)
		return
	}
	p.processPresentResult(p.swapChain.Present(frameSyncInterval, 0))
}

func (p *Painter) processPresentResult(hr com.HRESULT) bool {
	if hr == com.HRESULT(dxgi.DXGI_STATUS_OCCLUDED) {
		p.occluded = true
		return false
	}
	p.occluded = false
	if hr.Failed() {
		p.handleDeviceFailure(hr)
		return false
	}
	return true
}

func (p *Painter) Clear(color graphics.Color) {
	if !p.activeFrame {
		return
	}
	p.color.R, p.color.G, p.color.B, p.color.A = color.R, color.G, color.B, color.A
	p.render.Clear(&p.color)
}

func (p *Painter) DrawBoxShadow(rect graphics.Rectangle, radius float32, shadow graphics.BoxShadow) {
	if !p.activeFrame {
		return
	}
	if shadow.Color.A <= 0 {
		return
	}
	shape, ok := boxshadow.Normalize(rect, radius, shadow.Offset, shadow.BlurRadius, shadow.SpreadRadius)
	if !ok {
		return
	}
	if shape.BlurRadius <= 0 {
		p.drawClearBoxShadow(shape, shadow.Color)
		return
	}
	if !p.drawSoftBoxShadow(shape, shadow.Color) {
		p.drawClearBoxShadow(shape, shadow.Color)
	}
}

func (p *Painter) drawClearBoxShadow(shape boxshadow.Shape, color graphics.Color) {
	p.setRoundRect(shape.Rect, shape.Radius)
	p.render.FillRoundedRectangle(&p.roundRect, p.setColorBrush(color))
}

func (p *Painter) releaseShadowResources() {
	if p.shadowEffect != nil {
		p.shadowEffect.SetInput(0, nil, true)
	}
	p.releaseShadowCache()
	if p.shadowEffect != nil {
		p.shadowEffect.Release()
		p.shadowEffect = nil
	}
	if p.shadowBrush != nil {
		p.shadowBrush.Release()
		p.shadowBrush = nil
	}
}

func (p *Painter) drawSoftBoxShadow(shape boxshadow.Shape, color graphics.Color) bool {
	if p.shadowEffect == nil || p.shadowRender == nil {
		return false
	}
	commandList := p.shadowCommandList(shape)
	if commandList == nil {
		return false
	}

	sigma := shape.Sigma()
	d2dColor := d2d1.ColorF{R: color.R, G: color.G, B: color.B, A: color.A}
	optimization := d2d1.D2D1_SHADOW_OPTIMIZATION_QUALITY
	if p.shadowEffect.SetValue(uint32(d2d1.D2D1_SHADOW_PROP_BLUR_STANDARD_DEVIATION), d2d1.D2D1_PROPERTY_TYPE_FLOAT, unsafe.Pointer(&sigma), uint32(unsafe.Sizeof(sigma))).Failed() ||
		p.shadowEffect.SetValue(uint32(d2d1.D2D1_SHADOW_PROP_COLOR), d2d1.D2D1_PROPERTY_TYPE_VECTOR4, unsafe.Pointer(&d2dColor), uint32(unsafe.Sizeof(d2dColor))).Failed() ||
		p.shadowEffect.SetValue(uint32(d2d1.D2D1_SHADOW_PROP_OPTIMIZATION), d2d1.D2D1_PROPERTY_TYPE_ENUM, unsafe.Pointer(&optimization), uint32(unsafe.Sizeof(optimization))).Failed() {
		return false
	}
	p.shadowEffect.SetInput(0, &commandList.Image, true)
	output := p.shadowEffect.GetOutput()
	if output == nil {
		p.shadowEffect.SetInput(0, nil, true)
		return false
	}
	defer output.Release()
	defer p.shadowEffect.SetInput(0, nil, true)
	// Direct2D keeps the effect output in the input image's coordinate space.
	// Its negative blur bounds must not be added here: targetOffset positions
	// that coordinate space, so adding them shifts the shadow left/up by 3σ.
	offset := shadowTargetOffset(shape)
	p.render.DrawImage(output, &offset, nil, d2d1.D2D1_INTERPOLATION_MODE_LINEAR, d2d1.D2D1_COMPOSITE_MODE_SOURCE_OVER)
	return true
}

func (p *Painter) shadowCommandList(shape boxshadow.Shape) *d2d1.CommandList {
	key := shadowCacheKey{Width: shape.Rect.Width, Height: shape.Rect.Height, Radius: shape.Radius}
	p.shadowClock++
	for i := range p.shadowCache {
		if p.shadowCache[i].key == key {
			p.shadowCache[i].age = p.shadowClock
			return p.shadowCache[i].list
		}
	}

	list, hr := p.shadowRender.CreateCommandList()
	if hr.Failed() {
		return nil
	}
	p.shadowRender.SetTarget(&list.Image)
	p.shadowRender.BeginDraw()
	identity := d2d1.Matrix3x2F{M11: 1, M22: 1}
	p.shadowRender.SetTransform(&identity)
	roundRect := d2d1.RoundRect{
		Rect:    d2d1.RectF{Right: shape.Rect.Width, Bottom: shape.Rect.Height},
		RadiusX: shape.Radius, RadiusY: shape.Radius,
	}
	p.shadowRender.FillRoundedRectangle(&roundRect, &p.shadowBrush.Brush)
	hr = p.shadowRender.EndDraw(nil, nil)
	p.shadowRender.SetTarget(nil)
	closeHR := list.Close()
	if hr.Failed() || closeHR.Failed() {
		list.Release()
		return nil
	}

	entry := shadowCacheEntry{key: key, list: list, age: p.shadowClock}
	if len(p.shadowCache) < shadowCacheCapacity {
		p.shadowCache = append(p.shadowCache, entry)
		return list
	}
	oldest := oldestShadowIndex(p.shadowCache)
	p.shadowCache[oldest].list.Release()
	p.shadowCache[oldest] = entry
	return list
}

func shadowTargetOffset(shape boxshadow.Shape) d2d1.Point2F {
	return d2d1.Point2F{X: shape.Rect.X, Y: shape.Rect.Y}
}

func oldestShadowIndex(entries []shadowCacheEntry) int {
	oldest := 0
	for i := 1; i < len(entries); i++ {
		if entries[i].age < entries[oldest].age {
			oldest = i
		}
	}
	return oldest
}

func (p *Painter) releaseShadowCache() {
	for i := range p.shadowCache {
		p.shadowCache[i].list.Release()
	}
	p.shadowCache = p.shadowCache[:0]
	p.shadowClock = 0
}

func (p *Painter) FillRect(rect graphics.Rectangle, brush graphics.Brush) {
	if !p.activeFrame {
		return
	}
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		p.setRect(p.snapRect(rect))
		p.render.FillRectangle(&p.rect, d2dBrush)
	}
}

func (p *Painter) FillRoundRect(rect graphics.Rectangle, radius float32, brush graphics.Brush) {
	if !p.activeFrame {
		return
	}
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		rect = p.snapRect(rect)
		p.setRoundRect(rect, radius)
		p.render.FillRoundedRectangle(&p.roundRect, d2dBrush)
	}
}

func (p *Painter) FillEllipse(center graphics.Point, xRadius, yRadius float32, brush graphics.Brush) {
	if !p.activeFrame {
		return
	}
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		p.setEllipse(p.snapPoint(center), xRadius, yRadius)
		p.render.FillEllipse(&p.ellipse, d2dBrush)
	}
}

func (p *Painter) FillPath(path graphics.Path, brush graphics.Brush) {
	if !p.activeFrame {
		return
	}
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		geometry, err := p.createPathGeometry(p.snapPath(path), true)
		if err == nil {
			defer geometry.Release()
			p.render.FillGeometry(geometry, d2dBrush, nil)
		}
	}
}

func (p *Painter) DrawLine(p0, p1 graphics.Point, strokeWidth float32, brush graphics.Brush) {
	if !p.activeFrame {
		return
	}
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		point0 := d2d1.Point2F{X: p.snap(p0.X), Y: p.snap(p0.Y)}
		point1 := d2d1.Point2F{X: p.snap(p1.X), Y: p.snap(p1.Y)}
		p.render.DrawLine(point0, point1, d2dBrush, strokeWidth, nil) // TODO: strokeStyle
	}
}

func (p *Painter) DrawRect(rect graphics.Rectangle, strokeWidth float32, brush graphics.Brush) {
	if !p.activeFrame {
		return
	}
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		p.setRect(p.snapRect(rect))
		p.render.DrawRectangle(&p.rect, d2dBrush, strokeWidth, nil)
	}
}

func (p *Painter) DrawRoundRect(rect graphics.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
	if !p.activeFrame {
		return
	}
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		rect = p.snapRect(rect)
		p.setRoundRect(rect, radius)
		p.render.DrawRoundedRectangle(&p.roundRect, d2dBrush, strokeWidth, nil)
	}
}

func (p *Painter) DrawEllipse(center graphics.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush) {
	if !p.activeFrame {
		return
	}
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		p.setEllipse(p.snapPoint(center), xRadius, yRadius)
		p.render.DrawEllipse(&p.ellipse, d2dBrush, strokeWidth, nil)
	}
}

func (p *Painter) DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush) {
	if !p.activeFrame {
		return
	}
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		geometry, err := p.createPathGeometry(p.snapPath(path), false)
		if err == nil {
			defer geometry.Release()
			p.render.DrawGeometry(geometry, d2dBrush, strokeWidth, nil)
		}
	}
}

func (p *Painter) DrawTextLayout(origin graphics.Point, layout typography.TextLayout) {
	if !p.activeFrame {
		return
	}
	if textLayout, ok := layout.(*directwrite.TextLayout); ok {
		point := d2d1.Point2F{X: p.snap(origin.X), Y: p.snap(origin.Y)}
		textLayout.Draw(&p.render.RenderTarget, point, d2d1.D2D1_DRAW_TEXT_OPTIONS_ENABLE_COLOR_FONT|d2d1.D2D1_DRAW_TEXT_OPTIONS_CLIP)
	}
}

func (p *Painter) SetTransform(t geometry.Transform) {
	if !p.activeFrame {
		return
	}
	p.transform = t
	p.matrix = d2dMatrix(t)
	p.render.SetTransform(&p.matrix)
}

// d2dMatrix converts GOUI's column-vector transform into Direct2D's
// row-vector matrix layout. The linear off-diagonal terms must be
// transposed: Direct2D maps x' = x*M11 + y*M21 + M31, while
// geometry.Transform maps x' = A11*x + A12*y + TX.
func d2dMatrix(t geometry.Transform) d2d1.Matrix3x2F {
	return d2d1.Matrix3x2F{
		M11: t.A11, M12: t.A21,
		M21: t.A12, M22: t.A22,
		M31: t.TX, M32: t.TY,
	}
}

func (p *Painter) DrawImage(rect graphics.Rectangle, img graphics.Image) {
	if !p.activeFrame {
		return
	}
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	native, ok := img.(*imageResource)
	if !ok || native == nil || native.owner != p || native.destroyed {
		panic("direct2d: image does not belong to painter or was destroyed")
	}
	if native.bitmap == nil {
		bitmap, hr := p.createImageBitmap(native.pixels)
		if hr.Failed() {
			if isDeviceLost(hr) {
				p.handleDeviceFailure(hr)
			}
			return
		}
		native.bitmap = bitmap
	}
	p.drawNativeImage(p.snapRect(rect), native.bitmap)
}

func (p *Painter) SetClipRect(rect graphics.Rectangle) {
	if !p.activeFrame {
		return
	}
	// D2D's PushAxisAlignedClip applies the current render target transform
	// to the clip rect. Since the clip rect is already in window-local
	// coordinates (the transform's offset has been applied by the GUI layer),
	// we must set the clip in identity transform space. Save and restore the
	// transform manually.
	prev := p.render.GetTransform()
	identity := d2d1.Matrix3x2F{M11: 1, M22: 1}
	p.render.SetTransform(&identity)

	var zero d2d1.RectF
	if p.clip != zero {
		p.render.PopAxisAlignedClip()
		p.clip = zero
	}
	if rect.X != 0 || rect.Y != 0 || rect.Width != 0 || rect.Height != 0 {
		p.clip.Left = rect.X
		p.clip.Top = rect.Y
		p.clip.Right = rect.X + rect.Width
		p.clip.Bottom = rect.Y + rect.Height
		p.render.PushAxisAlignedClip(&p.clip, d2d1.D2D1_ANTIALIAS_MODE_ALIASED)
	}

	p.render.SetTransform(&prev)
}

func (p *Painter) createPathGeometry(path graphics.Path, fill bool) (geometry *d2d1.Geometry, err error) {
	pathGeometry, hr := p.factory.CreatePathGeometry()
	if hr.Failed() {
		return nil, fmt.Errorf("create d2d path geometry err: %v", err)
	}

	sink, hr := pathGeometry.Open()
	if hr.Failed() {
		return nil, fmt.Errorf("open d2d path geometry sink err: %v", err)
	}
	defer sink.Release()

	begFigure := d2d1.D2D1_FIGURE_BEGIN_HOLLOW
	if fill {
		begFigure = d2d1.D2D1_FIGURE_BEGIN_FILLED
	}
	closed := false

	var (
		arc    d2d1.ArcSegment
		bezier d2d1.BezierSegment
	)

	path.Range(func(op graphics.PathOperation, args []float32) (stop bool) {
		switch op {
		case graphics.PathMoveTo:
			sink.BeginFigure(d2d1.Point2F{X: args[0], Y: args[1]}, begFigure)

		case graphics.PathLineTo:
			sink.AddLine(d2d1.Point2F{X: args[0], Y: args[1]})

		case graphics.PathArcTo:
			arc = makeArcSegment(args[0], args[1], args[2], args[3], args[4], args[5], args[6])
			sink.AddArc(&arc)

		case graphics.PathBezierTo:
			bezier = makeBezierSegment(args[0], args[1], args[2], args[3], args[4], args[5])
			sink.AddBezier(&bezier)

		case graphics.PathClose:
			closed = true
			sink.EndFigure(d2d1.D2D1_FIGURE_END_CLOSED)
		}
		return closed
	})
	if !closed {
		sink.EndFigure(d2d1.D2D1_FIGURE_END_OPEN)
	}
	sink.Close()

	return &pathGeometry.Geometry, nil
}

func (p *Painter) setBrush(brush graphics.Brush) *d2d1.Brush {
	switch brush := brush.(type) {
	case graphics.Color:
		return p.setColorBrush(brush)
	case graphics.LinearGradient:
		if brush.Start == brush.End {
			return p.setColorBrush(brush.StartColor)
		}
		if !p.setLinearGradient(brush) {
			return p.setColorBrush(brush.StartColor)
		}
		return &p.linearBrush.Brush
	default:
		return nil
	}
}

func (p *Painter) setColorBrush(color graphics.Color) *d2d1.Brush {
	p.color.R, p.color.G, p.color.B, p.color.A = color.R, color.G, color.B, color.A
	p.colorBrush.SetColor(&p.color)
	return &p.colorBrush.Brush
}

func (p *Painter) setLinearGradient(gradient graphics.LinearGradient) bool {
	if !p.hasLinear || p.linearStart != gradient.StartColor || p.linearEnd != gradient.EndColor {
		if p.linearBrush != nil {
			p.linearBrush.Release()
			p.linearBrush = nil
		}
		if p.linearStops != nil {
			p.linearStops.Release()
			p.linearStops = nil
		}

		stops := [2]d2d1.GradientStop{
			{Position: 0, Color: d2d1.ColorF{R: gradient.StartColor.R, G: gradient.StartColor.G, B: gradient.StartColor.B, A: gradient.StartColor.A}},
			{Position: 1, Color: d2d1.ColorF{R: gradient.EndColor.R, G: gradient.EndColor.G, B: gradient.EndColor.B, A: gradient.EndColor.A}},
		}
		var hr com.HRESULT
		p.linearStops, hr = p.render.CreateGradientStopCollection(stops[:], d2d1.D2D1_GAMMA_2_2, d2d1.D2D1_EXTEND_MODE_CLAMP)
		if hr.Failed() {
			return false
		}
		props := d2d1.LinearGradientBrushProperties{
			StartPoint: d2d1.Point2F{X: gradient.Start.X, Y: gradient.Start.Y},
			EndPoint:   d2d1.Point2F{X: gradient.End.X, Y: gradient.End.Y},
		}
		p.linearBrush, hr = p.render.CreateLinearGradientBrush(&props, nil, p.linearStops)
		if hr.Failed() {
			p.linearStops.Release()
			p.linearStops = nil
			return false
		}
		p.linearStart, p.linearEnd = gradient.StartColor, gradient.EndColor
		p.hasLinear = true
	} else {
		p.linearBrush.SetStartPoint(d2d1.Point2F{X: gradient.Start.X, Y: gradient.Start.Y})
		p.linearBrush.SetEndPoint(d2d1.Point2F{X: gradient.End.X, Y: gradient.End.Y})
	}
	return true
}

func (p *Painter) setRect(rect graphics.Rectangle) {
	leftTop := rect.LeftTop()
	rightBottom := rect.RightBottom()
	p.rect.Left, p.rect.Top = leftTop.X, leftTop.Y
	p.rect.Right, p.rect.Bottom = rightBottom.X, rightBottom.Y
}

func (p *Painter) setRoundRect(rect graphics.Rectangle, radius float32) {
	leftTop := rect.LeftTop()
	rightBottom := rect.RightBottom()
	p.roundRect.Rect.Left, p.roundRect.Rect.Top = leftTop.X, leftTop.Y
	p.roundRect.Rect.Right, p.roundRect.Rect.Bottom = rightBottom.X, rightBottom.Y
	p.roundRect.RadiusX, p.roundRect.RadiusY = radius, radius
}

func (p *Painter) setEllipse(center graphics.Point, radiusX, radiusY float32) {
	p.ellipse.Point.X = center.X
	p.ellipse.Point.Y = center.Y
	p.ellipse.RadiusX = radiusX
	p.ellipse.RadiusY = radiusY
}

func makeArcSegment(rx, ry, xRotation, large, sweep, x, y float32) (arc d2d1.ArcSegment) {
	return d2d1.ArcSegment{
		Point:          d2d1.Point2F{X: x, Y: y},
		Size:           d2d1.SizeF{Width: rx, Height: ry},
		RotationAngle:  xRotation,
		SweepDirection: d2d1.SweepDirection(sweep),
		ArcSize:        d2d1.ArcSize(large),
	}
}

func makeBezierSegment(c1x, c1y, c2x, c2y, x, y float32) d2d1.BezierSegment {
	return d2d1.BezierSegment{
		Point1: d2d1.Point2F{X: c1x, Y: c1y},
		Point2: d2d1.Point2F{X: c2x, Y: c2y},
		Point3: d2d1.Point2F{X: x, Y: y},
	}
}

func (p *Painter) drawNativeImage(rect graphics.Rectangle, d2dBitmap *d2d1.Bitmap) {
	dstRect := d2d1.RectF{
		Left:   rect.X,
		Top:    rect.Y,
		Right:  rect.X + rect.Width,
		Bottom: rect.Y + rect.Height,
	}
	p.render.DrawBitmap(d2dBitmap, &dstRect, 1, d2d1.D2D1_BITMAP_INTERPOLATION_MODE_LINEAR, nil)
}

func (p *Painter) snap(x float32) float32 {
	if p.scale <= 0 || p.transform != geometry.Identity() {
		// Coordinates are local to the active transform. Snapping them before
		// Direct2D applies that transform shifts translated widgets and distorts
		// rotated or scaled geometry. Pixel snapping is only valid when local
		// and target coordinates are the same.
		return x
	}
	// D2D strokes the center of a path. For a stroke to fall entirely within
	// a single physical pixel, its center must align to a pixel center, which
	// in D2D is at half-integer coordinates (0.5, 1.5, 2.5...). Snap to pixel
	// center rather than pixel edge; otherwise a 1px border at an integer
	// coordinate straddles two pixels and appears blurred.
	return (mathx.Floor(x*p.scale) + 0.5) / p.scale
}

func (p *Painter) snapRect(rect graphics.Rectangle) geometry.Rectangle {
	// Snap the top-left corner to pixel center, then compute width/height so
	// the bottom-right corner also lands on a pixel center. Snapping width and
	// height independently would misalign the far edge (snap(X)+snap(W) !=
	// snap(X+W)).
	right := rect.X + rect.Width
	bottom := rect.Y + rect.Height
	rect.X = p.snap(rect.X)
	rect.Y = p.snap(rect.Y)
	rect.Width = p.snap(right) - rect.X
	rect.Height = p.snap(bottom) - rect.Y
	return rect
}

func (p *Painter) snapPoint(pt graphics.Point) graphics.Point {
	pt.X = p.snap(pt.X)
	pt.Y = p.snap(pt.Y)
	return pt
}

func (p *Painter) snapPath(path graphics.Path) graphics.Path {
	var snapped graphics.Path
	empty := true
	path.Range(func(op graphics.PathOperation, args []float32) (stop bool) {
		switch op {
		case graphics.PathMoveTo:
			snapped = graphics.MoveTo(p.snap(args[0]), p.snap(args[1]))
			empty = false
		case graphics.PathLineTo:
			if empty {
				snapped = graphics.MoveTo(p.snap(args[0]), p.snap(args[1]))
				empty = false
			}
			snapped = snapped.LineTo(p.snap(args[0]), p.snap(args[1]))
		case graphics.PathArcTo:
			if empty {
				snapped = graphics.MoveTo(p.snap(args[5]), p.snap(args[6]))
				empty = false
			}
			snapped = snapped.ArcTo(p.snap(args[0]), p.snap(args[1]), p.snap(args[2]), p.snap(args[3]), p.snap(args[4]), p.snap(args[5]), p.snap(args[6]))
		case graphics.PathBezierTo:
			if empty {
				snapped = graphics.MoveTo(p.snap(args[4]), p.snap(args[5]))
				empty = false
			}
			snapped = snapped.BezierTo(
				p.snap(args[0]), p.snap(args[1]),
				p.snap(args[2]), p.snap(args[3]),
				p.snap(args[4]), p.snap(args[5]),
			)
		case graphics.PathClose:
			snapped = snapped.Close()
		}
		return false
	})
	return snapped
}
