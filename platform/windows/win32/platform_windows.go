package win32

import (
	"syscall"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/direct2d"
	"github.com/golang-gui/goui/platform/graphics/opengl"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/directwrite"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

type Platform struct {
	instance     winapi.HINSTANCE
	helperWindow winapi.HWND
	windowClass  winapi.LPWSTR
	windowTitle  winapi.LPWSTR
}

var platform *Platform

func NewPlatform() (p *Platform, err error) {
	if platform != nil {
		return p, nil
	}

	p, err = newPlatform()
	if err != nil {
		return
	}

	platform = p
	return
}

func (p *Platform) Destroy() {

}

func (p *Platform) Name() string {
	return "win32"
}

func (p *Platform) NewEventQueue() (common.EventQueue, error) {
	return newEventQueue()
}

func (p *Platform) NewWindow(handler events.EventHandler) (common.Window, error) {
	return newWindow(handler)
}

func (p *Platform) NewImage(width, height uint) (common.Image, error) {
	return graphics.MakeBitmap(0, 0, int(width), int(height), graphics.PixelFormatBGRA, nil), nil
}

func (p *Platform) NewTypography() (typography.Context, error) {
	return directwrite.NewContext()
}

func (p *Platform) NewPainter(win common.Window, typo typography.Context) (painter graphics.Painter, err error) {
	// TODO: error log
	painter, err = direct2d.NewPainter(win, typo)
	if err != nil {
		painter, err = opengl.NewPainter(win, typo)
		if err != nil {
			return software.NewPainter(win, typo)
		}
	}
	return
}

func newPlatform() (p *Platform, err error) {
	p = new(Platform)
	p.instance, _ = winapi.GetModuleHandle(nil)

	// set DPI awareness
	if err = winapi.SetProcessDpiAwarenessContext(winapi.DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2); err != nil {
		// TODO: log
	}

	if err = p.createHelperWindow(); err != nil {
		return nil, err
	}

	if err = p.registerWindow(); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Platform) createHelperWindow() (err error) {
	// TODO: return detail error
	cls, _ := syscall.UTF16PtrFromString("GOUI Helper")
	wdc := winapi.WNDCLASSEX{
		Size:      winapi.Sizeof_WNDCLASSEX,
		Style:     winapi.CS_OWNDC,
		WndProc:   winapi.GetDefWindowProc(),
		Instance:  p.instance,
		ClassName: cls,
	}
	_, err = winapi.RegisterClassEx(&wdc)
	if err != nil {
		return
	}

	p.helperWindow, err = winapi.CreateWindowEx(winapi.WS_EX_OVERLAPPEDWINDOW, cls, cls,
		winapi.WS_CLIPSIBLINGS|winapi.WS_CLIPCHILDREN,
		0, 0, 1, 1, 0, 0,
		p.instance, nil)

	if err != nil {
		return
	}

	winapi.ShowWindow(p.helperWindow, winapi.SW_HIDE)

	var msg winapi.MSG
	for {
		has, _ := winapi.PeekMessage(&msg, p.helperWindow, 0, 0, winapi.PM_REMOVE)
		if has == winapi.FALSE {
			break
		}
		winapi.TranslateMessage(&msg)
		winapi.DispatchMessage(&msg)
	}

	return nil
}

func (p *Platform) registerWindow() (err error) {
	p.windowClass, err = syscall.UTF16PtrFromString("GOUI Window")
	if err != nil {
		return
	}

	p.windowTitle, err = syscall.UTF16PtrFromString("Window")
	if err != nil {
		return
	}

	arrowCursor, _ := winapi.LoadCursor(0, winapi.IDC_ARROW)
	wdc := winapi.WNDCLASSEX{
		Size:       winapi.Sizeof_WNDCLASSEX,
		Style:      winapi.CS_HREDRAW | winapi.CS_VREDRAW | winapi.CS_OWNDC,
		WndProc:    winapi.MakeWindowProc(windowProc),
		Instance:   p.instance,
		Cursor:     arrowCursor,
		ClassName:  p.windowClass,
		Background: winapi.HBRUSH(winapi.COLOR_WINDOWFRAME),
	}
	_, err = winapi.RegisterClassEx(&wdc)
	return
}
