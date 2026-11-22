package widgets

import (
	"math"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

const (
	RoleTable        gui.Role = "table"
	RoleColumnHeader gui.Role = "columnheader"
	RoleTableRow     gui.Role = "row"
	RoleTableCell    gui.Role = "cell"
)

// TableView is a record table with a fixed header and an internal, dual-axis
// ScrollView. It realizes only visible rows. Methods are GUI-thread-affine.
type TableView struct {
	gui.WidgetBase
	model                         TableModel
	modelHandle                   signal.Handle
	columns                       []*TableColumn
	visibleColumns                []*TableColumn
	columnX                       []float32
	columnWidth                   float32
	columnViewportWidth           float32
	columnViewportKnown           bool
	columnLayoutSize              geometry.Size
	columnResize                  *tableColumnResizeState
	rowIDs                        []string
	indices                       map[string]int
	selected                      map[string]bool
	current, anchor               string
	mode                          SelectionMode
	sortColumn                    string
	sortOrder                     SortOrder
	list                          *gui.ListView
	body                          *tableBody
	scroll                        *gui.ScrollView
	headerViewport                *gui.WidgetBase
	headerRow                     *gui.WidgetBase
	headers                       map[*TableColumn]*tableHeader
	realized                      map[string]*tableRow
	revision                      uint64
	synchronizing, syncPending    bool
	items                         signal.Signal1[gui.ListChange]
	selection                     signal.Signal2[[]string, uint64]
	currentSignal                 signal.Signal2[string, uint64]
	activate                      signal.Signal2[string, uint64]
	sortRequest                   signal.Signal3[string, SortOrder, uint64]
	menuQuery                     signal.Signal2[tableMenuQuery, uint64]
	menuError                     signal.Signal1[error]
	menu                          *gui.PopoverMenu
	menuID, menuPending, revealID string
}

func NewTableView() *TableView {
	v := &TableView{indices: make(map[string]int), selected: make(map[string]bool),
		headers: make(map[*TableColumn]*tableHeader), realized: make(map[string]*tableRow)}
	v.SetFocusable(true)
	v.SetMainWeight(1)
	v.headerViewport, v.headerRow = new(gui.WidgetBase), new(gui.WidgetBase)
	v.headerViewport.SetLayoutManager(layout.NewFillLayout())
	v.headerViewport.AddChild(v.headerViewport, v.headerRow)
	v.headerRow.SetLayoutManager(&tableHeaderLayout{view: v})
	v.list = gui.NewListView()
	v.list.SetModel(&tableListModel{view: v})
	v.list.SetDelegate(&tableListDelegate{view: v})
	v.body = &tableBody{view: v}
	v.body.WidgetBase.AddChild(v.body, v.list)
	v.scroll = gui.NewScrollView()
	v.scroll.SetStyleName(v.styleName())
	v.scroll.SetChild(v.body)
	v.WidgetBase.AddChild(v, v.headerViewport)
	v.WidgetBase.AddChild(v, v.scroll)
	v.SetLayoutManager(&tableLayout{view: v})
	keys := gui.NewKeyEventController()
	keys.SetPhase(gui.PhaseTarget)
	keys.ConnectKeyDown(v.keyDown)
	v.AddEventController(keys)
	blank := gui.NewClickEventController()
	blank.ConnectClicked(func(ctx gui.EventContext, _ gui.ClickInfo) {
		ctx.StopPropagation()
		v.SetSelection(nil)
	})
	v.body.AddEventController(blank)
	menu := gui.NewContextMenuEventController()
	menu.ConnectRequest(v.contextMenuRequested)
	v.body.AddEventController(menu)
	// Keyboard menu requests target the focused table, not its body.
	keyboardMenu := gui.NewContextMenuEventController()
	keyboardMenu.SetPhase(gui.PhaseTarget)
	keyboardMenu.ConnectRequest(func(ctx gui.EventContext, inputInfo gui.ContextMenuInfo) {
		if !inputInfo.HasPosition {
			v.contextMenuRequested(ctx, inputInfo)
		}
	})
	v.AddEventController(keyboardMenu)
	v.list.ConnectRevealed(func(index int) {
		if target, ok := v.indices[v.revealID]; ok && target == index {
			v.revealID = ""
		}
	})
	v.ConnectFocused(func(bool) { v.RequestPaint() })
	v.ConnectMount(func() {
		if v.model != nil && v.modelHandle == nil {
			v.connectModel()
			v.modelChanged()
		}
		v.Refresh()
	})
	v.ConnectUnmount(func() {
		v.revision++
		v.invalidateColumnResize()
		v.revealID = ""
		if v.modelHandle != nil {
			v.modelHandle.Disconnect()
			v.modelHandle = nil
		}
		v.closeMenu()
		if v.menu != nil {
			v.menu.SetMenu(nil)
			v.menu = nil
		}
		if v.Destroyed() {
			for _, c := range v.columns {
				if c.owner == v {
					c.owner = nil
				}
			}
		}
	})
	return v
}

// HScrollBarOptions returns the normalized built-in horizontal bar options.
func (v *TableView) HScrollBarOptions() gui.ScrollBarOptions {
	return v.scroll.HScrollBarOptions()
}

// SetHScrollBarOptions configures horizontal bar layout without changing column
// base widths or emitting user resize notifications.
func (v *TableView) SetHScrollBarOptions(options gui.ScrollBarOptions) {
	before := v.scroll.HScrollBarOptions()
	v.scroll.SetHScrollBarOptions(options)
	if !v.Destroyed() && before != v.scroll.HScrollBarOptions() {
		v.Refresh()
	}
}

// BindScroll associates an external client with the table's scrolling body.
// The table retains ownership of its internal viewport; the external client is
// independently laid out and is not included in the table's borders or outline.
func (v *TableView) BindScroll(axis layout.Direction, client gui.ScrollClient) signal.Handle {
	if v.Destroyed() {
		return nil
	}
	return v.scroll.BindScroll(axis, client)
}

// VScrollBarOptions returns the normalized built-in vertical bar options.
func (v *TableView) VScrollBarOptions() gui.ScrollBarOptions {
	return v.scroll.VScrollBarOptions()
}

// SetVScrollBarOptions configures vertical bar layout without changing column
// base widths or emitting user resize notifications.
func (v *TableView) SetVScrollBarOptions(options gui.ScrollBarOptions) {
	before := v.scroll.VScrollBarOptions()
	v.scroll.SetVScrollBarOptions(options)
	if !v.Destroyed() && before != v.scroll.VScrollBarOptions() {
		v.Refresh()
	}
}

// SetStyleName selects this table's appearance, including its owned parts.
// Application-provided cell contents retain their own style names.
func (v *TableView) SetStyleName(name string) {
	if v.Destroyed() || v.StyleName() == name {
		return
	}
	v.WidgetBase.SetStyleName(name)
	v.scroll.SetStyleName(v.styleName())
	for _, h := range v.headers {
		h.label.SetStyleName(v.styleName())
		h.RequestLayout()
	}
	v.headerRow.RequestLayout()
	for _, row := range v.realized {
		for _, cell := range row.cells {
			cell.RequestLayout()
		}
		row.RequestLayout()
	}
}

type tableListModel struct{ view *TableView }

func (m *tableListModel) ItemsCount() int {
	if len(m.view.visibleColumns) == 0 {
		return 0
	}
	return len(m.view.rowIDs)
}
func (m *tableListModel) ConnectItems(fn func()) signal.Handle {
	return m.view.items.Connect(func(gui.ListChange) { fn() })
}
func (m *tableListModel) ConnectItemsChanged(fn func(gui.ListChange)) signal.Handle {
	return m.view.items.Connect(fn)
}

func (v *TableView) Model() TableModel { return v.model }

// SetModel always reinstalls the subscription and resets interaction/scroll
// state, even for the same object. Columns remain. Nil clears the records.
func (v *TableView) SetModel(model TableModel) {
	if v.Destroyed() {
		return
	}
	if v.modelHandle != nil {
		v.modelHandle.Disconnect()
		v.modelHandle = nil
	}
	oldSelection, oldCurrent := v.Selection(), v.current
	v.revision++
	v.closeMenu()
	v.model = model
	clear(v.selected)
	v.current, v.anchor, v.revealID = "", "", ""
	v.SetSort("", SortNone)
	v.reindex()
	v.connectModel()
	version := v.revision
	v.items.Emit(gui.ListChange{Reset: true})
	if !v.valid(version) {
		return
	}
	v.scroll.SetScrollX(0)
	v.scroll.SetScrollY(0)
	v.notify(oldSelection, oldCurrent)
}
func (v *TableView) connectModel() {
	if v.model != nil && v.modelHandle == nil {
		v.modelHandle = v.model.ConnectItems(func() {
			if !v.Destroyed() {
				v.modelChanged()
			}
		})
	}
}
func (v *TableView) reindex() {
	oldLast := ""
	if len(v.rowIDs) > 0 {
		oldLast = v.rowIDs[len(v.rowIDs)-1]
	}
	wasEmpty := len(v.rowIDs) == 0
	ids := []string(nil)
	indices := make(map[string]int)
	if v.model != nil {
		for i := 0; i < v.model.ItemsCount(); i++ {
			id := v.model.RowID(i)
			if id == "" {
				panic("widgets: empty table row ID")
			}
			if _, exists := indices[id]; exists {
				panic("widgets: duplicate table row ID")
			}
			indices[id] = i
			ids = append(ids, id)
		}
	}
	v.rowIDs, v.indices = ids, indices
	if wasEmpty != (len(ids) == 0) {
		// An empty header owns the data bottom outline instead of an internal
		// header separator. Its measurement can therefore change with the model.
		for _, header := range v.headers {
			header.RequestLayout()
		}
	}
	newLast := ""
	if len(ids) > 0 {
		newLast = ids[len(ids)-1]
	}
	if oldLast != newLast {
		// Keep incremental row identities while remeasuring the two live rows
		// whose trailing band changes between an internal line and an outline.
		for _, id := range []string{oldLast, newLast} {
			if row := v.realized[id]; row != nil {
				row.RequestLayout()
			}
		}
	}
}
func (v *TableView) modelChanged() {
	v.revision++
	if v.synchronizing {
		v.syncPending = true
		return
	}
	v.synchronizing = true
	defer func() { v.synchronizing = false }()
	for {
		v.syncPending = false
		oldIDs, oldSelection, oldCurrent := v.rowIDs, v.Selection(), v.current
		oldIndex := v.indices[oldCurrent]
		wasSelected := v.selected[oldCurrent]
		v.reindex()
		for id := range v.selected {
			if _, ok := v.indices[id]; !ok {
				delete(v.selected, id)
			}
		}
		if _, ok := v.indices[oldCurrent]; oldCurrent != "" && !ok {
			v.current = ""
			if len(v.rowIDs) > 0 {
				v.current = v.rowIDs[min(oldIndex, len(v.rowIDs)-1)]
				if wasSelected {
					if v.mode == SelectionSingle {
						clear(v.selected)
					}
					v.selected[v.current] = true
				}
			}
		}
		if _, ok := v.indices[v.anchor]; !ok {
			v.anchor = v.current
		}
		v.closeMenu()
		version := v.revision
		if slices.Equal(oldIDs, v.rowIDs) {
			v.Refresh()
		} else {
			v.items.Emit(tableRowChange(oldIDs, v.rowIDs))
		}
		if v.valid(version) {
			if index, ok := v.indices[v.revealID]; ok {
				v.list.Reveal(index)
			} else {
				v.revealID = ""
			}
			v.notify(oldSelection, oldCurrent)
		}
		if v.Destroyed() || !v.syncPending {
			return
		}
	}
}

// Retain the unchanged prefix and suffix. A general reorder may replace the
// middle, but inserting or removing records preserves all other editors.
func tableRowChange(before, after []string) gui.ListChange {
	start := 0
	for start < len(before) && start < len(after) && before[start] == after[start] {
		start++
	}
	oldEnd, newEnd := len(before), len(after)
	for oldEnd > start && newEnd > start && before[oldEnd-1] == after[newEnd-1] {
		oldEnd--
		newEnd--
	}
	return gui.ListChange{Position: start, Removed: oldEnd - start, Added: newEnd - start}
}

// Columns returns all configured columns in display order, including hidden ones.
func (v *TableView) Columns() []*TableColumn { return slices.Clone(v.columns) }

// SetColumns validates the whole collection before changing ownership. Reused
// column objects retain their content; replacing an ID with a new object does not.
func (v *TableView) SetColumns(columns ...*TableColumn) {
	if v.Destroyed() {
		return
	}
	seen := make(map[string]bool)
	for _, c := range columns {
		if c == nil || c.id == "" || seen[c.id] {
			panic("widgets: invalid or duplicate table column")
		}
		if c.owner != nil && c.owner != v && !c.owner.Destroyed() {
			panic("widgets: table column already owned")
		}
		seen[c.id] = true
	}
	if slices.Equal(v.columns, columns) {
		return
	}
	v.revision++
	v.closeMenu()
	version := v.revision
	// Rollback can call application code. Finish every cancellation before
	// changing the collection, so a reentrant update cannot leave half-owned columns.
	for _, c := range slices.Clone(v.columns) {
		if !slices.Contains(columns, c) {
			v.headers[c].handle.cancel()
			if !v.valid(version) {
				return
			}
		}
	}
	for _, c := range v.columns {
		if !slices.Contains(columns, c) {
			v.headerRow.RemoveChild(v.headers[c])
			delete(v.headers, c)
			c.owner = nil
		}
	}
	wasEmpty := len(v.visibleColumns) == 0
	v.columns = slices.Clone(columns)
	v.updateColumnGeometry()
	for _, c := range v.columns {
		c.owner = v
		if v.headers[c] == nil {
			h := newTableHeader(v, c)
			v.headers[c] = h
			v.headerRow.AddChild(v.headerRow, h)
		}
		v.headerRow.MoveChildBefore(v.headers[c], nil)
		v.headers[c].SetVisible(c.visible)
		if !v.valid(version) {
			return
		}
	}
	for _, row := range v.realized {
		row.syncCells()
		if !v.valid(version) {
			return
		}
	}
	if c := v.column(v.sortColumn); c == nil || !c.sortable {
		v.SetSort("", SortNone)
	}
	v.columnsChanged()
	if wasEmpty || len(v.visibleColumns) == 0 {
		v.items.Emit(gui.ListChange{Reset: true})
	}
}

func (v *TableView) setColumnVisible(c *TableColumn, visible bool) {
	v.revision++
	version := v.revision
	header := v.headers[c]
	if !visible {
		// Cancellation can emit Resize and replace the column collection.
		header.handle.cancel()
		if !v.valid(version) || c.owner != v {
			return
		}
		tableRelinquishFocus(header, v)
		if !v.valid(version) || c.owner != v {
			return
		}
	}
	wasEmpty := len(v.visibleColumns) == 0
	c.visible = visible
	v.closeMenu()
	v.updateColumnGeometry()
	header.SetVisible(visible)
	if !v.valid(version) {
		return
	}
	for _, row := range v.realized {
		row.syncCells()
		if !v.valid(version) {
			return
		}
	}
	v.columnsChanged()
	if wasEmpty || len(v.visibleColumns) == 0 {
		v.items.Emit(gui.ListChange{Reset: true})
	}
}
func (v *TableView) column(id string) *TableColumn {
	for _, c := range v.columns {
		if c.id == id {
			return c
		}
	}
	return nil
}
func (v *TableView) columnsChanged() {
	v.updateColumnGeometry()
	v.updateHeaders()
	v.Refresh()
}
func (v *TableView) updateColumnGeometry() {
	v.invalidateColumnResize()
	v.visibleColumns = v.visibleColumns[:0]
	for _, c := range v.columns {
		if c.visible {
			v.visibleColumns = append(v.visibleColumns, c)
		}
	}
	v.columnX = make([]float32, len(v.visibleColumns))
	width := float64(0)
	for _, c := range v.visibleColumns {
		c.allocatedWidth = c.width
		width += float64(c.width)
	}
	remaining := max(0, float64(v.columnViewportWidth)-width)
	if v.columnViewportKnown && width > float64(v.columnViewportWidth) {
		tableShrinkColumns(v.visibleColumns, width-float64(v.columnViewportWidth))
	}
	tableGrowColumns(v.visibleColumns, remaining, false)
	v.updateColumnPositions()
}

// Grow towards base preferences first, or distribute surplus to expanding columns.
func tableGrowColumns(columns []*TableColumn, remaining float64, restore bool) float64 {
	limit := func(c *TableColumn) float64 {
		if restore {
			return float64(c.width)
		}
		if c.maxWidth > 0 {
			return float64(max(c.minWidth, c.maxWidth))
		}
		return math.MaxFloat32
	}
	var expanding []*TableColumn
	for _, c := range columns {
		if (restore || c.expand) && float64(c.allocatedWidth) < limit(c) {
			expanding = append(expanding, c)
		}
	}
	for len(expanding) > 0 && remaining > 0 {
		share := remaining / float64(len(expanding))
		used := float64(0)
		next := expanding[:0]
		for _, c := range expanding {
			target := min(float64(c.allocatedWidth)+share, limit(c))
			allocated := float32(target)
			// Round shares down: a subpixel rounding excess must not create
			// a horizontal scrollbar when the columns fit the viewport.
			if float64(allocated) > target {
				allocated = math.Nextafter32(allocated, 0)
			}
			used += float64(allocated) - float64(c.allocatedWidth)
			c.allocatedWidth = allocated
			if float64(c.allocatedWidth) < limit(c) {
				next = append(next, c)
			}
		}
		remaining = max(0, remaining-used)
		// Without a newly capped column, another pass only redistributes
		// floating-point residue; no column can make meaningful progress.
		if len(next) == len(expanding) || used <= 0 {
			break
		}
		expanding = next
	}
	return remaining
}

func (v *TableView) updateColumnPositions() {
	width := float64(0)
	for i, c := range v.visibleColumns {
		v.columnX[i] = float32(width)
		width += float64(c.allocatedWidth)
	}
	v.columnWidth = float32(width)
}

// Shrink equally from current allocations, redistributing a capped column's share.
// The base preferences and user-resize signals are never changed by allocation.
func tableShrinkColumns(columns []*TableColumn, deficit float64) {
	var shrinking []*TableColumn
	for _, c := range columns {
		if c.allocatedWidth > c.minWidth {
			shrinking = append(shrinking, c)
		}
	}
	for len(shrinking) > 0 && deficit > 0 {
		share := deficit / float64(len(shrinking))
		used := float64(0)
		next := shrinking[:0]
		for _, c := range shrinking {
			target := max(float64(c.minWidth), float64(c.allocatedWidth)-share)
			allocated := float32(target)
			// Keep the sum within a fractional viewport without crossing MinWidth.
			if float64(allocated) > target {
				allocated = math.Nextafter32(allocated, 0)
			}
			allocated = max(c.minWidth, allocated)
			used += float64(c.allocatedWidth) - float64(allocated)
			c.allocatedWidth = allocated
			if allocated > c.minWidth {
				next = append(next, c)
			}
		}
		deficit = max(0, deficit-used)
		if len(next) == len(shrinking) || used <= 0 {
			break
		}
		shrinking = next
	}
}

func (v *TableView) setColumnViewportWidth(width float32) {
	if v.columnViewportKnown && v.columnViewportWidth == width {
		return
	}
	old := make([]float32, len(v.visibleColumns))
	for i, c := range v.visibleColumns {
		old[i] = c.allocatedWidth
	}
	v.columnViewportWidth = width
	v.columnViewportKnown = true
	if v.columnResize != nil {
		v.columnResize.allocate(v)
	} else {
		v.updateColumnGeometry()
	}
	for i, c := range v.visibleColumns {
		if old[i] != c.allocatedWidth {
			v.headerRow.RequestLayout()
			v.list.Refresh()
			return
		}
	}
}
func (v *TableView) updateHeaders() {
	if v.Destroyed() {
		return
	}
	for _, c := range v.columns {
		v.headers[c].update()
	}
	v.headerRow.RequestLayout()
	v.RequestLayout()
	v.RequestPaint()
}
func (v *TableView) reinstallRows() {
	if v.Destroyed() {
		return
	}
	v.revision++
	v.closeMenu()
	v.list.SetDelegate(&tableListDelegate{view: v})
	v.RequestLayout()
}

func (v *TableView) SelectionMode() SelectionMode { return v.mode }
func (v *TableView) SetSelectionMode(mode SelectionMode) {
	if v.Destroyed() || mode > SelectionMultiple || v.mode == mode {
		return
	}
	v.mode = mode
	if ids := v.Selection(); mode == SelectionSingle && len(ids) > 1 {
		v.SetSelection(ids[:1])
	}
}

// Selection returns a copy in current model order.
func (v *TableView) Selection() []string {
	var ids []string
	for _, id := range v.rowIDs {
		if v.selected[id] {
			ids = append(ids, id)
		}
	}
	return ids
}
func (v *TableView) SetSelection(ids []string) {
	if v.Destroyed() {
		return
	}
	next := make(map[string]bool)
	for _, id := range ids {
		if _, exists := v.indices[id]; exists {
			next[id] = true
			if v.mode == SelectionSingle {
				break
			}
		}
	}
	if treeEqualSet(next, v.selected) {
		return
	}
	oldSelection, oldCurrent := v.Selection(), v.current
	v.selected = next
	v.notify(oldSelection, oldCurrent)
}
func (v *TableView) Current() string { return v.current }

// SetCurrent does not change selection or scroll. Empty clears Current.
func (v *TableView) SetCurrent(id string) {
	if v.Destroyed() {
		return
	}
	if _, exists := v.indices[id]; id != "" && !exists {
		return
	}
	if v.current == id {
		return
	}
	old := v.current
	v.current = id
	v.notify(v.Selection(), old)
}
func (v *TableView) Reveal(id string) {
	if i, ok := v.indices[id]; ok && !v.Destroyed() {
		v.revealID = id
		v.list.Reveal(i)
	}
}

// Refresh rebinds visible cells, retaining IDs, columns, scroll and row shells.
func (v *TableView) Refresh() {
	if !v.Destroyed() {
		v.list.Refresh()
		v.RequestLayout()
	}
}
func (v *TableView) valid(version uint64) bool { return !v.Destroyed() && v.revision == version }
func (v *TableView) selectionMatches(ids []string) bool {
	if len(ids) != len(v.selected) {
		return false
	}
	for _, id := range ids {
		if !v.selected[id] {
			return false
		}
	}
	return true
}
func (v *TableView) notify(oldSelection []string, oldCurrent string) {
	if v.Destroyed() {
		return
	}
	version, selection, current := v.revision, v.Selection(), v.current
	for id, row := range v.realized {
		row.SetState(style.Selected, v.selected[id])
		for _, cell := range row.cells {
			cell.SetState(style.Selected, v.selected[id])
		}
	}
	v.RequestLayout()
	v.RequestPaint()
	// A model reorder alone must not emit a selection change.
	if !v.selectionMatches(oldSelection) {
		v.selection.Emit(selection, version)
	}
	if v.valid(version) && current == v.current && current != oldCurrent {
		v.currentSignal.Emit(current, version)
	}
}
func (v *TableView) ConnectSelection(fn func([]string)) signal.Handle {
	return v.selection.Connect(func(ids []string, version uint64) {
		if v.valid(version) && v.selectionMatches(ids) {
			fn(slices.Clone(ids))
		}
	})
}
func (v *TableView) ConnectCurrent(fn func(string)) signal.Handle {
	return v.currentSignal.Connect(func(id string, version uint64) {
		if v.valid(version) && id == v.current {
			fn(id)
		}
	})
}
func (v *TableView) ConnectActivate(fn func(string)) signal.Handle {
	return v.activate.Connect(func(id string, version uint64) {
		if v.valid(version) {
			fn(id)
		}
	})
}
func (v *TableView) Sort() (string, SortOrder) { return v.sortColumn, v.sortOrder }

// SetSort records confirmed application state; the view never sorts the model.
func (v *TableView) SetSort(id string, order SortOrder) {
	if v.Destroyed() || order > SortDescending {
		return
	}
	if order == SortNone {
		id = ""
	} else if c := v.column(id); c == nil || !c.sortable {
		return
	}
	v.sortColumn, v.sortOrder = id, order
	v.updateHeaders()
}
func (v *TableView) ConnectSortRequest(fn func(string, SortOrder)) signal.Handle {
	return v.sortRequest.Connect(func(id string, order SortOrder, version uint64) {
		if c := v.column(id); v.valid(version) && c != nil && c.sortable {
			fn(id, order)
		}
	})
}
func (v *TableView) requestSort(c *TableColumn) {
	if v.Destroyed() || c.owner != v || !c.sortable {
		return
	}
	order := SortAscending
	if v.sortColumn == c.id && v.sortOrder == SortAscending {
		order = SortDescending
	}
	v.sortRequest.Emit(c.id, order, v.revision)
}
func (v *TableView) rowState(index int) TableRow {
	id := v.rowIDs[index]
	return TableRow{ID: id, Index: index, Selected: v.selected[id], Current: v.current == id}
}
func (v *TableView) Paint(p gui.Painter) {
	name := v.styleName()
	r := geometry.Rect(0, 0, v.Rect().Width, v.Rect().Height)
	s := gui.ResolveWidgetStyle(v, name, "", style.Normal)
	if bg, ok := s.BackgroundColor(); ok && bg != nil {
		p.FillRect(r, graphics.ColorOf(bg))
	}
	v.paintHeaderScrollGap(p)
	v.paintOutline(p)
	h, w := v.borderWidths()
	h, w = min(h, r.Height/2), min(w, r.Width/2)
	horizontal := gui.ResolveWidgetStyle(v, name, "outer-horizontal", style.Normal)
	vertical := gui.ResolveWidgetStyle(v, name, "outer-vertical", style.Normal)
	tablePaintLine(p, geometry.Rect(0, 0, r.Width, h), horizontal)
	tablePaintLine(p, geometry.Rect(0, r.Height-h, r.Width, h), horizontal)
	// Horizontal sides own the corners, including translucent intersections.
	tablePaintLine(p, geometry.Rect(0, h, w, max(0, r.Height-2*h)), vertical)
	tablePaintLine(p, geometry.Rect(r.Width-w, h, w, max(0, r.Height-2*h)), vertical)
}
func (v *TableView) Snapshot() gui.WidgetInfo {
	info := v.WidgetBase.Snapshot()
	info.Role, info.ItemCount = RoleTable, len(v.rowIDs)
	info.SetAttribute(TableInfoKey, TableInfo{RowCount: len(v.rowIDs), ColumnCount: len(v.visibleColumns)})
	scroll, list := v.scroll.Snapshot(), v.list.Snapshot()
	info.ScrollX, info.ScrollY, info.MaxScrollX, info.MaxScrollY = scroll.ScrollX, scroll.ScrollY, scroll.MaxScrollX, scroll.MaxScrollY
	info.VisibleStart, info.VisibleEnd = list.VisibleStart, list.VisibleEnd
	info.Children = nil
	for _, c := range v.visibleColumns {
		info.Children = append(info.Children, v.headers[c].Snapshot())
	}
	info.Children = append(info.Children, list.Children...)
	return info
}
