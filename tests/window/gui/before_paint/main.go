// 布局后、绘制前准备验收。
//
// 环境：桌面窗口系统；Linux/X11 使用独立会话，验证 Software/OpenGL、1x/2x。
// Windows 可另验 Direct2D，macOS 使用支持的桌面后端；本例不验证移动宿主交付。
// 启动：GOUI_PLAT_PAINTER=opengl GOUI_PLAT_SCALE=1 go run ./tests/window/gui/before_paint
// 默认自动模式，输出 PASS 后退出；-auto=false 手工运行。
// 初始：Integrated 窗口包含说明、操作按钮和两个圆点；连线端点在 BeforePaint 中
// 按最终分配尺寸计算，Paint 读取缓存并断言准备尺寸与当前布局一致。
//
// 操作与逐项预期：
//  1. 拖动调整窗口尺寸：圆点和连线跟随可用空间，不出现旧尺寸或 panic。
//  2. 点击 Repaint：没有布局失效也执行准备，不能因连接信号而持续请求帧。
//  3. 点击 Relayout：先完成分配，再准备；同帧绘制读取新几何。
//  4. 点击 Popup：独立 Popover 同样显示圆点连线，可以反复打开/关闭。
//  5. 点击 Exit 或关闭主窗口退出。自动模式依次请求重绘、布局、打开/销毁 Popup，
//     检查每次 Paint 都有一次准备且位于同一 GUI 线程；提前退出不算通过。
//
// 无文件、网络、剪贴板、DevServer 或系统设置副作用；只创建本例窗口/Popup 和
// 普通 GUI Timer，退出清理。重启即可复位。程序断言不能代替其他平台真机验收。
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/xuges/gothread"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

type preparedArea struct {
	gui.WidgetBase
	size             geometry.Size
	left, right      geometry.Point
	prepares, paints int
	paintPreparation int
	thread           uint64
	connection       signal.Handle
}

func newArea() *preparedArea {
	a := &preparedArea{thread: uint64(gothread.GetId())}
	a.ConnectMount(func() { a.connection = a.Root().ConnectBeforePaint(a.prepare) })
	a.ConnectUnmount(func() {
		if a.connection != nil {
			a.connection.Disconnect()
			a.connection = nil
		}
	})
	return a
}

func (a *preparedArea) checkThread() {
	if uint64(gothread.GetId()) != a.thread {
		panic("准备或绘制偏离 GUI 线程")
	}
}

func (a *preparedArea) prepare() {
	a.checkThread()
	a.size = a.Rect().Size
	a.left = geometry.Point{X: a.size.Width / 4, Y: a.size.Height / 2}
	a.right = geometry.Point{X: a.size.Width * 3 / 4, Y: a.size.Height / 2}
	a.prepares++
}

func (*preparedArea) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: 260, Height: 160}))
}

func (a *preparedArea) Paint(p gui.Painter) {
	a.checkThread()
	if a.size != a.Rect().Size || a.prepares <= a.paintPreparation {
		panic(fmt.Sprintf("绘制读取过期准备：prepared=%v current=%v prepares=%d paints=%d", a.size, a.Rect().Size, a.prepares, a.paints))
	}
	a.paintPreparation = a.prepares
	a.paints++
	p.FillRoundRect(geometry.Rectangle{Size: a.size}, 12, graphics.RGB(235, 240, 246))
	p.DrawLine(a.left, a.right, 3, graphics.RGB(70, 110, 160))
	p.FillEllipse(a.left, 16, 16, graphics.RGB(36, 138, 104))
	p.FillEllipse(a.right, 16, 16, graphics.RGB(54, 112, 204))
}

func main() {
	auto := flag.Bool("auto", true, "自动验收并退出")
	flag.Parse()
	app, err := gui.NewApplication("org.golanggui.BeforePaint")
	if err != nil {
		log.Fatal(err)
	}
	var win gui.Window
	var popup gui.Popover
	passed := false
	app.ConnectActivate(func() {
		app.SetStyleSheet(modern.Sheet(modern.Options{}))
		win, err = app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 640, Height: 400}, Chrome: gui.WindowChromeIntegrated})
		must(err)
		must(win.SetTitle("GOUI BeforePaint"))
		root := gui.NewLinearBox(layout.DirectionVertical)
		root.SetPadding(geometry.UniformInsets(20))
		root.SetSpacing(12)
		root.AddChild(gui.NewLabel("Layout -> BeforePaint -> Paint"))
		row := gui.NewLinearBox(layout.DirectionHorizontal)
		row.SetSpacing(8)
		button := func(text string, fn func()) *gui.Button {
			b := gui.NewButton()
			b.SetChild(gui.NewLabel(text))
			b.ConnectClicked(fn)
			row.AddChild(b)
			return b
		}
		button("Repaint", func() { must(win.RequestPaint()) })
		button("Relayout", win.RequestLayout)
		open := button("Popup", func() { must(popup.Show()) })
		button("Exit", app.Quit)
		root.AddChild(row)
		area := newArea()
		area.SetMainWeight(1)
		root.AddChild(area)
		win.SetWidget(root)
		popupArea := newArea()
		popup = gui.NewPopover(open, nil)
		popup.SetWidget(popupArea)
		must(win.Show())
		if !*auto {
			return
		}
		phase, previous := 0, 0
		var previousSize geometry.Size
		deadline := time.Now().Add(10 * time.Second)
		timer := app.NewTimer()
		timer.ConnectTimeout(func() {
			if time.Now().After(deadline) {
				panic("准备/绘制验收超时")
			}
			switch phase {
			case 0:
				if area.paints == 0 {
					return
				}
				previous = area.paints
				must(win.RequestPaint())
				phase++
			case 1:
				if area.paints == previous {
					return
				}
				previousSize = area.size
				root.SetPadding(geometry.UniformInsets(28))
				must(popup.Show())
				phase++
			case 2:
				if popupArea.paints == 0 {
					return
				}
				if area.size == previousSize {
					panic("布局变化没有更新准备尺寸")
				}
				popup.Destroy()
				previous = area.paints
				must(win.RequestPaint())
				phase++
			case 3:
				if area.paints == previous {
					return
				}
				passed = true
				timer.Stop()
				fmt.Printf("PASS before_paint window=%d popup=%d\n", area.paints, popupArea.paints)
				app.Quit()
			}
		})
		must(timer.Start(150 * time.Millisecond))
	})
	app.ConnectShutdown(func() {
		if popup != nil {
			popup.Destroy()
		}
		if win != nil {
			win.Destroy()
		}
	})
	must(app.Run())
	if *auto && !passed {
		log.Fatal("自动验收未完成")
	}
}
