// TreeView model, virtualization, keyboard and binding window checks, without DevServer.
//
// Environment: interactive Linux/X11, Windows or macOS. Record OS, Linux WM,
// actual backend and scale. Run from the repository root:
//
//	go run ./tests/window/widgets/tree_view
//
// Default UI, -gui for imperative controls; -fallback-style uses explicit defaults.
// Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows),
// GOUI_PLAT_SCALE=2. Initially Native 920x640 DIP, Modern light, multiselect tree
// with no selection. Project is expanded; src, delayed and 10,000-row branches
// are collapsed. src has ordinary, two-line and embedded-button rows.
// Steps and expected results:
//  1. Row clicks select; arrows only expand/collapse. Double-click a leaf under
//     src: one activate log. Double-click a branch only toggles expansion.
//  2. Control/Command+click multiselects; Shift+click/arrows extends and reversibly
//     contracts ranges. Primary+arrows moves Current only, Primary+Space toggles
//     selection. Home/End/PageUp/PageDown/left/right navigate; Primary+A selects
//     expanded nodes only. Collapsing Current's ancestor moves Current/selection
//     to it while preserving selected nodes outside that branch.
//  3. Delayed emits one load and adds loaded child after about 500 ms. Collapse
//     while loading: completion does not reopen it. Re-expansion may request
//     again but must not insert duplicates.
//  4. Expand 10,000 rows and wheel: no stale text/selection/arrows. Reveal last
//     logs reveal and locates the end; later upward scrolling is not pulled back.
//     Narrow for horizontal scrolling; vertical reveal retains horizontal offset.
//  5. src row action logs only row-action without selecting/dragging. Row Enter
//     activates; embedded button focus retains its own keyboard input.
//  6. Right-click selection retains multiselect; another row becomes selected.
//     Menu identifies its ID. Activate logs it; Delete removes node/descendants
//     with reasonable Current fallback. Shift+F10 uses Current; also test macOS
//     Control+click. Empty space gets tree background.
//  7. Rebuild (GUI Refresh) retains selection, Current, expansion and offsets
//     while updating row revision text. Toggle dark preserves state. Selected
//     hover/press subtly changes fill; arrow states respond only to arrow input.
//     Release/recycling clears pressed fills; Current's border leaves selection
//     visible.
//  8. Snapshot prints ASSERT PASS for hierarchy, sibling indices, state and virtual
//     row count. These semantics do not replace real input/appearance checks.
//
// Platform differences: Primary follows the platform; activate macOS first.
// Long rows scroll at natural width rather than wrapping. GUI Refresh corresponds
// to a same-model UI rebuild. Side effects: window, in-memory data, timers/logs;
// no disk/clipboard/network. Reset: restart. Exit: close; semantic failures panic
// and return nonzero.
package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"slices"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	widgetstyle "github.com/golang-gui/goui/widgets/style"
	wui "github.com/golang-gui/goui/widgets/ui"
)

type entry struct {
	title  string
	action bool
}
type model struct {
	*widgets.TreeStore[entry]
	pending bool
	delay   func(func())
}

func newModel() *model {
	m := &model{TreeStore: widgets.NewTreeStore[entry]()}
	m.Modify(func(s *widgets.TreeStore[entry]) {
		s.Append("", "project", entry{title: "Project"})
		s.Append("project", "src", entry{title: "src"})
		s.Append("src", "main", entry{title: "main.go"})
		s.Append("src", "multiline", entry{title: "Two-line row\nSecond line remains fully visible"})
		s.Append("src", "action", entry{title: "Interactive row", action: true})
		s.Append("project", "readme", entry{title: "README.md"})
		s.Append("", "delayed", entry{title: "Delayed (500 ms)"})
		s.SetExpandable("delayed", true)
		s.Append("", "large", entry{title: "10000 rows"})
		for i := 0; i < 10000; i++ {
			s.Append("large", fmt.Sprintf("record-%05d", i), entry{title: fmt.Sprintf("Record %05d", i)})
		}
		s.Append("", "long", entry{title: "Long natural-width row: abcdefghijklmnopqrstuvwxyz 0123456789 — horizontal scrolling preserves the complete text"})
	})
	return m
}
func (m *model) RequestChildren(id string) {
	if id != "delayed" {
		return
	}
	fmt.Printf("load-request=%s\n", id)
	if m.pending || len(m.Children(id)) != 0 || m.delay == nil {
		return
	}
	m.pending = true
	m.delay(func() {
		m.pending = false
		if _, exists := m.Parent(id); exists && len(m.Children(id)) == 0 {
			m.Append(id, "loaded", entry{title: "loaded child"})
		}
	})
}
func sheet(dark, fallback bool) style.StyleSheet {
	if fallback {
		return style.Sheet(append(gui.DefaultStyleRules(), widgetstyle.Rules()...)...)
	}
	return modern.Sheet(modern.Options{Dark: dark})
}
func assertTree(tree *widgets.TreeView) {
	info := tree.Snapshot()
	if info.Role != widgets.RoleTree || info.ItemCount < len(info.Children) || len(info.Children) > 80 {
		panic("tree snapshot/virtualization incorrect")
	}
	seen := make(map[string]bool)
	for _, row := range info.Children {
		h, ok := row.Attributes[widgets.HierarchyInfoKey].(widgets.HierarchyInfo)
		if row.Role != widgets.RoleTreeItem || !ok || h.NodeID == "" || seen[h.NodeID] || h.Level < 1 || h.PositionInSet < 1 || h.PositionInSet > h.SetSize || row.Focused {
			panic("row hierarchy/focus incorrect")
		}
		seen[h.NodeID] = true
		if h.Current != (tree.Current() == h.NodeID) {
			panic("current snapshot incorrect")
		}
	}
	fmt.Printf("ASSERT PASS logical=%d realized=%d current=%s selection=%v\n", info.ItemCount, len(info.Children), tree.Current(), tree.Selection())
}
func main() {
	guiMode := flag.Bool("gui", false, "use imperative GUI")
	fallback := flag.Bool("fallback-style", false, "use explicitly composed fallback styles")
	flag.Parse()
	var err error
	if *guiMode {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		err = runGUI(*fallback)
	} else {
		err = runUI(*fallback)
	}
	if err != nil {
		log.Fatal(err)
	}
}
func runUI(fallback bool) error {
	m := newModel()
	version, dark := 0, false
	expanded := []string{"project"}
	return ui.Run("org.golang-gui.TreeViewProbe", func(app ui.App) ui.RootView {
		m.delay = func(fn func()) { app.TimeoutFunc(500*time.Millisecond, fn) }
		tree := wui.TreeView(m, func(row widgets.TreeRow, value entry) ui.View {
			label := ui.Label(fmt.Sprintf("%s [v%d]", value.title, version)).Style("tree-item-text")
			if value.action {
				return ui.HBox(label, ui.Button("row action").OnClick(func() { fmt.Printf("row-action=%s\n", row.ID) })).Spacing(8)
			}
			return label
		}).ID("tree").SelectionMode(widgets.SelectionMultiple).Expanded(expanded).
			OnSelection(func(ids []string) { fmt.Printf("selection=%v\n", ids) }).
			OnCurrent(func(id string) { fmt.Printf("current=%s\n", id) }).
			OnExpanded(func(id string, value bool) {
				if value {
					if !slices.Contains(expanded, id) {
						expanded = append(expanded, id)
					}
				} else {
					expanded = slices.DeleteFunc(expanded, func(next string) bool { return next == id })
				}
				fmt.Printf("expanded=%s:%v\n", id, value)
				app.RequestUpdate()
			}).
			OnActivate(func(id string) { fmt.Printf("activate=%s\n", id) }).
			ContextMenu(func(id string) []*ui.MenuItemView {
				if id == "" {
					return []*ui.MenuItemView{ui.MenuItem("tree background", nil).Enabled(false)}
				}
				return []*ui.MenuItemView{ui.MenuItem(id, nil).Enabled(false), ui.MenuItem("Activate", func() { fmt.Printf("menu-activate=%s\n", id) }), ui.MenuItem("Delete", func() { m.Remove(id) })}
			}).OnContextMenuError(func(err error) { fmt.Printf("menu-error=%v\n", err) })
		return ui.Root().StyleSheet(sheet(dark, fallback)).Windows(
			ui.Window("tree-window").Title("GOUI TreeView UI").Size(920, 640).MinSize(440, 260).Content(
				ui.VBox(
					ui.HBox(
						ui.Button("Rebuild").OnClick(func() { version++; app.RequestUpdate() }),
						ui.Button("Toggle dark").OnClick(func() { dark = !dark; app.RequestUpdate() }),
						ui.Button("Reveal last").OnClick(func() {
							app.FindWidget("tree-window", "tree").(*widgets.TreeView).Reveal("record-09999")
							fmt.Println("reveal=record-09999")
						}),
						ui.Button("Snapshot").OnClick(func() { assertTree(app.FindWidget("tree-window", "tree").(*widgets.TreeView)) }),
					).Spacing(8),
					ui.Label("Click / Primary / Shift; arrows / Enter / Shift+F10; lazy loading and 10000 rows"),
					ui.ScrollView(tree),
				).CrossAlign(layout.CrossStretch).Padding(12).Spacing(8),
			),
		)
	})
}

type delegate struct {
	model   *model
	version *int
	tree    *widgets.TreeView
}
type rowContent struct {
	*widgets.TreeExpander
	label  *gui.Label
	button *gui.Button
	id     string
}

func (d *delegate) Setup() gui.Widget {
	c := &rowContent{TreeExpander: widgets.NewTreeExpander(), label: gui.NewLabel("")}
	box := gui.NewLinearBox(layout.DirectionHorizontal)
	c.SetChild(box)
	c.ConnectToggle(func() {
		if c.id != "" {
			d.tree.SetExpanded(c.id, !d.tree.Expanded(c.id))
		}
	})
	c.label.SetStyleName("tree-item-text")
	box.SetSpacing(8)
	box.AddChild(c.label)
	c.button = gui.NewButton()
	c.button.SetChild(gui.NewLabel("row action"))
	c.button.ConnectClicked(func() { fmt.Printf("row-action=%s\n", c.id) })
	box.AddChild(c.button)
	return c
}
func (d *delegate) Bind(row widgets.TreeRow, widget gui.Widget) {
	c := widget.(*rowContent)
	value := d.model.Item(row.ID)
	c.id = row.ID
	c.SetDepth(row.Depth)
	c.SetIndentation(d.tree.Indentation())
	c.SetExpandable(row.Expandable)
	c.SetExpanded(row.Expanded)
	c.label.SetText(fmt.Sprintf("%s [v%d]", value.title, *d.version))
	c.button.SetVisible(value.action)
}
func (*delegate) Unbind(_ widgets.TreeRow, widget gui.Widget) { widget.(*rowContent).id = "" }
func runGUI(fallback bool) error {
	app, err := gui.NewApplication("org.golang-gui.TreeViewGUIProbe")
	if err != nil {
		return err
	}
	app.SetStyleSheet(sheet(false, fallback))
	window, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 920, Height: 640}})
	if err != nil {
		return err
	}
	defer window.Destroy()
	if err := window.SetTitle("GOUI TreeView GUI"); err != nil {
		return err
	}
	window.SetMinSize(geometry.Size{Width: 440, Height: 260})
	m := newModel()
	m.delay = func(fn func()) {
		timer := app.NewTimer()
		timer.ConnectTimeout(fn)
		if err := timer.StartOnce(500 * time.Millisecond); err != nil {
			panic(err)
		}
	}
	version, dark := 0, false
	tree := widgets.NewTreeView()
	tree.SetID("tree")
	tree.SetModel(m)
	tree.SetDelegate(&delegate{model: m, version: &version, tree: tree})
	tree.SetSelectionMode(widgets.SelectionMultiple)
	tree.SetExpanded("project", true)
	tree.ConnectSelection(func(ids []string) { fmt.Printf("selection=%v\n", ids) })
	tree.ConnectCurrent(func(id string) { fmt.Printf("current=%s\n", id) })
	tree.ConnectExpanded(func(id string, expanded bool) { fmt.Printf("expanded=%s:%v\n", id, expanded) })
	tree.ConnectActivate(func(id string) { fmt.Printf("activate=%s\n", id) })
	tree.ConnectContextMenu(func(id string, result *gui.MenuModel) {
		menu := gui.NewMenu()
		if id == "" {
			menu.Append("tree background", nil).SetEnabled(false)
		} else {
			menu.Append(id, nil).SetEnabled(false)
			menu.Append("Activate", func() { fmt.Printf("menu-activate=%s\n", id) })
			menu.Append("Delete", func() { m.Remove(id) })
		}
		*result = menu
	})
	tree.ConnectContextMenuError(func(err error) { fmt.Printf("menu-error=%v\n", err) })
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	root.SetPadding(12)
	root.SetSpacing(8)
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetSpacing(8)
	button := func(title string, fn func()) {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(title))
		b.ConnectClicked(fn)
		toolbar.AddChild(b)
	}
	button("Refresh", func() { version++; tree.Refresh() })
	button("Toggle dark", func() { dark = !dark; app.SetStyleSheet(sheet(dark, fallback)) })
	button("Reveal last", func() { tree.Reveal("record-09999"); fmt.Println("reveal=record-09999") })
	button("Snapshot", func() { assertTree(tree) })
	root.AddChild(toolbar)
	root.AddChild(gui.NewLabel("Click / Primary / Shift; arrows / Enter / Shift+F10; lazy loading and 10000 rows"))
	scroll := gui.NewScrollView()
	scroll.SetChild(tree)
	root.AddChild(scroll)
	window.SetWidget(root)
	if err := window.Show(); err != nil {
		return err
	}
	app.Run()
	return nil
}
