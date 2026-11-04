package gui

import (
	"slices"

	"github.com/golang-gui/goui/core/signal"
)

// Command is a shared, GUI-thread-owned operation. It has no presentation or
// registration scope. Keep it at a stable address after first use.
type Command struct {
	disabled         bool
	revision         uint64
	enabled          signal.Signal2[bool, uint64]
	activate         signal.Signal1[uint64]
	shortcuts        []*Shortcut
	shortcutsChanged signal.Signal0
}

// NewCommand creates an enabled operation without shortcuts or handlers.
func NewCommand() *Command       { return &Command{} }
func (c *Command) Enabled() bool { return !c.disabled }

// SetEnabled changes operation availability, independently of its entry points.
func (c *Command) SetEnabled(enabled bool) {
	if enabled == c.Enabled() {
		return
	}
	c.disabled = !enabled
	c.revision++
	c.enabled.Emit(enabled, c.revision)
}

func (c *Command) ConnectEnabled(fn func(bool)) signal.Handle {
	return c.enabled.Connect(func(enabled bool, revision uint64) {
		if c.revision == revision {
			fn(enabled)
		}
	})
}

// Activate invokes the operation if enabled. Disabling it during notification
// cancels the remaining handlers of that activation, even if reenabled.
func (c *Command) Activate() {
	if c.Enabled() {
		c.activate.Emit(c.revision)
	}
}

func (c *Command) ConnectActivate(fn func()) signal.Handle {
	return c.activate.Connect(func(revision uint64) {
		if c.Enabled() && c.revision == revision {
			fn()
		}
	})
}

// Shortcuts returns a detached list in association order, not active bindings.
// Controller registration, focus and local enablement do not change this list.
func (c *Command) Shortcuts() []*Shortcut { return slices.Clone(c.shortcuts) }

// ConnectShortcuts observes associations and their gestures. Query Shortcuts
// again from the callback; it receives the committed, current configuration.
func (c *Command) ConnectShortcuts(fn func()) signal.Handle {
	return c.shortcutsChanged.Connect(fn)
}

func (c *Command) shortcutText() string {
	for _, shortcut := range c.shortcuts {
		if text := shortcut.Gesture().DisplayText(); text != "" {
			return text
		}
	}
	return ""
}
