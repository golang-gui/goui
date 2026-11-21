package gui

import (
	"slices"

	"github.com/golang-gui/goui/core/signal"
)

// Command is a shared, GUI-thread-owned operation with availability and optional
// boolean state. It has no visual presentation or registration scope. Keep it at
// a stable address after first use.
type Command struct {
	disabled          bool
	revision          uint64
	enabled           signal.Signal2[bool, uint64]
	activate          signal.Signal1[uint64]
	shortcuts         []*Shortcut
	shortcutsChanged  signal.Signal0
	checkable         bool
	checked           bool
	checkableRevision uint64
	checkedRevision   uint64
	checkableChanged  signal.Signal2[bool, uint64]
	checkedChanged    signal.Signal2[bool, uint64]
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

// Checkable reports whether the operation publishes a boolean state.
func (c *Command) Checkable() bool { return c.checkable }

// SetCheckable changes the state capability without activating the operation.
// Disabling it clears Checked before either notification is delivered.
func (c *Command) SetCheckable(checkable bool) {
	if c.checkable == checkable {
		return
	}
	wasChecked := c.checked
	c.checkable = checkable
	if !checkable {
		c.checked = false
	}
	c.checkableRevision++
	cleared := wasChecked && !checkable
	if cleared {
		c.checkedRevision++
	}
	revision, checkedRevision := c.checkableRevision, c.checkedRevision
	c.checkableChanged.Emit(checkable, revision)
	if cleared && c.checkedRevision == checkedRevision {
		c.checkedChanged.Emit(false, checkedRevision)
	}
}

// ConnectCheckable observes the committed capability. A newer capability change
// during a callback cancels the remaining notifications of the older capability.
func (c *Command) ConnectCheckable(fn func(bool)) signal.Handle {
	return c.checkableChanged.Connect(func(value bool, revision uint64) {
		if c.checkableRevision == revision {
			fn(value)
		}
	})
}

func (c *Command) Checked() bool { return c.checked }

// SetChecked publishes actual state, including while the operation is disabled.
// It has no effect until Checkable is true and never activates the operation.
func (c *Command) SetChecked(checked bool) {
	if !c.checkable || c.checked == checked {
		return
	}
	c.checked = checked
	c.checkedRevision++
	c.checkedChanged.Emit(checked, c.checkedRevision)
}

// ConnectChecked observes committed values from every publication, not input
// requests. State publication does not invalidate an ongoing activation.
func (c *Command) ConnectChecked(fn func(bool)) signal.Handle {
	return c.checkedChanged.Connect(func(value bool, revision uint64) {
		if c.checkedRevision == revision {
			fn(value)
		}
	})
}

// Activate invokes the operation if enabled; it never changes Checked. The
// operation's handler commits actual state with SetChecked. Disabling it during
// notification cancels the remaining handlers of that activation, even if reenabled.
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
