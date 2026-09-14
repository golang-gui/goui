package gui

import "github.com/golang-gui/goui/platform"

// Cursor is a mouse-pointer appearance: either a standard shape or (in the
// future) a custom image/animation.
type Cursor interface {
	isCursor() // seal: only gui-provided types (CursorShape, future imageCursor) satisfy this
}

// CursorShape wraps platform.CursorShape as a new named type so it can satisfy
// the Cursor interface (type aliases cannot have methods).
type CursorShape platform.CursorShape

// isCursor makes CursorShape satisfy the Cursor interface, so shape constants
// like CursorPointing can be passed to SetCursor(Cursor) without wrapping.
func (CursorShape) isCursor() {}

const (
	CursorDefault           = CursorShape(platform.CursorDefault)
	CursorText              = CursorShape(platform.CursorText)
	CursorPointing          = CursorShape(platform.CursorPointing)
	CursorCrosshair         = CursorShape(platform.CursorCrosshair)
	CursorForbidden         = CursorShape(platform.CursorForbidden)
	CursorNone              = CursorShape(platform.CursorNone)
	CursorResizeHorizontal  = CursorShape(platform.CursorResizeHorizontal)
	CursorResizeVertical    = CursorShape(platform.CursorResizeVertical)
	CursorResizeNWSE        = CursorShape(platform.CursorResizeNWSE)
	CursorResizeNESW        = CursorShape(platform.CursorResizeNESW)
	CursorResizeLeft        = CursorShape(platform.CursorResizeLeft)
	CursorResizeRight       = CursorShape(platform.CursorResizeRight)
	CursorResizeTop         = CursorShape(platform.CursorResizeTop)
	CursorResizeBottom      = CursorShape(platform.CursorResizeBottom)
	CursorResizeTopLeft     = CursorShape(platform.CursorResizeTopLeft)
	CursorResizeTopRight    = CursorShape(platform.CursorResizeTopRight)
	CursorResizeBottomLeft  = CursorShape(platform.CursorResizeBottomLeft)
	CursorResizeBottomRight = CursorShape(platform.CursorResizeBottomRight)
)
