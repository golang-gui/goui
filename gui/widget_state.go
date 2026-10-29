package gui

import (
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
)

const widgetOwnedStates = style.Pressed | style.Selected | style.Checked | style.Mixed | style.DropActive

// State returns the widget's simultaneous effective states. Hovered includes
// descendants. Disabled and FocusVisible are derived from the existing enabled
// and host-focus facts, rather than separately writable copies of them.
func (w *WidgetBase) State() style.State {
	state := w.state
	if !w.isEnabled() {
		return (state | style.Disabled) &^ (style.Hovered | style.Pressed | style.Focused | style.ContainsFocus | style.FocusVisible | style.DropActive)
	}
	if w.FocusVisible() {
		state |= style.FocusVisible
	}
	return state
}

// SetState updates implementation-owned display flags. It does not activate,
// select or check a business model. Use a control's dedicated setter for that.
// Only Pressed, Selected, Checked, Mixed and DropActive are writable here;
// passing host-owned or unknown flags panics. Normal is a no-op. Like Paint,
// this helper is for widget implementations, on the GUI thread.
func (w *WidgetBase) SetState(flags style.State, enabled bool) {
	if flags&^widgetOwnedStates != 0 {
		panic("gui: widget state contains reserved or unknown flags")
	}
	if w.destroyed {
		return
	}
	if !w.isEnabled() {
		flags &^= style.Pressed | style.DropActive
	}
	w.updateState(flags, enabled)
}

func (w *WidgetBase) updateState(flags style.State, enabled bool) {
	next := w.state &^ flags
	if enabled {
		next |= flags
	}
	if next == w.state {
		return
	}
	w.state = next
	w.invalidateState()
}

// Retain the last measurement until the next frame can compare the new style.
// Pointer crossing must not mark otherwise valid chrome geometry dirty mid-event.
func (w *WidgetBase) invalidateState() {
	w.styleValid = false
	if w.unmounting {
		w.measureValid = false
		return
	}
	if owner, ok := w.root().(interface{ rootState() *rootBase }); ok && w.self != nil {
		host := owner.rootState()
		if host.stateStyles == nil {
			host.stateStyles = make(map[Widget]stateMeasurement)
		}
		if _, pending := host.stateStyles[w.self]; !pending {
			host.stateStyles[w.self] = stateMeasurement{w.measureConstraint, w.measureResult, w.measureValid}
		}
		w.RequestPaint()
	} else {
		w.RequestLayout()
	}
	w.requestSemanticUpdate()
}

type stateMeasurement struct {
	constraint layout.Constraint
	result     layout.Measurement
	valid      bool
}

func (b *rootBase) updateStateStyles() {
	pending := b.stateStyles
	b.stateStyles = nil // callbacks can schedule a later notification
	for widget, before := range pending {
		if b.frameDestroyed {
			return
		}
		root := widget.Root()
		owner, ok := root.(interface{ rootState() *rootBase })
		if widget.base().destroyed || !ok || owner.rootState() != b {
			continue
		}
		if !ensureWidgetStyle(widget) || widget.Root() != root {
			continue
		}
		if !b.layoutDirty && before.valid {
			after := measureWidget(widget, before.constraint)
			if widget.base().destroyed || widget.Root() != root {
				continue
			}
			if after != before.result {
				widget.RequestLayout()
			}
		}
	}
}
