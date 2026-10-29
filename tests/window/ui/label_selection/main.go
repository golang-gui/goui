// Label 可选择文本与声明式选区绑定验收，不使用 DevServer。
// 环境：Linux X11、Windows 或 macOS 桌面及系统字体；记录后端/缩放/窗口管理器。
// 启动：go run ./tests/window/ui/label_selection [-auto] [-capture-dir DIR]。
// GOUI_PLAT_PAINTER=software/opengl（Windows 另测 direct2d），GOUI_PLAT_SCALE=1/2。
// DIR 为仓库外已有目录。两种模式先运行自动断言；普通模式待 PASS 后手动操作。
// 初始：Integrated、Modern 亮色；第一段可选择的混合格式文字，第二段可选择的
// 双向文字，第三段普通 Label，以及供粘贴/焦点切换的 TextInput。
// 操作与预期：
//  1. 从第一段首字拖到正文外，整段选中；Shift+方向键调整；e+组合音标和 emoji
//     不可拆分，换行/混合字号没有缝隙或错误选区；仅背景变化，彩色字仍保留颜色。
//  2. 双击 Alpha 选中单词。右键不改变已有选区；菜单 Copy 后粘贴到输入框，
//     得到无格式语法的纯文本，硬换行保留、软换行不插入字符。
//  3. 在 Label 按 Primary+A/C，全选/复制；Primary 在 macOS 是 Command，其他 Ctrl。
//     Tab/Shift+Tab 在可选 Label、按钮和输入框间导航，普通 Label 不获得焦点。
//     Shift+F10/Menu 打开选择菜单；Escape 关闭菜单或折叠选区。
//  4. 点击 Theme：选区仍在、文字颜色随主题；点击 Format：只改变格式、选区保留；
//     点击 Text：正文更新、旧选区复位。Enable 开关关闭选择后不能拖选，重新开启恢复。
//  5. 在双向文本内选择/左右导航，按视觉字形移动；再点输入框，Label 的失焦选区
//     保留且减弱。鼠标拖动普通 Label 不能选择；编辑框不解释格式语法。
//
// 自动步骤走 Window.DispatchEvent，覆盖拖选捕获、快照、UI 写回、Unicode Cluster、
// Primary+A/C、菜单键及 Copy、正文/格式/主题变化、禁用恢复和实际系统剪贴板往返。
// 截图由 RenderWidget 导出供检查，不能代替原生窗口视觉验收；不验收 IME/触摸/自动滚动。
// 副作用：打开窗口和临时菜单，系统剪贴板会被测试文案覆盖；不更改系统主题/语言。
// 复位：重启。退出：关闭窗口；-auto 在 PASS 后退出，提前退出/超时/断言失败均失败。
package main

import (
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func require(ok bool, args ...any) {
	if !ok {
		log.Fatal(args...)
	}
}
func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	auto := flag.Bool("auto", false, "自动断言后退出")
	capture := flag.String("capture-dir", "", "仓库外截图目录")
	flag.Parse()
	const formatted = `\{format\}Alpha \{bold:é 👩‍💻 中文\} \{warning:warning\} العربية` + "\nSecond line / wrapped selection end"
	const plain = "Alpha é 👩‍💻 中文 warning العربية\nSecond line / wrapped selection end"
	text := ui.MakeState(formatted)
	selection := ui.MakeState(ui.TextSelection{})
	feedback := ui.MakeState("Selection: empty")
	dark, enabled, started, done, changedFormat := false, true, false, false, false
	stage, builds := 0, 0
	var original *gui.Label
	var clipboard string
	clipboardReady := false
	deadline := time.Now().Add(25 * time.Second)
	err := ui.Run("org.golang-gui.LabelSelection", func(app ui.App) ui.RootView {
		builds++
		if !started {
			started = true
			var timer *ui.Timer
			timer = app.TickFunc(220*time.Millisecond, func() {
				require(time.Now().Before(deadline), "Label selection timeout stage=", stage)
				win := app.FindWindow("main")
				label := app.FindWidget("main", "selectable").(*gui.Label)
				if label.Rect().Width <= 0 || !win.Focused() {
					return
				}
				key := func(k events.Key, mods events.Modifiers) {
					for _, kind := range []events.EventType{events.KeyDown, events.KeyUp} {
						must(win.DispatchEvent(events.KeyEvent{EventType: kind, Key: k, Modifiers: mods}))
					}
				}
				primary, _ := gui.ModPrimary.Resolve()
				pointer := func(kind events.EventType, point geometry.Point, button events.PointerButton) {
					e := events.PointerEvent{EventType: kind, Position: point, Button: button}
					if button == events.PointerButtonLeft && kind != events.PointerUp {
						e.Buttons = events.PointerButtonLeftDown
					}
					must(win.DispatchEvent(e))
				}
				requestClipboard := func() {
					clipboardReady = false
					gui.App.Clipboard().RequestText(func(value string, err error) { must(err); clipboard, clipboardReady = value, true })
				}
				switch stage {
				case 0:
					original = label
					require(label.PlainText() == plain, "unexpected text ", label.PlainText())
					r := label.Snapshot().Bounds
					pointer(events.PointerDown, geometry.Point{X: r.X, Y: r.Y + 2}, events.PointerButtonLeft)
					pointer(events.PointerMove, geometry.Point{X: r.X + r.Width + 30, Y: r.Y + r.Height + 10}, events.PointerButtonLeft)
					pointer(events.PointerUp, geometry.Point{X: r.X + r.Width + 30, Y: r.Y + r.Height + 10}, events.PointerButtonLeft)
					require(label.SelectedText() == plain && selection.Get() == label.Selection(), "drag capture or UI writeback: ", label.Selection())
					info := label.Snapshot()
					meta := info.Attributes[gui.LabelInfoKey].(gui.LabelInfo)
					require(info.Role == gui.RoleLabel && info.Text == plain && info.TextEditing == nil && meta.Selectable && meta.Selection == label.Selection(), "snapshot contract")
					key(events.KeyC, primary)
					requestClipboard()
				case 1:
					if !clipboardReady {
						return
					}
					require(clipboard == plain, "copy introduced markers or soft newlines: ", clipboard)
					// 独立预期：最后一个 ASCII 字形长度为一，Shift+Left 只选 d。
					key(events.KeyEnd, primary)
					key(events.KeyArrowLeft, events.ModifierShift)
					require(label.SelectedText() == "d", "visual endpoint navigation: ", label.Selection(), " / ", label.SelectedText())
					// 组合字符不得单独命中：用程序设置一个码点边界，再由真实 Cluster 校正。
					label.SetSelection(gui.TextSelection{Anchor: len("Alpha "), Caret: len("Alpha e")})
					require(label.Selection().Anchor == len("Alpha ") && label.Selection().Caret == len("Alpha "), "split combining Cluster: ", label.Selection())
					key(events.KeyA, primary)
					key(events.KeyF10, events.ModifierShift)
				case 2:
					require(len(win.Popovers()) == 1, "keyboard selection menu did not open")
					require(label.SelectedText() == plain, "menu changed selection")
					key(events.KeyHome, 0)
					key(events.KeyEnter, 0)
					requestClipboard()
				case 3:
					if !clipboardReady {
						return
					}
					require(clipboard == plain && len(win.Popovers()) == 0, "context Copy or dismissal failed")
					text.Set(`\{format\}Alpha \{italic:é 👩‍💻 中文\} \{warning:warning\} العربية` + "\nSecond line / wrapped selection end")
				case 4:
					require(label == original && label.SelectedText() == plain, "format change reset selection")
					dark = true
					app.RequestUpdate()
				case 5:
					require(label == original && label.SelectedText() == plain, "theme/rebuild reset selection")
					if *capture != "" {
						img, e := gui.RenderWidget(win.Widget(), 0)
						must(e)
						f, e := os.Create(filepath.Join(*capture, "label-selection.png"))
						must(e)
						must(png.Encode(f, img))
						must(f.Close())
					}
					// 失焦保留，正文变化没有显式 Selection 声明时复位。
					require(win.SetFocusedWidget(app.FindWidget("main", "paste")), "focus paste")
					require(label.SelectedText() == plain, "blur reset selection")
					label.SetText("replacement")
					require(label.Selection() == (gui.TextSelection{}), "new plain text did not reset selection")
					text.Set(formatted)
					selection.Set(ui.TextSelection{})
				case 6:
					enabled = false
					app.RequestUpdate()
				case 7:
					require(!label.Selectable() && !label.Focusable(), "selection disable")
					enabled = true
					app.RequestUpdate()
				case 8:
					require(label.Selectable() && label.Focusable() && label == original, "selection re-enable/identity")
					done = true
					timer.Stop()
					log.Printf("PASS Label selection typography=%s scale=%s painter=%s builds=%d", gui.App.Typography().Name(), os.Getenv("GOUI_PLAT_SCALE"), os.Getenv("GOUI_PLAT_PAINTER"), builds)
					if *auto {
						win.Destroy()
					}
				}
				stage++
			})
		}
		format := func() {
			changedFormat = !changedFormat
			if changedFormat {
				text.Set(`\{format\}\{italic:` + plain + `\}`)
			} else {
				text.Set(formatted)
			}
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{Dark: dark})).Windows(
			ui.Window("main").Title("GOUI selectable Label").Size(760, 510).Center(true).Chrome(gui.WindowChromeIntegrated).Content(
				ui.VBox(
					ui.HeaderBar(ui.Label("Label — plain-text selection / styled text")),
					ui.VBox(
						ui.HBox(
							ui.Button("Theme").OnClick(func() { dark = !dark; app.RequestUpdate() }),
							ui.Button("Format").OnClick(format),
							ui.Button("Text").OnClick(func() { text.Set("Replacement plain text") }),
							ui.Button("Enable").OnClick(func() { enabled = !enabled; app.RequestUpdate() }),
						).Spacing(8),
						ui.Label().ID("selectable").BindText(&text).Selectable(enabled).BindSelection(&selection).OnSelection(func(value ui.TextSelection) {
							selection.Set(value)
							feedback.Set(fmt.Sprintf("Selection: %d → %d", value.Anchor, value.Caret))
						}).OnCopyError(func(err error) { log.Fatal(err) }).WrapMode(ui.WrapWordChar).MaxWidth(400),
						ui.Label(`\{format\}LTR / \{bold:שלום العربية\} / end`).ID("bidi").Selectable(true),
						ui.Label("Ordinary Label: not selectable"),
						ui.Label().BindText(&feedback),
						ui.TextInput().ID("paste").Placeholder("Paste copied text here"),
						ui.Label("Primary+A/C · Shift+arrows · right-click Copy · Theme preserves selection").WrapMode(ui.WrapWordChar),
					).Padding(18).Spacing(16).CrossAlign(layout.CrossStretch).MainWeight(1),
				).CrossAlign(layout.CrossStretch),
			),
		)
	})
	must(err)
	require(done, "window closed before verification completed")
}
