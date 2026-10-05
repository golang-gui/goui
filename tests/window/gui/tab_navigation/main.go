// Tab 焦点导航窗口验证，不使用 DevServer。
//
// 环境：Linux/X11、Windows 或 macOS 可交互桌面；记录 OS、Linux WM、实际
// 后端与缩放。启动：go run ./tests/window/gui/tab_navigation。
// GOUI_PLAT_PAINTER=software/opengl，Windows 另可 direct2d；
// GOUI_PLAT_SCALE=2 请求 2x。初始：Native 680×600 DIP、Modern 亮色、无逻辑焦点。
// 主内容默认顺序：first、second、action、editor、far；工具栏不参与 Tab，
// 但仍可鼠标点击。左侧表单，右侧独立 ScrollView，far 位于其底部，初始不可见。
//
// 操作及预期：
//  1. 不点击控件，按 Tab 依次到上述五项，滚动显示 far，再按 Tab 回 first。
//     Shift+Tab 逆序循环。状态栏及终端 focus 日志同步；按钮没有额外焦点边框。
//     首次无焦点时 Shift+Tab 到 far，按住 Tab 可重复转移。
//  2. 聚焦 first/second 后键入 ASCII、中文，文字只进入当前字段；Tab 不插入。
//     聚焦 action 后 Tab 到 editor；本例不要求 Space/Enter 激活按钮。
//     点击 AcceptsTab 按钮开启插入：普通 Tab 插入制表符且保留焦点；Shift+Tab
//     返回 action。再次点击按钮关闭插入后，Tab 从 editor 到 far。
//  3. F2 隐藏/显示 second；F3 禁用/启用 second 的 Focusable（不是 Disabled）。
//     Tab 跳过被隐藏或不可聚焦的字段。F5 清空焦点，Tab 从 first 开始。
//     F4 开关自定义 Tab 快捷键（bubble）：启用后普通 Tab 输出 consumed，
//     不转移焦点；editor 的 AcceptsTab 仍优先，Shift+Tab 不受此快捷键影响。
//     Ctrl+Tab/Alt+Tab 等额外修饰不触发框架导航，可能由桌面系统接管。
//  4. F7 显示模态 Popover；Tab/Shift+Tab 仅在 popup-first、popup-second 循环，
//     主窗口字段保留焦点但不能收到键入。Esc 关闭，再次 F7/Tab 从 popup-first
//     开始；关闭 Popover 后主窗口恢复正常输入。点击窗口外区域也会关闭它。
//  5. F6 检查 Widget.Focused、ContainsFocus 与 Snapshot，输出 ASSERT PASS；
//     检查只读取公开状态，不改变焦点，不等同于外观或 IME 验收。
//     1x/2x 重复前述操作，滚入目标后边界应在视口内。
//
// 平台差异：macOS 同样使用物理 Tab/Shift，不受 Primary 命名影响；原生系统
// 快捷键、IME 候选窗可能先消费按键。Popover 无原生键盘焦点，由 owner 转发。
// 副作用：窗口、终端日志与内存文字；复制/粘贴须用户主动触发，会访问剪贴板。
// 复位：重启。退出：关闭主窗口。断言失败 panic 返回非零。
package main

import (
	"fmt"
	"image"
	"log"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
)

type backendProbe struct {
	gui.WidgetBase
	reported bool
}

func (p *backendProbe) Paint(painter gui.Painter) {
	if p.reported {
		return
	}
	p.reported = true
	resource, err := painter.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	if err != nil {
		panic(err)
	}
	fmt.Printf("backend=%T pixel-scale=%g\n", resource, painter.PixelScale())
	resource.Destroy()
}

func main() {
	runtime.LockOSThread()
	app, err := gui.NewApplication("org.golang-gui.TabNavigationTest")
	if err != nil {
		log.Fatal(err)
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 680, Height: 600}, Chrome: gui.WindowChromeNative})
	if err != nil {
		log.Fatal(err)
	}
	defer win.Destroy()
	if err := win.SetTitle("GOUI Tab navigation"); err != nil {
		log.Fatal(err)
	}
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(12)
	root.SetSpacing(8)
	root.SetCrossAlign(layout.CrossStretch)
	root.AddChild(gui.NewLabel("Tab / Shift+Tab — F6 checks focus state"))
	root.AddChild(gui.NewLabel("F2 visibility | F3 focusable | F4 consume Tab | F5 clear | F7 popup"))
	status := gui.NewLabel("focus: none")
	watch := func(widget gui.Widget, id string) {
		widget.SetID(id)
		widget.ConnectFocused(func(focused bool) {
			fmt.Printf("focus %s=%v\n", id, focused)
			if focused {
				status.SetText("focus: " + id)
			} else if current := win.FocusedWidget(); current != nil {
				status.SetText("focus: " + current.ID())
			} else {
				status.SetText("focus: none")
			}
		})
	}
	first, second := gui.NewTextInput(), gui.NewTextInput()
	first.SetText("first — type here")
	second.SetText("second — optional focus")
	watch(first, "first")
	watch(second, "second")
	action := gui.NewButton()
	action.SetChild(gui.NewLabel("action"))
	watch(action, "action")
	action.ConnectClicked(func() { fmt.Println("action clicked") })
	editor := gui.NewTextView()
	editor.SetModel(gui.NewTextModel("editor: enable AcceptsTab to insert tabs"))
	watch(editor, "editor")
	editorScroll := gui.NewScrollView()
	editorScroll.SetMinSize(geometry.Size{Height: 90})
	editorScroll.SetMaxSize(geometry.Size{Height: 90})
	editorScroll.SetChild(editor)
	far := gui.NewTextInput()
	far.SetText("far — last field")
	watch(far, "far")
	longContent := gui.NewLinearBox(layout.DirectionVertical)
	longContent.SetCrossAlign(layout.CrossStretch)
	spacer := gui.NewLabel("Far field below")
	spacer.SetMinSize(geometry.Size{Height: 550})
	longContent.AddChild(spacer)
	longContent.AddChild(far)
	longScroll := gui.NewScrollView()
	longScroll.SetMainWeight(1)
	longScroll.SetChild(longContent)
	form := gui.NewLinearBox(layout.DirectionVertical)
	form.SetMainWeight(1)
	form.SetCrossAlign(layout.CrossStretch)
	form.SetSpacing(8)
	for _, widget := range []gui.Widget{first, second, action, editorScroll} {
		form.AddChild(widget)
	}
	content := gui.NewLinearBox(layout.DirectionHorizontal)
	content.SetMainWeight(1)
	content.SetCrossAlign(layout.CrossStretch)
	content.SetSpacing(12)
	content.AddChild(form)
	content.AddChild(longScroll)
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetSpacing(8)
	tool := func(text string, fn func()) *gui.Button {
		button := gui.NewButton()
		button.SetFocusable(false)
		button.SetChild(gui.NewLabel(text))
		if fn != nil {
			button.ConnectClicked(fn)
		}
		toolbar.AddChild(button)
		return button
	}
	tool("AcceptsTab", func() {
		editor.SetAcceptsTab(!editor.AcceptsTab())
		fmt.Printf("accepts-tab=%v\n", editor.AcceptsTab())
	})
	popupAnchor := tool("Popup (F7)", nil)
	popup := gui.NewPopover(popupAnchor, nil)
	popup.SetModal(true)
	defer popup.Destroy()
	popup.SetPosition(geometry.Point{Y: 40})
	popupRoot := gui.NewLinearBox(layout.DirectionVertical)
	popupRoot.SetPadding(12)
	popupRoot.SetSpacing(8)
	for _, id := range []string{"popup-first", "popup-second"} {
		button := gui.NewButton()
		button.SetChild(gui.NewLabel(id))
		watch(button, id)
		popupRoot.AddChild(button)
	}
	popup.SetWidget(popupRoot)
	popup.ConnectDismissRequest(func() { popup.Hide() })
	showPopup := func() {
		if err := popup.Show(); err != nil {
			panic(err)
		}
		fmt.Println("popup shown")
	}
	popupAnchor.ConnectClicked(showPopup)
	consume := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyTab})
	consume.SetEnabled(false)
	consume.ConnectActivate(func() { fmt.Println("Tab consumed by shortcut") })
	win.Shortcuts().AddShortcut(consume)
	bind := func(key gui.Key, fn func()) {
		s := gui.NewShortcut(gui.KeyGesture{Key: key})
		s.ConnectActivate(fn)
		win.Shortcuts().AddShortcut(s)
	}
	bind(gui.KeyF2, func() { second.SetVisible(!second.Visible()); fmt.Printf("second-visible=%v\n", second.Visible()) })
	bind(gui.KeyF3, func() {
		second.SetFocusable(!second.Focusable())
		fmt.Printf("second-focusable=%v\n", second.Focusable())
	})
	bind(gui.KeyF4, func() { consume.SetEnabled(!consume.Enabled()); fmt.Printf("consume-tab=%v\n", consume.Enabled()) })
	bind(gui.KeyF5, func() { win.SetFocusedWidget(nil); status.SetText("focus: none") })
	check := func() {
		if verifyFocus(root) > 1 || verifyFocus(popupRoot) > 1 {
			panic("multiple focused widgets in one host")
		}
		focused := "none"
		if w := win.FocusedWidget(); w != nil {
			focused = w.ID()
			if !w.Focused() {
				panic("host focus does not match widget state")
			}
		}
		fmt.Printf("ASSERT PASS owner-focus=%s scroll-y=%g first=%q second=%q editor=%q far=%q\n", focused, longScroll.ScrollY(), first.Text(), second.Text(), editor.Model().Text(), far.Text())
	}
	bind(gui.KeyF6, check)
	popupShortcuts := gui.NewShortcutController()
	popupCheck := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyF6})
	popupCheck.ConnectActivate(check)
	popupShortcuts.AddShortcut(popupCheck)
	popupRoot.AddEventController(popupShortcuts)
	bind(gui.KeyF7, showPopup)
	for _, widget := range []gui.Widget{toolbar, content, status, &backendProbe{}} {
		root.AddChild(widget)
	}
	win.SetWidget(root)
	if err := win.Show(); err != nil {
		log.Fatal(err)
	}
	app.Run()
}

func verifyFocus(root gui.Widget) int {
	count := 0
	if root.Focused() {
		count++
	}
	for _, child := range root.Children() {
		count += verifyFocus(child)
	}
	contains := count != 0
	info := root.Snapshot()
	if info.Focused != root.Focused() || info.ContainsFocus != contains || root.ContainsFocus() != contains {
		panic(fmt.Sprintf("inconsistent focus state: id=%q focused=%v contains=%v snapshot=%+v", root.ID(), root.Focused(), contains, info))
	}
	return count
}
