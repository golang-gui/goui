package win32

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/win32/winapi"
)

type Window struct {
	hwnd    winapi.HWND
	parent  common.Window
	onEvent events.EventHandler
}

func newWindow(onEvent events.EventHandler) (common.Window, error) {
	err := registerWindow()
	if err != nil {
		return nil, err
	}
	win := &Window{
		onEvent: onEvent,
	}

	win.hwnd, err = winapi.CreateWindowEx(0, windowClass, windowTitle, winapi.WS_OVERLAPPEDWINDOW,
		winapi.CW_USEDEFAULT, winapi.CW_USEDEFAULT,
		winapi.CW_USEDEFAULT, winapi.CW_USEDEFAULT,
		0, 0, instance,
		unsafe.Pointer(win))

	if err != nil {
		return nil, err
	}

	runtime.KeepAlive(win)
	return win, nil
}

func (w *Window) NativeHandle() uintptr {
	return uintptr(w.hwnd)
}

func (w *Window) Destroy() {
	winapi.DestroyWindow(w.hwnd)
}

func (w *Window) Parent() common.Window {
	return w.parent
}

func (w *Window) SetParent(parent common.Window) error {
	_, err := winapi.SetParent(w.hwnd, winapi.HWND(parent.NativeHandle()))
	if err != nil {
		return err
	}
	w.parent = parent
	return nil
}

func (w *Window) Title() string {
	length, err := winapi.GetWindowTextLength(w.hwnd)
	if err != nil {
		return ""
	}

	buf := make([]uint16, length+1)
	_, err = winapi.GetWindowText(w.hwnd, &buf[0], length+1)
	if err != nil {
		return ""
	}

	return syscall.UTF16ToString(buf)
}

func (w *Window) SetTitle(title string) (err error) {
	text, _ := syscall.UTF16PtrFromString(title)
	return winapi.SetWindowText(w.hwnd, text)
}

func (w *Window) Show() error {
	winapi.UpdateWindow(w.hwnd)
	winapi.ShowWindow(w.hwnd, winapi.SW_SHOWNORMAL)
	return nil
}

func (w *Window) Close() error {
	winapi.CloseWindow(w.hwnd)
	return nil
}

var (
	registerOnce sync.Once
	instance     winapi.HMODULE
	windowClass  winapi.LPWSTR
	windowTitle  winapi.LPWSTR
	windowMap    map[winapi.HWND]*Window
)

func registerWindow() (err error) {
	registerOnce.Do(func() {
		instance, _ = winapi.GetModuleHandle(nil)
		windowClass, _ = syscall.UTF16PtrFromString("GOUI window")
		windowTitle, _ = syscall.UTF16PtrFromString("Window")
		windowMap = make(map[winapi.HWND]*Window)
		arrowCursor, _ := winapi.LoadCursor(0, winapi.IDC_ARROW)
		wndClass := winapi.WNDCLASSEX{
			Size:       winapi.Sizeof_WNDCLASSEX,
			WndProc:    winapi.MakeWindowProc(windowProc),
			Instance:   instance,
			Cursor:     arrowCursor,
			ClassName:  windowClass,
			Background: winapi.HBRUSH(winapi.COLOR_WINDOWFRAME),
		}
		_, err = winapi.RegisterClassEx(&wndClass)
	})
	return
}

func windowProc(hwnd winapi.HWND, message winapi.UINT, wParam winapi.WPARAM, lParam winapi.LPARAM) winapi.LRESULT {
	window, has := windowMap[hwnd]
	if !has {
		if message == winapi.WM_CREATE {
			createStruct := winapi.LPCREATESTRUCT(unsafe.Pointer(uintptr(lParam)))
			window = (*Window)(createStruct.CreateParams)
			windowMap[hwnd] = window
		} else {
			return winapi.DefWindowProc(hwnd, message, wParam, lParam)
		}
	}

	nativeEvent := &Event{
		Hwnd:    hwnd,
		Message: message,
		WParam:  wParam,
		LParam:  lParam,
	}

	switch message {
	case winapi.WM_CLOSE:
		closeEvent := new(events.CloseEvent)
		closeEvent.Window = window
		closeEvent.Native = nativeEvent
		window.onEvent(closeEvent)
		if closeEvent.Accepted() {
			return 0
		}

	case winapi.WM_DESTROY:
		window.onEvent(nativeEvent)
		delete(windowMap, hwnd)

	case winapi.WM_SIZE:
		sizeEvent := new(events.SizeEvent)
		sizeEvent.Window = window
		sizeEvent.Native = nativeEvent
		sizeEvent.Width = int(lParam & 0xFFFF)
		sizeEvent.Height = int((lParam & 0xFFFF0000) >> 16)
		window.onEvent(sizeEvent)

	case winapi.WM_PAINT:
		var ps winapi.PAINTSTRUCT
		winapi.BeginPaint(hwnd, &ps)
		window.onEvent(nativeEvent)
		winapi.EndPaint(hwnd, &ps)
		return 0

	default:
		window.onEvent(nativeEvent)
	}

	if nativeEvent.Accepted() {
		return nativeEvent.Result
	}

	return winapi.DefWindowProc(hwnd, message, wParam, lParam)
}
