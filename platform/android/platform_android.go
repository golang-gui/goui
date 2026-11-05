// Package android implements the Android content-window platform.
package android

import (
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/opengl"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/internal/androidruntime"
	"github.com/golang-gui/goui/platform/internal/mobile"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/androidtext"
)

// Platform adopts the process's single Android content host. Its resources are
// thread-affine to the GOUI thread; Android callbacks remain on the system thread.
type Platform struct {
	appID     string
	loop      *EventLoop
	window    *Window
	destroyed bool
}

var instance *Platform

// NewPlatform initializes Android support on the host's attached GOUI thread.
// A matching native host and typography Dex component must be packaged in the
// APK. Repeated creation returns the same live platform for the same identity.
func NewPlatform(appID string) (*Platform, error) {
	if instance != nil {
		if instance.destroyed {
			return nil, common.ErrUnavailable
		}
		if instance.appID != appID {
			return nil, fmt.Errorf("android: platform already created with application ID %q", instance.appID)
		}
		return instance, nil
	}
	if _, err := androidruntime.Environment(); err != nil {
		return nil, err
	}
	instance = &Platform{appID: appID}
	return instance, nil
}

func (*Platform) Name() string { return "android" }

func (p *Platform) Destroy() {
	if p.destroyed {
		return
	}
	p.destroyed = true
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
	p.loop = &EventLoop{mobile.NewEventLoop()}
	return p.loop, nil
}

func (p *Platform) NewWindow(size geometry.Size, handler events.EventHandler, options common.WindowOptions) (common.Window, error) {
	if p.destroyed || p.loop == nil {
		return nil, common.ErrUnavailable
	}
	if p.window != nil || options.Chrome == common.WindowChromeIntegrated {
		return nil, common.ErrUnsupported
	}
	w, err := newWindow(p.loop, size, handler, options)
	if err != nil {
		return nil, err
	}
	p.window = w
	return w, nil
}

func (*Platform) NewTypography() (typography.Context, error) { return androidtext.NewContext() }

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
		return opengl.NewPainter(p.window)
	default:
		return nil, common.ErrUnsupported
	}
}

func (*Platform) NewImage(width, height uint) (common.Image, error) {
	limit := uint(^uint(0) >> 1)
	if width == 0 || height == 0 || width > limit/4/height {
		return nil, fmt.Errorf("android: invalid image size %dx%d", width, height)
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
