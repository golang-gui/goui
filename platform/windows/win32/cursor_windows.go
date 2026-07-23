package win32

import (
	"errors"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

// cursor is the win32 Cursor: it owns the window's current shape and re-applies
// it on WM_SETCURSOR (DefWindowProc would otherwise reset to the class cursor).
type cursor struct {
	window *Window
	shape  common.CursorShape
}

// newCursor creates the win32 cursor capability for window and installs it so
// WndProc can consult it on WM_SETCURSOR.
func newCursor(window common.Window) (*cursor, error) {
	win, ok := window.(*Window)
	if !ok || win.hwnd == 0 {
		return nil, errors.New("win32: invalid window for cursor")
	}
	c := &cursor{window: win, shape: common.CursorDefault}
	win.cursor = c
	return c, nil
}

// SetShape records the shape and applies it immediately. The recorded shape is
// re-applied on every WM_SETCURSOR while the pointer is over the client area.
func (c *cursor) SetShape(shape common.CursorShape) {
	c.shape = shape
	if c.window != nil && c.window.hwnd != 0 {
		c.apply()
	}
}

// apply sets the native cursor for the current shape.
func (c *cursor) apply() {
	winapi.SetCursor(loadCursor(c.shape))
}

func (c *cursor) Destroy() {
	if c.window != nil {
		c.window.cursor = nil
		c.window = nil
	}
}

// cursorCache holds the shared standard cursors, loaded once. Standard cursors
// are process-shared and need no freeing.
var cursorCache = map[common.CursorShape]winapi.HCURSOR{}

// loadCursor returns the native handle for shape, loading and caching it on
// first use. Unknown shapes fall back to the arrow.
func loadCursor(shape common.CursorShape) winapi.HCURSOR {
	// CursorNone maps to the NULL cursor: SetCursor(0) hides the pointer, and
	// our WM_SETCURSOR handler re-applies it on every move so it stays hidden.
	if shape == common.CursorNone {
		return 0
	}
	if h, ok := cursorCache[shape]; ok {
		return h
	}
	h, _ := winapi.LoadCursor(0, cursorID(shape))
	cursorCache[shape] = h
	return h
}

func cursorID(shape common.CursorShape) winapi.LPWSTR {
	switch shape {
	case common.CursorText:
		return winapi.IDC_IBEAM
	case common.CursorPointing:
		return winapi.IDC_HAND
	case common.CursorCrosshair:
		return winapi.IDC_CROSS
	case common.CursorForbidden:
		return winapi.IDC_NO
	default:
		return winapi.IDC_ARROW
	}
}
