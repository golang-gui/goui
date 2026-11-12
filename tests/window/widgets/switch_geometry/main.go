// Switch 轨道与滑块共心回归，不依赖 DevServer。
// 环境：真实桌面事件循环；记录 OS/WM、实际 Painter 和缩放。
// 启动：go run ./tests/window/widgets/switch_geometry [-chrome=native|integrated]
// 后端：GOUI_PLAT_PAINTER=software/opengl（Windows 可 direct2d）；GOUI_PLAT_SCALE=1/2。
// 指定 -expect-scale=2 可等待实际帧确认请求的缩放，3s 未观察到则失败。
// 初始：三行固定高度（32、33、29.5 DIP），每行四个 0/.25/.5/.75 DIP 内偏移的开关。
// 红轨道、蓝圆点仅用于分离覆盖率，不是主题配色；开关禁用自动动画。
// 自动步骤与预期：
//  1. 等待实际首帧及有效布局，输出实际后端、PixelScale 和装饰模式；不接受装饰回退。
//  2. 对每个容器按 1/1.25/1.5/2x RenderWidget，保留子控件的分数平移；分别检查
//     关闭/开启状态，共 96 幅预乘 sRGB RGBA8 图像。蓝通道单独表示滑块覆盖率。
//  3. 用独立物理边界公式计算轨道中心，滑块像素面积重心须与其相差 <=0.08 px。
//     容差覆盖原生 AA 的采样差异，仍拒绝原有 0.5 px 错位；另外与同后端公共
//     FillEllipse 按独立预期几何绘制的标准圆逐像素比较蓝通道，允许 2/255 量化差，
//     防止用变形或改变大小掩盖偏移，不强求原生 AA 等于理想圆的面积积分。
//  4. 所有断言通过后输出 REGRESSION PASS 并销毁窗口，检查控件已销毁后正常退出。
//     任一断言失败 panic；提前关闭按未完成处理，不计为通过。
//
// 平台差异：AA 不比较跨后端金图；逻辑缩放不代表真实 HiDPI，交叉编译不代表运行。
// 副作用：创建测试窗口和后端离屏图像，输出日志；不修改系统设置、文件或剪贴板。
// 复位：重新启动。退出：自动完成；手动关闭会中断验收。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/widgets"
)

func main() {
	chromeFlag := flag.String("chrome", "integrated", "native/integrated")
	expectScale := flag.Float64("expect-scale", 0, "等待并核对实际帧的缩放；0 不指定")
	flag.Parse()
	chrome := gui.WindowChromeIntegrated
	if *chromeFlag == "native" {
		chrome = gui.WindowChromeNative
	} else if *chromeFlag != "integrated" {
		panic("未知 chrome 参数")
	}
	app, err := gui.NewApplication("org.golang-gui.SwitchGeometry")
	must(err)
	gui.App = app
	var rules []style.Rule
	for _, suffix := range []string{"", "-checked"} {
		rules = append(rules,
			style.Name("switch").Part("track"+suffix).BackgroundColor(color.RGBA{R: 255, A: 255}).BorderWidth(0).Radius(10),
			style.Name("switch").Part("thumb"+suffix).BackgroundColor(color.RGBA{B: 255, A: 255}).BorderWidth(0),
		)
	}
	app.SetStyleSheet(style.Sheet(rules...))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 620, Height: 300}, Chrome: chrome})
	must(err)
	must(win.SetTitle("GOUI Switch geometry"))
	win.Chrome().ConnectInfo(func(info gui.ChromeInfo) {
		if info.Mode != chrome {
			panic(fmt.Sprintf("装饰回退：请求 %v，实际 %+v", chrome, info))
		}
	})
	root := gui.NewLinearBox(layout.DirectionVertical)
	header := gui.NewHeaderBar()
	header.SetChild(gui.NewLabel("Switch geometry"))
	root.AddChild(header)
	root.AddChild(gui.NewLabel("Red track / blue thumb; automatic center and circle assertions"))
	type probe struct {
		box            *gui.LinearBox
		button         *widgets.Switch
		height, offset float32
	}
	var probes []probe
	for _, height := range []float32{32, 33, 29.5} {
		row := gui.NewLinearBox(layout.DirectionHorizontal)
		row.SetSpacing(20)
		row.AddChild(gui.NewLabel(fmt.Sprintf("height %g", height)))
		for _, offset := range []float32{0, .25, .5, .75} {
			button := widgets.NewSwitch()
			button.SetAnimated(false)
			button.SetMinSize(geometry.Size{Width: 48, Height: height})
			button.SetMaxSize(geometry.Size{Width: 48, Height: height})
			box := gui.NewLinearBox(layout.DirectionVertical)
			box.SetPadding(geometry.UniformInsets(offset))
			box.AddChild(button)
			row.AddChild(box)
			probes = append(probes, probe{box, button, height, offset})
		}
		root.AddChild(row)
	}
	reference := new(circleReference)
	root.AddChild(reference)
	root.SetSpacing(12)
	win.SetWidget(root)
	painted, complete := false, false
	var lastScale float32
	win.ConnectAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
		if painted && lastScale == p.PixelScale() {
			return
		}
		painted = true
		lastScale = p.PixelScale()
		img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		must(err)
		fmt.Printf("BACKEND %T scale=%g chrome=%s\n", img, p.PixelScale(), *chromeFlag)
		img.Destroy()
	})
	observer := app.NewTimer()
	win.ConnectDestroy(func() {
		observer.Stop()
		if !complete {
			panic("回归未完成，窗口提前关闭")
		}
		app.Post(func() {
			for _, probe := range probes {
				if !probe.button.Destroyed() {
					panic("窗口销毁后仍有存活控件")
				}
			}
			app.Quit()
		})
	})
	deadline := time.Now().Add(3 * time.Second)
	observer.ConnectTimeout(func() {
		if !painted || probes[0].button.Rect().Width == 0 || *expectScale > 0 && math.Abs(float64(lastScale)-*expectScale) > .001 {
			if time.Now().After(deadline) {
				panic(fmt.Sprintf("未观察到预期布局/缩放：实际 %g，要求 %g", lastScale, *expectScale))
			}
			return
		}
		observer.Stop()
		checks := 0
		for _, scale := range []float32{1, 1.25, 1.5, 2} {
			for _, probe := range probes {
				if probe.button.Rect().Size != (geometry.Size{Width: 48, Height: probe.height}) {
					panic("测试没有得到请求的分数高度布局")
				}
				factor := min(float64(1), float64(probe.height-12)/20)
				edge := func(v float64) float64 { return math.Floor((v+float64(probe.offset))*float64(scale) + .5) }
				left, right := edge(6), edge(6+36*factor)
				top := edge(6 + (float64(probe.height)-12-20*factor)/2)
				bottom := edge(6 + (float64(probe.height)-12+20*factor)/2)
				for _, checked := range []bool{false, true} {
					probe.button.SetChecked(checked)
					img, err := gui.RenderWidget(probe.box, scale)
					must(err)
					cx := left + (bottom-top)/2
					if checked {
						cx = right - (bottom-top)/2
					}
					cy, radius := (top+bottom)/2, (bottom-top)*.4
					checkCenter(img, cx, cy)
					reference.center = geometry.Point{X: float32(cx / float64(scale)), Y: float32(cy / float64(scale))}
					reference.radius = float32(radius / float64(scale))
					reference.Arrange(geometry.Rectangle{Size: probe.box.Rect().Size})
					standard, err := gui.RenderWidget(reference, scale)
					must(err)
					checkCirclePixels(img, standard)
					checks++
				}
			}
		}
		complete = true
		fmt.Printf("REGRESSION PASS checks=%d\n", checks)
		win.Destroy()
	})
	must(win.Show())
	must(observer.Start(80 * time.Millisecond))
	must(app.Run())
}

func checkCenter(img image.Image, cx, cy float64) {
	var mass, mx, my float64
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			w := float64(color.RGBAModel.Convert(img.At(x, y)).(color.RGBA).B) / 255
			px, py := float64(x)+.5, float64(y)+.5
			mass, mx, my = mass+w, mx+px*w, my+py*w
		}
	}
	if mass == 0 {
		panic("滑块未绘制")
	}
	x, y := mx/mass, my/mass
	if math.Abs(x-cx) > .08 || math.Abs(y-cy) > .08 {
		panic(fmt.Sprintf("滑块重心=(%.4f,%.4f)，轨道要求=(%.4f,%.4f)", x, y, cx, cy))
	}
}

func checkCirclePixels(actual, standard image.Image) {
	if actual.Bounds() != standard.Bounds() {
		panic("标准圆图像尺寸不匹配")
	}
	for y := 0; y < actual.Bounds().Dy(); y++ {
		for x := 0; x < actual.Bounds().Dx(); x++ {
			got := color.RGBAModel.Convert(actual.At(x, y)).(color.RGBA).B
			want := color.RGBAModel.Convert(standard.At(x, y)).(color.RGBA).B
			if delta := int(got) - int(want); delta < -2 || delta > 2 {
				panic(fmt.Sprintf("滑块圆形/大小错误：(%d,%d) blue=%d，标准圆=%d", x, y, got, want))
			}
		}
	}
}

type circleReference struct {
	gui.WidgetBase
	center geometry.Point
	radius float32
}

func (c *circleReference) Paint(p gui.Painter) {
	p.FillEllipse(c.center, c.radius, c.radius, graphics.RGB(0, 0, 255))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
