// 非显示区域绘制与续帧验证。
//
// 环境：独立桌面；Linux/X11 的 Software／OpenGL，Windows 另可用 Direct2D，macOS OpenGL。
// 启动：GOUI_PLAT_PAINTER=opengl GOUI_PLAT_SCALE=1 go run ./tests/window/gui/drawable
// 另验 software、2x；-auto=false 手工运行，-integrated 请求 Integrated 窗口。
// 初始：顶部固定操作区，滚动区显示内容顶部，忙等待已开启，但两个进度条和 DrawingArea
// 位于视口之外。手工模式带 HUD；自动模式只采集，不显示 HUD，避免其清零帧干扰断言。
//
// 操作与预期：
//  1. 保持 Top：HUD 静止一秒后显示 0 FPS，完全裁出的动画不持续续帧。
//  2. 点击 Indicators 或向下滚动：长条／圆形忙等待和缓存绘图持续变化，FPS 恢复。
//  3. 点击 Top：忙等待开关保持开启；视口裁出后停止续帧。滚回自动恢复，不用重新开关。
//  4. 点击 Update hidden probe：改变下方色块；它在视口之外时不请求绘制，滚回显示最新颜色。
//  5. 点击 Capture probe：即使色块在屏幕之外，RenderWidget 仍得到非空图片，不启动续帧。
//  6. 拖动窗口尺寸／滚动条，部分可见时继续绘制，完全裁出时跳过；关闭主窗口退出。
//
// 自动断言额外覆盖：初始无离屏 Paint／栅格生成、稳定离屏请求不转发、布局失效保留、
// 一 DIP 部分可见、RenderWidget 不发 Frame／AfterPaint、状态与 Snapshot.Visible 保持。
// FPS 是 CPU 完成间隔，不是 GPU／显示刷新率。副作用为一个窗口及终端输出，不写文件、
// 不启动 DevServer；重启复位。自动模式输出 PASS 后退出，提前关闭视为未完成验收。
package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/diagnostics"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func check(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

type probe struct {
	gui.WidgetBase
	paints int
	value  graphics.Color
}

func (p *probe) Paint(canvas gui.Painter) {
	p.paints++
	canvas.FillRect(geometry.Rectangle{Size: p.Rect().Size}, p.value)
}

func (p *probe) update() {
	p.value = graphics.RGB(30, 180, 90)
	p.RequestPaint()
}

type content struct {
	gui.WidgetBase
	linear, circular *widgets.ProgressBar
	area             *gui.DrawingArea
	probe            *probe
	continuous       bool
	generations      int
}

func newContent() *content {
	c := &content{linear: widgets.NewProgressBar(), circular: widgets.NewProgressBar(), probe: &probe{value: graphics.RGB(200, 80, 50)}, continuous: true}
	c.circular.SetShape(widgets.ProgressCircular)
	c.area = gui.NewDrawingArea(func(p gui.Painter, size geometry.Size) {
		c.generations++
		p.FillRoundRect(geometry.Rectangle{Size: size}, 8, graphics.RGB(uint8(c.generations%150+50), 100, 180))
	})
	c.area.ConnectShouldDraw(func(redraw *bool) { *redraw = *redraw || c.continuous })
	for _, child := range []gui.Widget{c.linear, c.circular, c.area, c.probe} {
		c.WidgetBase.AddChild(c, child)
	}
	return c
}

func (c *content) Measure(constraint layout.Constraint) layout.Measurement {
	return layout.Measured(constraint.Clamp(geometry.Size{Width: 480, Height: 1100}))
}

func (c *content) Arrange(rect geometry.Rectangle) {
	c.WidgetBase.Arrange(rect)
	c.linear.Arrange(geometry.Rect(20, 420, max(0, rect.Width-40), 8))
	c.circular.Arrange(geometry.Rect(20, 460, 64, 64))
	c.area.Arrange(geometry.Rect(20, 550, max(0, rect.Width-40), 70))
	c.probe.Arrange(geometry.Rect(20, 650, 120, 40))
}

func (c *content) busy(on bool) {
	c.linear.SetIndeterminate(on)
	c.circular.SetIndeterminate(on)
	c.continuous = on
}

func frames(s diagnostics.Snapshot) uint64 {
	var total uint64
	for _, r := range s.Roots {
		total += r.Frames
	}
	return total
}

func main() {
	auto := flag.Bool("auto", true, "自动断言并退出")
	integrated := flag.Bool("integrated", false, "使用 Integrated 窗口")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.Drawable")
	if err != nil {
		log.Fatal(err)
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	options := &gui.WindowOptions{Size: geometry.Size{Width: 640, Height: 470}}
	if *integrated {
		options.Chrome = gui.WindowChromeIntegrated
	}
	win, err := app.NewWindow(options)
	must(err)
	defer win.Destroy()
	must(win.SetTitle("GOUI drawable / scroll activity"))
	c := newContent()
	scroll := gui.NewScrollView()
	scroll.SetChild(c)
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(geometry.UniformInsets(16))
	root.SetSpacing(10)
	row := gui.NewLinearBox(layout.DirectionHorizontal)
	row.SetSpacing(8)
	add := func(label string, fn func()) {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(label))
		b.ConnectClicked(fn)
		row.AddChild(b)
	}
	add("Top", func() { scroll.SetScrollY(0) })
	add("Indicators", func() { scroll.SetScrollY(400) })
	add("Update probe", c.probe.update)
	captureProbe := func() {
		img, err := gui.RenderWidget(c.probe, 1)
		must(err)
		want := color.RGBAModel.Convert(c.probe.value).(color.RGBA)
		check(color.RGBAModel.Convert(img.At(10, 10)).(color.RGBA) == want, "截图未包含色块最新颜色")
	}
	add("Capture probe", captureProbe)
	root.AddChild(row)
	hint := gui.NewLabel("Busy stays enabled. Scroll the indicators out and back in; redraws should stop and resume.")
	hint.SetWrapMode(gui.WrapWordChar)
	root.AddChild(hint)
	root.AddChild(scroll)
	win.SetWidget(root)
	capture, err := diagnostics.Start(app, diagnostics.Options{Widgets: true})
	must(err)
	defer capture.Stop()
	var scale float32
	painted := 0
	win.ConnectAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
		scale = p.PixelScale()
		painted++
		if c.continuous && gui.IsDrawable(c.area) {
			must(win.RequestPaint())
		}
	})
	c.busy(true)
	must(win.Show())
	if !*auto {
		hud, err := capture.ShowHUD(win)
		must(err)
		hud.SetPosition(geometry.Point{X: 16, Y: 110})
		app.Run()
		return
	}
	step := 0
	var settled diagnostics.Snapshot
	var generations, probePaints int
	timer := app.NewTimer()
	timer.ConnectTimeout(func() {
		switch step {
		case 0:
			s := capture.Snapshot()
			check(len(s.Roots) == 1 && s.Roots[0].Completed > 0, "缺少初始完成帧")
			if requested := os.Getenv("GOUI_PLAT_PAINTER"); requested != "" {
				check(strings.EqualFold(s.Roots[0].Backend, requested), "未使用指定绘制后端")
			}
			check(!gui.IsDrawable(c.linear) && !gui.IsDrawable(c.area) && c.generations == 0 && c.probe.paints == 0, "初始屏幕外内容仍被绘制")
			settled = s
			for range 100 {
				c.probe.update()
			}
		case 1, 2:
			check(frames(capture.Snapshot()) == frames(settled), "屏幕外请求唤醒了宿主")
		case 3:
			scroll.SetScrollY(400)
		case 4:
			check(gui.IsDrawable(c.linear) && gui.IsDrawable(c.circular) && gui.IsDrawable(c.area), "滚动后指示器仍不可绘制")
			check(frames(capture.Snapshot()) > frames(settled)+3 && c.generations > 0, "滚回后被动连接没有恢复活动")
		case 5:
			scroll.SetScrollY(0)
		case 6:
			settled = capture.Snapshot()
			generations, probePaints = c.generations, c.probe.paints
			check(!gui.IsDrawable(c.linear) && !gui.IsDrawable(c.area), "滚动后指示器没有裁出")
			for range 100 {
				c.probe.update()
			}
		case 7, 8, 9:
			check(frames(capture.Snapshot()) == frames(settled) && c.generations == generations && c.probe.paints == probePaints, "完全裁出后仍持续绘制")
		case 10:
			prior := painted
			captureProbe()
			check(painted == prior && c.probe.paints == probePaints+1 && !gui.IsDrawable(c.probe), "RenderWidget 改变了屏幕活动或可绘制性")
		case 11:
			check(frames(capture.Snapshot()) == frames(settled), "RenderWidget 调度了屏幕帧")
			c.probe.RequestLayout()
		case 12:
			check(frames(capture.Snapshot()) > frames(settled), "丢弃了屏幕外布局失效")
			scroll.SetScrollY(689) // 色块仅剩一 DIP，必须继续认为可绘制。
		case 13:
			check(gui.IsDrawable(c.probe) && c.probe.paints > probePaints+1, "部分可见内容被剔除")
			scroll.SetScrollY(400)
			settled = capture.Snapshot()
		case 14:
			check(frames(capture.Snapshot()) > frames(settled)+3 && c.linear.Indeterminate() && c.circular.Indeterminate(), "滚回没有保留并恢复忙等待")
			check(c.probe.Snapshot().Visible, "滚动改变了语义 Visible")
			c.busy(false)
		case 15:
			settled = capture.Snapshot()
		case 16, 17:
			check(frames(capture.Snapshot()) == frames(settled), "停止后仍有持续活动")
		case 18:
			timer.Stop()
			fmt.Printf("PASS drawable backend=%s scale=%g frames=%d generations=%d\n", settled.Roots[0].Backend, scale, frames(settled), c.generations)
			win.Destroy()
		}
		step++
	})
	must(timer.Start(300 * time.Millisecond))
	app.Run()
	check(step == 19, "验证完成前窗口已关闭")
}
