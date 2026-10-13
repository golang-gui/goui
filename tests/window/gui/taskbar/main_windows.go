package main

import (
	"fmt"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

func checkNative(window gui.Window, skip bool) error {
	hwnd := winapi.HWND(window.PlatformWindow().NativeHandle())
	style, err := winapi.GetWindowLong(hwnd, winapi.GWL_EXSTYLE)
	if err != nil {
		return err
	}
	if (style&winapi.WS_EX_TOOLWINDOW != 0) != skip || style&(winapi.WS_EX_NOACTIVATE|winapi.WS_EX_TOPMOST) != 0 {
		return fmt.Errorf("unexpected extended style %#x skip=%v", style, skip)
	}
	var r winapi.RECT
	if err := winapi.GetClientRect(hwnd, &r); err != nil {
		return err
	}
	if r.Right <= r.Left || r.Bottom <= r.Top {
		return fmt.Errorf("empty client area: %+v", r)
	}
	if window.State() == gui.WindowStateHidden {
		// Independently observe creation geometry, including Native's shorter
		// tool caption. Later user resizing must not be compared with this request.
		dpi, err := winapi.GetDpiForWindow(hwnd)
		if err != nil {
			return err
		}
		scale := float32(dpi) / 96
		if preferred := common.GetPreferScale(); preferred > 0 {
			scale = preferred
		}
		width, height := float32(440), float32(360)
		if skip {
			width, height = 400, 320
		}
		if r.Right-r.Left != winapi.LONG(width*scale) || r.Bottom-r.Top != winapi.LONG(height*scale) {
			return fmt.Errorf("initial client size %+v, want %gx%g DIP at scale %g", r, width, height, scale)
		}
	}
	return nil
}
