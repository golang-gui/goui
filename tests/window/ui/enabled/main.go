// Widget 禁用、父子状态、焦点、文字输入与声明式恢复验收；不依赖 DevServer。
// 环境：Linux X11、Windows、macOS 可交互桌面。记录 OS/WM、后端、缩放。
// 启动：go run ./tests/window/ui/enabled；GOUI_PLAT_PAINTER 可选 software、
// opengl（Windows 另有 direct2d），GOUI_PLAT_SCALE=2 验证 2x。
// 初始：Integrated 860×700 DIP、亮色；整组启用，普通/主按钮与输入框启用，
// 固定禁用按钮不可点击，文字编辑保留初值，ReadOnly 字段可选择复制。
// 操作与预期：
//  1. 点击普通/主按钮、勾选/切换、下拉选择、编辑输入框，计数和文本更新。
//  2. F1（或 Toggle group）禁用整组：保持布局和内容；文字/图标弱化；所有
//     交互、Tab 焦点和右键编辑菜单失效。进度动画继续，外部工具栏可操作。
//  3. F2/F3 分别改变普通按钮/输入框自身设置，即使整组禁用也保存设置；
//     F1 恢复时按各自设置恢复，固定禁用按钮始终禁用。再按 F2/F3 恢复。
//  4. 输入框获得焦点、输入文字（含中文预编辑）后按 F1：焦点和光标清除，
//     未提交预编辑取消、已提交文本保留；F1 恢复不会自动抢回焦点。
//  5. 鼠标按住普通按钮时按 F1、F1 再松手，计数不增加；重新点击有效。
//     指针停在普通按钮上按 F2 禁用再恢复：悬停外观随状态恢复；指针离开后
//     再切换不能凭空出现悬停。按 F7 后在两秒内展开下拉，等待整组禁用：
//     既有失焦可能关闭弹层；若仍展开，下次鼠标或键盘操作应关闭弹层且
//     不能提交选择。弹层模态期间快捷键由弹层接收，不能用 F1 触发此场景。
//  6. Tab/Shift+Tab 跳过禁用项；ReadOnly 可取得焦点并复制，不能编辑。
//  7. F4（或 Dark）切换亮暗主题，禁用色中性且可读，尺寸不跳动。F5 只移除
//     普通按钮 Enabled 声明，恢复构造初值，但仍受整组限制。
//  8. F6（或 Check）检查真实 Widget 状态、子树/内容、动作和 Focusable 配置；
//     输出 ASSERT PASS 与窗口局部 DIP 边界。按 F1/F2/F3/F5 后
//     再检查，验证多次重建。窗口级 F1～F6 在整组禁用时仍生效。
//
// 副作用：剪贴板复制仅按用户操作；不写文件。复位：重启例程。
// 退出：Escape、Quit 或原生关闭按钮；正常返回输出 EXIT PASS。
// 原生关闭触发声明式 OnDestroy 时另输出 DESTROY PASS；App.Quit 不要求此信号。
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"time"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

type squareIcon struct{}

func (squareIcon) Image(width, height int, foreground gui.Color) image.Image {
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	c := color.RGBA{R: uint8(foreground.R * 255), G: uint8(foreground.G * 255), B: uint8(foreground.B * 255), A: uint8(foreground.A * 255)}
	draw.Draw(result, result.Bounds().Inset(max(1, width/6)), image.NewUniform(c), image.Point{}, draw.Src)
	return result
}

func main() {
	groupEnabled, buttonEnabled, inputEnabled := true, true, true
	buttonDeclared, dark, connected := true, false, false
	clicks, changes, submits := 0, 0, 0
	text := "Editable text / 中文"
	checked, switched, selected := false, true, 0
	model := gui.NewSliceListModel([]widgets.DropDownItem{{Text: "First"}, {Text: "Second"}, {Text: "Unavailable", Disabled: true}})
	var previous []gui.Widget
	var verify func()
	err := ui.Run("org.golang-gui.EnabledTest", func(app ui.App) ui.RootView {
		update := app.RequestUpdate
		toggleGroup := func() { groupEnabled = !groupEnabled; update() }
		toggleButton := func() { buttonDeclared = true; buttonEnabled = !buttonEnabled; update() }
		toggleInput := func() { inputEnabled = !inputEnabled; update() }
		toggleDark := func() { dark = !dark; update() }
		removeButton := func() { buttonDeclared = false; update() }
		disableLater := func() {
			app.TimeoutFunc(2*time.Second, func() { groupEnabled = false; update() })
		}
		wantGroup, wantButton, wantInput, wantDeclared := groupEnabled, buttonEnabled, inputEnabled, buttonDeclared
		verify = func() {
			win := app.FindWindow("enabled")
			if win == nil {
				panic("missing window")
			}
			ids := []string{"group", "button", "primary", "input", "check", "switch", "choice", "always-disabled", "readonly", "icon"}
			for i, id := range ids {
				w := gui.FindWidget(win, id)
				if w == nil || len(previous) != 0 && previous[i] != w {
					panic("identity: " + id)
				}
				own := true
				switch id {
				case "group":
					own = wantGroup
				case "button":
					own = !wantDeclared || wantButton
				case "input":
					own = wantInput
				case "always-disabled":
					own = false
				}
				effective := own && (id == "readonly" || wantGroup)
				if id == "icon" {
					effective = effective && (!wantDeclared || wantButton)
				}
				info := w.Snapshot()
				if w.Enabled() != own || gui.IsEnabled(w) != effective || info.Enabled != effective ||
					!effective && (info.Focused || len(info.Actions) != 0) {
					panic("enable state/actions: " + id)
				}
				if id != "group" && id != "icon" && !w.Focusable() {
					panic("lost focus configuration: " + id)
				}
				fmt.Printf("BOUNDS %s %.1f %.1f %.1f %.1f own=%v effective=%v focus=%v\n", id,
					info.Bounds.X, info.Bounds.Y, info.Bounds.Width, info.Bounds.Height, own, effective, info.Focused)
			}
			input := gui.FindWidget(win, "input").(*gui.TextInput)
			// Editing changes the Widget before a queued declarative rebuild.
			// Compare with synchronous OnText writeback, not an old declaration.
			if input.Text() != text {
				panic(fmt.Sprintf("content: got %q, want %q", input.Text(), text))
			}
			previous = previous[:0]
			for _, id := range ids {
				previous = append(previous, gui.FindWidget(win, id))
			}
			fmt.Printf("ASSERT PASS clicks=%d changes=%d submits=%d text=%q\n", clicks, changes, submits, text)
		}
		// 快速输入可能先改变下一次声明值，再运行上一轮排队的检查。
		// 读取最新完成构建对应的预期，不拿待协调状态比较旧控件树。
		check := func() { app.Post(func() { verify() }) }
		if !connected {
			connected = true
			check()
		}
		button := ui.Button().ID("button").Child(ui.HBox(ui.Icon(squareIcon{}).ID("icon").Style(modern.AccentIcon), ui.Label("Normal button")).Spacing(6)).
			OnClick(func() { clicks++; fmt.Printf("CLICK %d\n", clicks); update() })
		if buttonDeclared {
			button.Enabled(buttonEnabled)
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{Dark: dark})).Windows(
			ui.Window("enabled").Title("GOUI Enabled").Size(860, 700).Chrome(ui.WindowChromeIntegrated).
				OnDestroy(func() { fmt.Println("DESTROY PASS"); app.Quit() }).
				Shortcuts(
					ui.Shortcut(ui.KeyF1).OnActivate(toggleGroup), ui.Shortcut(ui.KeyF2).OnActivate(toggleButton),
					ui.Shortcut(ui.KeyF3).OnActivate(toggleInput), ui.Shortcut(ui.KeyF4).OnActivate(toggleDark),
					ui.Shortcut(ui.KeyF5).OnActivate(removeButton), ui.Shortcut(ui.KeyF6).OnActivate(check),
					ui.Shortcut(ui.KeyF7).OnActivate(disableLater),
					ui.Shortcut(ui.KeyEscape).OnActivate(app.Quit),
				).Content(ui.VBox(
				ui.HeaderBar(ui.Label("Widget Enabled · subtree / focus / style")),
				ui.VBox(
					ui.HBox(ui.Button("Toggle group").OnClick(toggleGroup), ui.Button("Toggle button").OnClick(toggleButton),
						ui.Button("Toggle input").OnClick(toggleInput), ui.Button("Dark").OnClick(toggleDark),
						ui.Button("Check").OnClick(check), ui.Button("Quit").OnClick(app.Quit)).Spacing(8),
					ui.Label(fmt.Sprintf("group=%v | button=%v declared=%v | input=%v | clicks=%d changes=%d submits=%d", groupEnabled, buttonEnabled, buttonDeclared, inputEnabled, clicks, changes, submits)),
					ui.VBox(
						ui.HBox(button, ui.Button("Primary").ID("primary").Style(modern.Primary).OnClick(func() { clicks++; update() }),
							ui.Button("Always disabled").ID("always-disabled").Enabled(false).OnClick(func() { panic("disabled click") })).Spacing(12),
						ui.TextInput().ID("input").Text(text).Enabled(inputEnabled).MinWidth(420).
							OnText(func(value string) { text = value; changes++; update() }).OnSubmit(func() { submits++; update() }),
						wui.CheckButton("CheckButton").ID("check").Checked(checked).OnChange(func(s wui.CheckState) { checked = s == wui.CheckChecked; changes++; update() }),
						wui.Switch("Switch").ID("switch").Checked(switched).OnChange(func(value bool) { switched = value; changes++; update() }),
						wui.DropDown(model).ID("choice").Selected(selected).MinWidth(260).OnSelected(func(index int) { selected = index; changes++; update() }).OnOpenError(func(err error) { panic(err) }),
						wui.ProgressBar(0).Indeterminate(true).MinWidth(300),
					).ID("group").Enabled(groupEnabled).Spacing(18).CrossAlign(layout.CrossStart),
					ui.Label("ReadOnly stays interactive outside the disabled group:"),
					ui.TextInput().ID("readonly").Text("Select and copy; editing is read-only").ReadOnly(true).MinWidth(420),
					ui.Label("F1 group · F2 button · F3 input · F4 dark · F5 reset button · F6 check · F7 disable in 2s · Esc quit"),
				).Padding(20).Spacing(18).CrossAlign(layout.CrossStart),
			).CrossAlign(layout.CrossStretch)),
		)
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("EXIT PASS")
}
