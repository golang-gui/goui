package platform

import (
	"errors"
	"github.com/golang-gui/goui/platform/common"
	"runtime"
)

type Platform interface {
	Destroy()
	Name() string
	NewImage(width, height int) (common.Image, error)
	NewWindow(handler EventHandler) (Window, error)
	NewEventQueue() (EventQueue, error)
}

var ErrUnsupported = errors.New("unsupported platform")

func NewPlatform(name string) (Platform, error) {
	return newPlatform(name)
}

func DefaultName() string {
	switch runtime.GOOS {
	case "windows":
		return "win32"
	}
	return ""
}
