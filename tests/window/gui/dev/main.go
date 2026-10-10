// DevServer 引用、文本及 Popover 窗口验证。
//
// 环境：Linux/X11、Windows 或 macOS 桌面、原生字体库；记录后端和 1x/2x。
// POSIX：GOUI_DEV_ADDR=:8888 go run ./tests/window/gui/dev
// PowerShell：$env:GOUI_DEV_ADDR=':8888'; go run ./tests/window/gui/dev
// 可加 -verify 执行本协议自身的自动断言，通过后关闭窗口；-addr 可指定服务 URL。
// GOUI_PLAT_PAINTER=software/opengl，Windows 另有 direct2d；可用 GOUI_PLAT_SCALE=2。
// 初始：Native 窗口，无 Window/Widget ID；单行 abc、多行 line1/line2、只读字段。
//
// 操作及预期：
//  1. GET http://127.0.0.1:8888/goui/dev/snapshot。Window 和所有语义节点均有
//     非空且互不相同的 ref；id 仍为空。后续 Snapshot 的 Window ref 保持不变。
//  2. 按 Bounds 向 Window ref POST /event 的 pointer_down/up，点击 main-editor；
//     POST /text {"target":"窗口ref","text":"你👋é"}，文字插入光标/替换选区。
//     通过 /event 发送 Ctrl+A、Ctrl+Z（macOS Command），检查全选替换、撤销；
//     /text 的空字符串不删除选区。事件返回不表示画面完成，重新读取 Snapshot 确认。
//  3. 点击只读字段后提交，返回 ok=false，字段与主编辑框均不改变。真实 IME
//     有非空组合时提交也应失败，保留组合；确认/取消组合后恢复提交。-verify 不测试真实 IME。
//  4. 点击“显示模态输入面板”，Window.popovers 报告独立 ref 与局部 DIP 子树。
//     按面板自己的 Bounds 向 Popover ref 点击 popup-editor 后提交，只有面板文字改变；
//     向 Window ref 提交失败。向 Popover ref 发送 Ctrl/Command+A、Esc，复用模态内容
//     分发；关闭后旧 Popover ref 返回错误，重新显示后获得新 ref。
//  5. 点击“内置菜单”，即使应用不持有其 Popover，也能从 Snapshot 查到它；按菜单项
//     Bounds 点击“菜单操作”，menu 计数增一且菜单关闭；不承诺菜单选项键盘导航。
//  6. 在多行正文中点击并 /text 提交带换行的文字，Snapshot 与界面均保留换行；
//     单行将换行归一化为空格。不存在的 ref、Widget ref、缺少 text 均明确失败。
//
// 平台差异：Popover 无原生键盘焦点；此例程序化提交不验收原生 Popover IME 委托。
// 副作用：显示原生窗口，并按 GOUI_DEV_ADDR 开启开发 HTTP 服务，不访问剪贴板或文件。
// 仅用于 DevServer 自身验收，不作为其他控件的自动化验收前置。
// 复位：重启；退出：关闭主窗口。自动验证失败输出 FAIL、返回非零。用完恢复环境变量。
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	_ "github.com/golang-gui/goui/gui/dev"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
)

type backendProbe struct {
	gui.WidgetBase
	reported bool
}

func (probe *backendProbe) Paint(p gui.Painter) {
	if probe.reported {
		return
	}
	probe.reported = true
	resource, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	if err != nil {
		panic(err)
	}
	fmt.Printf("backend=%T pixel-scale=%g\n", resource, p.PixelScale())
	resource.Destroy()
}

func main() {
	runtime.LockOSThread()
	verify := flag.Bool("verify", false, "verify the live dev protocol and exit")
	addr := flag.String("addr", "http://127.0.0.1:8888", "dev server URL")
	flag.Parse()
	if *verify && os.Getenv("GOUI_DEV_ADDR") == "" {
		log.Fatal("-verify requires GOUI_DEV_ADDR before starting the process")
	}
	app, err := gui.NewApplication("org.golang-gui.DevProtocolTest")
	if err != nil {
		log.Fatal(err)
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 640, Height: 480}})
	if err != nil {
		log.Fatal(err)
	}
	defer win.Destroy()
	if err := win.SetTitle("GOUI Dev protocol — no IDs"); err != nil {
		log.Fatal(err)
	}
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(16)
	root.SetSpacing(10)
	root.SetCrossAlign(layout.CrossStretch)
	root.AddChild(&backendProbe{})
	root.AddChild(gui.NewLabel("dev ref：窗口、模态面板、内置菜单和 Unicode 文本"))
	input := gui.NewTextInput()
	input.SetName("main-editor")
	input.SetText("abc")
	root.AddChild(input)
	readOnly := gui.NewTextInput()
	readOnly.SetName("readonly-editor")
	readOnly.SetText("read-only")
	readOnly.SetReadOnly(true)
	root.AddChild(readOnly)
	editor := gui.NewTextView()
	editor.SetName("multiline-editor")
	editor.SetModel(gui.NewTextModel("line1\nline2"))
	editor.SetMainWeight(1)
	root.AddChild(editor)
	open := gui.NewButton()
	open.SetName("show-popup")
	open.SetChild(gui.NewLabel("显示模态输入面板"))
	p := gui.NewPopover(open, nil)
	defer p.Destroy()
	p.SetModal(true)
	p.SetPlacement(gui.PopoverPlacementBottom)
	panel := gui.NewLinearBox(layout.DirectionVertical)
	panel.SetPadding(12)
	panel.SetSpacing(8)
	panel.SetCrossAlign(layout.CrossStretch)
	panel.SetMinSize(geometry.Size{Width: 320})
	panel.AddChild(gui.NewLabel("已确认文本，不模拟原生 IME"))
	popupInput := gui.NewTextInput()
	popupInput.SetName("popup-editor")
	popupInput.SetText("popup")
	panel.AddChild(popupInput)
	close := gui.NewButton()
	close.SetName("close-popup")
	close.SetChild(gui.NewLabel("关闭面板"))
	close.ConnectClicked(p.Hide)
	panel.AddChild(close)
	p.SetWidget(panel)
	p.ConnectDismissRequest(p.Hide)
	open.ConnectClicked(func() {
		if err := p.Show(); err != nil {
			panic(err)
		}
	})
	root.AddChild(open)
	menuButton := gui.NewMenuButton()
	menuButton.SetName("show-menu")
	menuButton.SetChild(gui.NewLabel("内置菜单"))
	menu := gui.NewMenu()
	count := 0
	status := gui.NewLabel("menu=0")
	status.SetName("menu-count")
	menu.Append("菜单操作", func() { count++; status.SetText(fmt.Sprintf("menu=%d", count)) })
	menu.Append("另一操作", func() {})
	menuButton.SetMenu(menu)
	root.AddChild(menuButton)
	root.AddChild(status)
	win.SetWidget(root)
	if err := win.Show(); err != nil {
		log.Fatal(err)
	}
	var result chan error
	if *verify {
		result = make(chan error, 1)
		go func() {
			result <- verifyProtocol(*addr)
			app.Post(win.Destroy)
		}()
	}
	app.Run()
	if *verify {
		if err := <-result; err != nil {
			log.Fatalf("FAIL: %v", err)
		}
		fmt.Println("PASS: native Window/Popover dev input, Unicode, undo and refs")
	}
}
