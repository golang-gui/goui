package gui

import (
	"sync"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
)

// mockListWidget is a widget whose measured height is set explicitly
// (standing in for a real content widget; the delegate sets it during Bind).
type mockListWidget struct {
	WidgetBase
	height float32
}

func (m *mockListWidget) Measure(c layout.Constraint) geometry.Size {
	return geometry.Size{Width: c.Min.Width, Height: m.height}
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
}

func (d *mockListDelegate) Unbind(index int, w Widget) {
	d.unbinds = append(d.unbinds, index)
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
	if d.setups != 11 {
		t.Fatalf("scroll down should create 1 new shell, setups=%d", d.setups)
	}

	// Scroll back up: item 0 re-bound from the pool, no new Setup.
	bindsBefore := len(d.binds)
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if len(d.binds)-bindsBefore != 1 || d.setups != 11 {
		t.Fatalf("scroll back should re-bind cached item, binds=%d setups=%d", len(d.binds)-bindsBefore, d.setups)
	}
}

func TestListViewReloadOnModelChange(t *testing.T) {
	model := NewSliceListModel(make([]int, 10))
	lv := NewListView()
	d := newMockListDelegate(nil)
	lv.SetModel(model)
	lv.SetDelegate(d)

	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if d.setups != 10 {
		t.Fatalf("expected 10 setups, got %d", d.setups)
	}

	// Model mutation emits → ListView reloads: all items unbound and rebuilt.
	model.Append(0)
	if len(d.unbinds) != 10 {
		t.Fatalf("reload should unbind all 10 items, got %v", d.unbinds)
	}
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if d.setups != 20 {
		t.Fatalf("reload should rebuild visible items, setups=%d", d.setups)
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
	if got := m.ItemAt(1).(row); got.name != "second" || got.n != 2 {
		t.Fatalf("unexpected item: %+v", got)
	}

	m.Set(0, row{name: "renamed", n: 10})
	if got := m.ItemAt(0).(row); got.name != "renamed" {
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
		v := m.ItemAt(i).(int)
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
	_ ListModel        = (*SliceListModel[int])(nil)
	_ ListModel        = (*SliceListModel[row])(nil)
	_ ListItemDelegate = (*mockListDelegate)(nil)
)
