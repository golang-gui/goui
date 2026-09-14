package cocoa

import (
	"errors"

	"github.com/golang-gui/goui/platform/common"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
)

// newCursor creates the cocoa cursor capability for window.
func newCursor(window common.Window) (*cursor, error) {
	win, ok := window.(*Window)
	if !ok {
		return nil, errors.New("cocoa: invalid window for cursor")
	}
	c := &cursor{window: win}
	win.cursor = c
	return c, nil
}

// cursor is the cocoa Cursor capability. It resolves a CursorShape to an
// NSCursor and applies it. Because AppKit resets the cursor as the pointer
// moves (and gui dedups same-shape SetShape calls), the current NSCursor is
// stored on the window and re-applied from mouseMoved (see reapply). The
// hidden state (CursorNone) uses the global [NSCursor hide]/[unhide] pair,
// which persists across moves on its own.
type cursor struct {
	window  *Window
	current NSCursor // last resolved visible cursor; re-applied on mouseMoved
	hasCur  bool
	hidden  bool // whether we have an outstanding [NSCursor hide] to balance
}

func (c *cursor) SetShape(shape common.CursorShape) {
	if c.window == nil {
		return
	}
	if shape == common.CursorNone {
		// Hide is a global, ref-counted call; keep exactly one outstanding hide.
		if !c.hidden {
			NSCursorClassId.Hide()
			c.hidden = true
		}
		return
	}
	// Leaving the hidden state: balance the outstanding hide first.
	if c.hidden {
		NSCursorClassId.Unhide()
		c.hidden = false
	}
	c.current = nsCursorForShape(shape)
	c.hasCur = true
	c.current.Set()
}

// reapply re-sets the current visible cursor so it survives AppKit's own cursor
// resets during pointer motion. Called from mouseMoved; a no-op while hidden.
func (c *cursor) reapply() {
	if c.hidden || !c.hasCur {
		return
	}
	c.current.Set()
}

func (c *cursor) Destroy() {
	// Balance any outstanding hide so the cursor is not left invisible.
	if c.hidden {
		NSCursorClassId.Unhide()
		c.hidden = false
	}
	if c.window != nil {
		c.window.cursor = nil
		c.window = nil
	}
	c.hasCur = false
}

// nsCursorForShape maps a CursorShape to the matching NSCursor factory.
func nsCursorForShape(shape common.CursorShape) NSCursor {
	if position, ok := frameCursorPosition(shape); ok {
		if classRespondsToSelector(NSCursorClassId.ID(), "frameResizeCursorFromPosition:inDirections:") {
			return NSCursorClassId.FrameResizeCursorFromPositionInDirections(position, NSCursorFrameResizeDirectionsAll)
		}
		// Older AppKit exposes axis cursors, but no public diagonal cursor.
		switch shape {
		case common.CursorResizeLeft, common.CursorResizeRight:
			return NSCursorClassId.ResizeLeftRightCursor()
		case common.CursorResizeTop, common.CursorResizeBottom:
			return NSCursorClassId.ResizeUpDownCursor()
		default:
			return NSCursorClassId.ArrowCursor()
		}
	}
	switch shape {
	case common.CursorText:
		return NSCursorClassId.IBeamCursor()
	case common.CursorPointing:
		return NSCursorClassId.PointingHandCursor()
	case common.CursorCrosshair:
		return NSCursorClassId.CrosshairCursor()
	case common.CursorForbidden:
		return NSCursorClassId.OperationNotAllowedCursor()
	case common.CursorResizeHorizontal:
		return NSCursorClassId.ResizeLeftRightCursor()
	case common.CursorResizeVertical:
		return NSCursorClassId.ResizeUpDownCursor()
	case common.CursorResizeNWSE, common.CursorResizeNESW:
		// Public frame-resize cursors are available on newer AppKit. Older
		// systems have no public diagonal factory; keep the standard arrow
		// fallback instead of reaching into private cursor selectors.
		if classRespondsToSelector(NSCursorClassId.ID(), "frameResizeCursorFromPosition:inDirections:") {
			position := NSCursorFrameResizePositionTopLeft
			if shape == common.CursorResizeNESW {
				position = NSCursorFrameResizePositionTopRight
			}
			return NSCursorClassId.FrameResizeCursorFromPositionInDirections(position, NSCursorFrameResizeDirectionsAll)
		}
		return NSCursorClassId.ArrowCursor()
	default:
		return NSCursorClassId.ArrowCursor()
	}
}

func frameCursorPosition(shape common.CursorShape) (NSCursorFrameResizePosition, bool) {
	switch shape {
	case common.CursorResizeLeft:
		return NSCursorFrameResizePositionLeft, true
	case common.CursorResizeRight:
		return NSCursorFrameResizePositionRight, true
	case common.CursorResizeTop:
		return NSCursorFrameResizePositionTop, true
	case common.CursorResizeBottom:
		return NSCursorFrameResizePositionBottom, true
	case common.CursorResizeTopLeft:
		return NSCursorFrameResizePositionTopLeft, true
	case common.CursorResizeTopRight:
		return NSCursorFrameResizePositionTopRight, true
	case common.CursorResizeBottomLeft:
		return NSCursorFrameResizePositionBottom | NSCursorFrameResizePositionLeft, true
	case common.CursorResizeBottomRight:
		return NSCursorFrameResizePositionBottom | NSCursorFrameResizePositionRight, true
	default:
		return 0, false
	}
}
