package win32

import (
	"fmt"
	"image"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"

	"github.com/golang-gui/goui/platform/windows/sdk/winapi"

	"github.com/goexlib/cgo"
)

type Window struct {
	hwnd    winapi.HWND
	parent  common.Window
	onEvent events.EventHandler
}

func newWindow(onEvent events.EventHandler) (w *Window, err error) {
	win := &Window{
		onEvent: onEvent,
	}

	win.hwnd, err = winapi.CreateWindowEx(0, platform.windowClass, platform.windowTitle, winapi.WS_OVERLAPPEDWINDOW,
		winapi.CW_USEDEFAULT, winapi.CW_USEDEFAULT,
		winapi.CW_USEDEFAULT, winapi.CW_USEDEFAULT,
		0, 0, platform.instance,
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

func (w *Window) Draw(img image.Image) error {
	bmp, ok := graphics.ToBitmap(img, graphics.PixelFormatBGRA)
	if !ok {
		bmp = graphics.CopyToBitmap(img, graphics.PixelFormatBGRA, nil)
	}
	return w.drawImage(bmp)
}

func (w *Window) ScaleFactor() (float64, error) {
	dpi, err := winapi.GetDpiForWindow(w.hwnd)
	if err != nil {
		return 0, err
	}
	return float64(dpi) / 96, nil
}

var windowMap = map[winapi.HWND]*Window{}

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

func (w *Window) drawImage(img graphics.Bitmap) error {
	if img.Width == 0 || img.Height == 0 {
		return nil
	}

	var rect winapi.RECT
	_ = winapi.GetClientRect(w.hwnd, &rect)
	if rect.Right == 0 || rect.Bottom == 0 {
		return nil
	}

	hdc, err := winapi.GetDC(w.hwnd)
	if err != nil {
		return err
	}
	defer winapi.ReleaseDC(hdc)

	width := winapi.INT(img.Width)
	height := winapi.INT(img.Height)

	bmi := winapi.BITMAPINFO{
		Header: winapi.BITMAPINFOHEADER{
			Size:     winapi.Sizeof_BITMAPINFOHEADER,
			Width:    width,
			Height:   -height,
			Planes:   1,
			BitCount: 32, //RGBA
		},
	}

	ret := winapi.StretchDIBits(hdc, 0, 0, width, height, 0, 0, width, height, cgo.CSlice(img.Pixels), &bmi, winapi.DIB_RGB_COLORS, winapi.SRCCOPY)
	if ret == 0 {
		byteSize := img.Stride * img.Height
		bits := winapi.LocalAlloc(0, uint(byteSize))
		if bits == nil {
			return fmt.Errorf("alloc local image memeory failed")
		}
		defer winapi.LocalFree(bits)
		copy(cgo.GoSliceNTemp[byte](bits, byteSize), img.Pixels)
		ret = winapi.StretchDIBits(hdc, 0, 0, width, height, 0, 0, width, height, bits, &bmi, winapi.DIB_RGB_COLORS, winapi.SRCCOPY)
		if ret == 0 {
			return fmt.Errorf("draw image failed")
		}
	}
	return nil
}
