package common

// CursorShape is a named standard cursor. Backends use native cursor resources;
// unsupported shapes fall back to the arrow (notably diagonal resize on macOS
// before the public frame-resize cursor API).
type CursorShape uint8

const (
	// CursorDefault is the arrow. It also doubles as "unset": a widget that has
	// not chosen a cursor reports CursorDefault, meaning "do not override".
	CursorDefault CursorShape = iota
	// CursorText is the I-beam used over editable text.
	CursorText
	// CursorPointing is the pointing-finger hand shown over clickable things.
	// Named for the gesture ("pointing"), not "Hand", which reads as the
	// open/closed drag hand, nor "Pointer", which is a synonym for cursor.
	CursorPointing
	// CursorCrosshair is the precision crosshair.
	CursorCrosshair
	// CursorForbidden marks an action that cannot be performed.
	CursorForbidden
	// CursorNone makes the pointer invisible while still tracking it (unlike
	// pointer capture/lock, which is a separate, deferred feature). It is a shape
	// value — "the pointer looks like nothing here" — so it flows through the
	// same resolution as every other shape.
	CursorNone
	// Axis resize cursors do not identify an active edge (for example, a
	// splitter). Window borders should use the edge shapes below.
	CursorResizeHorizontal
	CursorResizeVertical
	// CursorResizeNWSE is the top-left / bottom-right diagonal.
	CursorResizeNWSE
	// CursorResizeNESW is the top-right / bottom-left diagonal.
	CursorResizeNESW
	// Edge resize semantics preserve the active edge. A platform/theme may
	// share images between opposite edges; callers must not collapse them.
	CursorResizeLeft
	CursorResizeRight
	CursorResizeTop
	CursorResizeBottom
	CursorResizeTopLeft
	CursorResizeTopRight
	CursorResizeBottomLeft
	CursorResizeBottomRight
)

// Cursor is a window's mouse-cursor capability — the object that sets the
// window's current cursor, not a single cursor. Like Window/Popup/InputMethod/
// Painter it is a capability noun. Obtain one via Platform.NewCursor; not every
// platform or window supports it (the factory returns an error when it does
// not). It is thread-affine like the window it belongs to.
type Cursor interface {
	// SetShape sets the window's current cursor to the given standard shape.
	SetShape(shape CursorShape)
	// Destroy releases any native cursor resources held by this capability.
	Destroy()
}
