// DropDown 原生浮窗、真实内容、键鼠选择和声明式协调验收，不依赖 DevServer。
// 环境：Linux X11、Windows、macOS 可交互桌面，记录 OS/WM、后端和缩放。
// 启动：go run ./tests/window/widgets/drop_down；-gui 使用命令式分支，
// -fallback-style 使用 widgets 显式兜底，-destroy-on-selected 验证信号内销毁。
// GOUI_PLAT_PAINTER=software/opengl（Windows 另有 direct2d），GOUI_PLAT_SCALE=2 为 2x。
// 初始：Integrated 850×640 DIP、亮色蓝主题；Basic 未选、Custom 选 0、Long 选 9998、
// Uncontrolled 未选（-gui 初始选 0），Disabled 禁用且选 0，Empty 无选项；选项 1 禁用。
//
// 操作与逐项预期：
//  1. 点击 Basic 后显示独立圆角阴影列表，宽度等于控件、内容不进入圆角外。
//     鼠标按下并移出后释放不打开；Disabled 和 Empty 不打开。点击选项 1 不确认。
//     选择 C 后只增加一次 Selected 计数，收起显示 C；再次确认 C 不增加计数。
//  2. Tab/Shift+Tab 导航；Enter/Space/Alt+Down 打开。Down 跳过禁用项 1，
//     浏览时收起值不变。Enter 确认，Esc 或外部点击取消。打开时 Tab 关闭并
//     将焦点移到下一个可用控件，Shift+Tab 移到前一个；关闭时滚轮不改选择。
//  3. Custom 的弹出行显示“● 名称 + 说明”，UI 收起只显示名称（-gui 沿用行展示）；都是实际控件，
//     文字布局清晰，不把同一 Widget 在两个位置移动。可通过包内测试检查实例隔离。
//     1x/2x 四行均完整可见、没有滚动条；关闭后 Shrink 再打开应只有一行，
//     关闭后 Reset 再打开恢复四行且仍无滚动条，覆盖原生 Popup 创建与重新调整尺寸。
//  4. Long 只显示有限行、滚动条可拖动；打开后自动显示已选的 9998。
//     左边缘到选项与选项到滚动条的留白一致（默认 6 DIP）；高亮和勾选不进入
//     留白区，点击选项与滚动条之间的空隙不提交选择或关闭列表。
//     Home/End 到首尾、上下浏览后确认，不创建一万行控件或卡住；高度不超过
//     320 DIP，宽度不随滚动跳变。移动主窗口再打开，浮窗跟随当前控件位置。
//     将窗口移到屏幕下缘再打开，列表退避到上方或工作区内；1x/2x 均不越界。
//  5. Set 静默设置 Basic，不增加计数。Rebuild 保留各 Widget 和模型实例、选择及
//     单个信号；Uncontrolled 不声明 Selected，用户选择在重建后保持。Shrink
//     将 Basic 模型缩短到一项，其越界选择静默清空；Reset 恢复所有数据和计数。
//  6. Dark/Accent 切换亮暗及蓝红；选项背景为中性灰、不随主题色改变；文字、
//     箭头、勾选与阴影可辨。键盘焦点为 2 DIP 柔和边框，鼠标焦点无额外边框。
//     Check/F6 输出 ASSERT PASS 和窗口局部 DIP BOUNDS；Popup 是独立 Root。
//  7. -destroy-on-selected：用户确认一次后立即关闭窗口，后续 Selected 回调不执行，
//     输出 DESTROY PASS，无悬挂浮窗或 panic。常规关闭同样输出 DESTROY PASS。
//
// 平台差异：字体/抗锯齿及实际装饰可能不同，Integrated 可回退 Native。
// 副作用：仅窗口和日志，不修改剪贴板、文件、系统设置。复位：Reset 或重启。
// 退出：关闭主窗口。所有可计算的状态/身份/快照异常 panic；视觉需逐项观察。
package main

import (
	"flag"
	"fmt"
	"image/color"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	widgetstyle "github.com/golang-gui/goui/widgets/style"
	wui "github.com/golang-gui/goui/widgets/ui"
)

var options = []widgets.DropDownItem{{Text: "A · Follow system"}, {Text: "B · Disabled", Disabled: true}, {Text: "C · Light"}, {Text: "D · Dark"}}

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	imperative := flag.Bool("gui", false, "命令式分支")
	fallback := flag.Bool("fallback-style", false, "显式兜底")
	destroy := flag.Bool("destroy-on-selected", false, "首次用户选择后销毁")
	flag.Parse()
	if *imperative {
		runGUI(*fallback, *destroy)
	} else {
		runUI(*fallback, *destroy)
	}
}

func sheet(dark, red, fallback bool, family string, size float32) style.StyleSheet {
	if fallback {
		return style.Sheet(append(gui.DefaultStyleRules(), widgetstyle.Rules()...)...)
	}
	accent := color.RGBA{R: 70, G: 130, B: 220, A: 255}
	if red {
		accent = color.RGBA{R: 210, G: 48, B: 48, A: 255}
	}
	return modern.Sheet(modern.Options{Dark: dark, AccentColor: accent, FontFamily: family, FontSize: size})
}

func longModel() *gui.SliceListModel[widgets.DropDownItem] {
	items := make([]widgets.DropDownItem, 10000)
	for i := range items {
		items[i].Text = fmt.Sprintf("Option %05d", i)
	}
	return gui.NewSliceListModel(items)
}

func check(win gui.Window, previous map[string]*widgets.DropDown, want map[string]int, changes, late int) {
	for _, id := range []string{"basic", "custom", "long", "uncontrolled", "disabled", "empty"} {
		d, ok := gui.FindWidget(win, id).(*widgets.DropDown)
		if !ok || previous[id] != nil && previous[id] != d {
			panic("widget identity: " + id)
		}
		previous[id] = d
		if value, controlled := want[id]; controlled && d.Selected() != value {
			panic(fmt.Sprintf("%s selected %d, want %d", id, d.Selected(), value))
		}
		info := d.Snapshot()
		attr := info.Attributes[widgets.DropDownInfoKey].(widgets.DropDownInfo)
		if info.Role != widgets.RoleComboBox || attr.Index != d.Selected() || attr.Expanded != d.Opened() || info.Enabled != (id != "disabled") {
			panic("snapshot: " + id)
		}
		fmt.Printf("BOUNDS %s %.2f %.2f %.2f %.2f SELECTED %d OPEN %v\n", id, info.Bounds.X, info.Bounds.Y, info.Bounds.Width, info.Bounds.Height, d.Selected(), d.Opened())
		if info.Focused {
			fmt.Printf("FOCUS %s visible=%v\n", id, info.FocusVisible)
		}
	}
	fmt.Printf("ASSERT PASS changes=%d late=%d retained=true\n", changes, late)
}

func runUI(fallback, destroy bool) {
	model := gui.NewSliceListModel(options)
	long := longModel()
	empty := gui.NewSliceListModel[widgets.DropDownItem](nil)
	basic, custom, longSelection := -1, 0, 9998
	changes, late := 0, 0
	dark, red := false, false
	previous := make(map[string]*widgets.DropDown)
	connected := false
	err := ui.Run("org.golang-gui.DropDownTest", func(app ui.App) ui.RootView {
		gui.App.SetQuitOnLastWindowClosed(false)
		update := func() { app.RequestUpdate() }
		change := func(id string, value *int) func(int) {
			return func(index int) {
				*value = index
				changes++
				fmt.Printf("SELECT %s %d changes=%d\n", id, index, changes)
				if destroy {
					app.FindWindow("choices").Destroy()
				} else {
					update()
				}
			}
		}
		verify := func() {
			win := app.FindWindow("choices")
			check(win, previous, map[string]int{"basic": basic, "custom": custom, "long": longSelection, "disabled": 0, "empty": -1}, changes, late)
			for _, id := range []string{"set", "rebuild", "shrink", "dark", "accent", "reset", "check"} {
				r := gui.FindWidget(win, id).Snapshot().Bounds
				fmt.Printf("BOUNDS %s %.2f %.2f %.2f %.2f\n", id, r.X, r.Y, r.Width, r.Height)
			}
			if !connected {
				previous["basic"].ConnectSelected(func(int) { late++ })
				connected = true
			}
		}
		settings := app.Settings()
		makeChoice := func(id string, m gui.ListData[widgets.DropDownItem]) *wui.DropDownView {
			return wui.DropDown(m).ID(id).MinWidth(240).Placeholder("Choose an option").OnOpenError(func(err error) { panic(err) })
		}
		return ui.Root().StyleSheet(sheet(dark, red, fallback, settings.FontFamily(), settings.FontSize())).Windows(
			ui.Window("choices").Title("GOUI DropDown").Size(850, 640).Chrome(ui.WindowChromeIntegrated).
				OnDestroy(func() {
					app.Post(func() {
						for _, d := range previous {
							if !d.Destroyed() || d.Opened() {
								panic("destroy retained live widget/popup")
							}
						}
						if destroy && (changes != 1 || late != 0) {
							panic("destroyed signal continued")
						}
						fmt.Println("DESTROY PASS")
						app.Quit()
					})
				}).Content(ui.VBox(
				ui.HeaderBar(ui.Label("DropDown · Selected / OnSelected")),
				ui.VBox(
					ui.Label(fmt.Sprintf("Selected events %d | Basic %d | Custom %d | Long %d", changes, basic, custom, longSelection)),
					ui.HBox(
						ui.Button("Set").ID("set").OnClick(func() { basic = 0; update() }),
						ui.Button("Rebuild").ID("rebuild").OnClick(update),
						ui.Button("Shrink").ID("shrink").OnClick(func() {
							model.SetItems(options[:1])
							if basic >= 1 {
								basic = -1
							}
							if custom >= 1 {
								custom = -1
							}
							update()
						}),
						ui.Button("Dark").ID("dark").OnClick(func() { dark = !dark; update() }),
						ui.Button("Accent").ID("accent").OnClick(func() { red = !red; update() }),
						ui.Button("Reset").ID("reset").OnClick(func() { model.SetItems(options); basic, custom, longSelection = -1, 0, 9998; changes = 0; update() }),
						ui.Button("Check").ID("check").OnClick(verify),
					).Spacing(8),
					ui.HBox(ui.Label("Basic"), makeChoice("basic", model).Selected(basic).OnSelected(change("basic", &basic))).Spacing(16),
					ui.HBox(ui.Label("Custom"), makeChoice("custom", model).Selected(custom).OnSelected(change("custom", &custom)).
						Item(func(_ int, item wui.DropDownItem) ui.View {
							name := "drop-down-item-text"
							if item.Disabled {
								name += "-disabled"
							}
							return ui.VBox(ui.Label("● "+item.Text).Style(name), ui.Label("A custom Widget subtree").Style(modern.MutedText)).Spacing(2)
						}).
						SelectedItem(func(_ int, item wui.DropDownItem) ui.View { return ui.Label(item.Text).Style("drop-down-text") })).Spacing(16),
					ui.HBox(ui.Label("Long"), makeChoice("long", long).Selected(longSelection).OnSelected(change("long", &longSelection))).Spacing(16),
					ui.HBox(ui.Label("Uncontrolled"), makeChoice("uncontrolled", model).OnSelected(func(index int) {
						changes++
						fmt.Printf("SELECT uncontrolled %d changes=%d\n", index, changes)
						update()
					})).Spacing(16),
					ui.HBox(ui.Label("Disabled"), makeChoice("disabled", model).Selected(0).Enabled(false)).Spacing(16),
					ui.HBox(ui.Label("Empty"), makeChoice("empty", empty)).Spacing(16),
					ui.Label("Tab / Shift+Tab, Enter / Space, Up / Down, Home / End, Esc. F6 checks."),
				).Padding(20).Spacing(20).CrossAlign(layout.CrossStart),
			).CrossAlign(layout.CrossStretch).Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(verify))),
		)
	})
	if err != nil {
		panic(err)
	}
}

type customDelegate struct {
	model gui.ListData[widgets.DropDownItem]
}

func (d customDelegate) Setup() gui.Widget {
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.SetSpacing(2)
	title, description := gui.NewLabel(""), gui.NewLabel("A custom Widget subtree")
	title.SetStyleName("drop-down-item-text")
	description.SetStyleName(modern.MutedText)
	box.AddChild(title)
	box.AddChild(description)
	return box
}
func (d customDelegate) Bind(index int, w gui.Widget) {
	item := d.model.ItemAt(index)
	label := w.Children()[0].(*gui.Label)
	name := "drop-down-item-text"
	if item.Disabled {
		name += "-disabled"
	}
	label.SetStyleName(name)
	label.SetText("● " + item.Text)
}
func (customDelegate) Unbind(int, gui.Widget) {}

func runGUI(fallback, destroy bool) {
	app, err := gui.NewApplication("org.golang-gui.DropDownTest")
	if err != nil {
		panic(err)
	}
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 850, Height: 640}, Chrome: gui.WindowChromeIntegrated})
	if err != nil {
		panic(err)
	}
	if err = win.SetTitle("GOUI DropDown · GUI"); err != nil {
		panic(err)
	}
	model, long := gui.NewSliceListModel(options), longModel()
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetSpacing(20)
	root.SetPadding(20)
	root.SetCrossAlign(layout.CrossStart)
	status := gui.NewLabel("Selected events 0")
	root.AddChild(status)
	changes, late := 0, 0
	dark, red := false, false
	previous := make(map[string]*widgets.DropDown)
	choices := make(map[string]*widgets.DropDown)
	for _, id := range []string{"basic", "custom", "long", "uncontrolled", "disabled", "empty"} {
		d := widgets.NewDropDown()
		d.SetID(id)
		d.SetMinSize(geometry.Size{Width: 240})
		d.SetPlaceholder("Choose an option")
		m := gui.ListData[widgets.DropDownItem](model)
		if id == "long" {
			m = long
		} else if id == "empty" {
			m = nil
		}
		d.SetModel(m)
		if id == "custom" {
			d.SetDelegate(customDelegate{model})
			d.SetSelected(0)
		}
		if id == "long" {
			d.SetSelected(9998)
		}
		if id == "disabled" {
			d.SetSelected(0)
			d.SetEnabled(false)
		}
		if id == "uncontrolled" {
			d.SetSelected(0)
		}
		d.ConnectOpenError(func(err error) { panic(err) })
		d.ConnectSelected(func(index int) {
			changes++
			fmt.Printf("SELECT %s %d changes=%d\n", id, index, changes)
			status.SetText(fmt.Sprintf("Selected events %d", changes))
			if destroy {
				win.Destroy()
			}
		})
		row := gui.NewLinearBox(layout.DirectionHorizontal)
		row.SetSpacing(16)
		row.AddChild(gui.NewLabel(id))
		row.AddChild(d)
		root.AddChild(row)
		choices[id] = d
	}
	choices["basic"].ConnectSelected(func(int) { late++ })
	buttons := gui.NewLinearBox(layout.DirectionHorizontal)
	buttons.SetSpacing(8)
	for _, entry := range []struct {
		text string
		fn   func()
	}{
		{"Set", func() { choices["basic"].SetSelected(0) }},
		{"Shrink", func() { model.SetItems(options[:1]) }},
		{"Reset", func() {
			model.SetItems(options)
			choices["basic"].SetSelected(-1)
			choices["custom"].SetSelected(0)
			choices["long"].SetSelected(9998)
			changes = 0
			status.SetText("Selected events 0")
		}},
		{"Dark", func() {
			dark = !dark
			settings := app.Settings()
			app.SetStyleSheet(sheet(dark, red, fallback, settings.FontFamily(), settings.FontSize()))
		}},
		{"Accent", func() {
			red = !red
			settings := app.Settings()
			app.SetStyleSheet(sheet(dark, red, fallback, settings.FontFamily(), settings.FontSize()))
		}},
		{"Check", func() { check(win, previous, nil, changes, late) }},
	} {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(entry.text))
		b.ConnectClicked(entry.fn)
		buttons.AddChild(b)
	}
	root.AddChild(buttons)
	win.SetWidget(root)
	verify := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyF6})
	verify.ConnectActivate(func() { check(win, previous, nil, changes, late) })
	win.Shortcuts().AddShortcut(verify)
	win.ConnectDestroy(func() {
		if destroy && (changes != 1 || late != 0) {
			panic("destroyed signal continued")
		}
		fmt.Println("DESTROY PASS")
	})
	settings := app.Settings()
	app.SetStyleSheet(sheet(false, false, fallback, settings.FontFamily(), settings.FontSize()))
	if err = win.Show(); err != nil {
		panic(err)
	}
	app.Run()
}
