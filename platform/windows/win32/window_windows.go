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
	hwnd          winapi.HWND
	parent        common.Window
	onEvent       events.EventHandler
	trackingMouse bool
	lastPointerX  float32
	lastPointerY  float32
	lastButtons   events.PointerButtons
	lastModifiers events.Modifiers
	modifiers     events.Modifiers
	scale         float32      // cached device scale; updated on WM_SIZE
	noActivate    bool         // popups: decline activation/focus on click (WM_MOUSEACTIVATE)
	im            *inputMethod // this window's IME (nil when none); WndProc routes WM_IME_* to it
	cursor        *cursor      // this window's cursor (nil when none); WndProc consults it on WM_SETCURSOR
}

func newWindow(width, height float32, onEvent events.EventHandler) (w *Window, err error) {
	win := &Window{
		onEvent: onEvent,
		scale:   1,
	}

	// No window exists yet to query per-monitor DPI, so estimate with the system
	// DPI; WM_SIZE reports the authoritative client size afterwards. Size is the
	// outer window (frame included) — good enough for an advisory hint.
	scale := float32(winapi.GetDpiForSystem()) / 96
	win.hwnd, err = winapi.CreateWindowEx(0, platform.windowClass, platform.windowTitle, winapi.WS_OVERLAPPEDWINDOW,
		winapi.CW_USEDEFAULT, winapi.CW_USEDEFAULT,
		int(width*scale), int(height*scale),
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
	if w.hwnd != 0 {
		winapi.DestroyWindow(w.hwnd)
	}
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

func (w *Window) Hide() error {
	if w.hwnd == 0 {
		return nil
	}
	winapi.ShowWindow(w.hwnd, winapi.SW_HIDE)
	return nil
}

func (w *Window) RequestClose() error {
	if w.hwnd == 0 {
		return nil
	}
	return winapi.PostMessage(w.hwnd, winapi.WM_CLOSE, 0, 0)
}

func (w *Window) RequestPaint() error {
	if w.hwnd == 0 {
		return nil
	}
	return winapi.InvalidateRect(w.hwnd, nil, winapi.FALSE)
}

func (w *Window) Draw(img image.Image) error {
	bmp, ok := graphics.ToBitmap(img, graphics.PixelFormatBGRA)
	if !ok {
		bmp = graphics.CopyToBitmap(img, graphics.PixelFormatBGRA, nil)
	}
	return w.drawImage(bmp)
}

// scaleFactor returns the window scale, falling back to 1 on error. Used to
// normalize physical pixels (client rect, pointer coords) to logical (DIP).
func (w *Window) scaleFactor() float32 {
	dpi, err := winapi.GetDpiForWindow(w.hwnd)
	if err != nil || dpi == 0 {
		return 1
	}
	return float32(dpi) / 96
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

	switch message {
	case winapi.WM_MOUSEACTIVATE:
		if window.noActivate {
			// Decline activation AND keyboard focus so a click inside a popup does
			// not steal focus from (and thus dismiss, via the owner's WM_KILLFOCUS)
			// its owner window. The click itself is still delivered. WS_EX_NOACTIVATE
			// alone blocks activation but not same-thread focus transfer.
			return winapi.MA_NOACTIVATE
		}
		return winapi.DefWindowProc(hwnd, message, wParam, lParam)

	case winapi.WM_CLOSE:
		window.onEvent(events.CloseEvent{})
		return 0

	case winapi.WM_DESTROY:
		delete(windowMap, hwnd)
		window.hwnd = 0
		return 0

	case winapi.WM_SETFOCUS:
		window.onEvent(events.FocusEvent{Focused: true})
		return 0

	case winapi.WM_KILLFOCUS:
		window.onEvent(events.FocusEvent{Focused: false})
		return 0

	case winapi.WM_SIZE:
		winapi.InvalidateRect(hwnd, nil, winapi.FALSE)
		pw := float32(lParam & 0xFFFF)
		ph := float32((lParam & 0xFFFF0000) >> 16)
		scale := window.scaleFactor()
		window.scale = scale
		window.onEvent(events.SizeEvent{
			Width:       pw / scale,
			Height:      ph / scale,
			PixelWidth:  pw,
			PixelHeight: ph,
		})

	case winapi.WM_PAINT:
		var ps winapi.PAINTSTRUCT
		winapi.BeginPaint(hwnd, &ps)
		window.onEvent(events.PaintEvent{})
		winapi.EndPaint(hwnd, &ps)
		return 0

	case winapi.WM_DPICHANGED:
		// Resize to the suggested rect; the resulting WM_SIZE carries the new
		// scale (logical + physical) via SizeEvent.
		rect := (*winapi.RECT)(unsafe.Pointer(uintptr(lParam)))
		winapi.SetWindowPos(hwnd, 0,
			int(rect.Left), int(rect.Top),
			int(rect.Right-rect.Left), int(rect.Bottom-rect.Top),
			winapi.SWP_NOZORDER|winapi.SWP_NOACTIVATE)

	case winapi.WM_SETCURSOR:
		// Over the client area, apply our cursor and claim the message so
		// DefWindowProc does not reset it to the (NULL) class cursor. Elsewhere
		// (borders, resize edges) let the system pick the cursor.
		if window.cursor != nil && (lParam&0xFFFF) == winapi.HTCLIENT {
			window.cursor.apply()
			return winapi.TRUE
		}

	case winapi.WM_MOUSEMOVE:
		window.handlePointerMove(wParam, lParam)
		return 0

	case winapi.WM_MOUSELEAVE:
		window.handlePointerLeave()
		return 0

	case winapi.WM_LBUTTONDOWN:
		window.handlePointerButton(events.PointerDown, events.PointerButtonLeft, wParam, lParam)
		return 0

	case winapi.WM_LBUTTONUP:
		window.handlePointerButton(events.PointerUp, events.PointerButtonLeft, wParam, lParam)
		return 0

	case winapi.WM_RBUTTONDOWN:
		window.handlePointerButton(events.PointerDown, events.PointerButtonRight, wParam, lParam)
		return 0

	case winapi.WM_RBUTTONUP:
		window.handlePointerButton(events.PointerUp, events.PointerButtonRight, wParam, lParam)
		return 0

	case winapi.WM_MBUTTONDOWN:
		window.handlePointerButton(events.PointerDown, events.PointerButtonMiddle, wParam, lParam)
		return 0

	case winapi.WM_MBUTTONUP:
		window.handlePointerButton(events.PointerUp, events.PointerButtonMiddle, wParam, lParam)
		return 0

	case winapi.WM_XBUTTONDOWN:
		window.handlePointerButton(events.PointerDown, xButton(wParam), wParam, lParam)
		return winapi.LRESULT(winapi.TRUE)

	case winapi.WM_XBUTTONUP:
		window.handlePointerButton(events.PointerUp, xButton(wParam), wParam, lParam)
		return winapi.LRESULT(winapi.TRUE)

	case winapi.WM_MOUSEWHEEL:
		window.handleWheel(false, wParam, lParam)
		return 0

	case winapi.WM_MOUSEHWHEEL:
		window.handleWheel(true, wParam, lParam)
		return 0

	case winapi.WM_KEYDOWN, winapi.WM_SYSKEYDOWN:
		// While the IME is processing a key (composition/candidate navigation) the
		// virtual key is VK_PROCESSKEY; drop it so it does not double up with the
		// committed text delivered via WM_IME_COMPOSITION.
		if wParam != winapi.VK_PROCESSKEY {
			window.handleKey(events.KeyDown, wParam, lParam)
		}
		return 0

	case winapi.WM_KEYUP, winapi.WM_SYSKEYUP:
		window.handleKey(events.KeyUp, wParam, lParam)
		return 0

	case winapi.WM_IME_STARTCOMPOSITION:
		if window.im != nil {
			// Suppress the default composition window; preedit is rendered inline
			// via the input method's Preedit handler.
			return 0
		}

	case winapi.WM_IME_COMPOSITION:
		if window.im != nil {
			window.im.handleComposition(lParam)
			return 0
		}

	case winapi.WM_IME_ENDCOMPOSITION:
		if window.im != nil {
			window.im.endComposition()
			return 0
		}
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
	defer winapi.ReleaseDC(w.hwnd, hdc)

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
