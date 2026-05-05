package cocoa

import (
	"github.com/golang-gui/goui/platform/cocoa/frameworks"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/appkit"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/foundation"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
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

func (p *Platform) NewEventQueue() (common.EventQueue, error) {
	return newEventQueue()
}

func (p *Platform) NewImage(width, height uint) (common.Image, error) {
	return graphics.MakeBitmap(0, 0, int(width), int(height), graphics.PixelFormatRGBA, nil), nil
}

func (p *Platform) NewWindow(onEvent events.EventHandler) (common.Window, error) {
	return newWindow(onEvent)
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
	foundation.AutoReleasePool(func() {
		appkit.NSApplicationClassId.SharedApplication()
	})
	return
}
