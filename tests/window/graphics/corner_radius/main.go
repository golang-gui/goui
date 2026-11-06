// 独立四角、透明环带和盒形阴影的三后端验收。
//
// 环境：Linux X11+合成器（Software/OpenGL）、Windows（Direct2D/OpenGL）、macOS OpenGL。
// 启动：GOUI_PLAT_PAINTER=opengl go run ./tests/window/graphics/corner_radius [-auto]
// 初始：固定顺序显示四角不同、直角与圆角混合、对角大圆角、小圆角粗边框；
// 背景为深灰色，环带中央露出背景，没有填充覆盖或重复合成的深色连接线。
// 操作：点击重绘；Space 切换样例排列。检查每块四角与相邻边相切，边框内部透明；
// 阴影跟随各自轮廓，不出现统一最大圆角、四块拼接或左右偏移。
// 程序在首次真实 Paint 前，对 1/1.25/1.5/2x 离屏目标做像素断言，包含连接处和退化；
// 输出 PIXELS PASS、后端、系统与缩放。-auto 真实提交一帧后退出，不替代外观检查。
// 平台差异：AA 曲线可略有差异，几何内外侧及预乘颜色必须一致；记录实际后端，
// 不将后端回退、构建或仅窗口启动算作指定后端验收。
// 选边圆角的收尖区域另按独立椭圆方程积分验证，覆盖1/1.25/1.5/2x、旋转、
// clip、透明／不透明／渐变及ClearPath；尖端不能出现近乎不透明的孤立亮点。
// 副作用：仅一个窗口与终端输出，不写文件／系统设置，不用 DevServer。
// 复位：重启。退出：Escape／关闭；释放 Painter 后销毁窗口，提前失败返回非零。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"runtime"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/utils"
)

var cases = []geometry.CornerRadius{{TopLeft: 1, TopRight: 18, BottomRight: 5, BottomLeft: 0}, {TopLeft: 36, TopRight: 0, BottomRight: 36, BottomLeft: 0}, geometry.UniformCornerRadius(1), {TopLeft: 0, TopRight: 0, BottomRight: 18, BottomLeft: 18}}

func main() {
	if err := run(); err != nil {
		log.Fatal("FAIL: ", err)
	}
}
func run() error {
	auto := flag.Bool("auto", false, "断言并提交一帧后退出")
	flag.Parse()
	plat, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.CornerRadius")
	if err != nil {
		return err
	}
	defer plat.Destroy()
	loop, err := plat.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	var painter graphics.Painter
	var size events.SizeEvent
	var failure error
	checked := false
	order := 0
	paint := func() {
		if painter == nil || size.Width <= 0 {
			return
		}
		if !checked {
			checked = true
			failure = checkPixels(painter)
			if failure != nil {
				loop.Quit()
				return
			}
			fmt.Printf("PIXELS PASS OS=%s painter=%s scale=%g\n", runtime.GOOS, painter.Name(), size.PixelWidth/size.Width)
		}
		painter.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
		painter.Clear(graphics.RGB(45, 48, 54))
		for i := range 4 {
			r := cases[(i+order)%len(cases)]
			body := geometry.Rect(30+float32(i)*160, 45, 120, 100)
			painter.FillPath(utils.RoundRectBorderPath(body, r, 8), graphics.RGBA(80, 180, 230, 190))
			body.Y = 210
			painter.DrawBoxShadow(body, r, graphics.BoxShadow{Color: graphics.Color{A: .5}, Offset: geometry.Point{Y: 5}, BlurRadius: 12})
			painter.FillPath(utils.RoundRectPath(body, r), graphics.RGB(210, 215, 220))
			painter.FillPath(utils.RoundRectBorderPath(body, r, 2), graphics.RGB(100, 140, 200))
		}
		painter.End()
		if *auto {
			loop.Post(loop.Quit)
		}
	}
	win, err := plat.NewWindow(geometry.Size{Width: 700, Height: 360}, func(event platform.Event) {
		switch e := event.(type) {
		case events.SizeEvent:
			size = e
		case events.PaintEvent:
			paint()
		case events.CloseEvent:
			loop.Quit()
		case events.PointerEvent:
			if e.EventType == events.PointerDown {
				order = (order + 1) % len(cases)
				paint()
			}
		case events.KeyEvent:
			if e.EventType != events.KeyDown {
				return
			}
			if e.Key == events.KeyEscape {
				loop.Quit()
			}
			if e.Key == events.KeySpace {
				order = (order + 1) % len(cases)
				paint()
			}
		}
	}, platform.WindowOptions{})
	if err != nil {
		return err
	}
	defer win.Destroy()
	_ = win.SetTitle("GOUI CornerRadius / BoxShadow")
	painter, err = plat.NewPainter(win)
	if err != nil {
		return err
	}
	defer painter.Destroy()
	if requested := os.Getenv("GOUI_PLAT_PAINTER"); requested != "" && !strings.EqualFold(requested, painter.Name()) {
		return fmt.Errorf("请求后端%s，实际%s", requested, painter.Name())
	}
	if err = win.Show(); err != nil {
		return err
	}
	loop.Run()
	if failure != nil {
		return failure
	}
	if !checked {
		return fmt.Errorf("没有完成原生绘制")
	}
	fmt.Println("DESTROY PASS")
	return nil
}

func checkPixels(p graphics.Painter) error {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, r := range cases {
			for _, width := range []float32{1, 6, 24} {
				rect := geometry.Rect(4, 4, 48, 40)
				r = r.Normalize(rect.Size)
				img, err := p.RenderImage(int(64*scale), int(54*scale), scale, func() {
					p.Clear(graphics.Color{})
					p.FillPath(utils.RoundRectBorderPath(rect, r, width), graphics.RGBA(200, 80, 40, 128))
				})
				if err != nil {
					return err
				}
				if err = checkRing(img, rect, r, width, scale); err != nil {
					return fmt.Errorf("%s scale=%g radius=%+v width=%g: %w", p.Name(), scale, r, width, err)
				}
			}
		}
	}
	if err := checkTaperedBorders(p); err != nil {
		return err
	}
	return checkShadows(p)
}

func checkTaperedBorders(p graphics.Painter) error {
	rect := geometry.Rect(16, 16, 80, 56)
	radius := geometry.UniformCornerRadius(16)
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, angle := range []float32{0, 90} {
			transform := geometry.Translate(64, 64).Multiply(geometry.Rotate(angle)).Multiply(geometry.Translate(-56, -44))
			inverse := transform.Inverse()
			for _, insets := range []geometry.Insets{{Top: 4}, {Top: 4, Bottom: 4}, {Top: 4, Bottom: 4, Left: 4}} {
				for _, mode := range []string{"opaque", "translucent", "gradient", "erase"} {
					base, foreground := graphics.Color{}, graphics.RGBA(200, 80, 40, 255)
					if mode != "opaque" {
						base, foreground = graphics.RGBA(32, 64, 96, 192), graphics.RGBA(200, 80, 40, 128)
					}
					clip := geometry.Rect(0, 0, 128, 64)
					img, err := p.RenderImage(int(128*scale), int(128*scale), scale, func() {
						p.Clear(base)
						p.SetTransform(transform)
						p.SetClipRect(clip)
						path := utils.RoundRectBorderInsetsPath(rect, radius, insets)
						if mode == "erase" {
							p.ClearPath(path)
						} else if mode == "gradient" {
							p.FillPath(path, graphics.LinearGradient{
								Start: geometry.Point{X: rect.X, Y: rect.Y}, End: geometry.Point{X: rect.X + rect.Width, Y: rect.Y},
								StartColor: foreground, EndColor: graphics.RGBA(40, 180, 200, 128),
							})
						} else {
							p.FillPath(path, foreground)
						}
						p.SetClipRect(geometry.Rectangle{})
						p.SetTransform(geometry.Identity())
						// 下一个普通调用不能继承覆盖边表或擦除合成模式。
						p.FillRect(geometry.Rect(4, 4, 8, 8), graphics.RGBA(30, 180, 60, 128))
					})
					if err != nil {
						return err
					}
					// 只测尖端前两行及紧邻的外部像素。独立面积积分结果约为0~十余级
					// alpha；8级容差包含此处末段弦近似与64×64积分、8位量化误差，
					// 旧实现会给出约240级，不能把它视作允许的AA曲线差异。
					for _, point := range []geometry.Point{
						{X: rect.X + rect.Width - .5/scale, Y: rect.Y + 16 - .5/scale},
						{X: rect.X + rect.Width + .5/scale, Y: rect.Y + 16 - .5/scale},
						{X: rect.X + rect.Width - .5/scale, Y: rect.Y + 16 - 1.5/scale},
						{X: rect.X + rect.Width - .5/scale, Y: rect.Y + rect.Height - 16 + .5/scale},
					} {
						windowPoint := transform.TransformPoint(point)
						x, y := int(math.Floor(float64(windowPoint.X*scale))), int(math.Floor(float64(windowPoint.Y*scale)))
						coverage := taperedCoverage(rect, insets, inverse, clip, x, y, scale)
						fg := foreground
						if mode == "gradient" {
							center := inverse.TransformPoint(geometry.Point{X: (float32(x) + .5) / scale, Y: (float32(y) + .5) / scale})
							t := min(max((center.X-rect.X)/rect.Width, 0), 1)
							end := graphics.RGBA(40, 180, 200, 128)
							fg.R += (end.R - fg.R) * t
							fg.G += (end.G - fg.G) * t
							fg.B += (end.B - fg.B) * t
						}
						want := overCoverage(base, fg, coverage, mode == "erase")
						if err := checkCoveragePixel(img, x, y, want, 8); err != nil {
							return fmt.Errorf("尖端 %s/%gx/%g°/%v/%s coverage=%g: %w", p.Name(), scale, angle, insets, mode, coverage, err)
						}
					}
					want := overCoverage(base, graphics.RGBA(30, 180, 60, 128), 1, false)
					if err := checkCoveragePixel(img, int(6*scale), int(6*scale), want, 2); err != nil {
						return fmt.Errorf("尖端绘制后状态泄漏: %w", err)
					}
				}
			}
		}
	}
	return nil
}

func taperedCoverage(rect geometry.Rectangle, insets geometry.Insets, inverse geometry.Transform, clip geometry.Rectangle, x, y int, scale float32) float32 {
	inside := func(pt geometry.Point, i geometry.Insets) bool {
		left, top, right, bottom := rect.X+i.Left, rect.Y+i.Top, rect.X+rect.Width-i.Right, rect.Y+rect.Height-i.Bottom
		if pt.X < left || pt.Y < top || pt.X > right || pt.Y > bottom {
			return false
		}
		for _, c := range [][4]float32{
			{pt.X - left, pt.Y - top, 16 - i.Left, 16 - i.Top},
			{right - pt.X, pt.Y - top, 16 - i.Right, 16 - i.Top},
			{right - pt.X, bottom - pt.Y, 16 - i.Right, 16 - i.Bottom},
			{pt.X - left, bottom - pt.Y, 16 - i.Left, 16 - i.Bottom},
		} {
			if c[0] < c[2] && c[1] < c[3] {
				u, v := (c[0]-c[2])/c[2], (c[1]-c[3])/c[3]
				if u*u+v*v > 1 {
					return false
				}
			}
		}
		return true
	}
	count := 0
	const samples = 64
	for sy := 0; sy < samples; sy++ {
		for sx := 0; sx < samples; sx++ {
			point := geometry.Point{X: (float32(x) + (float32(sx)+.5)/samples) / scale, Y: (float32(y) + (float32(sy)+.5)/samples) / scale}
			if point.X < clip.X || point.Y < clip.Y || point.X >= clip.X+clip.Width || point.Y >= clip.Y+clip.Height {
				continue
			}
			point = inverse.TransformPoint(point)
			if inside(point, geometry.Insets{}) && !inside(point, insets) {
				count++
			}
		}
	}
	return float32(count) / (samples * samples)
}

func overCoverage(base, fg graphics.Color, coverage float32, erase bool) graphics.Color {
	if erase {
		return graphics.Color{R: base.R * (1 - coverage), G: base.G * (1 - coverage), B: base.B * (1 - coverage), A: base.A * (1 - coverage)}
	}
	a := fg.A * coverage
	return graphics.Color{R: fg.R*coverage + base.R*(1-a), G: fg.G*coverage + base.G*(1-a), B: fg.B*coverage + base.B*(1-a), A: a + base.A*(1-a)}
}

func checkCoveragePixel(img image.Image, x, y int, want graphics.Color, tolerance int) error {
	got := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
	for i, c := range []byte{got.R, got.G, got.B, got.A} {
		w := float64([]float32{want.R, want.G, want.B, want.A}[i] * 255)
		if math.Abs(float64(c)-w) > float64(tolerance) {
			return fmt.Errorf("像素(%d,%d)=%v，预期预乘RGBA%v", x, y, got, want)
		}
	}
	return nil
}

// 独立的圆方程／矩形不等式参考，不调用 CornerRadius.Contains 或路径构造。
func contains(rect geometry.Rectangle, r geometry.CornerRadius, x, y float32) bool {
	x -= rect.X
	y -= rect.Y
	if x < 0 || y < 0 || x >= rect.Width || y >= rect.Height {
		return false
	}
	for i, c := range [][3]float32{{r.TopLeft, r.TopLeft, r.TopLeft}, {rect.Width - r.TopRight, r.TopRight, r.TopRight}, {rect.Width - r.BottomRight, rect.Height - r.BottomRight, r.BottomRight}, {r.BottomLeft, rect.Height - r.BottomLeft, r.BottomLeft}} {
		// 仅检测对应角包围方块，允许对角大圆角包围区域重叠。
		dx, dy := x-c[0], y-c[1]
		left, top := i == 0 || i == 3, i < 2
		inX := (left && x < c[2]) || (!left && x > rect.Width-c[2])
		inY := (top && y < c[2]) || (!top && y > rect.Height-c[2])
		if inX && inY && dx*dx+dy*dy > c[2]*c[2] {
			return false
		}
	}
	return true
}
func checkRing(img image.Image, rect geometry.Rectangle, r geometry.CornerRadius, width, scale float32) error {
	inner := rect.Inset(width)
	ir := r.Inset(width).Normalize(inner.Size)
	mask := func(x, y float32) bool { return contains(rect, r, x, y) && !contains(inner, ir, x, y) }
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			px, py := (float32(x)+.5)/scale, (float32(y)+.5)/scale
			value, stable := mask(px, py), true
			for _, dx := range []float32{-.75 / scale, 0, .75 / scale} {
				for _, dy := range []float32{-.75 / scale, 0, .75 / scale} {
					stable = stable && mask(px+dx, py+dy) == value
				}
			}
			pixel := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
			if pixel.A > 130 || pixel.R > pixel.A || pixel.G > pixel.A || pixel.B > pixel.A {
				return fmt.Errorf("像素%d,%d 重复合成或非预乘：%v", x, y, pixel)
			}
			if stable && ((value && pixel.A < 125) || (!value && pixel.A > 3)) {
				return fmt.Errorf("像素%d,%d 环带覆盖%v，预期%v", x, y, pixel, value)
			}
		}
	}
	return nil
}

func checkShadows(p graphics.Painter) error {
	rect := geometry.Rect(24, 24, 60, 48)
	colors := graphics.Color{R: .2, G: .1, B: .3, A: .5}
	maxNativeDelta := 0.0
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, corners := range []geometry.CornerRadius{{TopLeft: 18, TopRight: 0, BottomRight: 8, BottomLeft: 26}, {TopLeft: 0, TopRight: 0, BottomRight: 24, BottomLeft: 24}, geometry.UniformCornerRadius(8)} {
			for _, spread := range []float32{-2, 0, 4} {
				for _, transform := range []geometry.Transform{geometry.Identity(), geometry.Transform{A12: -1, A21: 1, TX: 100, TY: 8}, geometry.Translate(8, 0).Scale(.75, 1.2)} {
					shadow := graphics.BoxShadow{Color: colors, Offset: geometry.Point{X: 3, Y: 4}, BlurRadius: 6, SpreadRadius: spread}
					img, err := p.RenderImage(int(132*scale), int(112*scale), scale, func() {
						p.Clear(graphics.Color{})
						p.SetTransform(transform)
						p.SetClipRect(geometry.Rect(0, 0, 100, 110))
						p.DrawBoxShadow(rect, corners, shadow)
					})
					if err != nil {
						return err
					}
					shape := rect.Inset(-spread).Translate(shadow.Offset)
					radii := corners.Normalize(rect.Size).Inset(-spread).Normalize(shape.Size)
					for _, point := range []geometry.Point{{X: 28, Y: 28}, {X: 80, Y: 28}, {X: 80, Y: 65}, {X: 28, Y: 64}, {X: 23, Y: 40}, {X: 51, Y: 23}, {X: 50, Y: 45}, {X: 82, Y: 74}} {
						onScreen := transform.TransformPoint(point)
						x, y := int(onScreen.X*scale), int(onScreen.Y*scale)
						sample := transform.Inverse().TransformPoint(geometry.Point{X: (float32(x) + .5) / scale, Y: (float32(y) + .5) / scale})
						coverage := shadowReference(shape, radii, sample, 3)
						low, high := coverage, coverage
						if p.Name() == "Direct2D" {
							// 原生Shadow核及DrawImage重采样允许既有契约的一物理像素
							// 包络；仍用独立积分界定范围，不放大固定色值容差。
							for _, dx := range []float32{-.5, 0, .5} {
								for _, dy := range []float32{-.5, 0, .5} {
									q := transform.Inverse().TransformPoint(geometry.Point{X: (float32(x) + .5 + dx) / scale, Y: (float32(y) + .5 + dy) / scale})
									v := shadowReference(shape, radii, q, 3)
									low = math.Min(low, v)
									high = math.Max(high, v)
								}
							}
						}
						pixel := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
						want := [4]float64{.2 * 255 * coverage, .1 * 255 * coverage, .3 * 255 * coverage, .5 * 255 * coverage}
						for i, got := range []uint8{pixel.R, pixel.G, pixel.B, pixel.A} {
							if p.Name() == "Direct2D" {
								maxNativeDelta = math.Max(maxNativeDelta, math.Abs(float64(got)-want[i]))
							}
							factor := []float64{.2, .1, .3, .5}[i] * 255
							if float64(got) < factor*low-3 || float64(got) > factor*high+3 {
								return fmt.Errorf("%s 阴影scale=%g corners=%+v spread=%g transform=%v 像素%d,%d=%v，参考%v", p.Name(), scale, corners, spread, transform, x, y, pixel, want)
							}
						}
						if pixel.R > pixel.A || pixel.G > pixel.A || pixel.B > pixel.A {
							return fmt.Errorf("阴影非预乘像素%v", pixel)
						}
					}
					if pixel := color.RGBAModel.Convert(img.At(int(115*scale), int(50*scale))).(color.RGBA); pixel != (color.RGBA{}) {
						return fmt.Errorf("阴影越过clip：%v", pixel)
					}
				}
			}
		}
	}
	if p.Name() == "Direct2D" {
		fmt.Printf("NATIVE SHADOW center delta max=%.3f/255; one-physical-pixel envelope checked\n", maxNativeDelta)
	}
	return checkNarrowShadow(p)
}

// 半径150、sigma1时，全角固定采样会漏掉局部高斯峰；专门在圆弧附近检查。
func checkNarrowShadow(p graphics.Painter) error {
	rect := geometry.Rect(24, 24, 180, 170)
	r := geometry.CornerRadius{TopLeft: 150}
	for _, scale := range []float32{1, 2} {
		img, err := p.RenderImage(int(240*scale), int(230*scale), scale, func() {
			p.Clear(graphics.Color{})
			p.DrawBoxShadow(rect, r, graphics.BoxShadow{Color: graphics.Color{R: .25, B: .5, A: .5}, BlurRadius: 2})
		})
		if err != nil {
			return err
		}
		for _, pt := range []geometry.Point{{X: 67, Y: 68}, {X: 68, Y: 68}, {X: 69, Y: 68}, {X: 90, Y: 45}, {X: 45, Y: 90}, {X: 180, Y: 60}} {
			x, y := int(pt.X*scale), int(pt.Y*scale)
			cx, cy := (float32(x)+.5)/scale, (float32(y)+.5)/scale
			coverage := shadowReference(rect, r, geometry.Point{X: cx, Y: cy}, 1)
			low, high := coverage, coverage
			if p.Name() == "Direct2D" {
				for _, dx := range []float32{-.5 / scale, 0, .5 / scale} {
					for _, dy := range []float32{-.5 / scale, 0, .5 / scale} {
						v := shadowReference(rect, r, geometry.Point{X: cx + dx, Y: cy + dy}, 1)
						low = math.Min(low, v)
						high = math.Max(high, v)
					}
				}
			}
			got := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
			if float64(got.A) < 127.5*low-3 || float64(got.A) > 127.5*high+3 || got.B > got.A || got.R > got.A {
				return fmt.Errorf("%s 大圆角窄模糊scale%g 像素%d,%d=%v，参考%g", p.Name(), scale, x, y, got, coverage)
			}
		}
	}
	return nil
}

// 从圆弧定义求每个x处填充的上下界，以标准库高斯CDF做竖向解析积分、
// 横向2048点积分。不调用生产角缺口近似，误差阈值3/255覆盖核近似及8位量化。
func shadowReference(rect geometry.Rectangle, r geometry.CornerRadius, p geometry.Point, sigma float64) float64 {
	width, height := float64(rect.Width), float64(rect.Height)
	px, py := float64(p.X-rect.X), float64(p.Y-rect.Y)
	lo, hi := math.Max(0, px-6*sigma), math.Min(width, px+6*sigma)
	if hi <= lo {
		return 0
	}
	dx := (hi - lo) / 2048
	total := 0.0
	for n := 0; n < 2048; n++ {
		x := lo + (float64(n)+.5)*dx
		top, bottom := 0.0, height
		for i, corner := range []float32{r.TopLeft, r.TopRight, r.BottomRight, r.BottomLeft} {
			rad := float64(corner)
			if rad == 0 {
				continue
			}
			left, upper := i == 0 || i == 3, i < 2
			center := rad
			if !left {
				center = width - rad
			}
			if (left && x >= rad) || (!left && x <= width-rad) {
				continue
			}
			arc := math.Sqrt(math.Max(0, rad*rad-(x-center)*(x-center)))
			if upper {
				top = math.Max(top, rad-arc)
			} else {
				bottom = math.Min(bottom, height-rad+arc)
			}
		}
		if top >= bottom {
			continue
		}
		z := (x - px) / sigma
		total += math.Exp(-.5*z*z) / (sigma * math.Sqrt(2*math.Pi)) * .5 * (math.Erf((bottom-py)/(sigma*math.Sqrt2)) - math.Erf((top-py)/(sigma*math.Sqrt2))) * dx
	}
	return total
}
