// ProgressBar 声明式窗口验收，不依赖 DevServer。
// 环境：三平台可交互桌面，记录 OS、Linux WM、后端与缩放。
// 启动：go run ./tests/window/widgets/ui_progress_bar。GOUI_PLAT_PAINTER=software/opengl，
// Windows 另可 direct2d；GOUI_PLAT_SCALE=2 请求 2x。
// 初始：Integrated 900×560 DIP，Modern 亮色蓝主题，确定进度 25%，下方两项忙等待。
//
// 操作与预期：
//  1. +25% 到 50%、75%、100%、0%，Reset 回 25%；长条比例与圆环同步，
//     圆环从顶部顺时针增长，100% 闭合无接缝，0% 只显示轨道，布局不变。
//  2. 下方长条始终向右，完整移出后从左侧循环，无回头／可见跳变；圆形为
//     固定 90° 圆头弧段，顺时针匀速旋转，1.4 秒一圈，不伸缩、停顿或跳变。
//     Rebuild 仅请求 UI 重建，反复点击不使动画跳回起点，也不创建新的 Timer。
//     Check 输出 ASSERT PASS，核对四个 ID 的 Widget 身份、形态、比例／忙等待
//     和快照；模式／隐藏变化后等 100ms 再 Check，活动 Timer 应有两个（忙且
//     可见）或零。多次 Check 不替代动画及像素观察。
//  3. Busy 切换下方模式，切回确定进度时停表并显示保存值。Hide 隐藏共同祖先，
//     恢复后正常动画，无残影，上方按钮始终可用。
//  4. Dark／Accent 切换亮暗及蓝／红，轨道中性，前景跟随主题，不重置活动。
//     调整窗口大小，圆形不变成椭圆、绘制不越界。
//  5. 在 1x/2x 重复以上步骤，4 DIP 对应 4/8 物理像素，圆头无接缝／加深。
//
// 平台差异：字体／装饰可不同，抗锯齿允许细微差别，不承诺刷新同步。
// 副作用：窗口与日志，无系统设置／剪贴板／文件写入。
// 复位：Reset 恢复比例，重启恢复全部状态。退出：关闭主窗口，先检查所有 Timer
// 已停止再退出事件循环；终端输出 DESTROY PASS。断言失败 panic。
package main

import (
	"fmt"
	"image/color"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

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
	value, busy, visible, dark, red := float32(.25), true, true, false, false
	ids := []string{"linear", "circular", "busy-linear", "spinner"}
	previous := make(map[string]gui.Widget)
	var monitor *progressApplication
	if err := ui.Run("org.golang-gui.ProgressBarUITest", func(app ui.App) ui.RootView {
		if monitor == nil {
			monitor = &progressApplication{Application: gui.App}
			gui.App = monitor
			gui.App.SetQuitOnLastWindowClosed(false)
		}
		accent := color.RGBA{R: 70, G: 130, B: 220, A: 255}
		if red {
			accent = color.RGBA{R: 210, G: 48, B: 48, A: 255}
		}
		check := func() {
			win := app.FindWindow("progress")
			for i, id := range ids {
				widget := gui.FindWidget(win, id)
				p, ok := widget.(*widgets.ProgressBar)
				if !ok || previous[id] != nil && previous[id] != widget {
					panic("progress widget replaced: " + id)
				}
				previous[id] = widget
				info := p.Snapshot()
				progress := info.Attributes[widgets.ProgressInfoKey].(widgets.ProgressInfo)
				wantBusy, wantShape := i >= 2 && busy, widgets.ProgressLinear
				if i%2 == 1 {
					wantShape = widgets.ProgressCircular
				}
				if info.Role != widgets.RoleProgressBar || info.Range != nil || len(info.Actions) != 0 ||
					p.Value() != value || progress.Shape != wantShape || progress.Indeterminate != wantBusy ||
					wantBusy && progress.Value != nil || !wantBusy && (progress.Value == nil || *progress.Value != value) {
					panic("progress snapshot incorrect: " + id)
				}
			}
			want := 0
			if busy && visible {
				want = 2
			}
			monitor.checkTimers(want)
			fmt.Printf("ASSERT PASS value=%.2f busy=%v visible=%v dark=%v retained=true\n", value, busy, visible, dark)
		}
		row := func(text string, p *wui.ProgressBarView) ui.View {
			return ui.HBox(ui.Label(text).MinSize(160, 0), p).Spacing(12)
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{Dark: dark, AccentColor: accent})).Windows(
			ui.Window("progress").Title("GOUI ProgressBar UI").Size(900, 560).Chrome(ui.WindowChromeIntegrated).
				OnDestroy(func() {
					app.Post(func() {
						monitor.checkTimers(0) // Check host teardown before quitting the application.
						fmt.Println("DESTROY PASS")
						app.Quit()
					})
				}).Content(
				ui.VBox(
					ui.HeaderBar(ui.Label("Progress indicators")),
					ui.VBox(
						ui.Label(fmt.Sprintf("Value %.0f%% | Busy %v | Visible %v | Dark %v | Red %v", value*100, busy, visible, dark, red)),
						ui.HBox(
							ui.Button("+25%").OnClick(func() {
								value += .25
								if value > 1 {
									value = 0
								}
								app.RequestUpdate()
							}),
							ui.Button("Reset").OnClick(func() { value = .25; app.RequestUpdate() }),
							ui.Button("Busy").OnClick(func() { busy = !busy; app.RequestUpdate() }),
							ui.Button("Hide").OnClick(func() { visible = !visible; app.RequestUpdate() }),
							ui.Button("Dark").OnClick(func() { dark = !dark; app.RequestUpdate() }),
							ui.Button("Accent").OnClick(func() { red = !red; app.RequestUpdate() }),
							ui.Button("Rebuild").OnClick(func() {
								count := len(monitor.timers)
								app.RequestUpdate()
								app.Post(func() {
									if len(monitor.timers) != count {
										panic("rebuild allocated new timers")
									}
								})
							}),
							ui.Button("Check").OnClick(check),
						).Spacing(8),
						row("Linear value", wui.ProgressBar(value).ID(ids[0]).MainWeight(1)),
						row("Circular value", wui.ProgressBar(value).ID(ids[1]).Shape(wui.ProgressCircular).MinSize(48, 48).MaxSize(48, 48)),
						ui.VBox(
							row("Linear activity", wui.ProgressBar(value).ID(ids[2]).Indeterminate(busy).MainWeight(1)),
							row("Circular / Spinner", wui.ProgressBar(value).ID(ids[3]).Shape(wui.ProgressCircular).Indeterminate(busy).MinSize(48, 48).MaxSize(48, 48)),
						).CrossAlign(layout.CrossStretch).Spacing(24).Visible(visible),
					).CrossAlign(layout.CrossStretch).Padding(20).Spacing(24),
				).CrossAlign(layout.CrossStretch),
			),
		)
	}); err != nil {
		panic(err)
	}
}
