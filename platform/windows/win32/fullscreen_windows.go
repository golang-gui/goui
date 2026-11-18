package win32

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

type fullscreenWindow struct {
	style     winapi.DWORD
	placement winapi.WINDOWPLACEMENT
}

func fullscreenStyle(style winapi.DWORD) winapi.DWORD {
	return style&^(winapi.WS_OVERLAPPEDWINDOW|winapi.WS_MINIMIZE|winapi.WS_MAXIMIZE) | winapi.WS_POPUP
}

func setFullscreenBounds(m *winapi.MINMAXINFO, r winapi.RECT) {
	size := winapi.POINT{X: r.Right - r.Left, Y: r.Bottom - r.Top}
	m.MaxPosition = winapi.POINT{}
	m.MaxSize, m.MaxTrackSize = size, size
	m.MinTrackSize = winapi.POINT{}
}

func (w *Window) observedFullscreen() bool {
	style, err := winapi.GetWindowLong(w.hwnd, winapi.GWL_STYLE)
	if err != nil || winapi.DWORD(style)&(winapi.WS_CAPTION|winapi.WS_THICKFRAME) != 0 {
		return false
	}
	var rect winapi.RECT
	if winapi.GetWindowRect(w.hwnd, &rect) != nil {
		return false
	}
	m, err := w.monitorInfo()
	return err == nil && rect == m.Monitor
}

func (w *Window) fitFullscreen() error {
	if w.closed || w.hwnd == 0 {
		return common.ErrUnavailable
	}
	monitor, err := w.monitorInfo()
	if err != nil {
		return err
	}
	return w.fitFullscreenBounds(monitor.Monitor)
}

func (w *Window) fitFullscreenBounds(r winapi.RECT) error {
	if w.closed || w.hwnd == 0 {
		return common.ErrUnavailable
	}
	var rect winapi.RECT
	if err := winapi.GetWindowRect(w.hwnd, &rect); err != nil {
		return err
	}
	if rect == r {
		return nil
	}
	return winapi.SetWindowPos(w.hwnd, winapi.HWND_TOP, int(r.Left), int(r.Top), int(r.Right-r.Left), int(r.Bottom-r.Top),
		winapi.SWP_FRAMECHANGED|winapi.SWP_NOACTIVATE)
}

func (w *Window) enterFullscreen() error {
	if w.fullscreen != nil {
		return w.Show()
	}
	style, err := winapi.GetWindowLong(w.hwnd, winapi.GWL_STYLE)
	if err != nil {
		return err
	}
	saved := &fullscreenWindow{style: winapi.DWORD(style), placement: winapi.WINDOWPLACEMENT{Length: winapi.UINT(unsafe.Sizeof(winapi.WINDOWPLACEMENT{}))}}
	if err := winapi.GetWindowPlacement(w.hwnd, &saved.placement); err != nil {
		return err
	}
	// Observe the target before changing a maximized/restored window's frame.
	monitor, err := w.monitorInfo()
	if err != nil {
		return err
	}
	w.fullscreen = saved
	rollback := func(cause error) error {
		if w.closed || w.hwnd == 0 {
			return cause
		}
		return errors.Join(cause, w.leaveFullscreen(common.WindowStateNormal))
	}
	if _, err := winapi.SetWindowLong(w.hwnd, winapi.GWL_STYLE, winapi.LONG(fullscreenStyle(saved.style))); err != nil {
		w.fullscreen = nil
		return err
	}
	if w.closed || w.hwnd == 0 {
		return common.ErrUnavailable
	}
	if w.integrated {
		if err := w.extendFrame(); err != nil {
			return rollback(err)
		}
	}
	if winapi.IsIconic(w.hwnd) != winapi.FALSE || winapi.IsZoomed(w.hwnd) != winapi.FALSE {
		winapi.ShowWindow(w.hwnd, winapi.SW_SHOWNOACTIVATE)
	}
	if w.closed || w.hwnd == 0 {
		return common.ErrUnavailable
	}
	r := monitor.Monitor
	if err := winapi.SetWindowPos(w.hwnd, winapi.HWND_TOP, int(r.Left), int(r.Top), int(r.Right-r.Left), int(r.Bottom-r.Top),
		winapi.SWP_FRAMECHANGED|winapi.SWP_NOACTIVATE); err != nil {
		return rollback(err)
	}
	if err := w.Show(); err != nil {
		return rollback(err)
	}
	return nil
}

func (w *Window) leaveFullscreen(target common.WindowState) error {
	saved := w.fullscreen
	if saved == nil {
		return nil
	}
	style, err := winapi.GetWindowLong(w.hwnd, winapi.GWL_STYLE)
	if err != nil {
		return err
	}
	// Visibility is a current fact; normal presentation never restores a saved
	// maximized/minimized bit or a historical ShowCmd.
	restored := saved.style &^ (winapi.WS_VISIBLE | winapi.WS_MINIMIZE | winapi.WS_MAXIMIZE)
	restored |= winapi.DWORD(style) & winapi.WS_VISIBLE
	if _, err := winapi.SetWindowLong(w.hwnd, winapi.GWL_STYLE, winapi.LONG(restored)); err != nil {
		return err
	}
	if w.closed || w.hwnd == 0 {
		return common.ErrUnavailable
	}
	w.fullscreen = nil
	if w.integrated {
		if err := w.extendFrame(); err != nil {
			return err
		}
	}
	placement := saved.placement
	placement.ShowCmd, placement.Flags = winapi.SW_SHOWNORMAL, 0
	if target == common.WindowStateMaximized {
		placement.ShowCmd = winapi.SW_SHOWMAXIMIZED
	}
	if err := winapi.SetWindowPlacement(w.hwnd, &placement); err != nil {
		return fmt.Errorf("restore fullscreen placement: %w", err)
	}
	if w.closed || w.hwnd == 0 {
		return common.ErrUnavailable
	}
	return winapi.SetWindowPos(w.hwnd, 0, 0, 0, 0, 0,
		winapi.SWP_FRAMECHANGED|winapi.SWP_NOMOVE|winapi.SWP_NOSIZE|winapi.SWP_NOZORDER|winapi.SWP_NOACTIVATE)
}
