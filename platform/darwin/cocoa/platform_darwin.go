package cocoa

import (
	"errors"
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/opengl"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/internal/desktopopen"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/coretext"

	"github.com/golang-gui/goui/platform/darwin/frameworks"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

type Platform struct {
}

var platform *Platform

func NewPlatform(appId string) (p *Platform, err error) {
	if platform != nil {
		return platform, nil
	}

	p, err = newPlatform(appId)
	if err != nil {
		return
	}

	platform = p
	return
}

func newPlatform(appId string) (p *Platform, err error) {
	err = frameworks.Init()
	if err != nil {
		return
	}

	err = checkApplicationBundle(appId)
	if err != nil {
		return nil, err
	}

	err = initWindowClass()
	if err != nil {
		return
	}

	p = new(Platform)
	AutoReleasePool(func() {
		app := NSApplicationClassId.SharedApplication()
		app.SetActivationPolicy(NSApplicationActivationPolicyRegular)
		app.FinishLaunching()
	})
	return
}

func (p *Platform) Destroy() {

}

func (p *Platform) Name() string {
	return "cocoa"
}

func (p *Platform) NewEventLoop() (common.EventLoop, error) {
	return newEventLoop()
}

func (p *Platform) NewImage(width, height uint) (common.Image, error) {
	return graphics.MakeBitmap(0, 0, int(width), int(height), graphics.PixelFormatRGBA, nil), nil
}

func (p *Platform) NewWindow(size geometry.Size, onEvent events.EventHandler, options common.WindowOptions) (common.Window, error) {
	window, err := newWindow(size, onEvent, options)
	if err != nil {
		return nil, err
	}
	return window, nil
}

func (p *Platform) NewPopup(owner common.Window, size geometry.Size, onEvent events.EventHandler, options common.PopupOptions) (common.Popup, error) {
	return newPopup(owner, size, onEvent, options)
}

func (p *Platform) NewTypography() (typography.Context, error) {
	return coretext.NewContext()
}

func (p *Platform) NewPainter(surface common.Surface) (painter graphics.Painter, err error) {
	switch common.GetPreferPainter() {
	case "software":
		return software.NewPainter(surface)
	default:
		if painter, err = opengl.NewPainter(surface); err != nil {
			return software.NewPainter(surface)
		}
		return
	}
}

func (p *Platform) NewInputMethod(window common.Window, handler common.InputMethodHandler) (common.InputMethod, error) {
	im, err := newInputMethod(window, handler)
	if err != nil {
		return nil, err
	}
	return im, nil
}

// NewCursor creates a cocoa cursor capability for window.
func (p *Platform) NewCursor(window common.Window) (common.Cursor, error) {
	return newCursor(window)
}

func (p *Platform) NewDragDrop(surface common.Surface) (common.DragDrop, error) {
	return newDragService(surface)
}

func (p *Platform) NewSettings() (common.Settings, error) {
	return newSettings()
}

func (p *Platform) NewClipboard() (common.Clipboard, error) {
	// The cocoa clipboard is stateless (talks to the shared general pasteboard),
	// so a fresh instance per call is safe.
	return newClipboard()
}

func (p *Platform) NewFileDialog() (common.FileDialog, error) {
	return newFileDialog()
}

func (p *Platform) OpenURL(rawURL string) (err error) {
	if err = desktopopen.ValidateURL(rawURL); err != nil {
		return err
	}
	AutoReleasePool(func() {
		url := NSURLClassId.URLWithString(ToNSString(rawURL))
		if url.ID == 0 || !NSWorkspaceClassId.SharedWorkspace().OpenURL(url) {
			err = errors.New("open URL: NSWorkspace rejected the request")
		}
	})
	return
}

func (p *Platform) OpenPath(path string) (err error) {
	abs, err := desktopopen.AbsolutePath(path)
	if err != nil {
		return err
	}
	AutoReleasePool(func() {
		url := NSURLClassId.FileURLWithPath(ToNSString(abs))
		if url.ID == 0 || !NSWorkspaceClassId.SharedWorkspace().OpenURL(url) {
			err = errors.New("open path: NSWorkspace rejected the request")
		}
	})
	return
}

func checkApplicationBundle(appId string) (err error) {
	if appId == "" {
		return nil
	}
	AutoReleasePool(func() {
		bundle := NSBundleClassId.MainBundle()
		key := ToNSString("CFBundlePackageType")
		defer key.Release()
		kind := StringFromObject(bundle.ObjectForInfoDictionaryKey(key).ID)
		bundleID := bundle.BundleIdentifier().UTF8String()
		err = validateBundleIdentity(appId, bundleID, kind == "APPL")
	})
	return
}

// A bare executable has no application bundle to configure. Never pretend that
// a requested ID supplies missing bundle resources or changes native identity.
func validateBundleIdentity(appId, bundleID string, applicationBundle bool) error {
	if appId != "" && (applicationBundle || bundleID != "") && appId != bundleID {
		return fmt.Errorf("cocoa: application ID %q does not match bundle identifier %q", appId, bundleID)
	}
	return nil
}
