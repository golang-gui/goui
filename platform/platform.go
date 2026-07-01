package platform

import (
	"errors"
	"runtime"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

type (
	Event        = events.Event
	Image        = common.Image
	Settings     = common.Settings
	ColorScheme  = common.ColorScheme
	Window       = common.Window
	EventLoop    = common.EventLoop
	EventHandler = events.EventHandler
)

const (
	ColorSchemeLight = common.ColorSchemeLight
	ColorSchemeDark  = common.ColorSchemeDark
)

// Platform owns low-level operating-system resources. It and every object
// created from it must be used on the same OS thread, except for EventLoop.Post
// and EventLoop.Quit.
type Platform interface {
	Destroy()
	Name() string
	NewImage(width, height uint) (Image, error)
	NewWindow(handler EventHandler) (Window, error)
	NewEventLoop() (EventLoop, error)
	NewTypography() (typography.Context, error)
	NewPainter(win Window, typo typography.Context) (graphics.Painter, error)
	NewSettings(onChanged func()) (Settings, error)
}

var ErrUnsupported = errors.New("unsupported platform")

func NewPlatform(name string) (Platform, error) {
	return newPlatform(name)
}

func DefaultName() string {
	switch runtime.GOOS {
	case "windows":
		return "win32"
	case "linux":
		return "x11"
	case "darwin":
		return "cocoa"
	}
	return ""
}
