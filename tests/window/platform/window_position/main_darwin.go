// macOS 普通窗口默认位置验收，不使用 DevServer。
//
// 环境：macOS arm64 可交互桌面，从 main 创建平台并运行；记录系统版本、后端和缩放。
// 启动：go run ./tests/window/platform/window_position [-chrome=native|integrated|none] [-auto]
// 可设置 GOUI_PLAT_PAINTER=software/opengl、GOUI_PLAT_SCALE=1/2。
// 初始：请求 480×320 DIP 的普通窗口，无 SetPosition 请求；应在 AppKit 的原生居中
// 位置出现，而不是桌面左下角。Native 有原生标题栏，Integrated/None 可没有标题文字。
//
// 操作与逐项预期：
//  1. 分别用三种 Chrome 启动：初始窗口水平居中、垂直略偏上，无显示后搬动闪烁。
//     终端 PASS default 核对隐藏状态和原生 frame/visibleFrame，不以截图代替断言。
//  2. 按 F5：客户区定位到当前工作区 (120,80) DIP，日志确认实际 Position。
//  3. 按 F6：隐藏后重新显示，必须保持当前位置，不重新居中。
//     Native 可先拖动系统标题栏再按 F6，仍保持用户拖动后的原生 frame。
//  4. 按 F7：显示 Popup，左上角相对本窗口客户区 (20,40) DIP；再按一次隐藏。
//     Popup 不得居中屏幕，普通窗口的位置也不变。
//  5. 按 Esc 或点击原生关闭按钮：释放窗口、Popup、Painter 和事件循环并退出。
//
// -auto：在真正主线程创建原生窗口，断言隐藏默认位置、显示、显式定位、重复 Hide/Show
// 和 Popup 初始/锚点位置后退出；失败非零退出。包含桌面依赖，不由 go test 自动执行。
// 自动模式不代替可见闪烁、原生拖动、多屏/混合 DPI、Stage Manager 或 Spaces 验收。
// 副作用：仅创建/定位/显示本程序窗口，激活应用；不改系统设置或剪贴板。
// 复位：重新启动。None 模式通过键盘退出，未模拟原生标题栏或窗口按钮。
package main

import (
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	"github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	auto := flag.Bool("auto", false, "check native placement and exit")
	mode := flag.String("chrome", "native", "native, integrated or none")
	flag.Parse()
	chrome, ok := map[string]platform.WindowChrome{
		"native": platform.WindowChromeNative, "integrated": platform.WindowChromeIntegrated, "none": platform.WindowChromeNone,
	}[*mode]
	if !ok {
		return fmt.Errorf("unknown chrome %q", *mode)
	}
	p, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.DefaultPositionProbe")
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
	var popup platform.Popup
	var desktop platform.DesktopWindow
	var painter graphics.Painter
	var size events.SizeEvent
	var result error
	popupShown := false
	position := func() error {
		want := geometry.Point{X: 120, Y: 80}
		if err := desktop.SetPosition(nil, want); err != nil {
			return err
		}
		got, err := desktop.Position(nil)
		if err != nil || math.Abs(float64(got.X-want.X)) > .5 || math.Abs(float64(got.Y-want.Y)) > .5 {
			return fmt.Errorf("explicit position: got %v, want %v: %v", got, want, err)
		}
		fmt.Println("PASS explicit client position", got)
		return nil
	}
	reshow := func() error {
		before := nativeFrame(win)
		if err := win.Hide(); err != nil {
			return err
		}
		if err := win.Show(); err != nil {
			return err
		}
		if after := nativeFrame(win); after != before {
			return fmt.Errorf("Hide/Show moved frame: before=%v after=%v", before, after)
		}
		fmt.Println("PASS Hide/Show preserves frame", before)
		return nil
	}
	win, err = p.NewWindow(geometry.Size{Width: 480, Height: 320}, func(event platform.Event) {
		switch e := event.(type) {
		case events.SizeEvent:
			size = e
		case events.CloseEvent:
			loop.Quit()
		case events.PaintEvent:
			if painter != nil && size.Width > 0 && size.Height > 0 {
				painter.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
				painter.Clear(graphics.RGB(225, 237, 247))
				painter.End()
			}
		case events.KeyEvent:
			if e.EventType == events.KeyDown {
				switch e.Key {
				case events.KeyEscape:
					loop.Quit()
				case events.KeyF5:
					result = position()
				case events.KeyF6:
					result = reshow()
				case events.KeyF7:
					if popupShown {
						result = popup.Hide()
					} else {
						result = showPopup(win, popup, size)
					}
					popupShown = !popupShown
				}
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
	if err := checkDefault(win); err != nil {
		return err
	}
	initial := nativeFrame(win)
	popup, err = p.NewPopup(win, geometry.Size{Width: 160, Height: 80}, func(platform.Event) {}, platform.PopupOptions{})
	if err != nil {
		return err
	}
	defer popup.Destroy()
	if frame := nativeFrame(popup); frame.Origin.X != 0 || frame.Origin.Y != 0 {
		return fmt.Errorf("popup creation was repositioned: %v", frame)
	}
	if err := win.SetTitle("GOUI default position — F5 place / F6 reshow / F7 popup / Esc exit"); err != nil {
		return err
	}
	if !*auto {
		painter, err = p.NewPainter(win)
		if err != nil {
			return err
		}
		defer painter.Destroy()
		fmt.Println("backend="+painter.Name(), "chrome="+*mode, "scale="+os.Getenv("GOUI_PLAT_SCALE"))
	}
	if err := win.Show(); err != nil {
		return err
	}
	if frame := nativeFrame(win); frame != initial {
		return fmt.Errorf("first Show moved frame: before=%v after=%v", initial, frame)
	}
	if *auto {
		if err := position(); err != nil {
			return err
		}
		for range 2 {
			if err := reshow(); err != nil {
				return err
			}
		}
		return showPopup(win, popup, size)
	}
	loop.Run()
	return result
}

// Native screen geometry is an independent expected value; never call Center
// from the probe or compare two windows created by the same positioning code.
func checkDefault(win platform.Window) (err error) {
	foundation.AutoReleasePool(func() {
		var native appkit.NSWindow
		native.ID = foundation.ID(win.NativeHandle())
		if native.IsVisible() {
			err = fmt.Errorf("new window became visible before Show")
			return
		}
		frame := native.Frame()
		screens := appkit.NSScreenClassId.Screens()
		for i := uintptr(0); i < screens.Count(); i++ {
			var screen appkit.NSScreen
			screen.ID = screens.ObjectAtIndex(i)
			area := screen.VisibleFrame()
			// One AppKit point permits native pixel rounding at 1x/2x. The
			// vertical contract is above center, not an invented fixed ratio.
			centerX := frame.Origin.X + frame.Size.Width/2
			centerY := frame.Origin.Y + frame.Size.Height/2
			if math.Abs(float64(centerX-area.Origin.X-area.Size.Width/2)) <= 1 &&
				centerY >= area.Origin.Y+area.Size.Height/2 &&
				frame.Origin.Y+frame.Size.Height <= area.Origin.Y+area.Size.Height+1 {
				fmt.Printf("PASS default hidden frame=%v visibleFrame=%v\n", frame, area)
				return
			}
		}
		err = fmt.Errorf("default frame is not natively centered: %v", frame)
	})
	return
}

func nativeFrame(surface platform.Surface) (frame foundation.NSRect) {
	foundation.AutoReleasePool(func() {
		var native appkit.NSWindow
		native.ID = foundation.ID(surface.NativeHandle())
		frame = native.Frame()
	})
	return
}

func showPopup(win platform.Window, popup platform.Popup, size events.SizeEvent) error {
	before := nativeFrame(win)
	popup.SetPosition(20, 40)
	if err := popup.Show(); err != nil {
		return err
	}
	var content foundation.NSRect
	foundation.AutoReleasePool(func() {
		var native appkit.NSWindow
		native.ID = foundation.ID(win.NativeHandle())
		content = native.ContentRectForFrameRect(before)
	})
	// Reported backing pixels per DIP / backing pixels per point is the
	// logical-to-native scale, independently of the requested override.
	var backing float64
	foundation.AutoReleasePool(func() {
		var native appkit.NSWindow
		native.ID = foundation.ID(win.NativeHandle())
		backing = float64(native.BackingScaleFactor())
	})
	if size.Width <= 0 || size.PixelWidth <= 0 || backing <= 0 {
		return fmt.Errorf("popup scale unavailable: size=%v backing=%g", size, backing)
	}
	scale := float64(size.PixelWidth/size.Width) / backing
	frame := nativeFrame(popup)
	wantX := float64(content.Origin.X) + 20*scale
	wantY := float64(content.Origin.Y+content.Size.Height) - 40*scale
	if math.Abs(float64(frame.Origin.X)-wantX) > 1 || math.Abs(float64(frame.Origin.Y+frame.Size.Height)-wantY) > 1 || nativeFrame(win) != before {
		return fmt.Errorf("popup anchor: frame=%v want top-left=(%g,%g)", frame, wantX, wantY)
	}
	fmt.Println("PASS popup owner-local anchor", frame)
	return nil
}
