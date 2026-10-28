// ClearPath 原生绘制与透明提交验收。
//
// 环境：Linux X11+合成器、Windows 或 macOS 桌面；记录实际后端和缩放。
// 启动：go run ./tests/window/graphics/clear_path [-auto]
// GOUI_PLAT_PAINTER=software/opengl，Windows 另可用 direct2d。
// 初始：透明无框窗口，四色底板及透明圆孔；自动先执行 1/1.25/1.5/2x 离屏像素断言。
// 操作与预期：
//  1. 四个外角及中央圆孔应显露桌面，弧线平滑，没有黑边或颜色残留；
//     圆弧与直边相切处连续，不能出现一像素缺口或突然缩进。
//  2. 点击任意非透明区域重绘；圆孔、角部和颜色保持一致，不能擦掉整个底板。
//  3. 调整 GOUI_PLAT_SCALE=1/2 重启，分别验证各后端；圆角不变为直角。
//  4. 检查终端 PIXELS PASS、实际 Painter 与 scale；失败打印 FAIL 并非零退出。
//     -auto 在像素断言和一次真实提交后自动退出；不代替步骤 1 的视觉验收。
//
// 平台差异：透明表面由各系统合成；透明像素不自动穿透输入，无自动拖动缩放。
// 副作用：只创建窗口，不写文件、剪贴板或系统设置。复位：重新启动。
// 退出：Escape、关闭请求或 -auto 正常退出，释放 Painter 后销毁窗口。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

func main() {
	if err := run(); err != nil {
		log.Fatal("FAIL: ", err)
	}
}

func run() error {
	auto := flag.Bool("auto", false, "像素检查及首次提交后退出")
	flag.Parse()
	plat, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.ClearPath")
	if err != nil {
		return err
	}
	defer plat.Destroy()
	loop, err := plat.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	var win platform.Window
	var painter graphics.Painter
	var size events.SizeEvent
	var failure error
	checked := false
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
		painter.Clear(graphics.Color{})
		for i, ink := range []graphics.Color{graphics.RGB(35, 105, 195), graphics.RGB(215, 100, 60), graphics.RGB(50, 160, 100), graphics.RGB(150, 80, 180)} {
			painter.FillRect(graphics.Rect(float32(i%2)*size.Width/2, float32(i/2)*size.Height/2, size.Width/2, size.Height/2), ink)
		}
		r := min(size.Width, size.Height) / 5
		cx, cy := size.Width/2, size.Height/2
		painter.ClearPath(graphics.MoveTo(cx-r, cy).ArcTo(r, r, 0, 0, 1, cx+r, cy).ArcTo(r, r, 0, 0, 1, cx-r, cy).Close())
		clearCorners(painter, size.Width, size.Height, 24, size.PixelWidth/size.Width)
		painter.End()
		if *auto {
			loop.Post(loop.Quit)
		}
	}
	win, err = plat.NewWindow(geometry.Size{Width: 480, Height: 320}, func(event platform.Event) {
		switch e := event.(type) {
		case events.SizeEvent:
			size = e
		case events.PaintEvent:
			paint()
		case events.CloseEvent:
			loop.Quit()
		case events.KeyEvent:
			if e.Key == events.KeyEscape {
				loop.Quit()
			}
		case events.PointerEvent:
			if e.EventType == events.PointerDown && win != nil {
				_ = win.RequestPaint()
			}
		}
	}, platform.WindowOptions{Chrome: platform.WindowChromeNone, Transparent: true})
	if err != nil {
		return err
	}
	defer win.Destroy()
	_ = win.SetTitle("GOUI ClearPath pixels")
	painter, err = plat.NewPainter(win)
	if err != nil {
		return err
	}
	defer painter.Destroy()
	if err = win.Show(); err != nil {
		return err
	}
	loop.Run()
	if failure != nil {
		return failure
	}
	if !checked {
		return fmt.Errorf("no native frame was drawn")
	}
	fmt.Println("DESTROY PASS")
	return nil
}

func rectangle(x, y, w, h float32) graphics.Path {
	return graphics.MoveTo(x, y).LineTo(x+w, y).LineTo(x+w, y+h).LineTo(x, y+h).Close()
}

func clearCorners(p graphics.Painter, width, height, radius, scale float32) {
	// 两物理像素的外延只移动闭合边，不移动圆弧及其相切点。
	padding := 2 / scale
	corner := graphics.MoveTo(-padding, -padding).LineTo(radius, -padding).
		LineTo(radius, 0).ArcTo(radius, radius, 0, 0, 0, 0, radius).
		LineTo(-padding, radius).Close()
	for _, c := range [][4]float32{{0, 0, 1, 1}, {width, 0, -1, 1}, {width, height, -1, -1}, {0, height, 1, -1}} {
		p.SetTransform(geometry.Translate(c[0], c[1]).Scale(c[2], c[3]))
		p.ClearPath(corner)
	}
	p.SetTransform(geometry.Identity())
}

func checkPixels(p graphics.Painter) error {
	pixels := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			pixels.SetRGBA(x, y, color.RGBA{G: 200, B: 80, A: 255})
		}
	}
	img, err := p.NewImage(pixels)
	if err != nil {
		return err
	}
	defer img.Destroy()
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		if err := checkCornerTangencies(p, scale); err != nil {
			return err
		}
		for frame := 0; frame < 2; frame++ {
			result, err := p.RenderImage(32, 24, scale, func() {
				p.Clear(graphics.Color{R: .5, G: .25, B: .125, A: .5})
				p.ClearPath(rectangle(4.5/scale, 4/scale, 12/scale, 12/scale))
				p.ClearPath(graphics.Path{})
				p.FillRect(graphics.Rect(20/scale, 4/scale, 4/scale, 4/scale), graphics.RGBA(0, 0, 255, 128))
				p.DrawImage(graphics.Rect(20/scale, 12/scale, 4/scale, 4/scale), img)
			})
			if err != nil {
				return err
			}
			// 直边穿过物理像素中心，解析覆盖率 1/2；允许两次 8 位量化累计 2。
			for _, s := range []struct {
				x, y      int
				want      color.RGBA
				tolerance int
			}{
				{4, 8, color.RGBA{R: 64, G: 32, B: 16, A: 64}, 2},
				{8, 8, color.RGBA{}, 0},
				{2, 8, color.RGBA{R: 128, G: 64, B: 32, A: 128}, 1},
				{21, 5, color.RGBA{R: 64, G: 32, B: 144, A: 192}, 2},
				{21, 13, color.RGBA{G: 200, B: 80, A: 255}, 1},
			} {
				if err := pixelNear(result, s.x, s.y, s.want, s.tolerance); err != nil {
					return fmt.Errorf("%s scale=%g frame=%d: %w", p.Name(), scale, frame, err)
				}
			}
		}
		// 圆形几何使用独立距离公式核对内外侧；只在原生 AA 边界一物理像素内
		// 允许栅格化差异，同时确认确实存在半透明边缘。
		extent := int(48 * scale)
		circle, err := p.RenderImage(extent, extent, scale, func() {
			p.Clear(graphics.Color{R: .5, G: .25, B: .125, A: .5})
			p.ClearPath(graphics.MoveTo(12, 24).ArcTo(12, 12, 0, 0, 1, 36, 24).ArcTo(12, 12, 0, 0, 1, 12, 24).Close())
		})
		if err != nil {
			return err
		}
		partial := false
		for y := 0; y < extent; y++ {
			for x := 0; x < extent; x++ {
				distance := math.Hypot(float64(x)+.5-float64(24*scale), float64(y)+.5-float64(24*scale))
				got := color.RGBAModel.Convert(circle.At(x, y)).(color.RGBA)
				if got.A > 0 && got.A < 127 {
					partial = true
				}
				if distance < float64(12*scale)-1 {
					if err := pixelNear(circle, x, y, color.RGBA{}, 0); err != nil {
						return fmt.Errorf("circle inside: %w", err)
					}
				} else if distance > float64(12*scale)+1 {
					if err := pixelNear(circle, x, y, color.RGBA{R: 128, G: 64, B: 32, A: 128}, 1); err != nil {
						return fmt.Errorf("circle outside: %w", err)
					}
				}
				if got.R > got.A || got.G > got.A || got.B > got.A {
					return fmt.Errorf("invalid premultiplied circle pixel %v", got)
				}
			}
		}
		if !partial {
			return fmt.Errorf("circle lacks antialias coverage")
		}
		// 同向绕行两次的单轮廓在非零绕数规则下仍填满；偶奇规则会错误保留中心。
		winding, err := p.RenderImage(24, 24, 1, func() {
			p.Clear(graphics.Color{R: .5, G: .25, B: .125, A: .5})
			p.ClearPath(graphics.MoveTo(4, 4).LineTo(18, 4).LineTo(18, 18).LineTo(4, 18).
				LineTo(4, 4).LineTo(18, 4).LineTo(18, 18).LineTo(4, 18).Close())
		})
		if err != nil {
			return err
		}
		if err := pixelNear(winding, 10, 10, color.RGBA{}, 0); err != nil {
			return fmt.Errorf("nonzero winding: %w", err)
		}
		for _, transform := range []geometry.Transform{geometry.Translate(8, 8), geometry.Translate(8, 16).Rotate(90), geometry.Translate(8, 8).Scale(2, .5)} {
			result, err := p.RenderImage(40, 40, 1, func() {
				p.Clear(graphics.RGB(80, 120, 200))
				p.SetTransform(transform)
				p.SetClipRect(graphics.Rect(0, 0, 14, 40))
				p.ClearPath(rectangle(0, 0, 8, 8))
			})
			if err != nil {
				return err
			}
			for _, s := range []struct {
				x, y int
				want color.RGBA
			}{{11, 9, color.RGBA{}}, {15, 9, color.RGBA{R: 80, G: 120, B: 200, A: 255}}, {2, 2, color.RGBA{R: 80, G: 120, B: 200, A: 255}}} {
				if err := pixelNear(result, s.x, s.y, s.want, 0); err != nil {
					return fmt.Errorf("transform=%v: %w", transform, err)
				}
			}
		}
	}
	return nil
}

func checkCornerTangencies(p graphics.Painter, scale float32) error {
	w, h := int(128*scale), int(96*scale)
	for _, border := range []bool{false, true} {
		img, err := p.RenderImage(w, h, scale, func() {
			p.Clear(graphics.Color{})
			p.FillRect(graphics.Rect(0, 0, 128, 96), graphics.RGB(245, 246, 248))
			if border {
				// 与 Integrated 一样保持内轮廓、把边框外轮廓放到最终遮罩以外。
				margin := 2 / scale
				stroke := float32(1) + margin
				half := stroke / 2
				p.DrawRoundRect(graphics.Rect(-margin+half, -margin+half, 128+2*margin-stroke, 96+2*margin-stroke),
					12+margin-half, stroke, graphics.RGB(165, 165, 165))
			}
			clearCorners(p, 128, 96, 12, scale)
		})
		if err != nil {
			return err
		}
		for step := 0; step < 3; step++ {
			y := int(12*scale) - 1 - step
			// 用圆的面积独立估算边缘像素覆盖率，采用 64x64 子像素采样。
			// 切点处要求轮廓落在原始圆内外各 1/4 物理像素的面积上下界中，
			// 容纳路径细分和原生 AA 曲线，而非人为固定两后端的 Alpha 差值。
			// 旧路径将这些大部分在圆内的像素整颗擦除，明确违反该下界。
			var inside [2]int
			const samples = 64
			r := float64(12 * scale)
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					dx := (float64(sx)+.5)/samples - r
					dy := float64(y) + (float64(sy)+.5)/samples - r
					for i, limit := range []float64{r - .25, r + .25} {
						if dx*dx+dy*dy <= limit*limit {
							inside[i]++
						}
					}
				}
			}
			// 面积采样及 8 位量化的边界各留 2，不放宽几何误差范围。
			lower := int(math.Round(float64(inside[0])*255/(samples*samples))) - 2
			upper := int(math.Round(float64(inside[1])*255/(samples*samples))) + 2
			for _, q := range [][2]int{{0, y}, {y, 0}, {w - 1, y}, {w - 1 - y, 0},
				{w - 1, h - 1 - y}, {w - 1 - y, h - 1}, {0, h - 1 - y}, {y, h - 1}} {
				got := color.RGBAModel.Convert(img.At(q[0], q[1])).(color.RGBA)
				if int(got.A) < lower || int(got.A) > upper {
					return fmt.Errorf("corner tangent %s scale=%g border=%v pixel=%v alpha=%d want=[%d,%d]", p.Name(), scale, border, q, got.A, lower, upper)
				}
				if got.R > got.A || got.G > got.A || got.B > got.A {
					return fmt.Errorf("corner tangent invalid premultiplied pixel %v", got)
				}
			}
		}
	}
	return nil
}

func pixelNear(img image.Image, x, y int, want color.RGBA, tolerance int) error {
	got := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
	for _, v := range [][2]uint8{{got.R, want.R}, {got.G, want.G}, {got.B, want.B}, {got.A, want.A}} {
		if math.Abs(float64(v[0])-float64(v[1])) > float64(tolerance) {
			return fmt.Errorf("pixel(%d,%d)=%v want %v tolerance=%d", x, y, got, want, tolerance)
		}
	}
	return nil
}
