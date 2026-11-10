// TextInput/TextView空闲常亮及停止重绘验收。
//
// 环境：Linux独立X11 Software/OpenGL、Windows Direct2D、macOS OpenGL；1x/2x。
// 启动：go run ./tests/window/gui/caret_idle [-auto]，GOUI_PLAT_PAINTER及
// GOUI_PLAT_SCALE指定后端和缩放。自动模式约需30秒，必须保持窗口激活、不操作。
// 初始：Modern亮色，空的单行和多行编辑框；单行框获得焦点。
// 操作与预期：
// 1. 不输入11秒；光标从每500ms闪烁转为常亮，随后不再因光标产生绘制。
// 2. 移入/移出鼠标、调整窗口大小；光标保持常亮，不能重新开始周期重绘。
// 3. 点击原光标位置，或在空文本按Home/左键/Backspace；恢复闪烁。
// 4. 切换到多行编辑框重复；切换到其他应用时停止闪烁但保留常亮，返回后恢复。
// 自动模式通过Window.DispatchEvent发送Home；用diagnostics实际绘制请求计数与
// RenderWidget的独立光标颜色采样验证两种控件的空闲、布局/样式维护及交互恢复。
// 每种控件等待真实10秒，不缩短生产超时。观察计时器只读取结果，不请求帧，不用HUD。
// 平台差异：字体与窗口装饰由系统决定；采样空文本插入光标，不依赖字体金图。
// 像素为预乘sRGB/RGBA8，光标采用公开样式解析结果，颜色容许1级量化误差。
// 副作用：仅本进程窗口、终端日志；不会写文件、修改剪贴板或系统设置。
// 复位：重新启动。退出：关闭窗口；-auto输出PASS后退出，提前关闭视为未完成。
package main

import (
	"flag"
	"fmt"
	"image/color"
	"math"
	"os"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/diagnostics"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/modern"
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

type caretEditor interface {
	gui.Widget
	Padding() geometry.Insets
}

// 空文本左内边缘只可能有插入光标；背景/边框不在此采样区域。
func checkSteadyCaret(widget caretEditor, name string) {
	foreground, _ := gui.WidgetStyle(widget, name, "caret").ForegroundColor()
	check(foreground != nil, "缺少光标前景样式")
	want := color.RGBAModel.Convert(foreground).(color.RGBA)
	for _, scale := range []float32{1, 2} {
		img, err := gui.RenderWidget(widget, scale)
		must(err)
		padding := widget.Padding()
		x0 := int(math.Floor(float64(padding.Left*scale) + .5))
		count := 0
		for y := int(padding.Top*scale) + 1; y < int((widget.Rect().Height-padding.Bottom)*scale)-1; y++ {
			for x := x0; x < x0+int(scale); x++ {
				got := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
				match := true
				for i, channel := range []uint8{got.R, got.G, got.B, got.A} {
					expected := []uint8{want.R, want.G, want.B, want.A}[i]
					match = match && math.Abs(float64(channel)-float64(expected)) <= 1
				}
				if match {
					count++
				}
			}
		}
		check(count >= 4, fmt.Sprintf("%s %gx空闲光标不可见：像素数%d", name, scale, count))
	}
}

func main() {
	auto := flag.Bool("auto", false, "自动检查空闲、维护及交互恢复后退出")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.CaretIdle")
	must(err)
	applyStyle := func() { app.SetStyleSheet(modern.Sheet(modern.Options{})) }
	applyStyle()
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 460, Height: 280}})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("GOUI caret idle — wait 10 seconds"))
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(geometry.UniformInsets(16))
	root.SetSpacing(10)
	root.AddChild(gui.NewLabel("TextInput / TextView — idle caret stays visible"))
	input, view := gui.NewTextInput(), gui.NewTextView()
	view.SetMinSize(geometry.Size{Height: 120})
	root.AddChild(input)
	root.AddChild(view)
	win.SetWidget(root)
	check(win.SetFocusedWidget(input), "单行框未获得逻辑焦点")
	must(win.Show())
	done := false
	if *auto {
		capture, err := diagnostics.Start(app, diagnostics.Options{})
		must(err)
		defer capture.Stop()
		editors := []caretEditor{input, view}
		names := []string{"text-input", "text-view"}
		index, phase := 0, 0
		started, since := time.Now(), time.Now()
		var baseline diagnostics.Root
		timer := app.NewTimer()
		timer.ConnectTimeout(func() {
			now := time.Now()
			stats := capture.Snapshot().Roots
			if len(stats) == 0 || stats[0].Backend == "" {
				check(now.Sub(started) < 5*time.Second, "没有实际绘制记录")
				return
			}
			current := stats[0]
			if requested := os.Getenv("GOUI_PLAT_PAINTER"); requested != "" {
				check(strings.EqualFold(current.Backend, requested), "未使用指定后端："+current.Backend)
			}
			check(win.Focused(), "自动验收需要保持窗口激活")
			switch phase {
			case 0:
				check(win.SetFocusedWidget(editors[index]), "编辑框未获得焦点")
				// Home即使位于空文本开头也必须开启一个新的闪烁周期。
				must(win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyHome}))
				baseline, since, phase = current, now, 1
			case 1:
				if now.Sub(since) < 1100*time.Millisecond {
					return
				}
				check(current.PaintRequests > baseline.PaintRequests, "开始后未发生闪烁重绘")
				phase = 2
			case 2:
				if now.Sub(since) < 11*time.Second {
					return
				}
				checkSteadyCaret(editors[index], names[index])
				baseline, since, phase = current, now, 3
			case 3:
				if now.Sub(since) < time.Second {
					return
				}
				check(current.PaintRequests == baseline.PaintRequests && current.Completed == baseline.Completed, "空闲后仍有周期重绘")
				win.RequestLayout()
				applyStyle()
				since, phase = now, 4
			case 4:
				if now.Sub(since) < 800*time.Millisecond {
					return
				}
				checkSteadyCaret(editors[index], names[index])
				baseline, since, phase = current, now, 5
			case 5:
				if now.Sub(since) < time.Second {
					return
				}
				check(current.PaintRequests == baseline.PaintRequests && current.Completed == baseline.Completed, "布局/样式维护重新启动了闪烁")
				must(win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyHome}))
				baseline, since, phase = current, now, 6
			case 6:
				if now.Sub(since) < 800*time.Millisecond {
					return
				}
				check(current.PaintRequests > baseline.PaintRequests, "边界Home没有恢复闪烁")
				fmt.Printf("PASS %s idle/maintenance/resume backend=%s host_scale=%s\n", names[index], current.Backend, os.Getenv("GOUI_PLAT_SCALE"))
				index++
				if index < len(editors) {
					phase = 0
					return
				}
				timer.Stop()
				done = true
				fmt.Println("PASS caret_idle")
				win.Destroy()
			}
		})
		must(timer.Start(200 * time.Millisecond))
	}
	must(app.Run())
	check(!*auto || done, "验收前窗口已关闭")
}
