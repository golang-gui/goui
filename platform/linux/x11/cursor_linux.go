package x11

import (
	"errors"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/linux/libs/xcursor"
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
	if p.cursorTheme == nil {
		p.cursorTheme = newCursorTheme(p.display)
	}
	c.theme = p.cursorTheme
	c.theme.cursors[c] = struct{}{}
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
	theme        *cursorTheme
	shape        common.CursorShape
}

func (c *cursor) SetShape(shape common.CursorShape) {
	if c.window == nil || c.display == 0 {
		return
	}
	c.shape = shape

	var xc xlib.Cursor
	if shape == common.CursorNone {
		// Transparent cursor: create once, cache it. Uses a 1x1 transparent bitmap.
		if c.transparentC == 0 {
			c.transparentC = c.createTransparentCursor()
		}
		xc = c.transparentC
	} else {
		// Resolve semantic names in the current theme before the core fallback.
		if cached, ok := c.cache[shape]; ok {
			xc = cached
		} else {
			xc = loadNamedCursor(shape, func(name string) xlib.Cursor {
				if c.theme.available {
					return xcursor.LibraryLoadCursor(c.display, name)
				}
				return 0
			}, c.display.CreateFontCursor)
			if xc != 0 {
				c.cache[shape] = xc
			}
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
	if c.theme != nil {
		delete(c.theme.cursors, c)
		if len(c.theme.cursors) == 0 {
			c.theme.destroy()
			if platform != nil && platform.cursorTheme == c.theme {
				platform.cursorTheme = nil
			}
		}
		c.theme = nil
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
	case common.CursorResizeHorizontal:
		return xlib.XC_sb_h_double_arrow
	case common.CursorResizeVertical:
		return xlib.XC_sb_v_double_arrow
	case common.CursorResizeNWSE:
		return xlib.XC_top_left_corner
	case common.CursorResizeNESW:
		return xlib.XC_top_right_corner
	case common.CursorResizeLeft:
		return xlib.XC_left_side
	case common.CursorResizeRight:
		return xlib.XC_right_side
	case common.CursorResizeTop:
		return xlib.XC_top_side
	case common.CursorResizeBottom:
		return xlib.XC_bottom_side
	case common.CursorResizeTopLeft:
		return xlib.XC_top_left_corner
	case common.CursorResizeTopRight:
		return xlib.XC_top_right_corner
	case common.CursorResizeBottomLeft:
		return xlib.XC_bottom_left_corner
	case common.CursorResizeBottomRight:
		return xlib.XC_bottom_right_corner
	default:
		return xlib.XC_left_ptr // fallback to arrow
	}
}

// Names follow GTK's standard-name/legacy-name lookup. Themes can alias any
// number of names to the same image; no desktop-specific shape policy is needed.
func cursorNames(shape common.CursorShape) []string {
	switch shape {
	case common.CursorText:
		return []string{"text", "xterm"}
	case common.CursorPointing:
		return []string{"pointer", "hand2"}
	case common.CursorCrosshair:
		return []string{"crosshair", "cross"}
	case common.CursorForbidden:
		return []string{"not-allowed", "crossed_circle"}
	case common.CursorResizeHorizontal:
		return []string{"ew-resize", "sb_h_double_arrow"}
	case common.CursorResizeVertical:
		return []string{"ns-resize", "sb_v_double_arrow"}
	case common.CursorResizeNWSE:
		return []string{"nwse-resize", "size_fdiag"}
	case common.CursorResizeNESW:
		return []string{"nesw-resize", "size_bdiag"}
	case common.CursorResizeLeft:
		return []string{"w-resize", "left_side"}
	case common.CursorResizeRight:
		return []string{"e-resize", "right_side"}
	case common.CursorResizeTop:
		return []string{"n-resize", "top_side"}
	case common.CursorResizeBottom:
		return []string{"s-resize", "bottom_side"}
	case common.CursorResizeTopLeft:
		return []string{"nw-resize", "top_left_corner"}
	case common.CursorResizeTopRight:
		return []string{"ne-resize", "top_right_corner"}
	case common.CursorResizeBottomLeft:
		return []string{"sw-resize", "bottom_left_corner"}
	case common.CursorResizeBottomRight:
		return []string{"se-resize", "bottom_right_corner"}
	default:
		return []string{"default", "left_ptr"}
	}
}

func loadNamedCursor(shape common.CursorShape, load func(string) xlib.Cursor, core func(uint) xlib.Cursor) xlib.Cursor {
	for _, name := range cursorNames(shape) {
		if cursor := load(name); cursor != 0 {
			return cursor
		}
	}
	return core(cursorFontIndex(shape))
}

func (c *cursor) reloadTheme() {
	for _, resource := range c.cache {
		c.display.FreeCursor(resource)
	}
	clear(c.cache)
	c.current = 0
	// Reapply even if GUI deduplicates same-shape updates or the pointer is idle.
	c.SetShape(c.shape)
}
