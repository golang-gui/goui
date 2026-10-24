// 声明式 Tooltip 手工验收，不依赖 DevServer，不自动模拟输入。
// 环境：Linux X11、Windows 或 macOS 的交互桌面；记录 OS/WM、后端和缩放。
// 启动：go run ./tests/window/ui/tooltip；可设置 GOUI_PLAT_SCALE=1.5/2，
// GOUI_PLAT_PAINTER=software/opengl（Windows 另可 direct2d）。
// 独立 X11 未运行 Fcitx 时可用 XMODIFIERS=@im=none 启用本地 XIM 输入。
// 初始：Integrated 800×560 DIP、Basic 样式、显式提示、目标已挂载且可见。
//
// 操作与逐项预期：
//  1. 悬停“提示目标”约 700 ms，出现多行提示；Tab 到目标也能显示提示。
//     切换目标或离开后重新进入也等待 700 ms。分别停在按钮文字和四周留白，
//     每处保持三秒，提示与 Hover 应保持稳定；Modern 亮暗均为无阴影圆角边框。
//     Escape 关闭；点击目标只增加一次计数，提示不抢焦点或阻断点击。
//     禁用按钮显示不可用原因，但点击不执行动作；输入框可正常输入和选取。
//  2. 在输入框输入文字并选中一部分，依次点“更新文字”“无关重建”，文字和
//     选区保持不变。提示更新为最新版本，目标身份保持；F6/“检查”输出 ASSERT PASS。
//  3. 点“清空提示”，目标不再显示提示；点“省略修饰器”，恢复控件构造时的
//     默认提示，区别于显式空串。点“更新文字”恢复显式值。
//  4. 点“延迟更新”，随后悬停目标不动；两秒后提示重新等待并显示新版文字。
//     点“显示/隐藏”或“挂载/卸载”，旧提示立即失效；恢复后可重新触发。
//     卸载再挂载应创建新控件，输入框和其他已有控件不被替换。
//  5. 切换 Basic / Modern Light / Modern Dark，提示颜色同步改变；输入、计数
//     和控件身份保持。拖动 HeaderBar、调整窗口和靠近屏幕边缘，提示定位合理；
//     1x/1.5x/2x 复查文字、折行及圆角无裁剪。
//
// ASSERT PASS 仅检查文字、控件身份及重建时的输入/选区，不代替上述视觉验收。
// 平台差异：字体、抗锯齿、窗口装饰与定位可不同，需分别记录实际结果。
// 副作用：仅测试窗口、计时器和标准输出；不访问剪贴板、不更改系统设置。
// 复位：点击“复位”取消待执行更新并恢复初始选项，保留输入；重启清除全部状态。
// 退出：关闭主窗口；断言失败 panic，进程非零退出。
package main

import (
	"fmt"
	"time"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

const defaultTooltip = "控件构造时的默认提示。\n省略 .Tooltip(...) 后恢复此值，显式空串则不显示。"

// 用有默认提示的普通 Button 验证共享修饰器；不实现另一套提示行为。
type defaultButtonView struct {
	ui.ViewBase[defaultButtonView]
	onClick func()
}

type buttonState struct {
	onClick func()
	handle  signal.Handle
}

func defaultButton(onClick func()) *defaultButtonView {
	v := &defaultButtonView{onClick: onClick}
	v.Self = v
	return v
}

func (v *defaultButtonView) Build() ui.View { return v }
func (v *defaultButtonView) Mount(ctx ui.BuildContext) gui.Widget {
	b := gui.NewButton()
	b.SetTooltip(defaultTooltip)
	s := &buttonState{}
	s.handle = b.ConnectClicked(func() { s.onClick() })
	ctx.SetState(s)
	return b
}
func (v *defaultButtonView) Update(ctx ui.BuildContext, w gui.Widget) {
	ctx.State().(*buttonState).onClick = v.onClick
	ctx.UpdateChild(w.(*gui.Button), ui.Label("提示目标 / Hover, Tab, Click"))
}
func (*defaultButtonView) Unmount(ctx ui.BuildContext, _ gui.Widget) {
	ctx.State().(*buttonState).handle.Disconnect()
}

func main() {
	mode, theme, revision, clicks, rebuilds := 0, 0, 1, 0, 0
	visible, mounted, started := true, true, false
	var target, input gui.Widget
	var removed gui.Widget
	var pending *ui.Timer
	modeNames := []string{"显式文字", "显式空串", "省略修饰器"}
	tooltipText := func() string {
		switch mode {
		case 0:
			return fmt.Sprintf("声明式提示，版本 %d。\n点击目标增加计数；提示不阻止正常输入。这是一段用于检查自动折行的较长说明文字。", revision)
		case 2:
			return defaultTooltip
		}
		return ""
	}
	if err := ui.Run("org.golang-gui.TooltipUITest", func(app ui.App) ui.RootView {
		check := func() {
			w := app.FindWidget("tooltip", "target")
			if mounted {
				if w == nil || w.Tooltip() != tooltipText() || target != nil && target != w || w == removed {
					panic("目标文字、身份或重新挂载不符合预期")
				}
				target = w
			} else if w != nil {
				panic("目标卸载后仍可查询")
			}
			w = app.FindWidget("tooltip", "input")
			if w == nil || input != nil && input != w {
				panic("重建替换了输入框")
			}
			input = w
			fmt.Printf("ASSERT PASS mode=%s revision=%d mounted=%v visible=%v clicks=%d rebuilds=%d input=%q\n",
				modeNames[mode], revision, mounted, visible, clicks, rebuilds, w.(*gui.TextInput).Text())
		}
		// 每次交互更新前保存编辑状态，更新后检查；不控制 Text/Selection。
		rebuild := func(change func()) {
			editor := app.FindWidget("tooltip", "input").(*gui.TextInput)
			beforeText, beforeSelection := editor.Text(), editor.Selection()
			change()
			rebuilds++
			app.RequestUpdate()
			app.Post(func() {
				if editor.Text() != beforeText || editor.Selection() != beforeSelection {
					panic("重建改变了输入文字或选区")
				}
				check()
				fmt.Printf("INPUT PASS text=%q selection=%+v\n", beforeText, beforeSelection)
			})
		}
		update := func() { rebuild(func() { mode = 0; revision++ }) }
		var sheet style.StyleSheet = basic.Sheet()
		if theme != 0 {
			sheet = modern.Sheet(modern.Options{Dark: theme == 2})
		}
		t := defaultButton(func() { rebuild(func() { clicks++ }) }).ID("target").Hidden(!visible)
		if mode != 2 {
			t.Tooltip(tooltipText())
		}
		var content ui.View = t
		if !mounted {
			content = ui.Label("提示目标已卸载")
		}
		if !started {
			started = true
			app.Post(func() {
				check()
				fmt.Println("READY: 手工操作；F6 检查状态，不代表视觉验收通过。")
			})
		}
		inspect := func() {
			check()
			for _, id := range []string{"target", "input", "update", "clear", "omit", "delay", "rebuild", "visible", "mount", "basic", "light", "dark", "reset", "verify"} {
				if w := app.FindWidget("tooltip", id); w != nil {
					fmt.Printf("BOUNDS %s %+v\n", id, w.Snapshot().Bounds)
				}
			}
		}
		return ui.Root().StyleSheet(sheet).Windows(
			ui.Window("tooltip").Title("GOUI Declarative Tooltip").Chrome(ui.WindowChromeIntegrated).Size(800, 560).
				Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(inspect)).Content(ui.VBox(
				ui.HeaderBar(ui.Label("Declarative Tooltip — 手工验收")),
				ui.VBox(
					ui.Label(fmt.Sprintf("模式：%s；版本：%d；点击：%d；重建：%d", modeNames[mode], revision, clicks, rebuilds)),
					ui.HBox(content, ui.Button("禁用按钮").Enabled(false).Tooltip("当前操作不可用；禁用控件仍可显示原因。")).Spacing(12),
					ui.TextInput().ID("input").Placeholder("输入文字并选择一部分，重建应保留文字与选区").Tooltip("普通 TextInput 的共享提示；输入和选区由编辑控件管理。"),
					ui.HWrap(
						ui.Button("更新文字").ID("update").OnClick(update),
						ui.Button("清空提示").ID("clear").OnClick(func() { rebuild(func() { mode = 1 }) }),
						ui.Button("省略修饰器").ID("omit").OnClick(func() { rebuild(func() { mode = 2 }) }),
						ui.Button("延迟更新 (2s)").ID("delay").OnClick(func() {
							if pending != nil {
								pending.Stop()
							}
							pending = app.TimeoutFunc(2*time.Second, update)
						}),
					).Spacing(8).LineSpacing(8),
					ui.HWrap(
						ui.Button("无关重建").ID("rebuild").OnClick(func() { rebuild(func() {}) }),
						ui.Button("显示/隐藏").ID("visible").OnClick(func() { rebuild(func() { visible = !visible }) }),
						ui.Button("挂载/卸载").ID("mount").OnClick(func() {
							rebuild(func() {
								if mounted {
									removed, target = app.FindWidget("tooltip", "target"), nil
								}
								mounted = !mounted
							})
						}),
					).Spacing(8).LineSpacing(8),
					ui.HWrap(
						ui.Button("Basic").ID("basic").OnClick(func() { rebuild(func() { theme = 0 }) }),
						ui.Button("Modern Light").ID("light").OnClick(func() { rebuild(func() { theme = 1 }) }),
						ui.Button("Modern Dark").ID("dark").OnClick(func() { rebuild(func() { theme = 2 }) }),
					).Spacing(8).LineSpacing(8),
					ui.HBox(
						ui.Button("检查 / F6").ID("verify").OnClick(inspect),
						ui.Button("复位").ID("reset").OnClick(func() {
							rebuild(func() {
								if pending != nil {
									pending.Stop()
								}
								mode, theme, revision, clicks, rebuilds = 0, 0, 1, 0, -1
								visible, mounted = true, true
							})
						}),
					).Spacing(8),
					ui.Label("悬停 / Tab → 提示；Escape → 关闭；F6 → 断言与边界日志。"),
				).Padding(16).Spacing(16),
			).CrossAlign(layout.CrossStretch)),
		)
	}); err != nil {
		panic(err)
	}
}
