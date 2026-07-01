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

func (p *Platform) NewWindow(onEvent events.EventHandler) (common.Window, error) {
	return newWindow(onEvent)
}

func (p *Platform) NewTypography() (typography.Context, error) {
	return coretext.NewContext()
}

func (p *Platform) NewPainter(win common.Window, typo typography.Context) (painter graphics.Painter, err error) {
	painter, err = opengl.NewPainter(win, typo)
	if err != nil {
		// TODO: error log
		painter, err = software.NewPainter(win, typo)
	}
	return
}

func (p *Platform) NewSettings(onChanged func()) (common.Settings, error) {
	return newSettings(onChanged)
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
