package ui

import (
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/widgets"
)

// 受控OnResize写回及普通重建都保留手调实际分配，不另建声明式调整状态机。
func TestTableViewControlledResizeKeepsAllocation(t *testing.T) {
	m, ctx := newTableBindingModel(), new(treeBindingContext)
	widths := map[string]float32{"name": 200, "size": 200, "description": 200}
	notifications := map[string]int{}
	build := func() *TableViewView[string] {
		columns := make([]*TableColumnView[string], 0, 3)
		for _, id := range []string{"name", "size", "description"} {
			columns = append(columns, TableColumn[string](id, id, nil).Width(widths[id]).OnResize(func(w float32) { widths[id] = w; notifications[id]++ }))
		}
		return TableView[string](m, columns...)
	}
	v := build()
	table := v.Mount(ctx).(*widgets.TableView)
	defer table.SetModel(nil)
	defer v.Unmount(ctx, table)
	v.Update(ctx, table)
	arrange := func() {
		size := geometry.Size{Width: 320, Height: 180}
		table.Measure(layout.Tight(size))
		table.Arrange(geometry.Rectangle{Size: size})
	}
	arrange()
	columns := table.Columns()
	host, d := &tableBindingHost{widget: table}, new(gui.EventDispatcher)
	for _, index := range []int{1, 2} {
		before := table.Snapshot()
		bounds := before.Children[index].Bounds
		x := bounds.X + bounds.Width - 4
		for _, step := range []struct {
			kind  events.EventType
			delta float32
		}{{events.PointerDown, 0}, {events.PointerMove, 12}, {events.PointerUp, 12}} {
			buttons := events.PointerButtonLeftDown
			if step.kind == events.PointerUp {
				buttons = 0
			}
			if err := d.DispatchEvent(host, events.PointerEvent{EventType: step.kind, Position: geometry.Point{X: x + step.delta, Y: 16}, Button: events.PointerButtonLeft, Buttons: buttons}); err != nil {
				t.Fatal(err)
			}
			v = build()
			v.Update(ctx, table)
			arrange()
		}
		after := table.Snapshot()
		for i := 0; i <= index; i++ {
			want := before.Children[i].Bounds.Width
			if i == index {
				want += 12
			}
			if math.Abs(float64(after.Children[i].Bounds.Width-want)) > .001 || table.Columns()[i] != columns[i] {
				t.Fatalf("受控重建改变列%d身份或分配：%g，预期%g", i, after.Children[i].Bounds.Width, want)
			}
		}
	}
	if notifications["name"] != 0 || notifications["size"] != 1 || notifications["description"] != 1 || m.connections != 1 {
		t.Fatalf("额外通知或订阅：%v/%d", notifications, m.connections)
	}
}
