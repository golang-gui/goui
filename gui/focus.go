package gui

import (
	"slices"

	"github.com/golang-gui/goui/platform/events"
)

// navigateTab is the default key fallback, after controllers and shortcuts.
// Rebuild the order from the live tree on each key: tree changes need no
// separate focus chain or invalidation protocol.
func (d *EventDispatcher) navigateTab(host EventTarget, event events.Event) {
	key, ok := event.(events.KeyEvent)
	if !ok || key.EventType != events.KeyDown || key.Key != events.KeyTab ||
		(key.Modifiers != 0 && key.Modifiers != events.ModifierShift) ||
		(key.Handled != nil && *key.Handled) {
		return
	}

	var candidates []Widget
	var visit func(Widget)
	visit = func(widget Widget) {
		if widget == nil || widget.base().destroyed || !widget.Visible() || widget.Root() == nil {
			return
		}
		if widget.Focusable() {
			candidates = append(candidates, widget)
		}
		for _, child := range widget.Children() {
			visit(child)
		}
	}
	visit(host.Widget())
	visit(d.decoration)
	if len(candidates) == 0 {
		return
	}

	current := host.FocusedWidget()
	index := slices.Index(candidates, current)
	if key.Modifiers == events.ModifierShift {
		if index < 0 {
			index = 0
		}
		index = (index + len(candidates) - 1) % len(candidates)
	} else {
		index = (index + 1) % len(candidates)
	}
	next := candidates[index]
	if next != current && !host.SetFocusedWidget(next) {
		return
	}
	key.PreventDefault()
	// Focus notifications may remove the target or deliberately redirect focus.
	if host.FocusedWidget() == next && !next.base().destroyed && next.Root() != nil {
		revealFocusedWidget(host, next)
	}
}

// Reveal inner viewports first so outer ones use the final target geometry.
// This only visits the current tree; virtual records are not materialized.
func revealFocusedWidget(host EventTarget, target Widget) {
	for parent := target.Parent(); parent != nil; parent = parent.Parent() {
		if host.FocusedWidget() != target || target.base().destroyed || target.Root() == nil {
			return
		}
		scroll, ok := parent.(*ScrollView)
		if !ok || scroll.content == nil || !target.base().isDescendant(target, scroll.content) {
			continue
		}
		rect, origin := target.base().windowRect(), scroll.content.base().windowRect().Pos
		rect.X -= origin.X
		rect.Y -= origin.Y
		if _, virtual := scroll.content.(Scrollable); virtual {
			// Scrollable places visible children inside a fixed viewport-sized
			// content widget; its origin does not include the scrolling offset.
			rect.X += scroll.ScrollX()
			rect.Y += scroll.ScrollY()
		}
		scroll.ScrollIntoView(rect)
	}
}
