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
	commandID  string
	commandRef *gui.Command
	definition *CommandView
}

// Shortcut declares an enabled binding with no modifiers and no auto-repeat.
// Without an OnActivate callback or a resolved Command it does not intercept key events.
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
	s.commandID = ""
	s.commandRef = nil
	s.definition = nil
	return s
}

// Command selects an operation from the nearest ancestor command scope.
// It replaces OnActivate; an unresolved reference does not consume input.
func (s *ShortcutView) Command(id string) *ShortcutView {
	s.commandID = id
	s.commandRef = nil
	s.definition = nil
	s.onActivate = nil
	return s
}

// CommandRef borrows an external operation without requiring a CommandGroup.
// It replaces Command and OnActivate. Unmount only detaches this binding;
// the caller retains ownership of the command and its activation handlers.
func (s *ShortcutView) CommandRef(command *gui.Command) *ShortcutView {
	s.commandRef, s.commandID, s.definition, s.onActivate = command, "", nil, nil
	return s
}

// DefineCommand supplies a scope-local definition and uses it for this binding.
// Views use their nearest CommandGroup; Window.Shortcuts uses the window's
// default group, never a descendant group in Content.
// Binding Enabled remains independent of the command's availability.
// nil clears the operation.
func (s *ShortcutView) DefineCommand(command *CommandView) *ShortcutView {
	s.definition, s.commandID, s.commandRef, s.onActivate = command, "", nil, nil
	if command != nil {
		s.commandID = command.id
	}
	return s
}

// Shortcuts declares bindings for the view's focused subtree (Bubble phase).
// It replaces the previous list. Nil entries are ignored; bindings reconcile
// by position among non-nil entries, not by descriptor pointer.
func (v *ViewBase[T]) Shortcuts(bindings ...*ShortcutView) *T {
	v.shortcuts = slices.Clone(bindings)
	v.fields.Set(viewShortcuts, true)
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
	command  commandReference
}

type shortcutBindings struct {
	controller  *gui.ShortcutController
	entries     []*shortcutEntry
	version     uint64
	definitions commandDefinitions
}

func (b *shortcutBindings) update(controller *gui.ShortcutController, views []*ShortcutView, contexts ...BuildContext) {
	var ctx BuildContext
	if len(contexts) > 0 {
		ctx = contexts[0]
	}
	if b.controller != controller {
		previous := b.version
		b.clear()
		if b.version != previous+1 {
			return
		}
		b.controller = controller
	}
	b.version++
	version := b.version
	current := func() bool { return b.version == version && b.controller == controller }
	var definitions []*CommandView
	for _, view := range views {
		if view != nil && view.definition != nil {
			definitions = append(definitions, view.definition)
		}
	}
	b.definitions.update(ctx, nil, definitions, "shortcut-command:")
	if !current() {
		return
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
		if !current() {
			return
		}
		e.shortcut.SetEnabled(v.enabled && (v.onActivate != nil || v.commandID != "" || v.commandRef != nil))
		if v.commandRef != nil {
			e.command.clear()
			e.shortcut.SetCommand(v.commandRef)
		} else {
			e.command.update(ctx, v.commandID, e.shortcut.SetCommand)
		}
		if !current() {
			return
		}
		e.shortcut.SetAutoRepeat(v.repeat)
		i++
	}
	for len(b.entries) > i {
		last := len(b.entries) - 1
		e := b.entries[last]
		b.entries = b.entries[:last]
		e.handle.Disconnect()
		e.command.clear()
		controller.RemoveShortcut(e.shortcut)
		e.callback = nil
		e.shortcut.SetCommand(nil)
		if !current() {
			return
		}
	}
}

func (b *shortcutBindings) clear() {
	// Publish detachment before external command notifications can release or
	// rebuild this owner. Cleanup must not overwrite a newer binding list.
	entries, controller := b.entries, b.controller
	b.version++
	b.entries, b.controller = nil, nil
	b.definitions.clear()
	for _, e := range entries {
		e.handle.Disconnect()
		e.command.clear()
		controller.RemoveShortcut(e.shortcut)
		e.callback = nil
		e.shortcut.SetCommand(nil)
	}
}

func (b *shortcutBindings) reparented() {
	b.definitions.reparented()
	for _, entry := range b.entries {
		entry.command.reparent()
	}
}
