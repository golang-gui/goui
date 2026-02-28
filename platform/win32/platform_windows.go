package win32

import (
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/win32/winapi"
	"image"
)

type Platform struct {
}

func NewPlatform() (Platform, error) {
	if err := winapi.SetProcessDpiAwarenessContext(winapi.DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2); err != nil {
		return Platform{}, err
	}
	return Platform{}, nil
}

func (p Platform) Destroy() {

}

func (p Platform) Name() string {
	return "win32"
}

func (p Platform) NewEventQueue() (common.EventQueue, error) {
	return newEventQueue()
}

func (p Platform) NewWindow(handler events.EventHandler) (common.Window, error) {
	return newWindow(handler)
}

func (p Platform) NewImage(width, height uint) (common.Image, error) {
	return common.NewBGRAImage(image.Rect(0, 0, int(width), int(height))), nil
}
