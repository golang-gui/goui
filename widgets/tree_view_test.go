package widgets

import (
	"encoding/json"
	"fmt"
	"image/color"
	"runtime"
	"slices"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

// 内存模型、无字体行和正常 EventDispatcher 路径，不依赖桌面。
type treeTestDelegate struct {
	binds, setups int
	unbound       []TreeRow
	view          *TreeView
	bindings      map[*TreeExpander]TreeRow
}

func (d *treeTestDelegate) Setup() gui.Widget {
	d.setups++
	e := NewTreeExpander()
	e.SetChild(splitTestChild(160, 20))
	e.ConnectToggle(func() {
		if row, ok := d.bindings[e]; ok && d.view != nil {
			d.view.SetExpanded(row.ID, !d.view.Expanded(row.ID))
		}
	})
	return e
}
func (d *treeTestDelegate) Bind(row TreeRow, w gui.Widget) {
	d.binds++
	e := w.(*TreeExpander)
	if d.bindings == nil {
		d.bindings = make(map[*TreeExpander]TreeRow)
	}
	d.bindings[e] = row
	e.SetDepth(row.Depth)
	e.SetExpandable(row.Expandable)
	e.SetExpanded(row.Expanded)
	if d.view != nil {
		e.SetIndentation(d.view.Indentation())
	}
}
func (d *treeTestDelegate) Unbind(row TreeRow, w gui.Widget) {
	d.unbound = append(d.unbound, row)
	delete(d.bindings, w.(*TreeExpander))
	w.(*TreeExpander).SetExpandable(false)
}
func treeFixture() (*TreeView, *TreeStore[string]) {
	m := NewTreeStore[string]()
	m.Modify(func(m *TreeStore[string]) {
		m.Append("", "root", "Root")
		m.Append("root", "a", "A")
		m.Append("root", "b", "B")
		m.Append("a", "nested", "Nested")
		m.Append("", "other", "Other")
	})
	v := NewTreeView()
	v.SetModel(m)
	v.SetDelegate(&treeTestDelegate{view: v})
	return v, m
}
func treeLayout(v *TreeView, height, y float32) {
	v.Arrange(geometry.Rect(0, 0, 300, height))
	v.LayoutVisible(geometry.Size{Width: 300, Height: height}, geometry.Point{Y: y})
}
func checkTreeState(t *testing.T, v *TreeView, current string, selected ...string) {
	t.Helper()
	if v.Current() != current || !slices.Equal(v.Selection(), selected) {
		t.Fatalf("current=%q selected=%v, want %q %v", v.Current(), v.Selection(), current, selected)
	}
}
func TestTreeViewStateIsolationCollapseAndRemoval(t *testing.T) {
	v, m := treeFixture()
	other := NewTreeView()
	other.SetModel(m)
	v.SetExpandedIDs([]string{"root", "a"})
	v.SetSelectionMode(SelectionMultiple)
	v.SetCurrent("nested")
	v.SetSelection([]string{"other", "nested", "a", "missing", "a"})
	checkTreeState(t, v, "nested", "a", "nested", "other")
	v.SetExpanded("a", false)
	checkTreeState(t, v, "a", "a", "other")
	v.SetExpanded("root", false)
	checkTreeState(t, v, "root", "root", "other")
	if other.Expanded("root") || other.Current() != "" || len(other.Selection()) != 0 {
		t.Fatal("shared model polluted state")
	}
	v.SetExpanded("root", true)
	v.SetCurrent("b")
	v.SetSelection([]string{"b"})
	m.Remove("b")
	checkTreeState(t, v, "root", "root")
	v.SetCurrent("other")
	v.SetSelection(nil)
	m.Remove("other")
	checkTreeState(t, v, "a")
	v.SetModel(m)
	if v.Expanded("root") {
		t.Fatal("explicit reinstallation retained expansion")
	}
	checkTreeState(t, v, "")
	v.SetModel(NewTreeStore[string]())
	checkTreeState(t, v, "")
}
func TestTreeViewSignalsCommitCopyAndReentrancy(t *testing.T) {
	v, m := treeFixture()
	var order []string
	v.ConnectExpanded(func(id string, expanded bool) {
		order = append(order, "expanded")
		if id == "root" && expanded && len(v.rows) != 4 {
			t.Fatal("not committed before signal")
		}
	})
	v.SetExpanded("root", true)
	v.SetSelectionMode(SelectionMultiple)
	v.ConnectSelection(func(ids []string) {
		order = append(order, "selection")
		if len(ids) > 0 {
			ids[0] = "changed"
		}
	})
	v.ConnectSelection(func(ids []string) {
		if len(ids) > 0 && ids[0] == "changed" {
			t.Fatal("receiver shared slice")
		}
	})
	v.ConnectCurrent(func(string) { order = append(order, "current") })
	v.choose("a", false, false)
	if !slices.Equal(order, []string{"expanded", "selection", "current"}) {
		t.Fatal(order)
	}
	checkTreeState(t, v, "a", "a")
	v.ConnectExpanded(func(id string, expanded bool) {
		if id == "a" && expanded {
			m.Remove("a")
		}
	})
	v.SetExpanded("a", true)
	if v.Expanded("a") || v.current != "root" {
		t.Fatal("reentrant model update lost")
	}
}
func TestTreeViewNavigationSelectionAndDisclosure(t *testing.T) {
	v, _ := treeFixture()
	v.SetExpanded("root", true)
	v.SetSelectionMode(SelectionMultiple)
	treeLayout(v, 180, 0)
	host := &tabInputHost{root: v}
	dispatcher := new(gui.EventDispatcher)
	key := func(k events.Key, mods events.Modifiers) {
		if err := dispatcher.DispatchEvent(host, events.KeyEvent{EventType: events.KeyDown, Key: k, Modifiers: mods}); err != nil {
			t.Fatal(err)
		}
	}
	host.SetFocusedWidget(v)
	key(events.KeyArrowDown, 0)
	checkTreeState(t, v, "root", "root")
	key(events.KeyArrowDown, events.ModifierShift)
	checkTreeState(t, v, "a", "root", "a")
	key(events.KeyArrowDown, events.ModifierShift)
	checkTreeState(t, v, "b", "root", "a", "b")
	key(events.KeyArrowUp, events.ModifierShift)
	checkTreeState(t, v, "a", "root", "a")
	primary, _ := gui.ModPrimary.Resolve()
	key(events.KeyArrowDown, primary)
	checkTreeState(t, v, "b", "root", "a")
	key(events.KeySpace, primary)
	checkTreeState(t, v, "b", "root", "a", "b")
	key(events.KeyA, primary)
	checkTreeState(t, v, "b", "root", "a", "b", "other")
	key(events.KeyArrowLeft, 0)
	checkTreeState(t, v, "root", "root")
	key(events.KeyArrowLeft, 0)
	if v.Expanded("root") {
		t.Fatal("left did not collapse")
	}
	key(events.KeyArrowRight, 0)
	if !v.Expanded("root") {
		t.Fatal("right did not expand")
	}
	treeLayout(v, 180, 0)
	v.SetSelection(nil)
	v.SetCurrent("")
	dispatchTabPointer(t, dispatcher, host, events.PointerDown, 14, 14)
	dispatchTabPointer(t, dispatcher, host, events.PointerUp, 14, 14)
	if v.Expanded("root") || len(v.Selection()) != 0 {
		t.Fatal("expander altered selection or failed")
	}
}
func TestTreeViewRowsSnapshotDelegateAndVirtualization(t *testing.T) {
	v, _ := treeFixture()
	v.SetExpandedIDs([]string{"root", "a"})
	treeLayout(v, 180, 0)
	v.SetSelection([]string{"nested"})
	v.SetCurrent("nested")
	info := v.Snapshot()
	if info.Role != RoleTree || info.ItemCount != 5 || len(info.Children) != 5 {
		t.Fatalf("tree snapshot %+v", info)
	}
	nested := info.Children[2]
	hierarchy, ok := nested.Attributes[HierarchyInfoKey].(HierarchyInfo)
	if nested.Role != RoleTreeItem || !ok || hierarchy.NodeID != "nested" || hierarchy.Level != 3 || !nested.Selected || !hierarchy.Current || nested.Focused {
		t.Fatal("hierarchy/current/focus incorrect")
	}
	if info.Children[0].Children[0].Children[0].Role != gui.RoleButton || info.Children[0].Children[0].Children[0].Name != "折叠" {
		t.Fatal("disclosure semantics")
	}
	old := v.Delegate().(*treeTestDelegate)
	newer := new(treeTestDelegate)
	v.SetDelegate(newer)
	if len(old.unbound) != 5 || len(newer.unbound) != 0 {
		t.Fatal("new delegate unbound old content")
	}
	m := NewTreeStore[int]()
	m.Modify(func(m *TreeStore[int]) {
		for i := 0; i < 10000; i++ {
			m.Append("", fmt.Sprint(i), i)
		}
	})
	v.SetModel(m)
	treeLayout(v, 140, 0)
	if len(v.realized) != 5 || newer.setups != 5 {
		t.Fatal("virtualization created all nodes")
	}
	treeLayout(v, 140, 28000)
	if len(v.realized) > 6 || newer.setups > 6 {
		t.Fatalf("jump grew widget pool: %d/%d", len(v.realized), newer.setups)
	}
	if hierarchy, ok := v.Snapshot().Children[0].Attributes[HierarchyInfoKey].(HierarchyInfo); !ok || hierarchy.NodeID != "1000" {
		t.Fatalf("scrolled range/order: %+v", hierarchy)
	}
}

func TestTreeViewSnapshotAttributesIsolation(t *testing.T) {
	v, _ := treeFixture()
	defer v.SetModel(nil)
	v.SetExpandedIDs([]string{"root", "a"})
	v.SetSelection([]string{"nested"})
	v.SetCurrent("nested")
	treeLayout(v, 180, 0)
	old := v.Snapshot()
	root := old.Children[0].Attributes[HierarchyInfoKey].(HierarchyInfo)
	nested := old.Children[2].Attributes[HierarchyInfoKey].(HierarchyInfo)
	v.SetSelection(nil)
	v.SetCurrent("root")
	v.SetExpanded("root", false)
	treeLayout(v, 180, 0)
	now := v.Snapshot()
	if got := now.Children[0].Attributes[HierarchyInfoKey].(HierarchyInfo); got.Expanded || !got.Current {
		t.Fatalf("new hierarchy did not capture changed state: %+v", got)
	}
	now.Children[0].SetAttribute(HierarchyInfoKey, HierarchyInfo{})
	if old.Children[0].Attributes[HierarchyInfoKey].(HierarchyInfo) != root || !root.Expanded || old.Children[2].Attributes[HierarchyInfoKey].(HierarchyInfo) != nested || !nested.Current || !old.Children[2].Selected {
		t.Fatal("previous tree snapshot shares mutable state with a later snapshot")
	}
	data, err := json.Marshal(old.Children[2])
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if _, legacy := fields["hierarchy"]; legacy {
		t.Fatal("snapshot still emits the retired hierarchy field")
	}
	var attributes map[string]json.RawMessage
	if err := json.Unmarshal(fields["attributes"], &attributes); err != nil {
		t.Fatal(err)
	}
	var decoded HierarchyInfo
	if err := json.Unmarshal(attributes[HierarchyInfoKey], &decoded); err != nil {
		t.Fatal(err)
	}
	want := HierarchyInfo{NodeID: "nested", ParentID: "a", Level: 3, PositionInSet: 1, SetSize: 1, Current: true}
	if decoded != want {
		t.Fatalf("JSON changed the hierarchy schema: got %+v want %+v", decoded, want)
	}
}

type lazyTreeModel struct {
	*TreeStore[string]
	requests []string
	queried  []string
}

func (m *lazyTreeModel) Children(id string) []string {
	m.queried = append(m.queried, id)
	return m.TreeStore.Children(id)
}
func (m *lazyTreeModel) RequestChildren(id string) {
	m.requests = append(m.requests, id)
	if len(m.Children(id)) == 0 {
		m.Append(id, id+"/child", "loaded")
	}
}
func TestTreeViewLazyLoadMountCyclesAndCollapsedQueries(t *testing.T) {
	m := &lazyTreeModel{TreeStore: NewTreeStore[string]()}
	m.Append("", "lazy", "Lazy")
	m.SetExpandable("lazy", true)
	v := NewTreeView()
	v.SetModel(m)
	v.SetExpanded("lazy", true)
	if len(m.requests) != 0 {
		t.Fatal("detached view loaded")
	}
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(v)
	if !slices.Equal(m.requests, []string{"lazy"}) || len(v.rows) != 2 {
		t.Fatal("initial expanded branch not loaded")
	}
	treeLayout(v, 100, 0)
	treeLayout(v, 100, 0)
	m.Set("lazy", "updated")
	if len(m.requests) != 1 {
		t.Fatal("layout or data refresh repeated load")
	}
	v.SetExpanded("lazy", false)
	m.queried = nil
	m.Set("lazy", "changed")
	if !slices.Equal(m.queried, []string{""}) {
		t.Fatalf("queried collapsed descendants: %v", m.queried)
	}
	v.SetExpanded("lazy", true)
	if len(m.requests) != 2 {
		t.Fatal("reactivation did not request")
	}
	owner.SetWidget(nil)
	owner.SetWidget(v)
	if len(m.requests) != 3 {
		t.Fatal("remount request cycle")
	}
	owner.Destroy()
}

type treeContentDelegate struct{ setup func() gui.Widget }

func (d treeContentDelegate) Setup() gui.Widget        { return d.setup() }
func (treeContentDelegate) Bind(TreeRow, gui.Widget)   {}
func (treeContentDelegate) Unbind(TreeRow, gui.Widget) {}

func TestTreeViewInteractiveContentAndDragCompetition(t *testing.T) {
	for _, dragging := range []bool{false, true} {
		t.Run(fmt.Sprint(dragging), func(t *testing.T) {
			v, _ := treeFixture()
			clicks, drags, selections, activations := 0, 0, 0, 0
			v.SetDelegate(treeContentDelegate{setup: func() gui.Widget {
				if dragging {
					w := splitTestChild(100, 20)
					controller := gui.NewDragEventController()
					controller.ConnectBegin(func(geometry.Point, events.Modifiers) { drags++ })
					w.AddEventController(controller)
					return w
				}
				button := gui.NewButton()
				button.SetChild(splitTestChild(100, 20))
				button.ConnectClicked(func() { clicks++ })
				return button
			}})
			v.ConnectSelection(func([]string) { selections++ })
			v.ConnectActivate(func(string) { activations++ })
			treeLayout(v, 150, 0)
			host, dispatcher := &tabInputHost{root: v}, new(gui.EventDispatcher)
			// Row content starts after the 24 DIP disclosure/padding prefix.
			dispatchTabPointer(t, dispatcher, host, events.PointerDown, 60, 16)
			if dragging {
				dispatchTabPointer(t, dispatcher, host, events.PointerMove, 90, 16)
			}
			dispatchTabPointer(t, dispatcher, host, events.PointerUp, 90, 16)
			if selections != 0 || activations != 0 || v.Current() != "" {
				t.Fatal("content interaction leaked into row selection/activation")
			}
			if dragging && drags != 1 || !dragging && clicks != 1 {
				t.Fatalf("clicks=%d drags=%d", clicks, drags)
			}
		})
	}
}

func TestTreeViewRevealCompletesAndPreservesHorizontalOffset(t *testing.T) {
	v := NewTreeView()
	m := NewTreeStore[int]()
	m.Modify(func(m *TreeStore[int]) {
		for i := 0; i < 100; i++ {
			m.Append("", fmt.Sprint(i), i)
		}
	})
	v.SetModel(m)
	// 23.5 content height + 8 padding = 31.5 DIP: final content has a
	// fractional bottom edge while ScrollView floors its maximum offset.
	v.SetDelegate(treeContentDelegate{setup: func() gui.Widget { return splitTestChild(500, 23.5) }})
	scroll := gui.NewScrollView()
	scroll.SetChild(v)
	arrange := func() {
		scroll.Measure(layout.Tight(geometry.Size{Width: 200, Height: 100}))
		scroll.Arrange(geometry.Rect(0, 0, 200, 100))
	}
	arrange()
	scroll.SetScrollX(60)
	v.Reveal("99")
	for i := 0; i < 20 && v.revealID != ""; i++ {
		arrange()
	}
	if v.revealID != "" || v.realized["99"] == nil || scroll.ScrollX() != 60 {
		t.Fatalf("target=%q realized=%v x=%g", v.revealID, v.realized["99"] != nil, scroll.ScrollX())
	}
	scroll.SetScrollY(0)
	arrange()
	v.SetSelection([]string{"0"})
	arrange()
	if scroll.ScrollY() != 0 {
		t.Fatal("completed reveal stole scroll on a subsequent state change")
	}
}

func TestTreeViewHostDestructionStopsStaleReceivers(t *testing.T) {
	v, _ := treeFixture()
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(v)
	called := 0
	v.ConnectSelection(func([]string) { owner.Destroy() })
	v.ConnectSelection(func([]string) { called++ })
	v.ConnectCurrent(func(string) { called++ })
	v.choose("root", false, false)
	// A lazy Popover releases and unmounts content, but does not perform the
	// final Widget destruction owned by a Window. Unmount must invalidate the
	// notification version just as final destruction does.
	if v.Root() != nil || called != 0 {
		t.Fatal("unmounted tree dispatched stale notifications")
	}
}

type nonComparableTreeModel struct {
	*TreeStore[int]
	marker []int
}

func TestTreeViewNonComparableModelNotifications(t *testing.T) {
	m := NewTreeStore[int]()
	m.Append("", "a", 1)
	v := NewTreeView()
	v.SetModel(nonComparableTreeModel{TreeStore: m})
	m.Append("", "b", 2)
	if v.Snapshot().ItemCount != 2 {
		t.Fatal("non-comparable model subscription ignored notifications")
	}
}

type treeInstallModel struct {
	*TreeStore[int]
	connections, deliveries int
}

func (m *treeInstallModel) ConnectItems(fn func()) signal.Handle {
	m.connections++
	return m.TreeStore.ConnectItems(func() { m.deliveries++; fn() })
}

func TestTreeViewExplicitModelAndDelegateReinstallation(t *testing.T) {
	m := &treeInstallModel{TreeStore: NewTreeStore[int]()}
	m.Append("", "root", 0)
	m.Append("root", "child", 1)
	v, d := NewTreeView(), new(treeTestDelegate)
	v.SetModel(m)
	v.SetDelegate(d)
	v.SetExpanded("root", true)
	v.SetSelection([]string{"child"})
	v.SetCurrent("child")
	treeLayout(v, 100, 0)
	before := d.setups
	v.SetDelegate(d)
	checkTreeState(t, v, "child", "child")
	if !v.Expanded("root") || len(d.unbound) != 2 {
		t.Fatal("delegate reinstall lost state or retained bindings")
	}
	treeLayout(v, 100, 0)
	if d.setups != before+2 {
		t.Fatal("same delegate kept its old row shells")
	}
	selections, currents := 0, 0
	v.ConnectSelection(func([]string) { selections++ })
	v.ConnectCurrent(func(string) { currents++ })
	v.Reveal("child")
	v.SetModel(m)
	checkTreeState(t, v, "")
	if v.Expanded("root") || v.revealID != "" || m.connections != 2 || selections != 1 || currents != 1 {
		t.Fatal("same model did not fully reinstall and report actual state changes")
	}
	v.SetModel(m)
	if m.connections != 3 || selections != 1 || currents != 1 {
		t.Fatal("empty state emitted false changes or failed to reconnect")
	}
	m.Set("root", 2)
	if m.deliveries != 1 {
		t.Fatal("old subscription still receives items")
	}
	v.SetModel(nil)
	m.Set("root", 3)
	if m.deliveries != 1 {
		t.Fatal("cleared model retained a subscription")
	}
}

type treeValueModel struct {
	*treeInstallModel
	payload any
}
type treeValueDelegate struct {
	*treeTestDelegate
	payload any
}

func TestTreeViewInstallsInterfaceValuesWithoutComparing(t *testing.T) {
	for _, payload := range []any{[]int{1}, map[string]int{"x": 1}, func() {}} {
		m := treeValueModel{&treeInstallModel{TreeStore: NewTreeStore[int]()}, payload}
		m.Append("", "root", 0)
		d := treeValueDelegate{new(treeTestDelegate), payload}
		v := NewTreeView()
		v.SetModel(m)
		v.SetDelegate(d)
		treeLayout(v, 100, 0)
		v.SetModel(m)
		v.SetDelegate(d)
		if m.connections != 2 || len(d.unbound) != 1 {
			t.Fatal("value reinstallation was skipped")
		}
		v.SetModel(nil)
	}
}

func TestTreeViewUnmountReleasesLastBindings(t *testing.T) {
	v, _ := treeFixture()
	v.SetExpanded("root", true)
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(v)
	treeLayout(v, 180, 0)
	d := v.Delegate().(*treeTestDelegate)
	v.SetSelection([]string{"a"})
	treeLayout(v, 180, 0) // rebind state before checking the Unbind snapshot
	owner.SetWidget(nil)
	if len(d.unbound) != 4 || len(v.realized) != 0 {
		t.Fatal("unmount retained row bindings")
	}
	selected := slices.IndexFunc(d.unbound, func(row TreeRow) bool { return row.ID == "a" })
	if selected < 0 || !d.unbound[selected].Selected {
		t.Fatal("unbind did not receive the last binding snapshot")
	}
	owner.SetWidget(v)
	treeLayout(v, 180, 0)
	if len(v.realized) != 4 || len(d.unbound) != 4 {
		t.Fatal("remount did not bind clean rows or unbound twice")
	}
	owner.Destroy()
	if len(d.unbound) != 8 {
		t.Fatal("second host release lost row cleanup")
	}
}

func TestTreeViewStableIDsSurviveRenameReorderAndReparent(t *testing.T) {
	v, m := treeFixture()
	m.SetExpandable("other", true)
	v.SetExpandedIDs([]string{"root", "a", "other"})
	v.SetCurrent("nested")
	v.SetSelection([]string{"nested"})
	m.Modify(func(m *TreeStore[string]) {
		m.Set("nested", "renamed")
		// A custom model may sort or reparent; TreeStore's private records let
		// this package test express that atomic model notification without
		// adding production mutation APIs solely for the test.
		m.nodes["a"].children = nil
		m.nodes["other"].children = []string{"nested"}
		m.nodes["nested"].parent = "other"
		slices.Reverse(m.roots)
		m.notify()
	})
	checkTreeState(t, v, "nested", "nested")
	treeLayout(v, 180, 0)
	rows := v.Snapshot().Children
	parent, parentOK := rows[0].Attributes[HierarchyInfoKey].(HierarchyInfo)
	child, childOK := rows[1].Attributes[HierarchyInfoKey].(HierarchyInfo)
	if !parentOK || !childOK || parent.NodeID != "other" || child.NodeID != "nested" || child.ParentID != "other" || child.Level != 2 {
		t.Fatal("stable identity lost hierarchy or visible order")
	}
}

// 仅第一行很宽；它滚出视口后，选择、当前项或 Refresh 不能丢失水平范围。
type treeWidthDelegate struct {
	treeTestDelegate
	model *TreeStore[float32]
}

func (d *treeWidthDelegate) Bind(row TreeRow, w gui.Widget) {
	d.treeTestDelegate.Bind(row, w)
	w.(*TreeExpander).Child().SetMinSize(geometry.Size{Width: d.model.Item(row.ID), Height: 20})
}

func TestTreeViewStateRefreshRetainsMeasurementsAndBindings(t *testing.T) {
	m := NewTreeStore[float32]()
	m.Modify(func(m *TreeStore[float32]) {
		for i := 0; i < 200; i++ {
			width := float32(80)
			if i == 0 {
				width = 1000
			}
			m.Append("", fmt.Sprint(i), width)
		}
	})
	v, d := NewTreeView(), &treeWidthDelegate{model: m}
	v.SetModel(m)
	v.SetDelegate(d)
	s := gui.NewScrollView()
	s.SetChild(v)
	arrange := func() {
		for i := 0; i < 3; i++ {
			s.Measure(layout.Tight(geometry.Size{Width: 200, Height: 100}))
			s.Arrange(geometry.Rect(0, 0, 200, 100))
		}
	}
	arrange()
	s.SetScrollX(80)
	s.SetScrollY(280)
	arrange()
	row, before := v.realized["10"], d.binds
	if row == nil {
		t.Fatal("test row is not realized")
	}
	v.SetSelection([]string{"10"})
	arrange()
	if d.binds-before != 1 || v.realized["10"] != row || s.ScrollX() != 80 {
		t.Fatalf("selection rebuilt rows or lost width: binds=%d x=%g", d.binds-before, s.ScrollX())
	}
	before = d.binds
	v.SetCurrent("10")
	arrange()
	if d.binds-before != 1 || s.ScrollX() != 80 {
		t.Fatal("current change rebuilt unrelated bindings or lost width")
	}
	setups, unbound := d.setups, len(d.unbound)
	m.Set("10", 90)
	arrange()
	if d.setups != setups || len(d.unbound) != unbound || v.realized["10"] != row || row.content.(*TreeExpander).Child().MinSize().Width != 90 {
		t.Fatal("data-only refresh replaced shells/bindings or kept stale content")
	}
	v.Refresh()
	arrange()
	if s.ScrollX() != 80 {
		t.Fatal("explicit refresh discarded an offscreen width estimate")
	}
	// 结构变更必须解绑旧 ID，但不应再创建同规模视口的行壳。
	m.Append("", "new", 80)
	arrange()
	if d.setups != setups || len(d.unbound) == unbound {
		t.Fatal("structural reload did not reuse unbound shells")
	}
}

func TestTreeViewNotificationsSeparateCurrentAndSelection(t *testing.T) {
	v, _ := treeFixture()
	var selections [][]string
	var currents []string
	v.ConnectSelection(func([]string) { v.SetCurrent("other") })
	v.ConnectSelection(func(ids []string) { selections = append(selections, ids) })
	v.ConnectCurrent(func(id string) { currents = append(currents, id) })
	v.choose("root", false, false)
	if len(selections) != 1 || !slices.Equal(selections[0], []string{"root"}) || !slices.Equal(currents, []string{"other"}) {
		t.Fatalf("unrelated notification suppressed or stale current delivered: %v %v", selections, currents)
	}
	// 同一种状态在回调里改变时，后续监听者只收到最新状态。
	v, _ = treeFixture()
	selections = nil
	v.ConnectSelection(func(ids []string) {
		if slices.Equal(ids, []string{"root"}) {
			v.SetSelection([]string{"other"})
		}
	})
	v.ConnectSelection(func(ids []string) { selections = append(selections, ids) })
	v.SetSelection([]string{"root"})
	if len(selections) != 1 || !slices.Equal(selections[0], []string{"other"}) {
		t.Fatalf("stale selection delivered: %v", selections)
	}
}

func TestTreeViewModelReentryInvalidatesPendingNotifications(t *testing.T) {
	v, m := treeFixture()
	v.SetExpanded("root", true)
	v.SetCurrent("a")
	v.SetSelection([]string{"a"})
	var selections [][]string
	v.ConnectSelection(func(ids []string) {
		if slices.Equal(ids, []string{"root"}) {
			m.Remove("root")
		}
	})
	v.ConnectSelection(func(ids []string) {
		for _, id := range ids {
			if _, exists := m.Parent(id); !exists {
				t.Fatalf("notification contains deleted node %q", id)
			}
		}
		selections = append(selections, ids)
	})
	m.Remove("a")
	checkTreeState(t, v, "other", "other")
	if len(selections) != 1 || !slices.Equal(selections[0], []string{"other"}) {
		t.Fatalf("final state not delivered: %v", selections)
	}
}

type treeRightClickController struct {
	gui.EventControllerBase
	calls   *int
	consume bool
}

func (c *treeRightClickController) HandleEvent(ctx gui.EventContext) {
	if e, ok := ctx.Event().(events.PointerEvent); ok && e.EventType == events.PointerDown && e.Button == events.PointerButtonRight {
		*c.calls++
		if c.consume {
			ctx.StopPropagation()
		}
	}
}

func TestTreeViewContextMenuBubblesAfterContent(t *testing.T) {
	for _, consume := range []bool{false, true} {
		t.Run(fmt.Sprint(consume), func(t *testing.T) {
			v, _ := treeFixture()
			childCalls, queries := 0, 0
			v.SetDelegate(treeContentDelegate{setup: func() gui.Widget {
				w := splitTestChild(100, 20)
				w.AddEventController(&treeRightClickController{
					EventControllerBase: gui.NewEventControllerBase(gui.PhaseTarget), calls: &childCalls, consume: consume,
				})
				return w
			}})
			v.ConnectContextMenu(func(id string, _ *gui.MenuModel) {
				queries++
				if childCalls != 1 || id != "root" {
					t.Fatal("tree queried before child or targeted the wrong row")
				}
			})
			treeLayout(v, 100, 0)
			dispatcher, host := new(gui.EventDispatcher), &tabInputHost{root: v}
			err := dispatcher.DispatchEvent(host, events.PointerEvent{
				EventType: events.PointerDown, Button: events.PointerButtonRight,
				Buttons: events.PointerButtonRightDown, Position: geometry.Point{X: 60, Y: 14},
			})
			if runtime.GOOS == "windows" && !consume {
				err = dispatcher.DispatchEvent(host, events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonRight, Position: geometry.Point{X: 60, Y: 14}})
			}
			wantQueries := 1
			if consume {
				wantQueries = 0
			}
			if err != nil || childCalls != 1 || queries != wantQueries {
				t.Fatalf("err=%v child=%d tree=%d", err, childCalls, queries)
			}
		})
	}
}

type treePaintRecorder struct {
	gui.Painter
	fills, borders int
	ink            graphics.Brush
}

func (p *treePaintRecorder) FillRoundRect(geometry.Rectangle, float32, graphics.Brush) { p.fills++ }
func (p *treePaintRecorder) DrawRoundRect(geometry.Rectangle, float32, float32, graphics.Brush) {
	p.borders++
}
func (p *treePaintRecorder) DrawLine(_ geometry.Point, _ geometry.Point, _ float32, ink graphics.Brush) {
	p.ink = ink
}

func TestTreeViewCurrentOnlyPaintsOutlineAndDisclosureUsesOwnState(t *testing.T) {
	old := gui.App
	defer func() { gui.App = old }()
	// current 故意不指定透明背景：继承到的默认背景也不能覆盖 selected。
	gui.App = &separatorApp{sheet: style.Sheet(
		style.Name("tree-item").Radius(4).BackgroundColor(color.White),
		style.Name("tree-item").Part("selected").BackgroundColor(color.Black),
		style.Name("tree-item").Part("current").BorderWidth(1).BorderColor(color.Black),
		style.Name("tree-expander").ForegroundColor(color.Black),
		style.Name("tree-expander").State(style.Hovered).ForegroundColor(color.RGBA{R: 255, A: 255}),
		style.Name("tree-expander").State(style.Pressed).ForegroundColor(color.RGBA{B: 255, A: 255}),
	)}
	v, _ := treeFixture()
	v.choose("root", false, false)
	treeLayout(v, 100, 0)
	host, dispatcher := &tabInputHost{root: v}, new(gui.EventDispatcher)
	host.SetFocusedWidget(v)
	// Focus crossing uses the same dispatcher route as real input.
	if err := dispatcher.DispatchEvent(host, events.FocusEvent{Focused: true}); err != nil {
		t.Fatal(err)
	}
	p := new(treePaintRecorder)
	r := v.realized["root"]
	r.Paint(p)
	if p.fills != 1 || p.borders != 1 {
		t.Fatalf("fills=%d borders=%d focused=%v", p.fills, p.borders, v.Focused())
	}
	dispatchTabPointer(t, dispatcher, host, events.PointerMove, 14, 14)
	r.content.(*TreeExpander).button.Paint(p)
	if p.ink != graphics.ColorOf(color.RGBA{R: 255, A: 255}) {
		t.Fatal("disclosure ignored hover")
	}
	dispatchTabPointer(t, dispatcher, host, events.PointerDown, 14, 14)
	r.content.(*TreeExpander).button.Paint(p)
	if p.ink != graphics.ColorOf(color.RGBA{B: 255, A: 255}) {
		t.Fatal("disclosure ignored press")
	}
	dispatchTabPointer(t, dispatcher, host, events.PointerUp, 14, 14)
	v.LayoutVisible(geometry.Size{Width: 300, Height: 100}, geometry.Point{})
	for _, row := range v.realized {
		if row.content.(*TreeExpander).button.pressed {
			t.Fatal("recycled disclosure retained pressed state")
		}
	}
}
