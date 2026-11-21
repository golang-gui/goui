// 声明式 CommandGroup、菜单与快捷键的窗口验证。
//
// 环境：桌面、字体和原生事件循环；Linux 在独立 X11／窗口管理器中运行。
// 启动：go run ./tests/window/ui/commands；-auto 自动派发正常输入并检查退出。
// -basic/-dark 选择外观；GOUI_PLAT_PAINTER、GOUI_PLAT_SCALE 选择后端与倍率。
// 初始：外层 copy 启用，局部菜单由内层同名 copy 遮蔽；两个计数为 0。
// 操作与预期：
//  1. 打开“编辑”，F6 禁用／恢复 copy，菜单保持打开并实时灰显／恢复。
//  2. F7 在 Ctrl/Cmd+C 与 Ctrl/Cmd+Shift+C 间切换，打开的提示立即变化且保持列对齐。
//     关闭菜单后新按键响应，旧按键不响应；点击 Copy 与按键均恰好执行一次。
//  3. “切换菜单入口”和“切换快捷键入口”只限制各自入口，不改变 copy 的 Enabled。
//  4. “局部”菜单默认增加局部计数；F9 删除内层定义后回退到外层，菜单增加外层计数。
//     再按 F9 恢复遮蔽，外层与内层对象互不污染。
//  5. F8/Check 输出构建次数、计数与 BOUNDS；局部设置不重建 Window，未变化菜单模型
//     和命令对象保留。正常使用时点击按钮，自动模式用相同事件路径触发控制操作。
//  6. 外层 copy 由根控件的 Shortcut.DefineCommand 提供，菜单按 ID 引用；内层由
//     MenuItem.DefineCommand 提供。BindEnabled 变化不重建其余内容，F9 只更新局部菜单。
//     声明不生成额外 Widget、空行或容器间距，也不依赖菜单是否打开。
//  7. “External”菜单及窗口 F12 借用同一 gui.Command；F4 切换其启用状态，打开的
//     菜单实时灰显，copy 不受影响。F12 和菜单各增加外部计数一次。
//  8. 窗口默认分组不需要显式包裹 CommandGroup；“Window”操作在 Window.Commands
//     声明，窗口 F3 和菜单按 ID 引用。F5 切换该命令启用状态，菜单实时更新。
//     Check 使用 App.FindCommand(windowID, commandID) 查询，不要求 BuildContext。
//  9. External 与 Window 初始未勾选；菜单、F12／F3 均由同一处理函数提交勾选状态。
//     禁用后保留勾号；F2 直接发布 Window 状态，打开菜单实时更新且不增加执行计数。
//     普通项与勾选项标题对齐，勾号不随字体改变，切换状态不改变菜单宽度。
//
// 自动场景还检查重复关联、最新操作回调、删除后的回退、重开菜单和绘图后端／倍率。
// 自动派发不替代原生菜单 grab、IME 和桌面输入；不使用 DevServer 或语义动作捷径。
// 副作用：创建窗口、焦点和 stdout；不修改用户文件、设置或剪贴板。
// 复位：重启；退出：关闭窗口或自动 PASS。自动模式提前关闭返回失败。
package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}

type inputTarget interface{ DispatchEvent(events.Event) error }

func click(target inputTarget, p geometry.Point) {
	for _, kind := range []events.EventType{events.PointerMove, events.PointerDown, events.PointerUp} {
		must(target.DispatchEvent(events.PointerEvent{EventType: kind, Position: p, Button: events.PointerButtonLeft}))
	}
}
func key(target inputTarget, k events.Key, mods events.Modifiers) {
	must(target.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: k, Modifiers: mods}))
}
func findRow(info gui.WidgetInfo, text string) (gui.WidgetInfo, bool) {
	if (info.Role == gui.RoleMenuItem || info.Role == gui.RoleMenuItemCheckbox) && info.Text == text {
		return info, true
	}
	for _, child := range info.Children {
		if found, ok := findRow(child, text); ok {
			return found, true
		}
	}
	return gui.WidgetInfo{}, false
}

type painterProbe struct {
	gui.WidgetBase
	backend string
	scale   float32
}

func (p *painterProbe) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: 1, Height: 1}))
}

func (p *painterProbe) Paint(draw gui.Painter) {
	if p.backend != "" {
		return
	}
	img, err := draw.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	must(err)
	p.backend = fmt.Sprintf("%T", img)
	p.scale = draw.PixelScale()
	img.Destroy()
	requested := os.Getenv("GOUI_PLAT_PAINTER")
	if requested != "" {
		require(strings.Contains(strings.ToLower(p.backend), requested), "requested painter fell back")
	}
	fmt.Printf("PAINTER %s scale=%g\n", p.backend, p.scale)
}

type probeView struct {
	ui.ViewBase[probeView]
	probe *painterProbe
}

func newProbe(p *painterProbe) *probeView              { v := &probeView{probe: p}; v.Self = v; return v }
func (v *probeView) Mount(ui.BuildContext) gui.Widget  { return v.probe }
func (*probeView) Update(ui.BuildContext, gui.Widget)  {}
func (*probeView) Unmount(ui.BuildContext, gui.Widget) {}

func main() {
	auto := flag.Bool("auto", false, "自动检查并退出")
	basic := flag.Bool("basic", false, "Basic 外观")
	dark := flag.Bool("dark", false, "Modern 暗色")
	flag.Parse()
	enabled, shifted, override := ui.MakeState(true), ui.MakeState(false), ui.MakeState(true)
	itemEnabled, bindingEnabled := ui.MakeState(true), ui.MakeState(true)
	windowEnabled := ui.MakeState(true)
	windowChecked := ui.MakeState(false)
	status := ui.MakeState("外层：0　局部：0")
	outerCount, innerCount, global, local := 0, 0, 0, 0
	externalCount, definitionBuilds, buildsBeforeAvailability := 0, 0, 0
	buildsBeforeDefinition, definitionsBeforeUpdate := 0, 0
	windowCount := 0
	external := gui.NewCommand()
	external.SetCheckable(true)
	report := func() { status.Set(fmt.Sprintf("外层：%d　局部：%d", outerCount, innerCount)) }
	external.ConnectActivate(func() {
		externalCount++
		external.SetChecked(!external.Checked())
		status.Set(fmt.Sprintf("外部：%d", externalCount))
	})
	toggleExternal := func() { external.SetEnabled(!external.Enabled()) }
	toggleWindow := func() { windowEnabled.Set(!windowEnabled.Get()) }
	publishWindow := func() { windowChecked.Set(!windowChecked.Get()) }
	p := new(painterProbe)
	started, complete, step := false, false, 0
	var app ui.App
	var original *gui.Command
	var originalModel gui.MenuModel
	toggle := func() { enabled.Set(!enabled.Get()) }
	change := func() { shifted.Set(!shifted.Get()) }
	toggleOverride := func() { override.Set(!override.Get()) }
	toggleItem := func() { itemEnabled.Set(!itemEnabled.Get()) }
	toggleBinding := func() { bindingEnabled.Set(!bindingEnabled.Get()) }
	menu := func(id string) *gui.MenuButton { return app.FindWidget("commands", id).(*gui.MenuButton) }
	check := func() {
		m := menu("edit").Menu()
		command := m.ItemAt(0).Command()
		require(command != nil && (original == nil || original == command), "command identity changed")
		require(len(command.Shortcuts()) == 1, "duplicate or missing shortcut association")
		require(app.FindCommand("commands", "copy") == command, "public lookup did not resolve window default group")
		require(app.FindCommand("commands", "window.run") == m.ItemAt(3).Command(), "public lookup did not resolve Window.Commands")
		original = command
		if originalModel == nil {
			originalModel = m
		}
		fmt.Printf("CHECK global=%d local=%d definitions=%d outer=%d inner=%d external=%d checked=%t enabled=%t window=%d checked=%t\n", global, local, definitionBuilds, outerCount, innerCount, externalCount, external.Checked(), command.Enabled(), windowCount, app.FindCommand("commands", "window.run").Checked())
		for _, id := range []string{"edit", "inner", "enable", "gesture", "scope", "menu-entry", "binding", "check"} {
			b := app.FindWidget("commands", id).Snapshot().Bounds
			fmt.Printf("BOUNDS %s %.1f %.1f %.1f %.1f\n", id, b.X, b.Y, b.Width, b.Height)
		}
	}
	err := ui.Run("org.golanggui.UICommandVerification", func(current ui.App) ui.RootView {
		app = current
		global++
		if !started {
			started = true
			if *auto {
				var timer *ui.Timer
				var checkBounds geometry.Rectangle
				var checkModel gui.MenuModel
				deadline := time.Now().Add(45 * time.Second)
				timer = app.TickFunc(400*time.Millisecond, func() {
					require(time.Now().Before(deadline), "automatic verification timed out")
					w := app.FindWindow("commands")
					pop := func() gui.PopoverTarget { require(len(w.Popovers()) == 1, "expected one menu"); return w.Popovers()[0] }
					row := func() gui.WidgetInfo {
						r, ok := findRow(pop().Widget().Snapshot(), "Copy")
						require(ok, "missing Copy row")
						return r
					}
					fmt.Printf("STEP %d popovers=%d edit=%t inner=%t\n", step, len(w.Popovers()), menu("edit").Opened(), menu("inner").Opened())
					switch step {
					case 0:
						if p.backend == "" || menu("edit").Snapshot().Bounds.Width <= 0 {
							return
						}
						check()
						click(w, menu("edit").Snapshot().Bounds.Center())
					case 1:
						require(row().Enabled, "initial menu disabled")
						buildsBeforeAvailability = local
						key(w, events.KeyF6, 0)
					case 2:
						require(!row().Enabled && menu("edit").Opened(), "local update did not disable open menu")
						require(menu("edit").Menu() == originalModel, "command state reloaded unchanged menu")
						require(local == buildsBeforeAvailability, "BindEnabled rebuilt content")
						key(w, events.KeyF6, 0)
					case 3:
						require(row().Enabled, "open menu did not reenable")
						require(local == buildsBeforeAvailability, "BindEnabled reenable rebuilt content")
						key(w, events.KeyF7, 0)
					case 4:
						expected := original.Shortcuts()[0].Gesture().DisplayText()
						require(row().Attributes[gui.MenuItemInfoKey].(gui.MenuItemInfo).Shortcut == expected, "hint not updated after local build")
						click(pop(), row().Bounds.Center())
					case 5:
						require(outerCount == 1 && !menu("edit").Opened(), "menu activation duplicated or missing")
						mods, _ := original.Shortcuts()[0].Gesture().Modifiers.Resolve()
						key(w, events.KeyC, mods)
					case 6:
						require(outerCount == 2, "shortcut did not use shared command")
						click(w, menu("inner").Snapshot().Bounds.Center())
					case 7:
						require(menu("inner").Menu().ItemAt(0).Command() != original, "inner command did not shadow outer")
						click(pop(), row().Bounds.Center())
					case 8:
						require(innerCount == 1 && outerCount == 2, "inner command used wrong callback")
						buildsBeforeDefinition, definitionsBeforeUpdate = local, definitionBuilds
						key(w, events.KeyF9, 0)
					case 9:
						require(menu("inner").Menu().ItemAt(0).Command() == original, "removed definition did not fall back")
						require(local == buildsBeforeDefinition && definitionBuilds == definitionsBeforeUpdate+1, "definition update did not stay local")
						click(w, menu("inner").Snapshot().Bounds.Center())
					case 10:
						click(pop(), row().Bounds.Center())
					case 11:
						require(outerCount == 3 && innerCount == 1, "fallback command did not run")
						key(w, events.KeyF10, 0)
					case 12:
						click(w, menu("edit").Snapshot().Bounds.Center())
					case 13:
						require(!row().Enabled && original.Enabled(), "local menu enablement leaked into command")
						key(w, events.KeyEscape, 0)
						mods, _ := original.Shortcuts()[0].Gesture().Modifiers.Resolve()
						key(w, events.KeyC, mods)
					case 14:
						require(outerCount == 4, "menu disable blocked shortcut")
						key(w, events.KeyF10, 0)
						key(w, events.KeyF11, 0)
					case 15:
						mods, _ := original.Shortcuts()[0].Gesture().Modifiers.Resolve()
						key(w, events.KeyC, mods)
						click(w, menu("edit").Snapshot().Bounds.Center())
					case 16:
						require(outerCount == 4 && row().Enabled, "binding disable leaked into command")
						click(pop(), row().Bounds.Center())
					case 17:
						require(outerCount == 5 && innerCount == 1 && global == 1, "incorrect counts or global rebuild")
						key(w, events.KeyF12, 0)
					case 18:
						require(externalCount == 1 && external.Checked(), "window shortcut did not publish external state")
						click(w, menu("edit").Snapshot().Bounds.Center())
					case 19:
						key(w, events.KeyF4, 0)
					case 20:
						ext, ok := findRow(pop().Widget().Snapshot(), "External")
						require(ok && !ext.Enabled && row().Enabled, "external enabled state did not update independently")
						require(ext.Role == gui.RoleMenuItemCheckbox && ext.Attributes[gui.MenuItemInfoKey].(gui.MenuItemInfo).Checked, "disabled external lost checked state")
						key(w, events.KeyF4, 0)
					case 21:
						ext, ok := findRow(pop().Widget().Snapshot(), "External")
						require(ok && ext.Enabled, "external menu did not reenable")
						click(pop(), ext.Bounds.Center())
					case 22:
						require(externalCount == 2 && !external.Checked() && len(external.Shortcuts()) == 1, "external menu duplicated or lost state publication")
						key(w, events.KeyF3, 0)
					case 23:
						require(windowCount == 1, "window ID shortcut did not activate default command")
						key(w, events.KeyF5, 0)
					case 24:
						click(w, menu("edit").Snapshot().Bounds.Center())
					case 25:
						item, ok := findRow(pop().Widget().Snapshot(), "Window")
						require(ok && !item.Enabled && row().Enabled, "Window.Commands binding did not update independently")
						require(item.Role == gui.RoleMenuItemCheckbox && item.Attributes[gui.MenuItemInfoKey].(gui.MenuItemInfo).Checked, "disabled window lost bound checked state")
						key(w, events.KeyF5, 0)
					case 26:
						item, ok := findRow(pop().Widget().Snapshot(), "Window")
						require(ok && item.Enabled, "window command did not reenable")
						click(pop(), item.Bounds.Center())
					case 27:
						require(windowCount == 2 && global == 1 && local == 4, "window command rebuilt unrelated content or duplicated activation")
						require(!app.FindCommand("commands", "window.run").Checked(), "menu activation did not commit bound state")
						click(w, menu("edit").Snapshot().Bounds.Center())
					case 28:
						if len(w.Popovers()) != 1 || !menu("edit").Opened() {
							return
						}
						checkBounds = pop().Widget().Snapshot().Bounds
						checkModel = menu("edit").Menu()
						key(w, events.KeyF2, 0)
					case 29:
						item, ok := findRow(pop().Widget().Snapshot(), "Window")
						if !ok || !item.Attributes[gui.MenuItemInfoKey].(gui.MenuItemInfo).Checked {
							return
						}
						require(windowCount == 2 && global == 1 && local == 4, "program publication activated or rebuilt content")
						require(pop().Widget().Snapshot().Bounds == checkBounds && menu("edit").Menu() == checkModel, fmt.Sprintf("checked publication changed geometry or model: before=%v after=%v same-model=%t", checkBounds, pop().Widget().Snapshot().Bounds, menu("edit").Menu() == checkModel))
						key(w, events.KeyEscape, 0)
						key(w, events.KeyF3, 0)
					case 30:
						if app.FindCommand("commands", "window.run").Checked() {
							return
						}
						require(windowCount == 3 && global == 1 && local == 4, "shortcut did not use shared checked operation")
						check()
						require(p.backend != "", "no actual paint")
						complete = true
						fmt.Printf("ASSERT PASS UI commands global=%d local=%d backend=%s scale=%g\n", global, local, p.backend, p.scale)
						timer.Stop()
						app.Quit()
					}
					step++
				})
			}
		}
		content := ui.Build(func() ui.View {
			local++
			mods := ui.ModPrimary
			if shifted.Get() {
				mods |= ui.ModShift
			}
			inner := ui.CommandGroup(ui.Build(func() ui.View {
				definitionBuilds++
				item := ui.MenuItem("Copy").Command("copy")
				if override.Get() {
					item.DefineCommand(ui.Command("copy").OnActivate(func() { innerCount++; report() }))
				}
				return ui.MenuButton("局部").ID("inner").Menu(item)
			}))
			return ui.VBox(
				ui.HBox(ui.MenuButton("编辑").ID("edit").Menu(ui.MenuItem("Copy").Command("copy").Enabled(itemEnabled.Get()), ui.MenuItem("没有快捷键的长菜单项", func() {}), ui.MenuItem("External").CommandRef(external), ui.MenuItem("Window").Command("window.run")), inner).Spacing(12),
				ui.Label("F2：发布勾选　F3/F12：勾选操作　F5/F4：启用"), ui.Label().BindText(&status),
				ui.Button("切换操作启用").ID("enable").OnClick(toggle),
				ui.Button("修改快捷键").ID("gesture").OnClick(change),
				ui.Button("切换局部定义").ID("scope").OnClick(toggleOverride),
				ui.Button("切换菜单入口").ID("menu-entry").OnClick(toggleItem),
				ui.Button("切换快捷键入口").ID("binding").OnClick(toggleBinding),
				ui.Button("Check").ID("check").OnClick(check), newProbe(p),
			).Spacing(10).Padding(16).Shortcuts(ui.Shortcut(ui.KeyC).Modifiers(mods).Enabled(bindingEnabled.Get()).DefineCommand(ui.Command("copy").BindEnabled(&enabled).OnActivate(func() { outerCount++; report() })))
		})
		win := ui.Window("commands").Title("GOUI UI CommandGroup").Size(620, 490).Center(true).Content(content).Shortcuts(
			ui.Shortcut(ui.KeyF6).OnActivate(toggle), ui.Shortcut(ui.KeyF7).OnActivate(change), ui.Shortcut(ui.KeyF8).OnActivate(check), ui.Shortcut(ui.KeyF9).OnActivate(toggleOverride), ui.Shortcut(ui.KeyF10).OnActivate(toggleItem), ui.Shortcut(ui.KeyF11).OnActivate(toggleBinding),
			ui.Shortcut(ui.KeyF4).OnActivate(toggleExternal), ui.Shortcut(ui.KeyF12).CommandRef(external),
			ui.Shortcut(ui.KeyF3).Command("window.run"), ui.Shortcut(ui.KeyF5).OnActivate(toggleWindow),
			ui.Shortcut(ui.KeyF2).OnActivate(publishWindow),
		).Commands(ui.Command("window.run").BindEnabled(&windowEnabled).Checkable(true).BindChecked(&windowChecked).OnActivate(func() {
			windowCount++
			windowChecked.Set(!windowChecked.Get())
			status.Set(fmt.Sprintf("窗口操作：%d", windowCount))
		}))
		root := ui.Root().Windows(win)
		if !*basic {
			root = root.StyleSheet(modern.Sheet(modern.Options{Dark: *dark}))
		}
		return root
	})
	must(err)
	if *auto {
		require(complete, "automatic verification exited early")
		require(external.Enabled() && external.Checkable() && !external.Checked() && len(external.Shortcuts()) == 0, "unmount mutated or retained external command association")
	}
}
