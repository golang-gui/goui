// ProgressBar 命令式窗口验收，不依赖 DevServer。
// 环境：三平台可交互桌面，记录 OS、Linux WM、后端与缩放。
// 启动：go run ./tests/window/widgets/progress_bar。GOUI_PLAT_PAINTER=software/opengl，
// Windows 另可 direct2d；GOUI_PLAT_SCALE=2 请求 2x。
// 初始：Integrated 900×560 DIP，Modern 亮色蓝主题，确定进度 25%，下方两项忙等待。
//
// 操作与预期：
//  1. +25% 循环 50%、75%、100%、0%，Reset 回 25%；长条比例正确，圆环从
//     顶部顺时针增长，100% 闭合无接缝，0% 只有轨道，进度变化不改变布局。
//  2. 下方长条胶囊从左侧进入、始终向右移出，再从左侧循环，无回头或可见跳变；
//     圆形为固定 90° 圆头弧段，顺时针匀速旋转，1.4 秒一圈，不伸缩、不倒退，
//     周期衔接无停顿／跳变；Busy 切换为确定进度后停止，
//     保存当前比例，再次 Busy 恢复动画。等待不改变上方进度或布局。
//  3. Hide 隐藏下方共同祖先，再次点击恢复，动画可恢复且无隐藏残影。
//  4. Dark／Accent 切换亮暗及蓝／红，轨道保持中性色；大小、比例和活动模式
//     不变。拖动窗口大小，圆形不拉成椭圆，不超出控件边界。
//  5. Check 输出 ASSERT PASS：核对角色、形态、确定值、忙等待省略值，无
//     可调节 Range／点击动作。模式／隐藏变化后等 100ms 再 Check，同时检查
//     当前应有的活动 Timer 数量（忙且可见时两个，否则零）；此断言不替代动画观察。
//  6. 在 1x/2x 重复以上步骤，4 DIP 对应 4/8 物理像素，圆头不出现接缝／加深。
//
// 平台差异：字体／装饰可不同，边缘抗锯齿允许细微差别，不承诺刷新同步。
// 副作用：窗口与终端日志，无系统设置／剪贴板／文件写入。
// 复位：Reset 恢复比例，重启恢复全部状态。退出：关闭主窗口，先检查所有 Timer
// 已停止再退出事件循环；终端输出 DESTROY PASS。断言失败 panic。
package main

import (
	"fmt"
	"image/color"
	"log"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

// Only observe public Timer contracts; no control/scheduler private state.
type progressApplication struct {
	gui.Application
	timers []*gui.Timer
}

func (a *progressApplication) NewTimer() *gui.Timer {
	timer := a.Application.NewTimer()
	a.timers = append(a.timers, timer)
	return timer
}
func (a *progressApplication) checkTimers(want int) {
	active := 0
	for _, timer := range a.timers {
		if timer.Active() {
			active++
		}
	}
	if active != want {
		panic(fmt.Sprintf("active timers: got %d, want %d", active, want))
	}
}

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	app, err := gui.NewApplication("org.golang-gui.ProgressBarTest")
	if err != nil {
		log.Fatal(err)
	}
	monitor := &progressApplication{Application: app}
	gui.App = monitor
	app.SetQuitOnLastWindowClosed(false)
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 900, Height: 560}, Chrome: gui.WindowChromeIntegrated})
	if err != nil {
		log.Fatal(err)
	}
	if err := win.SetTitle("GOUI ProgressBar GUI"); err != nil {
		log.Fatal(err)
	}
	win.ConnectDestroy(func() {
		app.Post(func() {
			monitor.checkTimers(0) // Before Quit: host teardown, not app-wide cancellation.
			fmt.Println("DESTROY PASS")
			app.Quit()
		})
	})
	value, busy, visible, dark, red := float32(.25), true, true, false, false
	all := []*widgets.ProgressBar{widgets.NewProgressBar(), widgets.NewProgressBar(), widgets.NewProgressBar(), widgets.NewProgressBar()}
	for i, p := range all {
		p.SetID(fmt.Sprintf("progress-%d", i))
		if i%2 == 1 {
			p.SetShape(widgets.ProgressCircular)
			p.SetMinSize(geometry.Size{Width: 48, Height: 48})
			p.SetMaxSize(geometry.Size{Width: 48, Height: 48})
		} else {
			p.SetMainWeight(1)
		}
	}
	row := func(text string, p *widgets.ProgressBar) *gui.LinearBox {
		b := gui.NewLinearBox(layout.DirectionHorizontal)
		b.SetSpacing(12)
		label := gui.NewLabel(text)
		label.SetMinSize(geometry.Size{Width: 160})
		b.AddChild(label)
		b.AddChild(p)
		return b
	}
	activity := gui.NewLinearBox(layout.DirectionVertical)
	activity.SetCrossAlign(layout.CrossStretch)
	activity.SetSpacing(24)
	activity.AddChild(row("Linear activity", all[2]))
	activity.AddChild(row("Circular / Spinner", all[3]))
	status := gui.NewLabel("")
	refresh := func() {
		accent := color.RGBA{R: 70, G: 130, B: 220, A: 255}
		if red {
			accent = color.RGBA{R: 210, G: 48, B: 48, A: 255}
		}
		app.SetStyleSheet(modern.Sheet(modern.Options{Dark: dark, AccentColor: accent}))
		for i, p := range all {
			p.SetValue(value)
			p.SetIndeterminate(i >= 2 && busy)
		}
		activity.SetVisible(visible)
		status.SetText(fmt.Sprintf("Value %.0f%% | Busy %v | Visible %v | Dark %v | Red %v", value*100, busy, visible, dark, red))
	}
	check := func() {
		for i, p := range all {
			info := p.Snapshot()
			progress := info.Attributes[widgets.ProgressInfoKey].(widgets.ProgressInfo)
			wantBusy, wantShape := i >= 2 && busy, widgets.ProgressLinear
			if i%2 == 1 {
				wantShape = widgets.ProgressCircular
			}
			if info.Role != widgets.RoleProgressBar || info.Focusable || len(info.Actions) != 0 || info.Range != nil ||
				progress.Shape != wantShape || progress.Indeterminate != wantBusy ||
				wantBusy && progress.Value != nil || !wantBusy && (progress.Value == nil || *progress.Value != value) {
				panic(fmt.Sprintf("progress %d incorrect: %+v", i, info))
			}
		}
		if all[0].Rect().Width > 0 { // Startup Check precedes the first layout.
			want := 0
			if busy && visible {
				want = 2
			}
			monitor.checkTimers(want)
		}
		fmt.Printf("ASSERT PASS value=%.2f busy=%v visible=%v dark=%v\n", value, busy, visible, dark)
	}
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetSpacing(8)
	button := func(text string, fn func()) {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		b.ConnectClicked(fn)
		toolbar.AddChild(b)
	}
	button("+25%", func() {
		value += .25
		if value > 1 {
			value = 0
		}
		refresh()
	})
	button("Reset", func() { value = .25; refresh() })
	button("Busy", func() { busy = !busy; refresh() })
	button("Hide", func() { visible = !visible; refresh() })
	button("Dark", func() { dark = !dark; refresh() })
	button("Accent", func() { red = !red; refresh() })
	button("Check", check)
	body := gui.NewLinearBox(layout.DirectionVertical)
	body.SetCrossAlign(layout.CrossStretch)
	body.SetPadding(20)
	body.SetSpacing(24)
	for _, w := range []gui.Widget{status, toolbar, row("Linear value", all[0]), row("Circular value", all[1]), activity} {
		body.AddChild(w)
	}
	header := gui.NewHeaderBar()
	header.SetChild(gui.NewLabel("Progress indicators"))
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	root.AddChild(header)
	root.AddChild(body)
	refresh()
	win.SetWidget(root)
	if err := win.Show(); err != nil {
		log.Fatal(err)
	}
	check()
	app.Run()
}
