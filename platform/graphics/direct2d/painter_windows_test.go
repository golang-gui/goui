package direct2d

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"runtime"
	"testing"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
	"github.com/golang-gui/goui/platform/windows/sdk/d2d1"
	"github.com/golang-gui/goui/platform/windows/sdk/d3d11"
	"github.com/golang-gui/goui/platform/windows/sdk/dxgi"
)

func TestSwapChainPresentationPolicy(t *testing.T) {
	opaque := swapChainCandidates(false)
	if opaque[0].SwapEffect != dxgi.DXGI_SWAP_EFFECT_SEQUENTIAL {
		t.Fatal("opaque HWND must prefer resize-synchronized sequential presentation")
	}
	for _, desc := range opaque {
		if desc.SwapEffect != dxgi.DXGI_SWAP_EFFECT_SEQUENTIAL && desc.SwapEffect != dxgi.DXGI_SWAP_EFFECT_DISCARD {
			t.Fatal("opaque fallback reintroduced asynchronous HWND flip presentation")
		}
		if desc.BufferCount != 1 || desc.Scaling != dxgi.DXGI_SCALING_STRETCH || desc.Flags != 0 {
			t.Fatalf("invalid blt-model descriptor: %+v", desc)
		}
	}
	composition := swapChainCandidates(true)
	for i, desc := range composition {
		flags := uint32(0)
		if i == 0 {
			flags = dxgi.DXGI_SWAP_CHAIN_FLAG_FRAME_LATENCY_WAITABLE_OBJECT
		}
		if desc.SwapEffect != dxgi.DXGI_SWAP_EFFECT_FLIP_SEQUENTIAL ||
			desc.Scaling != dxgi.DXGI_SCALING_STRETCH || desc.BufferCount != 2 || desc.Flags != flags {
			t.Fatalf("transparent composition contract changed: %+v", desc)
		}
	}
	for _, transparent := range []bool{false, true} {
		for _, desc := range swapChainCandidates(transparent) {
			if desc.Format != dxgi.DXGI_FORMAT_B8G8R8A8_UNORM || desc.SampleDesc.Count != 1 ||
				desc.BufferUsage != dxgi.DXGI_USAGE_RENDER_TARGET_OUTPUT {
				t.Fatalf("incompatible Direct2D buffer: %+v", desc)
			}
		}
	}
}

// Exercise the native rasterizer, not just the coordinate helpers: a correctly
// inset border can still be blurred or cut in half by transform/clip handling.
func TestRectanglePixelsWithWidgetTranslationAndClip(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2} {
		for _, pixels := range []int{0, 1, 2, 3} {
			for _, radius := range []float32{0, 3} {
				t.Run(fmt.Sprintf("scale=%g/pixels=%d/radius=%g", scale, pixels, radius), func(t *testing.T) {
					width := float32(pixels) / scale
					outer := graphics.Rect(0, 0, 23.4, 17.7)
					offset := graphics.Point{X: 6.2, Y: 5.3}
					clip := outer.Translate(offset)
					draw := func(p *Painter, r graphics.Rectangle) {
						black := graphics.RGB(0, 0, 0)
						if pixels == 0 {
							if radius > 0 {
								p.FillRoundRect(r, radius, black)
							} else {
								p.FillRect(r, black)
							}
						} else if radius > 0 {
							p.DrawRoundRect(r.Inset(width/2), radius, width, black)
						} else {
							p.DrawRect(r.Inset(width/2), width, black)
						}
					}
					img := renderTestImage(t, scale, geometry.Translate(offset.X, offset.Y), clip, func(p *Painter) {
						draw(p, outer)
					})
					// Clipping to a widget's own bounds must not weaken its border.
					unclipped := renderTestImage(t, scale, geometry.Translate(offset.X, offset.Y), graphics.Rectangle{}, func(p *Painter) {
						draw(p, outer)
					})
					assertSamePixels(t, img, unclipped)

					left := int(math.Round(float64(clip.X * scale)))
					top := int(math.Round(float64(clip.Y * scale)))
					right := int(math.Round(float64((clip.X + clip.Width) * scale)))
					bottom := int(math.Round(float64((clip.Y + clip.Height) * scale)))
					// Test all four straight edges, away from rounded corners.
					for x := left - 1; x <= right; x++ {
						inside := x >= left && x < right
						ink := inside && (pixels == 0 || x < left+pixels || x >= right-pixels)
						assertMonochromePixel(t, img, x, (top+bottom)/2, ink)
					}
					for y := top - 1; y <= bottom; y++ {
						inside := y >= top && y < bottom
						ink := inside && (pixels == 0 || y < top+pixels || y >= bottom-pixels)
						assertMonochromePixel(t, img, (left+right)/2, y, ink)
					}
				})
			}
		}
	}
}

func TestFractionalDIPBorderKeepsStrokeCoverage(t *testing.T) {
	for _, scale := range []float32{1.25, 1.5, 1.75} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			outer := graphics.Rect(0, 0, 23.4, 17.7)
			offset := graphics.Point{X: 6.2, Y: 5.3}
			img := renderTestImage(t, scale, geometry.Translate(offset.X, offset.Y), outer.Translate(offset), func(p *Painter) {
				p.DrawRect(outer.Inset(0.5), 1, graphics.RGB(0, 0, 0))
			})
			left := int(math.Round(float64(offset.X * scale)))
			y := int((offset.Y + outer.Height/2) * scale)
			assertMonochromePixel(t, img, left-1, y, false)
			assertMonochromePixel(t, img, left, y, true)
			var coverage float64
			for x := left; x < left+3; x++ {
				coverage += float64(255-img.RGBAAt(x, y).R) / 255
			}
			if math.Abs(coverage-float64(scale)) > 0.02 {
				t.Fatalf("1 DIP stroke at scale %g covers %g pixels", scale, coverage)
			}
		})
	}
}

func TestEmptyRoundedClipCanBeReplaced(t *testing.T) {
	img := renderTestImage(t, 1, geometry.Identity(), graphics.Rect(0, 0, 0.2, 0.2), func(p *Painter) {
		p.FillRect(graphics.Rect(0, 0, 20, 20), graphics.RGB(0, 0, 0))
		p.SetClipRect(graphics.Rectangle{})
		p.FillRect(graphics.Rect(4, 4, 2, 2), graphics.RGB(0, 0, 0))
	})
	assertMonochromePixel(t, img, 0, 0, false)
	assertMonochromePixel(t, img, 4, 4, true)
	assertMonochromePixel(t, img, 6, 6, false)
}

func TestStraightLinePixelsWithTranslation(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2} {
		for _, pixels := range []int{1, 2} {
			for _, vertical := range []bool{false, true} {
				t.Run(fmt.Sprintf("scale=%g/pixels=%d/vertical=%t", scale, pixels, vertical), func(t *testing.T) {
					width := float32(pixels) / scale
					p0, p1 := graphics.Point{X: 0, Y: width / 2}, graphics.Point{X: 20, Y: width / 2}
					if vertical {
						p0, p1 = graphics.Point{X: width / 2, Y: 0}, graphics.Point{X: width / 2, Y: 20}
					}
					img := renderTestImage(t, scale, geometry.Translate(6.2, 5.3), graphics.Rectangle{}, func(p *Painter) {
						p.DrawLine(p0, p1, width, graphics.RGB(0, 0, 0))
					})
					for delta := -1; delta <= pixels; delta++ {
						x, y := 15, int(math.Round(float64(float32(5.3)*scale)))+delta
						if vertical {
							x, y = int(math.Round(float64(float32(6.2)*scale)))+delta, 15
						}
						assertMonochromePixel(t, img, x, y, delta >= 0 && delta < pixels)
					}
				})
			}
		}
	}
}

func TestPathAndImageKeepSubpixelGeometry(t *testing.T) {
	// A whole-device-pixel translation must only translate the result. The old
	// identity-only snapping altered path parameters and image sampling.
	for _, kind := range []string{"path", "ellipse", "image", "diagonal", "rotated rectangle"} {
		t.Run(kind, func(t *testing.T) {
			draw := func(p *Painter) {
				black := graphics.RGB(0, 0, 0)
				switch kind {
				case "path":
					path := graphics.MoveTo(5.2, 5.3).LineTo(17.8, 5.3).
						ArcTo(3.2, 4.7, 0, 0, 1, 21, 10).
						BezierTo(20.6, 16.1, 9.7, 17.3, 5.2, 5.3).Close()
					p.FillPath(path, black)
				case "ellipse":
					p.FillEllipse(graphics.Point{X: 13.2, Y: 12.3}, 7.4, 5.2, black)
				case "image":
					pixels := image.NewRGBA(image.Rect(0, 0, 2, 2))
					pixels.SetRGBA(0, 0, color.RGBA{A: 255})
					pixels.SetRGBA(1, 1, color.RGBA{A: 255})
					img, err := p.NewImage(pixels)
					if err != nil {
						t.Fatal(err)
					}
					p.DrawImage(graphics.Rect(5.2, 5.3, 14.7, 13.6), img)
				case "diagonal":
					p.DrawLine(graphics.Point{X: 5.2, Y: 5.3}, graphics.Point{X: 19.6, Y: 15.7}, 1.3, black)
				case "rotated rectangle":
					p.SetTransform(p.transform.Multiply(geometry.Translate(12, 12).Rotate(17)))
					p.DrawRect(graphics.Rect(-4.2, -4.3, 8.4, 8.6), 1.3, black)
				}
			}
			plain := renderTestImage(t, 1, geometry.Identity(), graphics.Rectangle{}, draw)
			shifted := renderTestImage(t, 1, geometry.Translate(3, 2), graphics.Rectangle{}, draw)
			for y := 0; y < 30; y++ {
				for x := 0; x < 30; x++ {
					if a, b := plain.RGBAAt(x, y), shifted.RGBAAt(x+3, y+2); a != b {
						t.Fatalf("translation changed rasterization at (%d,%d): %v -> %v", x, y, a, b)
					}
				}
			}
		})
	}
}

func assertSamePixels(t *testing.T, a, b *image.RGBA) {
	t.Helper()
	for y := 0; y < a.Rect.Dy(); y++ {
		for x := 0; x < a.Rect.Dx(); x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				t.Fatalf("clipped and unclipped drawing differ at (%d,%d): %v != %v", x, y, a.RGBAAt(x, y), b.RGBAAt(x, y))
			}
		}
	}
}

func assertMonochromePixel(t *testing.T, img *image.RGBA, x, y int, ink bool) {
	t.Helper()
	want := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	if ink {
		want = color.RGBA{A: 255}
	}
	if got := img.RGBAAt(x, y); got != want {
		t.Fatalf("pixel (%d,%d) = %v, want %v (edge blurred or clipped)", x, y, got, want)
	}
}

// An offscreen WARP-backed target keeps the regression independent of desktop
// capture, monitor DPI, window occlusion, and swap-chain timing. Use the SDK's
// existing vtable slots for test-only bitmap allocation and CPU readback.
func renderTestImage(t *testing.T, scale float32, transform geometry.Transform, clip graphics.Rectangle, draw func(*Painter)) *image.RGBA {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	p := &Painter{scale: scale, images: make(map[*imageResource]struct{})}
	defer p.Destroy()
	var err error
	p.factory, err = d2d1.CreateFactory[d2d1.Factory1](d2d1.D2D1_FACTORY_TYPE_SINGLE_THREADED, d2d1.IID_ID2D1Factory1, nil)
	if err != nil {
		t.Fatal(err)
	}
	var immediate *d3d11.DeviceContext
	var hr com.HRESULT
	p.d3dDevice, _, immediate, hr = d3d11.CreateDevice(d3d11.D3D_DRIVER_TYPE_WARP, d3d11.D3D11_CREATE_DEVICE_BGRA_SUPPORT, []d3d11.FeatureLevel{d3d11.D3D_FEATURE_LEVEL_11_0})
	if immediate != nil {
		defer immediate.Release()
	}
	checkTestHR(t, "create WARP device", hr)
	var unknown *com.Unknown
	checkTestHR(t, "query DXGI device", p.d3dDevice.QueryInterface(dxgi.IID_IDXGIDevice, &unknown))
	p.dxgiDevice = (*dxgi.Device)(unsafe.Pointer(unknown))
	p.d2dDevice, hr = p.factory.CreateDevice(p.dxgiDevice)
	checkTestHR(t, "create Direct2D device", hr)
	p.render, hr = p.d2dDevice.CreateDeviceContext(d2d1.D2D1_DEVICE_CONTEXT_OPTIONS_NONE)
	checkTestHR(t, "create Direct2D context", hr)
	p.colorBrush, hr = p.render.CreateSolidColorBrush(&p.color, nil)
	checkTestHR(t, "create brush", hr)
	const width, height = 80, 64
	bitmap := func(options d2d1.BitmapOptions) *d2d1.Bitmap1 {
		props := d2d1.BitmapProperties1{
			PixelFormat: d2d1.PixelFormat{Format: dxgi.DXGI_FORMAT_B8G8R8A8_UNORM, AlphaMode: d2d1.D2D1_ALPHA_MODE_IGNORE},
			DpiX:        96 * scale, DpiY: 96 * scale, BitmapOptions: options,
		}
		var result *d2d1.Bitmap1
		method := (*d2d1.DeviceContextClass)(p.render.Class).CreateBitmap1
		hr := cgo.CallRet[com.HRESULT](method, p.render, d2d1.SizeU{Width: width, Height: height}, uintptr(0), uint32(0), &props, &result)
		checkTestHR(t, "create bitmap", hr)
		return result
	}
	p.target = bitmap(d2d1.D2D1_BITMAP_OPTIONS_TARGET | d2d1.D2D1_BITMAP_OPTIONS_CANNOT_DRAW)
	p.render.SetTarget((*d2d1.Image)(unsafe.Pointer(p.target)))
	p.render.SetDpi(96*scale, 96*scale)
	p.render.BeginDraw()
	p.activeFrame = true
	// Also balance the native frame if an assertion in draw aborts the test.
	defer func() {
		if p.activeFrame {
			p.SetClipRect(graphics.Rectangle{})
			p.render.EndDraw(nil, nil)
			p.activeFrame = false
		}
	}()
	p.SetTransform(transform)
	p.Clear(graphics.RGB(255, 255, 255))
	p.SetClipRect(clip)
	draw(p)
	p.SetClipRect(graphics.Rectangle{})
	hr = p.render.EndDraw(nil, nil)
	p.activeFrame = false
	checkTestHR(t, "EndDraw", hr)

	const cpuRead d2d1.BitmapOptions = 4 // D2D1_BITMAP_OPTIONS_CPU_READ
	readback := bitmap(cpuRead | d2d1.D2D1_BITMAP_OPTIONS_CANNOT_DRAW)
	defer readback.Release()
	class := (*d2d1.Bitmap1Class)(readback.Class)
	ret, _, _ := class.CopyFromBitmap.CallRaw(uintptr(unsafe.Pointer(readback)), 0, uintptr(unsafe.Pointer(p.target)), 0)
	checkTestHR(t, "copy bitmap", com.HRESULT(ret))
	var mapped struct {
		Pitch uint32
		Bits  *byte
	}
	ret, _, _ = class.Map.CallRaw(uintptr(unsafe.Pointer(readback)), 1, uintptr(unsafe.Pointer(&mapped))) // D2D1_MAP_OPTIONS_READ
	checkTestHR(t, "map bitmap", com.HRESULT(ret))
	defer class.Unmap.CallRaw(uintptr(unsafe.Pointer(readback)))
	data := unsafe.Slice(mapped.Bits, int(mapped.Pitch)*height)
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*int(mapped.Pitch) + x*4
			img.SetRGBA(x, y, color.RGBA{R: data[i+2], G: data[i+1], B: data[i], A: 255})
		}
	}
	return img
}

func checkTestHR(t *testing.T, operation string, hr com.HRESULT) {
	t.Helper()
	if hr.Failed() {
		t.Fatalf("%s: %v", operation, hr)
	}
}

// WARP requires no window or interactive desktop. Lock the calling goroutine
// itself, including test cleanup; these tests intentionally do not use t.Run.
func newRenderImageTestPainter(t *testing.T) *Painter {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	p := &Painter{images: make(map[*imageResource]struct{})}
	t.Cleanup(p.Destroy)
	var err error
	p.factory, err = d2d1.CreateFactory[d2d1.Factory1](d2d1.D2D1_FACTORY_TYPE_SINGLE_THREADED, d2d1.IID_ID2D1Factory1, nil)
	if err != nil {
		t.Fatal(err)
	}
	var immediate *d3d11.DeviceContext
	var hr com.HRESULT
	p.d3dDevice, _, immediate, hr = d3d11.CreateDevice(d3d11.D3D_DRIVER_TYPE_WARP, d3d11.D3D11_CREATE_DEVICE_BGRA_SUPPORT, []d3d11.FeatureLevel{d3d11.D3D_FEATURE_LEVEL_11_0})
	checkTestHR(t, "create WARP device", hr)
	if immediate != nil {
		immediate.Release()
	}
	var unknown *com.Unknown
	checkTestHR(t, "query DXGI", p.d3dDevice.QueryInterface(dxgi.IID_IDXGIDevice, &unknown))
	p.dxgiDevice = (*dxgi.Device)(unsafe.Pointer(unknown))
	p.d2dDevice, hr = p.factory.CreateDevice(p.dxgiDevice)
	checkTestHR(t, "create D2D device", hr)
	p.render, hr = p.d2dDevice.CreateDeviceContext(d2d1.D2D1_DEVICE_CONTEXT_OPTIONS_NONE)
	checkTestHR(t, "create context", hr)
	p.colorBrush, hr = p.render.CreateSolidColorBrush(&p.color, nil)
	checkTestHR(t, "create brush", hr)
	return p
}

func TestRenderImageResourceReuseAndRestore(t *testing.T) {
	p := newRenderImageTestPainter(t)
	p.render.SetTextAntialiasMode(d2d1.D2D1_TEXT_ANTIALIAS_MODE_CLEARTYPE)
	p.render.SetDpi(120, 144)
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for i := 0; i < len(src.Pix); i += 4 {
		src.Pix[i], src.Pix[i+3] = 128, 128
	}
	native, err := p.NewImage(src)
	if err != nil {
		t.Fatal(err)
	}
	defer native.Destroy()
	for _, scale := range []float32{1, 2} {
		img, err := p.RenderImage(int(12*scale), int(10*scale), scale, func() {
			p.SetClipRect(graphics.Rect(2, 2, 5, 5))
			p.SetTransform(geometry.Translate(2, 2))
			p.DrawImage(graphics.Rect(0, 0, 8, 8), native)
			if _, err := p.RenderImage(1, 1, 1, func() {}); err == nil {
				t.Fatal("nested accepted")
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		// Interior UNORM samples, premultiplied RGBA; no AA edge tolerance needed.
		for _, tc := range []struct {
			x, y int
			want color.RGBA
		}{
			{0, 0, color.RGBA{}}, {3, 3, color.RGBA{R: 128, A: 128}}, {8, 3, color.RGBA{}},
		} {
			got := color.RGBAModel.Convert(img.At(tc.x*int(scale), tc.y*int(scale))).(color.RGBA)
			if got != tc.want {
				t.Fatalf("scale=%g: %v want %v", scale, got, tc.want)
			}
		}
		x, y := p.render.GetDpi()
		if x != 120 || y != 144 || p.activeFrame || p.clipActive || p.render.GetTextAntialiasMode() != d2d1.D2D1_TEXT_ANTIALIAS_MODE_CLEARTYPE {
			t.Fatal("state not restored")
		}
		if _, err := p.RenderImage(12, 10, 1, func() { p.Clear(graphics.RGB(0, 255, 0)) }); err != nil {
			t.Fatal(err)
		}
		if img.At(0, 0) != (color.RGBA{}) {
			t.Fatal("image aliases storage")
		}
	}
}

func TestRenderImagePanicCleanup(t *testing.T) {
	p := newRenderImageTestPainter(t)
	var retained graphics.Image
	func() {
		defer func() {
			if recover() != "paint failure" {
				t.Fatal("wrong panic")
			}
		}()
		_, _ = p.RenderImage(8, 8, 1, func() {
			retained, _ = p.NewImage(image.NewRGBA(image.Rect(0, 0, 2, 2)))
			retired, _ := p.NewImage(image.NewRGBA(image.Rect(0, 0, 2, 2)))
			p.DrawImage(graphics.Rect(0, 0, 2, 2), retired)
			retired.Destroy()
			p.SetClipRect(graphics.Rect(1, 1, 2, 2))
			panic("paint failure")
		})
	}()
	if p.activeFrame || p.clipActive || p.pendingImages != 0 || len(p.images) != 1 {
		t.Fatal("frame cleanup failed")
	}
	if _, err := p.RenderImage(8, 8, 1, func() { p.DrawImage(graphics.Rect(0, 0, 2, 2), retained) }); err != nil {
		t.Fatal(err)
	}
	retained.Destroy()
}

func TestImageDeferredDestroy(t *testing.T) {
	for _, deviceLost := range []bool{false, true} {
		p := &Painter{images: make(map[*imageResource]struct{}), activeFrame: true}
		img := &imageResource{owner: p, width: 2, height: 2, pixels: graphics.Bitmap{Pixels: make([]byte, 16)}}
		p.images[img] = struct{}{}
		img.Destroy()
		img.Destroy()
		if !img.destroyed || !img.pendingDestroy || img.pixels.Pixels == nil || p.pendingImages != 1 {
			t.Fatal("Destroy must invalidate once and retain storage until EndDraw")
		}
		if err := img.Update(image.NewRGBA(image.Rect(0, 0, 2, 2))); err == nil {
			t.Fatal("destroyed image accepted update")
		}
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("destroyed image accepted drawing")
				}
			}()
			p.DrawImage(graphics.Rect(0, 0, 2, 2), img)
		}()
		if deviceLost {
			p.releaseDeviceResources()
		} else {
			p.activeFrame = false
			p.flushPendingImages()
		}
		if img.owner != nil || img.pixels.Pixels != nil || img.pendingDestroy || p.pendingImages != 0 || len(p.images) != 0 {
			t.Fatal("deferred resource retained after EndDraw/device loss")
		}
		img.Destroy()
		outside := &imageResource{owner: p}
		p.images[outside] = struct{}{}
		outside.Destroy()
		if outside.owner != nil {
			t.Fatal("outside-frame release was deferred")
		}
		alive := &imageResource{owner: p}
		p.images[alive] = struct{}{}
		p.Destroy()
		if alive.owner != nil || !alive.destroyed {
			t.Fatal("Painter did not release remaining image")
		}
	}
}

// WARP offscreen rendering requires Windows, but no desktop or HWND.
func TestRecordedImageSurvivesDestroy(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		got := renderAlphaTestImage(t, scale, geometry.Identity(), graphics.Rectangle{}, func(p *Painter) {
			src := image.NewRGBA(image.Rect(0, 0, 1, 1))
			src.SetRGBA(0, 0, color.RGBA{R: 128, A: 128})
			img, err := p.NewImage(src)
			if err != nil {
				t.Fatal(err)
			}
			p.DrawImage(graphics.Rect(4, 4, 16, 16), img)
			img.Destroy()
			if img.(*imageResource).bitmap == nil {
				t.Fatal("bitmap released before EndDraw")
			}
		})
		// Interior premultiplied sample; no AA boundary, ±1 for 8-bit rounding.
		c := got.RGBAAt(12*int(scale), 12*int(scale))
		if c.R < 127 || c.R > 129 || c.G != 0 || c.B != 0 || c.A < 127 || c.A > 129 {
			t.Fatal(c)
		}
	}
}

func TestPremultipliedNativeColors(t *testing.T) {
	ink := graphics.RGBA(255, 0, 0, 128)
	for _, scale := range []float32{1, 2} {
		gradient := graphics.LinearGradient{Start: graphics.Point{X: 4 + .5/scale}, End: graphics.Point{X: 20 + .5/scale}, StartColor: graphics.RGBA(255, 0, 0, 64), EndColor: graphics.RGBA(0, 0, 255, 192)}
		rect := graphics.Rect(4, 4, 16, 16)
		for _, tc := range []struct {
			name string
			x, y int
			want color.RGBA
			draw func(*Painter)
		}{
			{"clear", 12, 12, color.RGBA{R: 128, A: 128}, func(p *Painter) { p.Clear(ink) }},
			{"fill", 12, 12, color.RGBA{R: 128, A: 128}, func(p *Painter) { p.FillRect(rect, ink) }},
			{"stroke", 12, 4, color.RGBA{R: 128, A: 128}, func(p *Painter) { p.DrawRect(rect, 4, ink) }},
			{"gradient", 12, 12, color.RGBA{R: 32, B: 96, A: 128}, func(p *Painter) { p.FillRect(rect, gradient) }},
			{"stroke-gradient", 12, 4, color.RGBA{R: 32, B: 96, A: 128}, func(p *Painter) { p.DrawRect(rect, 4, gradient) }},
			{"degenerate", 12, 12, color.RGBA{R: 128, A: 128}, func(p *Painter) { p.FillRect(rect, graphics.LinearGradient{StartColor: ink, EndColor: ink}) }},
			{"shadow", 12, 12, color.RGBA{R: 128, A: 128}, func(p *Painter) { p.DrawBoxShadow(rect, 0, graphics.BoxShadow{Color: ink}) }},
			{"over-white", 12, 12, color.RGBA{R: 255, G: 127, B: 127, A: 255}, func(p *Painter) { p.Clear(graphics.RGB(255, 255, 255)); p.FillRect(rect, ink) }},
		} {
			t.Run(fmt.Sprintf("%s/%g", tc.name, scale), func(t *testing.T) {
				img := renderAlphaTestImage(t, scale, geometry.Identity(), graphics.Rectangle{}, tc.draw)
				got := img.RGBAAt(tc.x*int(scale), tc.y*int(scale))
				for _, pair := range [][2]byte{{got.R, tc.want.R}, {got.G, tc.want.G}, {got.B, tc.want.B}, {got.A, tc.want.A}} {
					if math.Abs(float64(pair[0])-float64(pair[1])) > 2 {
						t.Fatalf("got %v want %v", got, tc.want)
					}
				}
			})
		}
	}
}

func TestD2DColorBoundary(t *testing.T) {
	for _, alpha := range []byte{0, 1, 64, 128, 255} {
		c := graphics.RGBA(255, 100, 60, alpha)
		native := d2dColor(c)
		for _, pair := range [][2]float32{{native.R * native.A, c.R}, {native.G * native.A, c.G}, {native.B * native.A, c.B}, {native.A, c.A}} {
			if math.Abs(float64(pair[0]-pair[1])) > 1e-6 {
				t.Fatalf("incorrect native conversion: %+v -> %+v", c, native)
			}
		}
	}
}

// WARP + CPU-readable bitmaps; no HWND, display connection or desktop needed.
func renderAlphaTestImage(t *testing.T, scale float32, transform geometry.Transform, clip graphics.Rectangle, draw func(*Painter)) *image.RGBA {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	p := &Painter{scale: scale, images: make(map[*imageResource]struct{})}
	defer p.Destroy()
	var err error
	p.factory, err = d2d1.CreateFactory[d2d1.Factory1](d2d1.D2D1_FACTORY_TYPE_SINGLE_THREADED, d2d1.IID_ID2D1Factory1, nil)
	if err != nil {
		t.Fatal(err)
	}
	var immediate *d3d11.DeviceContext
	var hr com.HRESULT
	p.d3dDevice, _, immediate, hr = d3d11.CreateDevice(d3d11.D3D_DRIVER_TYPE_WARP, d3d11.D3D11_CREATE_DEVICE_BGRA_SUPPORT, []d3d11.FeatureLevel{d3d11.D3D_FEATURE_LEVEL_11_0})
	if immediate != nil {
		defer immediate.Release()
	}
	checkTestHR(t, "create WARP device", hr)
	var unknown *com.Unknown
	checkTestHR(t, "query DXGI device", p.d3dDevice.QueryInterface(dxgi.IID_IDXGIDevice, &unknown))
	p.dxgiDevice = (*dxgi.Device)(unsafe.Pointer(unknown))
	p.d2dDevice, hr = p.factory.CreateDevice(p.dxgiDevice)
	checkTestHR(t, "create Direct2D device", hr)
	p.render, hr = p.d2dDevice.CreateDeviceContext(d2d1.D2D1_DEVICE_CONTEXT_OPTIONS_NONE)
	checkTestHR(t, "create Direct2D context", hr)
	p.colorBrush, hr = p.render.CreateSolidColorBrush(&p.color, nil)
	checkTestHR(t, "create brush", hr)
	const width, height = 80, 64
	bitmap := func(options d2d1.BitmapOptions) *d2d1.Bitmap1 {
		props := d2d1.BitmapProperties1{
			PixelFormat: d2d1.PixelFormat{Format: dxgi.DXGI_FORMAT_B8G8R8A8_UNORM, AlphaMode: d2d1.D2D1_ALPHA_MODE_PREMULTIPLIED},
			DpiX:        96 * scale, DpiY: 96 * scale, BitmapOptions: options,
		}
		var result *d2d1.Bitmap1
		method := (*d2d1.DeviceContextClass)(p.render.Class).CreateBitmap1
		hr := cgo.CallRet[com.HRESULT](method, p.render, d2d1.SizeU{Width: width, Height: height}, uintptr(0), uint32(0), &props, &result)
		checkTestHR(t, "create bitmap", hr)
		return result
	}
	p.target = bitmap(d2d1.D2D1_BITMAP_OPTIONS_TARGET | d2d1.D2D1_BITMAP_OPTIONS_CANNOT_DRAW)
	p.render.SetTarget((*d2d1.Image)(unsafe.Pointer(p.target)))
	p.render.SetDpi(96*scale, 96*scale)
	p.render.BeginDraw()
	p.activeFrame = true
	// Also balance the native frame if an assertion in draw aborts the test.
	defer func() {
		if p.activeFrame {
			p.SetClipRect(graphics.Rectangle{})
			p.render.EndDraw(nil, nil)
			p.activeFrame = false
		}
	}()
	p.SetTransform(transform)
	p.Clear(graphics.Color{})
	p.SetClipRect(clip)
	draw(p)
	p.SetClipRect(graphics.Rectangle{})
	hr = p.render.EndDraw(nil, nil)
	p.activeFrame = false
	checkTestHR(t, "EndDraw", hr)

	const cpuRead d2d1.BitmapOptions = 4 // D2D1_BITMAP_OPTIONS_CPU_READ
	readback := bitmap(cpuRead | d2d1.D2D1_BITMAP_OPTIONS_CANNOT_DRAW)
	defer readback.Release()
	class := (*d2d1.Bitmap1Class)(readback.Class)
	ret, _, _ := class.CopyFromBitmap.CallRaw(uintptr(unsafe.Pointer(readback)), 0, uintptr(unsafe.Pointer(p.target)), 0)
	checkTestHR(t, "copy bitmap", com.HRESULT(ret))
	var mapped struct {
		Pitch uint32
		Bits  *byte
	}
	ret, _, _ = class.Map.CallRaw(uintptr(unsafe.Pointer(readback)), 1, uintptr(unsafe.Pointer(&mapped))) // D2D1_MAP_OPTIONS_READ
	checkTestHR(t, "map bitmap", com.HRESULT(ret))
	defer class.Unmap.CallRaw(uintptr(unsafe.Pointer(readback)))
	data := unsafe.Slice(mapped.Bits, int(mapped.Pitch)*height)
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*int(mapped.Pitch) + x*4
			img.SetRGBA(x, y, color.RGBA{R: data[i+2], G: data[i+1], B: data[i], A: data[i+3]})
		}
	}
	return img
}

func TestBoxShadowPremultipliedColor(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, mode := range []string{"clear", "soft", "fallback"} {
			for _, layers := range []int{1, 2} {
				t.Run(fmt.Sprintf("scale=%g/%s/layers=%d", scale, mode, layers), func(t *testing.T) {
					img := renderTestImage(t, scale, geometry.Identity(), graphics.Rectangle{}, func(p *Painter) {
						if mode == "soft" {
							context, hr := p.d2dDevice.CreateDeviceContext(d2d1.D2D1_DEVICE_CONTEXT_OPTIONS_NONE)
							p.shadowRender = context
							checkTestHR(t, "create shadow context", hr)
							white := d2d1.ColorF{R: 1, G: 1, B: 1, A: 1}
							p.shadowBrush, hr = p.shadowRender.CreateSolidColorBrush(&white, nil)
							checkTestHR(t, "create shadow brush", hr)
							p.shadowEffect, hr = p.render.CreateEffect(d2d1.CLSID_D2D1Shadow)
							checkTestHR(t, "create shadow effect", hr)
						}
						blur := float32(4)
						if mode == "clear" {
							blur = 0
						}
						for range layers {
							p.DrawBoxShadow(graphics.Rect(8, 8, 24, 16), 3, graphics.BoxShadow{
								Color: graphics.ColorOf(color.RGBA{R: 96, G: 48, B: 24, A: 128}), BlurRadius: blur,
							})
						}
						if mode == "soft" && len(p.shadowCache) != 1 {
							t.Fatal("soft shadow did not create/reuse its native mask")
						}
					})
					// The native fixture clears to white. Test source-over, not just
					// alpha: incorrect straight/premultiplied conversion darkens RGB.
					want := color.RGBA{R: 223, G: 175, B: 151, A: 255}
					if layers == 2 {
						want = color.RGBA{R: 207, G: 135, B: 99, A: 255}
					}
					got := img.RGBAAt(int(20*scale), int(16*scale))
					if mode == "soft" && img.RGBAAt(int(7*scale), int(16*scale)).G >= 250 {
						t.Fatal("native effect did not produce a soft edge (unexpected clear fallback)")
					}
					for i, pair := range [][2]uint8{{got.R, want.R}, {got.G, want.G}, {got.B, want.B}, {got.A, want.A}} {
						if math.Abs(float64(pair[0])-float64(pair[1])) > 2 {
							t.Fatalf("channel %d: pixel = %v, want %v", i, got, want)
						}
					}
				})
			}
		}
	}
}

func TestBoxShadowTransparentDoesNotDraw(t *testing.T) {
	img := renderTestImage(t, 1, geometry.Identity(), graphics.Rectangle{}, func(p *Painter) {
		for _, blur := range []float32{0, 4} {
			p.DrawBoxShadow(graphics.Rect(8, 8, 24, 16), 3, graphics.BoxShadow{
				Color: graphics.Color{R: 1, G: 1, B: 1}, BlurRadius: blur,
			})
		}
	})
	for _, b := range img.Pix {
		if b != 255 {
			t.Fatal("zero-alpha shadow changed the white target")
		}
	}
}
