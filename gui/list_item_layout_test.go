package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
)

type viewportItemDelegate struct {
	*mockListDelegate
	bounds []geometry.Rectangle
	during func()
}

func (d *viewportItemDelegate) ArrangeItem(_ int, item Widget, bounds geometry.Rectangle, viewport geometry.Size, offset geometry.Point) {
	d.bounds = append(d.bounds, bounds)
	item.Arrange(geometry.Rect(0, bounds.Y, viewport.Width, bounds.Height))
	if d.during != nil {
		d.during()
	}
}

func TestListViewOptionalItemAllocationAndDefault(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(map[bool]string{false: "普通行保留自然宽度", true: "视口行保留自然测量"}[custom], func(t *testing.T) {
			lv := NewListView()
			d := newMockListDelegate(map[int]float32{0: 10, 1: 10})
			d.widths = map[int]float32{0: 600, 1: 20}
			lv.SetModel(NewSliceListModel([]int{0, 1}))
			delegate := &viewportItemDelegate{mockListDelegate: d}
			if custom {
				lv.SetDelegate(delegate)
			} else {
				lv.SetDelegate(d)
			}
			defer lv.SetModel(nil)
			lv.LayoutVisible(geometry.Size{Width: 100, Height: 20}, geometry.Point{X: 400})
			if lv.ContentSize().Width != 600 || lv.widths[1] != 20 {
				t.Fatal("行分配反向污染自然宽度", lv.ContentSize(), lv.widths)
			}
			want := geometry.Rect(-400, 10, 100, 10)
			if custom {
				want.X = 0
				if len(delegate.bounds) != 2 || delegate.bounds[0] != (geometry.Rect(-400, 0, 600, 10)) {
					t.Fatal("分配委托未收到原始内容几何", delegate.bounds)
				}
			}
			if got := lv.items[1].Rect(); got != want {
				t.Fatalf("短行=%v，预期%v", got, want)
			}
		})
	}
}

func TestListViewNaturalWidthCacheSurvivesViewportResize(t *testing.T) {
	lv := NewListView()
	m := NewSliceListModel(make([]int, 100))
	d := newMockListDelegate(nil)
	d.widths = make(map[int]float32)
	for i := range 100 {
		d.widths[i] = 50
	}
	d.widths[0] = 600
	lv.SetModel(m)
	lv.SetDelegate(d)
	defer lv.SetModel(nil)
	layout := func(width, y float32) {
		lv.LayoutVisible(geometry.Size{Width: width, Height: 20}, geometry.Point{Y: y})
	}
	layout(200, 0)
	layout(200, 100)
	for _, width := range []float32{120, 800, 100} {
		layout(width, 100)
		if lv.ContentSize().Width != max(width, 600) || lv.widths[0] != 600 || lv.items[0] != nil {
			t.Fatal("调整视口丢失离屏自然宽度或重新实现离屏行", width, lv.ContentSize(), lv.widths[0])
		}
	}
	m.Remove(0)
	if lv.ContentSize().Width != 100 {
		t.Fatal("删除最宽行没有收缩估计范围", lv.ContentSize())
	}
	// 改小视口后，短行的旧分配宽度不得冒充自然宽度而撑出滚动范围。
	lv.SetModel(NewSliceListModel([]int{0}))
	d.widths[0] = 20
	layout(300, 0)
	layout(100, 0)
	if lv.ContentSize().Width != 100 || lv.widths[0] != 20 {
		t.Fatal("缓存了视口撑宽后的值", lv.ContentSize(), lv.widths)
	}
}

func TestListViewItemAllocationStopsAfterLifecycleChange(t *testing.T) {
	for _, destroy := range []bool{false, true} {
		t.Run(map[bool]string{false: "更换模型", true: "销毁列表"}[destroy], func(t *testing.T) {
			lv := NewListView()
			d := &viewportItemDelegate{mockListDelegate: newMockListDelegate(nil)}
			lv.SetModel(NewSliceListModel([]int{0, 1, 2}))
			lv.SetDelegate(d)
			d.during = func() {
				if destroy {
					lv.destroy(lv)
				} else {
					lv.SetModel(nil)
				}
			}
			lv.LayoutVisible(geometry.Size{Width: 100, Height: 30}, geometry.Point{})
			if len(d.bounds) != 1 || !destroy && len(lv.items) != 0 {
				t.Fatal("生命周期失效后继续分配旧行", d.bounds, lv.VisibleIndexes())
			}
		})
	}
}
