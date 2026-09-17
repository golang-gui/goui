package win32

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/direct2d"
	"github.com/golang-gui/goui/platform/graphics/opengl"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/internal/desktopopen"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/directwrite"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
	"github.com/golang-gui/goui/platform/windows/sdk/shell"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

type Platform struct {
	appId            string
	destroyed        bool
	icons            applicationIcons
	windowRegistered bool
	helperClass      winapi.LPWSTR
	instance         winapi.HINSTANCE
	helperWindow     winapi.HWND
	windowClass      winapi.LPWSTR
	windowTitle      winapi.LPWSTR
	wakeHandler      func()
}

var platform *Platform

func NewPlatform(appId string) (p *Platform, err error) {
	if platform != nil {
		if platform.destroyed {
			return nil, common.ErrUnavailable
		}
		if platform.appId != appId {
			return nil, fmt.Errorf("win32: platform already created with application ID %q, requested %q", platform.appId, appId)
		}
		return platform, nil
	}

	p, err = newPlatform(appId)
	if err != nil {
		return
	}

	platform = p
	return
}

func (p *Platform) Destroy() {
	if p.destroyed {
		return
	}
	// The caller must destroy windows first. If Windows refuses to unregister
	// a class still in use, retain its icons and allow a later cleanup attempt.
	if p.windowRegistered {
		if err := winapi.UnregisterClass(p.windowClass, p.instance); err != nil {
			return
		}
		p.windowRegistered = false
	}
	p.icons.destroy()
	if p.helperWindow != 0 {
		winapi.DestroyWindow(p.helperWindow)
		p.helperWindow = 0
	}
	if p.helperClass != nil {
		_ = winapi.UnregisterClass(p.helperClass, p.instance)
		p.helperClass = nil
	}
	p.wakeHandler = nil
	p.destroyed = true
}

func (p *Platform) Name() string {
	return "win32"
}

func (p *Platform) NewEventLoop() (common.EventLoop, error) {
	return newEventLoop(p)
}

// setWakeHandler lets the event loop inject its task-draining callback, which
// the helper window procedure invokes when the loop is woken.
func (p *Platform) setWakeHandler(fn func()) {
	p.wakeHandler = fn
}

func (p *Platform) NewWindow(size geometry.Size, handler events.EventHandler, options common.WindowOptions) (common.Window, error) {
	window, err := newWindow(size, handler, options)
	if err != nil {
		return nil, err
	}
	return window, nil
}

func (p *Platform) NewPopup(owner common.Window, size geometry.Size, handler events.EventHandler, options common.PopupOptions) (common.Popup, error) {
	return newPopup(owner, size, handler, options)
}

func (p *Platform) NewImage(width, height uint) (common.Image, error) {
	return graphics.MakeBitmap(0, 0, int(width), int(height), graphics.PixelFormatBGRA, nil), nil
}

func (p *Platform) NewTypography() (typography.Context, error) {
	return directwrite.NewContext()
}

func (p *Platform) NewPainter(surface common.Surface) (painter graphics.Painter, err error) {
	switch common.GetPreferPainter() {
	case "opengl":
		return opengl.NewPainter(surface)
	case "software":
		return newSoftwarePainter(surface)
	default:
		// D2D → OpenGL → Software
		if painter, err = direct2d.NewPainter(surface); err != nil {
			// TODO: add log
			if painter, err = opengl.NewPainter(surface); err != nil {
				return newSoftwarePainter(surface)
			}
		}
		return
	}
}

func newSoftwarePainter(surface common.Surface) (graphics.Painter, error) {
	if surface.Transparent() {
		win := windowMap[winapi.HWND(surface.NativeHandle())]
		if win == nil {
			return nil, common.ErrUnavailable
		}
		if err := win.ensureUpload(); err != nil {
			return nil, err
		}
	}
	return software.NewPainter(surface)
}

func (p *Platform) NewInputMethod(window common.Window, handler common.InputMethodHandler) (common.InputMethod, error) {
	return newInputMethod(window, handler)
}

func (p *Platform) NewCursor(window common.Window) (common.Cursor, error) {
	return newCursor(window)
}

func (p *Platform) NewSettings() (common.Settings, error) {
	return newSettings()
}

func (p *Platform) NewClipboard() (common.Clipboard, error) {
	// The win32 clipboard is stateless (talks to the shared system clipboard),
	// so a fresh instance per call is safe.
	return newClipboard()
}

func (p *Platform) NewFileDialog() (common.FileDialog, error) {
	return newFileDialog()
}

func (p *Platform) OpenURL(rawURL string) error {
	if err := desktopopen.ValidateURL(rawURL); err != nil {
		return err
	}
	return p.openExternal(rawURL, "open URL")
}

func (p *Platform) OpenPath(path string) error {
	abs, err := desktopopen.AbsolutePath(path)
	if err != nil {
		return err
	}
	return p.openExternal(abs, "open path")
}

func (p *Platform) openExternal(target, operation string) error {
	if p.destroyed {
		return fmt.Errorf("%s: %w", operation, common.ErrUnavailable)
	}
	file, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	verb, _ := syscall.UTF16PtrFromString("open")
	info := shell.SHELLEXECUTEINFO{
		Size: uint32(unsafe.Sizeof(shell.SHELLEXECUTEINFO{})),
		Mask: shell.SEE_MASK_FLAG_NO_UI,
		Verb: verb,
		File: file,
		Show: winapi.SW_SHOWNORMAL,
	}
	err = shell.ShellExecuteEx(&info)
	runtime.KeepAlive(file)
	runtime.KeepAlive(verb)
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}

func newPlatform(appId string) (p *Platform, err error) {
	p = &Platform{appId: appId}
	resources := p
	defer func() {
		if err != nil {
			resources.Destroy()
		}
	}()
	p.instance, _ = winapi.GetModuleHandle(nil)

	// Initialize COM as STA before creating any COM-dependent objects.
	if hr := com.Initialize(com.COINIT_APARTMENTTHREADED | com.COINIT_DISABLE_OLE1DDE); hr.Failed() {
		return nil, fmt.Errorf("win32: initialize COM: %w", hr)
	}
	if appId != "" {
		id, _ := syscall.UTF16PtrFromString(appId)
		if err = shell.SetCurrentProcessExplicitAppUserModelID(id); err != nil {
			return nil, fmt.Errorf("win32: set application ID: %w", err)
		}
	}

	// set DPI awareness
	if err = winapi.SetProcessDpiAwarenessContext(winapi.DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2); err != nil {
		// TODO: log
	}

	if err = p.createHelperWindow(); err != nil {
		return nil, err
	}

	if executable, pathErr := os.Executable(); pathErr == nil {
		p.icons = loadApplicationIcons(executable)
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
		WndProc:   winapi.MakeWindowProc(helperWindowProc),
		Instance:  p.instance,
		ClassName: cls,
	}
	_, err = winapi.RegisterClassEx(&wdc)
	if err != nil {
		return
	}
	p.helperClass = cls

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

// helperWindowProc invokes the injected wake handler when the event loop wakes
// the helper window. It stays independent of the event loop's internals so the
// native wake mechanism and task draining are cleanly separated.
func helperWindowProc(hwnd winapi.HWND, message winapi.UINT, wParam winapi.WPARAM, lParam winapi.LPARAM) winapi.LRESULT {
	if message == eventLoopWakeMessage {
		if platform != nil && platform.wakeHandler != nil {
			platform.wakeHandler()
		}
		return 0
	}
	return winapi.DefWindowProc(hwnd, message, wParam, lParam)
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
		Size:      winapi.Sizeof_WNDCLASSEX,
		Style:     winapi.CS_HREDRAW | winapi.CS_VREDRAW | winapi.CS_OWNDC,
		WndProc:   winapi.MakeWindowProc(windowProc),
		Instance:  p.instance,
		Cursor:    arrowCursor,
		ClassName: p.windowClass,
		Icon:      p.icons.large,
		IconSm:    p.icons.small,
	}
	_, err = winapi.RegisterClassEx(&wdc)
	p.windowRegistered = err == nil
	return
}

// applicationIcons belongs to the registered window class, not an individual
// window. Extraction is done once; the handles outlive every class instance.
type applicationIcons struct {
	large winapi.HICON
	small winapi.HICON
}

func loadApplicationIcons(executable string) (icons applicationIcons) {
	path, err := syscall.UTF16PtrFromString(executable)
	if err != nil {
		return
	}
	_, err = shell.ExtractIconEx(path, 0, &icons.large, &icons.small, 1)
	if err != nil {
		icons.destroy()
		return
	}
	// A module may supply only one usable size. Sharing it is safe while the
	// class owns both fields, provided destroy releases distinct handles once.
	if icons.large == 0 {
		icons.large = icons.small
	} else if icons.small == 0 {
		icons.small = icons.large
	}
	return
}

func (icons *applicationIcons) destroy() {
	if icons.large != 0 {
		winapi.DestroyIcon(icons.large)
	}
	if icons.small != 0 && icons.small != icons.large {
		winapi.DestroyIcon(icons.small)
	}
	*icons = applicationIcons{}
}
