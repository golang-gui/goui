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
	// Shared Enabled is applied by the coordinator, covered by ui's real-root
	// regression test; this double exercises only the control's own Update.
	ctx := new(tabContext)
	first, latest := 0, 0
	v := Switch("Automatic").Checked(true).Padding(9).Animated(false).OnChange(func(bool) { first++ })
	b := v.Mount(ctx).(*widgets.Switch)
	v.Update(ctx, b)
	child := b.Child()
	if !b.Checked() || b.Padding() != geometry.UniformInsets(9) || b.Animated() || !b.Enabled() || child == nil || first != 0 {
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
	if b.Checked() || b.Padding() != geometry.UniformInsets(6) || !b.Animated() || !b.Enabled() || b.Child() != child {
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
	b.SetAnimated(false)
	b.SetPadding(geometry.Insets{})
	v.Update(ctx, b)
	if b.Checked() || !b.Enabled() || !b.Animated() || b.Padding() != geometry.UniformInsets(6) {
		t.Fatal("declaration retained external state")
	}
	v.Unmount(ctx, b)
}

func TestSwitchPaddingsLastDeclarationWins(t *testing.T) {
	ctx := new(tabContext)
	v := Switch("Choice").Paddings(geometry.Insets{Left: 1, Top: 2, Right: 3, Bottom: 4})
	b := v.Mount(ctx).(*widgets.Switch)
	v.Update(ctx, b)
	if b.Padding() != (geometry.Insets{Left: 1, Top: 2, Right: 3, Bottom: 4}) {
		t.Fatalf("Paddings not applied: %+v", b.Padding())
	}

	v = Switch("Choice").Paddings(geometry.Insets{Left: 1, Top: 2, Right: 3, Bottom: 4}).Padding(9)
	v.Update(ctx, b)
	if b.Padding() != geometry.UniformInsets(9) {
		t.Fatalf("Padding after Paddings did not win: %+v", b.Padding())
	}

	v = Switch("Choice")
	v.Update(ctx, b)
	if b.Padding() != geometry.UniformInsets(6) {
		t.Fatalf("default not restored: %+v", b.Padding())
	}
}
