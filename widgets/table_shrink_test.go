package widgets

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
)

// 基础宽度是偏好值；不足时平分缺口，达到各自下限后由其余列继续承担。
func TestTableColumnShrinkAllocation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		viewport float32
		minimum  [3]float32
		want     [3]float32
	}{
		{"等量收缩", 240, [3]float32{40, 40, 40}, [3]float32{80, 80, 80}},
		{"下限重新分配", 180, [3]float32{90, 40, 40}, [3]float32{90, 45, 45}},
		{"固定首列", 200, [3]float32{100, 40, 40}, [3]float32{100, 50, 50}},
		{"全部达到下限", 100, [3]float32{80, 40, 40}, [3]float32{80, 40, 40}},
		{"零视口仍保留下限", 0, [3]float32{80, 40, 40}, [3]float32{80, 40, 40}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := NewTableView()
			columns := make([]*TableColumn, 3)
			resizes := 0
			for i := range columns {
				c := NewTableColumn(fmt.Sprint(i), "Column")
				c.SetWidth(100)
				c.SetMinWidth(tc.minimum[i])
				c.SetResizable(false) // 手动调整开关不能禁止自动分配。
				c.ConnectResize(func(float32) { resizes++ })
				columns[i] = c
			}
			v.SetColumns(columns...)
			v.setColumnViewportWidth(tc.viewport)
			x := float32(0)
			for i, c := range columns {
				if c.Width() != 100 || c.allocatedWidth != tc.want[i] || v.columnX[i] != x {
					t.Fatalf("列%d：base=%g actual=%g x=%g，预期100/%g/%g", i, c.Width(), c.allocatedWidth, v.columnX[i], tc.want[i], x)
				}
				x += tc.want[i]
			}
			if resizes != 0 || v.columnWidth != x {
				t.Fatalf("自动分配发出Resize或总宽错误：resizes=%d sum=%g", resizes, v.columnWidth)
			}
		})
	}
}

func TestTableColumnShrinkFractionalAndRestore(t *testing.T) {
	for count := 2; count <= 8; count++ {
		v := NewTableView()
		columns := make([]*TableColumn, count)
		for i := range columns {
			columns[i] = NewTableColumn(fmt.Sprint(i), "Column")
			columns[i].SetWidth(100)
			columns[i].SetMinWidth(20 + float32(i)*3.25)
		}
		v.SetColumns(columns...)
		minimum := float32(0)
		for _, c := range columns {
			minimum += c.MinWidth()
		}
		for step := 0; step <= 1000; step++ {
			width := minimum + (float32(count*100)-minimum)*float32(step)/1000
			v.setColumnViewportWidth(width)
			if v.columnWidth > width || width-v.columnWidth > .001 {
				t.Fatalf("%d列小数视口误溢出/过量收缩：viewport=%g actual=%g", count, width, v.columnWidth)
			}
			for _, c := range columns {
				if c.allocatedWidth < c.MinWidth() || c.allocatedWidth > c.Width() || math.IsNaN(float64(c.allocatedWidth)) {
					t.Fatalf("实际列宽越界：%g，范围[%g,%g]", c.allocatedWidth, c.MinWidth(), c.Width())
				}
			}
		}
		v.setColumnViewportWidth(2000)
		for _, c := range columns {
			if c.Width() != 100 || c.allocatedWidth != 100 {
				t.Fatal("空间恢复后未恢复基础宽度")
			}
		}
		columns[count-1].SetExpand(true)
		if v.columnWidth != 2000 {
			t.Fatal("收缩后未恢复伸展策略")
		}
	}
}

func TestTableColumnShrinkViewportAndResizeCancel(t *testing.T) {
	v, _, first, _ := tableFixture(100)
	defer v.SetModel(nil)
	other := NewTableColumn("other", "Other")
	v.SetColumns(first, other)
	var resizes []float32
	first.ConnectResize(func(width float32) { resizes = append(resizes, width) })
	tableArrange(v, 200, 180)
	if v.columnWidth != v.body.Rect().Width || v.scroll.Snapshot().MaxScrollX != 0 || first.allocatedWidth >= first.Width() || len(resizes) != 0 {
		t.Fatal("收缩没有使用扣除滚动条后的实际视口，或修改了用户宽度")
	}
	allocated := first.allocatedWidth
	d, host := new(gui.EventDispatcher), &tabInputHost{root: v}
	bounds := v.headers[first].Snapshot().Bounds
	x := bounds.X + bounds.Width - 4
	dispatchTabPointer(t, d, host, events.PointerDown, x, 16)
	dispatchTabPointer(t, d, host, events.PointerMove, x+20, 16)
	if first.Width() != allocated+20 {
		t.Fatal("手动拖动未从收缩后的实际边缘开始")
	}
	tableKey(t, d, host, events.KeyEscape, 0)
	tableArrange(v, 200, 180)
	if first.Width() != 160 || first.allocatedWidth != allocated || !slices.Equal(resizes, []float32{allocated + 20, 160}) {
		t.Fatal("取消未恢复基础宽度或发出额外Resize")
	}
	tableArrange(v, 60, 180)
	if first.allocatedWidth != 40 || other.allocatedWidth != 40 || v.scroll.Snapshot().MaxScrollX <= 0 {
		t.Fatal("达到下限仍不足时没有横向滚动")
	}
	tableArrange(v, 400, 180)
	if first.allocatedWidth != 160 || other.allocatedWidth != 160 || first.Width() != 160 {
		t.Fatal("放大后未恢复基础宽度")
	}
}
