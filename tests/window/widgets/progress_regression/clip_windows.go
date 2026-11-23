package main

import (
	"image"
	"image/color"
	"math"
	"runtime"
	"unsafe"

	"github.com/golang-gui/goui/platform/windows/sdk/com"
	"github.com/golang-gui/goui/platform/windows/sdk/d2d1"
	"github.com/golang-gui/goui/platform/windows/sdk/d3d11"
	"github.com/golang-gui/goui/platform/windows/sdk/dxgi"
)

func init() { nativeArcReference = renderNativeArcs }

// 独立原生指令与窗口实际 Direct2D 输出对照；不经过 GOUI Painter、路径转换或像素对齐。
// WARP/BGRA8/预乘/sRGB，与RenderWidget的DPI及结构clip一致。全部资源在调用线程内释放。
func renderNativeArcs(s *clipScene, bounds image.Rectangle) image.Image {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	check := func(hr com.HRESULT) {
		if hr.Failed() {
			panic(hr)
		}
	}
	factory, err := d2d1.CreateFactory[d2d1.Factory1](d2d1.D2D1_FACTORY_TYPE_SINGLE_THREADED, d2d1.IID_ID2D1Factory1, nil)
	must(err)
	defer factory.Release()
	device, _, immediate, hr := d3d11.CreateDevice(d3d11.D3D_DRIVER_TYPE_WARP, d3d11.D3D11_CREATE_DEVICE_BGRA_SUPPORT, []d3d11.FeatureLevel{d3d11.D3D_FEATURE_LEVEL_11_0})
	check(hr)
	defer device.Release()
	if immediate != nil {
		defer immediate.Release()
	}
	var unknown *com.Unknown
	check(device.QueryInterface(dxgi.IID_IDXGIDevice, &unknown))
	defer unknown.Release()
	d2dDevice, hr := factory.CreateDevice((*dxgi.Device)(unsafe.Pointer(unknown)))
	check(hr)
	defer d2dDevice.Release()
	context, hr := d2dDevice.CreateDeviceContext(d2d1.D2D1_DEVICE_CONTEXT_OPTIONS_NONE)
	check(hr)
	defer context.Release()
	bitmap := func(options d2d1.BitmapOptions) *d2d1.Bitmap1 {
		result, hr := context.CreateBitmap1(d2d1.SizeU{Width: uint32(bounds.Dx()), Height: uint32(bounds.Dy())}, nil, 0, &d2d1.BitmapProperties1{
			PixelFormat: d2d1.PixelFormat{Format: dxgi.DXGI_FORMAT_B8G8R8A8_UNORM, AlphaMode: d2d1.D2D1_ALPHA_MODE_PREMULTIPLIED},
			DpiX:        96 * s.scale, DpiY: 96 * s.scale, BitmapOptions: options,
		})
		check(hr)
		return result
	}
	target := bitmap(d2d1.D2D1_BITMAP_OPTIONS_TARGET | d2d1.D2D1_BITMAP_OPTIONS_CANNOT_DRAW)
	defer target.Release()
	context.SetTarget((*d2d1.Image)(unsafe.Pointer(target)))
	defer context.SetTarget(nil)
	context.SetDpi(96*s.scale, 96*s.scale)
	context.BeginDraw()
	drawing := true
	defer func() {
		if drawing {
			context.EndDraw(nil, nil)
		}
	}()
	context.Clear(&d2d1.ColorF{})
	brush, hr := context.CreateSolidColorBrush(&d2d1.ColorF{}, nil)
	check(hr)
	defer brush.Release()
	track := s.trackColor()
	for _, ring := range s.rings {
		r := ring.widget.Rect()
		radius := r.Width / 2
		identity := d2d1.Matrix3x2F{M11: 1, M22: 1}
		context.SetTransform(&identity)
		clip := d2d1.RectF{
			Left: float32(math.Floor(float64(r.X*s.scale))) / s.scale, Top: float32(math.Floor(float64(r.Y*s.scale))) / s.scale,
			Right: float32(math.Ceil(float64((r.X+r.Width)*s.scale))) / s.scale, Bottom: float32(math.Ceil(float64((r.Y+r.Height)*s.scale))) / s.scale,
		}
		context.PushAxisAlignedClip(&clip, d2d1.D2D1_ANTIALIAS_MODE_ALIASED)
		matrix := d2d1.Matrix3x2F{M11: 1, M22: 1, M31: r.X, M32: r.Y}
		context.SetTransform(&matrix)
		brush.SetColor(&d2d1.ColorF{R: float32(track.R) / 255, G: float32(track.G) / 255, B: float32(track.B) / 255, A: 1})
		path := nativeArcGeometry(factory, radius, true, check)
		context.FillGeometry(&path.Geometry, &brush.Brush, nil)
		path.Release()
		// 与公开角度约定相同的float32弧度；独立构造绕圆心的原生行向量矩阵。
		rad := -float32(s.angle) * math.Pi / 180
		c, sn := float32(math.Cos(float64(rad))), float32(math.Sin(float64(rad)))
		matrix = d2d1.Matrix3x2F{M11: c, M12: -sn, M21: sn, M22: c,
			M31: r.X + c*(-radius) + sn*(-radius) + radius, M32: r.Y - sn*(-radius) + c*(-radius) + radius}
		context.SetTransform(&matrix)
		brush.SetColor(&d2d1.ColorF{R: 69.0 / 255, G: 128.0 / 255, B: 217.0 / 255, A: float32(ring.alpha) / 255})
		path = nativeArcGeometry(factory, radius, false, check)
		context.FillGeometry(&path.Geometry, &brush.Brush, nil)
		path.Release()
		context.PopAxisAlignedClip()
	}
	hr = context.EndDraw(nil, nil)
	drawing = false
	check(hr)
	readback := bitmap(d2d1.D2D1_BITMAP_OPTIONS_CPU_READ | d2d1.D2D1_BITMAP_OPTIONS_CANNOT_DRAW)
	defer readback.Release()
	check(readback.CopyFromBitmap(nil, &target.Bitmap, nil))
	mapped, hr := readback.Map(d2d1.D2D1_MAP_OPTIONS_READ)
	check(hr)
	defer readback.Unmap()
	data := unsafe.Slice(mapped.Bits, int(mapped.Pitch)*bounds.Dy())
	img := image.NewRGBA(bounds)
	for y := range bounds.Dy() {
		for x := range bounds.Dx() {
			i := y*int(mapped.Pitch) + 4*x
			img.SetRGBA(x, y, color.RGBA{R: data[i+2], G: data[i+1], B: data[i], A: data[i+3]})
		}
	}
	return img
}

func nativeArcGeometry(factory *d2d1.Factory1, radius float32, full bool, check func(com.HRESULT)) *d2d1.PathGeometry {
	path, hr := factory.CreatePathGeometry()
	check(hr)
	ready := false
	defer func() {
		if !ready {
			path.Release()
		}
	}()
	sink, hr := path.Open()
	check(hr)
	defer sink.Release()
	sink.SetFillMode(d2d1.D2D1_FILL_MODE_ALTERNATE)
	sink.BeginFigure(d2d1.Point2F{X: radius}, d2d1.D2D1_FIGURE_BEGIN_FILLED)
	arc := func(x, y, r float32, sweep d2d1.SweepDirection) {
		sink.AddArc(&d2d1.ArcSegment{Point: d2d1.Point2F{X: x, Y: y}, Size: d2d1.SizeF{Width: r, Height: r}, SweepDirection: sweep})
	}
	inner := radius - 4
	if full {
		arc(radius, 2*radius, radius, d2d1.D2D1_SWEEP_DIRECTION_CLOCKWISE)
		arc(radius, 0, radius, d2d1.D2D1_SWEEP_DIRECTION_CLOCKWISE)
		sink.EndFigure(d2d1.D2D1_FIGURE_END_CLOSED)
		sink.BeginFigure(d2d1.Point2F{X: radius, Y: 4}, d2d1.D2D1_FIGURE_BEGIN_FILLED)
		arc(radius, radius+inner, inner, d2d1.D2D1_SWEEP_DIRECTION_COUNTER_CLOCKWISE)
		arc(radius, 4, inner, d2d1.D2D1_SWEEP_DIRECTION_COUNTER_CLOCKWISE)
	} else {
		arc(2*radius, radius, radius, d2d1.D2D1_SWEEP_DIRECTION_CLOCKWISE)
		arc(radius+inner, radius, 2, d2d1.D2D1_SWEEP_DIRECTION_CLOCKWISE)
		arc(radius, 4, inner, d2d1.D2D1_SWEEP_DIRECTION_COUNTER_CLOCKWISE)
		arc(radius, 0, 2, d2d1.D2D1_SWEEP_DIRECTION_CLOCKWISE)
	}
	sink.EndFigure(d2d1.D2D1_FIGURE_END_CLOSED)
	check(sink.Close())
	ready = true
	return path
}
