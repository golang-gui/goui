// 显式窗口最小尺寸与内容布局隔离验证，不使用 DevServer。
//
// 环境：Linux/X11、Windows 或 macOS 可交互桌面；记录后端、缩放和 Linux WM。
// 启动：go run ./tests/window/gui/window_min_size
// 参数：-minimum none（默认，不设置）、explicit（320×240 DIP）、
// cleared（首次显示前先设置 820×600，再用零值清除）。
// GOUI_PLAT_PAINTER=opengl/software，Windows 另可 direct2d；GOUI_PLAT_SCALE=2。
// 初始：Integrated 400×320 DIP，八个 180 DIP 宽按钮按窗口宽度换行。
// 内容无界测量宽度超过 1400 DIP，但不应自动成为原生窗口最小尺寸。
//
// 操作与预期：
//  1. 在约 600 DIP 宽桌面启动默认模式，按 F8 最大化，再按 F9 恢复。
//     预期进入系统允许的最大化状态、按钮重新换行；恢复后回到原尺寸。
//  2. 默认模式将窗口缩至约 280×220 DIP，按 F7 检查根 Rect 与 Snapshot。
//     预期仍可缩小，根铺满实际客户区，输出 CHECK PASS；较小空间可裁剪内容。
//  3. F5 给根 Widget 设置或清除 1400×900 DIP MinSize，重复缩窗和最大化。
//     预期 Widget 约束不阻止原生窗口缩小，F7 仍通过；不自动放大窗口。
//  4. 重启使用 -minimum explicit，尝试缩到 320×240 以下。
//     系统应按支持程度遵守窗口限制；F6 清除后可继续缩小，F10 重新设置。
//  5. 重启使用 -minimum cleared；预期首次绘制不恢复 820×600 限制，
//     在 600 DIP 宽桌面上仍能最大化。F7 检查，终端尺寸变化记录实际结果。
//
// Linux 可用 xprop -id <启动日志中的 nativeWindow> WM_NORMAL_HINTS 检查：
// none/cleared 不应有应用设置的 minimum size，explicit 应为 320×240×缩放。
// F5、重绘及恢复不应添加或改写该值；窗口管理器可能保留自身的系统限制。
// 平台差异：最大化是请求，按实际状态判断；Linux 无合成器时 Integrated 可回退
// Native。macOS zoom 按已有窗口状态契约；不强求各系统相同外框尺寸或恢复位置。
// 副作用：仅窗口、内存和终端日志，不保存用户数据或修改系统配置。
// 复位：F9、F6，或重启选择模式。退出：关闭窗口；断言失败 panic 并非零退出。
package main

import (
	"flag"
	"fmt"
	"image"
	"log"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
)

func main() {
	mode := flag.String("minimum", "none", "none、explicit 或 cleared")
	flag.Parse()
	if *mode != "none" && *mode != "explicit" && *mode != "cleared" {
		log.Fatal("minimum 必须为 none、explicit 或 cleared")
	}
	app, err := gui.NewApplication("org.golanggui.WindowMinimum")
	if err != nil {
		log.Fatal(err)
	}
	app.SetStyleSheet(modern.FromSettings(app.Settings()))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 400, Height: 320}, Chrome: gui.WindowChromeIntegrated})
	if err != nil {
		log.Fatal(err)
	}
	defer win.Destroy()
	if err := win.SetTitle("GOUI window minimum"); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("mode=%s platform=%s nativeWindow=%#x\n", *mode, app.Platform().Name(), win.PlatformWindow().NativeHandle())
	minimum := geometry.Size{Width: 320, Height: 240}
	switch *mode {
	case "explicit":
		win.SetMinSize(minimum)
	case "cleared":
		win.SetMinSize(geometry.Size{Width: 820, Height: 600})
		win.SetMinSize(geometry.Size{})
	}
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	root.SetPadding(geometry.UniformInsets(12))
	root.SetSpacing(8)
	header := gui.NewHeaderBar()
	header.SetChild(gui.NewLabel("窗口尺寸限制"))
	root.AddChild(header)
	hint := gui.NewLabel("F5 Widget 限制 · F6 清除窗口限制 · F7 检查 · F8 最大化 · F9 恢复 · F10 设置窗口限制")
	hint.SetWrapMode(gui.WrapWordChar)
	root.AddChild(hint)
	wrap := gui.NewWrapBox(layout.DirectionHorizontal)
	wrap.SetSpacing(8)
	wrap.SetLineSpacing(8)
	for i := range 8 {
		button := gui.NewButton()
		button.SetChild(gui.NewLabel(fmt.Sprintf("按钮 %d", i+1)))
		button.SetMinSize(geometry.Size{Width: 180, Height: 32})
		button.SetMaxSize(geometry.Size{Width: 180, Height: 32})
		wrap.AddChild(button)
	}
	root.AddChild(wrap)
	win.SetWidget(root)
	widgetMinimum := false
	bind := func(key gui.Key, fn func()) {
		s := gui.NewShortcut(gui.KeyGesture{Key: key})
		s.ConnectActivate(fn)
		win.Shortcuts().AddShortcut(s)
	}
	bind(gui.KeyF5, func() {
		widgetMinimum = !widgetMinimum
		min := geometry.Size{}
		if widgetMinimum {
			min = geometry.Size{Width: 1400, Height: 900}
		}
		root.SetMinSize(min)
		fmt.Printf("Widget.MinSize=%v\n", min)
	})
	bind(gui.KeyF6, func() { win.SetMinSize(geometry.Size{}); fmt.Println("Window.MinSize cleared") })
	bind(gui.KeyF7, func() {
		if got := win.Snapshot().Widget.Bounds; got != root.Rect() {
			panic(fmt.Sprintf("snapshot=%v root=%v", got, root.Rect()))
		}
		fmt.Printf("CHECK PASS state=%v root=%v Widget.MinSize=%v\n", win.State(), root.Rect(), root.MinSize())
	})
	bind(gui.KeyF8, func() { win.RequestState(gui.WindowStateMaximized) })
	bind(gui.KeyF9, func() { win.RequestState(gui.WindowStateNormal) })
	bind(gui.KeyF10, func() { win.SetMinSize(minimum); fmt.Printf("Window.MinSize=%v\n", minimum) })
	last := geometry.Size{}
	reportedBackend := false
	win.ConnectAfterPaint(func(p gui.Painter, bounds geometry.Rectangle) {
		if bounds.Width <= 0 || bounds.Height <= 0 {
			return
		}
		if root.Rect() != bounds {
			panic(fmt.Sprintf("root=%v client=%v", root.Rect(), bounds))
		}
		if !reportedBackend {
			img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
			if err != nil {
				panic(err)
			}
			fmt.Printf("BACKEND image=%T scale=%g\n", img, p.PixelScale())
			img.Destroy()
			reportedBackend = true
		}
		if bounds.Size != last {
			last = bounds.Size
			fmt.Printf("FRAME state=%v client=%v scale=%g\n", win.State(), bounds, p.PixelScale())
		}
	})
	win.ConnectState(func(s gui.WindowState) { fmt.Printf("STATE %v\n", s) })
	win.ConnectDestroy(app.Quit)
	if err := win.Show(); err != nil {
		log.Fatal(err)
	}
	app.Run()
}
