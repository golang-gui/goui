// ListRow 设置页窗口验收，不依赖 DevServer。
// 环境：桌面事件循环；记录 OS/WM、实际 Painter 和缩放。
// 独立 X11 会话没有输入法服务时，用 XMODIFIERS=@im=none 选择本地 XIM。
// 启动：go run ./tests/window/widgets/list_row [-chrome=native|integrated] [-basic] [-large]
// 后端：GOUI_PLAT_PAINTER=software/opengl（Windows 可 direct2d）；GOUI_PLAT_SCALE=1/2。
// 初始：720×760 DIP，亮色 Modern，自动更新关闭，整行开关关闭，下拉选择 Default。
// 操作和预期：
//  1. 独立 Switch 每点击一次只增加 changes 一次，不增加 actions；点所在行正文无效。
//  2. Action 行正文/尾部 Button 各有独立计数。整行 Switch 的正文切换一次，
//     其尾部 Switch 也只切换一次，不能因冒泡发生二次切换。
//  3. Tab/Shift+Tab 访问行与尾部控件，Space/Enter 激活 Action 行各一次；重复按键
//     不重复激活。输入框输入空格/Enter 不激活父行；下拉选择也不激活父行。
//  4. Rebuild 连续重建不替换行、Switch、DropDown 或 TextInput，不增加回调。
//     Slots 移除/恢复 Action 前后插槽，正文与行实例保持，移除内容不继续命中。
//  5. Disabled 行和子按钮不响应，Check 核对有效禁用、动作、文字和尾部语义。
//  6. 调到窄宽/使用 -large，说明换行增高，尾部控件不与文字重叠；滚动可访问下方。
//     Dark 在 Modern 下切换主题、不改变状态，Basic 保持固定外观；1x/2x 重复。
//     Check 输出 ASSERT PASS
//     与累计计数/选项/文本及只读 Snapshot，首帧自动 Check；断言失败 panic。
//     终端输出实际后端/缩放。快照仅用于观察，操作仍通过正常桌面输入。
//
// 平台差异：字体/装饰不同，边缘 AA 不比较金图；移动原生触摸需另行验收。
// 副作用：测试窗口和日志，不写文件，不操作系统设置/剪贴板或真正检查更新。
// 复位：重启。退出：关闭窗口，输出 DESTROY PASS 并退出；不后台续帧。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

func main() {
	chromeFlag := flag.String("chrome", "integrated", "native/integrated")
	basicFlag := flag.Bool("basic", false, "Basic theme")
	large := flag.Bool("large", false, "24pt text")
	flag.Parse()
	chrome := ui.WindowChromeIntegrated
	if *chromeFlag == "native" {
		chrome = ui.WindowChromeNative
	} else if *chromeFlag != "integrated" {
		panic("未知 chrome")
	}
	independent, checked, dark, slots := false, false, false, true
	changes, actions, buttons, selected := 0, 0, 0, 0
	text := ui.MakeState("Editable")
	options := gui.NewSliceListModel([]widgets.DropDownItem{{Text: "Default"}, {Text: "Alternative"}, {Text: "Disabled", Disabled: true}})
	retained := make(map[string]gui.Widget)
	attached := false
	err := ui.Run("org.golang-gui.ListRowTest", func(app ui.App) ui.RootView {
		font := float32(14)
		if *large {
			font = 24
		}
		rules := modern.Rules(modern.Options{Dark: dark, FontSize: font})
		if *basicFlag {
			rules = basic.Rules()
		}
		if *large {
			for _, name := range []string{"widget", "button", "label", "list-row-title", "list-row-subtitle", "drop-down-text"} {
				rules = append(rules, style.Name(name).FontSize(font))
			}
		}
		sheet := style.Sheet(rules...)
		check := func() {
			win := app.FindWindow("rows")
			for _, id := range []string{"independent", "action", "whole-switch", "dropdown-row", "input-row", "disabled", "switch", "whole-control", "dropdown", "input"} {
				w := gui.FindWidget(win, id)
				if w == nil || retained[id] != nil && retained[id] != w {
					panic("重建替换控件：" + id)
				}
				retained[id] = w
			}
			for _, id := range []string{"action", "whole-switch", "dropdown-row", "input-row"} {
				r := retained[id].(*widgets.ListRow)
				info := r.Snapshot()
				if info.Role != gui.RoleListItem || !r.Activatable() || !r.Focusable() || len(info.Actions) != 1 || info.Text == "" {
					panic("激活行快照错误：" + id)
				}
				end := float32(0)
				for _, child := range r.Children() {
					if !child.Visible() {
						continue
					}
					b := child.Rect()
					if b.X < end || b.X+b.Width > r.Rect().Width || b.Y < 0 || b.Y+b.Height > r.Rect().Height {
						panic("行内容重叠/越界：" + id)
					}
					end = b.X + b.Width
				}
			}
			plain := retained["independent"].Snapshot()
			if plain.Focusable || len(plain.Actions) != 0 || len(plain.Children) != 2 || plain.Children[1].Role != widgets.RoleSwitch {
				panic("独立设置行语义错误")
			}
			disabled := retained["disabled"].Snapshot()
			if disabled.Enabled || len(disabled.Actions) != 0 || disabled.Children[0].Enabled {
				panic("禁用行语义错误")
			}
			fmt.Printf("ASSERT PASS independent=%v checked=%v changes=%d actions=%d buttons=%d selected=%d text=%q slots=%v\n", independent, checked, changes, actions, buttons, selected, text.Get(), slots)
			snapshot, err := json.Marshal(win.Snapshot())
			if err != nil {
				panic(err)
			}
			fmt.Printf("SNAPSHOT %s\n", snapshot)
		}
		var prefix, suffix ui.View
		if slots {
			prefix = ui.Label("A")
			suffix = ui.Button("Child").ID("child-button").OnClick(func() { buttons++; app.RequestUpdate() })
		}
		content := ui.VBox(
			ui.HeaderBar(ui.Label("ListRow settings")),
			ui.HWrap(ui.Button("Check").ID("check").OnClick(check), ui.Button("Rebuild").ID("rebuild").OnClick(app.RequestUpdate), ui.Button("Slots").ID("slots").OnClick(func() { slots = !slots; app.RequestUpdate() }), ui.Button("Dark").ID("dark").OnClick(func() { dark = !dark; app.RequestUpdate() })).Spacing(8).Padding(12),
			ui.ScrollView(ui.VBox(
				wui.ListRow("Automatic updates").ID("independent").Subtitle("Notify when a new version is ready. This setting only changes memory.").Prefix(ui.Label("U")).Suffix(wui.Switch().ID("switch").Checked(independent).OnChange(func(v bool) { independent = v; changes++; app.RequestUpdate() })),
				wui.ListRow("Action row").ID("action").Subtitle("Click the row, or focus it and press Space / Enter.").Prefix(prefix).Suffix(suffix).Activatable(true).OnActivate(func() { actions++; app.RequestUpdate() }),
				wui.ListRow("Whole-row switch").ID("whole-switch").Subtitle("The row and control update one business value.").Suffix(wui.Switch().ID("whole-control").Checked(checked).OnChange(func(v bool) { checked = v; changes++; app.RequestUpdate() })).Activatable(true).OnActivate(func() { checked = !checked; changes++; app.RequestUpdate() }),
				wui.ListRow("Choice").ID("dropdown-row").Suffix(wui.DropDown(options).ID("dropdown").Selected(selected).MaxWidth(200).OnSelected(func(i int) { selected = i; app.RequestUpdate() }).OnOpenError(func(err error) { panic(err) })).Activatable(true).OnActivate(func() { actions++; app.RequestUpdate() }),
				wui.ListRow("Editable value").ID("input-row").Suffix(ui.TextInput().ID("input").BindText(&text).MaxWidth(220)).Activatable(true).OnActivate(func() { actions++; app.RequestUpdate() }),
				wui.ListRow("Unavailable").ID("disabled").Suffix(ui.Button("Disabled").OnClick(func() { panic("禁用子控件被激活") })).Activatable(true).Enabled(false).OnActivate(func() { panic("禁用行被激活") }),
				wui.ListRow("Application version").Suffix(ui.Label("0.7.0")),
			).Spacing(2).Padding(12)).Direction(ui.ScrollVertical).MainWeight(1),
		)
		if !attached {
			attached = true
			app.Post(func() {
				win := app.FindWindow("rows")
				win.Chrome().ConnectInfo(func(info gui.ChromeInfo) {
					if info.Mode != chrome {
						panic("装饰回退，不能计为请求模式通过")
					}
				})
				once := false
				win.ConnectAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
					if once || win.Widget() == nil || win.Widget().Rect().Width <= 0 {
						return
					}
					once = true
					img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
					if err != nil {
						panic(err)
					}
					fmt.Printf("BACKEND %T scale=%g\n", img, p.PixelScale())
					img.Destroy()
					check()
				})
			})
		}
		return ui.Root().StyleSheet(sheet).Windows(ui.Window("rows").Title("GOUI ListRow").Size(720, 760).Chrome(chrome).OnDestroy(func() { fmt.Println("DESTROY PASS"); app.Quit() }).Content(content))
	})
	if err != nil {
		panic(err)
	}
}
