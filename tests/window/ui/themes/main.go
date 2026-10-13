// 默认 Basic 与可选 Modern 样式窗口验收，不依赖 DevServer。
// 环境：Linux X11、Windows 或 macOS 的可交互桌面；记录 OS/WM、后端和缩放。
// 启动：go run ./tests/window/ui/themes；GOUI_PLAT_PAINTER=software/opengl，
// Windows 另可 direct2d；GOUI_PLAT_SCALE=2 请求应用 2x。
// 初始：Integrated 640×460 DIP、未设置自定义样式表；HeaderBar 可用，
// CheckButton 和 Switch 均选中，禁用的两个控件不可操作。
//
// 操作与逐项预期：
//  1. 默认 Basic 下方框、勾、开关轨道/滑块及各自文字均可见。点击任一控件，
//     两者同步切换，HeaderBar 同步启用/禁用；禁用示例没有状态变化。
//     Basic 选中方框和开启开关不叠加同色描边；开关蓝色稍柔和，未选中保留中性轮廓。
//  2. 点击“显式 Basic”，外观与默认 Basic 一致；点击“系统 Modern”，外观
//     采用系统亮暗/主题色/字体。点击“深色 Modern”，背景和文字适配深色。
//     切换期间选中状态、计数和 Widget 身份不变，HeaderBar 拖动语义不变。
//  3. 点击“默认 Basic”恢复 nil 样式表，不需手动追加 widgets/style；
//     字体/颜色恢复 Basic，控件不会消失。系统 Modern 模式下改变系统亮暗
//     或主题色，已有 Settings 自动重建应用；其他模式保持显式外观。
//  4. Tab/Shift+Tab 导航，Space 切换选中项；只在键盘焦点显示 2 DIP 边框。
//     默认禁用项不可聚焦；鼠标点击不额外显示键盘焦点边框。
//  5. F6 或“检查”输出 ASSERT PASS 和 BOUNDS；检查选中/禁用、Snapshot、
//     控件身份、指示器区域及内部标签非空。此断言不替代实际像素观察。
//     1x/2x 重复操作，圆角、文字和滑块无裁剪；关闭后正常退出。
//
// 平台差异：字体/抗锯齿和实际窗口装饰可不同；Windows/macOS 需分别验收。
// 副作用：仅窗口和标准输出；系统设置仅在第 3 步由验证者自愿改变并负责还原。
// 复位：点击“复位”；恢复模式也可重启。退出：关闭主窗口。
package main

import (
	"fmt"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

func main() {
	mode, changes := 0, 0
	enabled := ui.MakeState(true)
	previous := make(map[string]gui.Widget)
	modeNames := []string{"默认 Basic", "显式 Basic", "系统 Modern", "深色 Modern"}
	if err := ui.Run("org.golang-gui.ThemeTest", func(app ui.App) ui.RootView {
		var sheet style.StyleSheet
		switch mode {
		case 1:
			sheet = basic.Sheet()
		case 2:
			sheet = modern.FromSettings(app.Settings())
		case 3:
			s := app.Settings()
			sheet = modern.Sheet(modern.Options{Dark: true, AccentColor: s.AccentColor(), FontFamily: s.FontFamily(), FontSize: s.FontSize()})
		}
		change := func(value bool) { changes++; enabled.Set(value) }
		check := func() {
			win := app.FindWindow("themes")
			if (gui.App.StyleSheet() == nil) != (mode == 0) {
				panic("explicit stylesheet/nil contract changed")
			}
			for _, id := range []string{"header", "check", "switch", "disabled-check", "disabled-switch"} {
				w := gui.FindWidget(win, id)
				if w == nil || w.Rect().Width <= 0 || w.Rect().Height <= 0 || previous[id] != nil && previous[id] != w {
					panic("missing geometry or replaced widget: " + id)
				}
				previous[id] = w
				info := w.Snapshot()
				if info.Enabled != (id != "disabled-check" && id != "disabled-switch" && (id != "header" || enabled.Get())) {
					panic("enabled snapshot mismatch: " + id)
				}
				switch w := w.(type) {
				case *widgets.CheckButton:
					want := enabled.Get() || id == "disabled-check"
					if w.Checked() != want || info.Attributes[widgets.CheckInfoKey].(widgets.CheckInfo).State != w.CheckState() {
						panic("checkbox state/snapshot mismatch")
					}
				case *widgets.Switch:
					want := enabled.Get() || id == "disabled-switch"
					if w.Checked() != want || info.Attributes[widgets.SwitchInfoKey].(widgets.SwitchInfo).Checked != want {
						panic("switch state/snapshot mismatch")
					}
				}
				if len(info.Children) != 1 || info.Children[0].Text == "" || info.Children[0].Bounds.Width <= 0 || info.Children[0].Bounds.Height <= 0 {
					panic("independent content label missing: " + id)
				}
			}
			for _, id := range []string{"mode-0", "mode-1", "mode-2", "mode-3", "check", "switch", "verify", "reset"} {
				info := gui.FindWidget(win, id).Snapshot()
				fmt.Printf("BOUNDS %s %.2f %.2f %.2f %.2f focused=%v focusVisible=%v\n", id, info.Bounds.X, info.Bounds.Y, info.Bounds.Width, info.Bounds.Height, info.Focused, info.FocusVisible)
			}
			fmt.Printf("ASSERT PASS mode=%d enabled=%v changes=%d retained=true\n", mode, enabled.Get(), changes)
		}
		modes := make([]ui.View, len(modeNames))
		for i, name := range modeNames {
			modes[i] = ui.Button(name).ID(fmt.Sprintf("mode-%d", i)).OnClick(func() { mode = i; app.RequestUpdate() })
		}
		return ui.Root().StyleSheet(sheet).Windows(
			ui.Window("themes").Title("GOUI Basic / Modern").Chrome(ui.WindowChromeIntegrated).Size(640, 460).
				Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(check)).
				Content(ui.VBox(
					ui.HeaderBar(ui.Label("HeaderBar")).ID("header").Enabled(enabled.Get()),
					ui.VBox(
						ui.HWrap(modes...).Spacing(8).LineSpacing(8),
						ui.Label(fmt.Sprintf("模式：%s；启用：%v；变化次数：%d", modeNames[mode], enabled.Get(), changes)),
						wui.CheckButton("HeaderBar CheckButton").ID("check").Checked(enabled.Get()).OnChange(func(s wui.CheckState) { change(s != wui.CheckUnchecked) }),
						wui.Switch("HeaderBar Switch").ID("switch").Checked(enabled.Get()).OnChange(change),
						wui.CheckButton("Disabled CheckButton").ID("disabled-check").Checked(true).Enabled(false),
						wui.Switch("Disabled Switch").ID("disabled-switch").Checked(true).Enabled(false),
						ui.HBox(
							ui.Button("检查 / F6").ID("verify").OnClick(check),
							ui.Button("复位").ID("reset").OnClick(func() { mode, changes = 0, 0; enabled.Set(true) }),
						).Spacing(8),
					).Padding(16).Spacing(12),
				).CrossAlign(layout.CrossStretch)),
		)
	}); err != nil {
		panic(err)
	}
}
