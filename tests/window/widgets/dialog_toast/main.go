// Dialog / Toast 及声明绑定桌面验收，不使用 DevServer。
//
// 环境：Linux X11、Windows 或 macOS 桌面。启动：go run ./tests/window/widgets/dialog_toast。
// GOUI_PLAT_PAINTER 选择后端、GOUI_PLAT_SCALE 选择倍率；-auto 经正常输入路径断言。
// 初始：Integrated 窗口、64 DIP HeaderBar、后台编辑框、Dialog 隐藏、Toast 为空；使用 Modern 亮色。
// 操作与预期：
//  1. 点 Open dialog：焦点进入内容，Tab/Shift+Tab 仅在内部循环；F5 不作用于后台。
//     输入中文，点 Rebuild：内容、焦点范围与输入保留。Esc 提出关闭请求，由状态隐藏。
//     打开、重建和关闭 Dialog 时，窗口按钮的大小和位置始终不变，仍在 HeaderBar 垂直居中。
//  2. Caption 按钮循环 Off、All、Top。Off 下点击遮罩请求关闭，All 下整片遮罩可拖动。
//     选 Top 再打开 Dialog，从卡片上方拖动窗口，松手后 Dialog 保持打开。重新打开后
//     分别点击卡片左右或下方的遮罩，每次应发出一次关闭请求。卡片内边距和间隙保留
//     普通输入。跨区按下和松开不应误触取消。窗口控制按钮及缩放区域照常操作。
//  3. 点 Timed toast：显示约 1 秒后隐藏。悬停或聚焦内容中的 Dismiss 按钮暂停；移出
//     并转移焦点后按剩余时间继续。点 Persistent toast：不自动隐藏，Dismiss 关闭。
//  4. 反复请求 Toast：替换当前内容并重新计时，不排队；旧超时不关闭新提示。
//     Toast 显示时按 F5 或 Rebuild 更新声明，不重启计时；结束后重建不复活旧内容。
//  5. Theme 在 Modern 亮／暗和 Basic 间切换：Dialog、Toast 均有背景、边框和圆角；
//     阴影保留在自己的区域，内容文字和按钮维持自身样式。调整窗口大小应仍合理布局。
//
// -auto 覆盖窗口按钮位置、模态、文字、计时、声明释放，以及 Top 模式的区域点击。
// 实际窗口移动通过手动拖动或 X11 原生输入验收。
// 平台差异：Chrome 拖动遵循现有平台能力；CommitText 不模拟 IME 预编辑或候选窗。
// Linux 无合成器而回退 Native 时，-auto 明确跳过 Integrated 按钮几何检查，其余检查继续。
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
	started := false
	caption := wui.BackdropCaptionNone
	mode, keys, closeRequests, step, builds := 0, 0, 0, 0, 0
	var controlsBefore geometry.Rectangle
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
				clickAt := func(p geometry.Point) {
					pointer(events.PointerDown, p)
					pointer(events.PointerUp, p)
				}
				click := func(id string) { clickAt(widget(id).Snapshot().Bounds.Center()) }
				currentToast := func() *widgets.Toast {
					for w := widget("toast-text"); w != nil; w = w.Parent() {
						if t, ok := w.(*widgets.Toast); ok {
							return t
						}
					}
					return nil
				}
				controlsInfo := func() (info gui.ChromeInfo) {
					win.Chrome().ConnectInfo(func(current gui.ChromeInfo) { info = current }).Disconnect()
					return info
				}
				switch step {
				case 0:
					info := controlsInfo()
					controlsBefore = info.ControlsBounds
					if info.Enabled {
						require(controlsBefore.Height > 0 && controlsBefore.Center().Y == 32, "窗口按钮未在 64 DIP HeaderBar 中居中")
					} else {
						fmt.Println("SKIP Integrated 按钮几何：已回退原生装饰")
					}
					key(events.KeyF5)
					click("open")
				case 1:
					require(controlsInfo().ControlsBounds == controlsBefore, "Dialog 改变了窗口按钮位置")
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
					require(controlsInfo().ControlsBounds == controlsBefore, "关闭 Dialog 改变了窗口按钮位置")
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
					caption = wui.BackdropCaptionTop
					show.Set(true)
				case 18:
					d := widget("dialog").(*widgets.Dialog)
					require(d.Visible() && d.BackdropCaptionMode() == widgets.BackdropCaptionTop, "Top mode not applied")
					bounds := d.Snapshot().Bounds
					top := geometry.Point{X: bounds.Center().X, Y: bounds.Y + 24}
					clickAt(top)
					require(show.Get() && closeRequests == 1, "top click dismissed dialog")
					child := d.Child().Snapshot().Bounds
					side := geometry.Point{X: bounds.X + 24, Y: child.Y - d.Padding().Top + 0.25}
					above := side
					above.Y -= 0.5
					pointer(events.PointerDown, side)
					pointer(events.PointerUp, above)
					require(show.Get() && closeRequests == 1, "cross-region release dismissed dialog")
					pointer(events.PointerDown, side)
					pointer(events.PointerMove, side.Add(geometry.Point{Y: 24}))
					pointer(events.PointerUp, side)
					require(show.Get() && closeRequests == 1, "side drag dismissed dialog")
					clickAt(side)
				case 19:
					require(!show.Get() && closeRequests == 2, "side click did not dismiss dialog")
					show.Set(true)
				case 20:
					bounds := widget("dialog").Snapshot().Bounds
					clickAt(geometry.Point{X: bounds.Center().X, Y: bounds.Y + bounds.Height - 24})
				case 21:
					require(!show.Get() && closeRequests == 3, "bottom click did not dismiss dialog")
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
				ui.VBox(ui.HeaderBar(ui.Label("Dialog / Toast — ordinary Overlay composition")).MinHeight(64), ui.VBox(
					ui.Label(fmt.Sprintf("Build %d | F5 %d | Close requests %d", builds, keys, closeRequests)),
					ui.TextInput().ID("background").Text("background"),
					ui.Button("Open dialog").ID("open").OnClick(func() { show.Set(true) }),
					ui.Button("Backdrop Caption: "+[...]string{"Off", "All", "Top"}[caption]).OnClick(func() {
						caption = (caption + 1) % 3
						app.RequestUpdate()
					}),
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
						BackdropCaptionMode(caption).CloseOnOutside(true).OnCloseRequest(func() { closeRequests++; show.Set(false); app.RequestUpdate() })
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
		require(step >= 22, "window exited before checks completed")
		require(toast.Close() == wui.ErrToastUnavailable, "UI release did not unbind handle")
		fmt.Printf("PASS Dialog Toast builds=%d\n", builds)
	}
}
