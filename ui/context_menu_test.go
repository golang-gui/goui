package ui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
)

func TestContextMenuBindingReplacesCallbackAndRemovesController(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	first, second := 0, 0
	w := r.update(Label("menu").OnContextMenu(func(ctx gui.EventContext) { first++; ctx.StopPropagation() }))
	w.Arrange(geometry.Rect(0, 0, 100, 40))
	dispatcher, host := new(gui.EventDispatcher), newTestWindow()
	host.SetWidget(w)
	request := events.KeyEvent{EventType: events.KeyDown, Key: events.KeyF10, Modifiers: events.ModifierShift}
	w.SetFocusable(true)
	host.SetFocusedWidget(w)
	_ = dispatcher.DispatchEvent(host, request)
	w2 := r.update(Label("menu").OnContextMenu(func(ctx gui.EventContext) { second++; ctx.StopPropagation() }))
	_ = dispatcher.DispatchEvent(host, request)
	if w2 != w || first != 1 || second != 1 {
		t.Fatal(first, second)
	}
	r.update(Label("menu"))
	_ = dispatcher.DispatchEvent(host, request)
	if first != 1 || second != 1 {
		t.Fatal("removed callback remains connected")
	}
	for _, c := range w.EventControllers() {
		if _, ok := c.(*gui.ContextMenuEventController); ok {
			t.Fatal("removed controller remains installed")
		}
	}
}

func TestLabelWrapModeBindingRestoresDefault(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	w := r.update(Label("wrapped").WrapMode(gui.WrapWordChar)).(*gui.Label)
	if w.WrapMode() != gui.WrapWordChar {
		t.Fatal(w.WrapMode())
	}
	r.update(Label("plain"))
	if w.WrapMode() != gui.WrapNone {
		t.Fatal("omitted WrapMode did not restore default")
	}
}
