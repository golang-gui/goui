// 内置浮层共享宿主验证，不依赖 DevServer。
// 环境：Linux X11／Windows／macOS 交互桌面，记录绘图后端和缩放。
// 启动：go run ./tests/window/gui/transient_popup；GOUI_PLAT_PAINTER 选择后端。
// 初始：三列各有 TextInput、MenuButton、DropDown 和 Tooltip 按钮；另有普通 Popover。
// 操作与预期：
//  1. 依次右击三个 TextInput，每次 Esc 关闭；菜单内容、输入焦点不串到其他输入框。
//     依次打开三个 MenuButton 和 DropDown，Esc 关闭，再选择某个选项；状态和快照一致。
//  2. 依次悬停三个 Tooltip 按钮 1 秒；提示文字分别为对应列号，移出关闭。
//     打开菜单或下拉框后等待 1 秒，不得出现 Tooltip。重复以上步骤三轮。
//  3. 点击 Ordinary 显示用户 Popover，它可以与菜单／下拉框／Tooltip 共存；
//     点击 Ordinary hide 只关闭普通 Popover。关闭主窗口后没有残留浮窗。
//  4. F6 断言最多一个内置浮层可见，并输出当前角色／文字和展开状态；Esc 不修改选择。
//     若需严格核对资源身份和工厂次数，先 go generate，再使用输出的 overlay 构建：
//     GOUI_GENERATE_DIR=/absolute/out go generate ./tests/window/gui/transient_popup
//     go build -overlay /absolute/out/tests/window/gui/transient_popup/overlay.json ...
//     TRACE CREATE 每窗口只能一条共享 Popup 和一条共享 Painter，所有 TRACE LEASE
//     的 native、handle、painter 必须相同，且只在窗口关闭时出现 TRACE DESTROY。
//     普通 Popover 的 CREATE 标注 independent，单独计数。不能仅按同时存活数量验收。
//
// 副作用：只创建窗口与日志，不改文件、剪贴板或系统设置。复位：重启。退出：关闭主窗口。
package main

//go:generate go run generate.go

import (
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
		menus[i].SetMenu(gui.NewSliceListModel([]*gui.MenuItem{gui.NewMenuItem(fmt.Sprintf("Action %d", i+1), func() { log.Printf("ACTION %d", i+1) })}))
		menus[i].ConnectOpenError(func(err error) { log.Fatal(err) })
		column.AddChild(menus[i])
		lists[i] = widgets.NewDropDown()
		lists[i].SetModel(gui.NewSliceListModel([]widgets.DropDownItem{{Text: fmt.Sprintf("Choice %d-A", i+1)}, {Text: fmt.Sprintf("Choice %d-B", i+1)}}))
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
	ordinaryButton.ConnectClicked(func() { must(ordinary.Show()) })
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
			if p != ordinary {
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
			if p != ordinary {
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
	})
	must(ready.StartOnce(500 * time.Millisecond))
	win.ConnectDestroy(func() { log.Print("DESTROY PASS") })
	must(app.Run())
	ready.Stop()
	monitor.Stop()
}
