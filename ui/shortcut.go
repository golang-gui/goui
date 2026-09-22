package ui

import (
	"slices"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

// ShortcutView describes a binding, not a widget. Rebuild the descriptor to
// reflect state changes; its pointer is not a reconciliation identity.
// Constructing or rebuilding it never runs its callback. RequestUpdate remains
// explicit, as for other UI callbacks.
type ShortcutView struct {
	gesture    gui.KeyGesture
	enabled    bool
	repeat     bool
	onActivate func()
}

// Shortcut declares an enabled binding with no modifiers and no auto-repeat.
// Without an OnActivate callback it does not intercept key events.
func Shortcut(key Key) *ShortcutView {
	return &ShortcutView{gesture: gui.KeyGesture{Key: key}, enabled: true}
}

// Modifiers replaces the exact modifier combination; extra modifiers do not match.
func (s *ShortcutView) Modifiers(modifiers KeyModifiers) *ShortcutView {
	s.gesture.Modifiers = modifiers
	return s
}

func (s *ShortcutView) Enabled(enabled bool) *ShortcutView {
	s.enabled = enabled
	return s
}

func (s *ShortcutView) AutoRepeat(repeat bool) *ShortcutView {
	s.repeat = repeat
	return s
}

func (s *ShortcutView) OnActivate(fn func()) *ShortcutView {
	s.onActivate = fn
	return s
}

// Shortcuts declares bindings for the view's focused subtree (Bubble phase).
// It replaces the previous list. Nil entries are ignored; bindings reconcile
// by position among non-nil entries, not by descriptor pointer.
func (v *ViewBase[T]) Shortcuts(bindings ...*ShortcutView) *T {
	v.shortcuts = slices.Clone(bindings)
	return v.self()
}

// Shortcuts declares window bindings, after focused-widget handling.
// It replaces the previous list. Nil entries are ignored; bindings reconcile
// by position among non-nil entries, not by descriptor pointer.
func (v WindowView) Shortcuts(bindings ...*ShortcutView) WindowView {
	v.shortcuts = slices.Clone(bindings)
	return v
}

type shortcutEntry struct {
	shortcut *gui.Shortcut
	handle   signal.Handle
	callback func()
}

type shortcutBindings struct {
	controller *gui.ShortcutController
	entries    []*shortcutEntry
}

func (b *shortcutBindings) update(controller *gui.ShortcutController, views []*ShortcutView) {
	if b.controller != controller {
		b.clear()
		b.controller = controller
	}
	i := 0
	for _, v := range views {
		if v == nil {
			continue
		}
		if i == len(b.entries) {
			e := &shortcutEntry{shortcut: gui.NewShortcut(v.gesture)}
			e.handle = e.shortcut.ConnectActivate(func() {
				if e.callback != nil {
					e.callback()
				}
			})
			b.entries = append(b.entries, e)
			b.controller.AddShortcut(e.shortcut)
		}
		e := b.entries[i]
		e.callback = v.onActivate
		e.shortcut.SetGesture(v.gesture)
		e.shortcut.SetEnabled(v.enabled && v.onActivate != nil)
		e.shortcut.SetAutoRepeat(v.repeat)
		i++
	}
	for len(b.entries) > i {
		last := len(b.entries) - 1
		e := b.entries[last]
		e.handle.Disconnect()
		b.controller.RemoveShortcut(e.shortcut)
		e.callback = nil
		b.entries = b.entries[:last]
	}
}

func (b *shortcutBindings) clear() {
	for _, e := range b.entries {
		e.handle.Disconnect()
		b.controller.RemoveShortcut(e.shortcut)
		e.callback = nil
	}
	b.entries = nil
	b.controller = nil
}
