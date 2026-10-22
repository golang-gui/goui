package ui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	"slices"
	"testing"
)

// 使用已有普通 Container 协调替身验证绑定；通用协调仍由 ui 包测试负责。
type tableBindingModel struct {
	*gui.SliceListModel[string]
	connections, deliveries int
}

func (m *tableBindingModel) RowID(index int) string { return m.ItemAt(index) }
func (m *tableBindingModel) ConnectItems(fn func()) signal.Handle {
	m.connections++
	return m.SliceListModel.ConnectItems(func() { m.deliveries++; fn() })
}
func newTableBindingModel() *tableBindingModel {
	return &tableBindingModel{SliceListModel: gui.NewSliceListModel([]string{"a", "b", "c"})}
}
func tableBindingArrange(v *widgets.TableView) {
	size := geometry.Size{Width: 300, Height: 180}
	v.Measure(layout.Tight(size))
	v.Arrange(geometry.Rectangle{Size: size})
}

func TestTableViewTextInputFillsCell(t *testing.T) {
	m := newTableBindingModel()
	ctx := new(treeBindingContext)
	v := TableView[string](m, TableColumn("note", "Note", func(_ widgets.TableRow, text string) baseui.View {
		return baseui.TextInput().Text(text)
	}).Width(200))
	table := v.Mount(ctx).(*widgets.TableView)
	defer table.SetModel(nil)
	defer v.Unmount(ctx, table)
	v.Update(ctx, table)
	tableBindingArrange(table)
	check := func(width float32) {
		t.Helper()
		if len(ctx.widgets) != 3 {
			t.Fatalf("unexpected realized editors: %d", len(ctx.widgets))
		}
		for _, widget := range ctx.widgets {
			input := widget.(*gui.TextInput)
			want := geometry.Rect(0, 0, width, 32)
			if input.Rect() != want || input.Parent().Rect() != want {
				t.Fatalf("UI shell must not shrink the editor: input=%v shell=%v want=%v", input.Rect(), input.Parent().Rect(), want)
			}
			if info := input.Snapshot(); info.Role != gui.RoleTextInput || !info.Focusable || info.Text == "" {
				t.Fatal("cell editor lost its own input semantics")
			}
		}
	}
	check(200)
	table.Columns()[0].SetWidth(240)
	tableBindingArrange(table)
	check(240)
	v.Update(ctx, table)
	tableBindingArrange(table)
	check(200)
}

func TestTableViewColumnIdentityAndUncontrolledState(t *testing.T) {
	m := newTableBindingModel()
	ctx := new(treeBindingContext)
	column := func(title string) *TableColumnView[string] {
		return TableColumn("name", title, func(_ widgets.TableRow, text string) baseui.View { return baseui.Label(title + text) })
	}
	v := TableView[string](m, column("old:"))
	table := v.Mount(ctx).(*widgets.TableView)
	defer table.SetModel(nil)
	defer v.Unmount(ctx, table)
	v.Update(ctx, table)
	tableBindingArrange(table)
	c := table.Columns()[0]
	delegate := c.Delegate()
	c.SetWidth(220)
	table.SetSelection([]string{"b"})
	table.SetCurrent("b")
	v = TableView[string](m, column("new:"))
	v.Update(ctx, table)
	tableBindingArrange(table)
	if m.connections != 1 || table.Columns()[0] != c || c.Delegate() != delegate || c.Width() != 220 || table.Current() != "b" || !slices.Equal(table.Selection(), []string{"b"}) {
		t.Fatal("uncontrolled rebuild reset columns/model/state")
	}
	found := false
	for _, w := range ctx.widgets {
		if label, ok := w.(*gui.Label); ok && label.Text() == "new:b" {
			found = true
		}
	}
	if !found {
		t.Fatal("stable delegate used old builder")
	}
	callbacks := 0
	v = TableView[string](m, column("controlled").Width(120).Sortable(true)).Selection([]string{"c"}).Current("a").Sort("name", widgets.SortDescending).OnSelection(func([]string) { callbacks++ })
	v.Update(ctx, table)
	if callbacks != 0 || c.Width() != 120 || table.Current() != "a" || !slices.Equal(table.Selection(), []string{"c"}) {
		t.Fatal("controlled update feedback or wrong state")
	}
	id, order := table.Sort()
	if id != "name" || order != widgets.SortDescending {
		t.Fatal("confirmed sort declaration missing")
	}
	v = TableView[string](m, column("omitted").Sortable(true))
	v.Update(ctx, table)
	if c.Width() != 120 || table.Current() != "a" {
		t.Fatal("removing declaration did not release control")
	}
	m.Set(0, "new")
	if m.deliveries != 1 {
		t.Fatal("duplicate model subscription")
	}
}
func TestTableViewColumnRemovalReorderAndModelReplacement(t *testing.T) {
	m, next := newTableBindingModel(), newTableBindingModel()
	ctx := new(treeBindingContext)
	column := func(id string) *TableColumnView[string] {
		return TableColumn(id, id, func(_ widgets.TableRow, text string) baseui.View { return baseui.Label(text) })
	}
	v := TableView[string](m, column("one"), column("two"))
	table := v.Mount(ctx).(*widgets.TableView)
	defer table.SetModel(nil)
	defer v.Unmount(ctx, table)
	v.Update(ctx, table)
	tableBindingArrange(table)
	old := table.Columns()
	v = TableView[string](m, column("two"), column("one"))
	v.Update(ctx, table)
	tableBindingArrange(table)
	if table.Columns()[0] != old[1] || table.Columns()[1] != old[0] {
		t.Fatal("column reorder did not preserve ID identity")
	}
	if len(ctx.widgets) != 6 {
		t.Fatal("cell subtrees unexpectedly recreated/leaked")
	}
	v = TableView[string](m, column("one"))
	v.Update(ctx, table)
	tableBindingArrange(table)
	if len(ctx.widgets) != 3 || len(ctx.State().(*tableState[string]).columns) != 1 {
		t.Fatal("removed column retained declarative content/state")
	}
	table.SetCurrent("b")
	table.SetSelection([]string{"b"})
	v = TableView[string](next, column("one")).Current("c").Selection([]string{"c"})
	v.Update(ctx, table)
	if next.connections != 1 || table.Current() != "c" || !slices.Equal(table.Selection(), []string{"c"}) {
		t.Fatal("new model not installed before controlled state")
	}
	m.Set(0, "old")
	next.Set(0, "fresh")
	if m.deliveries != 0 || next.deliveries != 1 {
		t.Fatal("old subscription retained")
	}
	v = TableView[string](nil)
	v.Update(ctx, table)
	next.Set(1, "detached")
	if table.Model() != nil || table.Current() != "" || len(table.Columns()) != 0 || next.deliveries != 1 {
		t.Fatal("nil model/zero columns did not clear")
	}
}

type tableBindingHost struct{ widget, focus gui.Widget }

func (h *tableBindingHost) Widget() gui.Widget                 { return h.widget }
func (h *tableBindingHost) FocusedWidget() gui.Widget          { return h.focus }
func (h *tableBindingHost) SetFocusedWidget(w gui.Widget) bool { h.focus = w; return true }
func TestTableViewResizeUsesLatestCallbackAndDisconnects(t *testing.T) {
	m := newTableBindingModel()
	ctx := new(treeBindingContext)
	first, latest := 0, 0
	v := TableView[string](m, TableColumn[string]("name", "Name", nil).OnResize(func(float32) { first++ }))
	table := v.Mount(ctx).(*widgets.TableView)
	defer table.SetModel(nil)
	v.Update(ctx, table)
	tableBindingArrange(table)
	v = TableView[string](m, TableColumn[string]("name", "Name", nil).OnResize(func(float32) { latest++ }))
	v.Update(ctx, table)
	tableBindingArrange(table)
	host, d := &tableBindingHost{widget: table}, new(gui.EventDispatcher)
	dispatch := func(kind events.EventType, x float32) {
		buttons := events.PointerButtonLeftDown
		if kind == events.PointerUp {
			buttons = 0
		}
		if err := d.DispatchEvent(host, events.PointerEvent{EventType: kind, Position: geometry.Point{X: x, Y: 16}, Button: events.PointerButtonLeft, Buttons: buttons}); err != nil {
			t.Fatal(err)
		}
	}
	dispatch(events.PointerDown, 156)
	dispatch(events.PointerMove, 186)
	dispatch(events.PointerUp, 186)
	if first != 0 || latest != 1 || table.Columns()[0].Width() != 190 {
		t.Fatalf("resize latest callback: first=%d latest=%d width=%g", first, latest, table.Columns()[0].Width())
	}
	v = TableView[string](m, TableColumn[string]("name", "Name", nil).Width(200).OnResize(func(float32) { latest++ }))
	v.Update(ctx, table)
	if latest != 1 {
		t.Fatal("declarative width emitted user callback")
	}
	v.Unmount(ctx, table)
	tableBindingArrange(table)
	dispatch(events.PointerDown, 196)
	dispatch(events.PointerMove, 216)
	dispatch(events.PointerUp, 216)
	if latest != 1 {
		t.Fatal("unmounted binding retained user signal")
	}
}

type tableBindingValue struct {
	*tableBindingModel
	payload any
}

func TestTableViewValueModelsReinstallWithoutComparison(t *testing.T) {
	for _, payload := range []any{1, []int{1}, map[string]int{"a": 1}, func() {}} {
		m := tableBindingValue{newTableBindingModel(), payload}
		ctx := new(treeBindingContext)
		v := TableView[string](m, TableColumn[string]("name", "Name", nil))
		table := v.Mount(ctx).(*widgets.TableView)
		v.Update(ctx, table)
		table.SetCurrent("b")
		v.Update(ctx, table)
		if m.connections != 2 || table.Current() != "" {
			t.Fatal("value model guessed identity instead of reinstalling")
		}
		v.Unmount(ctx, table)
		table.SetModel(nil)
	}
}
