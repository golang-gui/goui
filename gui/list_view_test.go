package gui

import (
	"slices"
	"sync"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

// 变高行的大跨度Reveal完成后，悬停／焦点只能更新状态，不能重估整份文档。
// 使用真实DispatchEvent与样式／布局／绘制遍历，不创建原生窗口。
func TestListViewRevealEndSurvivesPointerAndFocus(t *testing.T) {
	setTestApplication(t, nil)
	lv := NewListView()
	lv.SetFocusable(true)
	lv.SetModel(NewSliceListModel(make([]int, 10000)))
	d := newMockListDelegate(make(map[int]float32))
	d.widths = make(map[int]float32)
	for i := 0; i < 10000; i++ {
		d.heights[i], d.widths[i] = 55, 1100
		if i%7 == 0 {
			d.heights[i] = 124
		}
	}
	lv.SetDelegate(d)
	sv := NewScrollView()
	sv.SetChild(lv)
	win := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{width: 320, height: 500}}})
	win.SetWidget(sv)
	defer win.Destroy()
	settle := func() {
		t.Helper()
		for range 20 {
			win.updateStateStyles()
			win.layoutFrame(sv)
			paintStyleTestWidget(sv)
			if !win.layoutDirty && len(win.stateStyles) == 0 {
				return
			}
		}
		t.Fatal("列表布局没有收敛")
	}
	settle()
	completed := 0
	lv.ConnectRevealed(func(index int) {
		if index != 9999 {
			t.Fatalf("Reveal完成目标错误：%d", index)
		}
		completed++
	})
	lv.Reveal(9999)
	settle()
	last := lv.items[9999]
	if completed != 1 || last == nil || last.Rect().Y < 0 || last.Rect().Y+last.Rect().Height > lv.viewport.Height {
		t.Fatalf("末行未完整到达视口：completed=%d visible=%v", completed, lv.VisibleIndexes())
	}
	before, extent, rect := sv.Snapshot(), lv.ContentSize(), last.Rect()
	indexes := lv.VisibleIndexes()
	measured, setups, binds := len(lv.heights), d.setups, len(d.binds)
	if measured <= len(indexes) || lv.heights[0] != 124 {
		t.Fatal("回归场景必须保留首屏的离屏行高")
	}
	point := last.Snapshot().Bounds.Pos.Add(geometry.Point{X: 10, Y: 5})
	for _, event := range []events.Event{
		events.PointerEvent{EventType: events.PointerMove, Position: point},
		events.PointerEvent{EventType: events.PointerDown, Position: point, Button: events.PointerButtonLeft, Buttons: events.PointerButtonLeftDown},
		events.PointerEvent{EventType: events.PointerUp, Position: point, Button: events.PointerButtonLeft},
		events.PointerEvent{EventType: events.PointerLeave, Position: point},
		events.KeyEvent{EventType: events.KeyDown, Key: events.KeyTab},
	} {
		if err := win.DispatchEvent(event); err != nil {
			t.Fatal(err)
		}
		settle()
		// 滚动条消费端使用SetScrollY；这里单独验证已在末尾时的重复设置。
		sv.SetScrollY(sv.Snapshot().MaxScrollY)
		settle()
		if lv.ContentSize() != extent || len(lv.heights) != measured || lv.heights[0] != 124 ||
			sv.ScrollY() != before.ScrollY || sv.Snapshot().MaxScrollY != before.MaxScrollY ||
			lv.items[9999] != last || last.Rect() != rect || !slices.Equal(lv.VisibleIndexes(), indexes) ||
			d.setups != setups || len(d.binds) != binds || completed != 1 {
			t.Fatalf("%T使末尾回跳或丢失测量：offset=%g/%g extent=%v/%v cached=%d/%d visible=%v", event,
				sv.ScrollY(), before.ScrollY, lv.ContentSize(), extent, len(lv.heights), measured, lv.VisibleIndexes())
		}
	}
}

func TestListViewDirectLayoutConsumesStyleBeforeRows(t *testing.T) {
	setTestApplication(t, nil)
	lv := NewListView()
	lv.SetModel(NewSliceListModel(make([]int, 100)))
	lv.SetDelegate(newMockListDelegate(nil))
	defer lv.SetModel(nil)
	// TableView等组合控件可以直接调用LayoutVisible，不经过ScrollView对ListView的Measure。
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 30}, geometry.Point{})
	if !lv.styleValid {
		t.Fatal("直接布局未先消费样式通知，绘制阶段仍可能使测量失效")
	}
	before, count := lv.ContentSize(), len(lv.heights)
	paintStyleTestWidget(lv)
	if lv.ContentSize() != before || len(lv.heights) != count {
		t.Fatal("绘制阶段改变了刚完成的列表测量")
	}
}

func TestListViewRevealEstimatedVariableAndOversizedRows(t *testing.T) {
	for _, oversized := range []bool{false, true} {
		lv := NewListView()
		lv.SetModel(NewSliceListModel(make([]int, 1000)))
		heights := map[int]float32{999: 31.5}
		if oversized {
			heights[999] = 230
		}
		d := newMockListDelegate(heights)
		lv.SetDelegate(d)
		sv := NewScrollView()
		sv.SetChild(lv)
		var completed []int
		lv.ConnectRevealed(func(index int) { completed = append(completed, index) })
		lv.Reveal(999) // valid before any allocation or measured estimate
		for i := 0; i < 20; i++ {
			sv.Measure(layout.Tight(geometry.Size{Width: 200, Height: 100}))
			sv.Arrange(geometry.Rect(0, 0, 200, 100))
			if !lv.revealPending {
				break
			}
		}
		if lv.revealPending || !slices.Contains(lv.VisibleIndexes(), 999) || d.setups > 22 {
			t.Fatalf("reveal did not converge or created all rows: pending=%v visible=%v setups=%d", lv.revealPending, lv.VisibleIndexes(), d.setups)
		}
		if oversized && lv.items[999].Rect().Y < 0 || oversized && lv.items[999].Rect().Y >= 1 {
			t.Fatalf("oversized leading edge=%g", lv.items[999].Rect().Y)
		}
		sv.SetScrollY(0)
		sv.Arrange(sv.Rect())
		if sv.ScrollY() != 0 {
			t.Fatal("completed request stole user scroll")
		}
		if !slices.Equal(completed, []int{999}) {
			t.Fatalf("completion must be reported once, not on ordinary scrolling: %v", completed)
		}
		lv.Reveal(4)
		lv.Reveal(-1)
		if lv.revealIndex != 4 {
			t.Fatal("invalid request replaced valid target")
		}
		lv.SetModel(NewSliceListModel([]int{0}))
		if lv.revealPending {
			t.Fatal("model replacement retained old target")
		}
		if !slices.Equal(completed, []int{999}) {
			t.Fatal("cancelled reveal emitted completion")
		}
	}
}
func TestListViewDelegateReplacementUsesOldBindings(t *testing.T) {
	lv := NewListView()
	lv.SetModel(NewSliceListModel([]int{0, 1, 2}))
	old, next := newMockListDelegate(nil), newMockListDelegate(nil)
	lv.SetDelegate(old)
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 30}, geometry.Point{})
	lv.SetDelegate(next)
	if len(old.unbinds) != 3 || len(next.unbinds) != 0 {
		t.Fatal("wrong delegate unbound old widgets")
	}
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 30}, geometry.Point{})
	if next.setups != 3 {
		t.Fatal("new delegate reused incompatible shells")
	}
}

type listInstallModel struct {
	*SliceListModel[int]
	connections, deliveries int
}

func (m *listInstallModel) ConnectItems(fn func()) signal.Handle {
	m.connections++
	return m.SliceListModel.ConnectItems(func() { m.deliveries++; fn() })
}

func (m *listInstallModel) ConnectItemsChanged(fn func(ListChange)) signal.Handle {
	m.connections++
	return m.SliceListModel.ConnectItemsChanged(func(change ListChange) { m.deliveries++; fn(change) })
}

// 显式重新设置同一模型必须重连、解绑，但只有一个有效订阅。
func TestListViewExplicitReinstallation(t *testing.T) {
	m := &listInstallModel{SliceListModel: NewSliceListModel([]int{1, 2, 3})}
	lv := NewListView()
	d := newMockListDelegate(nil)
	lv.SetDelegate(d)
	lv.SetModel(m)
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 30}, geometry.Point{})
	lv.SetModel(m)
	if m.connections != 2 || len(d.unbinds) != 3 {
		t.Fatalf("same model was not reinstalled: connections=%d unbinds=%d", m.connections, len(d.unbinds))
	}
	m.Set(0, 4)
	if m.deliveries != 1 {
		t.Fatal("old model subscription leaked")
	}
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 30}, geometry.Point{})
	before := d.setups
	lv.SetDelegate(d)
	if len(d.unbinds) != 6 || len(lv.pool) != 0 {
		t.Fatal("same delegate retained old bindings or shells")
	}
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 30}, geometry.Point{})
	if d.setups != before+3 {
		t.Fatal("explicit delegate installation reused the old pool")
	}
	lv.SetModel(nil)
	m.Set(0, 5)
	if m.deliveries != 1 || lv.ItemsCount() != 0 {
		t.Fatal("nil model did not disconnect")
	}
}

type listValueModel struct {
	*listInstallModel
	payload any
}
type listValueDelegate struct {
	*mockListDelegate
	payload any
}

func TestListViewInstallsInterfaceValuesWithoutComparing(t *testing.T) {
	for _, payload := range []any{[]int{1}, map[string]int{"x": 1}, func() {}} {
		m := listValueModel{&listInstallModel{SliceListModel: NewSliceListModel([]int{1})}, payload}
		d := listValueDelegate{newMockListDelegate(nil), payload}
		lv := NewListView()
		lv.SetModel(m)
		lv.SetDelegate(d)
		lv.LayoutVisible(geometry.Size{Width: 100, Height: 10}, geometry.Point{})
		lv.SetModel(m)
		lv.SetDelegate(d)
		if m.connections != 2 || len(d.unbinds) != 1 {
			t.Fatal("value reinstallation was skipped")
		}
		lv.SetModel(nil)
	}
}

// mockListWidget is a widget whose measured height is set explicitly
// (standing in for a real content widget; the delegate sets it during Bind).
type mockListWidget struct {
	WidgetBase
	height float32
	width  float32
}

func (m *mockListWidget) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(geometry.Size{Width: max(c.Min.Width, m.width), Height: m.height})
}

func (m *mockListWidget) Arrange(rect geometry.Rectangle) {
	m.WidgetBase.Arrange(rect)
}

// mockListDelegate records Setup/Bind/Unbind calls and serves per-index
// heights (the height is applied to the widget during Bind, so ListView's
// post-Bind measurement observes it).
type mockListDelegate struct {
	setups   int
	binds    []int
	unbinds  []int
	heights  map[int]float32
	widths   map[int]float32
	lastBind map[int]Widget
}

func newMockListDelegate(heights map[int]float32) *mockListDelegate {
	return &mockListDelegate{heights: heights, lastBind: make(map[int]Widget)}
}

func (d *mockListDelegate) Setup() Widget {
	d.setups++
	return &mockListWidget{height: 10} // shell default height
}

func (d *mockListDelegate) Bind(index int, w Widget) {
	d.binds = append(d.binds, index)
	d.lastBind[index] = w
	if h, ok := d.heights[index]; ok {
		w.(*mockListWidget).height = h
	}
	if width, ok := d.widths[index]; ok {
		w.(*mockListWidget).width = width
	}
}

func (d *mockListDelegate) Unbind(index int, w Widget) {
	d.unbinds = append(d.unbinds, index)
}

func TestListViewMeasurementInvalidation(t *testing.T) {
	t.Run("offscreen measurements remain estimates until rebound", func(t *testing.T) {
		lv := NewListView()
		d := newMockListDelegate(map[int]float32{})
		d.widths = make(map[int]float32)
		for i := 0; i < 100; i++ {
			d.heights[i] = 10
			d.widths[i] = 100
		}
		d.widths[0] = 500
		lv.SetModel(NewSliceListModel(make([]int, 100)))
		lv.SetDelegate(d)
		defer lv.SetModel(nil)
		viewport := geometry.Size{Width: 100, Height: 20}
		lv.LayoutVisible(viewport, geometry.Point{})
		lv.LayoutVisible(viewport, geometry.Point{Y: 100})
		d.widths[0], d.heights[0] = 700, 30
		lv.Refresh()
		lv.LayoutVisible(viewport, geometry.Point{Y: 100})
		if lv.ContentSize().Width != 500 || lv.heights[0] != 10 {
			t.Fatal("refresh discarded offscreen estimates and changed the scroll range")
		}
		lv.LayoutVisible(viewport, geometry.Point{})
		if lv.ContentSize().Width != 700 || lv.heights[0] != 30 {
			t.Fatal("rebound row treated historical dimensions as exact")
		}
	})
	t.Run("local layout and refresh preserve estimated extents", func(t *testing.T) {
		lv := NewListView()
		d := newMockListDelegate(map[int]float32{0: 10, 1: 20})
		lv.SetModel(NewSliceListModel(make([]int, 100)))
		lv.SetDelegate(d)
		viewport := geometry.Size{Width: 100, Height: 30}
		lv.LayoutVisible(viewport, geometry.Point{})
		row := d.lastBind[0].(*mockListWidget)
		row.height = 30
		row.RequestLayout()
		lv.LayoutVisible(viewport, geometry.Point{})
		if lv.heights[0] != 30 {
			t.Fatal("row RequestLayout was hidden by the index cache")
		}
		d.heights[0] = 50
		before := lv.ContentSize()
		lv.Refresh()
		if lv.ContentSize() != before {
			t.Fatal("Refresh discarded the estimated scroll extent")
		}
		lv.LayoutVisible(viewport, geometry.Point{})
		if lv.heights[0] != 50 {
			t.Fatal("Refresh did not rebind and remeasure visible rows")
		}
		lv.SetModel(nil)
	})
	t.Run("width invalidation repairs the locator prefix", func(t *testing.T) {
		lv := NewListView()
		lv.SetModel(NewSliceListModel(make([]int, 100)))
		lv.SetDelegate(newMockListDelegate(map[int]float32{1: 40}))
		lv.LayoutVisible(geometry.Size{Width: 100, Height: 80}, geometry.Point{})
		lv.LayoutVisible(geometry.Size{Width: 100, Height: 20}, geometry.Point{Y: 50})
		// Resizing invalidates offscreen measurements. The old prefix (10+40)
		// must not survive while heightAt now uses the original 10 DIP seed.
		lv.LayoutVisible(geometry.Size{Width: 80, Height: 20}, geometry.Point{Y: 50})
		visible := lv.VisibleIndexes()
		if len(visible) == 0 || visible[0] != 5 || lv.items[5].Rect().Y != 0 {
			t.Fatalf("stale locator after resize: visible=%v; want row 5 at y=0", visible)
		}
		lv.SetModel(nil)
	})
}

// --- ListView: layout & virtualization ---

func TestListViewVisibleRange(t *testing.T) {
	lv := NewListView()
	lv.SetModel(NewSliceListModel(make([]int, 100)))
	lv.SetDelegate(newMockListDelegate(nil))

	// All items measure 10 (delegate default) → waterfall layout.
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	got := lv.VisibleIndexes()
	if len(got) != 10 || got[0] != 0 || got[9] != 9 {
		t.Fatalf("visible at offset 0 should be [0..9], got %v", got)
	}

	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{Y: 50})
	got = lv.VisibleIndexes()
	if len(got) != 10 || got[0] != 5 || got[9] != 14 {
		t.Fatalf("visible at offset 50 should be [5..14], got %v", got)
	}

	// Offset at the content end (total 1000, max offset 900): last page.
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{Y: 900})
	got = lv.VisibleIndexes()
	if len(got) != 10 || got[0] != 90 || got[9] != 99 {
		t.Fatalf("visible at offset 900 should be [90..99], got %v", got)
	}
}

func TestListViewVariableHeights(t *testing.T) {
	// Every other item is twice as tall.
	heights := make(map[int]float32)
	for i := 0; i < 10; i++ {
		heights[i] = 10
		if i%2 == 1 {
			heights[i] = 20
		}
	}
	lv := NewListView()
	lv.SetModel(NewSliceListModel(make([]int, 10)))
	lv.SetDelegate(newMockListDelegate(heights))

	// Heights: 10,20,10,20,... Cumulative: 10,30,40,60,70,90,100,120,130,150
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 60}, geometry.Point{})
	got := lv.VisibleIndexes()
	// Viewport [0,60): item3 spans [40,60) and is visible; item4 starts at 60.
	if len(got) != 4 || got[0] != 0 || got[3] != 3 {
		t.Fatalf("visible should be [0..3], got %v", got)
	}

	// Scroll to y=60: item 4 starts at 60 → first=4, visible 4..7 (60..120)
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 60}, geometry.Point{Y: 60})
	got = lv.VisibleIndexes()
	if len(got) != 4 || got[0] != 4 || got[3] != 7 {
		t.Fatalf("visible at 60 should be [4..7], got %v", got)
	}

	// ContentSize reflects exact heights once measured.
	if h := lv.ContentSize().Height; h != 150 {
		t.Fatalf("content height should be 150, got %v", h)
	}
}

func TestListViewItemReuse(t *testing.T) {
	lv := NewListView()
	lv.SetModel(NewSliceListModel(make([]int, 100)))
	d := newMockListDelegate(nil)
	lv.SetDelegate(d)

	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if d.setups != 10 {
		t.Fatalf("first layout should create 10 items, got %d", d.setups)
	}
	firstBinds := len(d.binds)

	// Scroll down one row (10px): item 0 scrolls out, item 10 comes in.
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{Y: 10})
	if len(d.binds)-firstBinds != 1 {
		t.Fatalf("scroll down should bind 1 new item, got %d", len(d.binds)-firstBinds)
	}
	if len(d.unbinds) != 1 || d.unbinds[0] != 0 {
		t.Fatalf("scrolled-out item 0 should be unbound, got %v", d.unbinds)
	}
	if d.setups != 10 {
		t.Fatalf("scroll down should reuse the outgoing shell, setups=%d", d.setups)
	}

	// On the reverse scroll the incoming row precedes the outgoing row, so
	// one spare shell is created; further scrolling can reuse that spare.
	bindsBefore := len(d.binds)
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if len(d.binds)-bindsBefore != 1 || d.setups != 11 {
		t.Fatalf("scroll back should re-bind cached item, binds=%d setups=%d", len(d.binds)-bindsBefore, d.setups)
	}
}

// 追加项不解绑原行；插入/删除只平移保留行，不让焦点或声明子树离开原记录。
func TestListViewIncrementalChangesPreserveRows(t *testing.T) {
	model := NewSliceListModel([]int{1, 2, 3})
	lv := NewListView()
	d := newMockListDelegate(nil)
	lv.SetModel(model)
	lv.SetDelegate(d)
	defer lv.SetModel(nil)
	viewport := geometry.Size{Width: 100, Height: 100}
	lv.LayoutVisible(viewport, geometry.Point{})
	a, b, c := lv.items[0], lv.items[1], lv.items[2]
	model.Append(4)
	if lv.items[0] != a || lv.items[1] != b || lv.items[2] != c || len(d.unbinds) != 0 {
		t.Fatal("追加行解绑了原有可见行")
	}
	lv.LayoutVisible(viewport, geometry.Point{})
	model.Insert(1, 9)
	if lv.items[0] != a || lv.items[2] != b || lv.items[3] != c || len(d.unbinds) != 0 {
		t.Fatal("插入行没有平移保留项")
	}
	lv.LayoutVisible(viewport, geometry.Point{})
	model.Remove(1)
	if lv.items[0] != a || lv.items[1] != b || lv.items[2] != c || !slices.Equal(d.unbinds, []int{1}) {
		t.Fatal("删除新增行影响了其他记录", d.unbinds)
	}
	lv.LayoutVisible(viewport, geometry.Point{})
	if got := lv.Snapshot(); len(got.Children) != 4 || d.setups != 5 {
		t.Fatal("增量更新后的子树或回收数量错误", len(got.Children), d.setups)
	}
}

func TestSliceListModelChangeRanges(t *testing.T) {
	m := NewSliceListModel([]int{1, 2})
	var changes []ListChange
	legacy := 0
	m.ConnectItems(func() { legacy++ })
	h := m.ConnectItemsChanged(func(change ListChange) { changes = append(changes, change) })
	m.Append(3)
	m.Insert(1, 4)
	m.Remove(0)
	m.Set(0, 5)
	m.Modify(func(items []int) []int { return items })
	m.SetItems([]int{6})
	want := []ListChange{{Position: 2, Added: 1}, {Position: 1, Added: 1}, {Removed: 1},
		{Removed: 1, Added: 1}, {Reset: true}, {Reset: true}}
	if legacy != len(want) || !slices.Equal(changes, want) {
		t.Fatal("旧订阅或精确通知范围错误", legacy, changes)
	}
	h.Disconnect()
	m.Append(7)
	if legacy != 7 || len(changes) != 6 {
		t.Fatal("精确订阅断开影响旧订阅，或泄漏连接")
	}
	// 先连接的监听者同步修改模型，后连接者不能继续收到旧区间。
	nested := NewSliceListModel([]int{1})
	mutated := false
	nested.ConnectItems(func() {
		if !mutated {
			mutated = true
			nested.SetItems([]int{8, 9})
		}
	})
	changes = nil
	nested.ConnectItemsChanged(func(change ListChange) { changes = append(changes, change) })
	nested.Append(2)
	if !slices.Equal(changes, []ListChange{{Reset: true}, {Reset: true}}) {
		t.Fatal("重入后向观察者交付了过期区间", changes)
	}
}

func TestListViewIncrementalMeasurementsAndReveal(t *testing.T) {
	m := NewSliceListModel(make([]int, 10))
	lv := NewListView()
	d := newMockListDelegate(map[int]float32{0: 10, 1: 20})
	d.widths = map[int]float32{1: 500}
	lv.SetModel(m)
	lv.SetDelegate(d)
	defer lv.SetModel(nil)
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 30}, geometry.Point{})
	row := lv.items[1]
	lv.Reveal(8)
	m.Insert(0, 1)
	if lv.items[2] != row || lv.heights[2] != 20 || lv.widths[2] != 500 || lv.revealIndex != 9 || !lv.revealPending {
		t.Fatal("索引平移丢失行、尺寸缓存或Reveal目标")
	}
	m.Remove(0)
	if lv.items[1] != row || lv.heights[1] != 20 || lv.revealIndex != 8 {
		t.Fatal("删除前置项没有恢复保留索引")
	}
	m.Remove(8)
	if lv.revealPending {
		t.Fatal("已删除的Reveal目标被转移到另一条记录")
	}
	m.Remove(1)
	if lv.ContentSize().Width != 100 {
		t.Fatal("删除最宽项后仍保留错误的横向范围", lv.ContentSize())
	}
	// 错误或过期范围必须重载，不保留错误身份，也不能访问越界索引。
	m.changed.Emit(ListChange{Position: 99, Added: 1}, m.revision)
	if len(lv.Children()) != 0 || lv.modelCount != m.ItemsCount() {
		t.Fatal("无法应用的范围未安全重载")
	}
}

type changingListDelegate struct {
	*mockListDelegate
	unbind func()
}

func (d *changingListDelegate) Unbind(index int, widget Widget) {
	d.mockListDelegate.Unbind(index, widget)
	if d.unbind != nil {
		fn := d.unbind
		d.unbind = nil
		fn()
	}
}

func TestListViewIncrementalUnbindReentry(t *testing.T) {
	for _, action := range []string{"reset", "model", "delegate"} {
		t.Run(action, func(t *testing.T) {
			m := NewSliceListModel([]int{1, 2, 3})
			lv := NewListView()
			d := &changingListDelegate{mockListDelegate: newMockListDelegate(nil)}
			lv.SetModel(m)
			lv.SetDelegate(d)
			defer lv.SetModel(nil)
			viewport := geometry.Size{Width: 100, Height: 100}
			lv.LayoutVisible(viewport, geometry.Point{})
			d.unbind = func() {
				switch action {
				case "reset":
					m.SetItems([]int{4, 5})
				case "model":
					lv.SetModel(NewSliceListModel([]int{6, 7}))
				case "delegate":
					lv.SetDelegate(newMockListDelegate(nil))
				}
			}
			m.Remove(0)
			lv.LayoutVisible(viewport, geometry.Point{})
			if len(lv.items) != 2 || len(lv.Children()) != 2 || lv.modelCount != 2 {
				t.Fatal("回调重入后留下旧行或错误数量", lv.VisibleIndexes(), len(lv.Children()))
			}
			for _, index := range []int{0, 1, 2} {
				count := 0
				for _, previous := range d.unbinds {
					if previous == index {
						count++
					}
				}
				if count != 1 {
					t.Fatal("旧绑定未恰好解绑一次", d.unbinds)
				}
			}
		})
	}
}

type listEditorRecord struct{ text string }
type listEditorDelegate struct {
	model *SliceListModel[*listEditorRecord]
}

func (*listEditorDelegate) Setup() Widget { return NewTextInput() }
func (d *listEditorDelegate) Bind(index int, widget Widget) {
	widget.(*TextInput).SetText(d.model.ItemAt(index).text)
}
func (*listEditorDelegate) Unbind(int, Widget) {}

// 真实输入分发和IME管理器替身；不创建原生窗口，不以重新Focus掩盖卸载。
func TestListViewIncrementalEditorComposition(t *testing.T) {
	_, win, _ := newInputFixture(t, "")
	m := NewSliceListModel([]*listEditorRecord{{}})
	lv := NewListView()
	lv.SetModel(m)
	lv.SetDelegate(&listEditorDelegate{model: m})
	win.SetWidget(lv)
	viewport := geometry.Size{Width: 200, Height: 200}
	lv.LayoutVisible(viewport, geometry.Point{})
	editor := lv.items[0].(*TextInput)
	win.SetFocusedWidget(editor)
	editor.ConnectText(func(text string) {
		m.ItemAt(0).text = text
		if m.ItemsCount() == 1 {
			m.Append(&listEditorRecord{})
		}
	})
	if err := win.CommitText("a"); err != nil {
		t.Fatal(err)
	}
	lv.LayoutVisible(viewport, geometry.Point{})
	if win.FocusedWidget() != editor || lv.items[0] != editor || m.ItemsCount() != 2 || editor.Text() != "a" {
		t.Fatal("输入新增行后丢失原编辑器或焦点")
	}
	editor.SetSelection(TextSelection{Anchor: 0, Caret: 1})
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "中", Caret: 3})
	composition := editor.preedit
	m.Append(&listEditorRecord{})
	lv.LayoutVisible(viewport, geometry.Point{})
	if composition == nil || editor.preedit != composition || editor.Selection() != (TextSelection{Anchor: 0, Caret: 1}) || win.FocusedWidget() != editor {
		t.Fatal("追加其他行取消了已有预编辑或选区")
	}
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "文"})
	dispatchKey(t, win, events.KeyZ, textCommandModifier())
	if editor.Text() != "a" || win.FocusedWidget() != editor {
		t.Fatal("追加行丢失原编辑历史")
	}
}

func TestListViewReloadOnModelChange(t *testing.T) {
	model := NewSliceListModel(make([]int, 10))
	lv := NewListView()
	d := newMockListDelegate(nil)
	// 只公开旧通知接口，继续覆盖旧模型必须完整重载的兼容契约。
	lv.SetModel(struct{ ListModel }{model})
	lv.SetDelegate(d)

	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if d.setups != 10 {
		t.Fatalf("expected 10 setups, got %d", d.setups)
	}

	// Model mutation invalidates index bindings, not delegate-owned shells.
	model.Append(0)
	if len(d.unbinds) != 10 {
		t.Fatalf("reload should unbind all 10 items, got %v", d.unbinds)
	}
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if d.setups != 10 || len(d.binds) != 20 {
		t.Fatalf("reload should rebind pooled shells, setups=%d binds=%d", d.setups, len(d.binds))
	}
	if got := lv.ItemsCount(); got != 11 {
		t.Fatalf("ItemsCount should follow model, got %d", got)
	}
}

func TestListViewUnbindOnScrollOut(t *testing.T) {
	lv := NewListView()
	lv.SetModel(NewSliceListModel(make([]int, 20)))
	d := newMockListDelegate(nil)
	lv.SetDelegate(d)

	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	// Scroll far enough to evict everything (10px rows, viewport 100):
	// offset 100 → items 10..19, items 0..9 unbound.
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{Y: 100})
	if len(d.unbinds) != 10 {
		t.Fatalf("expected 10 unbinds, got %v", d.unbinds)
	}
	for i := 0; i < 10; i++ {
		found := false
		for _, u := range d.unbinds {
			if u == i {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("item %d should have been unbound, got %v", i, d.unbinds)
		}
	}
}

func TestListViewEmpty(t *testing.T) {
	lv := NewListView()
	lv.SetDelegate(newMockListDelegate(nil))
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if got := lv.VisibleIndexes(); len(got) != 0 {
		t.Fatalf("empty list should have no visible items, got %v", got)
	}

	lv.SetModel(NewSliceListModel(make([]int, 10)))
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if got := lv.VisibleIndexes(); len(got) == 0 {
		t.Fatal("10 items should be visible")
	}
}

func TestListViewDropsUnregisteredChildren(t *testing.T) {
	lv := NewListView()
	lv.SetModel(NewSliceListModel(make([]int, 10)))
	lv.SetDelegate(newMockListDelegate(nil))
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})

	// Bypass the virtualization: mount a stray widget directly.
	stray := newTestWidget()
	lv.WidgetBase.AddChild(lv, stray)
	if len(lv.Children()) != 11 {
		t.Fatalf("stray child should be mounted, got %d children", len(lv.Children()))
	}

	// The next layout drops it: only registered items survive.
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if len(lv.Children()) != 10 {
		t.Fatalf("stray child should be dropped, got %d children", len(lv.Children()))
	}
}

func TestListViewContentSizeEstimate(t *testing.T) {
	lv := NewListView()
	lv.SetModel(NewSliceListModel(make([]int, 100)))
	lv.SetDelegate(newMockListDelegate(nil))

	// Nothing measured and no seed yet: content height is 0. After the first
	// layout the setupHeight seed (10) drives the estimate for the rest.
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if h := lv.ContentSize().Height; h != 1000 {
		t.Fatalf("content height should be 100*10, got %v", h)
	}

	// The estimate updates as the running mean of measured heights; with
	// uniform 10px rows it stays 10, so the total stays exact.
	if h := lv.ContentSize().Height; h != 1000 {
		t.Fatalf("content height should stay 1000, got %v", h)
	}
}

// --- SliceListModel (generic) ---

type row struct {
	name string
	n    int
}

func TestSliceListModelCRUD(t *testing.T) {
	m := NewSliceListModel([]string{"a", "b", "c"})
	if m.ItemsCount() != 3 {
		t.Fatalf("count should be 3, got %d", m.ItemsCount())
	}
	if m.ItemAt(1) != "b" {
		t.Fatal("ItemAt(1) should be b")
	}

	var changes int
	h := m.ConnectItems(func() { changes++ })

	m.Append("d")
	m.Insert(1, "x")
	m.Set(0, "A")
	m.Remove(2)
	if changes != 4 {
		t.Fatalf("expected 4 change notifications, got %d", changes)
	}
	if m.ItemsCount() != 4 {
		t.Fatalf("count should be 4, got %d", m.ItemsCount())
	}
	if m.ItemAt(0) != "A" || m.ItemAt(1) != "x" || m.ItemAt(2) != "c" || m.ItemAt(3) != "d" {
		t.Fatalf("unexpected items: %v", m.items)
	}

	h.Disconnect()
	m.Append("e")
	if changes != 4 {
		t.Fatalf("disconnected handler should not fire, got %d", changes)
	}
}

func TestSliceListModelStructs(t *testing.T) {
	m := NewSliceListModel([]row{{name: "first", n: 1}})
	m.Append(row{name: "second", n: 2})
	if got := m.ItemAt(1); got.name != "second" || got.n != 2 {
		t.Fatalf("unexpected item: %+v", got)
	}

	m.Set(0, row{name: "renamed", n: 10})
	if got := m.ItemAt(0); got.name != "renamed" {
		t.Fatalf("Set should replace the item, got %+v", got)
	}
}

func TestSliceListModelSetItems(t *testing.T) {
	m := NewSliceListModel([]int{1, 2})
	m.SetItems([]int{9, 8, 7, 6})
	if m.ItemsCount() != 4 || m.ItemAt(2) != 7 {
		t.Fatalf("SetItems should replace contents, got count=%d", m.ItemsCount())
	}
}

func TestSliceListModelModifyEmitsOnce(t *testing.T) {
	m := NewSliceListModel([]int{1, 2, 3, 4, 5})
	changes := 0
	h := m.ConnectItems(func() { changes++ })

	// Batch: remove evens, prepend 0, append 9 — all in one Modify → one emit.
	m.Modify(func(prev []int) []int {
		after := prev[:0]
		for _, v := range prev {
			if v%2 != 0 {
				after = append(after, v)
			}
		}
		return append(after, 9)
	})
	if changes != 1 {
		t.Fatalf("Modify should emit exactly once, got %d", changes)
	}
	if m.ItemsCount() != 4 {
		t.Fatalf("expected 4 items after batch, got %d", m.ItemsCount())
	}
	if m.ItemAt(0) != 1 || m.ItemAt(3) != 9 {
		t.Fatalf("unexpected items after batch: %v", m.items)
	}

	// In-place mutation without reallocation also works.
	m.Modify(func(prev []int) []int {
		for i := range prev {
			prev[i] *= 10
		}
		return prev
	})
	if changes != 2 {
		t.Fatalf("second Modify should emit once, got %d", changes)
	}
	if m.ItemAt(1) != 30 {
		t.Fatalf("in-place mutation should apply, got %v", m.items)
	}

	h.Disconnect()
}

func TestSliceListModelConcurrent(t *testing.T) {
	m := NewSliceListModel([]int{})
	const goroutines = 8
	const ops = 500

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				m.Append(base*ops + i)
				if i%10 == 0 {
					// Batch mutation racing with appends: must not corrupt.
					m.Modify(func(prev []int) []int {
						return append(prev, -1)
					})
				}
				if n := m.ItemsCount(); n > 0 {
					_ = m.ItemAt(0)
				}
			}
		}(g)
	}
	wg.Wait()

	expected := goroutines*ops + goroutines*(ops/10)
	if got := m.ItemsCount(); got != expected {
		t.Fatalf("expected %d items, got %d", expected, got)
	}
	// Every appended value must be present (no lost updates). -1 markers from
	// Modify can sit anywhere, so scan everything and skip them.
	seen := make(map[int]bool)
	for i := 0; i < m.ItemsCount(); i++ {
		v := m.ItemAt(i)
		if v != -1 {
			seen[v] = true
		}
	}
	for v := 0; v < goroutines*ops; v++ {
		if !seen[v] {
			t.Fatalf("value %d lost under concurrency", v)
		}
	}
}

// --- signal.Handle integration ---

// shellZeroDelegate: the shell measures 0 (like an empty Label) and the
// bound item measures 10. The estimate seed must come from the post-Bind
// measurement, or ContentSize collapses to 0 and scrolling breaks.
type shellZeroDelegate struct{}

func (d *shellZeroDelegate) Setup() Widget { return &mockListWidget{height: 0} }
func (d *shellZeroDelegate) Bind(index int, w Widget) {
	w.(*mockListWidget).height = 10
}
func (d *shellZeroDelegate) Unbind(index int, w Widget) {}

func TestListViewEstimateSeedAfterBind(t *testing.T) {
	lv := NewListView()
	lv.SetModel(NewSliceListModel(make([]int, 100)))
	lv.SetDelegate(&shellZeroDelegate{})

	// First layout: the seed comes from the bound item, not the empty shell.
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if lv.estimate != 10 {
		t.Fatalf("estimate seed should come from post-Bind measure, got %v", lv.estimate)
	}
	if h := lv.ContentSize().Height; h != 1000 {
		t.Fatalf("content height should be 100*10, got %v", h)
	}

	// A ScrollView hosting this list must become scrollable after layout.
	sv := NewScrollView()
	sv.SetChild(lv)
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	if !sv.Scrollable() {
		t.Fatalf("list viewport should be scrollable, content=%v rect=%v", sv.contentHeight, sv.Rect().Height)
	}
	sv.SetScrollY(50)
	if sv.ScrollY() != 50 {
		t.Fatalf("SetScrollY should take effect, got %v", sv.ScrollY())
	}
}

func TestListViewSetModelDisconnects(t *testing.T) {
	modelA := NewSliceListModel(make([]int, 5))
	modelB := NewSliceListModel(make([]int, 5))
	lv := NewListView()
	lv.SetModel(modelA)
	lv.SetDelegate(newMockListDelegate(nil))

	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if got := lv.ItemsCount(); got != 5 {
		t.Fatalf("count should follow modelA, got %d", got)
	}

	lv.SetModel(modelB)
	if got := lv.ItemsCount(); got != 5 {
		t.Fatalf("count should follow modelB, got %d", got)
	}

	// Mutating modelA must not affect the list anymore.
	modelA.Append(0)
	if got := lv.ItemsCount(); got != 5 {
		t.Fatalf("modelA mutation should be ignored after switch, got %d", got)
	}
}

// Compile-time interface assertions.
var (
	_ ListData[int]    = (*SliceListModel[int])(nil)
	_ ListData[row]    = (*SliceListModel[row])(nil)
	_ ListItemDelegate = (*mockListDelegate)(nil)
)

func TestListViewSnapshot(t *testing.T) {
	lv := NewListView()
	lv.SetModel(NewSliceListModel(make([]int, 100)))
	lv.SetDelegate(&shellZeroDelegate{})

	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	info := lv.Snapshot()
	if info.Role != RoleList {
		t.Fatalf("role should be list, got %q", info.Role)
	}
	if info.ItemCount != 100 {
		t.Fatalf("itemCount should be 100, got %d", info.ItemCount)
	}
	if info.VisibleStart != 0 || info.VisibleEnd != 9 {
		t.Fatalf("visible range should be 0..9, got %d..%d", info.VisibleStart, info.VisibleEnd)
	}
	if len(info.Children) != 10 {
		t.Fatalf("snapshot should include 10 visible rows, got %d", len(info.Children))
	}
	for _, child := range info.Children {
		if child.Role != RoleListItem {
			t.Fatalf("row role should be listitem, got %q", child.Role)
		}
	}

	// Empty list: no visible range, zero children.
	empty := NewListView()
	info = empty.Snapshot()
	if info.Role != RoleList || info.ItemCount != 0 {
		t.Fatalf("empty list snapshot wrong: %q %d", info.Role, info.ItemCount)
	}
	if info.VisibleStart != 0 || info.VisibleEnd != 0 {
		t.Fatalf("empty list should omit visible range, got %d..%d", info.VisibleStart, info.VisibleEnd)
	}
}

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
