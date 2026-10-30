// 任意 View 通用修饰的窗口验收，不依赖 DevServer。
// 环境：Linux X11 / Windows / macOS 桌面，记录后端与缩放。
// 启动：go run ./tests/window/ui/modifiers [-auto]；可设置 GOUI_PLAT_PAINTER、GOUI_PLAT_SCALE。
// 初始：Modern 主题，左侧固定说明、右侧 Label；Global=1、文本为空、Visible/Enabled=true。
// 操作与预期：
//  1. 拖动窗口大小：右侧内容占剩余宽度，最小宽度 240 DIP，无额外包装边框或命中区域。
//  2. Cycle 依次切换 Label -> TextInput -> 空 -> Label；每次输出 ID 都是 output，
//     非空时保持宽度下限/权重/Tooltip，Global 不变。输入框可输入中文，内容保存在 State。
//  3. 点击右侧取得焦点，按 F8：Local key 加 1；Label 状态按 Shift+F10，Menu 加 1；
//     Label 上右键也只增加 Menu；TextInput 保留自身编辑菜单优先处理。反复 Rebuild 不重复连接。
//  4. Visible 隐藏/恢复，Enabled 禁用/恢复；隐藏不是卸载，禁用时内部快捷键不执行。
//  5. F6/Check 检查最终属性、Snapshot、窗口真实 Widget 子树，日志 CHECK PASS 并输出按钮边界。
//  6. Destroy on blur 使右侧取得焦点，再点击其他控件：窗口在焦点回调内销毁，安全退出。
//
// -auto 经 Window.DispatchEvent 和 CommitText 检查上述核心状态，最后在应用禁用修饰导致的
// 焦点回调内销毁窗口；日志 UI MODIFIERS PASS。它不替代外部鼠标、系统 IME 和视觉检查。
// 平台差异：字体及原生窗口装饰不同，修饰语义一致。副作用：仅窗口和日志，不修改剪贴板/设置。
// 复位：重新启动。退出：关闭窗口，或操作 6，或 -auto 完成后退出。
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
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

type demo struct {
	app                        ui.App
	auto, started, closeOnBlur bool
	global, step               int
	mode                       ui.State[int]
	visible, enabled           ui.State[bool]
	text                       ui.State[string]
	keys, menus, focuses       ui.State[int]
	retained                   gui.Widget
	timer                      *ui.Timer
}

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func increment(value *ui.State[int]) { value.Update(func(n int) int { return n + 1 }) }
func toggle(value *ui.State[bool])   { value.Update(func(v bool) bool { return !v }) }

// 故意返回接口类型与组合声明，调用方只能通过 Modify 配置输出。
func (d *demo) content() ui.View {
	return ui.Build(func() ui.View {
		switch d.mode.Get() % 3 {
		case 0:
			return ui.Label("Label output — Cycle to edit").MinSize(120, 48)
		case 1:
			return ui.TextInput().BindText(&d.text)
		default:
			return nil
		}
	})
}
func (d *demo) check() {
	win := d.app.FindWindow("main")
	w := gui.FindWidget(win, "output")
	if d.mode.Peek()%3 == 2 {
		require(w == nil, "empty output has a widget")
	} else {
		require(w != nil, "missing output")
		require(w.MainWeight() == 1 && w.MinSize().Width == 240, "layout modifiers lost")
		require(w.Visible() == d.visible.Peek() && w.Enabled() == d.enabled.Peek(), "interaction modifiers lost")
		require(w.Name() == "Modified output" && w.Tooltip() == "Outer tooltip", "semantic modifiers lost")
		// TextInput 自带编辑输入和菜单控制器，外层只管理额外的菜单与快捷键。
		wantControllers, wantMenus := 2, 1
		if _, input := w.(*gui.TextInput); input {
			wantControllers, wantMenus = 4, 2
		}
		shortcuts, menus := 0, 0
		for _, c := range w.EventControllers() {
			switch c.(type) {
			case *gui.ShortcutController:
				shortcuts++
			case *gui.ContextMenuEventController:
				menus++
			}
		}
		require(w.Snapshot().ID == "output" && len(w.EventControllers()) == wantControllers && shortcuts == 1 && menus == wantMenus, "snapshot or controller ownership")
		if d.retained != nil {
			require(w == d.retained, "rebuild replaced same output")
		}
	}
	fmt.Printf("CHECK PASS global=%d mode=%d key=%d menu=%d visible=%v enabled=%v\n", d.global, d.mode.Peek(), d.keys.Peek(), d.menus.Peek(), d.visible.Peek(), d.enabled.Peek())
	for _, id := range []string{"cycle", "visible", "enabled", "rebuild", "arm"} {
		b := gui.FindWidget(win, id).Snapshot().Bounds
		fmt.Printf("BOUNDS %s %.1f %.1f %.1f %.1f\n", id, b.X, b.Y, b.Width, b.Height)
	}
}
func (d *demo) click(id string) {
	win := d.app.FindWindow("main")
	b := gui.FindWidget(win, id).Snapshot().Bounds
	p := geometry.Point{X: b.X + b.Width/2, Y: b.Y + b.Height/2}
	for _, kind := range []events.EventType{events.PointerMove, events.PointerDown, events.PointerUp} {
		err := win.DispatchEvent(events.PointerEvent{EventType: kind, Position: p, Button: events.PointerButtonLeft})
		if err != nil {
			panic(err)
		}
	}
}
func (d *demo) automate() {
	win := d.app.FindWindow("main")
	switch d.step {
	case 0:
		d.check()
		d.click("cycle")
	case 1:
		_, ok := gui.FindWidget(win, "output").(*gui.TextInput)
		require(ok, "type replacement")
		d.click("output")
		if err := win.CommitText("modified 中文"); err != nil {
			panic(err)
		}
		for _, e := range []events.KeyEvent{{EventType: events.KeyDown, Key: events.KeyF8}} {
			if err := win.DispatchEvent(e); err != nil {
				panic(err)
			}
		}
	case 2:
		d.check()
		require(d.keys.Peek() == 1 && d.menus.Peek() == 0 && d.text.Peek() == "modified 中文", "text or local callbacks")
		d.click("cycle")
	case 3:
		d.check()
		d.click("cycle")
	case 4:
		d.check()
		d.click("output")
		if err := win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyF10, Modifiers: events.ModifierShift}); err != nil {
			panic(err)
		}
		require(d.menus.Peek() == 1, "label context callback")
		d.click("visible")
	case 5:
		d.check()
		require(!d.visible.Peek(), "not hidden")
		d.click("visible")
	case 6:
		d.click("enabled")
	case 7:
		d.check()
		require(!d.enabled.Peek(), "not disabled")
		d.click("enabled")
	case 8:
		d.retained = gui.FindWidget(win, "output")
		d.click("rebuild")
	case 9:
		d.check()
		require(d.global == 2, "global rebuild count")
		require(win.SetFocusedWidget(gui.FindWidget(win, "output")), "cannot focus output")
		d.closeOnBlur = true
		d.enabled.Set(false)
	default:
		panic("focus-loss destruction did not finish")
	}
	d.step++
}
func (d *demo) build(app ui.App) ui.RootView {
	d.app = app
	d.global++
	if !d.started {
		d.started = true
		if d.auto {
			d.timer = app.TickFunc(400*time.Millisecond, d.automate)
		}
	}
	return ui.Root().StyleSheet(modern.FromSettings(app.Settings())).Windows(
		ui.Window("main").Title("GOUI View modifiers").Size(780, 360).Center(true).
			Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(d.check)).OnDestroy(func() {
			if d.auto {
				require(d.closeOnBlur && !d.enabled.Peek(), "unexpected close")
				d.timer.Stop()
				fmt.Println("UI MODIFIERS PASS")
			}
		}).Content(ui.VBox(
			ui.HBox(
				ui.Button("Cycle").ID("cycle").OnClick(func() { d.retained = nil; increment(&d.mode) }),
				ui.Button("Visible").ID("visible").OnClick(func() { toggle(&d.visible) }),
				ui.Button("Enabled").ID("enabled").OnClick(func() { toggle(&d.enabled) }),
				ui.Button("Rebuild").ID("rebuild").OnClick(func() { d.retained = app.FindWidget("main", "output"); app.RequestUpdate() }),
				ui.Button("Check").OnClick(d.check),
				ui.Button("Destroy on blur").ID("arm").OnClick(func() {
					d.closeOnBlur = true
					app.FindWindow("main").SetFocusedWidget(app.FindWidget("main", "output"))
				}),
			).Spacing(8),
			ui.Build(func() ui.View {
				return ui.Label(fmt.Sprintf("Global %d · Local key %d · Menu %d · Focus %d", d.global, d.keys.Get(), d.menus.Get(), d.focuses.Get()))
			}),
			ui.HBox(ui.Label("Fixed sibling").MinWidth(140), ui.Build(func() ui.View {
				return ui.Modify(d.content()).ID("output").Name("Modified output").Tooltip("Outer tooltip").
					MinWidth(240).MainWeight(1).Focusable(true).Cursor(ui.CursorPointing).
					Visible(d.visible.Get()).Enabled(d.enabled.Get()).
					OnFocus(func(focused bool) {
						increment(&d.focuses)
						if !focused && d.closeOnBlur {
							app.FindWindow("main").Destroy()
						}
					}).
					OnContextMenu(func(ctx gui.EventContext) { increment(&d.menus); ctx.StopPropagation() }).
					Shortcuts(ui.Shortcut(ui.KeyF8).OnActivate(func() { increment(&d.keys) }))
			})).MainWeight(1).Spacing(16).CrossAlign(layout.CrossStretch),
			ui.Label("Cycle: Label → TextInput → empty. Focus output: F8 / Shift+F10. F6 checks."),
		).Padding(20).Spacing(14).CrossAlign(layout.CrossStretch)),
	)
}
func main() {
	auto := flag.Bool("auto", false, "自动断言后退出")
	flag.Parse()
	d := &demo{auto: *auto, visible: ui.MakeState(true), enabled: ui.MakeState(true)}
	if err := ui.Run("org.golang-gui.ViewModifiers", d.build); err != nil {
		log.Fatal(err)
	}
}
