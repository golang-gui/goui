// 按钮分组窗口验收，需要真实桌面、系统字体和绘图后端，不使用 DevServer。
// 启动：go run ./tests/window/gui/button_groups；-auto 自动走正常窗口事件路径。
// 可加 -basic、-dark；GOUI_PLAT_PAINTER=software/opengl（Windows 可用 direct2d），
// GOUI_PLAT_SCALE=1/2。初始为横向普通/菜单连排、纵向连排、自动互斥组，以及跨区域显式组。
// 操作及预期：
//  1. 悬停、按住和点击 First；计数只在有效点击时递增，圆角只保留外侧，接缝一条。
//     Tab 到首按钮可见完整焦点边框，Space/Enter 激活；鼠标聚焦没有键盘焦点装饰。
//  2. 点击 Hide/Reverse，隐藏项不留空隙；实际可见首尾立即调整圆角。反向后按钮身份不变。
//  3. 点击 Check A/B，自动互斥；独立项 Group(nil) 与两者互不影响；跨区域 X/Y 也互斥。
//  4. File/Edit 同组；点 File 后移到 Edit 切换，左右键切换，Esc 关闭，空闲悬停不开启。
//     Other 为嵌套独立组，普通 Run 按钮不会参与菜单联动，点击仍执行一次。
//  5. 切换 Basic/Modern、亮暗；每个按钮仍用自己的 StyleName，文字不继承组或相邻按钮配色。
//     1x/2x 重复检查圆角和接缝，无内部圆角空洞或双边框；纵向同理。
//
// F6/Check 在 stdout 输出断言和窗口局部 DIP 边界。自动模式依次验证点击、隐藏、重排、
// 选择、键盘聚焦激活和主题重建；菜单原生指针切换另用 menu_group 用例验收。
// 副作用仅测试窗口、焦点、指针及日志；不修改文件/设置/剪贴板。重启复位，关闭退出。
package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
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
func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	auto := flag.Bool("auto", false, "通过正常事件分发自动验收")
	basic := flag.Bool("basic", false, "使用 Basic")
	dark := flag.Bool("dark", false, "Modern 暗色")
	flag.Parse()
	hidden, reverse, started, complete := false, false, false, false
	clicks, checks, selection, shared, crossSelection, step := 0, 0, 0, 0, 0, 0
	explicit := widgets.NewCheckButtonGroup()
	var app ui.App
	var original *gui.Button
	find := func(id string) gui.Widget { return app.FindWidget("groups", id) }
	check := func() {
		g := find("strip").(*gui.ButtonGroup)
		first := find("first").(*gui.Button)
		if original == nil {
			original = first
		}
		require(first == original, "重建替换了按钮")
		require(g.Snapshot().Role == gui.RoleHBox && len(g.Children()) == 3, "连排容器语义错误")
		require(find("menu-file").(*gui.MenuButton).Group() == find("menus"), "菜单没有使用实际容器")
		a, b := find("check-a").(*widgets.CheckButton), find("check-b").(*widgets.CheckButton)
		require(a.Group() != nil && a.Group() == b.Group() && find("independent").(*widgets.CheckButton).Group() == nil, "自动组或显式 nil 错误")
		require(find("x").(*widgets.CheckButton).Group() == explicit && find("y").(*widgets.CheckButton).Group() == explicit, "跨区域显式组错误")
		for _, id := range []string{"first", "middle", "last", "check-a", "check-b", "menu-file", "menu-edit", "hide", "reverse"} {
			r := find(id).Snapshot().Bounds
			fmt.Printf("BOUNDS %s %.1f %.1f %.1f %.1f\n", id, r.X, r.Y, r.Width, r.Height)
		}
		checks++
		fmt.Printf("ASSERT PASS checks=%d clicks=%d selection=%d hidden=%v reverse=%v\n", checks, clicks, selection, hidden, reverse)
	}
	must(ui.Run("org.golang-gui.ButtonGroups", func(a ui.App) ui.RootView {
		app = a
		if !started {
			started = true
			if *auto {
				app.TickFunc(180*time.Millisecond, func() {
					w := app.FindWindow("groups")
					if w == nil || find("first") == nil || find("first").Rect().Width == 0 {
						return
					}
					point := func(id string) geometry.Point {
						r := find(id).Snapshot().Bounds
						return geometry.Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
					}
					click := func(id string) {
						for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
							must(w.DispatchEvent(events.PointerEvent{EventType: kind, Position: point(id), Button: events.PointerButtonLeft}))
						}
					}
					key := func(k events.Key) {
						for _, kind := range []events.EventType{events.KeyDown, events.KeyUp} {
							must(w.DispatchEvent(events.KeyEvent{EventType: kind, Key: k}))
						}
					}
					switch step {
					case 0:
						check()
						click("first")
						require(clicks == 1, "点击没有执行一次")
					case 1:
						click("hide")
					case 2:
						require(!find("middle").Visible(), "隐藏未生效")
						click("reverse")
					case 3:
						require(find("strip").Children()[0] == find("last"), "没有按声明重排")
						click("check-b")
					case 4:
						require(find("check-b").(*widgets.CheckButton).Checked() && !find("check-a").(*widgets.CheckButton).Checked(), "自动组没有互斥")
						click("independent")
					case 5:
						require(find("independent").(*widgets.CheckButton).Checked() && find("check-b").(*widgets.CheckButton).Checked(), "显式 nil 参与互斥")
						click("x")
						click("y")
					case 6:
						require(find("y").(*widgets.CheckButton).Checked() && !find("x").(*widgets.CheckButton).Checked(), "跨区域组没有互斥")
						w.SetFocusedWidget(find("first"))
						key(events.KeySpace)
						require(clicks == 2, "键盘没有激活按钮")
						click("theme")
					case 7:
						check()
						complete = true
						fmt.Println("AUTO PASS")
						app.Quit()
					}
					step++
				})
			}
		}
		update := func() { app.RequestUpdate() }
		children := []ui.View{ui.Button("First").ID("first").OnClick(func() { clicks++; fmt.Printf("CLICK %d\n", clicks); update() }), ui.Button("Middle").ID("middle").Visible(!hidden), ui.Button("Last").ID("last")}
		if reverse {
			children[0], children[2] = children[2], children[0]
		}
		var sheet = gui.DefaultStyleSheet()
		if !*basic {
			sheet = modern.Sheet(modern.Options{Dark: *dark})
		}
		return ui.Root().StyleSheet(sheet).Windows(ui.Window("groups").Title("GOUI Button Groups").Chrome(ui.WindowChromeIntegrated).Size(840, 620).Center(true).Content(
			ui.VBox(ui.HeaderBar(ui.Label("Button groups / local scopes")),
				ui.Label("Connected buttons — direct children, independent styles"),
				ui.ButtonGroup(children...).ID("strip"),
				ui.HBox(ui.ButtonGroup(ui.Button("Top"), ui.Button("Bottom")).Direction(layout.DirectionVertical),
					ui.ButtonGroup(ui.Button("Primary").Style(modern.Primary).Content(ui.Label("Primary").Style(modern.PrimaryContent)), ui.MenuButton("Actions").Menu(ui.MenuItem("Action", func() { fmt.Println("ACTION") }))),
				).Spacing(24),
				wui.CheckButtonGroup(ui.ButtonGroup(
					wui.CheckButton("Check A").ID("check-a").Appearance(wui.CheckAppearanceButton).Checked(selection == 0).OnChange(func(v widgets.CheckState) {
						if v == widgets.CheckChecked {
							selection = 0
							update()
						}
					}),
					wui.CheckButton("Check B").ID("check-b").Appearance(wui.CheckAppearanceButton).Checked(selection == 1).OnChange(func(v widgets.CheckState) {
						if v == widgets.CheckChecked {
							selection = 1
							update()
						}
					}),
					wui.CheckButton("Independent").ID("independent").Appearance(wui.CheckAppearanceButton).Group(nil).Checked(shared&1 != 0).OnChange(func(v widgets.CheckState) {
						if v == widgets.CheckChecked {
							shared |= 1
						} else {
							shared &^= 1
						}
						update()
					}),
				)),
				ui.HBox(wui.CheckButton("Cross-area X").ID("x").Group(explicit).Checked(crossSelection == 1).OnChange(func(v widgets.CheckState) {
					if v == widgets.CheckChecked {
						crossSelection = 1
						update()
					}
				}), ui.Label("separate regions"), ui.VBox(wui.CheckButton("Cross-area Y").ID("y").Group(explicit).Checked(crossSelection == 2).OnChange(func(v widgets.CheckState) {
					if v == widgets.CheckChecked {
						crossSelection = 2
						update()
					}
				}))).Spacing(24),
				ui.MenuButtonGroup(ui.MenuButton("File").ID("menu-file").Menu(ui.MenuItem("Open", func() { fmt.Println("OPEN") })), ui.MenuButton("Edit").ID("menu-edit").Menu(ui.MenuItem("Copy", nil)), ui.Button("Run").OnClick(func() { fmt.Println("RUN") }), ui.MenuButtonGroup(ui.MenuButton("Other").Menu(ui.MenuItem("Other", nil)))).ID("menus").Spacing(8),
				ui.HBox(ui.Button("Hide middle").ID("hide").OnClick(func() { hidden = !hidden; update() }), ui.Button("Reverse").ID("reverse").OnClick(func() { reverse = !reverse; update() }), ui.Button("Basic / Modern").ID("theme").OnClick(func() { *basic = !*basic; update() }), ui.Button("Light / Dark").OnClick(func() { *dark = !*dark; update() }), ui.Button("Check (F6)").OnClick(check)).Spacing(8),
				ui.Label(fmt.Sprintf("Clicks %d | selection %d | checks %d", clicks, selection, checks)),
			).Padding(18).Spacing(16).CrossAlign(layout.CrossStart).Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(check)),
		))
	}))
	if *auto {
		require(complete, "自动验收提前退出")
	}
}
