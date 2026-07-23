package x11

import (
	"errors"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

var errNoCursor = errors.New("x11: cursor not supported")

// newCursor creates the x11 cursor capability for window.
func (p *Platform) newCursor(window common.Window) (*cursor, error) {
	win, ok := window.(*Window)
	if !ok {
		return nil, errNoCursor
	}
	c := &cursor{
		window:  win,
		display: p.display,
		cache:   make(map[common.CursorShape]xlib.Cursor),
	}
	win.cursor = c
	return c, nil
}

// cursor is the x11 Cursor capability: caches standard cursors and applies them
// to the window. The transparent cursor (CursorNone) is created lazily once.
type cursor struct {
	window       *Window
	display      xlib.Display
	cache        map[common.CursorShape]xlib.Cursor
	current      xlib.Cursor
	transparentC xlib.Cursor // lazy-created 1x1 transparent cursor for CursorNone
}

func (c *cursor) SetShape(shape common.CursorShape) {
	if c.window == nil || c.display == 0 {
		return
	}

	var xc xlib.Cursor
	if shape == common.CursorNone {
		// Transparent cursor: create once, cache it. Uses a 1x1 transparent bitmap.
		if c.transparentC == 0 {
			c.transparentC = c.createTransparentCursor()
		}
		xc = c.transparentC
	} else {
		// Standard cursor: load from cursor font and cache it.
		if cached, ok := c.cache[shape]; ok {
			xc = cached
		} else {
			xc = c.display.CreateFontCursor(cursorFontIndex(shape))
			c.cache[shape] = xc
		}
	}

	if xc == c.current {
		return
	}
	c.current = xc
	c.display.DefineCursor(c.window.wid, xc)
	c.display.Flush()
}

func (c *cursor) Destroy() {
	if c.display == 0 {
		return
	}
	// Undefine the cursor from the window first.
	if c.window != nil && c.window.wid != 0 {
		c.display.UndefineCursor(c.window.wid)
	}
	// Free all cached cursors.
	for _, xc := range c.cache {
		c.display.FreeCursor(xc)
	}
	if c.transparentC != 0 {
		c.display.FreeCursor(c.transparentC)
		c.transparentC = 0
	}
	c.cache = nil
	c.current = 0
	if c.window != nil {
		c.window.cursor = nil
		c.window = nil
	}
	c.display = 0
}

// createTransparentCursor creates a 1x1 fully transparent cursor for hiding.
func (c *cursor) createTransparentCursor() xlib.Cursor {
	// Create a 1x1 transparent bitmap (all bits 0).
	data := []byte{0}
	pixmap := c.display.CreateBitmapFromData(xlib.Drawable(c.window.wid), data, 1, 1)
	defer c.display.FreePixmap(pixmap)

	// Black color (transparent when used with a zero bitmap).
	color := xlib.XColor{}

	// Create cursor from the transparent pixmap (source = mask = same 1x1 zero bitmap).
	return c.display.CreatePixmapCursor(pixmap, pixmap, &color, &color, 0, 0)
}

// cursorFontIndex maps common.CursorShape to X cursor font indices (from cursorfont.h).
func cursorFontIndex(shape common.CursorShape) uint {
	switch shape {
	case common.CursorDefault:
		return xlib.XC_left_ptr
	case common.CursorText:
		return xlib.XC_xterm
	case common.CursorPointing:
		return xlib.XC_hand2
	case common.CursorCrosshair:
		return xlib.XC_crosshair
	case common.CursorForbidden:
		return xlib.XC_X_cursor
	default:
		return xlib.XC_left_ptr // fallback to arrow
	}
}
