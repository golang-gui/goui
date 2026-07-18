package cocoa

import (
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/opengl"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/coretext"

	"github.com/golang-gui/goui/platform/darwin/frameworks"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

type Platform struct {
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
	return "cocoa"
}

func (p *Platform) NewEventLoop() (common.EventLoop, error) {
	return newEventLoop()
}

func (p *Platform) NewImage(width, height uint) (common.Image, error) {
	return graphics.MakeBitmap(0, 0, int(width), int(height), graphics.PixelFormatRGBA, nil), nil
}

func (p *Platform) NewWindow(width, height float32, onEvent events.EventHandler) (common.Window, error) {
	return newWindow(width, height, onEvent)
}

func (p *Platform) NewPopup(owner common.Window, width, height float32, onEvent events.EventHandler) (common.Popup, error) {
	return newPopup(owner, width, height, onEvent)
}

func (p *Platform) NewTypography() (typography.Context, error) {
	return coretext.NewContext()
}

func (p *Platform) NewPainter(surface common.Surface, typo typography.Context) (painter graphics.Painter, err error) {
	painter, err = opengl.NewPainter(surface, typo)
	if err != nil {
		// TODO: error log
		painter, err = software.NewPainter(surface, typo)
	}
	return
}

func (p *Platform) NewInputMethod(window common.Window, handler common.InputMethodHandler) (common.InputMethod, error) {
	return newInputMethod(window, handler)
}

func (p *Platform) NewSettings() (common.Settings, error) {
	return newSettings()
}

func (p *Platform) NewClipboard() (common.Clipboard, error) {
	// The cocoa clipboard is stateless (talks to the shared general pasteboard),
	// so a fresh instance per call is safe.
	return newClipboard()
}

func newPlatform() (p *Platform, err error) {
	err = frameworks.Init()
	if err != nil {
		return
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
