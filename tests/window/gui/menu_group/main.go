// MenuButtonGroup 与非模态菜单的窗口回归，需要真实桌面、字体及原生绘图后端。
// 启动：go run ./tests/window/gui/menu_group；-auto 自动走正常 Window.DispatchEvent。
// 自动模式先把真实指针停在测试窗口外，运行中不同时输入，避免与派发坐标混用。
// 可加 -basic、-dark，使用 GOUI_PLAT_PAINTER 和 GOUI_PLAT_SCALE 选择后端及缩放。
// 初始 File/Edit/View 同组，Other 独立组，Run 是普通按钮，Disabled/Empty 不可打开。
// 操作：空闲悬停不打开；点击 File 后移到 Edit/View 切换，左右键循环、上下与 Enter
// 执行项目；点击展开按钮关闭；点击 Run 同时关闭并执行一次；Esc 后悬停保持关闭；
// 键盘切换后指针不动应保持该菜单，真实移动才重新跟随；Tab 关闭并继续焦点导航。Rebuild 更新动作代次，Dark 切换外观，组及控件身份保留。
// 级联菜单：File/Edit/View 的 Recent → More 可展开三级；悬停延迟展开，斜移到子层
// 不应切到掠过的兄弟项；Right/Left/逐次 Esc 分层进入返回，叶项关闭整链且执行一次。
// 子菜单主体应重叠父菜单约 4 DIP。移到其他父项，高亮即时跟随，旧展开项只保留较弱背景；
// 旧子层在约 250 ms 后关闭或替换。移到宿主留白应清空整链活动项，但保持已展开的菜单。
// 向子层斜移仍可跨行到达，进入后不被旧计时关闭；屏幕右侧翻转后同样验证。
// F6/Check 输出 ASSERT PASS、JSON 状态及窗口局部 DIP 的 BOUNDS，仅供观察和定位。
// 自动场景还验证最新动作闭包、主题重建、空菜单、禁用和不同组隔离；没有语义动作接口。
// 1x/1.5x/2x、Basic/Modern、亮暗重复。最终记录真实图像资源类型及 PixelScale。
// 平台原生鼠标由 Computer Use 另验；自动派发不替代原生 grab、系统键和 IME 验收。
// 副作用：本测试窗口、指针/焦点和 stdout；不写用户文件、设置或剪贴板。
// 重启复位；关闭退出，自动场景提前关闭属于失败，正常结束验证全部菜单资源释放。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
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

type probeView struct {
	ui.ViewBase[probeView]
	probe *probe
}

func newProbe(p *probe) ui.View                        { v := &probeView{probe: p}; v.Self = v; return v }
func (v *probeView) Mount(ui.BuildContext) gui.Widget  { return v.probe }
func (*probeView) Update(ui.BuildContext, gui.Widget)  {}
func (*probeView) Unmount(ui.BuildContext, gui.Widget) {}

type probe struct {
	gui.WidgetBase
	painter string
	scale   float32
}

func (p *probe) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: 1, Height: 1}))
}
func (p *probe) Paint(draw gui.Painter) {
	if p.scale != 0 {
		return
	}
	img, err := draw.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	must(err)
	p.painter, p.scale = fmt.Sprintf("%T", img), draw.PixelScale()
	img.Destroy()
}

func main() {
	auto := flag.Bool("auto", false, "自动派发正常窗口输入并退出")
	basic := flag.Bool("basic", false, "使用 Basic 样式")
	dark := flag.Bool("dark", false, "初始 Modern 暗色")
	flag.Parse()
	generation, actions, actionGeneration, runs, closed, failures := 1, 0, 0, 0, 0, 0
	submenuActions := 0
	deadline := time.Now().Add(20 * time.Second)
	started, complete, step := false, false, 0
	var holdUntil time.Time
	var app ui.App
	var group *gui.MenuButtonGroup
	previous := map[string]*gui.MenuButton{}
	models := map[string]gui.MenuModel{}
	var input ui.State[string]
	p := new(probe)
	button := func(id string) *gui.MenuButton { return app.FindWidget("menus", id).(*gui.MenuButton) }
	check := func() {
		win := app.FindWindow("menus")
		require(win != nil, "window missing")
		opened := 0
		for _, id := range []string{"file", "edit", "view"} {
			b := button(id)
			if group == nil {
				group = b.Group()
			}
			require(group != nil && b.Group() == group, "group identity lost")
			require(previous[id] == nil || previous[id] == b, "button rebuilt")
			require(models[id] == nil || models[id] == b.Menu(), "unchanged menu model rebuilt")
			previous[id], models[id] = b, b.Menu()
			if b.Opened() {
				opened++
			}
			data := b.Snapshot().Attributes[gui.MenuButtonInfoKey].(gui.MenuButtonInfo)
			require(data.Opened == b.Opened() && data.Grouped && data.HasMenu, "snapshot mismatch")
		}
		require(opened <= 1 && button("other").Group() != group, "group isolation lost")
		require(app.FindWidget("menus", "main-menu") == group && group.Snapshot().Role == gui.RoleHBox, "menu group is not the actual layout container")
		require(failures == 0, "unexpected open error")
		state := map[string]any{"file": button("file").Opened(), "edit": button("edit").Opened(), "view": button("view").Opened(), "runs": runs, "actions": actions, "submenuActions": submenuActions, "actionGeneration": actionGeneration, "generation": generation, "closed": closed, "errors": failures, "painter": p.painter, "scale": p.scale}
		data, err := json.Marshal(state)
		must(err)
		for _, id := range []string{"file", "edit", "view", "run", "empty", "disabled", "other", "rebuild", "dark", "check", "input"} {
			r := app.FindWidget("menus", id).Snapshot().Bounds
			fmt.Printf("BOUNDS %s %g %g %g %g\n", id, r.X, r.Y, r.Width, r.Height)
		}
		fmt.Printf("ASSERT PASS %s\n", data)
	}
	must(ui.Run("org.golang-gui.MenuButtonGroupVerification", func(a ui.App) ui.RootView {
		app = a
		if !started {
			started = true
			gui.App.SetQuitOnLastWindowClosed(false)
			if *auto {
				app.TickFunc(250*time.Millisecond, func() {
					win := app.FindWindow("menus")
					require(time.Now().Before(deadline), "automatic scenario timed out")
					require(win != nil, "window missing")
					// 先等待首帧完成；计时器可能先于原生窗口的首次布局。
					if step == 0 && (p.scale == 0 || button("file").Rect().Width == 0) {
						return
					}
					point := func(id string) geometry.Point {
						r := app.FindWidget("menus", id).Snapshot().Bounds
						return geometry.Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
					}
					pointer := func(kind events.EventType, id string) {
						must(win.DispatchEvent(events.PointerEvent{EventType: kind, Position: point(id), Button: events.PointerButtonLeft}))
					}
					click := func(id string) { pointer(events.PointerDown, id); pointer(events.PointerUp, id) }
					key := func(k events.Key) {
						for _, kind := range []events.EventType{events.KeyDown, events.KeyUp} {
							must(win.DispatchEvent(events.KeyEvent{EventType: kind, Key: k}))
						}
					}
					switch step {
					case 0:
						check()
						pointer(events.PointerMove, "edit")
						require(len(win.Popovers()) == 0, "idle hover opened a menu")
						click("file")
					case 1:
						require(button("file").Opened(), "File did not open")
						pointer(events.PointerMove, "edit")
					case 2:
						require(button("edit").Opened() && !button("file").Opened(), "hover did not switch")
						// 同一分发段建立位置基线；Tick 之间允许真实原生输入改变位置。
						pointer(events.PointerMove, "edit")
						key(events.KeyArrowLeft)
						pointer(events.PointerMove, "edit")
						require(button("file").Opened(), "stationary motion overrode keyboard navigation")
					case 3:
						require(button("file").Opened(), "horizontal navigation failed")
						click("run")
					case 4:
						require(len(win.Popovers()) == 0 && runs == 1, "outside click did not execute once")
						click("file")
					case 5:
						require(button("file").Opened(), "File did not reopen")
						generation++
						app.RequestUpdate()
					case 6:
						check()
						require(button("file").Opened(), "rebuild closed the menu")
						key(events.KeyArrowDown)
						key(events.KeyEnter)
					case 7:
						require(actions == 1 && actionGeneration == 2 && !button("file").Opened(), "action used a stale closure or did not close")
						click("file")
						key(events.KeyTab)
						require(button("edit").Focused() && !button("file").Opened(), "Tab did not close and navigate")
					case 8:
						click("empty")
						click("disabled")
						require(len(win.Popovers()) == 0, "empty or disabled menu opened")
						click("file")
						pointer(events.PointerMove, "other")
					case 9:
						require(button("file").Opened() && !button("other").Opened(), "another group switched")
						key(events.KeyEscape)
						pointer(events.PointerMove, "edit")
						require(len(win.Popovers()) == 0, "idle hover after Escape opened a menu")
						click("edit")
					case 10:
						require(button("edit").Opened(), "Edit did not reopen")
						*dark = !*dark
						app.RequestUpdate()
					case 11:
						require(button("edit").Opened(), "theme rebuild closed the menu")
						check()
						require(p.scale > 0, "painter probe missing")
						button("edit").Close()
						click("file")
						key(events.KeyEnd)
						key(events.KeyArrowRight)
					case 12:
						require(len(win.Popovers()) == 2, "child menu did not open")
						require(win.Popovers()[0].Widget().Snapshot().Role == gui.RoleMenu, "root semantics missing")
						key(events.KeyArrowRight)
					case 13:
						require(len(win.Popovers()) == 3, "grandchild menu did not open")
						key(events.KeyEscape)
						require(len(win.Popovers()) == 2, "Escape closed more than one layer")
						key(events.KeyArrowRight)
						key(events.KeyEnter)
					case 14:
						require(len(win.Popovers()) == 0 && submenuActions == 1, "leaf did not close and activate exactly once")
						click("file")
						key(events.KeyEnd)
						key(events.KeyArrowRight)
						key(events.KeyArrowLeft)
						require(len(win.Popovers()) == 1, "Left did not return to root")
						key(events.KeyEscape)
						require(len(win.Popovers()) == 0, "root Escape did not finish the chain")
						click("file")
						key(events.KeyEnd)
						key(events.KeyArrowRight)
					case 15:
						require(len(win.Popovers()) == 2, "child menu missing before delayed dismissal")
						target := win.Popovers()[0]
						var action gui.WidgetInfo
						var find func(gui.WidgetInfo)
						find = func(info gui.WidgetInfo) {
							if info.Role == gui.RoleMenuItem && info.Text == "Action" {
								action = info
							}
							for _, child := range info.Children {
								find(child)
							}
						}
						find(target.Widget().Snapshot())
						require(action.Bounds.Width > 0, "Action row missing")
						must(target.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: action.Bounds.Center()}))
						require(len(win.Popovers()) == 2, "moving to a leaf closed the submenu without grace")
						find(target.Widget().Snapshot())
						require(action.Selected, "leaf selection did not follow the pointer immediately")
					case 16:
						if len(win.Popovers()) == 2 {
							return // 等待实际关闭状态，仍受整体超时约束。
						}
						require(len(win.Popovers()) == 1, "delayed closing removed the root")
						key(events.KeyEnd)
						key(events.KeyArrowRight)
						require(len(win.Popovers()) == 2, "child menu did not reopen")
						key(events.KeyArrowRight)
						require(len(win.Popovers()) == 3, "grandchild did not reopen")
						// 选择宿主右下留白，避免输入框中心恰好被菜单覆盖。
						bounds := win.Widget().Snapshot().Bounds
						must(win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: bounds.X + bounds.Width - 12, Y: bounds.Y + bounds.Height - 12}}))
						holdUntil = time.Now().Add(600 * time.Millisecond)
					case 17:
						require(len(win.Popovers()) == 3 && button("file").Opened(), "owner motion closed the expanded path")
						var unselected func(gui.WidgetInfo)
						unselected = func(info gui.WidgetInfo) {
							require(!info.Selected, "outside motion retained an active menu item")
							for _, child := range info.Children {
								unselected(child)
							}
						}
						for _, popup := range win.Popovers() {
							unselected(popup.Widget().Snapshot())
						}
						if time.Now().Before(holdUntil) {
							return // 覆盖超过普通宽限和斜移保护期限的持续保留。
						}
						key(events.KeyEscape)
						require(len(win.Popovers()) == 2, "Escape did not return from retained grandchild")
						key(events.KeyArrowLeft)
						require(len(win.Popovers()) == 1, "Left did not return from retained child")
						key(events.KeyEscape)
						require(len(win.Popovers()) == 0, "root menu survived Escape")
						complete = true
						fmt.Printf("AUTO PASS painter=%s scale=%g\n", p.painter, p.scale)
						win.Destroy()
					}
					step++
				})
			}
		}
		captured := generation
		menu := func(label, id string) *ui.MenuButtonView {
			return ui.MenuButton(label).ID(id).Menu(
				ui.MenuItem("Action", func() { actions++; actionGeneration = captured; app.RequestUpdate() }),
				ui.MenuItem("Unavailable", nil).Enabled(false),
				ui.MenuItem("Recent").Submenu(
					ui.MenuItem("More").Submenu(
						ui.MenuItem("Nested action", func() { submenuActions++; app.RequestUpdate() }),
						ui.MenuItem("Unavailable", nil).Enabled(false),
					),
					ui.MenuItem("Other recent item", func() { submenuActions++; app.RequestUpdate() }),
				),
			).OnClosed(func() { closed++; app.RequestUpdate() }).OnOpenError(func(error) { failures++ })
		}
		sheet := gui.DefaultStyleSheet()
		if !*basic {
			settings := app.Settings()
			sheet = modern.Sheet(modern.Options{Dark: *dark, AccentColor: settings.AccentColor(), FontFamily: settings.FontFamily(), FontSize: settings.FontSize()})
		}
		return ui.Root().StyleSheet(sheet).Windows(ui.Window("menus").Title("GOUI MenuButtonGroup").Chrome(ui.WindowChromeIntegrated).Size(760, 420).Content(
			ui.VBox(
				ui.HeaderBar(ui.Label("MenuButtonGroup")),
				ui.MenuButtonGroup(menu("File", "file"), menu("Edit", "edit"), menu("View", "view"), ui.Button("Run").ID("run").OnClick(func() { runs++; app.RequestUpdate() }), ui.MenuButton("Disabled").ID("disabled").Enabled(false).Menu(ui.MenuItem("No", nil)), ui.MenuButton("Empty").ID("empty")).ID("main-menu").Spacing(8),
				ui.MenuButtonGroup(ui.MenuButton("Other").ID("other").Menu(ui.MenuItem("Other action", nil))),
				ui.TextInput().ID("input").BindText(&input).MinWidth(280),
				ui.HBox(ui.Button("Rebuild").ID("rebuild").OnClick(func() { generation++; app.RequestUpdate() }), ui.Button("Dark").ID("dark").OnClick(func() { *dark = !*dark; app.RequestUpdate() }), ui.Button("Check").ID("check").OnClick(check)).Spacing(10),
				ui.Label(fmt.Sprintf("Actions %d (generation %d) | Runs %d | Closed %d | Nested %d", actions, actionGeneration, runs, closed, submenuActions)),
				ui.Label("Open File, then hover Edit/View. Arrows navigate; Escape/Tab close. F6 checks."), newProbe(p),
			).Padding(16).Spacing(14).CrossAlign(layout.CrossStart).Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(check)),
		).OnDestroy(func() {
			app.Post(func() {
				for _, b := range previous {
					require(b.Destroyed() && !b.Opened(), "menu survives destruction")
				}
				fmt.Println("DESTROY PASS")
				app.Quit()
			})
		}))
	}))
	if *auto {
		require(complete, "automatic verification aborted")
	}
}
