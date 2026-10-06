// CheckButton 三态、互斥组、按钮式外观与声明协调窗口验收，不依赖 DevServer。
// 环境：Linux X11、Windows 或 macOS 可交互桌面，记录 OS/WM、后端、缩放。
// 启动：go run ./tests/window/widgets/check_button；GOUI_PLAT_PAINTER=software/opengl，
// Windows 另有 direct2d；GOUI_PLAT_SCALE=2 请求 2x；-fallback-style 使用显式兜底。
// 另加 -destroy-on-change：首次选择 Radio B/C 时旧成员回调销毁窗口，验证销毁安全。
// 初始：Integrated 900×640 DIP，亮色蓝主题，Independent 未选中、Mixed 部分选中，
// Radio A、Mode A 选中，Toggle 未选中，其左边是同文字的普通 Button 对照；
// Disabled 无输入。实际装饰可能回退 Native。
// Modern 使用 Settings 提供的界面字体和字号；显式兜底保留其固定默认字体。
//
// 操作与逐项预期：
//  1. 点击 Independent 的方框和文字均能切换；按下未抬起不切换，移出后抬起
//     不切换。Mixed 点击变勾选；Mixed 按钮静默恢复部分选中，不增加 Changes。
//  2. 选择 Radio B/C，原成员先取消，新成员选中；重复点击已选成员不取消。
//     Clear 清空两组，后续仍可选择。Toggle 独立切换；Mode A/B 是互斥按钮。
//  3. Tab/Shift+Tab 导航；Space/Enter 切换，长按只切换一次。只有键盘焦点有
//     2 DIP 柔和边框，鼠标点击没有焦点装饰。Disabled 不响应，也不参与 Tab。
//     Disable 禁用 Independent，启用后恢复操作与焦点参与。
//  4. 点击 Child 只增加 ChildClicks，不切换 Parent；点击 Parent 文字切换父项。
//     Rebuild 保留控件身份和选择；后续一次操作只产生一次对应 Change。
//  5. Dark/Accent 改变亮暗及蓝/红主题，选中状态不变，内容文字独立可读。
//     调整窗口及 1x/2x 重复，圆形不变椭圆，勾/横杠与边框清晰，无越界。
//     普通 Button 与未选中 Toggle 的大小、底色、边框、圆角一致；选中 Toggle
//     仅变中性灰底，Accent 切换不改变其选中底色。指示器 18 DIP；方框 4 DIP 圆角，选中为实色底
//     与白色勾/横杠；单选为主题色圆环、圆点，两者之间保留中性底色。
//     悬停/按下只改变底色，不突然出现主题色边框，也不使内容或大小跳动。
//  6. Check 或 F6 输出 ASSERT PASS 并核对所有状态、分组、Snapshot、身份；BOUNDS
//     是已布局的窗口局部 DIP，可供人工/Computer Use 找位置，不是动作接口。
//     Check 不替代视觉与手势观察。关闭窗口后 DESTROY PASS，两个组无选中者。
//  7. -destroy-on-change 独立启动，先 F6 再点击 Radio B。只发生旧成员的一个 Change，
//     新成员及后续回调不访问已销毁树；输出 DESTROY PASS 并正常退出，无 panic。
//
// 平台差异：原生装饰/字体/抗锯齿可不同；基础选中语义一致；组内方向键未实现。
// 副作用：仅窗口与标准输出；不修改系统设置/文件/剪贴板。
// 复位：Reset 恢复初始选择与计数；重启恢复主题和全部状态。退出：关闭主窗口。
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

func main() {
	fallback := flag.Bool("fallback-style", false, "使用显式组合的 widgets 兜底样式")
	destroyOnChange := flag.Bool("destroy-on-change", false, "旧选中成员的 Change 回调销毁窗口")
	flag.Parse()
	radioGroup, modeGroup := widgets.NewCheckGroup(), widgets.NewCheckGroup()
	independent, mixed, parent := wui.CheckUnchecked, wui.CheckMixed, wui.CheckUnchecked
	radio, mode := 1, 1
	toggle, enabled, dark, red := false, true, false, false
	changes, childClicks, ordinaryClicks := 0, 0, 0
	lateChanges := 0
	previous := make(map[string]gui.Widget)
	if err := ui.Run("org.golang-gui.CheckButtonTest", func(app ui.App) ui.RootView {
		gui.App.SetQuitOnLastWindowClosed(false)
		update := func() { app.RequestUpdate() }
		reset := func() {
			independent, mixed, parent = wui.CheckUnchecked, wui.CheckMixed, wui.CheckUnchecked
			radio, mode, toggle, enabled, changes, childClicks = 1, 1, false, true, 0, 0
			ordinaryClicks = 0
			update()
		}
		change := func(value *wui.CheckState) func(wui.CheckState) {
			return func(s wui.CheckState) { *value = s; changes++; update() }
		}
		choice := func(value *int, index int) func(wui.CheckState) {
			return func(s wui.CheckState) {
				if s == wui.CheckChecked {
					*value = index
				}
				changes++
				if *destroyOnChange && value == &radio && index == 1 && s == wui.CheckUnchecked {
					app.FindWindow("checks").Destroy()
					return
				}
				update()
			}
		}
		check := func() {
			win := app.FindWindow("checks")
			settings := app.Settings()
			fmt.Printf("FONT family=%q size=%g\n", settings.FontFamily(), settings.FontSize())
			want := map[string]wui.CheckState{"independent": independent, "mixed": mixed, "parent": parent, "disabled": wui.CheckChecked}
			want["toggle"] = wui.CheckUnchecked
			if toggle {
				want["toggle"] = wui.CheckChecked
			}
			for i := 1; i <= 3; i++ {
				want[fmt.Sprintf("radio-%d", i)] = wui.CheckUnchecked
				if radio == i {
					want[fmt.Sprintf("radio-%d", i)] = wui.CheckChecked
				}
			}
			for i := 1; i <= 2; i++ {
				want[fmt.Sprintf("mode-%d", i)] = wui.CheckUnchecked
				if mode == i {
					want[fmt.Sprintf("mode-%d", i)] = wui.CheckChecked
				}
			}
			for id, state := range want {
				b, ok := gui.FindWidget(win, id).(*widgets.CheckButton)
				if !ok || b.CheckState() != state || previous[id] != nil && previous[id] != b {
					panic("state/identity incorrect: " + id)
				}
				if b.Padding() != 6 {
					panic("default padding incorrect: " + id)
				}
				if *destroyOnChange && previous[id] == nil && (id == "radio-1" || id == "radio-2") {
					// Raw GUI connections are deliberately not owned by UI teardown:
					// safety must come from the transition's lifetime check.
					b.ConnectChange(func(widgets.CheckState) { lateChanges++ })
				}
				previous[id] = b
				info := b.Snapshot()
				data := info.Attributes[widgets.CheckInfoKey].(widgets.CheckInfo)
				if info.Focused {
					fmt.Printf("FOCUS %s visible=%v\n", id, info.FocusVisible)
				}
				if data.State != state || len(info.Children) != 1 {
					panic("snapshot incorrect: " + id)
				}
				if id == "disabled" && (b.Enabled() || b.Focusable()) {
					panic("disabled is focusable")
				}
				if id == "independent" && b.Enabled() != enabled {
					panic("enabled mismatch")
				}
				if id[:min(len(id), 6)] == "radio-" && (b.Group() != radioGroup || info.Role != widgets.RoleRadioButton) {
					panic("radio group/role mismatch")
				}
				if id == "toggle" && info.Role != widgets.RoleToggleButton {
					panic("toggle role mismatch")
				}
			}
			if (radioGroup.Checked() == nil) != (radio == 0) || (modeGroup.Checked() == nil) != (mode == 0) {
				panic("group selection mismatch")
			}
			if gui.FindWidget(win, "ordinary").Rect().Size != gui.FindWidget(win, "toggle").Rect().Size {
				panic("ordinary Button and Toggle text/padding allocation differs")
			}
			for _, id := range []string{"independent", "mixed", "radio-1", "radio-2", "radio-3", "ordinary", "toggle", "mode-1", "mode-2", "parent", "child", "disabled", "mixed-reset", "clear", "disable", "dark", "accent", "rebuild", "check", "reset"} {
				b := gui.FindWidget(win, id).Snapshot().Bounds
				fmt.Printf("BOUNDS %s %.2f %.2f %.2f %.2f\n", id, b.X, b.Y, b.Width, b.Height)
			}
			fmt.Printf("ASSERT PASS independent=%d mixed=%d radio=%d toggle=%v mode=%d parent=%d changes=%d childClicks=%d ordinaryClicks=%d enabled=%v dark=%v red=%v retained=true\n", independent, mixed, radio, toggle, mode, parent, changes, childClicks, ordinaryClicks, enabled, dark, red)
		}
		accent := color.RGBA{R: 70, G: 130, B: 220, A: 255}
		if red {
			accent = color.RGBA{R: 210, G: 48, B: 48, A: 255}
		}
		settings := app.Settings()
		sheet := modern.Sheet(modern.Options{
			Dark: dark, AccentColor: accent,
			FontFamily: settings.FontFamily(), FontSize: settings.FontSize(),
		})
		if *fallback {
			sheet = style.Sheet(append(gui.DefaultStyleRules(), widgetstyle.Rules()...)...)
		}
		return ui.Root().StyleSheet(sheet).Windows(
			ui.Window("checks").Title("GOUI CheckButton").Size(900, 640).Chrome(ui.WindowChromeIntegrated).
				OnDestroy(func() {
					app.Post(func() {
						if radioGroup.Checked() != nil || modeGroup.Checked() != nil {
							panic("destroyed selection retained")
						}
						if *destroyOnChange && (changes != 1 || lateChanges != 0) {
							panic("destroyed transition continued notification")
						}
						fmt.Println("DESTROY PASS")
						app.Quit()
					})
				}).Content(
				ui.VBox(
					ui.HeaderBar(ui.Label("CheckButton")),
					ui.VBox(
						ui.Label(fmt.Sprintf("Changes %d | ChildClicks %d | ButtonClicks %d | Dark %v | Accent red %v", changes, childClicks, ordinaryClicks, dark, red)),
						ui.HBox(
							ui.Button("Mixed").ID("mixed-reset").OnClick(func() { mixed = wui.CheckMixed; update() }),
							ui.Button("Clear").ID("clear").OnClick(func() { radio, mode = 0, 0; update() }),
							ui.Button("Disable").ID("disable").OnClick(func() { enabled = !enabled; update() }),
							ui.Button("Dark").ID("dark").OnClick(func() { dark = !dark; update() }),
							ui.Button("Accent").ID("accent").OnClick(func() { red = !red; update() }),
							ui.Button("Rebuild").ID("rebuild").OnClick(update),
							ui.Button("Check").ID("check").OnClick(check),
							ui.Button("Reset").ID("reset").OnClick(reset),
						).Spacing(8),
						wui.CheckButton("Independent").ID("independent").CheckState(independent).Enabled(enabled).OnChange(change(&independent)),
						wui.CheckButton("Mixed").ID("mixed").CheckState(mixed).OnChange(change(&mixed)),
						ui.HBox(
							wui.CheckButton("Radio A").ID("radio-1").Group(radioGroup).Checked(radio == 1).OnChange(choice(&radio, 1)),
							wui.CheckButton("Radio B").ID("radio-2").Group(radioGroup).Checked(radio == 2).OnChange(choice(&radio, 2)),
							wui.CheckButton("Radio C").ID("radio-3").Group(radioGroup).Checked(radio == 3).OnChange(choice(&radio, 3)),
						).Spacing(12),
						ui.HBox(
							ui.Button("Toggle").ID("ordinary").OnClick(func() { ordinaryClicks++; update() }),
							wui.CheckButton("Toggle").ID("toggle").Appearance(wui.CheckAppearanceButton).Checked(toggle).OnChange(func(s wui.CheckState) { toggle = s == wui.CheckChecked; changes++; update() }),
							wui.CheckButton("Mode A").ID("mode-1").Appearance(wui.CheckAppearanceButton).Group(modeGroup).Checked(mode == 1).OnChange(choice(&mode, 1)),
							wui.CheckButton("Mode B").ID("mode-2").Appearance(wui.CheckAppearanceButton).Group(modeGroup).Checked(mode == 2).OnChange(choice(&mode, 2)),
						).Spacing(12),
						wui.CheckButton().ID("parent").CheckState(parent).OnChange(change(&parent)).Content(
							ui.HBox(ui.Label("Parent"), ui.Button("Child").ID("child").OnClick(func() { childClicks++; update() })).Spacing(12),
						),
						wui.CheckButton("Disabled").ID("disabled").Checked(true).Enabled(false),
						ui.Label("Tab / Shift+Tab, Space / Enter; click Child without toggling Parent."),
					).Padding(20).Spacing(18).CrossAlign(layout.CrossStart),
				).CrossAlign(layout.CrossStretch).Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(check)),
			),
		)
	}); err != nil {
		panic(err)
	}
}
