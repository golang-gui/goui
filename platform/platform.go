package platform

import (
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

type (
	Event              = events.Event
	Image              = common.Image
	Settings           = common.Settings
	ColorScheme        = common.ColorScheme
	Surface            = common.Surface
	Window             = common.Window
	DesktopWindow      = common.DesktopWindow
	WindowOptions      = common.WindowOptions
	WindowChrome       = common.WindowChrome
	WindowState        = common.WindowState
	WindowHit          = common.WindowHit
	WindowEdge         = common.WindowEdge
	Popup              = common.Popup
	EventLoop          = common.EventLoop
	Clipboard          = common.Clipboard
	InputMethod        = common.InputMethod
	InputMethodHandler = common.InputMethodHandler
	InputMethodResult  = common.InputMethodResult
	InputMethodKind    = common.InputMethodKind
	Cursor             = common.Cursor
	CursorShape        = common.CursorShape
	EventHandler       = events.EventHandler
	FileFilter         = common.FileFilter
	DialogOptions      = common.DialogOptions
	FileDialog         = common.FileDialog
)

const (
	WindowChromeUnknown    = common.WindowChromeUnknown
	WindowChromeNative     = common.WindowChromeNative
	WindowChromeIntegrated = common.WindowChromeIntegrated
	WindowChromeNone       = common.WindowChromeNone

	WindowStateUnknown    = common.WindowStateUnknown
	WindowStateNormal     = common.WindowStateNormal
	WindowStateHidden     = common.WindowStateHidden
	WindowStateMinimized  = common.WindowStateMinimized
	WindowStateMaximized  = common.WindowStateMaximized
	WindowStateFullscreen = common.WindowStateFullscreen

	WindowHitDefault     = common.WindowHitDefault
	WindowHitClient      = common.WindowHitClient
	WindowHitSysMenu     = common.WindowHitSysMenu
	WindowHitCaption     = common.WindowHitCaption
	WindowHitMinimize    = common.WindowHitMinimize
	WindowHitMaximize    = common.WindowHitMaximize
	WindowHitClose       = common.WindowHitClose
	WindowHitTop         = common.WindowHitTop
	WindowHitBottom      = common.WindowHitBottom
	WindowHitLeft        = common.WindowHitLeft
	WindowHitRight       = common.WindowHitRight
	WindowHitTopLeft     = common.WindowHitTopLeft
	WindowHitTopRight    = common.WindowHitTopRight
	WindowHitBottomLeft  = common.WindowHitBottomLeft
	WindowHitBottomRight = common.WindowHitBottomRight

	WindowEdgeTop         = common.WindowEdgeTop
	WindowEdgeBottom      = common.WindowEdgeBottom
	WindowEdgeLeft        = common.WindowEdgeLeft
	WindowEdgeRight       = common.WindowEdgeRight
	WindowEdgeTopLeft     = common.WindowEdgeTopLeft
	WindowEdgeTopRight    = common.WindowEdgeTopRight
	WindowEdgeBottomLeft  = common.WindowEdgeBottomLeft
	WindowEdgeBottomRight = common.WindowEdgeBottomRight

	ColorSchemeLight = common.ColorSchemeLight
	ColorSchemeDark  = common.ColorSchemeDark

	InputMethodCommit  = common.InputMethodCommit
	InputMethodPreedit = common.InputMethodPreedit

	CursorDefault   = common.CursorDefault
	CursorText      = common.CursorText
	CursorPointing  = common.CursorPointing
	CursorCrosshair = common.CursorCrosshair
	CursorForbidden = common.CursorForbidden
	CursorNone      = common.CursorNone
)

// Platform owns low-level operating-system resources. It and every object
// created from it must be used on the same OS thread, except for EventLoop.Post
// and EventLoop.Quit.
type Platform interface {
	Destroy()
	Name() string
	NewImage(width, height uint) (Image, error)
	// NewWindow creates a top-level window with creation-time options. The
	// authoritative client size arrives via SizeEvent. Initial notifications
	// may arrive synchronously before NewWindow returns.
	NewWindow(size geometry.Size, handler EventHandler, options WindowOptions) (Window, error)
	// NewPopup creates a borderless popup owned by owner. width/height is its
	// requested logical (DIP) size; native pixel quantization may adjust it, and
	// the authoritative logical and physical client size arrives via SizeEvent.
	NewPopup(owner Window, width, height float32, handler EventHandler) (Popup, error)
	NewEventLoop() (EventLoop, error)
	NewTypography() (typography.Context, error)
	// NewPainter creates a painter for any paint target — a Window or a Popup.
	NewPainter(surface Surface) (graphics.Painter, error)
	NewSettings() (Settings, error)
	NewClipboard() (Clipboard, error)
	// NewInputMethod creates the text-composition (IME) capability for window.
	// handler receives committed/preedit text. Returns ErrUnsupported when the
	// platform or window does not support input methods.
	NewInputMethod(window Window, handler InputMethodHandler) (InputMethod, error)
	// NewCursor creates the mouse-cursor capability for window, used to change
	// the window's current cursor shape. Returns ErrUnsupported when the platform
	// or window does not support cursor control.
	NewCursor(window Window) (Cursor, error)
	// NewFileDialog creates the native file dialog capability. Returns
	// ErrUnsupported when the platform does not support file dialogs.
	NewFileDialog() (FileDialog, error)
}

var ErrUnsupported = common.ErrUnsupported
var ErrUnavailable = common.ErrUnavailable

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
