// Package ios implements the iOS content-window platform.
package ios

import (
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/darwin/libs/dispatch"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/opengl"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/internal/mobile"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/coretext"
)

// Platform adopts the process's single iOS content host. Its resources belong
// to the system UI thread; Run only waits on the Go startup thread.
type Platform struct {
	appID         string
	loop          *EventLoop
	window        *Window
	destroyed     bool
	systemHosted  bool
	windowHandler func(common.Window) events.EventHandler
}

var instance *Platform

// NewPlatform prepares iOS support on the host's startup thread.
// The native entry must initialize UIKit first. Repeated creation
// returns the same live platform for the same application identity.
func NewPlatform(appID string) (*Platform, error) {
	if instance != nil {
		if instance.destroyed {
			return nil, common.ErrUnavailable
		}
		if instance.appID != appID {
			return nil, fmt.Errorf("ios: platform already created with application ID %q", instance.appID)
		}
		return instance, nil
	}
	if !iosApplication.running.Load() {
		return nil, common.ErrUnavailable
	}
	instance = &Platform{appID: appID}
	return instance, nil
}

func (*Platform) Name() string { return "ios" }

func (p *Platform) Destroy() {
	if p.destroyed {
		return
	}
	p.destroyed = true
	p.windowHandler = nil
	iosApplication.onHostReady = nil
	if p.window != nil {
		p.window.Destroy()
	}
}

func (p *Platform) NewEventLoop() (common.EventLoop, error) {
	if p.destroyed {
		return nil, common.ErrUnavailable
	}
	if p.loop != nil {
		return nil, common.ErrUnsupported
	}
	loop := mobile.NewEventLoop(func() { dispatch.MainAsync(iosApplication.callback) }, dispatch.IsMainThread)
	iosApplication.loop.Store(loop)
	p.loop = &EventLoop{loop}
	return p.loop, nil
}

func (p *Platform) NewWindow(size geometry.Size, handler events.EventHandler, options common.WindowOptions) (common.Window, error) {
	if p.destroyed || p.loop == nil {
		return nil, common.ErrUnavailable
	}
	if p.systemHosted || p.window != nil || options.Chrome == common.WindowChromeIntegrated {
		return nil, common.ErrUnsupported
	}
	if err := common.ValidateWindowSize(size); err != nil {
		return nil, err
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if options.Transparent {
		return nil, common.ErrUnsupported
	}
	w, err := newWindow(p.loop, handler)
	if err != nil {
		return nil, err
	}
	p.window = w
	w.bind(p.loop)
	return w, nil
}

// SetWindowHandler registers future system-host deliveries. The callback runs
// on the native UI thread and returns the host's event handler. Existing hosts
// retain their handler when this callback changes. See platform.WindowSource.
func (p *Platform) SetWindowHandler(handler func(common.Window) events.EventHandler) error {
	if p.destroyed || p.loop == nil {
		return common.ErrUnavailable
	}
	if !p.systemHosted {
		if handler == nil {
			return nil
		}
		if p.window != nil {
			return common.ErrUnsupported
		}
		p.systemHosted = true
	}
	p.windowHandler = handler
	p.loop.Post(func() {
		if !p.destroyed {
			iosApplication.onHostReady = func() { p.loop.Deliver(p.offerWindow) }
			p.offerWindow()
		}
	})
	return nil
}

func (p *Platform) offerWindow() {
	if p.destroyed || p.windowHandler == nil || iosHost.view.ID == 0 || (p.window != nil && !p.window.Destroyed()) {
		return
	}
	w, err := newWindow(p.loop, nil)
	if err != nil {
		panic(err)
	}
	p.window = w
	w.SetEventHandler(p.windowHandler(w))
	if !p.destroyed && p.window == w && !w.Destroyed() {
		iosHost.wanted.Store(true)
		w.bind(p.loop)
	}
}

func (*Platform) NewTypography() (typography.Context, error) { return coretext.NewContext() }

func (p *Platform) NewPainter(surface common.Surface) (graphics.Painter, error) {
	if p.destroyed || surface == nil || surface != p.window {
		return nil, common.ErrUnavailable
	}
	// Software supports construction before the native target is attached.
	// An explicit GPU request never silently selects another backend.
	switch common.GetPreferPainter() {
	case "", "software":
		return software.NewPainter(surface)
	case "opengl":
		// EAGL initialization requires the foreground Scene's drawing permit.
		// Report unavailability rather than waiting for a foreground Scene.
		if !p.window.SurfaceState().Ready {
			return nil, common.ErrUnavailable
		}
		return opengl.NewPainter(p.window)
	default:
		return nil, common.ErrUnsupported
	}
}

func (*Platform) NewImage(width, height uint) (common.Image, error) {
	limit := uint(^uint(0) >> 1)
	if width == 0 || height == 0 || width > limit/4/height {
		return nil, fmt.Errorf("ios: invalid image size %dx%d", width, height)
	}
	return graphics.MakeBitmap(0, 0, int(width), int(height), graphics.PixelFormatRGBA, nil), nil
}

// These services have not been integrated. GUI owns any documented fallback;
// the platform does not fabricate native capabilities or system settings.
func (*Platform) OpenURL(string) error  { return common.ErrUnsupported }
func (*Platform) OpenPath(string) error { return common.ErrUnsupported }
func (*Platform) NewPopup(common.Window, geometry.Size, events.EventHandler, common.PopupOptions) (common.Popup, error) {
	return nil, common.ErrUnsupported
}
func (*Platform) NewSettings() (common.Settings, error)   { return nil, common.ErrUnsupported }
func (*Platform) NewClipboard() (common.Clipboard, error) { return nil, common.ErrUnsupported }
func (*Platform) NewInputMethod(common.Window, common.InputMethodHandler) (common.InputMethod, error) {
	return nil, common.ErrUnsupported
}
func (*Platform) NewCursor(common.Window) (common.Cursor, error) { return nil, common.ErrUnsupported }
func (*Platform) NewDragDrop(common.Surface) (common.DragDrop, error) {
	return nil, common.ErrUnsupported
}
func (*Platform) NewFileDialog() (common.FileDialog, error) { return nil, common.ErrUnsupported }
