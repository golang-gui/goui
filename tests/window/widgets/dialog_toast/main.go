// Dialog / Toast 及声明绑定桌面验收，不使用 DevServer。
//
// 环境：Linux X11、Windows 或 macOS 桌面。启动：go run ./tests/window/widgets/dialog_toast。
// GOUI_PLAT_PAINTER 选择后端、GOUI_PLAT_SCALE 选择倍率；-auto 经正常输入路径断言。
// 初始：Integrated 窗口、后台编辑框、Dialog 隐藏、Toast 为空；使用 Modern 亮色。
// 操作与预期：
//  1. 点 Open dialog：焦点进入内容，Tab/Shift+Tab 仅在内部循环；F5 不作用于后台。
//     输入中文，点 Rebuild：内容、焦点范围与输入保留。Esc 提出关闭请求，由状态隐藏。
//  2. Caption 默认关闭，点击遮罩按 CloseOnOutside 提出关闭请求；卡片内边距和控件
//     间隙不关闭。关闭后，打开 Caption 再开 Dialog：只有外部遮罩可拖动窗口，卡片
//     空白不可拖动；Caption 不同时触发外部点击关闭。原生控制按钮仍可关闭窗口。
//  3. 点 Timed toast：显示约 1 秒后隐藏。悬停或聚焦内容中的 Dismiss 按钮暂停；移出
//     并转移焦点后按剩余时间继续。点 Persistent toast：不自动隐藏，Dismiss 关闭。
//  4. 反复请求 Toast：替换当前内容并重新计时，不排队；旧超时不关闭新提示。
//     Toast 显示时按 F5 或 Rebuild 更新声明，不重启计时；结束后重建不复活旧内容。
//  5. Theme 在 Modern 亮／暗和 Basic 间切换：Dialog、Toast 均有背景、边框和圆角；
//     阴影保留在自己的区域，内容文字和按钮维持自身样式。调整窗口大小应仍合理布局。
//
// -auto 覆盖模态、文字、真实 Timer、悬停暂停、替换、关闭、普通重建与句柄释放。
// 平台差异：Chrome 拖动遵循现有平台能力；CommitText 不模拟 IME 预编辑或候选窗。
// 副作用：仅本进程窗口，不改系统设置或剪贴板。复位：重新启动。
// 退出：关闭窗口或 Exit；-auto 输出 PASS 后退出；启动方应设置约 20 秒的进程超时。
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

func main() {
	auto := flag.Bool("auto", false, "自动断言后退出")
	flag.Parse()
	show, text := ui.MakeState(false), ui.MakeState("")
	var toast wui.ToastHandle
	started, caption := false, false
	mode, keys, closeRequests, step, builds := 0, 0, 0, 0, 0
	err := ui.Run("org.golang-gui.DialogToast", func(app ui.App) ui.RootView {
		builds++
		requestToast := func(message string, duration time.Duration) {
			if err := toast.Show(ui.HBox(ui.Label(message).ID("toast-text"), ui.Button("Dismiss").ID("toast-dismiss").OnClick(func() { _ = toast.Close() })).Spacing(16), duration); err != nil {
				panic(err)
			}
			fmt.Println("TOAST", message)
		}
		if !started {
			started = true
			var timer *ui.Timer
			timer = app.TickFunc(250*time.Millisecond, func() {
				win := app.FindWindow("main")
				widget := func(id string) gui.Widget { return gui.FindWidget(win, id) }
				if back := widget("open"); step == 0 && (back == nil || back.Rect().Width <= 0) {
					return
				}
				if !*auto {
					fmt.Println("READY Dialog / Toast")
					timer.Stop()
					return
				}
				key := func(k events.Key) {
					if err := win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: k}); err != nil {
						panic(err)
					}
				}
				pointer := func(kind events.EventType, p geometry.Point) {
					if err := win.DispatchEvent(events.PointerEvent{EventType: kind, Position: p, Button: events.PointerButtonLeft}); err != nil {
						panic(err)
					}
				}
				click := func(id string) {
					p := widget(id).Snapshot().Bounds.Center()
					pointer(events.PointerDown, p)
					pointer(events.PointerUp, p)
				}
				currentToast := func() *widgets.Toast {
					for w := widget("toast-text"); w != nil; w = w.Parent() {
						if t, ok := w.(*widgets.Toast); ok {
							return t
						}
					}
					return nil
				}
				switch step {
				case 0:
					key(events.KeyF5)
					click("open")
				case 1:
					d := widget("dialog")
					info, ok := d.Snapshot().Attributes[gui.ModalInfoKey].(gui.ModalInfo)
					require(ok && info.Active && win.FocusedWidget() == widget("edit"), "dialog focus / modal")
					key(events.KeyF5)
					require(keys == 1, "background shortcut escaped")
					if err := win.CommitText("Dialog 中文"); err != nil {
						panic(err)
					}
					for range 9 {
						key(events.KeyTab)
						require(win.FocusedWidget() != widget("open"), "Tab escaped")
					}
					key(events.KeyEscape)
				case 2:
					require(!show.Get() && text.Get() == "Dialog 中文" && closeRequests == 1, "dialog close request / text")
					requestToast("timed original", time.Second)
				case 3:
					t := currentToast()
					require(t != nil && t.Visible(), "timed toast missing")
					pointer(events.PointerMove, widget("toast-text").Snapshot().Bounds.Center())
				case 4:
					for i := 0; i < 20 && win.FocusedWidget() != widget("toast-dismiss"); i++ {
						key(events.KeyTab)
					}
					require(win.FocusedWidget() == widget("toast-dismiss"), "toast action focus")
					pointer(events.PointerMove, geometry.Point{X: 40, Y: 80})
				case 5, 6, 7, 8:
					require(currentToast() != nil && currentToast().Visible(), "hover did not pause timeout")
				case 9:
					key(events.KeyTab)
					require(win.FocusedWidget() != widget("toast-dismiss"), "toast focus did not leave")
					pointer(events.PointerMove, geometry.Point{X: 40, Y: 80})
					requestToast("new timed request", time.Second)
				case 10:
					app.RequestUpdate()
				case 11:
					require(currentToast() != nil && widget("toast-text").Snapshot().Text == "new timed request", "old timeout closed replacement")
				case 15:
					require(currentToast() == nil, "timeout / rebuild resurrected toast")
					requestToast("persistent", 0)
				case 16:
					require(currentToast() != nil, "persistent toast missing")
					click("toast-dismiss")
				case 17:
					require(currentToast() == nil, "dismiss did not release declaration content")
					timer.Stop()
					win.Destroy()
				}
				step++
			})
		}
		var sheet style.StyleSheet = modern.Sheet(modern.Options{Dark: mode == 1})
		if mode == 2 {
			sheet = basic.Sheet()
		}
		return ui.Root().StyleSheet(sheet).Windows(ui.Window("main").Title("GOUI Dialog / Toast").Chrome(ui.WindowChromeIntegrated).Center(true).Size(840, 660).
			Shortcuts(ui.Shortcut(ui.KeyF5).OnActivate(func() { keys++; fmt.Printf("F5=%d\n", keys); app.RequestUpdate() })).
			Content(ui.Overlay(
				ui.VBox(ui.HeaderBar(ui.Label("Dialog / Toast — ordinary Overlay composition")), ui.VBox(
					ui.Label(fmt.Sprintf("Build %d | F5 %d | Close requests %d", builds, keys, closeRequests)),
					ui.TextInput().ID("background").Text("background"),
					ui.Button("Open dialog").ID("open").OnClick(func() { show.Set(true) }),
					ui.Button(fmt.Sprintf("Backdrop Caption: %v", caption)).OnClick(func() { caption = !caption; app.RequestUpdate() }),
					ui.Button("Timed toast").ID("timed").OnClick(func() { requestToast("Saved successfully", time.Second) }),
					ui.Button("Persistent toast").ID("persistent").OnClick(func() { requestToast("Waiting for confirmation", 0) }),
					ui.Button("Theme: Modern light / dark / Basic").ID("theme").OnClick(func() { mode = (mode + 1) % 3; app.RequestUpdate() }),
					ui.Button("Exit").OnClick(app.Quit),
				).Padding(20).Spacing(10).CrossAlign(layout.CrossStretch)).CrossAlign(layout.CrossStretch),
				ui.OverlayItem(ui.Build(func() ui.View {
					return wui.Dialog(ui.VBox(
						ui.Label("Dialog card — content decides its size"), ui.TextInput().ID("edit").BindText(&text),
						ui.Button("Rebuild").ID("rebuild").OnClick(app.RequestUpdate),
						ui.Button("Close dialog").ID("close").OnClick(func() { show.Set(false) }),
					).Spacing(12).MinWidth(360).MaxWidth(520).CrossAlign(layout.CrossStretch)).ID("dialog").Visible(show.Get()).
						BackdropCaption(caption).CloseOnOutside(true).OnCloseRequest(func() { closeRequests++; show.Set(false); app.RequestUpdate() })
				}).Observe(&show)).Fill(true).Modal(true),
				ui.OverlayItem(wui.Toast(&toast).ID("notifications")).OnPosition(func(available, size geometry.Size, p *geometry.Point) {
					*p = geometry.Point{X: (available.Width - size.Width) / 2, Y: available.Height - size.Height - 16}
				}),
			)))
	})
	if err != nil {
		log.Fatal(err)
	}
	if *auto {
		require(step >= 18, "window exited before checks completed")
		require(toast.Close() == wui.ErrToastUnavailable, "UI release did not unbind handle")
		fmt.Printf("PASS Dialog Toast builds=%d\n", builds)
	}
}
