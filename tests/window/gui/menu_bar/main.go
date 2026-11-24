// MenuBar 的窗口与系统菜单验收，需要真实桌面。
// 启动：go run ./tests/window/gui/menu_bar；-dark 切换初始 Modern 主题，-basic 使用 Basic。
// Linux/Windows 的 Auto 在窗口内，macOS Auto 在系统菜单栏；强制 InWindow 均在窗口内。
// 初始：A/B 两个不同来源窗口，各有 File/View；Run、Checked、禁用项和三级子菜单。
// 操作：点击 Run 或 Primary+1，每次计数恰好增加一次；勾选后重开保留状态，禁用项
// 不执行。使用 Latest 更新闭包，Run 显示最新代次；Language 切换顶层和子项语言。
// 在 A/B 之间切换，系统菜单只执行当前窗口动作；点击 Show empty，系统仅留基本应用
// 菜单；关闭无菜单窗口应恢复实际主窗口来源。普通 Popover 展开不抢走菜单来源。
// Mode 切换后窗口不留菜单高度/spacing，切回仍可用；Hidden 隐藏整个菜单祖先后来源
// 撤下，再次显示恢复。Arm update 后立即展开菜单并保持，1.5 秒后状态日志须在菜单
// 仍展开时出现；跟踪期结构更新在安全关闭后可见，旧动作不得按新索引执行。
// Dark/Language 不重建来源；Check 输出可观察的计数、请求/实际呈现、动作代次和边界。
// -auto 只在窗口内模式自动派发正常窗口输入，验证一次执行、最新闭包、模式恢复；
// macOS Auto 的系统菜单必须通过原生键鼠另验，裸可执行文件与 .app 分别记录。
// -deny-close 拒绝普通窗口关闭；macOS 菜单 Quit／Command+Q／Dock Quit 仍须完成退出。
// -no-window 不创建窗口；macOS 基本应用菜单的一次 Quit 仍须完成退出。
// 所有退出路径都要求 Shutdown 恰好一次，通知前定时器失活，Run 返回后无剩余宿主，
// 并输出 PASS application cleanup；不能只按进程消失判定通过。
// 副作用：本进程窗口、菜单和 stdout，不改文件/剪贴板/系统设置。关闭所有窗口或
// 应用 Quit 正常退出；重启复位。自动场景提前关闭/超时失败，正常完成输出 PASS。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func main() {
	auto := flag.Bool("auto", false, "自动验证窗口内菜单")
	darkFlag := flag.Bool("dark", false, "初始暗色")
	basicMode := flag.Bool("basic", false, "使用 Basic 主题")
	noWindow := flag.Bool("no-window", false, "不创建窗口，验证原生退出清理")
	keys := flag.Bool("keys", false, "逐项验证菜单等价键与正常输入的一致性")
	denyClose := flag.Bool("deny-close", false, "拒绝窗口关闭，验证应用退出仍完成清理")
	flag.Parse()
	if *keys {
		verifyMenuKeys()
		return
	}
	if *noWindow {
		verifyQuitWithoutWindow()
		return
	}
	modes := [2]gui.MenuBarMode{gui.MenuBarAuto, gui.MenuBarAuto}
	if *auto {
		modes = [2]gui.MenuBarMode{gui.MenuBarInWindow, gui.MenuBarInWindow}
	}
	actions := [2]int{}
	generation := [2]int{1, 1}
	last := [2]int{}
	checked := [2]bool{}
	hidden := [2]bool{}
	extra := [2]bool{}
	dark, chinese, showEmpty, started, complete := *darkFlag, false, false, false, false
	opened := [2]bool{true, true}
	rendered := [2]bool{}
	errors := 0
	shutdowns := 0
	step := 0
	deadline := time.Now().Add(30 * time.Second)
	var app ui.App
	label := func(en, zh string) string {
		if chinese {
			return zh
		}
		return en
	}
	bar := func(id string) *gui.MenuBar {
		w := app.FindWidget(id, "bar")
		if w == nil {
			return nil
		}
		return w.(*gui.MenuBar)
	}
	check := func() {
		for _, id := range []string{"A", "B"} {
			b := bar(id)
			if b == nil {
				continue
			}
			require(b != nil, "missing menu bar")
			data, err := json.Marshal(map[string]any{"window": id, "presentation": b.Snapshot().Attributes[gui.MenuBarInfoKey], "bounds": b.Snapshot().Bounds, "actions": actions, "last": last, "generation": generation, "errors": errors})
			must(err)
			fmt.Println("CHECK", string(data))
		}
		require(errors == 0, "unexpected menu error")
	}
	err := ui.Run("org.golang-gui.MenuBarVerification", func(a ui.App) ui.RootView {
		app = a
		if !started {
			started = true
			timer := gui.App.NewTimer()
			must(timer.Start(time.Hour))
			gui.App.ConnectShutdown(func() {
				shutdowns++
				require(!timer.Active() && timer.Start(time.Hour) != nil, "timer survived shutdown")
				fmt.Println("SHUTDOWN", shutdowns)
			})
			if *auto {
				app.TickFunc(100*time.Millisecond, func() {
					require(time.Now().Before(deadline), "window menu timeout")
					host := app.FindWindow("A")
					b := bar("A")
					if step == 3 {
						modes[0] = gui.MenuBarInWindow
						app.RequestUpdate()
						step++
						return
					}
					if host != nil && !host.Focused() {
						must(host.Show())
						return
					}
					if host == nil || b == nil || b.Rect().Height == 0 {
						return
					}
					group := b.Children()[0]
					if len(group.Children()) == 0 {
						return
					}
					button := group.Children()[0].(*gui.MenuButton)
					bounds := button.Snapshot().Bounds
					if bounds.Width == 0 {
						return
					}
					// 子节点可能已有测量期间的临时矩形，祖先尚未完成分配。
					// 等待当前窗口坐标真正命中按钮，再验证正常输入的结果。
					ready := false
					for hit := gui.Pick(host.Widget(), bounds.Center()); hit != nil; hit = hit.Parent() {
						if hit == button {
							ready = true
							break
						}
					}
					if !ready {
						return
					}
					click := func() {
						point := bounds.Center()
						for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
							must(host.DispatchEvent(events.PointerEvent{EventType: kind, Position: point, Button: events.PointerButtonLeft}))
						}
					}
					key := func(k events.Key) {
						for _, kind := range []events.EventType{events.KeyDown, events.KeyUp} {
							must(host.DispatchEvent(events.KeyEvent{EventType: kind, Key: k}))
						}
					}
					switch step {
					case 0:
						click()
						require(button.Opened(), "menu did not open")
						key(events.KeyHome)
						key(events.KeyEnter)
						require(actions[0] == 1, "menu action not once")
					case 1:
						generation[0]++
						app.RequestUpdate()
					case 2:
						click()
						key(events.KeyHome)
						key(events.KeyEnter)
						require(actions[0] == 2 && last[0] == 2, "stale action callback")
						modes[0] = gui.MenuBarAuto
						app.RequestUpdate()
					case 3:
						modes[0] = gui.MenuBarInWindow
						app.RequestUpdate()
					case 4:
						require(!b.Snapshot().Attributes[gui.MenuBarInfoKey].(gui.MenuBarInfo).Native, "InWindow did not restore")
						check()
						complete = true
						fmt.Println("PASS window menu bar")
						app.Quit()
					}
					step++
				})
			}
		}
		windows := make([]ui.WindowView, 0, 3)
		for i, id := range []string{"A", "B"} {
			if !opened[i] {
				continue
			}
			current := generation[i]
			entries := []*ui.MenuItemView{
				ui.MenuItem(label("Run "+id, "运行 "+id)).DefineCommand(ui.Command("run").OnActivate(func() {
					actions[i]++
					last[i] = current
					fmt.Printf("ACTION %s count=%d generation=%d\n", id, actions[i], current)
					app.RequestUpdate()
				})),
				ui.MenuItem(label("Checked", "勾选")).DefineCommand(ui.Command("checked").Checkable(true).Checked(checked[i]).OnActivate(func() { checked[i] = !checked[i]; app.RequestUpdate() })),
				ui.MenuItem(label("Disabled", "已禁用"), func() { panic("disabled menu activated") }).Enabled(false),
				ui.MenuSeparator(),
				ui.MenuItem(label("Recent", "最近")).Submenu(ui.MenuItem(label("More", "更多")).Submenu(ui.MenuItem(label("Nested action", "深层动作"), func() { fmt.Println("NESTED", id) }))),
			}
			if extra[i] {
				entries = append(entries, ui.MenuItem(label("Added during tracking", "跟踪期间新增"), func() { fmt.Println("ADDED", id) }))
			}
			content := ui.VBox(
				ui.VBox(ui.MenuBar(ui.MenuItem(label("File "+id, "文件 "+id)).Submenu(entries...), ui.MenuItem(label("View", "视图")).Submenu(ui.MenuItem("Dark", func() { dark = !dark; app.RequestUpdate() }))).ID("bar").Mode(modes[i]).OnError(func(err error) { errors++; fmt.Println("ERROR", err) })).ID("ancestor").Visible(!hidden[i]),
				ui.Label(fmt.Sprintf("Window %s · actions %d · latest %d/%d", id, actions[i], last[i], generation[i])),
				ui.HBox(ui.Button("Latest").OnClick(func() { generation[i]++; app.RequestUpdate() }), ui.Button("Mode").OnClick(func() {
					if modes[i] == gui.MenuBarAuto {
						modes[i] = gui.MenuBarInWindow
					} else {
						modes[i] = gui.MenuBarAuto
					}
					app.RequestUpdate()
				}), ui.Button("Hidden").OnClick(func() { hidden[i] = !hidden[i]; app.RequestUpdate() })),
				ui.HBox(ui.Button("Language").OnClick(func() { chinese = !chinese; app.RequestUpdate() }), ui.Button("Dark").OnClick(func() { dark = !dark; app.RequestUpdate() }), ui.Button("Show empty").OnClick(func() { showEmpty = true; app.RequestUpdate() })),
				ui.HBox(ui.Button("Arm update").OnClick(func() {
					fmt.Println("ARM", id)
					app.TimeoutFunc(1500*time.Millisecond, func() { extra[i] = !extra[i]; fmt.Println("TRACKING UPDATE", id); app.RequestUpdate() })
				}), ui.Button("Check").OnClick(check)),
				ui.TextInput().Placeholder("Typing / shortcuts / menu focus"),
				ui.Label("Primary+1: Run; close all windows or use application Quit."),
			).Spacing(8).Padding(12)
			windows = append(windows, ui.Window(id).Title("MenuBar "+id).OnCloseRequest(func(allow *bool) {
				if *denyClose {
					*allow = false
					fmt.Println("VETO WINDOW CLOSE", id)
				}
			}).OnAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
				if !rendered[i] {
					rendered[i] = true
					img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
					must(err)
					fmt.Printf("PAINTER %s %T scale=%g\n", id, img, p.PixelScale())
					img.Destroy()
				}
			}).OnDestroy(func() { opened[i] = false }).Size(580, 360).Shortcuts(ui.Shortcut(ui.Key1).Modifiers(ui.ModPrimary).Command("run")).Content(content))
		}
		if showEmpty {
			windows = append(windows, ui.Window("empty").Title("No menu source").OnDestroy(func() { showEmpty = false }).Size(350, 150).Content(ui.Label("Only the basic application menu should remain.")))
		}
		sheet := modern.Sheet(modern.Options{Dark: dark})
		if *basicMode {
			sheet = basic.Sheet()
		}
		return ui.Root().StyleSheet(sheet).Windows(windows...)
	})
	must(err)
	if *auto {
		require(complete, "automatic scenario interrupted")
	}
	require(shutdowns == 1 && len(gui.App.Surfaces()) == 0, "application cleanup incomplete")
	fmt.Println("PASS application cleanup")
}

func verifyQuitWithoutWindow() {
	app, err := gui.NewApplication("org.golang-gui.MenuBarVerification")
	must(err)
	shutdowns := 0
	var timer *gui.Timer
	app.ConnectActivate(func() {
		timer = app.NewTimer()
		must(timer.Start(time.Hour))
		fmt.Println("READY no-window quit")
	})
	app.ConnectShutdown(func() {
		shutdowns++
		require(timer != nil && !timer.Active() && timer.Start(time.Hour) != nil, "timer survived shutdown")
		fmt.Println("SHUTDOWN", shutdowns)
	})
	deadline := time.AfterFunc(3*time.Minute, func() {
		app.Post(func() { panic("native quit timed out") })
	})
	defer deadline.Stop()
	must(app.Run())
	require(shutdowns == 1 && len(app.Surfaces()) == 0, "application cleanup incomplete")
	fmt.Println("PASS application cleanup")
}

// -keys：输入框自动聚焦；每步完成后点 Next。前两步分别按一次 Primary+1；
// 第 2/3 步按一次 Primary+1，再点 Repeat 注入三个原生重复事件；也可用真实键盘按住。
// 仅 macOS 提供 Repeat 注入，其他平台须观察到实际 Repeat。第 4 步输入一个空格；
// 第 5 步按 F6；第 6 步点 Arm rename 后展开 File，等 RENAMED，再点重命名条目。
// 完成原生模式后再逐步完成 InWindow 模式，最后输出 PASS menu keys。
// 每步都有计数、文本或身份断言；没有观察到实际重复事件不得算重复策略通过。
// 十二分钟超时，提前退出判失败；副作用和复位方式同文件入口说明。
var postNativeRepeats func(gui.Window)

func verifyMenuKeys() {
	command, spaceCommand, functionCommand := gui.NewCommand(), gui.NewCommand(), gui.NewCommand()
	actions, callbacks, locals, repeats, functions := 0, 0, 0, 0, 0
	command.ConnectActivate(func() { actions++; fmt.Println("COMMAND", actions) })
	spaceCommand.ConnectActivate(func() { panic("菜单等价键抢占了文字输入") })
	functionCommand.ConnectActivate(func() { functions++; fmt.Println("FUNCTION", functions) })
	gesture := gui.KeyGesture{Key: gui.Key1, Modifiers: gui.ModPrimary}
	windowShortcut := gui.NewShortcut(gesture)
	windowShortcut.SetCommand(command)
	windowShortcut.ConnectActivate(func() { callbacks++; fmt.Println("SHORTCUT", callbacks) })
	local := gui.NewShortcut(gesture)
	local.ConnectActivate(func() { locals++; fmt.Println("LOCAL", locals) })
	space := gui.NewShortcut(gui.KeyGesture{Key: gui.KeySpace})
	space.SetCommand(spaceCommand)
	function := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyF6})
	function.SetCommand(functionCommand)
	controller := gui.NewShortcutController()
	controller.AddShortcut(local)
	shutdowns := 0
	phase, mode := 0, gui.MenuBarAuto
	initialized, complete, renamed, installed := false, false, false, false
	title := "Run"
	var app ui.App
	var oldModel gui.MenuModel
	var oldItem *gui.MenuItem
	baseActions, baseCallbacks, baseLocals, baseFunctions := 0, 0, 0, 0
	deadline := time.Now().Add(12 * time.Minute)
	ready := func() {
		fmt.Printf("READY keys phase=%d mode=%d actions=%d callbacks=%d local=%d\n", phase, mode, actions, callbacks, locals)
	}
	must(ui.Run("org.golang-gui.MenuBarVerification", func(a ui.App) ui.RootView {
		app = a
		if !initialized {
			initialized = true
			timer := gui.App.NewTimer()
			must(timer.Start(time.Hour))
			gui.App.ConnectShutdown(func() {
				shutdowns++
				require(!timer.Active() && timer.Start(time.Hour) != nil, "退出通知前定时器未停止")
				fmt.Println("SHUTDOWN", shutdowns)
			})
			app.TickFunc(time.Second, func() { require(time.Now().Before(deadline), "菜单按键验证超时") })
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{})).Windows(ui.Window("keys").Title("Menu keys").Size(620, 340).OnAfterPaint(func(_ gui.Painter, _ geometry.Rectangle) {
			if title == "Renamed" && !renamed && oldModel != nil {
				current := app.FindWidget("keys", "bar").(*gui.MenuBar).Menu()
				require(current == oldModel && current.ItemAt(0).Submenu().ItemAt(0) == oldItem && oldItem.Label() == "Renamed", "声明式标题更新替换了动作身份")
				renamed = true
				fmt.Println("RENAMED")
			}
			host := app.FindWindow("keys")
			input := app.FindWidget("keys", "input")
			if input == nil || input.Rect().Width == 0 {
				return
			}
			if !installed {
				installed = true
				host.Shortcuts().AddShortcut(windowShortcut)
				host.Shortcuts().AddShortcut(space)
				host.Shortcuts().AddShortcut(function)
				input.AddEventController(controller)
				raw := gui.NewKeyEventController()
				raw.SetPhase(gui.PhaseCapture)
				raw.ConnectKeyDown(func(_ gui.EventContext, e events.KeyEvent) {
					if e.Key == events.Key1 && e.Repeat {
						repeats++
						fmt.Println("REPEAT", repeats)
					}
				})
				input.AddEventController(raw)
				require(host.SetFocusedWidget(input), "输入框未能获取焦点")
				ready()
			}
		}).Content(ui.VBox(
			ui.MenuBar(ui.MenuItem("File").Submenu(ui.MenuItem(title).CommandRef(command), ui.MenuItem("Space").CommandRef(spaceCommand), ui.MenuItem("Function").CommandRef(functionCommand))).ID("bar").Mode(mode).OnError(func(err error) { panic(err) }),
			ui.Label(fmt.Sprintf("Phase %d, mode %d: follow the source instructions", phase, mode)),
			ui.TextInput().ID("input"),
			ui.HBox(ui.Button("Next").OnClick(func() {
				switch phase {
				case 0:
					require(locals == baseLocals+1 && actions == baseActions && callbacks == baseCallbacks, "局部快捷键优先级错误")
					local.SetEnabled(false)
				case 1:
					require(actions == baseActions+1 && callbacks == baseCallbacks+1, "窗口快捷键回调和命令没有各执行一次")
				case 2:
					require(repeats > 0 && actions == baseActions+1 && callbacks == baseCallbacks+1, "关闭 AutoRepeat 后仍执行重复按键")
					windowShortcut.SetAutoRepeat(true)
				case 3:
					require(repeats > 0 && actions > baseActions+1 && callbacks-baseCallbacks == actions-baseActions, "开启 AutoRepeat 后重复按键交付错误")
					windowShortcut.SetAutoRepeat(false)
				case 4:
					require(app.FindWidget("keys", "input").(*gui.TextInput).Text() == " " && actions == baseActions, "文字输入被窗口快捷键抢占")
				case 5:
					require(functions == baseFunctions+1, "功能键等价键未执行一次")
				case 6:
					require(renamed && actions == baseActions+1 && callbacks == baseCallbacks, "改标题后的菜单选择丢失或重复执行")
					if mode == gui.MenuBarInWindow {
						complete = true
						fmt.Println("PASS menu keys")
						app.Quit()
						return
					}
					mode = gui.MenuBarInWindow
					phase = -1
					local.SetEnabled(true)
					title = "Run"
					renamed = false
					app.FindWidget("keys", "input").(*gui.TextInput).SetText("")
				}
				phase++
				repeats = 0
				baseActions, baseCallbacks, baseLocals, baseFunctions = actions, callbacks, locals, functions
				app.RequestUpdate()
				require(app.FindWindow("keys").SetFocusedWidget(app.FindWidget("keys", "input")), "未能恢复输入框焦点")
				ready()
			}), ui.Button("Repeat").Enabled(postNativeRepeats != nil).OnClick(func() {
				require(phase == 2 || phase == 3, "当前阶段不接受重复事件")
				host := app.FindWindow("keys")
				require(host.SetFocusedWidget(app.FindWidget("keys", "input")), "重复事件输入框未能获取焦点")
				postNativeRepeats(host)
			}), ui.Button("Arm rename").OnClick(func() {
				require(phase == 6, "当前阶段不接受标题变更")
				bar := app.FindWidget("keys", "bar").(*gui.MenuBar)
				oldModel = bar.Menu()
				oldItem = oldModel.ItemAt(0).Submenu().ItemAt(0)
				app.TimeoutFunc(1500*time.Millisecond, func() { title = "Renamed"; app.RequestUpdate() })
			})),
		).Spacing(10).Padding(12)))
	}))
	require(complete, "菜单按键验证提前结束")
	require(shutdowns == 1 && len(gui.App.Surfaces()) == 0, "应用清理不完整")
	fmt.Println("PASS application cleanup")
}
