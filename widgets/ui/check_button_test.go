package ui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

type checkInputHost struct{ widget gui.Widget }

func (h *checkInputHost) Widget() gui.Widget               { return h.widget }
func (h *checkInputHost) FocusedWidget() gui.Widget        { return h.widget }
func (h *checkInputHost) SetFocusedWidget(gui.Widget) bool { return true }

func checkKey(t *testing.T, b *widgets.CheckButton) {
	t.Helper()
	if err := new(gui.EventDispatcher).DispatchEvent(&checkInputHost{widget: b}, events.KeyEvent{EventType: events.KeyDown, Key: events.KeySpace}); err != nil {
		t.Fatal(err)
	}
}

func TestCheckButtonReconcileAndSignals(t *testing.T) {
	// This adapter double calls Update directly. Shared modifiers, including
	// Enabled, are covered by ui.TestViewEnabledReconcilesOwnSettingsAndSubtrees
	// through the actual coordinator rather than bypassed here.
	ctx := new(tabContext)
	g := widgets.NewCheckGroup()
	first, latest := 0, 0
	v := CheckButton("Choice").Checked(true).Group(g).Appearance(CheckAppearanceButton).Padding(9).OnChange(func(CheckState) { first++ })
	b := v.Mount(ctx).(*widgets.CheckButton)
	v.Update(ctx, b)
	child := b.Child()
	if !b.Checked() || b.Group() != g || b.Appearance() != CheckAppearanceButton || b.Padding() != 9 || !b.Enabled() || child == nil || first != 0 {
		t.Fatal("initial declaration / silent synchronization failed")
	}
	for i := 0; i < 4; i++ {
		v = CheckButton("Updated").CheckState(CheckMixed).OnChange(func(s CheckState) {
			if s != CheckChecked {
				t.Fatal("unexpected user transition")
			}
			latest++
		})
		v.Update(ctx, b)
	}
	if b.Group() != nil || b.Appearance() != CheckAppearanceIndicator || b.Padding() != 6 || !b.Enabled() || b.CheckState() != CheckMixed || b.Child() != child {
		t.Fatal("removed declaration / retained identity failed")
	}
	b.Arrange(geometry.Rect(0, 0, 120, 32))
	checkKey(t, b)
	if first != 0 || latest != 1 || !b.Checked() {
		t.Fatal("rebuilt callback stale / duplicated")
	}
	v.Unmount(ctx, b)
	checkKey(t, b)
	if latest != 1 || b.Checked() {
		t.Fatal("unmount retained UI signal / destroyed GUI widget")
	}
}

func TestCheckButtonDeclarationRestoresExternalState(t *testing.T) {
	ctx := new(tabContext)
	v := CheckButton()
	b := v.Mount(ctx).(*widgets.CheckButton)
	v.Update(ctx, b)
	b.SetChecked(true)
	b.SetGroup(widgets.NewCheckGroup())
	b.SetAppearance(CheckAppearanceButton)
	b.SetPadding(0)
	v.Update(ctx, b)
	if b.Checked() || b.Group() != nil || b.Appearance() != CheckAppearanceIndicator || b.Padding() != 6 || !b.Enabled() {
		t.Fatal("declaration failed to restore externally changed GUI state")
	}
	if _, ok := any(b).(baseui.Bin); !ok {
		t.Fatal("not adapted through existing Bin")
	}
	v.Unmount(ctx, b)
}
