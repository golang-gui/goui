package ui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/widgets"
)

// 声明式重建不能把自动收缩后的实际宽度写回基础配置或发出用户调整通知。
func TestTableViewShrinkBindingPreservesUserWidth(t *testing.T) {
	m := newTableBindingModel()
	ctx := new(treeBindingContext)
	resizes := 0
	build := func() *TableViewView[string] {
		return TableView[string](m, TableColumn[string]("name", "Name", nil).MinWidth(80).OnResize(func(float32) { resizes++ }))
	}
	v := build()
	table := v.Mount(ctx).(*widgets.TableView)
	defer table.SetModel(nil)
	defer v.Unmount(ctx, table)
	v.Update(ctx, table)
	c := table.Columns()[0]
	c.SetWidth(200)
	for _, width := range []float32{120, 40, 320, 120, 320} {
		v = build()
		v.Update(ctx, table)
		size := geometry.Size{Width: width, Height: 180}
		table.Measure(layout.Tight(size))
		table.Arrange(geometry.Rectangle{Size: size})
		allocated := table.Snapshot().Children[0].Bounds.Width
		if c.Width() != 200 || table.Columns()[0] != c || resizes != 0 || m.connections != 1 {
			t.Fatal("自动收缩/重建改变基础宽度、列身份、用户信号或订阅")
		}
		if width == 40 && allocated != 80 || width == 120 && (allocated < 80 || allocated >= 120) || width == 320 && allocated != 200 {
			t.Fatalf("视口%g时实际列宽%g未遵守下限/恢复规则", width, allocated)
		}
	}
}
