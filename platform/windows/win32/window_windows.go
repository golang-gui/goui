package win32

import (
	"fmt"
	"image"
	"math"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/direct2d"
	"github.com/golang-gui/goui/platform/internal/workarea"
	"github.com/golang-gui/goui/platform/windows/sdk/dcomp"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"

	"github.com/goexlib/cgo"
)

type Window struct {
	transparent       bool
	uploadPainter     graphics.Painter
	uploadImage       graphics.Image
	style             winapi.DWORD
	hwnd              winapi.HWND
	parent            common.Window
	onEvent           events.EventHandler
	trackingMouse     bool
	trackingNonClient bool
	lastPointerX      float32
	lastPointerY      float32
	lastButtons       events.PointerButtons
	lastModifiers     events.Modifiers
	modifiers         events.Modifiers
	scale             float32      // cached device scale; updated on WM_SIZE
	inSizeMove        bool         // inside the modal move/resize loop
	resizedInSizeMove bool         // whether that loop delivered an authoritative WM_SIZE
	noActivate        bool         // popups: decline activation/focus on click (WM_MOUSEACTIVATE)
	im                *inputMethod // this window's IME (nil when none); WndProc routes WM_IME_* to it
	cursor            *cursor      // this window's cursor (nil when none); WndProc consults it on WM_SETCURSOR
	minW              float32      // minimum size hint in DIP; 0 = unbounded
	minH              float32      // Native: client size; None/Integrated: outer size
	hitTest           func(geometry.Point) common.WindowHit
	hitTesting        bool
	state             common.WindowState // last native notification, not request state
	integrated        bool               // custom non-client calculation is installed, not a creation preference
	frameExtended     bool               // last DWM frame-extension call succeeded
}

var _ common.DesktopWindow = (*Window)(nil)

func newWindow(size geometry.Size, onEvent events.EventHandler, options common.WindowOptions) (w *Window, err error) {
	if err := common.ValidateWindowSize(size); err != nil {
		return nil, err
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if options.Chrome == common.WindowChromeIntegrated {
		var composed winapi.BOOL
		if err := winapi.DwmIsCompositionEnabled(&composed); err != nil {
			return nil, fmt.Errorf("query DWM composition: %w", err)
		}
		if composed == winapi.FALSE {
			return nil, fmt.Errorf("integrated chrome requires DWM composition: %w", common.ErrUnsupported)
		}
	}
	style := windowStyle(options)
	if options.Transparent {
		if err := checkTransparency(); err != nil {
			return nil, err
		}
	}
	win := &Window{
		onEvent:     onEvent,
		scale:       1,
		style:       winapi.WS_OVERLAPPEDWINDOW,
		state:       common.WindowStateUnknown,
		transparent: options.Transparent,
	}

	// No window exists yet to query per-monitor DPI, so estimate with the system
	// DPI; WM_SIZE reports the authoritative client size afterwards. Only Native
	// chrome adds standard frame insets. Custom chrome takes the supplied extent
	// directly, without a caption adjustment or a later corrective resize.
	dpi := winapi.GetDpiForSystem()
	scale := float32(dpi) / 96
	if preferred := common.GetPreferScale(); preferred > 0 {
		scale = preferred
	}
	rect := windowSizeRect(size, scale, dpi, style)
	// CW_USEDEFAULT only places overlapped windows. Obtain the system position
	// before switching to WS_POPUP, as in ModernWindow; no visible caption is
	// shown because all frame setup completes before Show.
	win.hwnd, err = winapi.CreateWindowEx(surfaceExStyle(options.Transparent), platform.windowClass, platform.windowTitle, win.style,
		winapi.CW_USEDEFAULT, winapi.CW_USEDEFAULT,
		int(rect.Right-rect.Left), int(rect.Bottom-rect.Top),
		0, 0, platform.instance,
		unsafe.Pointer(win))

	if err != nil {
		return nil, err
	}
	if style != win.style {
		if _, err := winapi.SetWindowLong(win.hwnd, winapi.GWL_STYLE, winapi.LONG(style)); err != nil {
			win.Destroy()
			return nil, fmt.Errorf("set window style: %w", err)
		}
		win.style = style
		if options.Chrome == common.WindowChromeIntegrated {
			if err := win.extendFrame(); err != nil {
				win.Destroy()
				return nil, fmt.Errorf("extend integrated frame: %w", err)
			}
			win.integrated = true
		}
		if err := winapi.SetWindowPos(win.hwnd, 0, 0, 0, 0, 0,
			winapi.SWP_FRAMECHANGED|winapi.SWP_NOMOVE|winapi.SWP_NOSIZE|winapi.SWP_NOZORDER|winapi.SWP_NOACTIVATE); err != nil {
			win.Destroy()
			return nil, fmt.Errorf("install window frame: %w", err)
		}
	}

	runtime.KeepAlive(win)
	return win, nil
}

func (w *Window) NativeHandle() uintptr {
	return uintptr(w.hwnd)
}

func (w *Window) Destroy() {
	w.releaseUpload()
	w.hitTest = nil
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
	winapi.ShowWindow(w.hwnd, winapi.SW_SHOW)
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

func (w *Window) SetMinSize(width, height float32) {
	if w.hwnd == 0 {
		return
	}
	// Store logical (DIP) values; physical pixels are derived from the current
	// scale at WM_GETMINMAXINFO time so a later DPI change stays correct.
	w.minW, w.minH = width, height
}

func (w *Window) Draw(img image.Image) error {
	if w.transparent {
		return w.drawTransparent(img)
	}
	bmp, ok := graphics.ToBitmap(img, graphics.PixelFormatBGRA)
	if !ok {
		bmp = graphics.CopyToBitmap(img, graphics.PixelFormatBGRA, nil)
	}
	return w.drawImage(bmp)
}

// scaleFactor returns the window scale, falling back to 1 on error. Used to
// normalize physical pixels (client rect, pointer coords) to logical (DIP).
func (w *Window) scaleFactor() float32 {
	return hwndScale(w.hwnd)
}

// hwndScale is the shared DIP-to-pixel scale for windows and popups. The
// application override must take precedence in both native geometry requests
// and the inverse conversion used by SizeEvent and pointer events.
func hwndScale(hwnd winapi.HWND) float32 {
	if scale := common.GetPreferScale(); scale > 0 {
		return scale
	}
	dpi, err := winapi.GetDpiForWindow(hwnd)
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
			// WM_SIZE can arrive before CreateWindowEx returns. Its initial DPI
			// query must use the new HWND, not the still-zero constructor result.
			window.hwnd = hwnd
			windowMap[hwnd] = window
		} else {
			return winapi.DefWindowProc(hwnd, message, wParam, lParam)
		}
	}

	switch message {
	case winapi.WM_NCCALCSIZE:
		if window.integrated && wParam != 0 {
			return window.calculateClient(wParam, lParam)
		}
	case winapi.WM_DWMCOMPOSITIONCHANGED:
		if window.integrated {
			// DWM requires the extension to be re-applied after composition
			// changes. Failure makes Chrome unknown, not a fabricated success.
			_ = window.extendFrame()
		}
	case winapi.WM_NCACTIVATE:
		if window.integrated {
			// Update native activation state without letting User32 paint its
			// standard frame over our extended client area. In particular, the
			// default repaint can leave a white strip on Windows 10.
			return winapi.DefWindowProc(hwnd, message, wParam, -1)
		}
	case winapi.WM_ACTIVATE:
		if window.integrated {
			// Finish native activation/focus processing before presenting the
			// custom frame, so a subsequent default paint cannot overwrite it.
			result := winapi.DefWindowProc(hwnd, message, wParam, lParam)
			if window.hwnd != 0 {
				window.repaintFrame()
			}
			return result
		}
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
		window.releaseUpload()
		window.hitTest = nil
		delete(windowMap, hwnd)
		window.hwnd = 0
		return 0

	case winapi.WM_WINDOWPOSCHANGED:
		// DefWindowProc applies visibility/size consequences first. Query the
		// resulting native state, never the requested ShowWindow command.
		result := winapi.DefWindowProc(hwnd, message, wParam, lParam)
		if window.hwnd != 0 {
			window.notifyState()
		}
		return result

	case winapi.WM_NCHITTEST:
		return window.handleHitTest(lParam)

	case winapi.WM_NCMOUSEMOVE, winapi.WM_NCLBUTTONDOWN, winapi.WM_NCLBUTTONUP, winapi.WM_NCLBUTTONDBLCLK,
		winapi.WM_NCRBUTTONDOWN, winapi.WM_NCRBUTTONUP, winapi.WM_NCRBUTTONDBLCLK,
		winapi.WM_NCMBUTTONDOWN, winapi.WM_NCMBUTTONUP, winapi.WM_NCMBUTTONDBLCLK,
		winapi.WM_NCXBUTTONDOWN, winapi.WM_NCXBUTTONUP, winapi.WM_NCXBUTTONDBLCLK:
		if window.integrated && message == winapi.WM_NCLBUTTONDOWN {
			window.repaintFrame()
			if window.hwnd == 0 {
				return 0
			}
		}
		if window.handleCaptionPointer(message, wParam, lParam) || window.hwnd == 0 {
			if message == winapi.WM_NCXBUTTONDOWN || message == winapi.WM_NCXBUTTONUP || message == winapi.WM_NCXBUTTONDBLCLK {
				return winapi.TRUE
			}
			return 0
		}

	case winapi.WM_NCMOUSELEAVE:
		window.handleTrackedPointerLeave(true)
		if window.hwnd == 0 {
			return 0
		}

	case winapi.WM_SETFOCUS:
		window.onEvent(events.FocusEvent{Focused: true})
		return 0

	case winapi.WM_KILLFOCUS:
		window.onEvent(events.FocusEvent{Focused: false})
		return 0

	case winapi.WM_ENTERSIZEMOVE:
		window.inSizeMove = true
		window.resizedInSizeMove = false
		return 0

	case winapi.WM_EXITSIZEMOVE:
		window.inSizeMove = false
		if window.resizedInSizeMove {
			// Commit one final frame even if the last WM_SIZE was coalesced. This
			// also replaces any old swap-chain buffer still shown by DWM. A pure
			// move keeps the existing composited client contents and needs no paint.
			winapi.InvalidateRect(hwnd, nil, winapi.FALSE)
			winapi.UpdateWindow(hwnd)
		}
		window.resizedInSizeMove = false
		return 0

	case winapi.WM_SIZE:
		window.notifyState()
		if window.hwnd == 0 { // an event handler may destroy the window
			return 0
		}
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
		if window.hwnd == 0 {
			return 0
		}
		// Dispatch the authoritative size before painting so Painter.Begin can
		// resize the swap-chain buffers and render with matching dimensions.
		winapi.InvalidateRect(hwnd, nil, winapi.FALSE)
		if window.inSizeMove {
			window.resizedInSizeMove = true
			if pw > 0 && ph > 0 {
				// WM_PAINT is otherwise a low-priority queued message and may be
				// starved by a stream of WM_SIZE messages. Paint synchronously while
				// the user drags an edge to minimize the lifetime of the old buffer.
				winapi.UpdateWindow(hwnd)
			}
		}
		return 0

	case winapi.WM_ERASEBKGND:
		// Every painter presents a complete client-area frame. Letting GDI erase
		// the window first exposes a white intermediate surface during resize.
		return winapi.LRESULT(winapi.TRUE)

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

	case winapi.WM_GETMINMAXINFO:
		if window.integrated || window.minW > 0 || window.minH > 0 {
			mmi := (*winapi.MINMAXINFO)(unsafe.Pointer(uintptr(lParam)))
			if window.integrated {
				if monitor, err := window.monitorInfo(); err == nil {
					setMaximizedBounds(mmi, monitor)
				}
			}
			if window.minW > 0 || window.minH > 0 {
				dpi, _ := winapi.GetDpiForWindow(hwnd)
				if dpi == 0 {
					dpi = winapi.GetDpiForSystem()
				}
				rect := windowSizeRect(geometry.Size{Width: window.minW, Height: window.minH}, window.scaleFactor(), dpi, window.style)
				if window.minW > 0 {
					mmi.MinTrackSize.X = rect.Right - rect.Left
				}
				if window.minH > 0 {
					mmi.MinTrackSize.Y = rect.Bottom - rect.Top
				}
			}
			return 0
		}

	case winapi.WM_MOUSEMOVE:
		window.handlePointerMove(wParam, lParam)
		return 0

	case winapi.WM_MOUSELEAVE:
		window.handleTrackedPointerLeave(false)
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
		if window.integrated && window.hwnd != 0 && message == winapi.WM_SYSKEYDOWN {
			// Integrated removes the visible caption, not User32's system
			// commands (for example Alt+F4). Notify input before native handling.
			return winapi.DefWindowProc(hwnd, message, wParam, lParam)
		}
		return 0

	case winapi.WM_KEYUP, winapi.WM_SYSKEYUP:
		window.handleKey(events.KeyUp, wParam, lParam)
		if window.integrated && window.hwnd != 0 && message == winapi.WM_SYSKEYUP {
			return winapi.DefWindowProc(hwnd, message, wParam, lParam)
		}
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

func windowStyle(options common.WindowOptions) winapi.DWORD {
	switch options.Chrome {
	case common.WindowChromeIntegrated:
		return winapi.WS_POPUP | winapi.WS_THICKFRAME | winapi.WS_MAXIMIZEBOX
	case common.WindowChromeNone:
		return winapi.WS_POPUP // managed top-level, not a no-activate tool popup
	default:
		return winapi.WS_OVERLAPPEDWINDOW
	}
}

func surfaceExStyle(transparent bool) winapi.DWORD {
	// Windows 10 rejects changing NOREDIRECTIONBITMAP after HWND creation.
	// D2D and software upload present through DirectComposition; WGL instead
	// needs a redirected window for its DWM blur-behind presentation.
	if transparent && common.GetPreferPainter() != "opengl" {
		return winapi.WS_EX_NOREDIRECTIONBITMAP
	}
	return 0
}

func windowSizeRect(size geometry.Size, scale float32, dpi winapi.UINT, style winapi.DWORD) winapi.RECT {
	rect := winapi.RECT{Right: winapi.LONG(size.Width * scale), Bottom: winapi.LONG(size.Height * scale)}
	if style&winapi.WS_CAPTION != 0 {
		// Only the unchanged Native mode uses the standard caption calculation.
		winapi.AdjustWindowRectExForDpi(&rect, style, 0, 0, dpi)
	}
	return rect
}

// Integrated follows ModernWindow's WS_POPUP + DWM extension mechanism.
// A maximized client is the work area, not the normal frame plus a top inset.
func (w *Window) calculateClient(wParam winapi.WPARAM, lParam winapi.LPARAM) winapi.LRESULT {
	client := &(*winapi.NCCALCSIZE_PARAMS)(unsafe.Pointer(uintptr(lParam))).Rects[0]
	if winapi.IsZoomed(w.hwnd) != winapi.FALSE {
		if monitor, err := w.monitorInfo(); err == nil {
			*client = monitor.Work // screen coordinates, already in native pixels
		}
		// As in ModernWindow, keep the proposed rectangle if the monitor cannot
		// be observed. Do not invent work-area bounds or apply normal borders.
		return 0
	}
	outer := *client
	result := winapi.DefWindowProc(w.hwnd, winapi.WM_NCCALCSIZE, wParam, lParam)
	*client = integratedClientRect(outer, *client)
	return result
}

// A physical non-client pixel keeps the native top border; it is not DIP
// padding and must not grow with the application's logical scale override.
const integratedTopBorder winapi.LONG = 1

func integratedClientRect(outer, native winapi.RECT) winapi.RECT {
	native.Top = min(outer.Top+integratedTopBorder, native.Bottom)
	return native
}

func (w *Window) extendFrame() error {
	// This is separate from the one-pixel non-client top border. DWM also
	// extends one pixel into the client, retaining native border highlighting.
	margins := winapi.MARGINS{CYTopHeight: 1}
	err := winapi.DwmExtendFrameIntoClientArea(w.hwnd, &margins)
	w.frameExtended = err == nil
	return err
}

func (w *Window) repaintFrame() {
	winapi.InvalidateRect(w.hwnd, nil, winapi.FALSE)
	winapi.UpdateWindow(w.hwnd)
}

func (w *Window) monitorInfo() (winapi.MONITORINFO, error) {
	info := winapi.MONITORINFO{Size: winapi.DWORD(unsafe.Sizeof(winapi.MONITORINFO{}))}
	monitor := winapi.MonitorFromWindow(w.hwnd, winapi.MONITOR_DEFAULTTONEAREST)
	if monitor == 0 {
		return info, common.ErrUnavailable
	}
	if err := winapi.GetMonitorInfo(monitor, &info); err != nil {
		return info, err
	}
	if info.Work.Right <= info.Work.Left || info.Work.Bottom <= info.Work.Top {
		return info, common.ErrUnavailable
	}
	return info, nil
}

func setMaximizedBounds(mmi *winapi.MINMAXINFO, monitor winapi.MONITORINFO) {
	// MINMAXINFO position is monitor-relative; NCCALCSIZE uses screen space.
	// Preserve work-area offsets when a taskbar is at the left or top.
	mmi.MaxPosition = winapi.POINT{X: monitor.Work.Left - monitor.Monitor.Left, Y: monitor.Work.Top - monitor.Monitor.Top}
	mmi.MaxSize = winapi.POINT{X: monitor.Work.Right - monitor.Work.Left, Y: monitor.Work.Bottom - monitor.Work.Top}
}

func (w *Window) Chrome() common.WindowChrome {
	if w.hwnd == 0 {
		return common.WindowChromeUnknown
	}
	style, err := winapi.GetWindowLong(w.hwnd, winapi.GWL_STYLE)
	if err != nil {
		return common.WindowChromeUnknown
	}
	if w.integrated {
		const frameStyle = winapi.WS_POPUP | winapi.WS_THICKFRAME
		if !w.frameExtended || style&winapi.WS_CAPTION != 0 || winapi.DWORD(style)&frameStyle != frameStyle {
			return common.WindowChromeUnknown
		}
		var composed winapi.BOOL
		if err := winapi.DwmIsCompositionEnabled(&composed); err != nil || composed == winapi.FALSE {
			return common.WindowChromeUnknown
		}
		return common.WindowChromeIntegrated
	}
	if style&winapi.WS_CAPTION == 0 {
		return common.WindowChromeNone
	}
	return common.WindowChromeNative
}

func (w *Window) SetControlsPosition(*geometry.Point) error {
	if w.hwnd == 0 {
		return common.ErrUnavailable
	}
	return common.ErrUnsupported
}

func (w *Window) ControlsRect() (geometry.Rectangle, error) {
	if w.hwnd == 0 {
		return geometry.Rectangle{}, common.ErrUnavailable
	}
	if w.integrated {
		// The installed client calculation removes the native caption group.
		// DWM's cached caption bounds are not visible controls in this mode.
		return geometry.Rectangle{}, nil
	}
	style, err := winapi.GetWindowLong(w.hwnd, winapi.GWL_STYLE)
	if err != nil {
		return geometry.Rectangle{}, fmt.Errorf("caption style: %v: %w", err, common.ErrUnavailable)
	}
	if style&winapi.WS_CAPTION == 0 || style&winapi.WS_SYSMENU == 0 {
		return geometry.Rectangle{}, nil
	}
	if winapi.IsWindowVisible(w.hwnd) == winapi.FALSE || winapi.IsIconic(w.hwnd) != winapi.FALSE {
		return geometry.Rectangle{}, common.ErrUnavailable
	}
	// DWM caption bounds are physical coordinates relative to the outer window.
	// They are not defined for hidden or minimized windows.
	var buttons winapi.RECT
	if err := winapi.DwmGetWindowAttribute(w.hwnd, winapi.DWMWA_CAPTION_BUTTON_BOUNDS,
		unsafe.Pointer(&buttons), winapi.DWORD(unsafe.Sizeof(buttons))); err != nil {
		return geometry.Rectangle{}, fmt.Errorf("caption bounds: %v: %w", err, common.ErrUnavailable)
	}
	var frame winapi.RECT
	if err := winapi.GetWindowRect(w.hwnd, &frame); err != nil {
		return geometry.Rectangle{}, common.ErrUnavailable
	}
	var origin winapi.POINT
	if winapi.ClientToScreen(w.hwnd, &origin) == 0 {
		return geometry.Rectangle{}, common.ErrUnavailable
	}
	return captionRect(buttons, frame, origin, w.scaleFactor()), nil
}

func captionRect(buttons, frame winapi.RECT, clientOrigin winapi.POINT, scale float32) geometry.Rectangle {
	if buttons.Right <= buttons.Left || buttons.Bottom <= buttons.Top {
		return geometry.Rectangle{}
	}
	return geometry.Rect(
		float32(frame.Left+buttons.Left-clientOrigin.X)/scale,
		float32(frame.Top+buttons.Top-clientOrigin.Y)/scale,
		float32(buttons.Right-buttons.Left)/scale,
		float32(buttons.Bottom-buttons.Top)/scale)
}

func (w *Window) SetHitTest(f func(geometry.Point) common.WindowHit) error {
	if w.hwnd == 0 {
		return common.ErrUnavailable
	}
	w.hitTest = f
	return nil
}

// Windows starts native interaction through WM_NCHITTEST, not client input.
func (w *Window) BeginMove() error {
	if w.hwnd == 0 {
		return common.ErrUnavailable
	}
	return common.ErrUnsupported
}

func (w *Window) BeginResize(common.WindowEdge) error {
	if w.hwnd == 0 {
		return common.ErrUnavailable
	}
	return common.ErrUnsupported
}

func (w *Window) queryHitTest(p geometry.Point) common.WindowHit {
	if w.hitTest == nil || w.hitTesting || w.hwnd == 0 {
		return common.WindowHitDefault
	}
	w.hitTesting = true
	defer func() { w.hitTesting = false }()
	hit := w.hitTest(p)
	if hit > common.WindowHitBottomRight {
		return common.WindowHitDefault
	}
	return hit
}

func nativeWindowHit(hit common.WindowHit, fallback winapi.LRESULT) winapi.LRESULT {
	switch hit {
	case common.WindowHitClient:
		return winapi.HTCLIENT
	case common.WindowHitSysMenu:
		return winapi.HTSYSMENU
	case common.WindowHitCaption:
		// Drag backgrounds may extend to the client edge. Preserve the real
		// resize band rather than making GUI padding stand in for DPI metrics.
		switch fallback {
		case winapi.HTLEFT, winapi.HTRIGHT, winapi.HTTOP, winapi.HTBOTTOM,
			winapi.HTTOPLEFT, winapi.HTTOPRIGHT, winapi.HTBOTTOMLEFT, winapi.HTBOTTOMRIGHT:
			return fallback
		}
		return winapi.HTCAPTION
	case common.WindowHitMinimize:
		return winapi.HTMINBUTTON
	case common.WindowHitMaximize:
		return winapi.HTMAXBUTTON
	case common.WindowHitClose:
		return winapi.HTCLOSE
	case common.WindowHitTop:
		return winapi.HTTOP
	case common.WindowHitBottom:
		return winapi.HTBOTTOM
	case common.WindowHitLeft:
		return winapi.HTLEFT
	case common.WindowHitRight:
		return winapi.HTRIGHT
	case common.WindowHitTopLeft:
		return winapi.HTTOPLEFT
	case common.WindowHitTopRight:
		return winapi.HTTOPRIGHT
	case common.WindowHitBottomLeft:
		return winapi.HTBOTTOMLEFT
	case common.WindowHitBottomRight:
		return winapi.HTBOTTOMRIGHT
	default:
		return fallback
	}
}

func captionButtonHit(hit winapi.LRESULT) bool {
	return hit == winapi.HTMINBUTTON || hit == winapi.HTMAXBUTTON || hit == winapi.HTCLOSE
}

func (w *Window) defaultHitTest(lParam winapi.LPARAM) winapi.LRESULT {
	if w.integrated {
		return w.integratedHitTest(lParam)
	}
	var hit winapi.LRESULT
	if handled, err := winapi.DwmDefWindowProc(w.hwnd, winapi.WM_NCHITTEST, 0, lParam, &hit); err == nil && handled != winapi.FALSE {
		return hit
	}
	return winapi.DefWindowProc(w.hwnd, winapi.WM_NCHITTEST, 0, lParam)
}

func (w *Window) integratedHitTest(lParam winapi.LPARAM) winapi.LRESULT {
	hit := winapi.DefWindowProc(w.hwnd, winapi.WM_NCHITTEST, 0, lParam)
	// No system caption controls exist in this mode. In particular, do not
	// reserve DwmDefWindowProc's old button positions over custom content.
	if captionButtonHit(hit) || hit == winapi.HTSYSMENU || hit == winapi.HTCAPTION {
		hit = winapi.HTCLIENT
	}
	if winapi.IsZoomed(w.hwnd) != winapi.FALSE {
		return hit
	}
	var client winapi.RECT
	if err := winapi.GetClientRect(w.hwnd, &client); err != nil {
		return hit
	}
	dpi, err := winapi.GetDpiForWindow(w.hwnd)
	if err != nil {
		return hit
	}
	width, err := winapi.GetSystemMetricsForDpi(winapi.SM_CXSIZEFRAME, dpi)
	if err != nil {
		return hit
	}
	height, err := winapi.GetSystemMetricsForDpi(winapi.SM_CYSIZEFRAME, dpi)
	if err != nil {
		return hit
	}
	padding, err := winapi.GetSystemMetricsForDpi(winapi.SM_CXPADDEDBORDER, dpi)
	if err != nil {
		return hit
	}
	point := winapi.POINT{X: winapi.LONG(int16(lParam)), Y: winapi.LONG(int16(lParam >> 16))}
	if winapi.ScreenToClient(w.hwnd, &point) == winapi.FALSE {
		return hit
	}
	return integratedTopHit(point, client, winapi.LONG(width+padding), winapi.LONG(height+padding), hit)
}

func integratedTopHit(point winapi.POINT, client winapi.RECT, borderWidth, borderHeight winapi.LONG, native winapi.LRESULT) winapi.LRESULT {
	// DefWindowProc can classify too much of the side as a top corner after
	// the caption is removed. Keep ModernWindow's corner correction as well
	// as its client-side top resize band, using native DPI metrics, not 8 DIP.
	if native == winapi.HTTOPLEFT || native == winapi.HTTOPRIGHT {
		if point.Y > client.Top+borderHeight {
			if native == winapi.HTTOPLEFT {
				return winapi.HTLEFT
			}
			return winapi.HTRIGHT
		}
		return native
	}
	if point.Y <= client.Top+borderHeight {
		if point.X <= client.Left+borderWidth {
			return winapi.HTTOPLEFT
		}
		if point.X >= client.Right-borderWidth {
			return winapi.HTTOPRIGHT
		}
		return winapi.HTTOP
	}
	return native
}

func (w *Window) handleHitTest(lParam winapi.LPARAM) winapi.LRESULT {
	native := w.defaultHitTest(lParam)
	// These controls belong to the system, not the application's Widget tree.
	if captionButtonHit(native) || native == winapi.HTSYSMENU {
		return native
	}
	hit := w.queryHitTest(w.logicalPoint(screenPointToClient(w.hwnd, lParam)))
	return nativeWindowHit(hit, native)
}

// Custom caption buttons retain their native hover role (notably HTMAXBUTTON
// for Snap), but clicks enter the same GUI pointer path as other Widgets. The
// WM must not also execute a caption command for this click.
func (w *Window) handleCaptionPointer(message winapi.UINT, wParam winapi.WPARAM, lParam winapi.LPARAM) bool {
	custom := w.hitTest != nil && captionButtonHit(winapi.LRESULT(lowWord(uintptr(wParam))))
	if custom {
		native := w.defaultHitTest(lParam)
		custom = !captionButtonHit(native) && native != winapi.HTSYSMENU
	}
	if !custom {
		// Crossing from a custom button to another non-client role does not
		// produce WM_NCMOUSELEAVE: the pointer is still inside non-client space.
		if message == winapi.WM_NCMOUSEMOVE && w.trackingNonClient {
			w.cancelMouseLeave(true)
			w.handlePointerLeave()
		}
		return false
	}
	position := w.logicalPoint(screenPointToClient(w.hwnd, lParam))
	buttons, modifiers := nativePointerState()
	if message == winapi.WM_NCMOUSEMOVE {
		w.cancelMouseLeave(false)
		if !w.trackingNonClient {
			w.trackMouseLeave(true)
			w.emitPointer(events.PointerEnter, events.PointerButtonNone, position, buttons, modifiers)
		}
		if w.hwnd != 0 {
			w.emitPointer(events.PointerMove, events.PointerButtonNone, position, buttons, modifiers)
		}
		return false // allow the shell to handle native caption hover
	}
	var button events.PointerButton
	var flag events.PointerButtons
	typ := events.PointerDown
	switch message {
	case winapi.WM_NCLBUTTONUP:
		typ = events.PointerUp
		fallthrough
	case winapi.WM_NCLBUTTONDOWN, winapi.WM_NCLBUTTONDBLCLK:
		button, flag = events.PointerButtonLeft, events.PointerButtonLeftDown
	case winapi.WM_NCRBUTTONUP:
		typ = events.PointerUp
		fallthrough
	case winapi.WM_NCRBUTTONDOWN, winapi.WM_NCRBUTTONDBLCLK:
		button, flag = events.PointerButtonRight, events.PointerButtonRightDown
	case winapi.WM_NCMBUTTONUP:
		typ = events.PointerUp
		fallthrough
	case winapi.WM_NCMBUTTONDOWN, winapi.WM_NCMBUTTONDBLCLK:
		button, flag = events.PointerButtonMiddle, events.PointerButtonMiddleDown
	case winapi.WM_NCXBUTTONUP:
		typ = events.PointerUp
		fallthrough
	case winapi.WM_NCXBUTTONDOWN, winapi.WM_NCXBUTTONDBLCLK:
		button = xButton(wParam)
		if button == events.PointerButtonBack {
			flag = events.PointerButtonBackDown
		}
		if button == events.PointerButtonForward {
			flag = events.PointerButtonForwardDown
		}
		if flag == 0 {
			return false
		}
	default:
		return false
	}
	if typ == events.PointerDown {
		buttons |= flag
		w.cancelMouseLeave(true)
		winapi.SetCapture(w.hwnd) // subsequent move/up arrives in client coordinates
	} else {
		buttons &^= flag
		if buttons == 0 {
			winapi.ReleaseCapture()
		}
	}
	w.emitPointer(typ, button, position, buttons, modifiers)
	return true
}

func nativePointerState() (events.PointerButtons, events.Modifiers) {
	var flags winapi.WPARAM
	for _, key := range [...]struct {
		key  int
		mask winapi.WPARAM
	}{
		{winapi.VK_LBUTTON, winapi.MK_LBUTTON}, {winapi.VK_RBUTTON, winapi.MK_RBUTTON},
		{winapi.VK_MBUTTON, winapi.MK_MBUTTON}, {winapi.VK_XBUTTON1, winapi.MK_XBUTTON1},
		{winapi.VK_XBUTTON2, winapi.MK_XBUTTON2}, {winapi.VK_SHIFT, winapi.MK_SHIFT},
		{winapi.VK_CONTROL, winapi.MK_CONTROL},
	} {
		if winapi.GetKeyState(key.key) < 0 {
			flags |= key.mask
		}
	}
	mods := pointerModifiers(flags)
	if winapi.GetKeyState(winapi.VK_MENU) < 0 {
		mods |= events.ModifierAlt
	}
	if winapi.GetKeyState(winapi.VK_LWIN) < 0 || winapi.GetKeyState(winapi.VK_RWIN) < 0 {
		mods |= events.ModifierSuper
	}
	return pointerButtons(flags), mods
}

func (w *Window) State() common.WindowState {
	if w.hwnd == 0 {
		return common.WindowStateUnknown
	}
	if winapi.IsWindowVisible(w.hwnd) == winapi.FALSE {
		return common.WindowStateHidden
	}
	if winapi.IsIconic(w.hwnd) != winapi.FALSE {
		return common.WindowStateMinimized
	}
	if winapi.IsZoomed(w.hwnd) != winapi.FALSE {
		return common.WindowStateMaximized
	}
	// There is no universal Win32 fullscreen flag. Do not infer it from size.
	return common.WindowStateNormal
}

func (w *Window) RequestState(state common.WindowState) error {
	if w.hwnd == 0 {
		return common.ErrUnavailable
	}
	var command int
	switch state {
	case common.WindowStateHidden:
		return w.Hide()
	case common.WindowStateNormal:
		command = winapi.SW_SHOWNORMAL
	case common.WindowStateMinimized:
		command = winapi.SW_MINIMIZE
	case common.WindowStateMaximized:
		command = winapi.SW_SHOWMAXIMIZED
	case common.WindowStateFullscreen:
		return common.ErrUnsupported
	default:
		return fmt.Errorf("invalid window state: %d", state)
	}
	if w.State() != state {
		// Submit the explicit show target; only native messages report the result.
		winapi.ShowWindow(w.hwnd, command)
	}
	return nil
}

func (w *Window) notifyState() {
	state := w.State()
	if state != w.state {
		w.state = state
		w.onEvent(events.StateEvent{State: state})
	}
}

func checkTransparency() error {
	if err := dcomp.Available(); err != nil {
		return fmt.Errorf("transparent surfaces require Windows 8 or later: %w", common.ErrUnsupported)
	}
	var enabled winapi.BOOL
	if err := winapi.DwmIsCompositionEnabled(&enabled); err != nil {
		return err
	}
	if enabled == winapi.FALSE {
		return fmt.Errorf("DWM composition: %w", common.ErrUnavailable)
	}
	return nil
}

func (w *Window) Transparent() bool { return w.transparent }

// Software still rasterizes entirely on the CPU. Only presentation uses the
// composition swap chain, avoiding WS_EX_LAYERED's conflict with CS_OWNDC and
// retaining ordinary client/non-client geometry. The upload image is reused.
func (w *Window) drawTransparent(img image.Image) error {
	if w.hwnd == 0 {
		return common.ErrUnavailable
	}
	if img.Bounds().Empty() {
		return nil
	}
	if err := w.ensureUpload(); err != nil {
		return err
	}
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	if w.uploadImage != nil {
		iw, ih := w.uploadImage.Size()
		if iw != width || ih != height {
			w.uploadImage.Destroy()
			w.uploadImage = nil
		}
	}
	var err error
	if w.uploadImage == nil {
		w.uploadImage, err = w.uploadPainter.NewImage(img)
	} else {
		err = w.uploadImage.Update(img)
	}
	if err != nil {
		return err
	}
	w.uploadPainter.Begin(float32(width), float32(height), 1)
	w.uploadPainter.Clear(graphics.Color{})
	w.uploadPainter.DrawImage(graphics.Rect(0, 0, float32(width), float32(height)), w.uploadImage)
	w.uploadPainter.End()
	return nil
}

func (w *Window) ensureUpload() error {
	if w.uploadPainter != nil {
		return nil
	}
	var err error
	w.uploadPainter, err = direct2d.NewPainter(w)
	return err
}

func (w *Window) releaseUpload() {
	if w.uploadImage != nil {
		w.uploadImage.Destroy()
		w.uploadImage = nil
	}
	if w.uploadPainter != nil {
		w.uploadPainter.Destroy()
		w.uploadPainter = nil
	}
}

func (w *Window) WorkAreaAt(point geometry.Point) (geometry.Rectangle, error) {
	if err := workarea.ValidatePoint(point); err != nil {
		return geometry.Rectangle{}, err
	}
	if w.hwnd == 0 {
		return geometry.Rectangle{}, common.ErrUnavailable
	}
	var origin winapi.POINT
	if winapi.ClientToScreen(w.hwnd, &origin) == 0 {
		return geometry.Rectangle{}, common.ErrUnavailable
	}
	scale := w.scaleFactor()
	x, y := float64(origin.X)+float64(point.X)*float64(scale), float64(origin.Y)+float64(point.Y)*float64(scale)
	if x < math.MinInt32 || x > math.MaxInt32 || y < math.MinInt32 || y > math.MaxInt32 {
		return geometry.Rectangle{}, fmt.Errorf("work area point outside native coordinate range: %v", point)
	}
	monitor := winapi.MonitorFromPoint(winapi.POINT{X: winapi.LONG(x), Y: winapi.LONG(y)}, winapi.MONITOR_DEFAULTTONEAREST)
	if monitor == 0 {
		return geometry.Rectangle{}, common.ErrUnavailable
	}
	info := winapi.MONITORINFO{Size: winapi.DWORD(unsafe.Sizeof(winapi.MONITORINFO{}))}
	if err := winapi.GetMonitorInfo(monitor, &info); err != nil {
		return geometry.Rectangle{}, err
	}
	r := info.Work
	result := geometry.Rect(float32(int64(r.Left)-int64(origin.X))/scale, float32(int64(r.Top)-int64(origin.Y))/scale,
		float32(int64(r.Right)-int64(r.Left))/scale, float32(int64(r.Bottom)-int64(r.Top))/scale)
	if !workarea.Valid(result) {
		return geometry.Rectangle{}, common.ErrUnavailable
	}
	return result, nil
}
