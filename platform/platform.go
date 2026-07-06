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
	Surface      = common.Surface
	Window       = common.Window
	Popup        = common.Popup
	EventLoop    = common.EventLoop
	Clipboard    = common.Clipboard
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
	// NewWindow creates a top-level window. width/height is a logical (DIP)
	// preferred size; the platform may override it (WM/compositor), and the
	// authoritative size always arrives via SizeEvent.
	NewWindow(width, height float32, handler EventHandler) (Window, error)
	// NewPopup creates a borderless popup owned by owner. width/height is its
	// authoritative logical (DIP) size; use SetSize to change it later.
	NewPopup(owner Window, width, height float32, handler EventHandler) (Popup, error)
	NewEventLoop() (EventLoop, error)
	NewTypography() (typography.Context, error)
	// NewPainter creates a painter for any paint target — a Window or a Popup.
	NewPainter(surface Surface, typo typography.Context) (graphics.Painter, error)
	NewSettings() (Settings, error)
	NewClipboard() (Clipboard, error)
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
