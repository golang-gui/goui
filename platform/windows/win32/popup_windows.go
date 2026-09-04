package win32

import (
	"image"
	"runtime"
	"unsafe"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"

	"github.com/goexlib/mathx"
)

// Popup is a borderless WS_POPUP window owned by another window. It reuses the
// Window machinery (event routing, draw) and only adds borderless, non-taskbar
// creation plus owner-client-relative positioning.
type Popup struct {
	win   *Window
	owner common.Window
}

func newPopup(owner common.Window, width, height float32, onEvent events.EventHandler) (common.Popup, error) {
	win := &Window{
		onEvent:    onEvent,
		scale:      1,
		noActivate: true, // clicking the popup must not steal focus from its owner
	}

	ownerHwnd := winapi.HWND(owner.NativeHandle())

	// WS_POPUP has no frame, so the window size equals the client size. Create at
	// the minimum containing physical size (using the owner's DPI) so the native
	// paint surface is large enough from the start.
	scale := hwndScale(ownerHwnd)
	w, h := popupPhysicalExtent(width, scale), popupPhysicalExtent(height, scale)

	var err error
	win.hwnd, err = winapi.CreateWindowEx(
		winapi.WS_EX_TOOLWINDOW|winapi.WS_EX_NOACTIVATE, // no taskbar entry, no activation
		platform.windowClass, platform.windowTitle,
		winapi.WS_POPUP, // borderless
		0, 0, w, h,
		ownerHwnd, 0, platform.instance,
		unsafe.Pointer(win))
	if err != nil {
		return nil, err
	}

	runtime.KeepAlive(win)
	return &Popup{win: win, owner: owner}, nil
}

func (p *Popup) NativeHandle() uintptr      { return p.win.NativeHandle() }
func (p *Popup) Destroy()                   { p.win.Destroy() }
func (p *Popup) Hide() error                { return p.win.Hide() }
func (p *Popup) RequestPaint() error        { return p.win.RequestPaint() }
func (p *Popup) Draw(img image.Image) error { return p.win.Draw(img) }

// Show maps the popup without stealing activation from the owner. Unlike
// Window.Show (SW_SHOWNORMAL, which activates) it uses SW_SHOWNOACTIVATE, so it
// cannot delegate to p.win.Show.
func (p *Popup) Show() error {
	if p.win.hwnd == 0 {
		return nil
	}
	winapi.ShowWindow(p.win.hwnd, winapi.SW_SHOWNOACTIVATE)
	winapi.UpdateWindow(p.win.hwnd)
	return nil
}

// SetPosition places the popup at an owner-client-local logical point, converted
// to screen coordinates via the owner window.
func (p *Popup) SetPosition(x, y float32) {
	if p.win.hwnd == 0 {
		return
	}
	scale := p.ownerScale()
	pt := winapi.POINT{X: winapi.LONG(x * scale), Y: winapi.LONG(y * scale)}
	winapi.ClientToScreen(winapi.HWND(p.owner.NativeHandle()), &pt)
	winapi.SetWindowPos(p.win.hwnd, 0, int(pt.X), int(pt.Y), 0, 0,
		winapi.SWP_NOSIZE|winapi.SWP_NOZORDER|winapi.SWP_NOACTIVATE)
}

func (p *Popup) SetSize(width, height float32) {
	if p.win.hwnd == 0 {
		return
	}
	scale := p.ownerScale()
	w := popupPhysicalExtent(width, scale)
	h := popupPhysicalExtent(height, scale)
	winapi.SetWindowPos(p.win.hwnd, 0, 0, 0, w, h,
		winapi.SWP_NOMOVE|winapi.SWP_NOZORDER|winapi.SWP_NOACTIVATE)
}

// popupPhysicalExtent converts a requested logical extent to the minimum
// integer pixel extent that can contain it. Truncation (or rounding down) makes
// the native client area smaller than its measured Direct2D/DirectWrite content
// and can incorrectly turn an exact-fit viewport into a scrollable one.
func popupPhysicalExtent(logical, scale float32) int {
	pixels := int(mathx.Ceil(logical * scale))
	if pixels < 1 {
		return 1
	}
	return pixels
}

// ownerScale returns the owner window's device scale so popup logical coords map
// to the same physical pixels the owner uses.
func (p *Popup) ownerScale() float32 {
	return hwndScale(winapi.HWND(p.owner.NativeHandle()))
}

// hwndScale returns a window's device scale (DPI/96), falling back to 1.
func hwndScale(hwnd winapi.HWND) float32 {
	dpi, err := winapi.GetDpiForWindow(hwnd)
	if err != nil || dpi == 0 {
		return 1
	}
	return float32(dpi) / 96
}
