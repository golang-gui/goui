package main

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

func checkWindowChromeCustomSize(plat platform.Platform) {
	var err error
	for _, chrome := range []platform.WindowChrome{platform.WindowChromeNative, platform.WindowChromeNone, platform.WindowChromeIntegrated} {
		func() {
			fmt.Println("CHECK", fmt.Sprint(chrome))
			err = checkCustomWindowSize(plat, chrome)
			if chrome == platform.WindowChromeIntegrated && errors.Is(err, platform.ErrUnsupported) {
				panic(fmt.Errorf("native composition unavailable: %v", err))
			}
			if err != nil {
				panic(err)
			}
		}()
	}
}

func checkCustomWindowSize(plat platform.Platform, chrome platform.WindowChrome) error {
	var size events.SizeEvent
	win, err := plat.NewWindow(geometry.Size{Width: 640, Height: 400}, func(event platform.Event) {
		if event, ok := event.(events.SizeEvent); ok {
			size = event
		}
	}, platform.WindowOptions{Chrome: chrome})
	if err != nil {
		return err
	}
	defer win.Destroy()
	desktop := win.(platform.WindowManagement)
	if desktop.Chrome() != chrome {
		return fmt.Errorf("chrome=%v, want %v", desktop.Chrome(), chrome)
	}
	hwnd := winapi.HWND(win.NativeHandle())
	var outer, client winapi.RECT
	if err := winapi.GetWindowRect(hwnd, &outer); err != nil {
		return err
	}
	if err := winapi.GetClientRect(hwnd, &client); err != nil {
		return err
	}
	scale := float32(winapi.GetDpiForSystem()) / 96
	if override := common.GetPreferScale(); override > 0 {
		scale = override
	}
	if client.Right != winapi.LONG(640*scale) || client.Bottom != winapi.LONG(400*scale) {
		return fmt.Errorf("requested client 640x400 DIP: client=%+v outer=%+v scale=%g", client, outer, scale)
	}

	if chrome != platform.WindowChromeNative || size.Width > 0 {
		if size.PixelWidth != float32(client.Right) || size.PixelHeight != float32(client.Bottom) {
			return fmt.Errorf("SizeEvent disagrees with native client: event=%+v client=%+v", size, client)
		}
	}
	var origin winapi.POINT
	if winapi.ClientToScreen(hwnd, &origin) == winapi.FALSE {
		return fmt.Errorf("query client origin failed")
	}
	var topBorder winapi.LONG
	if chrome == platform.WindowChromeIntegrated {
		style, err := winapi.GetWindowLong(hwnd, winapi.GWL_STYLE)
		if err != nil {
			return err
		}

		if style&winapi.WS_CAPTION == 0 {
			topBorder = 1
		}
		if origin.Y-outer.Top != topBorder {
			return fmt.Errorf("normal top border: outer=%+v origin=%+v want=%d", outer, origin, topBorder)
		}
	}
	if chrome == platform.WindowChromeNone && (origin.X != outer.Left || origin.Y != outer.Top || client.Right != outer.Right-outer.Left || client.Bottom != outer.Bottom-outer.Top) {
		return fmt.Errorf("None retained non-client insets: outer=%+v client=%+v origin=%+v", outer, client, origin)
	}

	actual, expected := outer, outer
	var pin runtime.Pinner
	defer pin.Unpin()
	pin.Pin(&actual)
	pin.Pin(&expected)
	wantResult := winapi.DefWindowProc(hwnd, winapi.WM_NCCALCSIZE, 0, winapi.LPARAM(uintptr(unsafe.Pointer(&expected))))
	result := winapi.SendMessage(hwnd, winapi.WM_NCCALCSIZE, 0, winapi.LPARAM(uintptr(unsafe.Pointer(&actual))))
	runtime.KeepAlive(&expected)
	runtime.KeepAlive(&actual)
	if actual != expected || result != wantResult {
		return fmt.Errorf("WM_NCCALCSIZE(false): got %+v/%d, want %+v/%d", actual, result, expected, wantResult)
	}
	if chrome == platform.WindowChromeIntegrated {
		pos := winapi.WINDOWPOS{Hwnd: hwnd, X: outer.Left, Y: outer.Top, CX: outer.Right - outer.Left, CY: outer.Bottom - outer.Top}
		oldClient := winapi.RECT{Left: origin.X, Top: origin.Y, Right: origin.X + client.Right, Bottom: origin.Y + client.Bottom}
		want := winapi.NCCALCSIZE_PARAMS{Rects: [3]winapi.RECT{outer, outer, oldClient}, WindowPos: &pos}
		got := want
		pin.Pin(&pos)
		pin.Pin(&want)
		pin.Pin(&got)
		wantResult := winapi.DefWindowProc(hwnd, winapi.WM_NCCALCSIZE, 1, winapi.LPARAM(uintptr(unsafe.Pointer(&want))))
		want.Rects[0].Top = outer.Top + topBorder
		result := winapi.SendMessage(hwnd, winapi.WM_NCCALCSIZE, 1, winapi.LPARAM(uintptr(unsafe.Pointer(&got))))
		runtime.KeepAlive(&want)
		runtime.KeepAlive(&got)
		runtime.KeepAlive(&pos)
		if got.Rects != want.Rects || result != wantResult {
			return fmt.Errorf("normal WM_NCCALCSIZE(true): got %+v/%d, want %+v/%d", got.Rects, result, want.Rects, wantResult)
		}
	}

	desktop.SetMinSize(320, 200)
	mmi := winapi.MINMAXINFO{}
	pin.Pin(&mmi)
	winapi.SendMessage(hwnd, winapi.WM_GETMINMAXINFO, 0, winapi.LPARAM(uintptr(unsafe.Pointer(&mmi))))
	runtime.KeepAlive(&mmi)
	if size.Width > 0 {
		scale = size.PixelWidth / size.Width
	}
	wantMin := winapi.POINT{
		X: winapi.LONG(320*scale) + outer.Right - outer.Left - client.Right,
		Y: winapi.LONG(200*scale) + outer.Bottom - outer.Top - client.Bottom,
	}
	if mmi.MinTrackSize != wantMin {
		return fmt.Errorf("client minimum hint: got %+v, want %+v", mmi.MinTrackSize, wantMin)
	}
	desktop.SetMinSize(0, 0)
	mmi.MinTrackSize = winapi.POINT{X: 90, Y: 70}
	defaults := mmi
	pin.Pin(&defaults)
	if chrome != platform.WindowChromeIntegrated {
		winapi.DefWindowProc(hwnd, winapi.WM_GETMINMAXINFO, 0, winapi.LPARAM(uintptr(unsafe.Pointer(&defaults))))
		runtime.KeepAlive(&defaults)
	}
	winapi.SendMessage(hwnd, winapi.WM_GETMINMAXINFO, 0, winapi.LPARAM(uintptr(unsafe.Pointer(&mmi))))
	runtime.KeepAlive(&mmi)
	if mmi.MinTrackSize != defaults.MinTrackSize {
		return fmt.Errorf("cleared custom minimum hint changed defaults: %+v", mmi.MinTrackSize)
	}
	return nil
}

func checkWindowChromeIntegratedNativeGeometry(plat platform.Platform) {
	var err error
	err = checkIntegratedNativeGeometry(plat)
	if err != nil {
		panic(err)
	}
}

func checkIntegratedNativeGeometry(plat platform.Platform) error {
	var size events.SizeEvent
	win, err := plat.NewWindow(geometry.Size{Width: 640, Height: 400}, func(event platform.Event) {
		if event, ok := event.(events.SizeEvent); ok {
			size = event
		}
	}, platform.WindowOptions{Chrome: platform.WindowChromeIntegrated})
	if err != nil {
		return err
	}
	defer win.Destroy()
	desktop := win.(platform.WindowManagement)
	hwnd := winapi.HWND(win.NativeHandle())
	var normal winapi.RECT
	for _, state := range []platform.WindowState{platform.WindowStateNormal, platform.WindowStateMaximized, platform.WindowStateNormal} {
		if err := desktop.RequestState(state); err != nil {
			return err
		}
		if desktop.State() != state {
			return fmt.Errorf("native state=%v, want %v", desktop.State(), state)
		}
		var client winapi.RECT
		if err := winapi.GetClientRect(hwnd, &client); err != nil {
			return err
		}
		if size.PixelWidth != float32(client.Right) || size.PixelHeight != float32(client.Bottom) {
			return fmt.Errorf("state %v: SizeEvent=%+v, client=%+v", state, size, client)
		}
		if state == platform.WindowStateNormal {
			if normal == (winapi.RECT{}) {
				normal = client
			} else if client != normal {
				return fmt.Errorf("restore changed client size: before=%+v after=%+v", normal, client)
			}
			continue
		}
		var origin winapi.POINT
		if winapi.ClientToScreen(hwnd, &origin) == winapi.FALSE {
			return fmt.Errorf("query maximized client origin failed")
		}
		client.Left, client.Top = origin.X, origin.Y
		client.Right, client.Bottom = client.Right+origin.X, client.Bottom+origin.Y
		monitor := winapi.MONITORINFO{Size: winapi.DWORD(unsafe.Sizeof(winapi.MONITORINFO{}))}
		if err := winapi.GetMonitorInfo(winapi.MonitorFromWindow(hwnd, winapi.MONITOR_DEFAULTTONEAREST), &monitor); err != nil {
			return err
		}
		if client != monitor.Work {
			return fmt.Errorf("maximized client=%+v, work area=%+v", client, monitor.Work)
		}

		nc := winapi.NCCALCSIZE_PARAMS{Rects: [3]winapi.RECT{monitor.Monitor}}
		var pin runtime.Pinner
		pin.Pin(&nc)
		result := winapi.SendMessage(hwnd, winapi.WM_NCCALCSIZE, 1, winapi.LPARAM(uintptr(unsafe.Pointer(&nc))))
		pin.Unpin()
		runtime.KeepAlive(&nc)
		if result != 0 || nc.Rects[0] != monitor.Work {
			return fmt.Errorf("maximized NCCALCSIZE: result=%d client=%+v work=%+v", result, nc.Rects[0], monitor.Work)
		}
	}
	return nil
}

func checkNativeGeometry(plat platform.Platform) {
	checkWindowChromeCustomSize(plat)
	checkWindowChromeIntegratedNativeGeometry(plat)
}
