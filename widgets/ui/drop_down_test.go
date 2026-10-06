package ui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

type dropDownBindingModel struct {
	*gui.SliceListModel[DropDownItem]
	connections int
}

func (m *dropDownBindingModel) ConnectItems(fn func()) signal.Handle {
	m.connections++
	return m.SliceListModel.ConnectItems(fn)
}

func TestDropDownDeclarationAndLatestSignals(t *testing.T) {
	m := &dropDownBindingModel{SliceListModel: gui.NewSliceListModel([]DropDownItem{{Text: "A"}, {Text: "B", Disabled: true}, {Text: "C"}})}
	ctx := new(treeBindingContext)
	first, latest, errorsSeen := 0, 0, 0
	v := DropDown(m).Selected(0).Placeholder("choose").Padding(9).PopupMaxHeight(180).OnSelected(func(int) { first++ })
	d := v.Mount(ctx).(*widgets.DropDown)
	defer d.SetModel(nil)
	v.Update(ctx, d)
	d.Arrange(geometry.Rect(0, 0, 180, 32))
	if d.Selected() != 0 || d.Padding() != 9 || d.PopupMaxHeight() != 180 || d.Placeholder() != "choose" || first != 0 {
		t.Fatal("initial declaration")
	}
	d.SetSelected(2)
	for range 4 {
		v = DropDown(m).OnSelected(func(int) { latest++ }).OnOpenError(func(error) { errorsSeen++ })
		v.Update(ctx, d)
	}
	if d.Selected() != 2 || m.connections != 1 || d.Padding() != 6 || d.PopupMaxHeight() != 320 || d.Placeholder() != "" {
		t.Fatal("rebuild reset state/model or retained optional overrides")
	}
	h := &checkInputHost{widget: d}
	dispatch := new(gui.EventDispatcher)
	key := func(key events.Key) {
		t.Helper()
		if err := dispatch.DispatchEvent(h, events.KeyEvent{EventType: events.KeyDown, Key: key}); err != nil {
			t.Fatal(err)
		}
	}
	key(events.KeyArrowUp)
	if d.Selected() != 0 || first != 0 || latest != 1 {
		t.Fatal("stale/duplicate selected callback")
	}
	key(events.KeyEnter)
	if errorsSeen != 1 {
		t.Fatal("input error not delivered to latest handler")
	}
	// Shared Enabled coordination is tested with the real root in ui; this
	// adapter double only invokes the control-specific Update.
	v = DropDown(m).Selected(-1).Padding(0).PopupMaxHeight(0)
	v.Update(ctx, d)
	if d.Selected() != -1 || !d.Enabled() || !d.Focusable() || d.Padding() != 0 || d.PopupMaxHeight() != 0 {
		t.Fatal("explicit zero not applied")
	}
	v = DropDown(m).Selected(0).OnSelected(func(int) { latest++ })
	v.Update(ctx, d)
	v.Unmount(ctx, d)
	key(events.KeyArrowDown)
	if d.Selected() != 2 || latest != 1 {
		t.Fatal("unmount retained notification or took GUI ownership")
	}
}

func TestDropDownRealContentAndSelectedOverride(t *testing.T) {
	m := baseui.SliceList([]DropDownItem{{Text: "A"}, {Text: "B"}})
	ctx := new(treeBindingContext)
	item := func(_ int, item DropDownItem) baseui.View { return baseui.Label("row:" + item.Text) }
	v := DropDown(m).Selected(0).Item(item).SelectedItem(func(_ int, item DropDownItem) baseui.View { return baseui.Label("selected:" + item.Text) })
	d := v.Mount(ctx).(*widgets.DropDown)
	defer d.SetModel(nil)
	defer v.Unmount(ctx, d)
	v.Update(ctx, d)
	var display *gui.Label
	for _, widget := range ctx.widgets {
		display = widget.(*gui.Label)
	}
	if display == nil || display.Text() != "selected:A" {
		t.Fatal("selected builder not coordinated")
	}
	row := d.Delegate().Setup()
	d.Delegate().Bind(0, row)
	rowLabel := row.Children()[0].(*gui.Label)
	if rowLabel == display || rowLabel.Text() != "row:A" || rowLabel.Parent() != row {
		t.Fatal("row/display shared widget or lost subtree")
	}
	v = DropDown(m).Item(func(_ int, item DropDownItem) baseui.View { return baseui.Label("new:" + item.Text) })
	v.Update(ctx, d)
	d.Delegate().Bind(0, row)
	if row.Children()[0] != rowLabel || rowLabel.Text() != "new:A" || display.Text() != "new:A" {
		t.Fatal("refresh used stale builder or replaced retained labels")
	}
	d.Delegate().Unbind(0, row)
	if len(row.Children()) != 0 || display.Parent() == nil {
		t.Fatal("row unbind cleared collapsed content")
	}
	d.Measure(layout.Loose(geometry.Size{Width: 180, Height: 32}))
	v = DropDown(m).Selected(1)
	v.Update(ctx, d)
	if d.Selected() != 1 {
		t.Fatal("explicit index did not update")
	}
	m2 := baseui.SliceList([]DropDownItem{{Text: "new model"}})
	v = DropDown(m2)
	v.Update(ctx, d)
	if d.Selected() != -1 {
		t.Fatal("changed model retained old selection")
	}
}
