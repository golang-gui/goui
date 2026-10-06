// Switch 两态、点击／拖动、短动画及声明协调窗口验收，不依赖 DevServer。
// 环境：Linux X11、Windows、macOS 可交互桌面；记录 OS/WM、后端、缩放。
// 启动：go run ./tests/window/widgets/switch；GOUI_PLAT_PAINTER=software/opengl，
// Windows 另可 direct2d；GOUI_PLAT_SCALE=2 请求 2x；-fallback-style 使用显式兜底。
// -destroy-on-change 独立验证首次用户切换时立即销毁窗口，后续信号不再执行。
// 初始：Integrated 900×540 DIP，Modern 亮色蓝主题；Wi-Fi 关闭、Instant 开启、
// Parent 关闭、Disabled 开启。三个可用项可参与 Tab，实际装饰可能回退 Native。
//
// 操作与逐项预期：
//  1. 点击 Wi-Fi 的轨道或文字均能切换；按下不切换，移出后抬起不切换。
//     滑块在约 120ms 内平滑移动；快速反复切换从当前位置续接、不先跳回端点。
//     Set 静默切换 Wi-Fi，不增加 Changes；动画后 Check/F6 输出 ASSERT PASS。
//  2. 从 Wi-Fi 滑块水平拖动；Changes 及布尔状态保持不变，滑块跟随指针。
//     越过中点松手切换一次，未越过中点不切换，保持当前位置不再额外点击。
//     获胜后可拖出控件并松手；Esc／禁用／隐藏／窗口失焦取消，回到已提交状态。
//     纵向拖动不滑动；从文字开始拖动不变成轨道拖动。程序未含滚动容器，
//     纵向让出父手势由包内正常事件派发测试验证。
//  3. Tab/Shift+Tab 导航，Space/Enter 切换，长按只切换一次。键盘焦点有
//     2 DIP 柔和边框，鼠标焦点没有。Disabled 无输入，也不参与 Tab。
//     Disable 禁用 Wi-Fi；恢复后原状态保留。Animated 关闭自动过渡，拖动仍跟随。
//  4. Instant 无动画；Child 只增加 ChildClicks，不切换 Parent；点击 Parent 文字
//     或轨道切换父项。Rebuild 重复重建保留 Widget 身份和单个 Change 连接。
//  5. Dark/Accent 切换亮暗及蓝／红主题；关闭轨道中性弱轮廓、开启为主题色，
//     白色滑块无描边；悬停／按下时开启轨道边框与填充同色，滑块颜色不变。
//     Disabled 弱化但滑块仍可辨，位置区分状态；文字清晰。1x/2x 与缩放窗口时轨道为胶囊、
//     滑块保持圆形、边框不越界，内容保持垂直居中；主题切换不改变开关状态。
//  6. Check/F6 核对状态／Snapshot／身份；BOUNDS 为已布局的窗口局部 DIP。
//     等待 150ms 后再 Check，应显示 ActiveTimers 0；不以此替代动画观察。
//     关闭窗口后检查 Timer 全部停止、控件均已销毁，输出 DESTROY PASS 并退出。
//  7. -destroy-on-change：先 F6 再点击 Wi-Fi。只有一个 Change，LateChanges 为 0，
//     输出 DESTROY PASS 并正常退出；无悬挂 Timer 或 panic。
//
// 平台差异：原生装饰／字体／抗锯齿可不同；不承诺动画与垂直刷新同步。
// 副作用：仅窗口和标准输出，不修改系统设置、文件或剪贴板。
// 复位：Reset 恢复选择／计数并启用 Wi-Fi；重启恢复主题及全部选项。
// 退出：关闭主窗口。断言失败 panic。
package main

import (
	"flag"
	"fmt"
	"image/color"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	widgetstyle "github.com/golang-gui/goui/widgets/style"
	wui "github.com/golang-gui/goui/widgets/ui"
)

type switchApplication struct {
	gui.Application
	timers []*gui.Timer
}

func (a *switchApplication) NewTimer() *gui.Timer {
	timer := a.Application.NewTimer()
	a.timers = append(a.timers, timer)
	return timer
}
func (a *switchApplication) activeTimers() int {
	active := 0
	for _, timer := range a.timers {
		if timer.Active() {
			active++
		}
	}
	return active
}

func main() {
	fallback := flag.Bool("fallback-style", false, "使用 widgets 显式兜底样式")
	destroyOnChange := flag.Bool("destroy-on-change", false, "首次用户切换后销毁窗口")
	flag.Parse()
	wifi, instant, parent := false, true, false
	enabled, animated, visible, dark, red := true, true, true, false, false
	changes, childClicks, lateChanges := 0, 0, 0
	previous := make(map[string]*widgets.Switch)
	var monitor *switchApplication
	var lateConnected bool
	if err := ui.Run("org.golang-gui.SwitchTest", func(app ui.App) ui.RootView {
		if monitor == nil {
			monitor = &switchApplication{Application: gui.App}
			gui.App = monitor
			gui.App.SetQuitOnLastWindowClosed(false)
		}
		update := func() { app.RequestUpdate() }
		change := func(value *bool) func(bool) {
			return func(checked bool) {
				*value = checked
				changes++
				if *destroyOnChange {
					app.FindWindow("switches").Destroy()
				} else {
					update()
				}
			}
		}
		check := func() {
			win := app.FindWindow("switches")
			for id, want := range map[string]bool{"wifi": wifi, "instant": instant, "parent": parent, "disabled": true} {
				b, ok := gui.FindWidget(win, id).(*widgets.Switch)
				if !ok || b.Checked() != want || previous[id] != nil && previous[id] != b {
					panic("state/identity mismatch: " + id)
				}
				previous[id] = b
				info := b.Snapshot()
				if info.Role != widgets.RoleSwitch || info.Attributes[widgets.SwitchInfoKey].(widgets.SwitchInfo).Checked != want {
					panic("snapshot mismatch: " + id)
				}
				if b.Enabled() != (id != "disabled" && (id != "wifi" || enabled)) {
					panic("enabled mismatch: " + id)
				}
				if b.Padding() != 6 || b.Animated() != (id != "instant" && animated) {
					panic("default/animation mismatch: " + id)
				}
			}
			if !lateConnected {
				previous["wifi"].ConnectChange(func(bool) { lateChanges++ })
				lateConnected = true
			}
			for _, id := range []string{"wifi", "instant", "parent", "child", "disabled", "set", "disable", "animated", "hide", "dark", "accent", "rebuild", "check", "reset"} {
				i := gui.FindWidget(win, id).Snapshot()
				fmt.Printf("BOUNDS %s %.2f %.2f %.2f %.2f\n", id, i.Bounds.X, i.Bounds.Y, i.Bounds.Width, i.Bounds.Height)
				if i.Focused {
					fmt.Printf("FOCUS %s visible=%v\n", id, i.FocusVisible)
				}
			}
			fmt.Printf("ASSERT PASS wifi=%v instant=%v parent=%v changes=%d childClicks=%d enabled=%v animated=%v visible=%v dark=%v red=%v activeTimers=%d retained=true\n", wifi, instant, parent, changes, childClicks, enabled, animated, visible, dark, red, monitor.activeTimers())
		}
		reset := func() {
			wifi, instant, parent = false, true, false
			changes, childClicks = 0, 0
			enabled, visible = true, true
			update()
		}
		accent := color.RGBA{R: 70, G: 130, B: 220, A: 255}
		if red {
			accent = color.RGBA{R: 210, G: 48, B: 48, A: 255}
		}
		settings := app.Settings()
		sheet := modern.Sheet(modern.Options{Dark: dark, AccentColor: accent, FontFamily: settings.FontFamily(), FontSize: settings.FontSize()})
		if *fallback {
			sheet = style.Sheet(append(gui.DefaultStyleRules(), widgetstyle.Rules()...)...)
		}
		return ui.Root().StyleSheet(sheet).Windows(
			ui.Window("switches").Title("GOUI Switch").Size(900, 540).Chrome(ui.WindowChromeIntegrated).
				OnDestroy(func() {
					app.Post(func() {
						if monitor.activeTimers() != 0 {
							panic("destroy retained timers")
						}
						for _, b := range previous {
							if !b.Destroyed() {
								panic("window retained live widget")
							}
						}
						if *destroyOnChange && (changes != 1 || lateChanges != 0) {
							panic("destroyed transition continued")
						}
						fmt.Println("DESTROY PASS")
						app.Quit()
					})
				}).Content(
				ui.VBox(
					ui.HeaderBar(ui.Label("Switch")),
					ui.VBox(
						ui.Label(fmt.Sprintf("Changes %d | ChildClicks %d | Wi-Fi %v | Dark %v", changes, childClicks, wifi, dark)),
						ui.HBox(
							ui.Button("Set").ID("set").OnClick(func() { wifi = !wifi; update() }),
							ui.Button("Disable").ID("disable").OnClick(func() { enabled = !enabled; update() }),
							ui.Button("Animated").ID("animated").OnClick(func() { animated = !animated; update() }),
							ui.Button("Hide").ID("hide").OnClick(func() { visible = !visible; update() }),
							ui.Button("Dark").ID("dark").OnClick(func() { dark = !dark; update() }),
							ui.Button("Accent").ID("accent").OnClick(func() { red = !red; update() }),
							ui.Button("Rebuild").ID("rebuild").OnClick(update),
							ui.Button("Check").ID("check").OnClick(check),
							ui.Button("Reset").ID("reset").OnClick(reset),
						).Spacing(8),
						wui.Switch("Wi-Fi").ID("wifi").Checked(wifi).Enabled(enabled).Animated(animated).Visible(visible).OnChange(change(&wifi)),
						wui.Switch("Instant").ID("instant").Checked(instant).Animated(false).OnChange(change(&instant)),
						wui.Switch().ID("parent").Checked(parent).Animated(animated).OnChange(change(&parent)).Content(
							ui.HBox(ui.Label("Parent"), ui.Button("Child").ID("child").OnClick(func() { childClicks++; update() })).Spacing(12)),
						wui.Switch("Disabled").ID("disabled").Checked(true).Enabled(false).Animated(animated),
						ui.Label("Click / drag horizontally; Tab / Shift+Tab, Space / Enter; Esc cancels drag."),
					).Padding(20).Spacing(18).CrossAlign(layout.CrossStart),
				).CrossAlign(layout.CrossStretch).Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(check))),
		)
	}); err != nil {
		panic(err)
	}
}
