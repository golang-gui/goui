package gui

import (
	"fmt"
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

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
							if radius.Uniform() && transform.A11 == 1 && transform.A22 == 1 && transform.A12 == 0 && transform.A21 == 0 {
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
