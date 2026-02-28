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

func (w *Window) Draw(img common.Image) error {
	return w.drawImage(common.ToBGRAImage(img))
}

func (w *Window) ScaleFactor() (float64, error) {
	dpi, err := winapi.GetDpiForWindow(w.hwnd)
	if err != nil {
		return 0, err
	}
	return float64(dpi) / 96, nil
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
			Size: winapi.Sizeof_WNDCLASSEX,
			//Style:      winapi.CS_HREDRAW | winapi.CS_VREDRAW,
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

	windowEvent := events.WindowEventBase{
		Window: window,
		Native: nativeEvent,
	}

	switch message {
	case winapi.WM_CLOSE:
		closeEvent := &events.CloseEvent{
			WindowEventBase: windowEvent,
		}
		window.onEvent(closeEvent)
		if closeEvent.Accepted() {
			return 0
		}

	case winapi.WM_DESTROY:
		window.onEvent(nativeEvent)
		delete(windowMap, hwnd)

	case winapi.WM_SIZE:
		winapi.InvalidateRect(hwnd, nil, winapi.FALSE)
		sizeEvent := &events.SizeEvent{
			WindowEventBase: windowEvent,
			Width:           uint(lParam & 0xFFFF),
			Height:          uint((lParam & 0xFFFF0000) >> 16),
		}
		window.onEvent(sizeEvent)

	case winapi.WM_PAINT:
		var ps winapi.PAINTSTRUCT
		winapi.BeginPaint(hwnd, &ps)
		paintEvent := &events.PaintEvent{
			WindowEventBase: windowEvent,
		}
		window.onEvent(paintEvent)
		winapi.EndPaint(hwnd, &ps)
		return 0

	case winapi.WM_DPICHANGED:
		dpi := wParam & 0xFFFF
		scaleEvent := &events.ScaleEvent{
			WindowEventBase: windowEvent,
			ScaleFactor:     float64(dpi) / 96,
		}
		window.onEvent(scaleEvent)
		rect := (*winapi.RECT)(unsafe.Pointer(uintptr(lParam)))
		winapi.SetWindowPos(hwnd, 0,
			int(rect.Left), int(rect.Top),
			int(rect.Right-rect.Left), int(rect.Bottom-rect.Top),
			winapi.SWP_NOZORDER|winapi.SWP_NOACTIVATE)

	default:
		window.onEvent(nativeEvent)
	}

	if nativeEvent.Accepted() {
		return nativeEvent.Result
	}

	return winapi.DefWindowProc(hwnd, message, wParam, lParam)
}

func (w *Window) drawImage(img *common.BGRAImage) error {
	var rect winapi.RECT
	winapi.GetClientRect(w.hwnd, &rect)
	if rect.Right == 0 || rect.Bottom == 0 {
		return nil
	}

	hdc, err := winapi.GetDC(w.hwnd)
	if err != nil {
		return err
	}
	defer winapi.ReleaseDC(hdc)

	mdc := winapi.CreateCompatibleDC(hdc)
	mBitmap := winapi.CreateCompatibleBitmap(hdc, rect.Right, rect.Bottom)
	mOldObj := winapi.SelectObject(mdc, mBitmap)

	{
		bounds := img.Bounds()

		tdc := winapi.CreateCompatibleDC(hdc)
		width, height := winapi.INT(bounds.Dx()), winapi.INT(bounds.Dy())
		tBitmap := winapi.CreateCompatibleBitmap(hdc, width, height)
		tOldObj := winapi.SelectObject(tdc, tBitmap)

		info := winapi.BITMAPINFO{
			Header: winapi.BITMAPINFOHEADER{
				Size:     winapi.Sizeof_BITMAPINFOHEADER,
				Width:    width,
				Height:   -height,
				Planes:   1,
				BitCount: 32, //RGBA
			},
		}
		winapi.SetDIBits(tdc, tBitmap, 0, winapi.UINT(height), winapi.LPVOID(&img.Pix[0]), &info, 0 /*DIB_RGB_COLORS*/)

		winapi.BitBlt(mdc, winapi.INT(bounds.Min.X), winapi.INT(bounds.Min.Y), width, height, tdc, 0, 0, 0x00CC0020 /*SRCCOPY*/)
		winapi.SelectObject(tdc, tOldObj)
		winapi.DeleteObject(tBitmap)

		winapi.DeleteDC(tdc)
	}

	winapi.BitBlt(hdc, 0, 0, rect.Right, rect.Bottom, mdc, 0, 0, 0x00CC0020 /*SRCCOPY*/)
	winapi.SelectObject(mdc, mOldObj)
	winapi.DeleteObject(mBitmap)
	winapi.DeleteDC(mdc)

	return nil
}
