package ui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

func TestSwitchReconcile(t *testing.T) {
	ctx := new(tabContext)
	first, latest := 0, 0
	v := Switch("Automatic").Checked(true).Padding(9).Animated(false).Enabled(false).OnChange(func(bool) { first++ })
	b := v.Mount(ctx).(*widgets.Switch)
	v.Update(ctx, b)
	child := b.Child()
	if !b.Checked() || b.Padding() != 9 || b.Animated() || b.Enabled() || child == nil || first != 0 {
		t.Fatal("initial declaration / silent update")
	}
	for i := 0; i < 4; i++ {
		v = Switch("Updated").OnChange(func(checked bool) {
			if !checked {
				t.Fatal("unexpected transition")
			}
			latest++
		})
		v.Update(ctx, b)
	}
	if b.Checked() || b.Padding() != 6 || !b.Animated() || !b.Enabled() || b.Child() != child {
		t.Fatal("default restoration / retained child")
	}
	b.Arrange(geometry.Rect(0, 0, 120, 32))
	h := &checkInputHost{widget: b}
	d := new(gui.EventDispatcher)
	key := func() {
		t.Helper()
		if err := d.DispatchEvent(h, events.KeyEvent{EventType: events.KeyDown, Key: events.KeySpace}); err != nil {
			t.Fatal(err)
		}
	}
	key()
	if first != 0 || latest != 1 || !b.Checked() {
		t.Fatal("stale / duplicated callback")
	}
	v.Unmount(ctx, b)
	key()
	if latest != 1 || b.Checked() {
		t.Fatal("unmount retained connection / took widget ownership")
	}
	if _, ok := any(b).(baseui.Bin); !ok {
		t.Fatal("content did not reuse Bin")
	}
}

func TestSwitchDeclarationRestoresExternalState(t *testing.T) {
	ctx := new(tabContext)
	v := Switch()
	b := v.Mount(ctx).(*widgets.Switch)
	v.Update(ctx, b)
	b.SetChecked(true)
	b.SetEnabled(false)
	b.SetAnimated(false)
	b.SetPadding(0)
	v.Update(ctx, b)
	if b.Checked() || !b.Enabled() || !b.Animated() || b.Padding() != 6 {
		t.Fatal("declaration retained external state")
	}
	v.Unmount(ctx, b)
}
