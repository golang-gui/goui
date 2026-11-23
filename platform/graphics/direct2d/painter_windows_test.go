package direct2d

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
	"runtime"
	"slices"
	"syscall"
	"testing"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/internal/diagnostic"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
	"github.com/golang-gui/goui/platform/windows/sdk/d2d1"
	"github.com/golang-gui/goui/platform/windows/sdk/d3d11"
	"github.com/golang-gui/goui/platform/windows/sdk/dcomp"
	"github.com/golang-gui/goui/platform/windows/sdk/dxgi"
)

// 原生 Direct2D 是本后端的曲线参考，不要求原生 AA 逼近理想面积积分。
// 两幅图只共用 WARP 目标创建/回读；参考独立设置 DPI、矩阵、clip、颜色，
// 并直接调用原生图元或手写 GeometrySink，不调用生产路径转换、细分或像素对齐 helper。
// 预乘 sRGB/RGBA8，使用可精确表达的颜色，逐像素要求完全相同。
func TestCurvesMatchNativeDirect2D(t *testing.T) {
	p := newRenderImageTestPainter(t)
	arc := graphics.MoveTo(8, 20).ArcTo(12, 8, 17, 1, 1, 32, 20).LineTo(20, 34).Close()
	bezier := graphics.MoveTo(8, 20).BezierTo(8, 4, 32, 4, 32, 20).BezierTo(32, 36, 8, 36, 8, 20).Close()
	ring := graphics.MoveTo(8, 20).ArcTo(12, 8, 0, 0, 1, 32, 20).ArcTo(12, 8, 0, 0, 1, 8, 20).Close().
		MoveTo(12, 20).Hole().ArcTo(8, 4, 0, 0, 0, 28, 20).ArcTo(8, 4, 0, 0, 0, 12, 20).Close()
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, phase := range []float32{0, .5} {
			for _, linear := range []geometry.Transform{geometry.Identity(), geometry.Rotate(23).Scale(1.3, .8)} {
				transform := geometry.Translate(24+phase/scale, 12+phase/scale).Multiply(linear)
				for _, clipped := range []bool{false, true} {
					for _, alpha := range []float32{.5, 1} {
						for _, matte := range []graphics.Color{{}, {R: 1, G: 1, B: 1, A: 1}, {R: .125, G: .125, B: .125, A: 1}} {
							for _, kind := range []string{"ellipse", "ellipse-stroke", "rounded", "rounded-stroke", "arc", "arc-stroke", "bezier", "bezier-stroke", "ring", "erase"} {
								w, h := int(80*scale), int(64*scale)
								got, err := p.RenderImage(w, h, scale, func() {
									p.Clear(matte)
									p.SetTransform(transform)
									if clipped {
										p.SetClipRect(graphics.Rect(34.25, 18.5, 20.5, 22.25))
									}
									ink := graphics.Color{R: .25 * alpha, G: .5 * alpha, B: .75 * alpha, A: alpha}
									switch kind {
									case "ellipse":
										p.FillEllipse(graphics.Point{X: 20, Y: 20}, 12, 8, ink)
									case "ellipse-stroke":
										p.DrawEllipse(graphics.Point{X: 20, Y: 20}, 12, 8, 2, ink)
									case "rounded":
										p.FillRoundRect(graphics.Rect(8, 12, 24, 16), 6, ink)
									case "rounded-stroke":
										p.DrawRoundRect(graphics.Rect(8, 12, 24, 16), 6, 2, ink)
									case "arc":
										p.FillPath(arc, ink)
									case "arc-stroke":
										p.DrawPath(arc, 2, ink)
									case "bezier":
										p.FillPath(bezier, ink)
									case "bezier-stroke":
										p.DrawPath(bezier, 2, ink)
									case "ring":
										p.FillPath(ring, ink)
									case "erase":
										p.ClearPath(bezier)
									}
								})
								if err != nil {
									t.Fatal(err)
								}
								want, err := p.RenderImage(w, h, scale, func() {
									context := p.render
									context.SetDpi(96*scale, 96*scale)
									context.SetAntialiasMode(d2d1.D2D1_ANTIALIAS_MODE_PER_PRIMITIVE)
									context.Clear(&d2d1.ColorF{R: matte.R, G: matte.G, B: matte.B, A: matte.A})
									if clipped {
										// clip 是窗口局部的物理像素向外包络，不随图元变换。
										clip := d2d1.RectF{Left: float32(math.Floor(float64(34.25*scale))) / scale, Top: float32(math.Floor(float64(18.5*scale))) / scale,
											Right: float32(math.Ceil(float64(54.75*scale))) / scale, Bottom: float32(math.Ceil(float64(40.75*scale))) / scale}
										context.PushAxisAlignedClip(&clip, d2d1.D2D1_ANTIALIAS_MODE_ALIASED)
										defer context.PopAxisAlignedClip()
									}
									matrix := d2d1.Matrix3x2F{M11: transform.A11, M12: transform.A21, M21: transform.A12, M22: transform.A22, M31: transform.TX, M32: transform.TY}
									context.SetTransform(&matrix)
									brush, hr := context.CreateSolidColorBrush(&d2d1.ColorF{R: .25, G: .5, B: .75, A: alpha}, nil)
									checkTestHR(t, "创建原生参考画刷", hr)
									defer brush.Release()
									ellipse := d2d1.Ellipse{Point: d2d1.Point2F{X: 20, Y: 20}, RadiusX: 12, RadiusY: 8}
									round := d2d1.RoundRect{Rect: d2d1.RectF{Left: 8, Top: 12, Right: 32, Bottom: 28}, RadiusX: 6, RadiusY: 6}
									if linear == geometry.Identity() {
										// 矩形原语对齐外轮廓，描边随后恢复中心线；自由曲线不对齐。
										half := float32(0)
										if kind == "rounded-stroke" {
											half = 1
										}
										round.Rect = d2d1.RectF{
											Left:   float32(math.Floor(float64((8-half+transform.TX)*scale+.5)))/scale - transform.TX + half,
											Top:    float32(math.Floor(float64((12-half+transform.TY)*scale+.5)))/scale - transform.TY + half,
											Right:  float32(math.Floor(float64((32+half+transform.TX)*scale+.5)))/scale - transform.TX - half,
											Bottom: float32(math.Floor(float64((28+half+transform.TY)*scale+.5)))/scale - transform.TY - half,
										}
									}
									switch kind {
									case "ellipse":
										context.FillEllipse(&ellipse, &brush.Brush)
									case "ellipse-stroke":
										context.DrawEllipse(&ellipse, &brush.Brush, 2, nil)
									case "rounded":
										context.FillRoundedRectangle(&round, &brush.Brush)
									case "rounded-stroke":
										context.DrawRoundedRectangle(&round, &brush.Brush, 2, nil)
									default:
										path := nativeCurveReference(t, p.factory, kind)
										defer path.Release()
										if kind == "erase" {
											brush.SetColor(&d2d1.ColorF{})
											context.SetPrimitiveBlend(d2d1.D2D1_PRIMITIVE_BLEND_COPY)
											defer context.SetPrimitiveBlend(d2d1.D2D1_PRIMITIVE_BLEND_SOURCE_OVER)
										}
										if kind == "arc-stroke" || kind == "bezier-stroke" {
											context.DrawGeometry(&path.Geometry, &brush.Brush, 2, nil)
										} else {
											context.FillGeometry(&path.Geometry, &brush.Brush, nil)
										}
									}
								})
								if err != nil {
									t.Fatal(err)
								}
								if !bytes.Equal(got.(*image.RGBA).Pix, want.(*image.RGBA).Pix) {
									for y := range h {
										for x := range w {
											if got.At(x, y) != want.At(x, y) {
												t.Fatalf("%s/%gx/phase%g/transform%v/clip%t/alpha%g/matte%v (%d,%d)=%v，原生%v", kind, scale, phase, linear, clipped, alpha, matte, x, y, got.At(x, y), want.At(x, y))
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

// 固定输入的独立原生指令，不遍历 graphics.Path 或复用生产路径转换。
func nativeCurveReference(t *testing.T, factory *d2d1.Factory1, kind string) *d2d1.PathGeometry {
	t.Helper()
	path, hr := factory.CreatePathGeometry()
	checkTestHR(t, "创建原生参考路径", hr)
	sink, hr := path.Open()
	if hr.Failed() {
		path.Release()
		t.Fatal("打开原生参考路径", hr)
	}
	defer sink.Release()
	sink.SetFillMode(d2d1.D2D1_FILL_MODE_ALTERNATE)
	begin := d2d1.D2D1_FIGURE_BEGIN_FILLED
	if kind == "arc-stroke" || kind == "bezier-stroke" {
		begin = d2d1.D2D1_FIGURE_BEGIN_HOLLOW
	}
	sink.BeginFigure(d2d1.Point2F{X: 8, Y: 20}, begin)
	switch kind {
	case "arc", "arc-stroke":
		sink.AddArc(&d2d1.ArcSegment{Point: d2d1.Point2F{X: 32, Y: 20}, Size: d2d1.SizeF{Width: 12, Height: 8}, RotationAngle: 17, SweepDirection: d2d1.D2D1_SWEEP_DIRECTION_CLOCKWISE, ArcSize: d2d1.D2D1_ARC_SIZE_LARGE})
		sink.AddLine(d2d1.Point2F{X: 20, Y: 34})
	case "ring":
		sink.AddArc(&d2d1.ArcSegment{Point: d2d1.Point2F{X: 32, Y: 20}, Size: d2d1.SizeF{Width: 12, Height: 8}, SweepDirection: d2d1.D2D1_SWEEP_DIRECTION_CLOCKWISE})
		sink.AddArc(&d2d1.ArcSegment{Point: d2d1.Point2F{X: 8, Y: 20}, Size: d2d1.SizeF{Width: 12, Height: 8}, SweepDirection: d2d1.D2D1_SWEEP_DIRECTION_CLOCKWISE})
		sink.EndFigure(d2d1.D2D1_FIGURE_END_CLOSED)
		sink.BeginFigure(d2d1.Point2F{X: 12, Y: 20}, begin)
		sink.AddArc(&d2d1.ArcSegment{Point: d2d1.Point2F{X: 28, Y: 20}, Size: d2d1.SizeF{Width: 8, Height: 4}, SweepDirection: d2d1.D2D1_SWEEP_DIRECTION_COUNTER_CLOCKWISE})
		sink.AddArc(&d2d1.ArcSegment{Point: d2d1.Point2F{X: 12, Y: 20}, Size: d2d1.SizeF{Width: 8, Height: 4}, SweepDirection: d2d1.D2D1_SWEEP_DIRECTION_COUNTER_CLOCKWISE})
	default:
		sink.AddBezier(&d2d1.BezierSegment{Point1: d2d1.Point2F{X: 8, Y: 4}, Point2: d2d1.Point2F{X: 32, Y: 4}, Point3: d2d1.Point2F{X: 32, Y: 20}})
		sink.AddBezier(&d2d1.BezierSegment{Point1: d2d1.Point2F{X: 32, Y: 36}, Point2: d2d1.Point2F{X: 8, Y: 36}, Point3: d2d1.Point2F{X: 8, Y: 20}})
	}
	sink.EndFigure(d2d1.D2D1_FIGURE_END_CLOSED)
	if hr := sink.Close(); hr.Failed() {
		path.Release()
		t.Fatal("关闭原生参考路径", hr)
	}
	return path
}

func TestCurvePathUpdatesUseCurrentData(t *testing.T) {
	p := newRenderImageTestPainter(t)
	path := graphics.MoveTo(8, 20).BezierTo(8, 4, 32, 4, 32, 20).BezierTo(32, 36, 8, 36, 8, 20).Close()
	render := func() image.Image {
		img, err := p.RenderImage(80, 64, 1, func() { p.FillPath(path, graphics.RGB(255, 255, 255)) })
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	before := render()
	path.Range(func(op graphics.PathOperation, args []float32) bool {
		if op == graphics.PathMoveTo {
			args[0] += 32
		} else if op == graphics.PathBezierTo {
			args[0], args[2], args[4] = args[0]+32, args[2]+32, args[4]+32
		}
		return false
	})
	after := render()
	_, _, _, original := before.At(20, 20).RGBA()
	_, _, _, moved := after.At(52, 20).RGBA()
	_, _, _, cleared := after.At(20, 20).RGBA()
	if original != 65535 || moved != 65535 || cleared != 0 {
		t.Fatal("路径数据更新污染旧图片或复用过时几何", original, moved, cleared)
	}
}

// 相同 WARP、RGBA8 目标和 32 个图元，测量完整离屏绘制及回读；不据此推算桌面 FPS。
func BenchmarkCurvePrimitives(b *testing.B) {
	for _, kind := range []string{"ellipse", "rounded", "path"} {
		b.Run(kind, func(b *testing.B) {
			p := newRenderImageTestPainter(b)
			path := graphics.MoveTo(-8, 0).ArcTo(8, 8, 0, 0, 1, 8, 0).ArcTo(8, 8, 0, 0, 1, -8, 0).Close()
			b.ReportAllocs()
			for b.Loop() {
				_, err := p.RenderImage(256, 128, 1, func() {
					for i := range 32 {
						p.SetTransform(geometry.Translate(16+float32(i%8)*30, 16+float32(i/8)*30))
						switch kind {
						case "ellipse":
							p.FillEllipse(geometry.Point{}, 8, 8, graphics.RGB(40, 100, 220))
						case "rounded":
							p.FillRoundRect(graphics.Rect(-8, -8, 16, 16), 8, graphics.RGB(40, 100, 220))
						case "path":
							p.FillPath(path, graphics.RGB(40, 100, 220))
						}
					}
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// 不断改变 32 个椭圆的几何和像素相位，覆盖缓存未命中的离屏创建/绘制/回读成本。
func BenchmarkCurveGeometryUpdates(b *testing.B) {
	p := newRenderImageTestPainter(b)
	frame := 0
	b.ReportAllocs()
	for b.Loop() {
		_, err := p.RenderImage(256, 128, 1, func() {
			for i := range 32 {
				radius := 7 + float32((frame*32+i)%10000)/10000
				phase := float32(frame%100) / 100
				p.SetTransform(geometry.Translate(16+float32(i%8)*30+phase, 16+float32(i/8)*30+phase))
				p.FillEllipse(geometry.Point{}, radius, radius, graphics.RGB(40, 100, 220))
			}
		})
		if err != nil {
			b.Fatal(err)
		}
		frame++
	}
}

// 大曲面渐变以原局部坐标采样，擦除只影响真实覆盖。
// 预乘 sRGB/RGBA8，满覆盖渐变点保留既有 ±2 量化预算。
func TestCurveGradientAndErase(t *testing.T) {
	p := newRenderImageTestPainter(t)
	for _, scale := range []float32{1, 1.5, 2} {
		transform := geometry.Translate(12.25, 16.5).Rotate(7).Scale(.9, 1.1)
		img, err := p.RenderImage(int(360*scale), int(260*scale), scale, func() {
			p.SetTransform(transform)
			p.SetClipRect(graphics.Rect(4, 4, 340, 240))
			oldAA := p.render.GetAntialiasMode()
			p.FillRoundRect(graphics.Rect(12, 14, 280, 150), 24, graphics.LinearGradient{
				Start: graphics.Point{X: 12}, End: graphics.Point{X: 292},
				StartColor: graphics.RGBA(255, 0, 0, 64), EndColor: graphics.RGBA(0, 0, 255, 192),
			})
			if p.render.GetTransform() != p.matrix || p.render.GetAntialiasMode() != oldAA || !p.clipActive {
				t.Fatal("曲线绘制泄漏绘图状态")
			}
			identity := d2d1.Matrix3x2F{M11: 1, M22: 1}
			if p.linearBrush.GetTransform() != identity {
				t.Fatal("曲线绘制污染渐变画刷变换")
			}
			path := graphics.MoveTo(151, 70).ArcTo(9, 9, 0, 0, 1, 169, 70).ArcTo(9, 9, 0, 0, 1, 151, 70).Close()
			p.ClearPath(path)
			if p.render.GetTransform() != p.matrix || p.render.GetAntialiasMode() != oldAA || p.render.GetPrimitiveBlend() != d2d1.D2D1_PRIMITIVE_BLEND_SOURCE_OVER {
				t.Fatal("曲线擦除泄漏状态")
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		inverse := transform.Inverse()
		for _, pt := range []geometry.Point{{X: 60, Y: 70}, {X: 140, Y: 70}, {X: 160, Y: 70}, {X: 151, Y: 61}, {X: 220, Y: 70}, {X: 270, Y: 100}} {
			world := transform.TransformPoint(pt)
			x, y := int(world.X*scale), int(world.Y*scale)
			local := inverse.TransformPoint(geometry.Point{X: (float32(x) + .5) / scale, Y: (float32(y) + .5) / scale})
			tt := float64((local.X - 12) / 280)
			want := []float64{64 * (1 - tt), 0, 192 * tt, 64*(1-tt) + 192*tt}
			if pt.X == 160 && pt.Y == 70 {
				want = []float64{0, 0, 0, 0}
			}
			got := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
			for i, value := range []uint8{got.R, got.G, got.B, got.A} {
				if math.Abs(float64(value)-want[i]) > 2 {
					t.Fatalf("%gx/%v：%v，应为预乘渐变%v", scale, pt, got, want)
				}
			}
		}
	}
}

// WARP 离屏验证开关前后的像素与逻辑销毁；不需要桌面或 HWND。
func TestDiagnosticImageCountersPreservePixels(t *testing.T) {
	render := func() *image.RGBA {
		return renderAlphaTestImage(t, 1, geometry.Identity(), graphics.Rectangle{}, func(p *Painter) {
			// 此离屏夹具直接 BeginDraw，补齐正式 Begin 的帧序号。
			p.frameID++
			src := image.NewRGBA(image.Rect(0, 0, 2, 2))
			for y := range 2 {
				for x := range 2 {
					src.SetRGBA(x, y, color.RGBA{R: 128, A: 128})
				}
			}
			img, err := p.NewImage(src)
			if err != nil {
				t.Fatal(err)
			}
			if err := img.Update(src); err != nil {
				t.Fatal(err)
			}
			p.DrawImage(graphics.Rect(4, 4, 16, 16), img)
			img.Destroy()
			img.Destroy()
		})
	}
	before := render()
	s, err := diagnostic.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	after := render()
	c := after.RGBAAt(12, 12)
	if !bytes.Equal(before.Pix, after.Pix) || c.R < 127 || c.R > 129 || c.G != 0 || c.B != 0 || c.A < 127 || c.A > 129 {
		t.Fatal("诊断改变了预乘像素或结果为空", c)
	}
	v := s.Values()
	if v[diagnostic.ImageCreate] != 1 || v[diagnostic.ImageUpdate] != 1 || v[diagnostic.ImageDestroy] != 1 || v[diagnostic.ImageBytes] != 32 {
		t.Fatal("图像操作漏计或延迟释放重复计数", v)
	}
}

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

func TestEmptyClipCanBeReplaced(t *testing.T) {
	// 正面积小裁剪现在保留边界像素；用真正零宽矩形继续覆盖空栈项的替换。
	img := renderTestImage(t, 1, geometry.Identity(), graphics.Rect(.2, .2, 0, .2), func(p *Painter) {
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

func checkTestHR(t testing.TB, operation string, hr com.HRESULT) {
	t.Helper()
	if hr.Failed() {
		t.Fatalf("%s: %v", operation, hr)
	}
}

// WARP requires no window or interactive desktop. Lock the calling goroutine
// itself, including test cleanup; these tests intentionally do not use t.Run.
func newRenderImageTestPainter(t testing.TB) *Painter {
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
			{"shadow", 12, 12, color.RGBA{R: 128, A: 128}, func(p *Painter) {
				p.DrawBoxShadow(rect, geometry.UniformCornerRadius(0), graphics.BoxShadow{Color: ink})
			}},
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

func TestPremultipliedGradientInterpolation(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			// 采样落在渐变的 1/4、1/2、3/4 处，远离边缘和抗锯齿。
			// 预期为预乘 sRGB 的独立通道插值；仅容许 8 位量化的 ±2。
			partial := [3]color.RGBA{{R: 48, B: 48, A: 96}, {R: 32, B: 96, A: 128}, {R: 16, B: 144, A: 160}}
			cases := []struct {
				start, end graphics.Color
				reverse    bool
				want       [3]color.RGBA
			}{
				{graphics.RGBA(255, 0, 0, 64), graphics.RGBA(0, 0, 255, 192), false, partial},
				{graphics.RGBA(255, 0, 0, 64), graphics.RGBA(0, 0, 255, 192), true, [3]color.RGBA{partial[2], partial[1], partial[0]}},
				{graphics.RGBA(255, 0, 0, 0), graphics.RGB(0, 0, 255), false, [3]color.RGBA{{B: 64, A: 64}, {B: 128, A: 128}, {B: 191, A: 191}}},
				{graphics.RGBA(255, 0, 0, 64), graphics.RGBA(0, 0, 255, 192), false, partial},
			}
			img := renderAlphaTestImage(t, scale, geometry.Identity(), graphics.Rectangle{}, func(p *Painter) {
				for row, tc := range cases {
					start, end := float32(4)+.5/scale, float32(20)+.5/scale
					if tc.reverse {
						start, end = end, start
					}
					p.FillRect(graphics.Rect(4, float32(4+row*8), 16, 6), graphics.LinearGradient{
						Start: graphics.Point{X: start}, End: graphics.Point{X: end}, StartColor: tc.start, EndColor: tc.end,
					})
				}
			})
			for row, tc := range cases {
				for i, x := range []int{8, 12, 16} {
					got, want := img.RGBAAt(int(float32(x)*scale), int(float32(6+row*8)*scale)), tc.want[i]
					for _, pair := range [][2]byte{{got.R, want.R}, {got.G, want.G}, {got.B, want.B}, {got.A, want.A}} {
						if math.Abs(float64(pair[0])-float64(pair[1])) > 2 {
							t.Fatalf("row=%d x=%d got %v want %v", row, x, got, want)
						}
					}
				}
			}
		})
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
							p.DrawBoxShadow(graphics.Rect(8, 8, 24, 16), geometry.UniformCornerRadius(3), graphics.BoxShadow{
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
			p.DrawBoxShadow(graphics.Rect(8, 8, 24, 16), geometry.UniformCornerRadius(3), graphics.BoxShadow{
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

// 用真实 COM ABI 调用受控 vtable，不创建窗口或依赖 DWM/桌面。
func TestInvalidateCompositionPresentation(t *testing.T) {
	for _, failure := range []string{"", "detach", "commit", "wait", "active"} {
		t.Run(failure, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			var calls []string
			result := func(stage string) uintptr {
				calls = append(calls, stage)
				if stage == failure {
					return 0x80004005
				}
				return 0
			}
			visualClass := dcomp.VisualClass{SetContent: cgo.Symbol(syscall.NewCallback(func(_ uintptr, content uintptr) uintptr {
				if content != 0 {
					t.Error("失效时重新挂上了旧 swap chain")
				}
				return result("detach")
			}))}
			deviceClass := dcomp.DeviceClass{
				Commit:                  cgo.Symbol(syscall.NewCallback(func(uintptr) uintptr { return result("commit") })),
				WaitForCommitCompletion: cgo.Symbol(syscall.NewCallback(func(uintptr) uintptr { return result("wait") })),
			}
			visual := &dcomp.Visual{Unknown: com.Unknown{Class: unsafe.Pointer(&visualClass)}}
			device := &dcomp.Device{Unknown: com.Unknown{Class: unsafe.Pointer(&deviceClass)}}
			var pin runtime.Pinner
			pin.Pin(&visualClass)
			pin.Pin(&deviceClass)
			pin.Pin(visual)
			pin.Pin(device)
			defer pin.Unpin()
			p := &Painter{composition: device, compositionVisual: visual, activeFrame: failure == "active"}
			err := p.InvalidatePresentation()
			want := []string{"detach", "commit", "wait"}
			switch failure {
			case "detach":
				want = want[:1]
			case "commit":
				want = want[:2]
			case "active":
				want = nil
			}
			if !slices.Equal(calls, want) || (err == nil) != (failure == "") {
				t.Fatalf("失效调用顺序或错误结果：calls=%v want=%v err=%v", calls, want, err)
			}
			if p.composition != device || p.compositionVisual != visual {
				t.Fatal("失效重建了 compositor 对象")
			}
			if failure == "" && !p.compositionDetached {
				t.Fatal("新帧完成前没有保留 detached 状态")
			}
		})
	}
}

func TestFractionalClipEnvelopeAndEmptyRestore(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		clip := graphics.Rect(2.5/scale, 3.25/scale, 3/scale, 2.5/scale)
		img := renderAlphaTestImage(t, scale, geometry.Identity(), clip, func(p *Painter) {
			p.SetTransform(geometry.Translate(-4, -4))
			p.FillRect(graphics.Rect(0, 0, 30, 30), graphics.RGBA(255, 0, 0, 128))
		})
		for y := 0; y < 12; y++ {
			for x := 0; x < 12; x++ {
				want := color.RGBA{}
				if x >= 2 && x < 6 && y >= 3 && y < 6 {
					want = color.RGBA{R: 128, A: 128}
				}
				got := img.RGBAAt(x, y)
				if math.Abs(float64(got.R)-float64(want.R)) > 1 || math.Abs(float64(got.A)-float64(want.A)) > 1 || got.G != 0 || got.B != 0 {
					t.Fatalf("%gx (%d,%d)=%v want=%v", scale, x, y, got, want)
				}
			}
		}
	}
	img := renderAlphaTestImage(t, 1, geometry.Identity(), graphics.Rectangle{}, func(p *Painter) {
		p.SetClipRect(graphics.Rect(.5, .5, 0, 4))
		p.FillRect(graphics.Rect(0, 0, 12, 12), graphics.RGB(255, 0, 0))
		if !p.clipActive {
			t.Fatal("空裁剪丢失原生栈状态")
		}
		p.SetClipRect(graphics.Rectangle{})
		p.FillRect(graphics.Rect(2, 2, 2, 2), graphics.RGB(0, 255, 0))
		p.SetClipRect(graphics.Rect(8.1, 8.1, .1, .1))
		p.FillRect(graphics.Rect(0, 0, 12, 12), graphics.RGB(0, 0, 255))
	})
	for _, tc := range []struct {
		x, y int
		want color.RGBA
	}{{0, 0, color.RGBA{}}, {2, 2, color.RGBA{G: 255, A: 255}}, {8, 8, color.RGBA{B: 255, A: 255}}, {9, 8, color.RGBA{}}} {
		if got := img.RGBAAt(tc.x, tc.y); got != tc.want {
			t.Fatalf("空裁剪／恢复(%d,%d)=%v want=%v", tc.x, tc.y, got, tc.want)
		}
	}
}
