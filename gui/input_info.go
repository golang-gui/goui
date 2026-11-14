package gui

import (
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
)

// ClickInfo describes one phase of a mouse press. Position is widget-local DIP.
// Count is assigned on press and retained on release and successful click;
// Position, Buttons and Modifiers describe the phase being reported.
type ClickInfo struct {
	Position  geometry.Point
	Button    events.PointerButton
	Buttons   events.PointerButtons
	Modifiers events.Modifiers
	Count     int
}

// DragInfo describes a pointer drag in widget-local DIP. Begin reports the
// original press information; Update and End report the current input.
// Button identifies the button driving the drag, including during motion.
type DragInfo struct {
	Position  geometry.Point
	Button    events.PointerButton
	Buttons   events.PointerButtons
	Modifiers events.Modifiers
}

// WheelInfo carries a wheel input with a widget-local DIP position. Deltas
// retain their native Mode; converting Position does not scale the deltas.
type WheelInfo struct {
	Position  geometry.Point
	DeltaX    float32
	DeltaY    float32
	Mode      events.WheelDeltaMode
	Buttons   events.PointerButtons
	Modifiers events.Modifiers
}

// ContextMenuInfo describes a context-menu request. Position is widget-local
// DIP when HasPosition is true. Keyboard requests have no pointer position;
// the control chooses a caret, selection or other meaningful anchor.
type ContextMenuInfo struct {
	Position    geometry.Point
	HasPosition bool
	Modifiers   events.Modifiers
}

// clickSequence tracks consecutive presses, not gesture ownership or actions.
// Each controller owns its state and invalidates it when its input is canceled.
// Text selection additionally resets it when its model or selection unit changes.
type clickSequence struct {
	at       time.Time
	position geometry.Point
	button   events.PointerButton
	count    int
}

func (s *clickSequence) press(now time.Time, position geometry.Point, button events.PointerButton) int {
	count := 1
	if s.count > 0 && s.button == button && !s.at.IsZero() && now.Sub(s.at) >= 0 &&
		now.Sub(s.at) <= gestureClickInterval && !gestureMoved(position, s.position, gestureDefaultDistance) {
		count = s.count + 1
	}
	s.at, s.position, s.button, s.count = now, position, button, count
	return count
}

func (s *clickSequence) reset() { *s = clickSequence{} }

// inputNotification keeps external callback reentry from continuing an obsolete
// signal emission. The context is borrowed for synchronous dispatch only.
type inputNotification struct {
	ctx   EventContext
	valid func() bool
}

func newInputNotification(ctx EventContext, valid func() bool) inputNotification {
	var widget Widget
	var root Root
	var revision uint64
	if c, ok := ctx.(*eventContext); ok && c.current != nil {
		widget, root, revision = c.current, c.current.Root(), c.current.base().enabledRevision
	}
	return inputNotification{ctx: ctx, valid: func() bool {
		if valid != nil && !valid() {
			return false
		}
		if c, ok := ctx.(*eventContext); ok && c.alive != nil && !c.alive() {
			return false
		}
		return widget == nil || !widget.base().destroyed && widget.Root() == root &&
			widget.base().enabledRevision == revision && IsEnabled(widget) && visibleInTree(widget)
	}}
}
