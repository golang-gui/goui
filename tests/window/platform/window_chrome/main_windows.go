// Windows Native/Integrated/None 客户区尺寸与顶部圆角验证。
//
// 环境：Windows 10 或 Windows 11 x64；记录 OS Build、系统主题、实际 Painter、
// 系统 DPI 和日志中的渲染缩放。窗口检查不使用 DevServer。
// 启动：go run ./tests/window/platform/window_chrome -chrome integrated
// 可选：-chrome native/none；-auto 仅检查隐藏窗口的初始/最小尺寸并自动退出，
// 不代表桌面圆角验收。用 GOUI_PLAT_PAINTER=software/opengl（默认 Direct2D），
// GOUI_PLAT_SCALE=1/2 重复；再在真实 100%/200% 系统 DPI 下验证。
// 初始：请求 640×400 DIP 客户区，纯暗色填满，终端自动核对客户区和最小尺寸。
//
// 操作与预期：
//  1. Windows 11 暗色桌面观察顶部两角：无额外亮白像素/一行露白；Windows 10
//     允许系统直角边框，顶部保留 1 物理像素，无原生标题栏覆盖自绘区域。
//  2. F7 切换纯亮/纯暗背景，再点击其它应用后返回；边缘没有旧背景和额外白条。
//     本键不改变系统主题；系统亮暗切换另行操作，验收后恢复原系统设置。
//  3. Integrated 上方 40 DIP 中间区域拖动窗口，四边/四角缩放；F6 检查实际
//     客户区与 SizeEvent 一致。顶部缩放光标和拖动行为仍由 Win32 命中处理。
//  4. F8 最大化、F6 检查、F9 恢复、F6 再检查：最大化客户区等于当前工作区，
//     不保留普通顶部像素；恢复后版本对应边框重现，无标题栏或圆角接缝残留。
//  5. Native/None 分别重启：Native 请求尺寸不包含标题栏，None 无原生边框；
//     初始客户区均准确为 640×400 DIP，最小尺寸提示均描述 320×200 DIP 客户区。
//
// 副作用：仅本进程窗口、终端日志；F8/F9 改变本窗口状态，不写文件/剪贴板。
// 复位：F9 恢复、F7 切回暗色；重新启动重置大小。退出：Esc 或关闭窗口。
// 几何失败非零退出；PASS 几何日志不代表圆角/合成外观通过。远程 Session 0
// 可以执行隐藏几何检查，但不能代替可交互桌面、系统主题和真实 DPI 验收。
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	auto := flag.Bool("auto", false, "check hidden initial and minimum client geometry, then exit")
	mode := flag.String("chrome", "integrated", "native, integrated or none")
	flag.Parse()
	chrome, ok := map[string]platform.WindowChrome{
		"native": platform.WindowChromeNative, "integrated": platform.WindowChromeIntegrated, "none": platform.WindowChromeNone,
	}[*mode]
	if !ok {
		return fmt.Errorf("unknown chrome %q", *mode)
	}
	p, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.WindowChromeProbe")
	if err != nil {
		return err
	}
	defer p.Destroy()
	loop, err := p.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	var win platform.Window
	var desktop platform.DesktopWindow
	var painter graphics.Painter
	var size events.SizeEvent
	var result error
	dark := true
	win, err = p.NewWindow(geometry.Size{Width: 640, Height: 400}, func(event events.Event) {
		switch e := event.(type) {
		case events.SizeEvent:
			size = e
		case events.CloseEvent:
			loop.Quit()
		case events.PaintEvent:
			if painter == nil || size.Width <= 0 || size.Height <= 0 {
				return
			}
			painter.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
			color := graphics.RGB(32, 32, 36)
			if !dark {
				color = graphics.RGB(245, 245, 247)
			}
			painter.Clear(color)
			painter.End()
		case events.KeyEvent:
			if e.EventType != events.KeyDown {
				return
			}
			switch e.Key {
			case events.KeyEscape:
				loop.Quit()
			case events.KeyF6:
				result = checkGeometry(desktop, size, false)
			case events.KeyF7:
				dark = !dark
				result = win.RequestPaint()
			case events.KeyF8:
				result = desktop.RequestState(platform.WindowStateMaximized)
			case events.KeyF9:
				result = desktop.RequestState(platform.WindowStateNormal)
			}
			if result != nil {
				loop.Quit()
			}
		}
	}, platform.WindowOptions{Chrome: chrome})
	if err != nil {
		return err
	}
	defer win.Destroy()
	desktop = win.(platform.DesktopWindow)
	if desktop.Chrome() != chrome {
		return fmt.Errorf("observed chrome %v, requested %v", desktop.Chrome(), chrome)
	}
	if err := checkGeometry(desktop, size, true); err != nil {
		return err
	}
	if *auto {
		return nil
	}
	if err := win.SetTitle("GOUI Window Chrome — F6 check / F7 theme / F8 maximize / F9 normal / Esc exit"); err != nil {
		return err
	}
	if chrome == platform.WindowChromeIntegrated {
		if err := desktop.SetHitTest(func(point geometry.Point) platform.WindowHit {
			if point.Y < 40 && point.X > 24 && point.X < size.Width-24 {
				return platform.WindowHitCaption
			}
			return platform.WindowHitDefault
		}); err != nil {
			return err
		}
	}
	painter, err = p.NewPainter(win)
	if err != nil {
		return err
	}
	defer painter.Destroy()
	fmt.Printf("backend=%s\n", painter.Name()) // Native may defer WM_SIZE until Show.
	if err := win.Show(); err != nil {
		return err
	}
	loop.Run()
	return result
}

func checkGeometry(win platform.DesktopWindow, size events.SizeEvent, initial bool) error {
	hwnd := winapi.HWND(win.NativeHandle())
	var outer, client winapi.RECT
	if err := winapi.GetWindowRect(hwnd, &outer); err != nil {
		return err
	}
	if err := winapi.GetClientRect(hwnd, &client); err != nil {
		return err
	}
	dpi, err := winapi.GetDpiForWindow(hwnd)
	if err != nil {
		return err
	}
	scale := float32(dpi) / 96
	if preferred := common.GetPreferScale(); preferred > 0 {
		scale = preferred
	}
	// Native may defer its first WM_SIZE until Show. Geometry exists already;
	// validate notifications only when supplied, never synthesize a SizeEvent.
	if !initial || win.Chrome() != platform.WindowChromeNative || size.Width > 0 {
		if size.PixelWidth != float32(client.Right) || size.PixelHeight != float32(client.Bottom) || size.Width <= 0 {
			return fmt.Errorf("SizeEvent=%+v, actual client=%+v", size, client)
		}
		scale = size.PixelWidth / size.Width
	}
	if initial && (client.Right != winapi.LONG(640*scale) || client.Bottom != winapi.LONG(400*scale)) {
		return fmt.Errorf("requested 640x400 DIP client, got %+v at %g", client, scale)
	}
	var origin winapi.POINT
	if winapi.ClientToScreen(hwnd, &origin) == winapi.FALSE {
		return fmt.Errorf("query client origin failed")
	}
	if win.State() == platform.WindowStateMaximized && win.Chrome() == platform.WindowChromeIntegrated {
		monitor := winapi.MONITORINFO{Size: winapi.DWORD(unsafe.Sizeof(winapi.MONITORINFO{}))}
		if err := winapi.GetMonitorInfo(winapi.MonitorFromWindow(hwnd, winapi.MONITOR_DEFAULTTONEAREST), &monitor); err != nil {
			return err
		}
		actual := winapi.RECT{Left: origin.X, Top: origin.Y, Right: origin.X + client.Right, Bottom: origin.Y + client.Bottom}
		if actual != monitor.Work {
			return fmt.Errorf("maximized client=%+v, work=%+v", actual, monitor.Work)
		}
	}
	if initial {
		win.SetMinSize(320, 200)
		mmi := new(winapi.MINMAXINFO)
		var pin runtime.Pinner
		pin.Pin(mmi) // LPARAM carries this address through a native-to-Go callback.
		defer pin.Unpin()
		winapi.SendMessage(hwnd, winapi.WM_GETMINMAXINFO, 0, winapi.LPARAM(uintptr(unsafe.Pointer(mmi))))
		want := winapi.POINT{X: winapi.LONG(320*scale) + outer.Right - outer.Left - client.Right,
			Y: winapi.LONG(200*scale) + outer.Bottom - outer.Top - client.Bottom}
		if mmi.MinTrackSize != want {
			return fmt.Errorf("client minimum: got %+v, want %+v", mmi.MinTrackSize, want)
		}
	}
	fmt.Printf("PASS geometry chrome=%v state=%v client=%+v top=%d dpi=%d scale=%g initial=%t\n",
		win.Chrome(), win.State(), client, origin.Y-outer.Top, dpi, scale, initial)
	return nil
}
