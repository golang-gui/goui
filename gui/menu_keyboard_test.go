package gui

import (
	"github.com/golang-gui/goui/platform/events"
	"testing"
)

func TestMenuKeyboardNavigationAndActivation(t *testing.T) {
	m := NewMenu()
	m.Append("disabled", nil).SetEnabled(false)
	first := m.Append("first", nil)
	m.AppendSeparator()
	m.Append("hidden", nil).SetVisible(false)
	last := m.Append("last", nil)
	var activated *MenuItem
	content := newMenuContent(m, 0, func(item *MenuItem) { activated = item })
	win := &window{}
	win.SetWidget(content)
	defer win.Destroy()
	press := func(key events.Key) {
		t.Helper()
		if err := win.DispatchEvent(shortcutPress(key, 0)); err != nil {
			t.Fatal(err)
		}
	}
	press(events.KeyArrowDown)
	if content.active != first {
		t.Fatal("did not skip disabled item")
	}
	press(events.KeyArrowDown)
	if content.active != last {
		t.Fatal("did not skip hidden/separator items")
	}
	press(events.KeyArrowDown)
	if content.active != first {
		t.Fatal("did not wrap")
	}
	press(events.KeyEnd)
	if content.active != last {
		t.Fatal("End")
	}
	press(events.KeyEnter)
	if activated != last {
		t.Fatal("Enter did not activate selected item")
	}
	last.SetEnabled(false)
	press(events.KeyArrowUp)
	if content.active != first {
		t.Fatal("disabled active item retained")
	}
	first.SetVisible(false)
	activated = nil
	press(events.KeySpace)
	if activated != nil || content.active != nil {
		t.Fatal("activated unavailable item")
	}
	first.SetVisible(true)
	press(events.KeyHome)
	press(events.KeySpace)
	if activated != first {
		t.Fatal("Home/Space")
	}
	row := newMenuItemRow(nil)
	row.bind(first)
	row.selected = true
	if !row.Snapshot().Selected {
		t.Fatal("keyboard highlight missing from snapshot")
	}
	row.bind(last)
	if row.Snapshot().Selected {
		t.Fatal("pooled row retained selection")
	}
}
