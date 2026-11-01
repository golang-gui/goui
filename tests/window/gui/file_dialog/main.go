// 原生文件对话框的窗口模态、事件循环及 owner 生命周期验收；不使用 DevServer。
//
// 环境：Linux/X11（安装 zenity、kdialog 或 yad）、Windows 或 macOS 可交互桌面。
// 启动：go run ./tests/window/gui/file_dialog；-log 可指定新日志文件，默认临时文件。
// 记录 OS、Linux WM、绘制后端及缩放。GOUI_PLAT_PAINTER 可选 software/opengl，
// Windows 另可 direct2d；GOUI_PLAT_SCALE=2 请求 2x。初始为两个 Native 窗口，
// A 有选择按钮、计数器和输入框；B 有独立计数器、销毁 A 和退出按钮。
//
// 操作与预期：
//  1. 在 A 点击 Open file，反复点击 Increment A、滚动、按 F5 并尝试输入。
//     预期 A 不响应；B 的 Increment B 和 F5 仍有效；A 的 Ticks 持续增长。
//     取消选择器，预期 A 的计数和文本没有补执行；随后点击/输入立即恢复。
//  2. Open folder、Save file 重复第 1 项，选择成功和取消均应恢复输入。
//     Save 仅选择路径，不创建文件。本例成功后打印所选名称并关闭 FileAccess。
//  3. Unowned file 打开独立选择器，预期 A、B 都可操作。macOS 为独立 panel，
//     有 owner 时为 sheet；Linux 层叠与位置受当前 helper/WM 控制。
//  4. A 再次打开选择器，在 B 点击 Destroy A。预期选择器关闭，终端报告
//     owner unavailable，B 保持可用，不出现孤儿 helper 或迟到的有效文件结果。
//  5. 在有/无 owner 的选择器打开期间，从 B 点击 Quit。预期进程正常退出，
//     选择器一并关闭；应用停止后不要求业务回调继续执行。
//
// 日志记录 open/complete、计数、文本及每秒 tick。副作用：两个窗口、原生选择器、
// 临时日志；不读取、写入或删除选择的文件，不修改剪贴板/系统设置。
// 复位：重启。退出：B 的 Quit 或关闭 B；正常退出会销毁剩余窗口。
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
)

func main() {
	path := flag.String("log", "", "新日志路径；默认使用临时文件")
	flag.Parse()
	if err := run(*path); err != nil {
		log.Fatal(err)
	}
}

func run(path string) error {
	var f *os.File
	var err error
	if path == "" {
		f, err = os.CreateTemp("", "goui-file-dialog-*.log")
	} else {
		f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	}
	if err != nil {
		return err
	}
	defer f.Close()
	out := log.New(io.MultiWriter(os.Stdout, f), "", log.Lmicroseconds)
	out.Printf("LOG %s", f.Name())
	app, err := gui.NewApplication("org.golanggui.FileDialogModal")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.FromSettings(app.Settings()))
	a, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 620, Height: 460}})
	if err != nil {
		return err
	}
	defer a.Destroy()
	b, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 380, Height: 280}})
	if err != nil {
		return err
	}
	defer b.Destroy()
	if err := a.SetTitle("GOUI file dialog A"); err != nil {
		return err
	}
	if err := b.SetTitle("GOUI file dialog B"); err != nil {
		return err
	}
	a.SetID("a")
	b.SetID("b")
	alive := true
	a.ConnectDestroy(func() { alive = false; out.Print("DESTROY A") })
	countA, countB, ticks := 0, 0, 0
	labelA, labelB, tickLabel := gui.NewLabel("A: 0"), gui.NewLabel("B: 0"), gui.NewLabel("Ticks: 0")
	input := gui.NewTextInput()
	input.SetText("initial")
	input.ConnectText(func(_ string) { out.Printf("TEXT %q", input.Text()) })
	incA := func() { countA++; labelA.SetText(fmt.Sprintf("A: %d", countA)); out.Printf("COUNT A %d", countA) }
	incB := func() { countB++; labelB.SetText(fmt.Sprintf("B: %d", countB)); out.Printf("COUNT B %d", countB) }
	button := func(title string, fn func()) *gui.Button {
		w := gui.NewButton()
		w.SetChild(gui.NewLabel(title))
		w.ConnectClicked(fn)
		return w
	}
	box := func() *gui.LinearBox {
		v := gui.NewLinearBox(layout.DirectionVertical)
		v.SetSpacing(10)
		v.SetPadding(geometry.UniformInsets(16))
		v.SetCrossAlign(layout.CrossStretch)
		return v
	}
	dialog := app.FileDialog()
	open := func(kind string, owned bool) {
		var owner gui.Window
		if owned {
			owner = a
		}
		out.Printf("OPEN %s owned=%t A=%d B=%d", kind, owned, countA, countB)
		cb := func(access []gui.FileAccess, err error) {
			out.Printf("COMPLETE %s owned=%t error=%v A=%d B=%d alive=%t", kind, owned, err, countA, countB, alive)
			for _, item := range access {
				out.Printf("SELECTED %q", item.Name())
				if err := item.Close(); err != nil {
					out.Printf("CLOSE %v", err)
				}
			}
		}
		switch kind {
		case "file":
			dialog.OpenFile(owner, gui.DialogOptions{Title: "GOUI choose file"}, cb)
		case "folder":
			dialog.OpenDirectory(owner, gui.DialogOptions{Title: "GOUI choose folder"}, cb)
		case "save":
			dialog.SaveFile(owner, gui.DialogOptions{Title: "GOUI choose destination", DefaultName: "goui-modal.txt"}, cb)
		}
	}
	rootA := box()
	for _, w := range []gui.Widget{gui.NewLabel("A: owner-modal file picker"), button("Open file", func() { open("file", true) }), button("Open folder", func() { open("folder", true) }), button("Save file", func() { open("save", true) }), button("Unowned file", func() { open("file", false) }), button("Increment A", incA), labelA, input, tickLabel} {
		rootA.AddChild(w)
	}
	a.SetWidget(rootA)
	rootB := box()
	for _, w := range []gui.Widget{gui.NewLabel("B: remains interactive"), button("Increment B", incB), labelB, button("Destroy A", a.Destroy), button("Quit", app.Quit)} {
		rootB.AddChild(w)
	}
	b.SetWidget(rootB)
	for _, item := range []struct {
		w  gui.Window
		fn func()
	}{{a, incA}, {b, incB}} {
		s := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyF5})
		s.ConnectActivate(item.fn)
		item.w.Shortcuts().AddShortcut(s)
	}
	timer := app.NewTimer()
	timer.ConnectTimeout(func() {
		ticks++
		if alive {
			tickLabel.SetText(fmt.Sprintf("Ticks: %d", ticks))
		}
		if ticks%4 == 0 {
			out.Printf("TICK %d", ticks)
		}
	})
	if err := timer.Start(250 * time.Millisecond); err != nil {
		return err
	}
	defer timer.Stop()
	b.ConnectDestroy(app.Quit)
	if err := a.Show(); err != nil {
		return err
	}
	if err := b.SetPosition(a, geometry.Point{X: 650, Y: 0}); err != nil {
		out.Printf("POSITION B %v", err)
	}
	if err := b.Show(); err != nil {
		return err
	}
	app.Run()
	out.Print("END")
	return nil
}
