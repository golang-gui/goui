package gui

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

type styledBoxCommands struct {
	styledCommandPainter
	paths int
}

func (p *styledBoxCommands) FillPath(graphics.Path, graphics.Brush) { p.paths++ }

func TestStyledOpaqueMatchingFillCommands(t *testing.T) {
	ink := color.RGBA{48, 120, 196, 255}
	rect := geometry.Rect(6, 6, 18, 18)
	base := style.Default().BackgroundColor(ink).BorderColor(ink).BorderWidth(1).Radius(4)
	for _, tc := range []struct {
		name                  string
		rule                  style.Rule
		fills, borders, paths int
	}{
		{"同色不透明", base, 1, 0, 0},
		{"不同颜色类型", base.BackgroundColor(color.NRGBA{48, 120, 196, 255}), 1, 0, 0},
		{"等价16位颜色", base.BorderColor(color.RGBA64{48 * 257, 120 * 257, 196 * 257, 65535}), 1, 0, 0},
		{"不同16位颜色", base.BorderColor(color.RGBA64{48*257 + 1, 120 * 257, 196 * 257, 65535}), 1, 0, 1},
		{"异色边框", base.BorderColor(color.Black), 1, 0, 1},
		{"同色半透明", base.BackgroundColor(color.NRGBA{48, 120, 196, 128}).BorderColor(color.NRGBA{48, 120, 196, 128}), 1, 1, 0},
		{"半透明背景", base.BackgroundColor(color.NRGBA{48, 120, 196, 128}), 1, 1, 0},
		{"独立选边", base.BorderSides(style.BorderTop), 0, 0, 2},
		{"空边集合", base.BorderSides(0), 1, 0, 0},
		{"零边宽", base.BorderWidth(0), 1, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := new(styledBoxCommands)
			PaintStyledBox(p, rect, tc.rule.Style)
			if p.fills != tc.fills || p.borders != tc.borders || p.paths != tc.paths {
				t.Fatal("样式绘制指令错误", p.fills, p.borders, p.paths)
			}
		})
	}
}

// 样式的外框按设备边界对齐，内框保留小数宽度；只在远离圆角的直边采样。
// 预乘 sRGB/RGBA8 的预期直接来自像素与环带的面积交集，允许 1 级量化误差。
func TestStyledFrameStraightCoverage(t *testing.T) {
	backend, err := software.NewPainter(&widgetRenderSurface{})
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	rect := geometry.Rect(4.25, 5.5, 32.5, 24.25)
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, offset := range []float32{0, .5} {
			for _, radius := range []geometry.CornerRadius{geometry.UniformCornerRadius(4), {TopLeft: 4, BottomRight: 3}} {
				for _, sides := range []style.BorderSides{style.BorderAll, style.BorderTop | style.BorderLeft} {
					for _, width := range []float32{1, 1.5} {
						for _, alpha := range []uint8{128, 255} {
							for _, entry := range []string{"background", "border", "box"} {
								ink := color.NRGBA{R: 40, G: 100, B: 220, A: alpha}
								rule := style.Default().CornerRadius(radius).BorderSides(sides).BorderWidth(width / scale).BorderColor(ink)
								if entry == "background" {
									rule = rule.BackgroundColor(ink)
								}
								img, err := backend.RenderImage(int(48*scale), int(40*scale), scale, func() {
									p := newPainter(backend, geometry.Rect(0, 0, 48, 40), scale)
									p.SetTransform(geometry.Translate(offset, offset))
									switch entry {
									case "background":
										PaintStyledBackground(p, rect, rule.Style)
									case "border":
										PaintStyledBorder(p, rect, rule.Style)
									case "box":
										PaintStyledBox(p, rect, rule.Style)
									}
								})
								if err != nil {
									t.Fatal(err)
								}
								left := int(math.Floor(float64((rect.X+offset)*scale) + .5))
								top := int(math.Floor(float64((rect.Y+offset)*scale) + .5))
								right := int(math.Floor(float64((rect.X+rect.Width+offset)*scale) + .5))
								bottom := int(math.Floor(float64((rect.Y+rect.Height+offset)*scale) + .5))
								for side, pt := range []image.Point{{X: (left + right) / 2, Y: top}, {X: left, Y: (top + bottom) / 2}, {X: right - 1, Y: (top + bottom) / 2}} {
									for delta := -1; delta <= 3; delta++ {
										coverage := float64(0)
										if delta >= 0 {
											if entry == "background" {
												coverage = 1
											} else if side != 2 || sides&style.BorderRight != 0 {
												coverage = max(0, min(1, float64(width)-float64(delta)))
											}
										}
										point := pt
										if side == 0 {
											point.Y += delta
										} else if side == 1 {
											point.X += delta
										} else {
											point.X -= delta
										}
										var want color.RGBA
										for i, value := range []*uint8{&want.R, &want.G, &want.B, &want.A} {
											component := []uint8{ink.R, ink.G, ink.B, 255}[i]
											*value = uint8(math.Round(float64(component) * float64(alpha) / 255 * coverage))
										}
										got := color.RGBAModel.Convert(img.At(point.X, point.Y)).(color.RGBA)
										if !closePixel(got, want, 1) {
											t.Fatalf("%s scale=%g offset=%g radius=%v sides=%v width=%g alpha=%d side=%d delta=%d：%v，应为%v", entry, scale, offset, radius, sides, width, alpha, side, delta, got, want)
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

// 预乘sRGB/RGBA8，独立圆方程检测整颗像素及0.05物理像素余量都处于形内。
// 此区域必须完全不透明，只容许1级颜色量化误差，不能用AA容差掩盖角内漏像素。
func TestStyledOpaqueMatchingFillPixels(t *testing.T) {
	backend, err := software.NewPainter(&widgetRenderSurface{})
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	rect := geometry.Rect(6, 6, 18, 18)
	ink := color.RGBA{48, 120, 196, 255}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, offset := range []float32{0, .25, .5, .75} {
			for _, radius := range []geometry.CornerRadius{geometry.UniformCornerRadius(4), {TopLeft: 4, TopRight: 2, BottomRight: 7, BottomLeft: 1}} {
				for _, rotated := range []bool{false, true} {
					transform := geometry.Translate(offset, offset)
					if rotated {
						transform = geometry.Translate(12+offset, 4+offset).Multiply(geometry.Rotate(14))
					}
					for _, width := range []float32{1, 2, 10} {
						s := style.Default().BackgroundColor(ink).BorderColor(ink).BorderWidth(width).CornerRadius(radius).Style
						im, err := backend.RenderImage(int(48*scale), int(48*scale), scale, func() {
							p := newPainter(backend, geometry.Rect(0, 0, 48, 48), scale)
							p.SetTransform(transform)
							PaintStyledBox(p, rect, s)
						})
						if err != nil {
							t.Fatal(err)
						}
						// 样式外框在纯平移时统一取整；旋转保留原始几何。
						ref := rect
						if !rotated {
							left := float32(math.Floor(float64((rect.X+offset)*scale)+.5))/scale - offset
							top := float32(math.Floor(float64((rect.Y+offset)*scale)+.5))/scale - offset
							right := float32(math.Floor(float64((rect.X+rect.Width+offset)*scale)+.5))/scale - offset
							bottom := float32(math.Floor(float64((rect.Y+rect.Height+offset)*scale)+.5))/scale - offset
							ref = geometry.Rect(left, top, right-left, bottom-top)
						}
						inverse := transform.Inverse()
						inside, antialiased := 0, 0
						for y := 0; y < im.Bounds().Dy(); y++ {
							for x := 0; x < im.Bounds().Dx(); x++ {
								got := color.RGBAModel.Convert(im.At(x, y)).(color.RGBA)
								if got.A > 0 && got.A < 255 {
									antialiased++
								}
								covered := true
								for _, delta := range []geometry.Point{{X: -.05, Y: -.05}, {X: 1.05, Y: -.05}, {X: -.05, Y: 1.05}, {X: 1.05, Y: 1.05}} {
									point := inverse.TransformPoint(geometry.Point{X: (float32(x) + delta.X) / scale, Y: (float32(y) + delta.Y) / scale})
									if !borderSidesContains(ref, radius, 0, 0, point) {
										covered = false
										break
									}
								}
								if covered {
									inside++
									if got.A != 255 || !closePixel(got, ink, 1) {
										t.Fatalf("scale=%g offset=%g radius=%v rotated=%t width=%g pixel=(%d,%d)：角内%v，应为%v", scale, offset, radius, rotated, width, x, y, got, ink)
									}
								}
							}
						}
						if inside < 100 || antialiased == 0 {
							t.Fatal("缺少形内或抗锯齿边界采样", inside, antialiased)
						}
					}
				}
			}
		}
	}
}

// 独立用椭圆方程判断覆盖，不调用生产路径生成函数。采样距离轮廓至少
// 1.5 DIP（含clip），排除平台AA曲线；满覆盖预乘颜色仅允许2级量化误差。
func TestStyledBorderSidesPixels(t *testing.T) {
	backend, err := software.NewPainter(&widgetRenderSurface{})
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	rect := geometry.Rect(8, 8, 64, 48)
	bg := color.NRGBA{R: 60, G: 110, B: 160, A: 128}
	for _, scale := range []float32{1, 1.25, 2} {
		for _, transform := range []geometry.Transform{geometry.Identity(), geometry.Translate(30, 0).Multiply(geometry.Rotate(20))} {
			for _, radius := range []geometry.CornerRadius{{}, geometry.UniformCornerRadius(12), {TopLeft: 24, BottomRight: 12, BottomLeft: 4}} {
				for sides := style.BorderSides(0); sides <= style.BorderAll; sides++ {
					for _, alpha := range []uint8{128, 255} {
						t.Run(fmt.Sprintf("%gx/%v/%v/sides%d/alpha%d", scale, transform, radius, sides, alpha), func(t *testing.T) {
							border := color.NRGBA{R: 230, G: 200, B: 170, A: alpha}
							s := style.Default().CornerRadius(radius).BackgroundColor(bg).BorderColor(border).BorderWidth(4).BorderSides(sides).Style
							clip := geometry.Rect(5, 6, 68, 52)
							im, err := backend.RenderImage(int(96*scale), int(96*scale), scale, func() {
								p := newPainter(backend, geometry.Rect(0, 0, 96, 96), scale)
								p.SetTransform(transform)
								p.SetClipRect(clip)
								PaintStyledBox(p, rect, s)
							})
							if err != nil {
								t.Fatal(err)
							}
							// 每份参考色由标准库Over计算，确保透明度没有在角部叠加。
							ref := image.NewRGBA(image.Rect(0, 0, 1, 1))
							reference := func(bits uint8) color.RGBA {
								ref.SetRGBA(0, 0, color.RGBA{})
								if bits&1 != 0 {
									draw.Draw(ref, ref.Bounds(), image.NewUniform(bg), image.Point{}, draw.Over)
								}
								if bits&2 != 0 {
									draw.Draw(ref, ref.Bounds(), image.NewUniform(border), image.Point{}, draw.Over)
								}
								return ref.RGBAAt(0, 0)
							}
							coverage := func(pt geometry.Point) uint8 {
								// GUI的clip位于布局坐标，不跟随控件自己的绘制变换。
								cp := transform.TransformPoint(pt)
								if cp.X < clip.X || cp.Y < clip.Y || cp.X > clip.X+clip.Width || cp.Y > clip.Y+clip.Height {
									return 0
								}
								var bits uint8
								bgWidth := float32(0)
								if alpha == 255 {
									bgWidth = 2
								}
								if borderSidesContains(rect, radius, sides, bgWidth, pt) {
									bits |= 1
								}
								if borderSidesContains(rect, radius, 0, 0, pt) && !borderSidesContains(rect, radius, sides, 4, pt) {
									bits |= 2
								}
								return bits
							}
							inverse := transform.Inverse()
							checked := 0
							for y := 1; y < im.Bounds().Dy(); y += 2 {
								for x := 1; x < im.Bounds().Dx(); x += 2 {
									pt := inverse.TransformPoint(geometry.Point{X: (float32(x) + .5) / scale, Y: (float32(y) + .5) / scale})
									bits := coverage(pt)
									stable := true
									for _, offset := range []geometry.Point{{X: -1.5}, {X: 1.5}, {Y: -1.5}, {Y: 1.5}} {
										if coverage(geometry.Point{X: pt.X + offset.X, Y: pt.Y + offset.Y}) != bits {
											stable = false
										}
									}
									if !stable {
										continue
									}
									want := reference(bits)
									got := color.RGBAModel.Convert(im.At(x, y)).(color.RGBA)
									if !closePixel(got, want, 2) {
										t.Fatalf("像素(%d,%d)局部%v：得到%v，方程参考%v", x, y, pt, got, want)
									}
									checked++
								}
							}
							if checked < 100 {
								t.Fatal("没有足够的内部采样", checked)
							}
						})
					}
				}
			}
		}
	}
}

func closePixel(got, want color.RGBA, tolerance int) bool {
	return math.Abs(float64(int(got.R)-int(want.R))) <= float64(tolerance) &&
		math.Abs(float64(int(got.G)-int(want.G))) <= float64(tolerance) &&
		math.Abs(float64(int(got.B)-int(want.B))) <= float64(tolerance) &&
		math.Abs(float64(int(got.A)-int(want.A))) <= float64(tolerance)
}

func borderSidesContains(rect geometry.Rectangle, radius geometry.CornerRadius, sides style.BorderSides, width float32, pt geometry.Point) bool {
	var inset [4]float32 // 左、上、右、下
	for n, side := range []style.BorderSides{style.BorderLeft, style.BorderTop, style.BorderRight, style.BorderBottom} {
		if sides&side != 0 {
			inset[n] = width
		}
	}
	x, y := pt.X-rect.X-inset[0], pt.Y-rect.Y-inset[1]
	w, h := rect.Width-inset[0]-inset[2], rect.Height-inset[1]-inset[3]
	if x < 0 || y < 0 || x > w || y > h || w <= 0 || h <= 0 {
		return false
	}
	for n, r := range []float32{radius.TopLeft, radius.TopRight, radius.BottomRight, radius.BottomLeft} {
		left, top := n == 0 || n == 3, n < 2
		sx, sy, dx, dy := inset[2], inset[3], w-x, h-y
		if left {
			sx, dx = inset[0], x
		}
		if top {
			sy, dy = inset[1], y
		}
		rx, ry := max(r-sx, 0), max(r-sy, 0)
		if rx > 0 && ry > 0 && dx < rx && dy < ry {
			u, v := (dx-rx)/rx, (dy-ry)/ry
			if u*u+v*v > 1 {
				return false
			}
		}
	}
	return true
}

func TestStyledBorderSidesThickAndNone(t *testing.T) {
	backend, err := software.NewPainter(&widgetRenderSurface{})
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	for _, tc := range []struct {
		sides style.BorderSides
		width float32
		point image.Point
		want  color.RGBA
	}{
		{style.BorderTop, 28, image.Pt(40, 40), color.RGBA{}}, // 单边大于半高仍不能提前填满
		{style.BorderTop, 48, image.Pt(40, 40), color.RGBA{A: 128}},
		{style.BorderTop | style.BorderBottom, 24, image.Pt(40, 32), color.RGBA{A: 128}},
		{0, 100, image.Pt(8, 32), color.RGBA{}},
	} {
		im, err := backend.RenderImage(96, 96, 1, func() {
			p := newPainter(backend, geometry.Rect(0, 0, 96, 96), 1)
			s := style.Default().Radius(12).BorderWidth(tc.width).BorderColor(color.NRGBA{A: 128}).BorderSides(tc.sides).Style
			PaintStyledBorder(p, geometry.Rect(8, 8, 64, 48), s)
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := color.RGBAModel.Convert(im.At(tc.point.X, tc.point.Y)).(color.RGBA); got != tc.want {
			t.Fatalf("粗边框%v/%g在%v：得到%v，期望%v", tc.sides, tc.width, tc.point, got, tc.want)
		}
	}
}

func TestStyledBorderSidesDefaultAndBackground(t *testing.T) {
	backend, err := software.NewPainter(&widgetRenderSurface{})
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	rule := style.Default().CornerRadius(geometry.CornerRadius{TopLeft: 16, BottomLeft: 8}).BorderWidth(4).
		BorderColor(color.White).BackgroundColor(color.NRGBA{R: 60, G: 110, B: 160, A: 128})
	render := func(s style.Style, background bool) image.Image {
		im, err := backend.RenderImage(96, 96, 1, func() {
			p := newPainter(backend, geometry.Rect(0, 0, 96, 96), 1)
			if background {
				PaintStyledBackground(p, geometry.Rect(8, 8, 64, 48), s)
			} else {
				PaintStyledBox(p, geometry.Rect(8, 8, 64, 48), s)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		return im
	}
	for _, pair := range [][2]image.Image{
		{render(rule.Style, false), render(rule.BorderSides(style.BorderAll).Style, false)},
		{render(rule.BorderSides(0).Style, false), render(rule.Style, true)},
	} {
		for y := 0; y < 96; y++ {
			for x := 0; x < 96; x++ {
				if !colors.Equal(pair[0].At(x, y), pair[1].At(x, y)) {
					t.Fatalf("默认四边或显式空集的背景不兼容：(%d,%d)", x, y)
				}
			}
		}
	}
}

// 超粗单边不能从缺边方向越出另一端的圆角。离轮廓至少1.5DIP的外部
// 像素必须透明；同时检查内部确实绘制，不能用丢弃整个路径掩盖越界。
func TestStyledBorderSidesThickCornerBounds(t *testing.T) {
	backend, err := software.NewPainter(&widgetRenderSurface{})
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	rect := geometry.Rect(8, 8, 64, 48)
	for _, radius := range []geometry.CornerRadius{geometry.UniformCornerRadius(12), {BottomRight: 40}} {
		for _, width := range []float32{28, 44, 46} {
			for _, alpha := range []uint8{128, 255} {
				s := style.Default().CornerRadius(radius).BorderWidth(width).BorderSides(style.BorderTop).
					BorderColor(color.NRGBA{A: alpha}).BackgroundColor(color.White).Style
				im, err := backend.RenderImage(96, 96, 1, func() {
					p := newPainter(backend, geometry.Rect(0, 0, 96, 96), 1)
					PaintStyledBox(p, rect, s)
				})
				if err != nil {
					t.Fatal(err)
				}
				for y := 0; y < 96; y++ {
					for x := 0; x < 96; x++ {
						outside := true
						for _, dx := range []float32{-1.5, 0, 1.5} {
							for _, dy := range []float32{-1.5, 0, 1.5} {
								if borderSidesContains(rect, radius, 0, 0, geometry.Point{X: float32(x) + .5 + dx, Y: float32(y) + .5 + dy}) {
									outside = false
								}
							}
						}
						if _, _, _, a := im.At(x, y).RGBA(); outside && a != 0 {
							t.Fatalf("r%v/w%g/a%d：像素(%d,%d)超出圆角，alpha=%d", radius, width, alpha, x, y, a)
						}
					}
				}
				want := color.RGBA{A: 255}
				if alpha == 128 {
					want = color.RGBA{R: 127, G: 127, B: 127, A: 255}
				}
				got := color.RGBAModel.Convert(im.At(32, 24)).(color.RGBA)
				if !closePixel(got, want, 2) {
					t.Fatalf("粗边框内部丢失：r%v/w%g/a%d=%v，期望%v", radius, width, alpha, got, want)
				}
			}
		}
	}
}

// 预乘 sRGB RGBA：不透明边框的外缘只应含边框色，不能混入下方背景。
// 不透明组合用完整边框色填充作为参考，其余组合用独立边框；
// 两张独立遮罩共同筛出外轮廓的部分覆盖像素，
// 排除内边缘和中心，允许两级8位量化误差，不依赖后端AA曲线的具体形状。
func TestStyledBoxOpaqueBorderOuterPixels(t *testing.T) {
	backend, err := software.NewPainter(&widgetRenderSurface{})
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, radius := range []geometry.CornerRadius{{TopLeft: 24, BottomRight: 12, BottomLeft: 4}, geometry.UniformCornerRadius(12)} {
			for _, alpha := range []uint8{128, 255} {
				for _, transform := range []geometry.Transform{geometry.Identity(), {A12: -1, A21: 1, TX: 80}, geometry.Scale(1.1, .8)} {
					t.Run(fmt.Sprintf("%gx/%v/alpha%d/%v", scale, radius, alpha, transform), func(t *testing.T) {
						s := style.Default().CornerRadius(radius).BackgroundColor(color.NRGBA{R: 60, G: 110, B: 160, A: alpha}).BorderWidth(4).BorderColor(color.Gray{Y: 230}).Style
						render := func(mode int) image.Image {
							im, err := backend.RenderImage(int(96*scale), int(96*scale), scale, func() {
								p := newPainter(backend, geometry.Rect(0, 0, 96, 96), scale)
								p.SetTransform(transform)
								p.SetClipRect(geometry.Rect(3, 3, 85, 85))
								r := geometry.Rect(8, 8, 64, 48)
								switch mode {
								case 0:
									PaintStyledBox(p, r, s)
								case 1:
									if alpha == 255 {
										// 不透明组合的外缘来自一次完整边框色填充。
										// 不要求原生填充与中心线描边采用相同 AA 曲线。
										PaintStyledBackground(p, r, style.Default().CornerRadius(radius).BackgroundColor(color.Gray{Y: 230}).Style)
									} else {
										PaintStyledBorder(p, r, s)
									}
								case 2:
									PaintStyledBackground(p, r, s)
								}
							})
							if err != nil {
								t.Fatal(err)
							}
							return im
						}
						combined, border, background := render(0), render(1), render(2)
						checked := 0
						for y := range combined.Bounds().Dy() {
							for x := range combined.Bounds().Dx() {
								bg := color.RGBAModel.Convert(background.At(x, y)).(color.RGBA)
								want := color.RGBAModel.Convert(border.At(x, y)).(color.RGBA)
								if bg.A < 8 || bg.A > alpha-8 || want.A < 8 || want.A > 247 {
									continue
								}
								got := color.RGBAModel.Convert(combined.At(x, y)).(color.RGBA)
								for i, v := range []uint8{got.R, got.G, got.B, got.A} {
									w := []uint8{want.R, want.G, want.B, want.A}[i]
									if delta := int(v) - int(w); delta < -2 || delta > 2 {
										t.Fatalf("外缘(%d,%d)混入背景：%v，边框参考%v", x, y, got, want)
									}
								}
								checked++
							}
						}
						if checked == 0 {
							t.Fatal("没有验收到外缘部分覆盖像素")
						}
					})
				}
			}
		}
	}
}

func TestStyledBoxAlphaComposition(t *testing.T) {
	backend, err := software.NewPainter(&widgetRenderSurface{})
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	for _, tc := range []struct {
		name               string
		background, border color.Color
	}{
		{"半透明背景不透明边框", color.NRGBA{R: 60, G: 110, B: 160, A: 128}, color.Gray{Y: 230}},
		{"双方半透明", color.NRGBA{R: 60, G: 110, B: 160, A: 128}, color.NRGBA{R: 230, G: 230, B: 230, A: 128}},
		{"不透明背景半透明边框", color.NRGBA{R: 60, G: 110, B: 160, A: 255}, color.NRGBA{R: 230, G: 230, B: 230, A: 128}},
		{"透明背景", color.Transparent, color.Gray{Y: 230}},
		{"透明边框", color.NRGBA{R: 60, G: 110, B: 160, A: 128}, color.Transparent},
		{"无边框颜色", color.NRGBA{R: 60, G: 110, B: 160, A: 128}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := style.Default().Radius(12).BorderWidth(4).BackgroundColor(tc.background).BorderColor(tc.border).Style
			im, err := backend.RenderImage(96, 96, 1, func() {
				p := newPainter(backend, geometry.Rect(0, 0, 96, 96), 1)
				PaintStyledBox(p, geometry.Rect(8, 8, 64, 48), s)
			})
			if err != nil {
				t.Fatal(err)
			}
			// 使用标准库 Over 作为预乘参考；采样直边满覆盖处和中心，
			// 不把两个半透明层错误地当成互斥色块或预先混成不透明色。
			ref := image.NewRGBA(image.Rect(0, 0, 1, 1))
			draw.Draw(ref, ref.Bounds(), image.NewUniform(tc.background), image.Point{}, draw.Src)
			for i, point := range []image.Point{{X: 40, Y: 32}, {X: 40, Y: 9}} {
				if i == 1 && tc.border != nil {
					draw.Draw(ref, ref.Bounds(), image.NewUniform(tc.border), image.Point{}, draw.Over)
				}
				want := ref.RGBAAt(0, 0)
				got := color.RGBAModel.Convert(im.At(point.X, point.Y)).(color.RGBA)
				for j, v := range []uint8{got.R, got.G, got.B, got.A} {
					w := []uint8{want.R, want.G, want.B, want.A}[j]
					if delta := int(v) - int(w); delta < -2 || delta > 2 {
						t.Fatalf("预乘Over在%v错误：%v，标准库参考%v", point, got, want)
					}
				}
			}
		})
	}
}

// useTestApplication installs app as the global application for the duration of
// the test, restoring the previous one on cleanup. Unlike setTestApplication it
// lets a test provide a style sheet (and typography) up front.
func useTestApplication(t *testing.T, app *application) {
	t.Helper()
	old := App
	App = app
	t.Cleanup(func() {
		App = old
	})
}

// TestApplicationStyleSheetChangesButtonPaint proves a custom application style
// sheet flows all the way into what a widget actually paints, not just what the
// resolver returns.
func TestApplicationStyleSheetChangesButtonPaint(t *testing.T) {
	background := color.RGBA{R: 12, G: 24, B: 36, A: 255}
	border := color.RGBA{R: 40, G: 50, B: 60, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(
			style.Name(styleNameButton).
				BackgroundColor(background).
				BorderColor(border).
				BorderWidth(2).
				Radius(6),
		),
	})

	button := NewButton()
	button.Arrange(geometry.Rect(0, 0, 80, 30))

	painter := new(testButtonBackgroundPainter)
	button.Paint(painter)

	// 两者不透明时先绘完整边框底色，再绘真实内轮廓；仍核对几何和两份颜色。
	if painter.rect != geometry.Rect(0, 0, 80, 30) || painter.radius != 6 {
		t.Fatalf("unexpected fill geometry: rect=%+v radius=%v", painter.rect, painter.radius)
	}
	if painter.brush != graphics.ColorOf(background) {
		t.Fatalf("unexpected fill brush: %+v", painter.brush)
	}
	if painter.boxBrush != graphics.ColorOf(border) {
		t.Fatalf("unexpected border underlay: %+v", painter.boxBrush)
	}
	assertInnerBoxContour(t, painter.fillPath, geometry.Rect(2, 2, 76, 26), 4)
}

func TestStyledBoxClampsBorderRadiusAfterInset(t *testing.T) {
	border := color.RGBA{R: 40, G: 50, B: 60, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(
			style.Name(styleNameButton).
				BorderColor(border).
				BorderWidth(4).
				Radius(1),
		),
	})

	button := NewButton()
	button.Arrange(geometry.Rect(0, 0, 80, 30))
	painter := new(testButtonBackgroundPainter)
	button.Paint(painter)

	// 小圆角不能因中心线退化被绘成直角；环带外轮廓仍为半径1，
	// 内部矩形从(4,4)开始，保留已有内缩宽度并使用同一边框颜色。
	var starts []geometry.Point
	curves := 0
	painter.fillPath.Range(func(op graphics.PathOperation, args []float32) bool {
		if op == graphics.PathMoveTo {
			starts = append(starts, geometry.Point{X: args[0], Y: args[1]})
		}
		if op == graphics.PathBezierTo {
			curves++
		}
		return false
	})
	if len(starts) != 2 || starts[0] != (geometry.Point{X: 1}) || starts[1] != (geometry.Point{X: 4, Y: 4}) || curves != 4 || painter.brush != graphics.ColorOf(border) {
		t.Fatalf("小圆角环带几何错误：起点%v 圆弧%d 画刷%v", starts, curves, painter.brush)
	}
}

// TestWindowApplicationStyleSheetOverridesGlobal covers the two-tier lookup in
// widgetStyleSheet: a widget mounted in a window whose application has its own
// sheet uses that sheet, not the global one.
// TestButtonHoverBackgroundFromApplicationStyleSheet checks that the widget's
// current state selects the matching rule from the application sheet, and falls
// back to normal when the state clears.
func TestButtonHoverBackgroundFromApplicationStyleSheet(t *testing.T) {
	normalBackground := color.RGBA{R: 20, G: 20, B: 20, A: 255}
	hoverBackground := color.RGBA{R: 60, G: 60, B: 60, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(
			style.Name(styleNameButton).BackgroundColor(normalBackground).Radius(3),
			style.Name(styleNameButton).State(style.Hovered).BackgroundColor(hoverBackground),
		),
	})

	button := NewButton()
	button.Arrange(geometry.Rect(0, 0, 80, 30))

	painter := new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.ColorOf(normalBackground) {
		t.Fatalf("unexpected normal background: %+v", painter.brush)
	}

	button.updateState(style.Hovered, true)
	painter = new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.ColorOf(hoverBackground) {
		t.Fatalf("unexpected hover background: %+v", painter.brush)
	}

	button.updateState(style.Hovered, false)
	painter = new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.ColorOf(normalBackground) {
		t.Fatalf("hover leave should fall back to normal background: %+v", painter.brush)
	}
}

// TestTextInputFocusedBorderFromApplicationStyleSheet drives focus through the
// real window focus path and checks the focused-state border resolves from the
// application sheet.
func TestTextInputFocusedBorderFromApplicationStyleSheet(t *testing.T) {
	normalBorder := color.RGBA{R: 100, G: 100, B: 100, A: 255}
	focusBorder := color.RGBA{R: 10, G: 200, B: 90, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(
			style.Name(styleNameTextInput).
				BackgroundColor(color.White).
				BorderColor(normalBorder).
				BorderWidth(1),
			style.Name(styleNameTextInput).
				State(style.Focused).
				BorderColor(focusBorder),
		),
	})

	win := testWindowWithSurface(&window{})
	input := NewTextInput()
	win.SetWidget(input)
	input.Arrange(geometry.Rect(0, 0, 100, 24))

	painter := new(testTextInputPainter)
	input.Paint(painter)
	if painter.fillBrush != graphics.ColorOf(normalBorder) || painter.fillPathBrush != graphics.ColorOf(color.White) {
		t.Fatalf("unfocused border/background should use normal colors: %+v / %+v", painter.fillBrush, painter.fillPathBrush)
	}

	if !win.SetFocusedWidget(input) {
		t.Fatal("text input should accept focus")
	}
	painter = new(testTextInputPainter)
	input.Paint(painter)
	if painter.fillBrush != graphics.ColorOf(focusBorder) || painter.fillPathBrush != graphics.ColorOf(color.White) {
		t.Fatalf("focused border/background should use focused colors: %+v / %+v", painter.fillBrush, painter.fillPathBrush)
	}
}

// TestStyleNameSelectsRuleSet exercises StyleName routing: the widget's style
// name is what selects which rules apply, and changing it changes resolution.
// TestResolveFallsBackToDefaultStyleName proves StyleName() is explicit (empty
// when unset) while resolveStyle falls back to the widget's type default name.
func TestResolveFallsBackToDefaultStyleName(t *testing.T) {
	buttonBg := color.RGBA{R: 7, G: 8, B: 9, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(style.Name(styleNameButton).BackgroundColor(buttonBg)),
	})

	button := NewButton()
	if button.StyleName() != "" {
		t.Fatalf("unset style override should be empty (explicit getter), got %q", button.StyleName())
	}

	bg, ok := ResolveStyle(styleNameButton, style.PartDefault, style.Normal).BackgroundColor()
	if !ok || !colors.Equal(bg, buttonBg) {
		t.Fatalf("button default name should resolve the button style: %v ok=%v", bg, ok)
	}
}

func TestStyleNameSelectsRuleSet(t *testing.T) {
	setTestApplication(t, nil)

	// ResolveStyle routes purely by name: the generic widget name resolves the
	// base (transparent) style, a different name selects a different ruleset.
	background, ok := ResolveStyle(styleNameWidget, style.PartDefault, style.Normal).BackgroundColor()
	if !ok || !colors.Equal(background, color.Transparent) {
		t.Fatalf("widget style name should resolve transparent background: %v ok=%v", background, ok)
	}

	resolved := ResolveStyle(styleNameButton, style.PartDefault, style.Normal)
	background, ok = resolved.BackgroundColor()
	if !ok || !colors.Equal(background, color.RGBA{R: 210, G: 210, B: 210, A: 255}) {
		t.Fatalf("button style name should resolve button background: %v ok=%v", background, ok)
	}
	if radius, ok := resolved.CornerRadius(); !ok || radius != geometry.UniformCornerRadius(4) {
		t.Fatalf("button style name should resolve button radius: %v ok=%v", radius, ok)
	}
}

// 独立检查内轮廓的边界、起点和圆角段，避免记录器把填充伪装成旧描边指令。
func assertInnerBoxContour(t *testing.T, path graphics.Path, rect geometry.Rectangle, radius float32) {
	t.Helper()
	left, top, right, bottom := float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Inf(-1))
	var start geometry.Point
	moves, curves := 0, 0
	path.Range(func(op graphics.PathOperation, args []float32) bool {
		if op == graphics.PathClose || op == graphics.PathHole {
			return false // 轮廓标记的占位参数不是几何点。
		}
		if op == graphics.PathMoveTo {
			moves++
			start = geometry.Point{X: args[0], Y: args[1]}
		}
		if op == graphics.PathBezierTo {
			curves++
		}
		for i := 0; i+1 < len(args); i += 2 {
			left, top = min(left, args[i]), min(top, args[i+1])
			right, bottom = max(right, args[i]), max(bottom, args[i+1])
		}
		return false
	})
	// 四个内角各有一个曲线段；零半径内角允许退化控制点。
	if moves != 1 || curves != 4 || start != (geometry.Point{X: rect.X + radius, Y: rect.Y}) || geometry.Rect(left, top, right-left, bottom-top) != rect {
		t.Fatalf("内轮廓错误：边界%v 起点%v moves=%d curves=%d，预期%v radius=%g", geometry.Rect(left, top, right-left, bottom-top), start, moves, curves, rect, radius)
	}
}

func TestStyledOpaqueDifferentColorsStayOpaque(t *testing.T) {
	backend, err := software.NewPainter(&widgetRenderSurface{})
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	rect := geometry.Rect(6, 6, 36, 20)
	background, border := color.RGBA{199, 199, 201, 255}, color.RGBA{186, 186, 188, 255}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, offset := range []float32{0, .25, .5, .75} {
			for _, radius := range []geometry.CornerRadius{geometry.UniformCornerRadius(10), {TopLeft: 9, TopRight: 6, BottomLeft: 2, BottomRight: 3}} {
				for _, transform := range []geometry.Transform{
					geometry.Translate(offset, offset),
					geometry.Translate(12+offset, 2+offset).Multiply(geometry.Rotate(14)),
					geometry.Translate(offset, offset).Multiply(geometry.Scale(1.1, .8)),
				} {
					for _, width := range []float32{.5, 1, 2, 10} {
						t.Run(fmt.Sprintf("scale=%g/offset=%g/radius=%v/transform=%v/width=%g", scale, offset, radius, transform, width), func(t *testing.T) {
							s := style.Default().BackgroundColor(background).BorderColor(border).BorderWidth(width).CornerRadius(radius).Style
							img, err := backend.RenderImage(int(64*scale), int(48*scale), scale, func() {
								p := newPainter(backend, geometry.Rect(0, 0, 64, 48), scale)
								p.SetTransform(transform)
								p.SetClipRect(geometry.Rect(2, 2, 58, 42))
								PaintStyledBox(p, rect, s)
							})
							if err != nil {
								t.Fatal(err)
							}
							ref := rect
							if transform.A11 == 1 && transform.A22 == 1 && transform.A12 == 0 && transform.A21 == 0 {
								// 独立物理边界公式；不调用生产像素对齐/路径 helper。
								edge := func(x, shift float32) float32 { return float32(math.Floor(float64((x+shift)*scale)+.5))/scale - shift }
								l, top := edge(rect.X, transform.TX), edge(rect.Y, transform.TY)
								ref = geometry.Rect(l, top, edge(rect.X+rect.Width, transform.TX)-l, edge(rect.Y+rect.Height, transform.TY)-top)
							}
							inverse := transform.Inverse()
							inside, aa := 0, 0
							for y := 0; y < img.Bounds().Dy(); y++ {
								for x := 0; x < img.Bounds().Dx(); x++ {
									c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
									if c.A > 0 && c.A < 255 {
										aa++
									}
									covered := true
									// 整颗像素及 0.05 px 余量位于凸外形内，alpha 必须为255。
									// 色彩为预乘 sRGB RGBA8；不以 AA 容差放宽形内 alpha。
									for _, delta := range []geometry.Point{{X: -.05, Y: -.05}, {X: 1.05, Y: -.05}, {X: -.05, Y: 1.05}, {X: 1.05, Y: 1.05}} {
										windowPoint := geometry.Point{X: (float32(x) + delta.X) / scale, Y: (float32(y) + delta.Y) / scale}
										if windowPoint.X < 2 || windowPoint.X > 60 || windowPoint.Y < 2 || windowPoint.Y > 44 {
											covered = false
											break
										}
										point := inverse.TransformPoint(windowPoint)
										if !borderSidesContains(ref, radius, 0, 0, point) {
											covered = false
											break
										}
									}
									if covered {
										inside++
										if c.A != 255 {
											t.Fatalf("轨道形内漏底：(%d,%d)=%v，alpha须为255", x, y, c)
										}
										for i, got := range []uint8{c.R, c.G, c.B} {
											a := []uint8{background.R, background.G, background.B}[i]
											b := []uint8{border.R, border.G, border.B}[i]
											if int(got) < int(min(a, b))-1 || int(got) > int(max(a, b))+1 {
												t.Fatalf("不透明区域颜色越界：%v", c)
											}
										}
									}
								}
							}
							if inside < 100 || aa == 0 {
								t.Fatal("缺少形内或抗锯齿像素", inside, aa)
							}
						})
					}
				}
			}
		}
	}
}
