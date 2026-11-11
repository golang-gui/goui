// ProgressBar 桌面回归：实际帧驱动、宿主可见性及完整圆环接缝，不依赖 DevServer。
// 环境：桌面事件循环；记录 OS、WM、实际 Painter 与缩放，OpenGL 需要原生显示连接。
// 启动：go run ./tests/window/widgets/progress_regression -chrome=native -case=all
// 参数：-chrome=native/integrated；-case=all/animation/ring。
// 请求的装饰模式必须实际选中，不能把 Native 回退计为 Integrated 通过。
// 后端：GOUI_PLAT_PAINTER=software/opengl（Windows 另可 direct2d），GOUI_PLAT_SCALE=1/2。
// 初始：三个 48 DIP 圆环（0%、100%、半透明 99%），下方为长条/圆形忙等待。
// 自动步骤与断言：
//  1. 通过正常 Show/StateEvent 等待原生显示，检查 Visible 与 Snapshot 一致。
//  2. ring：RenderWidget 在实际后端按 1x/2x 读取预乘 sRGB RGBA8；圆心透明，
//     四个轴向中线像素为独立预期轨道/前景色，包含旧路径连接点，允许量化误差 1。
//  3. animation：每 180ms 被动采样真实帧计数和忙等待图像，连续三次图像均应变化；
//     观察 Timer 不请求布局/绘制。依次检查初显、祖先隐藏/恢复、原生隐藏/Show、
//     最小化/Show、切回确定进度/忙等待。隐藏期间切换忙等待使旧帧连接断开，
//     无持续帧；同尺寸恢复后须重新连接并产生变化的动画图像。
//  4. 最终销毁窗口并退出，输出 REGRESSION PASS；任何断言失败以非零退出。
//
// 平台差异：WM 可拒绝隐藏/最小化请求，3s 未观察到预期状态按失败报告；不算通过。
// 副作用：短暂创建、隐藏、最小化及激活本测试窗口；只读离屏像素，不操作剪贴板。
// 复位：重启。退出：自动完成或关闭窗口（中断时失败），不修改系统设置。
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/widgets"
)

func main() {
	chromeFlag := flag.String("chrome", "native", "native/integrated")
	scenario := flag.String("case", "all", "all/animation/ring")
	flag.Parse()
	chrome := gui.WindowChromeNative
	if *chromeFlag == "integrated" {
		chrome = gui.WindowChromeIntegrated
	} else if *chromeFlag != "native" {
		log.Fatal("未知 chrome 参数")
	}
	if *scenario != "all" && *scenario != "animation" && *scenario != "ring" {
		log.Fatal("未知 case 参数")
	}
	app, err := gui.NewApplication("org.golang-gui.ProgressRegression")
	must(err)
	gui.App = app
	app.SetStyleSheet(style.Sheet(
		style.Name("progress-bar").BackgroundColor(color.RGBA{B: 255, A: 255}).ForegroundColor(color.RGBA{R: 255, A: 255}).Radius(2),
		style.Name("alpha-ring").BackgroundColor(color.Transparent).ForegroundColor(color.NRGBA{R: 255, A: 128}),
	))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 480, Height: 360}, Chrome: chrome})
	must(err)
	win.Chrome().ConnectInfo(func(info gui.ChromeInfo) {
		if info.Mode != chrome {
			panic(fmt.Sprintf("装饰模式回退：预期 %v，实际 %+v", chrome, info))
		}
	})
	must(win.SetTitle("GOUI ProgressBar regression"))
	body := gui.NewLinearBox(layout.DirectionVertical)
	body.SetSpacing(12)
	body.SetPadding(geometry.UniformInsets(20))
	var rings []*widgets.ProgressBar
	for i, value := range []float32{0, 1, .99} {
		p := widgets.NewProgressBar()
		p.SetShape(widgets.ProgressCircular)
		p.SetValue(value)
		p.SetMinSize(geometry.Size{Width: 48, Height: 48})
		p.SetMaxSize(geometry.Size{Width: 48, Height: 48})
		if i == 2 {
			p.SetStyleName("alpha-ring")
		}
		rings = append(rings, p)
		body.AddChild(p)
	}
	activity := gui.NewLinearBox(layout.DirectionVertical)
	activity.SetSpacing(12)
	var busy []*widgets.ProgressBar
	for _, shape := range []widgets.ProgressShape{widgets.ProgressLinear, widgets.ProgressCircular} {
		p := widgets.NewProgressBar()
		p.SetShape(shape)
		p.SetIndeterminate(true)
		p.SetMinSize(geometry.Size{Width: 48, Height: 48})
		p.SetMaxSize(geometry.Size{Width: 260, Height: 48})
		activity.AddChild(p)
		busy = append(busy, p)
	}
	body.AddChild(activity)
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	header := gui.NewHeaderBar()
	header.SetChild(gui.NewLabel("ProgressBar regression"))
	root.AddChild(header)
	root.AddChild(body)
	win.SetWidget(root)
	frames := 0
	win.ConnectFrame(func(time.Time) { frames++ }) // 被动观察，不能替动画续帧。
	logged := false
	win.ConnectAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
		if logged {
			return
		}
		logged = true
		img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		must(err)
		fmt.Printf("BACKEND %T scale=%g chrome=%s\n", img, p.PixelScale(), *chromeFlag)
		img.Destroy()
	})
	observer := app.NewTimer()
	complete := false
	win.ConnectDestroy(func() {
		observer.Stop()
		if !complete {
			panic("回归尚未完成，窗口提前销毁")
		}
		app.Quit()
	})
	type stage struct {
		name    string
		enter   func()
		state   gui.WindowState
		visible bool
		active  bool
	}
	stages := []stage{
		{"initial", func() {}, gui.WindowStateNormal, true, true},
		{"ancestor-hidden", func() { activity.SetVisible(false) }, gui.WindowStateNormal, true, false},
		{"ancestor-restored", func() { activity.SetVisible(true) }, gui.WindowStateNormal, true, true},
		{"hidden", func() { win.RequestState(gui.WindowStateHidden) }, gui.WindowStateHidden, false, false},
		{"shown", func() { must(win.Show()) }, gui.WindowStateNormal, true, true},
		{"minimized", func() { win.RequestState(gui.WindowStateMinimized) }, gui.WindowStateMinimized, false, false},
		{"restored", func() { must(win.Show()) }, gui.WindowStateNormal, true, true},
		{"determinate", func() {
			for _, p := range busy {
				p.SetIndeterminate(false)
			}
		}, gui.WindowStateNormal, true, false},
		{"busy-again", func() {
			for _, p := range busy {
				p.SetIndeterminate(true)
			}
		}, gui.WindowStateNormal, true, true},
	}
	index, samples, firstFrame := 0, 0, 0
	var previous [][]byte
	deadline := time.Now().Add(3 * time.Second)
	observer.ConnectTimeout(func() {
		s := stages[index]
		if win.State() != s.state || rings[0].Rect().Width == 0 {
			if time.Now().After(deadline) {
				panic(fmt.Sprintf("%s 未得到原生状态 %v：实际 %v", s.name, s.state, win.State()))
			}
			return
		}
		if *scenario != "ring" && (win.Visible() != s.visible || win.Snapshot().Visible != s.visible) {
			panic(fmt.Sprintf("%s 可见性错误：Visible=%v Snapshot=%v 预期 %v", s.name, win.Visible(), win.Snapshot().Visible, s.visible))
		}
		if index == 0 && samples == 0 && *scenario != "animation" {
			checkRings(rings)
			fmt.Println("RING PASS")
			if *scenario == "ring" {
				complete = true
				win.Destroy()
				return
			}
		}
		if samples == 0 {
			firstFrame = frames
			if s.name == "hidden" {
				// 在原生隐藏已经确认后重设模式，保证 Show 必须重连动画，
				// 不能仅靠隐藏前留下的被动连接让恢复断言通过。
				for _, p := range busy {
					p.SetIndeterminate(false)
					p.SetIndeterminate(true)
				}
			}
		}
		if s.active {
			current := make([][]byte, len(busy))
			for i, p := range busy {
				img, err := gui.RenderWidget(p, 0)
				must(err)
				rgba := image.NewRGBA(img.Bounds())
				draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
				current[i] = rgba.Pix
				if previous != nil && bytes.Equal(previous[i], current[i]) {
					panic(fmt.Sprintf("%s：忙等待 %d 的实际图像没有变化（帧 %d）", s.name, i, frames))
				}
			}
			previous = current
		}
		samples++
		if samples < 4 {
			return
		}
		if s.active && frames-firstFrame < 3 || !s.active && frames != firstFrame {
			panic(fmt.Sprintf("%s：实际帧变化 %d 与活动状态 %v 不符", s.name, frames-firstFrame, s.active))
		}
		fmt.Printf("ANIMATION PASS %s frames=%d\n", s.name, frames-firstFrame)
		index++
		if index == len(stages) {
			complete = true
			fmt.Println("REGRESSION PASS")
			win.Destroy()
			return
		}
		samples, previous = 0, nil
		deadline = time.Now().Add(3 * time.Second)
		stages[index].enter()
	})
	must(win.Show())
	must(observer.Start(180 * time.Millisecond))
	must(app.Run())
}

func checkRings(rings []*widgets.ProgressBar) {
	for _, scale := range []float32{1, 2} {
		for i, p := range rings {
			img, err := gui.RenderWidget(p, scale)
			must(err)
			want := []color.RGBA{{B: 255, A: 255}, {R: 255, A: 255}, {R: 128, A: 128}}[i]
			// 半径 22 DIP 是 4 DIP 环带的中线，离内外 AA 边界均 2 DIP。
			for _, point := range []image.Point{{46, 24}, {24, 46}, {2, 24}, {24, 2}, {24, 24}} {
				c := color.RGBAModel.Convert(img.At(point.X*int(scale), point.Y*int(scale))).(color.RGBA)
				expected := want
				if point == (image.Point{24, 24}) {
					expected = color.RGBA{}
				}
				for channel, got := range []uint8{c.R, c.G, c.B, c.A} {
					value := []uint8{expected.R, expected.G, expected.B, expected.A}[channel]
					if delta := int(got) - int(value); delta < -1 || delta > 1 {
						panic(fmt.Sprintf("圆环 %d %gx %v = %v，预期 %v（误差<=1）", i, scale, point, c, expected))
					}
				}
			}
		}
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
