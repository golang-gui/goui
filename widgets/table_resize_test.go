package widgets

import (
	"math"
	"slices"
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
)

func tableActualWidths(v *TableView) []float32 {
	widths := make([]float32, len(v.visibleColumns))
	for i, c := range v.visibleColumns {
		widths[i] = c.allocatedWidth
	}
	return widths
}

func tableCheckWidths(t *testing.T, v *TableView, want []float32) {
	t.Helper()
	// float32小数边界累计允许0.001 DIP误差，2x下仍小于0.002物理像素。
	for i, width := range tableActualWidths(v) {
		if math.Abs(float64(width-want[i])) > .001 {
			t.Fatalf("列%d实际宽度=%g，预期%g；整组=%v", i, width, want[i], tableActualWidths(v))
		}
	}
}

// 从真实分配边缘派发输入，每次移动后布局，不能只断言基础Width。
func TestTableResizeSequentialShrunkColumns(t *testing.T) {
	v, _, first, _ := tableFixture(100)
	defer v.SetModel(nil)
	columns := []*TableColumn{first, NewTableColumn("note", "Notes"), NewTableColumn("action", "Action"), NewTableColumn("size", "Size"), NewTableColumn("description", "Description")}
	for i, c := range columns {
		c.SetWidth([]float32{240, 200, 100, 140, 420}[i])
		c.SetMinWidth([]float32{80, 80, 80, 40, 40}[i])
	}
	v.SetColumns(columns...)
	tableArrange(v, 856, 180)
	before := tableActualWidths(v)
	d, host := new(gui.EventDispatcher), &tabInputHost{root: v}
	for _, index := range []int{3, 4} {
		start := tableActualWidths(v)
		bounds := v.headers[columns[index]].Snapshot().Bounds
		x := bounds.X + bounds.Width - 4
		dispatchTabPointer(t, d, host, events.PointerDown, x, 16)
		dispatchTabPointer(t, d, host, events.PointerMove, x+12, 16)
		tableArrange(v, 856, 180)
		want := slices.Clone(start)
		want[index] += 12
		if index == 3 {
			want[4] -= 12
		}
		tableCheckWidths(t, v, want)
		dispatchTabPointer(t, d, host, events.PointerUp, x+12, 16)
		v.Refresh()
		v.SetColumns(columns...) // 相同声明式列集合不能触发重新分配。
		tableArrange(v, 856, 180)
		tableCheckWidths(t, v, want)
	}
	if math.Abs(float64(columns[3].Width()-before[3]-12)) > .001 || math.Abs(float64(columns[4].Width()-before[4])) > .001 || v.scroll.Snapshot().MaxScrollX <= 0 {
		t.Fatalf("连续拖动未保留选择的基础宽度/末列溢出：%g/%g maxX=%g", columns[3].Width(), columns[4].Width(), v.scroll.Snapshot().MaxScrollX)
	}
}

func TestTableResizeRightAllocationAndCancel(t *testing.T) {
	for _, expand := range []bool{false, true} {
		t.Run(map[bool]string{false: "保留空白", true: "右侧伸展"}[expand], func(t *testing.T) {
			v, _, first, _ := tableFixture(100)
			defer v.SetModel(nil)
			middle, last := NewTableColumn("middle", "Middle"), NewTableColumn("last", "Last")
			for _, c := range []*TableColumn{first, middle, last} {
				c.SetWidth(100)
			}
			middle.SetMinWidth(80)
			middle.SetMaxWidth(180)
			last.SetMinWidth(90)
			last.SetExpand(expand)
			last.SetResizable(false) // 间接分配不受直接拖动开关影响。
			v.SetColumns(first, middle, last)
			tableArrange(v, 310, 180)
			tableCheckWidths(t, v, []float32{100, 100, 100})
			var notifications []float32
			middle.ConnectResize(func(width float32) { notifications = append(notifications, width) })
			last.ConnectResize(func(float32) { t.Fatal("右侧自动分配发出Resize") })
			d, host := new(gui.EventDispatcher), &tabInputHost{root: v}
			bounds := v.headers[middle].Snapshot().Bounds
			x := bounds.X + bounds.Width - 4
			dispatchTabPointer(t, d, host, events.PointerDown, x, 16)
			for _, step := range []struct {
				delta float32
				want  []float32
			}{
				{30, []float32{100, 130, 90}},
				{5, []float32{100, 105, 95}},
				{-50, []float32{100, 80, map[bool]float32{false: 100, true: 120}[expand]}},
				{0, []float32{100, 100, 100}},
				{12, []float32{100, 112, 90}},
			} {
				dispatchTabPointer(t, d, host, events.PointerMove, x+step.delta, 16)
				tableArrange(v, 310, 180)
				tableCheckWidths(t, v, step.want)
			}
			tableKey(t, d, host, events.KeyEscape, 0)
			tableArrange(v, 310, 180)
			tableCheckWidths(t, v, []float32{100, 100, 100})
			if !slices.Equal(notifications, []float32{130, 105, 80, 100, 112, 100}) || last.Width() != 100 {
				t.Fatalf("通知或右侧基础宽度错误：%v/%g", notifications, last.Width())
			}
		})
	}
}

// 受控应用写入新值后，旧输入和Esc都不能恢复旧快照。
func TestTableResizeExplicitWriteInvalidatesGesture(t *testing.T) {
	v, _, column, _ := tableFixture(100)
	defer v.SetModel(nil)
	tableArrange(v, 300, 180)
	d, host := new(gui.EventDispatcher), &tabInputHost{root: v}
	dispatchTabPointer(t, d, host, events.PointerDown, 156, 16)
	dispatchTabPointer(t, d, host, events.PointerMove, 176, 16)
	column.SetWidth(210)
	dispatchTabPointer(t, d, host, events.PointerMove, 186, 16)
	tableKey(t, d, host, events.KeyEscape, 0)
	dispatchTabPointer(t, d, host, events.PointerUp, 186, 16)
	if column.Width() != 210 {
		t.Fatalf("旧手势覆盖应用的新宽度：%g", column.Width())
	}
}

func TestTableResizeBlankRestoreAndExistingOverflow(t *testing.T) {
	for _, tc := range []struct {
		name           string
		viewport       float32
		widths, minima []float32
		expand         int
		delta          float32
		want           []float32
	}{
		{"先用空白", 400, []float32{100, 100, 100}, nil, -1, 40, []float32{100, 140, 100}},
		{"再借右侧", 400, []float32{100, 100, 100}, nil, -1, 120, []float32{100, 220, 80}},
		{"先恢复基础", 300, []float32{200, 200, 200, 200}, nil, 3, -30, []float32{75, 45, 90, 90}},
		{"先减少已有溢出", 100, []float32{100, 100, 100}, []float32{80, 40, 80}, -1, -40, []float32{80, 40, 80}},
		{"不重分配已有溢出", 100, []float32{100, 100, 100}, []float32{80, 40, 80}, -1, 12, []float32{80, 52, 80}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := NewTableView()
			columns := make([]*TableColumn, len(tc.widths))
			for i, width := range tc.widths {
				c := NewTableColumn(string(rune('a'+i)), "Column")
				c.SetWidth(width)
				if tc.minima != nil {
					c.SetMinWidth(tc.minima[i])
				}
				c.SetExpand(i == tc.expand)
				columns[i] = c
			}
			v.SetColumns(columns...)
			v.setColumnViewportWidth(tc.viewport)
			r := v.beginColumnResize(columns[1])
			r.width = columns[1].clamp(r.width + tc.delta)
			columns[1].width = r.width
			r.allocate(v)
			tableCheckWidths(t, v, tc.want)
		})
	}
}

func TestTableResizeNoMovementAndReturnToOrigin(t *testing.T) {
	for _, expand := range []bool{false, true} {
		v, _, c, _ := tableFixture(100)
		c.SetWidth(200)
		c.SetExpand(expand)
		width := float32(120)
		if expand {
			width = 400
		}
		tableArrange(v, width, 180)
		before := c.allocatedWidth
		resizes := 0
		c.ConnectResize(func(float32) { resizes++ })
		d, host := new(gui.EventDispatcher), &tabInputHost{root: v}
		bounds := v.headers[c].Snapshot().Bounds
		x := bounds.X + bounds.Width - 4
		dispatchTabPointer(t, d, host, events.PointerDown, x, 16)
		dispatchTabPointer(t, d, host, events.PointerUp, x, 16)
		if c.Width() != 200 || c.allocatedWidth != before || resizes != 0 {
			t.Fatal("点击将实际宽度写回基础偏好")
		}
		dispatchTabPointer(t, d, host, events.PointerDown, x, 16)
		dispatchTabPointer(t, d, host, events.PointerMove, x-12, 16)
		tableArrange(v, width, 180)
		dispatchTabPointer(t, d, host, events.PointerMove, x, 16)
		tableArrange(v, width, 180)
		dispatchTabPointer(t, d, host, events.PointerUp, x, 16)
		if c.Width() != 200 || c.allocatedWidth != before || resizes != 2 {
			t.Fatal("拖回原边缘未恢复原偏好/产生重复通知")
		}
		v.SetModel(nil)
	}
}

func TestTableResizeScrollbarOccupancyAndAutomaticResume(t *testing.T) {
	v, _, first, _ := tableFixture(4)
	defer v.SetModel(nil)
	middle, last := NewTableColumn("middle", "Middle"), NewTableColumn("last", "Last")
	for _, c := range []*TableColumn{first, middle, last} {
		c.SetWidth(100)
	}
	v.SetColumns(first, middle, last)
	tableArrange(v, 310, 172)
	if v.scroll.Snapshot().MaxScrollY != 0 {
		t.Fatal("初始四行应能放下")
	}
	d, host := new(gui.EventDispatcher), &tabInputHost{root: v}
	bounds := v.headers[last].Snapshot().Bounds
	x := bounds.X + bounds.Width - 4
	dispatchTabPointer(t, d, host, events.PointerDown, x, 16)
	dispatchTabPointer(t, d, host, events.PointerMove, x+20, 16)
	tableArrange(v, 310, 172)
	tableCheckWidths(t, v, []float32{100, 100, 120})
	if v.scroll.Snapshot().MaxScrollY <= 0 || v.scroll.Snapshot().MaxScrollX <= 0 {
		t.Fatal("未建立横向条促使纵向条出现的场景")
	}
	dispatchTabPointer(t, d, host, events.PointerUp, x+20, 16)
	tableArrange(v, 310, 172)
	tableCheckWidths(t, v, []float32{100, 100, 120})
	// 外部尺寸变化后允许全列重新分配，但保留用户基础偏好。
	tableArrange(v, 400, 172)
	tableCheckWidths(t, v, []float32{100, 100, 120})
	tableArrange(v, 250, 172)
	if first.allocatedWidth >= 100 || last.Width() != 120 || v.columnResize != nil {
		t.Fatal("真实尺寸变化未恢复自动收缩")
	}
}

func TestTableResizeCancelRestoresPreviousManualAllocation(t *testing.T) {
	v, _, first, _ := tableFixture(100)
	defer v.SetModel(nil)
	last := NewTableColumn("last", "Last")
	v.SetColumns(first, last)
	tableArrange(v, 250, 180)
	d, host := new(gui.EventDispatcher), &tabInputHost{root: v}
	drag := func(c *TableColumn, delta float32, cancel bool) {
		bounds := v.headers[c].Snapshot().Bounds
		x := bounds.X + bounds.Width - 4
		dispatchTabPointer(t, d, host, events.PointerDown, x, 16)
		dispatchTabPointer(t, d, host, events.PointerMove, x+delta, 16)
		tableArrange(v, 250, 180)
		if cancel {
			tableKey(t, d, host, events.KeyEscape, 0)
		}
		dispatchTabPointer(t, d, host, events.PointerUp, x+delta, 16)
		tableArrange(v, 250, 180)
	}
	drag(first, 12, false)
	before := tableActualWidths(v)
	drag(last, 20, true)
	tableCheckWidths(t, v, before)
	if v.columnResize == nil || v.columnResize.previous != nil {
		t.Fatal("取消未恢复前一分配/保留无界手势历史")
	}
}

func TestTableResizeTracksContentCoordinatesWhileScrolling(t *testing.T) {
	v, _, first, _ := tableFixture(100)
	defer v.SetModel(nil)
	middle, last := NewTableColumn("middle", "Middle"), NewTableColumn("last", "Last")
	first.SetMinWidth(160)
	last.SetMinWidth(160)
	v.SetColumns(first, middle, last)
	tableArrange(v, 310, 180)
	v.scroll.SetScrollX(40)
	before := middle.allocatedWidth
	d, host := new(gui.EventDispatcher), &tabInputHost{root: v}
	bounds := v.headers[middle].Snapshot().Bounds
	x := bounds.X + bounds.Width - 4
	dispatchTabPointer(t, d, host, events.PointerDown, x, 16)
	dispatchTabPointer(t, d, host, events.PointerMove, x+24, 16)
	tableArrange(v, 310, 180)
	v.scroll.SetScrollX(52)
	dispatchTabPointer(t, d, host, events.PointerMove, x+24, 16)
	tableArrange(v, 310, 180)
	tableCheckWidths(t, v, []float32{160, before + 36, 160})
	after := v.headers[middle].Snapshot().Bounds
	if math.Abs(float64(after.X+after.Width-bounds.X-bounds.Width-24)) > .001 {
		t.Fatal("内容偏移变化后屏幕边缘没有保持在指针处")
	}
	dispatchTabPointer(t, d, host, events.PointerUp, x+24, 16)
}

func TestTableResizeGeometryChangesInvalidateOldInput(t *testing.T) {
	for _, name := range []string{"min", "max", "expand", "order", "other-width"} {
		t.Run(name, func(t *testing.T) {
			v, _, first, _ := tableFixture(100)
			defer v.SetModel(nil)
			other := NewTableColumn("other", "Other")
			v.SetColumns(first, other)
			tableArrange(v, 400, 180)
			d, host := new(gui.EventDispatcher), &tabInputHost{root: v}
			dispatchTabPointer(t, d, host, events.PointerDown, 156, 16)
			dispatchTabPointer(t, d, host, events.PointerMove, 176, 16)
			switch name {
			case "min":
				first.SetMinWidth(120)
			case "max":
				first.SetMaxWidth(190)
			case "expand":
				first.SetExpand(true)
			case "order":
				v.SetColumns(other, first)
			case "other-width":
				other.SetWidth(190)
			}
			tableArrange(v, 400, 180)
			before := tableActualWidths(v)
			dispatchTabPointer(t, d, host, events.PointerMove, 196, 16)
			tableKey(t, d, host, events.KeyEscape, 0)
			dispatchTabPointer(t, d, host, events.PointerUp, 196, 16)
			tableArrange(v, 400, 180)
			tableCheckWidths(t, v, before)
			if first.Width() != 180 {
				t.Fatal("旧手势覆盖了应用的新列配置")
			}
		})
	}
}

func TestTableResizeClampedMotionPreservesBasePreference(t *testing.T) {
	v, _, c, _ := tableFixture(100)
	defer v.SetModel(nil)
	tableArrange(v, 50, 180)
	if c.allocatedWidth != c.MinWidth() {
		t.Fatal("未建立最小宽度场景")
	}
	c.ConnectResize(func(float32) { t.Fatal("被下限挡住的位移发送Resize") })
	d, host := new(gui.EventDispatcher), &tabInputHost{root: v}
	bounds := v.headers[c].Snapshot().Bounds
	x := bounds.X + bounds.Width - 4
	dispatchTabPointer(t, d, host, events.PointerDown, x, 16)
	dispatchTabPointer(t, d, host, events.PointerMove, x-12, 16)
	dispatchTabPointer(t, d, host, events.PointerUp, x-12, 16)
	if c.Width() != 160 {
		t.Fatal("无有效位移时改写了基础偏好")
	}
}

func TestTableResizeActualChangeWithSameBaseStillNotifies(t *testing.T) {
	v, _, c, _ := tableFixture(100)
	defer v.SetModel(nil)
	c.SetWidth(100)
	c.SetExpand(true)
	tableArrange(v, 400, 180)
	before := c.allocatedWidth
	var widths []float32
	c.ConnectResize(func(w float32) { widths = append(widths, w) })
	d, host := new(gui.EventDispatcher), &tabInputHost{root: v}
	bounds := v.headers[c].Snapshot().Bounds
	x := bounds.X + bounds.Width - 4
	dispatchTabPointer(t, d, host, events.PointerDown, x, 16)
	dispatchTabPointer(t, d, host, events.PointerMove, x+100-before, 16)
	tableArrange(v, 400, 180)
	tableCheckWidths(t, v, []float32{100})
	tableKey(t, d, host, events.KeyEscape, 0)
	tableArrange(v, 400, 180)
	tableCheckWidths(t, v, []float32{before})
	if !slices.Equal(widths, []float32{100, 100}) {
		t.Fatalf("实际变化被相同基础宽度过滤：%v", widths)
	}
}
