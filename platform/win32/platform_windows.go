package win32

import (
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
)

type Platform struct {
}

func NewPlatform() (Platform, error) {
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
