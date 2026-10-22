// SkipTaskbar 窗口验收，不使用 DevServer。
//
// 环境：可交互的 Linux/X11、Windows 或 macOS 桌面；记录 OS/WM、后端和缩放。
// 启动：go run ./tests/window/gui/taskbar [-chrome=none|native|integrated] [-auto]
// GOUI_PLAT_PAINTER=software/opengl（Windows 另可 direct2d），GOUI_PLAT_SCALE=1/2。
// 初始：普通 Native 主窗口与 SkipTaskbar 辅助窗口均可见，无父子关系。
// 先显示主窗口，再将隐藏的辅助窗口定位到其实际客户区 (480,80) DIP，默认使用 None；
// None 自绘 HeaderBar 和 6 DIP 缩放命中区域，不设置置顶或禁止激活。
//
// 操作与逐项预期：
//  1. 查看任务栏：Windows/X11 仅有普通窗口条目，不增加辅助窗口条目；
//     Windows Alt+Tab 也不包含辅助窗口，X11 切换器行为由 WM 决定。
//     macOS Dock 仍为应用级图标，创建辅助窗口不能隐藏应用或其他窗口。
//  2. 点击辅助窗口输入框，键入 taskbar-test，再点击“计数”。
//     预期：输入与计数可见，主窗口不阻止辅助窗口获得焦点或键盘输入。
//  3. 拖动辅助窗口标题栏（None 使用自绘 HeaderBar），再拖动右/下边缘。
//     预期：可以移动和缩放；None 未自动配置缩放光标，不属于本例验收。
//     Native/Integrated 外观可有系统差异；Native 内容尺寸不因工具标题栏变小。
//  4. 在主窗口点击“重新定位”，辅助窗口回到 (480,80)；点击“检查”。
//     预期：输出 PASS，观察到的客户区位置与请求相符，两个窗口尺寸非零。
//  5. 点击“最小化辅助”，再点击主窗口“显示辅助”：仍能恢复并输入。
//     macOS 原生最小化 Dock 缩略图允许出现，不用隐藏整个应用模拟此行为。
//  6. 关闭辅助窗口后主窗口仍可用，点击“退出”释放两窗口并退出。
//
// -auto：自动断言原生创建标志、显示前定位请求和显示后位置，约 800ms 后退出；
// 不代替任务栏、激活、输入、移动/缩放的真实操作。WM 可调整定位请求，失败须记录。
// 副作用：仅创建/移动自身窗口和标准输出，不修改系统配置或剪贴板。
// 复位：重新启动。退出：主窗口“退出”或关闭主窗口。
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	chrome := flag.String("chrome", "none", "none, native or integrated")
	auto := flag.Bool("auto", false, "check native setup/position and exit")
	flag.Parse()
	modes := map[string]gui.WindowChromeMode{"none": gui.WindowChromeNone, "native": gui.WindowChromeNative, "integrated": gui.WindowChromeIntegrated}
	mode, ok := modes[*chrome]
	if !ok {
		return fmt.Errorf("invalid chrome %q", *chrome)
	}
	app, err := gui.NewApplication("org.golang-gui.TaskbarTest")
	if err != nil {
		return err
	}
	mainWindow, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 440, Height: 360}})
	if err != nil {
		return err
	}
	defer mainWindow.Destroy()
	tool, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 400, Height: 320}, Chrome: mode, SkipTaskbar: true})
	if err != nil {
		return err
	}
	defer tool.Destroy()
	mainWindow.SetTitle("GOUI Taskbar - ordinary")
	tool.SetTitle("GOUI Taskbar - skipped")
	mainWindow.ConnectDestroy(app.Quit)
	closed := false
	tool.ConnectDestroy(func() { closed = true })
	status := gui.NewLabel("辅助窗口可以激活并接收键盘输入")
	count := 0
	button := func(text string, action func()) *gui.Button {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		b.ConnectClicked(action)
		return b
	}
	toolContent := gui.NewLinearBox(layout.DirectionVertical)
	toolContent.SetPadding(12)
	toolContent.SetSpacing(10)
	input := gui.NewTextInput()
	input.ConnectText(func(text string) { fmt.Printf("INPUT %q\n", text) })
	toolContent.AddChild(input)
	toolContent.AddChild(button("计数", func() {
		count++
		status.SetText(fmt.Sprintf("计数=%d；输入=%s", count, input.Text()))
		fmt.Printf("CLICK count=%d text=%q\n", count, input.Text())
	}))
	toolContent.AddChild(button("最小化辅助", func() { tool.RequestState(gui.WindowStateMinimized) }))
	toolContent.AddChild(status)
	toolRoot := gui.NewLinearBox(layout.DirectionVertical)
	header := gui.NewHeaderBar()
	header.SetChild(gui.NewLabel("SkipTaskbar：拖动这里移动"))
	toolRoot.AddChild(header)
	toolRoot.AddChild(toolContent)
	tool.SetWidget(toolRoot)
	if mode == gui.WindowChromeNone {
		tool.Chrome().ConnectQueryRegion(func(p geometry.Point, region *gui.ChromeRegion) {
			r := toolRoot.Rect()
			left, right := p.X < 6, p.X >= r.Width-6
			top, bottom := p.Y < 6, p.Y >= r.Height-6
			switch {
			case top && left:
				*region = gui.ChromeRegionTopLeft
			case top && right:
				*region = gui.ChromeRegionTopRight
			case bottom && left:
				*region = gui.ChromeRegionBottomLeft
			case bottom && right:
				*region = gui.ChromeRegionBottomRight
			case top:
				*region = gui.ChromeRegionTop
			case bottom:
				*region = gui.ChromeRegionBottom
			case left:
				*region = gui.ChromeRegionLeft
			case right:
				*region = gui.ChromeRegionRight
			}
		})
	}
	want := geometry.Point{X: 480, Y: 80}
	position := func() error { return tool.SetPosition(mainWindow, want) }
	check := func() error {
		if closed {
			return fmt.Errorf("辅助窗口已关闭，请重新启动")
		}
		if err := checkNative(mainWindow, false); err != nil {
			return err
		}
		if err := checkNative(tool, true); err != nil {
			return err
		}
		got, err := tool.Position(mainWindow)
		if err != nil {
			return err
		}
		if math.Abs(float64(got.X-want.X)) > 1 || math.Abs(float64(got.Y-want.Y)) > 1 {
			return fmt.Errorf("observed client position %v, want %v", got, want)
		}
		fmt.Printf("PASS native setup and client position=%v\n", got)
		return nil
	}
	mainContent := gui.NewLinearBox(layout.DirectionVertical)
	mainContent.SetPadding(12)
	mainContent.SetSpacing(10)
	mainContent.AddChild(gui.NewLabel("普通窗口：任务栏对照"))
	mainContent.AddChild(button("重新定位", func() { fmt.Println("POSITION", position()) }))
	mainContent.AddChild(button("显示辅助", func() {
		if !closed {
			tool.RequestState(gui.WindowStateNormal)
			fmt.Println("SHOW", tool.Show())
		}
	}))
	mainContent.AddChild(button("检查", func() { fmt.Println("CHECK", check()) }))
	mainContent.AddChild(button("退出", app.Quit))
	mainWindow.SetWidget(mainContent)
	if err := mainWindow.SetPosition(nil, geometry.Point{X: 40, Y: 40}); err != nil {
		return err
	}
	if err := checkNative(mainWindow, false); err != nil {
		return err
	}
	if err := checkNative(tool, true); err != nil {
		return err
	}
	if err := mainWindow.Show(); err != nil {
		return err
	}
	fmt.Printf("OS=%s chrome=%s painter=%q scale=%q ordinary=%#x skipped=%#x\n", runtime.GOOS, *chrome,
		os.Getenv("GOUI_PLAT_PAINTER"), os.Getenv("GOUI_PLAT_SCALE"), mainWindow.PlatformWindow().NativeHandle(), tool.PlatformWindow().NativeHandle())
	var result error
	// Let the WM place the reference window first. SetPosition samples actual
	// geometry once; it does not follow an outstanding move on that reference.
	// These delays are observation/setup for the probe, not backend retries.
	timer := app.NewTimer()
	defer timer.Stop()
	timer.ConnectTimeout(func() {
		if *auto {
			result = check()
			app.Quit()
		}
	})
	showTimer := app.NewTimer()
	defer showTimer.Stop()
	showTimer.ConnectTimeout(func() {
		if result = position(); result == nil {
			result = tool.Show()
		}
		if result == nil && *auto {
			result = timer.StartOnce(400 * time.Millisecond)
		}
		if result != nil {
			app.Quit()
		}
	})
	if err := showTimer.StartOnce(400 * time.Millisecond); err != nil {
		return err
	}
	app.Run()
	return result
}
