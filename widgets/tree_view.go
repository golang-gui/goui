package widgets

import (
	"math"
	"slices"
	"sort"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

// SelectionMode controls row selection, not keyboard focus or Current.
type SelectionMode uint8

const (
	SelectionSingle SelectionMode = iota
	SelectionMultiple
)
const (
	RoleTree     gui.Role = "tree"
	RoleTreeItem gui.Role = "treeitem"
)

// TreeRow is a snapshot of one binding. Depth is zero-based. Durable data
// belongs to the model, since the content Widget may be recycled offscreen.
type TreeRow struct {
	ID                                      string
	Depth                                   int
	Expandable, Expanded, Selected, Current bool
}

// TreeItemDelegate constructs full-row content. Compose a TreeExpander for
// indentation/disclosure; the tree owns selection and row input. Unbind receives
// the last bound snapshot. Widgets may be recycled; retain node IDs, not rows.
type TreeItemDelegate interface {
	Setup() gui.Widget
	Bind(TreeRow, gui.Widget)
	Unbind(TreeRow, gui.Widget)
}
type flatTreeRow struct {
	id, parent                string
	depth, position, siblings int
}

// TreeView displays a virtualized data tree inside a ScrollView. State uses
// model-local node IDs and is independent for each view sharing the model.
// All methods and callbacks execute on the GUI thread.
type TreeView struct {
	gui.WidgetBase
	model                         TreeModel
	delegate                      TreeItemDelegate
	modelHandle                   signal.Handle
	list                          *gui.ListView
	rows                          []flatTreeRow
	indices                       map[string]int
	expanded, selected            map[string]bool
	current, anchor               string
	mode                          SelectionMode
	indentation                   float32
	realized                      map[string]*treeRow
	loads                         map[string]bool
	revision                      uint64 // lifecycle/model/binding generation, not selection/current
	synchronizing, syncPending    bool
	items                         signal.Signal0
	selection                     signal.Signal2[[]string, uint64]
	currentSignal                 signal.Signal2[string, uint64]
	expansion                     signal.Signal3[string, bool, uint64]
	activate                      signal.Signal2[string, uint64]
	revealSignal                  signal.Signal1[geometry.Rectangle]
	revealID, menuPending, menuID string
	menu                          *gui.PopoverMenu
	menuQuery                     signal.Signal3[string, *gui.MenuModel, uint64]
	menuError                     signal.Signal1[error]
}

func NewTreeView() *TreeView {
	v := &TreeView{
		indices: make(map[string]int), expanded: make(map[string]bool), selected: make(map[string]bool),
		realized: make(map[string]*treeRow), loads: make(map[string]bool), indentation: 16,
	}
	v.SetFocusable(true)
	v.SetMainWeight(1)
	v.SetLayoutManager(layout.NewFillLayout())
	v.list = gui.NewListView()
	v.list.SetModel(&treeListModel{view: v})
	v.list.SetDelegate(&treeListDelegate{view: v})
	v.WidgetBase.AddChild(v, v.list)
	v.list.ConnectScrollIntoView(func(rect geometry.Rectangle) {
		if !v.Destroyed() {
			v.revealSignal.Emit(rect)
		}
	})
	v.list.ConnectRevealed(func(index int) {
		if target, ok := v.indices[v.revealID]; ok && target == index {
			v.revealID = ""
		}
	})
	v.ConnectMount(func() {
		v.revision++
		if v.model != nil && v.modelHandle == nil {
			v.modelHandle = v.model.ConnectItems(func() {
				if !v.Destroyed() {
					v.modelChanged()
				}
			})
			v.modelChanged()
		}
		v.requestChildren()
	})
	v.ConnectUnmount(func() {
		v.revision++
		if v.modelHandle != nil {
			v.modelHandle.Disconnect()
			v.modelHandle = nil
		}
		clear(v.loads)
		v.revealID, v.menuPending = "", ""
		v.closeMenu()
	})
	v.ConnectFocused(func(bool) { v.RequestPaint() })
	keys := gui.NewKeyEventController()
	keys.ConnectKeyDown(v.keyDown)
	v.AddEventController(keys)
	blank := gui.NewClickEventController()
	blank.ConnectClicked(func(gui.EventContext) { v.SetSelection(nil) })
	v.AddEventController(blank)
	menu := gui.NewContextMenuEventController()
	menu.ConnectRequest(v.contextMenuRequested)
	v.AddEventController(menu)
	return v
}

type treeListModel struct{ view *TreeView }

func (m *treeListModel) ItemsCount() int                      { return len(m.view.rows) }
func (m *treeListModel) ConnectItems(fn func()) signal.Handle { return m.view.items.Connect(fn) }

func (v *TreeView) Model() TreeModel { return v.model }

// SetModel installs the model and reconnects its Items subscription, even for
// the same instance. It resets expansion, selection, Current and pending Reveal,
// and closes the menu. Nil clears the model. Use the model's Items signal for
// data changes, or Refresh for presentation changes without resetting state.
func (v *TreeView) SetModel(model TreeModel) {
	if v.Destroyed() {
		return
	}
	if v.modelHandle != nil {
		v.modelHandle.Disconnect()
		v.modelHandle = nil
	}
	v.closeMenu()
	oldSelection, oldCurrent := v.Selection(), v.current
	v.model = model
	clear(v.selected)
	clear(v.expanded)
	clear(v.loads)
	v.current, v.anchor, v.revealID = "", "", ""
	v.revision++
	if model != nil {
		v.modelHandle = model.ConnectItems(func() {
			if !v.Destroyed() {
				v.modelChanged()
			}
		})
	}
	v.flatten()
	v.items.Emit()
	v.notify(nil, oldSelection, oldCurrent)
}
func (v *TreeView) Delegate() TreeItemDelegate { return v.delegate }

// SetDelegate reinstalls row content even for the same instance, unbinding old
// rows and discarding their shells. Expansion, selection and Current are kept.
// Use Refresh to update presentation while retaining the current row shells.
func (v *TreeView) SetDelegate(delegate TreeItemDelegate) {
	if v.Destroyed() {
		return
	}
	v.closeMenu()
	v.revision++
	// Each proxy captures its delegate, so old rows unbind through the old one.
	v.delegate = delegate
	v.list.SetDelegate(&treeListDelegate{view: v, delegate: delegate})
}
func (v *TreeView) SelectionMode() SelectionMode { return v.mode }
func (v *TreeView) SetSelectionMode(mode SelectionMode) {
	if v.Destroyed() || mode > SelectionMultiple || v.mode == mode {
		return
	}
	v.mode = mode
	ids := v.Selection()
	if mode == SelectionSingle && len(ids) > 1 {
		v.SetSelection(ids[:1])
	}
}

// Selection returns a copy in flattened logical order, including offscreen
// nodes but excluding descendants of collapsed branches.
func (v *TreeView) Selection() []string {
	var ids []string
	for _, row := range v.rows {
		if v.selected[row.id] {
			ids = append(ids, row.id)
		}
	}
	return ids
}

// SetSelection filters hidden/unknown IDs and duplicates. In single-selection
// mode the first valid input ID wins. Programmatic changes emit Selection.
func (v *TreeView) SetSelection(ids []string) {
	if v.Destroyed() {
		return
	}
	next := make(map[string]bool)
	for _, id := range ids {
		if _, ok := v.indices[id]; ok {
			next[id] = true
			if v.mode == SelectionSingle {
				break
			}
		}
	}
	if treeEqualSet(v.selected, next) {
		return
	}
	old := v.Selection()
	v.selected = next
	v.notify(nil, old, v.current)
}

// Current is the logical keyboard-navigation node, not the focused Widget.
func (v *TreeView) Current() string { return v.current }

// SetCurrent neither selects nor scrolls. Empty ID clears Current; other IDs
// must be logically visible. Use Reveal separately to request scrolling.
func (v *TreeView) SetCurrent(id string) {
	if v.Destroyed() || id == v.current {
		return
	}
	if id != "" {
		if _, ok := v.indices[id]; !ok {
			return
		}
	}
	old := v.current
	v.current = id
	v.notify(nil, v.Selection(), old)
}
func (v *TreeView) Expanded(id string) bool { return v.expanded[id] }

// ExpandedIDs returns a sorted copy, retaining known descendants' expansion
// preferences even when an ancestor is collapsed.
func (v *TreeView) ExpandedIDs() []string { return treeSetIDs(v.expanded) }
func (v *TreeView) SetExpanded(id string, expanded bool) {
	if v.Destroyed() || v.model == nil || id == "" || v.expanded[id] == expanded {
		return
	}
	if _, ok := v.model.Parent(id); !ok || !v.model.Expandable(id) {
		return
	}
	old := slices.Clone(v.ExpandedIDs())
	ids := v.ExpandedIDs()
	if expanded {
		ids = append(ids, id)
	} else {
		ids = slices.DeleteFunc(ids, func(next string) bool { return next == id })
	}
	v.setExpandedIDs(ids, old)
}
func (v *TreeView) SetExpandedIDs(ids []string) {
	if v.Destroyed() {
		return
	}
	v.setExpandedIDs(ids, v.ExpandedIDs())
}
func (v *TreeView) setExpandedIDs(ids, old []string) {
	next := make(map[string]bool)
	if v.model != nil {
		for _, id := range ids {
			if id != "" {
				if _, ok := v.model.Parent(id); ok && v.model.Expandable(id) {
					next[id] = true
				}
			}
		}
	}
	if treeEqualSet(v.expanded, next) {
		return
	}
	selection, current := v.Selection(), v.current
	oldRows := slices.Clone(v.rows)
	v.expanded = next
	v.revision++
	v.flatten()
	v.normalize(oldRows, current)
	v.items.Emit()
	v.notify(old, selection, current)
}
func (v *TreeView) Indentation() float32 { return v.indentation }
func (v *TreeView) SetIndentation(value float32) {
	if v.Destroyed() || math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
		return
	}
	value = max(0, value)
	if value == v.indentation {
		return
	}
	v.indentation = value
	v.Refresh()
}

// RowAt queries a laid-out visible row using TreeView-local DIP, including
// indentation and row whitespace. Bounds are the full row and may be partially
// clipped. It never realizes rows or loads children; requery after layout.
func (v *TreeView) RowAt(point geometry.Point) (TreeRow, geometry.Rectangle, bool) {
	if v.Destroyed() || point.X < 0 || point.Y < 0 || point.X >= v.Rect().Width || point.Y >= v.Rect().Height {
		return TreeRow{}, geometry.Rectangle{}, false
	}
	for _, r := range v.realized {
		bounds := r.Rect()
		for parent := r.Parent(); parent != nil && parent != v; parent = parent.Parent() {
			bounds.Pos = bounds.Pos.Add(parent.Rect().Pos)
		}
		if point.X >= bounds.X && point.X < bounds.X+bounds.Width && point.Y >= bounds.Y && point.Y < bounds.Y+bounds.Height {
			return v.rowState(r.flat), bounds, true
		}
	}
	return TreeRow{}, geometry.Rectangle{}, false
}

// Refresh rebinds realized row content without changing model or view state.
// Structural data changes must be reported by the model's Items signal.
func (v *TreeView) Refresh() {
	if !v.Destroyed() {
		v.list.Refresh()
		v.RequestPaint()
	}
}

// Reveal expands known ancestors and requests vertical visibility through the
// containing ScrollView. It does not select, set Current, or wait for a load.
func (v *TreeView) Reveal(id string) {
	if v.Destroyed() || v.model == nil || id == "" {
		return
	}
	parent, ok := v.model.Parent(id)
	if !ok {
		return
	}
	ids := v.ExpandedIDs()
	seen := map[string]bool{id: true}
	for parent != "" {
		if seen[parent] {
			panic("widgets: cyclic tree parent chain")
		}
		seen[parent] = true
		ids = append(ids, parent)
		parent, ok = v.model.Parent(parent)
		if !ok {
			panic("widgets: missing tree parent")
		}
	}
	v.SetExpandedIDs(ids)
	if v.Destroyed() {
		return
	}
	if index, ok := v.indices[id]; ok {
		v.revealID = id
		v.list.Reveal(index)
	}
}
func (v *TreeView) ConnectScrollIntoView(fn func(geometry.Rectangle)) signal.Handle {
	return v.revealSignal.Connect(func(rect geometry.Rectangle) {
		if !v.Destroyed() {
			fn(rect)
		}
	})
}

func (v *TreeView) flatten() {
	v.rows = nil
	clear(v.indices)
	if v.model == nil {
		return
	}
	var visit func(string, int)
	visit = func(parent string, depth int) {
		children := v.model.Children(parent)
		for position, id := range children {
			actual, exists := v.model.Parent(id)
			if id == "" || !exists || actual != parent {
				panic("widgets: inconsistent tree parent or empty node ID")
			}
			if _, duplicate := v.indices[id]; duplicate {
				panic("widgets: duplicate or cyclic tree node")
			}
			v.indices[id] = len(v.rows)
			v.rows = append(v.rows, flatTreeRow{id, parent, depth, position + 1, len(children)})
			if v.expanded[id] && v.model.Expandable(id) {
				visit(id, depth+1)
			}
		}
	}
	visit("", 0)
}
func (v *TreeView) normalize(oldRows []flatTreeRow, oldCurrent string) {
	wasSelected := v.selected[oldCurrent]
	for id := range v.selected {
		if _, ok := v.indices[id]; !ok {
			delete(v.selected, id)
		}
	}
	for id := range v.expanded {
		if v.model == nil {
			delete(v.expanded, id)
			continue
		}
		if _, exists := v.model.Parent(id); !exists || !v.model.Expandable(id) {
			delete(v.expanded, id)
		}
	}
	if oldCurrent != "" {
		if _, visible := v.indices[oldCurrent]; !visible {
			v.current = ""
			oldIndex := slices.IndexFunc(oldRows, func(row flatTreeRow) bool { return row.id == oldCurrent })
			parents := make(map[string]string, len(oldRows))
			for _, row := range oldRows {
				parents[row.id] = row.parent
			}
			for parent := parents[oldCurrent]; parent != ""; parent = parents[parent] {
				if _, ok := v.indices[parent]; ok {
					v.current = parent
					break
				}
			}
			if v.current == "" && len(v.rows) > 0 {
				v.current = v.rows[max(0, min(oldIndex, len(v.rows)-1))].id
			}
			// A hidden current moves selection to its visible ancestor. A deleted
			// unselected current does not introduce a new selection.
			_, stillExists := "", false
			if v.model != nil {
				_, stillExists = v.model.Parent(oldCurrent)
			}
			if v.current != "" && (wasSelected || stillExists) {
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
}
func (v *TreeView) modelChanged() {
	// Invalidate in-flight notifications immediately, even when the next
	// synchronization must wait for a model callback to return.
	v.revision++
	if v.synchronizing {
		v.syncPending = true
		return
	}
	v.synchronizing = true
	defer func() { v.synchronizing = false }()
	for {
		v.syncPending = false
		oldRows, oldExpanded, oldSelection, oldCurrent := slices.Clone(v.rows), v.ExpandedIDs(), v.Selection(), v.current
		v.revision++
		v.flatten()
		v.normalize(oldRows, oldCurrent)
		v.closeMenu()
		if slices.Equal(oldRows, v.rows) {
			v.Refresh()
		} else {
			v.items.Emit()
		}
		v.notify(oldExpanded, oldSelection, oldCurrent)
		if v.Destroyed() || !v.syncPending {
			break
		}
	}
}
func (v *TreeView) notify(oldExpanded, oldSelection []string, oldCurrent string) {
	if v.Destroyed() {
		return
	}
	version := v.revision
	selected, current := v.Selection(), v.current
	// State changes update only changed bindings during layout; they must not
	// flush measurements or rebuild every visible content subtree.
	v.RequestLayout()
	v.RequestPaint()
	if v.revealID != "" {
		if index, ok := v.indices[v.revealID]; ok {
			v.list.Reveal(index)
		} else {
			v.revealID = ""
		}
	}
	if oldExpanded != nil {
		old := make(map[string]bool)
		for _, id := range oldExpanded {
			old[id] = true
		}
		union := make(map[string]bool)
		for id := range old {
			union[id] = true
		}
		for id := range v.expanded {
			union[id] = true
		}
		for _, id := range treeSetIDs(union) {
			if old[id] != v.expanded[id] {
				v.expansion.Emit(id, v.expanded[id], version)
			}
			if !v.valid(version) {
				return
			}
		}
	}
	if !slices.Equal(selected, oldSelection) && v.selectionMatches(selected) {
		v.selection.Emit(selected, version)
	}
	if !v.valid(version) {
		return
	}
	if oldCurrent != current && current == v.current {
		v.currentSignal.Emit(current, version)
	}
	if v.valid(version) {
		v.requestChildren()
	}
}
func (v *TreeView) valid(version uint64) bool { return !v.Destroyed() && v.revision == version }
func (v *TreeView) requestChildren() {
	if v.Destroyed() || v.Root() == nil {
		return
	}
	requester, ok := v.model.(TreeChildrenRequester)
	if !ok {
		return
	}
	active := make(map[string]bool)
	for _, row := range v.rows {
		if v.expanded[row.id] {
			active[row.id] = true
		}
	}
	for id := range v.loads {
		if !active[id] {
			delete(v.loads, id)
		}
	}
	version := v.revision
	for _, row := range slices.Clone(v.rows) {
		if active[row.id] && !v.loads[row.id] {
			v.loads[row.id] = true
			requester.RequestChildren(row.id)
			if !v.valid(version) {
				return
			}
		}
	}
}
func (v *TreeView) ConnectSelection(fn func([]string)) signal.Handle {
	return v.selection.Connect(func(ids []string, version uint64) {
		if v.valid(version) && v.selectionMatches(ids) {
			fn(slices.Clone(ids))
		}
	})
}

// The generation guards row order. Checking only the selected IDs avoids
// walking the entire flattened tree once per signal receiver.
func (v *TreeView) selectionMatches(ids []string) bool {
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
func (v *TreeView) ConnectCurrent(fn func(string)) signal.Handle {
	return v.currentSignal.Connect(func(id string, version uint64) {
		if v.valid(version) && id == v.current {
			fn(id)
		}
	})
}
func (v *TreeView) ConnectExpanded(fn func(string, bool)) signal.Handle {
	return v.expansion.Connect(func(id string, expanded bool, version uint64) {
		if v.valid(version) && v.expanded[id] == expanded {
			fn(id, expanded)
		}
	})
}
func (v *TreeView) ConnectActivate(fn func(string)) signal.Handle {
	return v.activate.Connect(func(id string, version uint64) {
		if v.valid(version) {
			fn(id)
		}
	})
}
func treeSetIDs(set map[string]bool) []string {
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func treeEqualSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for id := range a {
		if !b[id] {
			return false
		}
	}
	return true
}

func (v *TreeView) ContentSize() geometry.Size { return v.list.ContentSize() }
func (v *TreeView) LayoutVisible(viewport geometry.Size, offset geometry.Point) {
	if v.Destroyed() {
		return
	}
	version := v.revision
	for _, index := range v.list.VisibleIndexes() {
		if index >= len(v.rows) {
			continue
		}
		if row := v.realized[v.rows[index].id]; row != nil && row.bound != v.rowState(v.rows[index]) {
			v.list.Delegate().Bind(index, row)
			if !v.valid(version) {
				return
			}
		}
	}
	v.list.LayoutVisible(viewport, offset)
	if v.Destroyed() {
		return
	}
	if v.menuID != "" && v.realized[v.menuID] == nil {
		v.closeMenu()
	}
	if id := v.menuPending; id != "" {
		if row := v.realized[id]; row != nil {
			v.menuPending = ""
			v.showMenu(id, row.Rect().Pos.Add(geometry.Point{Y: row.Rect().Height}))
		}
	}
}
func (v *TreeView) Snapshot() gui.WidgetInfo {
	info := v.WidgetBase.Snapshot()
	info.Role, info.ItemCount = RoleTree, len(v.rows)
	if len(info.Children) == 1 {
		list := info.Children[0]
		info.Children, info.VisibleStart, info.VisibleEnd = list.Children, list.VisibleStart, list.VisibleEnd
	}
	return info
}
func (v *TreeView) Paint(p gui.Painter) {
	name := v.StyleName()
	if name == "" {
		name = "tree-view"
	}
	paintStyledBox(p, geometry.Rect(0, 0, v.Rect().Width, v.Rect().Height), name, "", style.Normal)
}

func (v *TreeView) choose(id string, primary, shift bool) {
	index, exists := v.indices[id]
	if !exists || v.Destroyed() {
		return
	}
	oldSelection, oldCurrent := v.Selection(), v.current
	v.current = id
	if v.mode == SelectionSingle || !primary && !shift {
		clear(v.selected)
		v.selected[id] = true
		v.anchor = id
	} else if shift {
		anchor, ok := v.indices[v.anchor]
		if !ok {
			anchor = index
			v.anchor = id
		}
		if !primary {
			clear(v.selected)
		}
		for i := min(anchor, index); i <= max(anchor, index); i++ {
			v.selected[v.rows[i].id] = true
		}
	} else {
		if v.selected[id] {
			delete(v.selected, id)
		} else {
			v.selected[id] = true
		}
		v.anchor = id
	}
	v.notify(nil, oldSelection, oldCurrent)
}
func treeModifiers(modifiers events.Modifiers) (primary, shift bool) {
	mask, _ := gui.ModPrimary.Resolve()
	return modifiers&mask != 0, modifiers&events.ModifierShift != 0
}
func (v *TreeView) keyDown(ctx gui.EventContext, event events.KeyEvent) {
	if v.Destroyed() || len(v.rows) == 0 {
		return
	}
	primary, shift := treeModifiers(event.Modifiers)
	allowed, _ := (gui.ModPrimary | gui.ModShift).Resolve()
	if event.Modifiers & ^allowed != 0 {
		return
	}
	index, hasCurrent := v.indices[v.current]
	if !hasCurrent {
		index = 0
		if selected := v.Selection(); len(selected) > 0 {
			index = v.indices[selected[0]]
		}
	}
	target, navigation := index, true
	switch event.Key {
	case events.KeyArrowUp:
		if hasCurrent {
			target--
		}
	case events.KeyArrowDown:
		if hasCurrent {
			target++
		}
	case events.KeyHome:
		target = 0
	case events.KeyEnd:
		target = len(v.rows) - 1
	case events.KeyPageUp:
		target -= max(1, len(v.list.VisibleIndexes())-1)
	case events.KeyPageDown:
		target += max(1, len(v.list.VisibleIndexes())-1)
	case events.KeyArrowLeft:
		id := v.rows[index].id
		if v.expanded[id] {
			v.SetExpanded(id, false)
			navigation = false
		} else if parent := v.rows[index].parent; parent != "" {
			target = v.indices[parent]
		}
	case events.KeyArrowRight:
		id := v.rows[index].id
		if v.model.Expandable(id) && !v.expanded[id] {
			v.SetExpanded(id, true)
			navigation = false
		} else if index+1 < len(v.rows) && v.rows[index+1].parent == id {
			target++
		}
	case events.KeySpace:
		v.choose(v.rows[index].id, primary, false)
		navigation = false
	case events.KeyEnter:
		if !hasCurrent {
			return
		}
		v.activate.Emit(v.rows[index].id, v.revision)
		navigation = false
	case events.KeyA:
		if !primary || v.mode != SelectionMultiple {
			return
		}
		ids := make([]string, len(v.rows))
		for i, row := range v.rows {
			ids[i] = row.id
		}
		v.SetSelection(ids)
		navigation = false
	default:
		return
	}
	ctx.StopPropagation()
	if navigation && !v.Destroyed() && len(v.rows) != 0 {
		target = max(0, min(target, len(v.rows)-1))
		id := v.rows[target].id
		if primary && !shift {
			v.SetCurrent(id)
		} else {
			v.choose(id, primary && shift, shift)
		}
		v.Reveal(id)
	}
}

func (v *TreeView) ConnectContextMenu(fn func(string, *gui.MenuModel)) signal.Handle {
	return v.menuQuery.Connect(func(id string, model *gui.MenuModel, version uint64) {
		if v.valid(version) {
			fn(id, model)
		}
	})
}
func (v *TreeView) ConnectContextMenuError(fn func(error)) signal.Handle {
	return v.menuError.Connect(func(err error) {
		if !v.Destroyed() {
			fn(err)
		}
	})
}
func (v *TreeView) closeMenu() {
	v.menuID, v.menuPending = "", ""
	if v.menu != nil {
		v.menu.SetMenu(nil)
	}
}
func (v *TreeView) showMenu(id string, position geometry.Point) {
	if v.Destroyed() {
		return
	}
	v.closeMenu()
	version := v.revision
	var model gui.MenuModel
	v.menuQuery.Emit(id, &model, version)
	if !v.valid(version) || model == nil || model.ItemsCount() == 0 {
		return
	}
	if v.menu == nil {
		v.menu = gui.NewPopoverMenu(v)
		v.menu.ConnectClosed(v.closeMenu)
	}
	v.menuID = id
	v.menu.SetMenu(model)
	if err := v.menu.ShowAt(position); err != nil {
		v.closeMenu()
		v.menuError.Emit(err)
	}
}

func (v *TreeView) contextMenuRequested(ctx gui.EventContext) {
	point, ok := ctx.Position()
	if !ok {
		if v.current != "" {
			ctx.StopPropagation()
			v.menuPending = v.current
			v.Reveal(v.menuPending)
		}
		return
	}
	ctx.StopPropagation()
	version := v.revision
	id := ""
	for _, row := range v.realized {
		r := row.Rect()
		if point.Y >= r.Y && point.Y < r.Y+r.Height {
			id = row.bound.ID
			break
		}
	}
	if id != "" {
		if v.selected[id] {
			v.SetCurrent(id)
		} else {
			v.choose(id, false, false)
		}
	}
	if v.valid(version) {
		v.showMenu(id, point)
	}
}
