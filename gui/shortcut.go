package gui

import (
	"slices"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/events"
)

// KeyGesture is a single logical key and an exact modifier combination.
// Extra modifiers do not match. Location is intentionally not constrained.
type KeyGesture struct {
	Key       Key
	Modifiers KeyModifiers
}

// Matches compares a raw key event with this GUI declaration. It does not
// consume input or apply KeyDown/auto-repeat policy.
func (g KeyGesture) Matches(event events.KeyEvent) bool {
	key, keyOK := g.Key.Resolve()
	mods, modsOK := g.Modifiers.Resolve()
	if !keyOK || !modsOK || key != event.Key {
		return false
	}
	// A modifier's own key-down includes that modifier in the native mask.
	// Normalize it on both sides so KeyPrimary alone can match its press.
	var own events.Modifiers
	switch key {
	case events.KeyShift:
		own = events.ModifierShift
	case events.KeyControl:
		own = events.ModifierControl
	case events.KeyAlt:
		own = events.ModifierAlt
	case events.KeyOption:
		own = events.ModifierOption
	case events.KeyCommand:
		own = events.ModifierCommand
	case events.KeyWin:
		own = events.ModifierWin
	case events.KeySuper:
		own = events.ModifierSuper
	case events.KeyAltGraph:
		own = events.ModifierAltGraph
	}
	return event.Modifiers&^own == mods&^own
}

// Shortcut is a GUI-thread-owned binding. It may be registered with several
// controllers; each registration has independent scope and ordering.
type Shortcut struct {
	gesture    KeyGesture
	enabled    bool
	autoRepeat bool
	activate   signal.Signal1[func() bool]
}

func NewShortcut(gesture KeyGesture) *Shortcut { return &Shortcut{gesture: gesture, enabled: true} }
func (s *Shortcut) Gesture() KeyGesture        { return s.gesture }
func (s *Shortcut) SetGesture(g KeyGesture)    { s.gesture = g }
func (s *Shortcut) Enabled() bool              { return s.enabled }
func (s *Shortcut) SetEnabled(enabled bool)    { s.enabled = enabled }
func (s *Shortcut) AutoRepeat() bool           { return s.autoRepeat }
func (s *Shortcut) SetAutoRepeat(repeat bool)  { s.autoRepeat = repeat }
func (s *Shortcut) ConnectActivate(fn func()) signal.Handle {
	return s.activate.Connect(func(alive func() bool) {
		if alive() {
			fn()
		}
	})
}

// ShortcutController participates in normal event propagation. Bubble (the
// default) covers its widget's focused subtree, Target only that widget, and
// Capture explicitly precedes descendant handlers. A matching shortcut consumes
// the key even when auto-repeat is disabled, so repeats cannot fall through to
// a different binding. It does not intercept keys consumed by a native IME.
type ShortcutController struct {
	EventControllerBase
	shortcuts []*Shortcut
	destroyed bool
}

func NewShortcutController() *ShortcutController {
	return &ShortcutController{EventControllerBase: NewEventControllerBase(PhaseBubble)}
}

func (c *ShortcutController) AddShortcut(s *Shortcut) {
	if s != nil && !c.destroyed && !slices.Contains(c.shortcuts, s) {
		c.shortcuts = append(c.shortcuts, s)
	}
}
func (c *ShortcutController) RemoveShortcut(s *Shortcut) {
	c.shortcuts = slices.DeleteFunc(c.shortcuts, func(v *Shortcut) bool { return v == s })
}

// Clear removes registrations without destroying shortcuts or their signals.
func (c *ShortcutController) Clear() { c.shortcuts = nil }

func (c *ShortcutController) HandleEvent(ctx EventContext) {
	e, ok := ctx.Event().(events.KeyEvent)
	if !ok || e.EventType != events.KeyDown || c.destroyed {
		return
	}
	for _, s := range c.shortcuts {
		if !s.enabled || !s.gesture.Matches(e) {
			continue
		}
		ctx.StopPropagation()
		e.PreventDefault()
		if e.Repeat && !s.autoRepeat {
			return
		}
		s.activate.Emit(func() bool {
			if c.destroyed || !slices.Contains(c.shortcuts, s) {
				return false
			}
			if native, ok := ctx.(*eventContext); ok && native.alive != nil {
				return native.alive()
			}
			return true
		})
		return
	}
}
