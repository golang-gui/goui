package gui

import (
	"math"
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
	widths  map[int]float32 // index -> measured row width (natural, >= viewport)

	estimate   float32 // estimated height of unmeasured items
	seedHeight float32 // first measured height after Bind (estimate seed)

	contentWidth float32 // horizontal scroll extent = max measured row width

	modelHandle signal.Handle // model.ConnectItems handle
	reloading   bool          // guards reentrant reload from model events

	scrollY  float32       // last LayoutVisible offset
	scrollX  float32       // last LayoutVisible horizontal offset
	viewport geometry.Size // last LayoutVisible viewport

	first             int     // first visible index (incremental locate cache)
	firstY            float32 // cumulative height before first
	lastContentHeight float32

	lastViewportWidth float32 // cache for invalidating heights on resize
	revision          uint64
	refresh           bool
	revealIndex       int
	revealPending     bool
	revealSignal      signal.Signal1[geometry.Rectangle]
	revealed          signal.Signal1[int]
}

// NewListView returns an empty ListView.
func NewListView() *ListView {
	return &ListView{
		items:   make(map[int]Widget),
		heights: make(map[int]float32),
	}
}

// StyleChanged drops exact row measurements, not the rows or their bindings.
// Keep the previous mean/width as estimates until LayoutVisible remeasures, so
// ScrollView does not clamp its offset against a transient empty extent.
func (lv *ListView) StyleChanged() {
	clear(lv.heights)
	clear(lv.widths)
	lv.seedHeight = lv.estimate
	lv.first, lv.firstY = 0, 0
	lv.lastContentHeight = 0
}

// Model returns the current data model.
func (lv *ListView) Model() ListModel {
	return lv.model
}

// SetModel installs the model, disconnecting the previous subscription and
// reloading the list even when m is the same instance. Nil clears the model.
// Data changes should use the model's Items signal; presentation-only changes
// should use Refresh. Declarative adapters decide whether installation is needed.
func (lv *ListView) SetModel(m ListModel) {
	if lv.Destroyed() {
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

// SetDelegate installs the renderer, unbinding old rows and discarding its
// reuse pool even when d is the same instance. Use Refresh to keep the renderer
// and its row shells while updating their presentation.
func (lv *ListView) SetDelegate(d ListItemDelegate) {
	if lv.Destroyed() {
		return
	}
	lv.detachAll()
	if lv.Destroyed() {
		return
	}
	// Shells belong to their creating delegate, never to its replacement.
	lv.pool = nil
	lv.delegate = d
	lv.reload()
}

// Refresh rebinds realized items on the next layout, preserving the model and
// scroll position. Use it when a delegate's presentation changes.
func (lv *ListView) Refresh() {
	if lv.Destroyed() {
		return
	}
	lv.refresh = true
	lv.RequestLayout()
}

// Reveal requests that index become vertically visible. ScrollView owns the
// actual offset; unmeasured rows use estimates until the target is realized.
func (lv *ListView) Reveal(index int) {
	if lv.Destroyed() || index < 0 || index >= lv.ItemsCount() {
		return
	}
	lv.revealIndex, lv.revealPending = index, true
	lv.RequestLayout()
}

func (lv *ListView) ConnectScrollIntoView(fn func(geometry.Rectangle)) signal.Handle {
	return lv.revealSignal.Connect(func(rect geometry.Rectangle) {
		if !lv.Destroyed() {
			fn(rect)
		}
	})
}

// ConnectRevealed reports completion of a Reveal request after the target was
// measured and reached the viewport. Replacing the model cancels the request
// without emitting completion. It does not report ordinary scrolling.
func (lv *ListView) ConnectRevealed(fn func(int)) signal.Handle {
	return lv.revealed.Connect(func(index int) {
		if !lv.Destroyed() {
			fn(index)
		}
	})
}

func (lv *ListView) revealItem() {
	if !lv.revealPending || lv.Destroyed() || lv.viewport.Height <= 0 || lv.estimate <= 0 {
		return
	}
	index := lv.revealIndex
	if index >= lv.ItemsCount() {
		lv.revealPending = false
		return
	}
	y := float32(0)
	for i := 0; i < index; i++ {
		y += lv.heightAt(i)
	}
	h := lv.heightAt(index)
	row, realized := lv.items[index]
	exact := realized && row.base().measureValid
	visible := y >= lv.scrollY && y+h <= lv.scrollY+lv.viewport.Height
	// ScrollView floors its maximum offset. A fractional final content edge
	// cannot be revealed further; accept only that host-imposed sub-DIP remainder.
	if index == lv.ItemsCount()-1 && y >= lv.scrollY && y+h-lv.scrollY-lv.viewport.Height < 1 {
		visible = true
	}
	if h > lv.viewport.Height {
		visible = float32(math.Floor(float64(y))) == lv.scrollY
	}
	// ScrollView rounds leading/trailing offsets to whole DIP.
	if exact && visible {
		lv.revealPending = false
		lv.revealed.Emit(index)
		return
	}
	lv.revealSignal.Emit(geometry.Rect(lv.scrollX, y, lv.viewport.Width, h))
}

// reload unbinds items and drops index-based measurements. Shells remain
// reusable with the same delegate even when the model's order changes.
func (lv *ListView) reload() {
	if lv.reloading {
		return
	}
	lv.reloading = true
	defer func() { lv.reloading = false }()

	lv.revision++
	lv.revealPending = false
	lv.detachAll()
	lv.heights = make(map[int]float32)
	lv.widths = make(map[int]float32)
	lv.first, lv.firstY = 0, 0
	lv.estimate = lv.seedHeight
	lv.contentWidth = 0
	lv.lastContentHeight = 0
	lv.RequestLayout()
}

// ContentSize implements Scrollable. Height is the exact sum of measured items
// plus an estimate for the rest (it grows as items are measured). Width is the
// horizontal scroll extent: the widest measured row (its natural width, at
// least the viewport width). It only reflects measured rows, so it can grow as
// wider rows are revealed by scrolling; ScrollView re-lays out when it changes.
func (lv *ListView) ContentSize() geometry.Size {
	n := lv.ItemsCount()
	if n == 0 {
		return geometry.Size{}
	}
	total := float32(0)
	for i := 0; i < n; i++ {
		if h, ok := lv.heights[i]; ok {
			total += h
		} else {
			total += lv.estimate
		}
	}
	return geometry.Size{Width: lv.contentWidth, Height: total}
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
	if lv.Destroyed() {
		return
	}
	revision := lv.revision
	lv.viewport = viewport
	lv.scrollY = offset.Y
	lv.scrollX = offset.X

	n := lv.ItemsCount()
	if n == 0 || lv.delegate == nil || viewport.Height <= 0 {
		lv.detachAll()
		return
	}
	if lv.refresh {
		lv.refresh = false
		for _, index := range lv.VisibleIndexes() {
			w := lv.items[index]
			lv.delegate.Bind(index, w)
			if lv.Destroyed() || revision != lv.revision {
				return
			}
			w.RequestLayout()
		}
		// Offscreen measurements remain estimates until those rows bind again.
		// Clearing them here would lose the widest row and clamp horizontal
		// scrolling, even when only the visible presentation changed.
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

	// A viewport width change can rewrap text, so cached heights are stale;
	// re-measure everything. During an unchanged-width scroll the heights stay
	// cached and re-scrolling the same items skips the (expensive text)
	// measurement. Width changes also reset the horizontal scroll extent.
	if lv.lastViewportWidth != viewport.Width {
		lv.lastViewportWidth = viewport.Width
		lv.heights = make(map[int]float32)
		lv.widths = make(map[int]float32)
		lv.estimate = lv.seedHeight
		lv.contentWidth = viewport.Width
		lv.first, lv.firstY = 0, 0
	}

	if len(lv.widths) == 0 {
		lv.contentWidth = viewport.Width
	}
	first, firstY := lv.locateFirst()
	// Recycle old rows before realizing the new viewport, keeping the pool
	// bounded during large jumps. A row taller than the viewport remains valid.
	for _, index := range lv.VisibleIndexes() {
		if index < first {
			w := lv.items[index]
			delete(lv.items, index)
			lv.delegate.Unbind(index, w)
			if lv.Destroyed() || revision != lv.revision {
				return
			}
			lv.WidgetBase.RemoveChild(w)
			lv.pool = append(lv.pool, w)
		}
	}
	last := first
	y := firstY
	for last < n && y < lv.scrollY+viewport.Height {
		w := lv.itemAt(last)
		if lv.Destroyed() || revision != lv.revision {
			return
		}
		if w == nil {
			break
		}
		h, known := lv.heights[last]
		// A local style/text change invalidates the row through RequestLayout
		// even when this list's own style did not change. Do not let the
		// per-index cache hide that invalidation (including a rebound shell).
		if !known || !w.base().measureValid {
			oldWidth := lv.widths[last]
			oldHeight := lv.heightAt(last)
			h = lv.measureItem(last, w)
			lv.heights[last] = h
			if oldHeight != h {
				// A later backwards scroll must not use a prefix computed before
				// these measurements changed (even if their mean is unchanged).
				lv.first, lv.firstY = 0, 0
			}
			if oldWidth == lv.contentWidth && lv.widths[last] < oldWidth {
				// The previous widest row shrank. Other cached rows still count
				// toward the extent, including rows outside the viewport.
				lv.contentWidth = viewport.Width
				for _, width := range lv.widths {
					lv.contentWidth = max(lv.contentWidth, width)
				}
			}
		}
		if lv.Destroyed() || revision != lv.revision {
			return
		}
		rowW := lv.widths[last]
		if rowW <= 0 {
			rowW = viewport.Width
		}
		if rowW > lv.contentWidth {
			lv.contentWidth = rowW
		}
		w.Arrange(geometry.Rect(-lv.scrollX, y-lv.scrollY, rowW, h))
		y += h
		last++
	}
	last-- // inclusive
	var sibling Widget
	for i := last; i >= first; i-- {
		if w := lv.items[i]; w != nil {
			lv.MoveChildBefore(w, sibling)
			sibling = w
		}
	}

	// Detach items outside [first, last]; shells go to the reuse pool.
	for index, w := range lv.items {
		if index < first || index > last {
			delete(lv.items, index)
			lv.delegate.Unbind(index, w)
			if lv.Destroyed() || revision != lv.revision {
				return
			}
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
		estimate := sum / float32(count)
		if estimate != lv.estimate {
			// The cached prefix may include rows still using the old estimate.
			// Otherwise retain the incremental locator for ordinary scrolling.
			lv.first, lv.firstY = 0, 0
			lv.estimate = estimate
		}
	}

	// Request a relayout only when the total extent changed, so ScrollView
	// re-clamps; once heights are cached this stops (no layout storm).
	if h := lv.ContentSize().Height; h != lv.lastContentHeight {
		lv.lastContentHeight = h
		lv.RequestLayout()
	}
	lv.revealItem()
}

// detachAll unbinds and detaches all realized rows using the current delegate.
func (lv *ListView) detachAll() {
	items := lv.items
	lv.items = make(map[int]Widget)
	if lv.delegate != nil {
		for index, w := range items {
			lv.delegate.Unbind(index, w)
			if lv.Destroyed() {
				return
			}
			lv.WidgetBase.RemoveChild(w)
			lv.pool = append(lv.pool, w)
		}
	} else {
		for _, w := range items {
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
	revision := lv.revision
	delegate := lv.delegate
	var w Widget
	if n := len(lv.pool); n > 0 {
		w = lv.pool[n-1]
		lv.pool = lv.pool[:n-1]
	} else {
		w = delegate.Setup()
		if w == nil || lv.Destroyed() || revision != lv.revision {
			return nil
		}
	}
	// Bind first, then measure: the shell is empty (an empty Label measures
	// 0), so the estimate seed must come from a bound item's real height.
	delegate.Bind(index, w)
	if lv.Destroyed() || w.base().destroyed || revision != lv.revision {
		return nil
	}
	w.RequestLayout()
	if lv.seedHeight == 0 {
		lv.seedHeight = lv.measureItem(index, w)
		lv.estimate = lv.seedHeight
	}
	lv.items[index] = w
	lv.WidgetBase.AddChild(lv, w)
	return w
}

// measureItem measures the item at unbounded width and height: an item is laid
// out at its natural width (never wrapped to the viewport), so wide content
// overflows horizontally and is reachable via the horizontal scrollbar. It
// records both the height and the row width (natural width, flushed to at
// least the viewport width), and returns the height.
func (lv *ListView) measureItem(index int, w Widget) float32 {
	if !w.Visible() {
		lv.widths[index] = 0
		return 0
	}
	size := measureWidget(w, layout.Constraint{
		Min: geometry.Size{},
		Max: geometry.Size{Width: layout.Inf, Height: layout.Inf},
	}).Size
	if size.Height <= 0 {
		size.Height = lv.estimate
	}
	if rowW := max(size.Width, lv.viewport.Width); rowW > 0 {
		lv.widths[index] = rowW
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

// Snapshot reports the list role plus the virtualized range (total items and
// the currently visible index span) so AI / automation can understand that
// more rows exist beyond the viewport. The shell widgets hosting the visible
// rows with a generic widget role are reported as listitem. Semantic delegates
// (e.g. menu items and separators) retain their own roles.
func (lv *ListView) Snapshot() WidgetInfo {
	info := lv.WidgetBase.Snapshot()
	info.Role = RoleList
	info.ItemCount = lv.ItemsCount()
	if vis := lv.VisibleIndexes(); len(vis) > 0 {
		info.VisibleStart = vis[0]
		info.VisibleEnd = vis[len(vis)-1]
	}
	for i := range info.Children {
		if info.Children[i].Role == RoleWidget {
			info.Children[i].Role = RoleListItem
		}
	}
	return info
}

// Measure reports the requested viewport size (the list itself is sized by
// its parent; content height comes from ContentSize).
func (lv *ListView) Measure(c layout.Constraint) layout.Measurement {
	if !lv.Visible() {
		return layout.Measurement{}
	}
	return layout.Measured(lv.constrain(c, c.Min))
}
