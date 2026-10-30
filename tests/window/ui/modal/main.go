// OverlayItem.Modal 声明式窗口验收，不使用 DevServer。
//
// 环境：Linux X11、Windows 或 macOS 桌面。启动：go run ./tests/window/ui/modal。
// GOUI_PLAT_PAINTER 选择后端；GOUI_PLAT_SCALE 选择倍率；-auto 经正常输入路径断言。
// 初始：Integrated 窗口，后台输入框为 background，Window F5 为 0，确认层隐藏。
// 操作与预期：
//  1. 后台编辑框可以输入，F5 增加窗口计数。点 Open modal，焦点进入内部输入框。
//  2. 内部输入文字，后台保持原值；连续 Tab/Shift+Tab 仅在确认层内循环，F5 不增加。
//  3. 点击遮罩：后台无响应，不关闭；卡片空白及遮罩都不拖窗，本例未声明 Caption。
//  4. 点 Rebuild：执行全应用声明更新，内部文本、Widget 身份及模态状态保持不变。
//  5. 点 Close 或按 Esc：仅隐藏确认层，恢复打开前焦点，F5 再次生效。
//     反复开关应保留内部输入文字。关闭后窗体仍能正常调整大小和移动。
//
// 平台差异：原生装饰、字体与 IME 由系统决定，-auto 的 CommitText 不模拟预编辑。
// 本例用普通 Overlay、布局和样式组合外观，不提供 Dialog 控件或自动关闭策略。
// 副作用：仅本进程窗口，不修改设置或剪贴板。复位：重新启动。
// 退出：关闭窗口或 Exit；-auto 输出 PASS 后退出。启动方应另设进程超时。
package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

func main() {
	auto := flag.Bool("auto", false, "自动断言后退出")
	flag.Parse()
	show, backText, text := ui.MakeState(false), ui.MakeState("background"), ui.MakeState("")
	started, builds, keys, step := false, 0, 0, 0
	var original gui.Widget
	err := ui.Run("org.golang-gui.UIOverlayModal", func(app ui.App) ui.RootView {
		builds++
		closeLayer := func() { show.Set(false) }
		if !started {
			started = true
			var timer *ui.Timer
			timer = app.TickFunc(500*time.Millisecond, func() {
				win := app.FindWindow("main")
				// 首次 Direct2D 初始化可能晚于首个 Tick；等真实布局完成再发输入。
				back := gui.FindWidget(win, "back")
				if step == 0 && (back == nil || back.Rect().Width <= 0 || back.Rect().Height <= 0) {
					return
				}
				widget := func(id string) gui.Widget { return gui.FindWidget(win, id) }
				if step == 0 {
					for _, id := range []string{"back", "open", "edit", "rebuild", "close"} {
						b := widget(id).Snapshot().Bounds
						fmt.Printf("BOUNDS %s %.1f %.1f %.1f %.1f\n", id, b.X, b.Y, b.Width, b.Height)
					}
				}
				if !*auto {
					timer.Stop()
					return
				}
				click := func(id string) {
					p := widget(id).Snapshot().Bounds.Center()
					for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
						if err := win.DispatchEvent(events.PointerEvent{EventType: kind, Position: p, Button: events.PointerButtonLeft}); err != nil {
							panic(err)
						}
					}
				}
				key := func(k events.Key, mods events.Modifiers) {
					if err := win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: k, Modifiers: mods}); err != nil {
						panic(err)
					}
				}
				active := func() bool {
					info, ok := widget("layer").Snapshot().Attributes[gui.ModalInfoKey].(gui.ModalInfo)
					return ok && info.Active
				}
				switch step {
				case 0:
					click("back")
					key(events.KeyF5, 0)
					click("open")
				case 1:
					require(active() && win.FocusedWidget() == widget("edit"), "initial modal focus")
					key(events.KeyF5, 0)
					require(keys == 1, "window shortcut escaped")
					for i := range 12 {
						mods := events.Modifiers(0)
						if i > 5 {
							mods = events.ModifierShift
						}
						key(events.KeyTab, mods)
						focus := win.FocusedWidget()
						require(focus == widget("edit") || focus == widget("rebuild") || focus == widget("close"), "Tab escaped")
					}
					click("edit")
					if err := win.CommitText("declarative 中文"); err != nil {
						panic(err)
					}
					require(text.Get() == "declarative 中文" && backText.Get() == "background", "text target")
					original = widget("edit")
					click("rebuild")
				case 2:
					require(builds == 2 && active() && original == widget("edit") && text.Get() == "declarative 中文", "rebuild lost modal state")
					click("close")
				case 3:
					require(!active() && !show.Get() && win.FocusedWidget() == widget("open"), "focus restore")
					key(events.KeyF5, 0)
					require(keys == 2, "shortcut restore")
					click("open")
				case 4:
					require(active() && original == widget("edit") && text.Get() == "declarative 中文", "show recreated content")
					key(events.KeyEscape, 0)
				case 5:
					require(!active() && !show.Get(), "Escape close")
					fmt.Println("PASS OverlayItem.Modal")
					timer.Stop()
					win.Destroy()
				}
				step++
			})
		}
		return ui.Root().StyleSheet(style.Sheet(append(modern.Rules(modern.Options{}),
			style.Name("modal-test-mask").BackgroundColor(color.NRGBA{A: 110}),
		)...)).Windows(ui.Window("main").Title("GOUI UI Overlay Modal").Size(820, 610).
			Center(true).Chrome(ui.WindowChromeIntegrated).
			Shortcuts(ui.Shortcut(ui.KeyF5).OnActivate(func() {
				keys++
				fmt.Printf("WINDOW F5=%d\n", keys)
			})).Content(ui.Overlay(
			ui.VBox(
				ui.HeaderBar(ui.Label("OverlayItem.Modal — declarative input boundary")),
				ui.VBox(
					ui.Label(fmt.Sprintf("Build %d — F5 tests the window shortcut", builds)),
					ui.TextInput().ID("back").BindText(&backText),
					ui.Button("Open modal").ID("open").OnClick(func() { show.Set(true) }),
					ui.Button("Exit").OnClick(app.Quit),
				).Padding(20).Spacing(12).CrossAlign(layout.CrossStretch),
			).CrossAlign(layout.CrossStretch),
			ui.OverlayItem(ui.Build(func() ui.View {
				return ui.Overlay(ui.HBox().Style("modal-test-mask"),
					ui.OverlayItem(ui.VBox(
						ui.Label("Modal content — Tab stays here"),
						ui.TextInput().ID("edit").BindText(&text),
						ui.Button("Rebuild").ID("rebuild").OnClick(app.RequestUpdate),
						ui.Button("Close").ID("close").OnClick(closeLayer),
					).Padding(24).Spacing(12).MinWidth(380).MaxWidth(500).
						CrossAlign(layout.CrossStretch).Style(modern.Card)).
						OnPosition(func(available, size geometry.Size, p *geometry.Point) {
							*p = geometry.Point{X: (available.Width - size.Width) / 2, Y: (available.Height - size.Height) / 2}
						}),
				).ID("layer").Visible(show.Get()).Shortcuts(ui.Shortcut(ui.KeyEscape).OnActivate(closeLayer))
			}).Observe(&show)).Fill(true).Modal(true),
		)))
	})
	if err != nil {
		log.Fatal(err)
	}
	if *auto {
		require(step >= 6, "window exited before checks completed")
	}
}
