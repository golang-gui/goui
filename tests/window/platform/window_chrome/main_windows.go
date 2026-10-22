// Windows Native/Integrated/None client-size and top-corner checks.
//
// Environment: Windows 10/11 x64. Record OS build, system theme, actual Painter,
// system DPI and logged rendering scale. No DevServer is used.
// Run: go run ./tests/window/platform/window_chrome -chrome integrated
// Also use -chrome native/none. -auto checks only hidden initial/minimum geometry
// and exits; it does not verify visible corners. Repeat with
// GOUI_PLAT_PAINTER=software/opengl (default preference: Direct2D) and
// GOUI_PLAT_SCALE=1 or 2, then real 100%/200% system DPI.
// Initially request 640x400 DIP client area filled dark; the log checks client
// and minimum sizes automatically.
// Steps and expected results:
//  1. On a dark Windows 11 desktop, both top corners have no extra white pixels
//     or bright row. Windows 10 may retain square native edges and one physical
//     top pixel; no native caption covers custom content.
//  2. F7 switches solid light/dark fill; activate another app and return. No old
//     fill or extra white strip remains. F7 does not change the system theme;
//     verify system theme changes separately and restore settings afterward.
//  3. Drag the middle of Integrated's top 40 DIP and resize all edges/corners.
//     F6 checks client geometry against SizeEvent. Win32 still owns top-edge
//     cursor and drag hit testing.
//  4. F8 maximizes, F6 checks, F9 restores and F6 checks again. Maximized client
//     area matches the work area without the normal top pixel. Restore returns
//     the OS-version-specific border without caption or rounded-corner seams.
//  5. Restart Native/None separately. Native requested size excludes the caption;
//     None has no native border. Both initially have exactly 640x400 DIP client
//     area; minimum hints describe a 320x200 DIP client area.
//
// Side effects: this process's windows/logs only; F8/F9 change their state.
// No files/clipboard writes. Reset: F9 normal state, F7 dark fill, restart for size.
// Exit: Esc or close the window. Geometry failures exit nonzero; PASS geometry
// does not verify composition/corners. Session 0 can check hidden geometry only,
// not interactive appearance, system themes or real DPI.
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
