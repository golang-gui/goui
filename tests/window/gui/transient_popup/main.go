// 内置浮层共享宿主验证，不依赖 DevServer。
// 环境：Linux X11／Windows／macOS 交互桌面，记录绘图后端和缩放。
// 启动：go run ./tests/window/gui/transient_popup；GOUI_PLAT_PAINTER 选择后端。
// 初始：三列各有 TextInput、MenuButton、DropDown 和 Tooltip 按钮；另有普通 Popover。
// 操作与预期：
//
//  1. 依次右击三个 TextInput，每次 Esc 关闭；菜单内容、输入焦点不串到其他输入框。
//     依次打开三个 MenuButton 和 DropDown，Esc 关闭，再选择某个选项；状态和快照一致。
//
//  2. 依次悬停三个 Tooltip 按钮 1 秒；提示文字分别为对应列号，移出关闭。
//     打开菜单或下拉框后等待 1 秒，不得出现 Tooltip。重复以上步骤三轮。
//
//  3. 点击 Ordinary 显示不透明用户 Popover，它可以与菜单／下拉框／Tooltip 共存；
//     用 Ordinary hide 关闭后重新打开，内容在宽行与 Small 之间交替；慢绘制期间不得残留旧字。
//     点击 Ordinary hide 只关闭普通 Popover。关闭主窗口后没有残留浮窗。
//
//  4. F6 断言最多一个内置浮层可见，并输出当前角色／文字和展开状态；Esc 不修改选择。
//     若需严格核对资源身份和工厂次数，先 go generate，再使用输出的 overlay 构建：
//     GOUI_GENERATE_DIR=/absolute/out go generate ./tests/window/gui/transient_popup
//     go build -overlay /absolute/out/tests/window/gui/transient_popup/overlay.json ...
//     TRACE CREATE 每窗口只能一条共享 Popup 和一条共享 Painter，所有 TRACE LEASE
//     的 native、handle、painter 必须相同，且只在窗口关闭时出现 TRACE DESTROY。
//     普通 Popover 的 CREATE 标注 independent，单独计数。不能仅按同时存活数量验收。
//     用该 overlay 构建后设置 GOUI_VERIFY_SLOW_MS=3000：打开 Menu 3、Esc、再开 Menu 1
//     或输入框右键菜单。等待期间只能为空白/透明或本次背景，不得显示上一个菜单的文字；
//     3 秒后内容应正确且窗口几何不跳动。这是故障放大，不是生产框架的延迟。
//
//  5. go run ./tests/window/gui/transient_popup -verify-reuse 自动派发普通鼠标／Esc 输入：
//     三列不同宽高的菜单和列表交替 60 次，断言 Show 当次已有布局、重开已有同步首帧、
//     下一轮事件循环几何不再跳动；测试窗口每 18 次在普通位置和工作区右下方切换。
//     另检查透明／不透明原生 Popup 的隐藏尺寸查询、
//     小数尺寸、同尺寸重复请求、同步绘制和销毁后错误。PASS 后自动关闭；失败退出非零。
//     Windows 另向真实 HWND 派发 WM_DPICHANGED，验证建议矩形不覆盖显式 Popup 几何；
//     不修改系统 DPI，此消息回归不等于真实混合 DPI 多屏迁移验收。
//
//  6. -verify-anchor 自动创建第二窗口，检查 SetAnchor 结束显示、同窗复用、旧锚点卸载、
//     共享客户端使用目标缓存且源缓存可继续使用、普通跨窗重建及源窗销毁隔离。
//     ANCHOR PASS 后自动退出。搭配观察 overlay：ordinary 同一 Root 两次 CREATE，
//     两个 Surface 的 shared 各一次 CREATE；最终四次 DESTROY，无额外资源分配。
//
//  7. -verify-invalidation 自动检查普通／共享弹窗可见更新尺寸、隐藏内容、空闲缓存、
//     卸载／重挂锚点期间切换样式：隐藏时无测量、绘制或 popup layout 日志，重开采用
//     新样式尺寸并复用共享 Root。观察 overlay 仅有一次 independent 和一次 shared
//     CREATE，最终各释放一次。INVALIDATION PASS 后自动退出；失败退出非零。
//
// 副作用：只创建窗口与日志，不改文件、剪贴板或系统设置。复位：重启。退出：关闭主窗口。
package main

//go:generate go run generate.go

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	verify := flag.Bool("verify-reuse", false, "run popup reuse assertions and exit")
	verifyAnchor := flag.Bool("verify-anchor", false, "run anchor and Surface migration assertions and exit")
	verifyInvalidation := flag.Bool("verify-invalidation", false, "run hidden popup invalidation assertions and exit")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.SharedPopupVerification")
	must(err)
	sheet := modern.Sheet(modern.Options{})
	app.SetStyleSheet(sheet)
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 680, Height: 460}, Chrome: gui.WindowChromeIntegrated})
	must(err)
	must(win.SetTitle("GOUI Shared Popup Verification"))
	_ = win.SetPosition(nil, geometry.Point{X: 100, Y: 100})
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(geometry.UniformInsets(20))
	root.SetSpacing(16)
	root.SetCrossAlign(layout.CrossStretch)
	root.AddChild(gui.NewLabel("Right click inputs; open menus/lists; hover tips. F6 checks."))
	columns := gui.NewLinearBox(layout.DirectionHorizontal)
	columns.SetSpacing(16)
	columns.SetCrossAlign(layout.CrossStretch)
	var inputs [3]*gui.TextInput
	var menus [3]*gui.MenuButton
	var lists [3]*widgets.DropDown
	var tips [3]*gui.Button
	for i := range 3 {
		column := gui.NewLinearBox(layout.DirectionVertical)
		column.SetMainWeight(1)
		column.SetCrossAlign(layout.CrossStretch)
		column.SetSpacing(18)
		inputs[i] = gui.NewTextInput()
		inputs[i].SetText(fmt.Sprintf("Input %d", i+1))
		column.AddChild(inputs[i])
		menus[i] = gui.NewMenuButton()
		menus[i].SetChild(gui.NewLabel(fmt.Sprintf("Menu %d", i+1)))
		var items []*gui.MenuItem
		for j := range 1 + i*3 {
			label := fmt.Sprintf("Action %d-%d", i+1, j+1)
			if i == 1 {
				label += " with a longer label"
			}
			items = append(items, gui.NewMenuItem(label, func() { log.Printf("ACTION %d-%d", i+1, j+1) }))
		}
		menus[i].SetMenu(gui.NewSliceListModel(items))
		menus[i].ConnectOpenError(func(err error) { log.Fatal(err) })
		column.AddChild(menus[i])
		lists[i] = widgets.NewDropDown()
		var choices []widgets.DropDownItem
		for j := range 2 + i*4 {
			choices = append(choices, widgets.DropDownItem{Text: fmt.Sprintf("Choice %d-%d", i+1, j+1)})
		}
		lists[i].SetModel(gui.NewSliceListModel(choices))
		lists[i].SetSelected(0)
		lists[i].ConnectOpenError(func(err error) { log.Fatal(err) })
		column.AddChild(lists[i])
		tips[i] = gui.NewButton()
		tips[i].SetChild(gui.NewLabel(fmt.Sprintf("Tooltip %d", i+1)))
		tips[i].SetTooltip(fmt.Sprintf("Help for column %d", i+1))
		column.AddChild(tips[i])
		columns.AddChild(column)
	}
	root.AddChild(columns)
	ordinaryButton := gui.NewButton()
	ordinaryButton.SetChild(gui.NewLabel("Ordinary"))
	ordinary := gui.NewPopover(ordinaryButton, nil)
	ordinary.SetPosition(geometry.Point{X: 460, Y: 0})
	ordinary.SetWidget(gui.NewLabel("Independent user Popover"))
	ordinaryOpens := 0
	ordinaryButton.ConnectClicked(func() {
		ordinaryOpens++
		text := "Independent user Popover with wider content"
		if ordinaryOpens%2 == 0 {
			text = "Small"
		}
		ordinary.SetWidget(gui.NewLabel(text))
		must(ordinary.Show())
	})
	root.AddChild(ordinaryButton)
	hide := gui.NewButton()
	hide.SetChild(gui.NewLabel("Ordinary hide"))
	hide.ConnectClicked(ordinary.Hide)
	root.AddChild(hide)
	keys := gui.NewKeyEventController()
	keys.ConnectKeyDown(func(_ gui.EventContext, e events.KeyEvent) {
		if e.Key != events.KeyF6 {
			return
		}
		transients := 0
		for _, p := range win.Popovers() {
			if p.Widget() != ordinary.Widget() {
				transients++
				log.Printf("VISIBLE style=%s role=%s text=%q", p.StyleName(), p.Widget().Snapshot().Role, p.Widget().Snapshot().Text)
			}
		}
		if transients > 1 {
			log.Fatal("多个内置浮层同时可见")
		}
		for i := range 3 {
			log.Printf("STATE column=%d text=%q menu=%v list=%v selected=%d", i+1, inputs[i].Text(), menus[i].Opened(), lists[i].Opened(), lists[i].Selected())
		}
		log.Printf("CHECK PASS transient=%d ordinary=%v", transients, ordinary.Visible())
	})
	keys.SetPhase(gui.PhaseCapture)
	root.AddEventController(keys)
	win.SetWidget(root)
	must(win.Show())
	monitor := app.NewTimer()
	previous := ""
	monitor.ConnectTimeout(func() {
		state := ""
		count := 0
		for _, p := range win.Popovers() {
			if p.Widget() != ordinary.Widget() {
				count++
				state = p.StyleName()
			}
		}
		if count > 1 {
			log.Fatal("多个内置浮层同时可见")
		}
		if state != previous {
			log.Printf("PRESENT style=%s ordinary=%v", state, ordinary.Visible())
			previous = state
		}
	})
	must(monitor.Start(100 * time.Millisecond))
	ready := app.NewTimer()
	ready.ConnectTimeout(func() {
		for i := range 3 {
			for name, w := range map[string]gui.Widget{"input": inputs[i], "menu": menus[i], "list": lists[i], "tip": tips[i]} {
				log.Printf("BOUNDS %s%d %+v", name, i+1, w.Snapshot().Bounds)
			}
		}
		log.Printf("READY OS=%s typography=%s", runtime.GOOS, app.Typography().Name())
		if *verifyInvalidation {
			startInvalidationVerification(app, win, ordinary, inputs[0], inputs[1])
		} else if *verifyAnchor {
			startAnchorVerification(app, win, inputs[0], inputs[1])
		} else if *verify {
			verifyNativeReuse(app, win)
			startReuseVerification(app, win, menus, lists)
		}
	})
	must(ready.StartOnce(500 * time.Millisecond))
	win.ConnectDestroy(func() { log.Print("DESTROY PASS") })
	must(app.Run())
	ready.Stop()
	monitor.Stop()
}
