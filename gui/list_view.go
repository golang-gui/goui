package gui

import (
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
)

// ListItemDelegate renders model items as widgets.
type ListItemDelegate interface {
	// Setup creates a fresh empty item widget. It is called once per pooled
	// widget; reuse only calls Bind again.
	Setup() Widget
	// Bind attaches the data at index to the item widget. It is called on
	// first use and on every scroll-back into view; the widget must reflect
	// the current data afterwards.
	Bind(index int, w Widget)
	// Unbind detaches the item widget from index when it scrolls out of view
	// or the list reloads. It is a lifecycle hook for delegates that connect
	// signals inside Bind; simple delegates may leave it empty.
	Unbind(index int, w Widget)
}

// ListView is a virtualized list: it implements Scrollable and keeps only the
// items visible in the viewport attached to the tree. It must be wrapped in a
// ScrollView, which owns the scroll offset, wheel input and scrollbar.
//
// Data comes from a ListModel and rendering from a ListItemDelegate; the view
// itself only measures, lays out and recycles item widgets. Item heights are
// measured (variable): each item is measured after Bind, so multi-line text
// and images size themselves.
type ListView struct {
	WidgetBase
	model    ListModel
	delegate ListItemDelegate

	items   map[int]Widget  // index -> attached widget (visible only)
	pool    []Widget        // detached shells, ready to be re-bound (reuse pool)
	heights map[int]float32 // index -> measured height (exact after Bind)

	estimate   float32 // estimated height of unmeasured items
	seedHeight float32 // first measured height after Bind (estimate seed)

	modelHandle signal.Handle // model.ConnectItems handle
	reloading   bool          // guards reentrant reload from model events

	scrollY  float32       // last LayoutVisible offset
	viewport geometry.Size // last LayoutVisible viewport

	first             int     // first visible index (incremental locate cache)
	firstY            float32 // cumulative height before first
	lastContentHeight float32
}

// NewListView returns an empty ListView.
func NewListView() *ListView {
	return &ListView{
		items:   make(map[int]Widget),
		heights: make(map[int]float32),
	}
}

// Model returns the current data model.
func (lv *ListView) Model() ListModel {
	return lv.model
}

// SetModel sets the data model, disconnecting the previous one. The list
// reloads immediately.
func (lv *ListView) SetModel(m ListModel) {
	if lv.model == m {
		return
	}
	if lv.modelHandle != nil {
		lv.modelHandle.Disconnect()
		lv.modelHandle = nil
	}
	lv.model = m
	if m != nil {
		lv.modelHandle = m.ConnectItems(lv.reload)
	}
	lv.reload()
}

// Delegate returns the current item renderer.
func (lv *ListView) Delegate() ListItemDelegate {
	return lv.delegate
}

// SetDelegate sets the item renderer. The list reloads immediately.
func (lv *ListView) SetDelegate(d ListItemDelegate) {
	if lv.delegate == d {
		return
	}
	lv.delegate = d
	lv.reload()
}

// reload drops all items and cached heights and requests a relayout.
func (lv *ListView) reload() {
	if lv.reloading {
		return
	}
	lv.reloading = true
	defer func() { lv.reloading = false }()

	lv.detachAll()
	lv.heights = make(map[int]float32)
	lv.pool = nil
	lv.first, lv.firstY = 0, 0
	lv.estimate = lv.seedHeight
	lv.lastContentHeight = 0
	lv.RequestLayout()
}

// ContentSize implements Scrollable: exact heights for measured items plus
// an estimate for the rest. The total grows as items get measured, which
// drives ScrollView to re-clamp and re-proportion the scrollbar.
func (lv *ListView) ContentSize() geometry.Size {
	n := lv.ItemsCount()
	if n == 0 {
		return geometry.Size{Width: lv.Rect().Width}
	}
	total := float32(0)
	for i := 0; i < n; i++ {
		if h, ok := lv.heights[i]; ok {
			total += h
		} else {
			total += lv.estimate
		}
	}
	return geometry.Size{Width: lv.Rect().Width, Height: total}
}

// ItemsCount returns the number of items from the model.
func (lv *ListView) ItemsCount() int {
	if lv.model == nil {
		return 0
	}
	return lv.model.ItemsCount()
}

// LayoutVisible implements Scrollable: waterfall virtualization. It locates
// the first visible index (incrementally from the previous position), binds
// and measures items until the viewport is filled, and unbinds items that
// scrolled out.
func (lv *ListView) LayoutVisible(viewport geometry.Size, offset geometry.Point) {
	lv.viewport = viewport
	lv.scrollY = offset.Y

	n := lv.ItemsCount()
	if n == 0 || lv.delegate == nil || viewport.Height <= 0 {
		lv.detachAll()
		return
	}

	// Guard: drop children that are not registered in the item map (someone
	// bypassed the virtualization by calling AddChild directly).
	for _, c := range lv.Children() {
		registered := false
		for _, w := range lv.items {
			if w == c {
				registered = true
				break
			}
		}
		if !registered {
			lv.WidgetBase.RemoveChild(c)
		}
	}

	first, firstY := lv.locateFirst()
	last := first
	y := firstY
	for last < n && y < lv.scrollY+viewport.Height {
		w := lv.itemAt(last)
		if w == nil {
			break
		}
		h := lv.measureItem(last, w)
		lv.heights[last] = h
		w.Arrange(geometry.Rect(0, y-lv.scrollY, viewport.Width, h))
		y += h
		last++
	}
	last-- // inclusive

	// Detach items outside [first, last]; shells go to the reuse pool.
	for index, w := range lv.items {
		if index < first || index > last {
			lv.delegate.Unbind(index, w)
			lv.WidgetBase.RemoveChild(w)
			delete(lv.items, index)
			lv.pool = append(lv.pool, w)
		}
	}

	// Update estimate as the running mean of measured heights.
	if count := len(lv.heights); count > 0 {
		sum := float32(0)
		for _, h := range lv.heights {
			sum += h
		}
		lv.estimate = sum / float32(count)
	}

	// Request a relayout only when the total extent changed, so ScrollView
	// re-clamps; once heights are cached this stops (no layout storm).
	if h := lv.ContentSize().Height; h != lv.lastContentHeight {
		lv.lastContentHeight = h
		lv.RequestLayout()
	}
}

// detachAll unbinds and detaches every attached item, keeping the shells in
// the reuse pool.
func (lv *ListView) detachAll() {
	if lv.delegate != nil {
		for index, w := range lv.items {
			lv.delegate.Unbind(index, w)
			lv.WidgetBase.RemoveChild(w)
		}
	} else {
		for _, w := range lv.items {
			lv.WidgetBase.RemoveChild(w)
		}
	}
	lv.items = make(map[int]Widget)
	lv.first, lv.firstY = 0, 0
}

// locateFirst finds the first visible index and its cumulative offset,
// adjusting incrementally from the previous position. Heights are exact for
// measured items and estimated for the rest, so a jump across unmeasured
// regions may be off by an estimate page on the first frame; the layout loop
// below measures everything visible, so the next frame is exact.
func (lv *ListView) locateFirst() (int, float32) {
	n := lv.ItemsCount()
	if n == 0 {
		return 0, 0
	}
	first, y := lv.first, lv.firstY
	if lv.scrollY >= y {
		for first < n-1 {
			h := lv.heightAt(first)
			if h <= 0 {
				break // no seed yet (nothing measured, estimate unknown)
			}
			if y+h > lv.scrollY {
				break
			}
			y += h
			first++
		}
	} else {
		for first > 0 && y > lv.scrollY {
			first--
			y -= lv.heightAt(first)
		}
	}
	lv.first, lv.firstY = first, y
	return first, y
}

// heightAt returns the exact height if measured, otherwise the estimate.
func (lv *ListView) heightAt(index int) float32 {
	if h, ok := lv.heights[index]; ok {
		return h
	}
	return lv.estimate
}

// itemAt returns the widget for index, binding a pooled or fresh widget.
func (lv *ListView) itemAt(index int) Widget {
	if w, ok := lv.items[index]; ok {
		return w
	}
	if lv.delegate == nil {
		return nil
	}
	var w Widget
	if n := len(lv.pool); n > 0 {
		w = lv.pool[n-1]
		lv.pool = lv.pool[:n-1]
	} else {
		w = lv.delegate.Setup()
		if w == nil {
			return nil
		}
	}
	// Bind first, then measure: the shell is empty (an empty Label measures
	// 0), so the estimate seed must come from a bound item's real height.
	lv.delegate.Bind(index, w)
	if lv.seedHeight == 0 {
		lv.seedHeight = lv.measureItem(index, w)
		lv.estimate = lv.seedHeight
	}
	lv.items[index] = w
	lv.WidgetBase.AddChild(lv, w)
	return w
}

// measureItem measures the item with the viewport width and unbounded height.
func (lv *ListView) measureItem(index int, w Widget) float32 {
	// ignore index: measurement only depends on the widget state
	size := w.Measure(layout.Constraint{
		Min: geometry.Size{},
		Max: geometry.Size{Width: lv.viewport.Width, Height: layout.Inf},
	})
	if size.Height <= 0 {
		size.Height = lv.estimate
	}
	return size.Height
}

// VisibleIndexes returns the currently attached item indices in ascending
// order (for tests and snapshotting).
func (lv *ListView) VisibleIndexes() []int {
	idx := make([]int, 0, len(lv.items))
	for i := range lv.items {
		idx = append(idx, i)
	}
	slices.Sort(idx)
	return idx
}

// Measure reports the requested viewport size (the list itself is sized by
// its parent; content height comes from ContentSize).
func (lv *ListView) Measure(c layout.Constraint) geometry.Size {
	if !lv.Visible() {
		return geometry.Size{}
	}
	return lv.constrain(c, c.Min)
}
