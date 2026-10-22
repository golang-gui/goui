// TableView records, virtualization, reconciliation and Modern appearance
// window checks, without DevServer.
//
// Environment: interactive Linux/X11, Windows or macOS. Record OS, Linux WM,
// actual backend and scale. Run: go run ./tests/window/widgets/table
// Default UI; -gui uses imperative controls, -fallback-style combines explicit
// defaults. -appearance=plain/zebra/rows/grid/thick selects initial appearance
// (default plain); Appearance cycles it. thick checks wide lines/custom content.
// Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows),
// GOUI_PLAT_SCALE=2. Initially Native 880x620 DIP, Modern light, 10,000 records,
// multiselection and five fixed-width columns. Name/Size sort; Notes is an input,
// descriptions wrap, and some Action cells contain Inspect. Content fills cells;
// ordinary text uses explicit container padding.
// Steps and expected results:
//  1. Click a row, Primary+click for multiselection (Command on macOS, Control
//     elsewhere), Shift for ranges. Arrows/Home/End/Page navigate; Primary+arrows
//     move only Current, Primary+A selects all. Enter/double-click activates;
//     Inspect emits only cell-action without selection changes.
//  2. Drag header right edges with horizontal resize cursors. Descriptions rewrap;
//     header/body boundaries agree. Name min/max are 80/480 DIP. Esc restores the
//     initial width and logs rollback; release neither sorts nor retains pressed
//     emphasis. UI rebuild retains user widths.
//  3. Name/Size clicks alternate ascending/descending with sort logs and arrows
//     matching data. Selection/Current retain IDs. Other headers do not sort.
//  4. Narrow and scroll horizontally: header/body synchronize. Vertical scrolling
//     keeps the header fixed. Fast 10,000-row scrolling leaves no stale text/press
//     fills. Reveal last targets record-09999 by ID rather than assuming last index.
//  5. Right-click a selected row retains multiselection; another row becomes
//     selected. Menu shows row/column IDs. Shift+F10 anchors at visible Current;
//     also test macOS Control+click. Delete removes the record with nearby Current
//     fallback. Empty body gets background; headers/scrollbars do not query row
//     menus. Failed presentation logs menu-error.
//  6. Rebuild (GUI Refresh) updates revision but retains selection, Current,
//     scroll and widths. Toggle dark changes neutral appearance; text resolves
//     its own table style and selection is not an accent fill. Modern header
//     differs subtly from body, with 1 DIP outer/header-bottom lines stronger
//     than row lines. Selection uses TreeView's neutral palette. Pointer selection
//     adds no current outline; Tab away and Shift+Tab back adds a 1 DIP neutral
//     outline that survives hover and clears on clicking the same row. First
//     return to the top so Current is visible. Explicit fallback keeps its
//     existing pointer-current outline and is outside this Modern convention.
//  7. Snapshot prints ASSERT PASS for logical rows/columns, order, current/selected
//     and virtual row count. These semantics do not verify input/appearance.
//     Repeat main operations at 1x/2x.
//  8. Cycle plain (no inner lines), zebra, rows (horizontal lines), grid (full grid
//     plus stripes), thick (12 DIP vertical/6 DIP horizontal lines, separate border).
//     The outer border stays visible around header/body/bars, vertical boundaries
//     align and intersections do not darken. Cyan custom Action widgets fill
//     their rectangles without lines covering their edges. Line-width changes
//     rewrap descriptions; stripes follow row order through scroll/rebuild/sort.
//     Cyan first paint logs backend/pixel-scale. Snapshot uses RenderWidget and
//     samples all cyan corners in memory to check custom edges; neither this
//     offscreen check nor startup is a frame-rate or native-appearance result.
//  9. Edit Notes: a square 2 DIP accent border fills the cell without outside
//     margins, retaining internal text padding. Blur removes it without a double
//     border. A tall wrapped row keeps the input full-height and text centered;
//     editing does not select the row. F6 snapshots without changing focus and
//     asserts text/bounds, sampling the current input's four border corners
//     offscreen. Inspect visible edges, clipping and HiDPI separately. Sort, scroll
//     away/back, rebuild/refresh and dark mode retain values by RowID, never
//     another record. Notes right-click/Shift+F10 opens its text menu, not a row
//     menu. Grid/thick lines adjacent to the editor must not cover it.
//
// Platform differences: Primary/context-menu timing follow platform event rules.
// Tab traverses mounted controls, without navigation across all virtual records.
// Column reordering and table editing transactions are not implemented here.
// Side effects: window, in-memory data and logs; no files/network. Explicit text
// copy/cut/paste accesses the clipboard. Reset: restart. Exit: close the window;
// failed assertions panic and return nonzero.
package main

import (
	"flag"
	"fmt"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	widgetstyle "github.com/golang-gui/goui/widgets/style"
	wui "github.com/golang-gui/goui/widgets/ui"
	"image"
	"image/color"
	"log"
	"math"
	"runtime"
	"slices"
	"strings"
)

type record struct {
	id, name, description string
	size                  int
	action                bool
}
type model struct {
	*gui.SliceListModel[record]
	notes map[string]string
}

func (m *model) RowID(i int) string { return m.ItemAt(i).id }
func newModel() *model {
	rows := make([]record, 10000)
	for i := range rows {
		description := "A record with fixed column widths and stable identity."
		if i%7 == 0 {
			description = "Long description: wrapping changes this row's height, without changing column allocation.\nA second paragraph remains visible after resizing."
		}
		rows[i] = record{fmt.Sprintf("record-%05d", i), fmt.Sprintf("Document %05d.txt", i), description, (i * 7919) % 100000, i%5 == 0}
	}
	return &model{SliceListModel: gui.NewSliceListModel(rows), notes: make(map[string]string)}
}
func (m *model) note(id string) string {
	if text, ok := m.notes[id]; ok {
		return text
	}
	return "Note " + strings.TrimPrefix(id, "record-")
}
func (m *model) setNote(id, text string) {
	if m.note(id) != text {
		m.notes[id] = text
		fmt.Printf("cell-edit=%s:%q\n", id, text)
	}
}
func (m *model) sort(column string, order widgets.SortOrder) {
	m.Modify(func(rows []record) []record {
		slices.SortStableFunc(rows, func(a, b record) int {
			result := strings.Compare(a.name, b.name)
			if column == "size" {
				result = a.size - b.size
			}
			if order == widgets.SortDescending {
				return -result
			}
			return result
		})
		return rows
	})
	fmt.Printf("sort=%s:%d\n", column, order)
}
func (m *model) remove(id string) {
	delete(m.notes, id)
	m.Modify(func(rows []record) []record {
		return slices.DeleteFunc(rows, func(row record) bool { return row.id == id })
	})
}

var appearances = []string{"plain", "zebra", "rows", "grid", "thick"}

func sheet(dark, fallback bool, appearance int) style.StyleSheet {
	rules := modern.Rules(modern.Options{Dark: dark})
	if fallback {
		rules = append(gui.DefaultStyleRules(), widgetstyle.Rules()...)
	}
	// Copy the editor's rules under an independent name. This is explicit
	// composition, not style inheritance from the cell or another style name.
	for _, rule := range rules {
		if rule.Sel.Name == "text-input" {
			rule.Sel.Name = "table-cell-input"
			rules = append(rules, rule)
		}
	}
	rules = append(rules,
		style.Name("table-cell-input").BackgroundColor(color.Transparent).BorderWidth(0).Radius(0),
		style.Name("table-cell-input").State(style.Focused).BorderWidth(2).Radius(0),
	)
	h, w := float32(0), float32(0)
	if appearance >= 2 {
		h = 1
	}
	if appearance >= 3 {
		w = 1
	}
	alternate := color.Color(color.Transparent)
	if appearance == 1 || appearance >= 3 {
		alternate = color.RGBA{R: 235, G: 235, B: 239, A: 255}
		if dark && !fallback {
			alternate = color.RGBA{R: 43, G: 43, B: 47, A: 255}
		}
	}
	if appearance == 4 {
		h, w = 6, 12
		rules = append(rules,
			style.Name("table-view").Part("outer-horizontal").BorderWidth(3),
			style.Name("table-view").Part("outer-vertical").BorderWidth(6),
			style.Name("table-grid").BorderColor(color.NRGBA{R: 90, G: 110, B: 140, A: 140}),
		)
	}
	rules = append(rules,
		style.Name("table-grid").Part("horizontal").BorderWidth(h),
		style.Name("table-grid").Part("vertical").BorderWidth(w),
		style.Name("table-row").Part("alternate").BackgroundColor(alternate).Radius(0),
	)
	return style.Sheet(rules...)
}
func assertTable(table *widgets.TableView, m *model) {
	info := table.Snapshot()
	columns := table.Columns()
	count := len(columns)
	tableInfo, ok := info.Attributes[widgets.TableInfoKey].(widgets.TableInfo)
	if count != 5 || info.Role != widgets.RoleTable || !ok || tableInfo.ColumnCount != count || tableInfo.RowCount != table.Model().ItemsCount() || len(info.Children) < count || len(info.Children) > 90 {
		panic("table extent/virtualization")
	}
	seen := make(map[string]bool)
	for i, header := range info.Children[:count] {
		h, ok := header.Attributes[widgets.TableInfoKey].(widgets.TableInfo)
		if header.Role != widgets.RoleColumnHeader || !ok || h.ColumnID != columns[i].ID() || h.ColumnIndex != i+1 {
			panic("header semantics")
		}
	}
	for _, row := range info.Children[count:] {
		r, ok := row.Attributes[widgets.TableInfoKey].(widgets.TableInfo)
		if row.Role != widgets.RoleTableRow || !ok || r.RowID == "" || seen[r.RowID] || len(row.Children) != count {
			panic("row semantics")
		}
		seen[r.RowID] = true
		if r.RowIndex < 1 || r.RowIndex > tableInfo.RowCount || table.Model().RowID(r.RowIndex-1) != r.RowID || r.Current != (r.RowID == table.Current()) || row.Selected != slices.Contains(table.Selection(), r.RowID) {
			panic("row identity/state")
		}
		for i, cell := range row.Children {
			c, ok := cell.Attributes[widgets.TableInfoKey].(widgets.TableInfo)
			if cell.Role != widgets.RoleTableCell || !ok || c.ColumnID != columns[i].ID() || c.RowID != r.RowID || c.ColumnIndex != i+1 || cell.Bounds.Width != columns[i].Width() || cell.Bounds.X != info.Children[i].Bounds.X {
				panic("cell/header geometry")
			}
			if c.ColumnID == "note" {
				input, found := editorInfo(cell)
				line, _ := gui.ResolveStyle("table-grid", "vertical", style.Normal).BorderWidth()
				want := cell.Bounds
				want.Width = max(0, want.Width-line)
				if !found || !input.Focusable || input.Bounds != want || input.Text != m.note(r.RowID) {
					panic(fmt.Sprintf("cell editor %s: bounds=%v want=%v text=%q want=%q", r.RowID, input.Bounds, want, input.Text, m.note(r.RowID)))
				}
				if input.Focused {
					fmt.Printf("focused-editor=%s bounds=%v text=%q\n", r.RowID, input.Bounds, input.Text)
				}
			}
		}
	}
	// Exercise the real GUI traversal and backend, not a separate test renderer.
	img, err := gui.RenderWidget(table, 1)
	if err != nil {
		panic(err)
	}
	var checkContent func(gui.Widget)
	checkContent = func(widget gui.Widget) {
		var ink color.RGBA
		check := false
		switch content := widget.(type) {
		case *paintedCell:
			check = content.Visible()
			ink = color.RGBA{G: 150, B: 170, A: 255}
		case *gui.TextInput:
			check = content.Visible() && content.Focused()
			if check {
				s := gui.ResolveStyle(content.StyleName(), "", style.Focused)
				width, _ := s.BorderWidth()
				radius, _ := s.Radius()
				border, _ := s.BorderColor()
				if width != 2 || radius != 0 || border == nil {
					panic("focused editor must have a 2 DIP square border")
				}
				ink = color.RGBAModel.Convert(border).(color.RGBA)
			}
		}
		if check {
			r := widget.Snapshot().Bounds
			scope := r
			for parent := widget.Parent(); parent != nil; parent = parent.Parent() {
				scope = scope.Intersect(parent.Snapshot().Bounds)
				if parent == table {
					break
				}
			}
			if r.Width >= 3 && r.Height >= 3 {
				for _, point := range []geometry.Point{{X: r.X + 1, Y: r.Y + 1}, {X: r.X + r.Width - 2, Y: r.Y + 1}, {X: r.X + 1, Y: r.Y + r.Height - 2}, {X: r.X + r.Width - 2, Y: r.Y + r.Height - 2}} {
					if point.X < scope.X || point.Y < scope.Y || point.X >= scope.X+scope.Width || point.Y >= scope.Y+scope.Height {
						continue
					}
					x, y := int(math.Floor(float64(point.X-info.Bounds.X))), int(math.Floor(float64(point.Y-info.Bounds.Y)))
					if got := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA); got != ink {
						panic(fmt.Sprintf("cell content edge (%d,%d)=%v want=%v", x, y, got, ink))
					}
				}
			}
		}
		for _, child := range widget.Children() {
			checkContent(child)
		}
	}
	checkContent(table)
	fmt.Printf("ASSERT PASS logical=%d realized=%d current=%s selection=%v scroll=(%g,%g)\n", tableInfo.RowCount, len(seen), table.Current(), table.Selection(), info.ScrollX, info.ScrollY)
}
func editorInfo(info gui.WidgetInfo) (gui.WidgetInfo, bool) {
	if info.Role == gui.RoleTextInput {
		return info, true
	}
	for _, child := range info.Children {
		if editor, found := editorInfo(child); found {
			return editor, true
		}
	}
	return gui.WidgetInfo{}, false
}

func textCellUI(text string, wrap bool) ui.View {
	label := ui.Label(text).Style("table-cell-text").MainWeight(1)
	if wrap {
		label.WrapMode(gui.WrapWordChar)
	}
	return ui.HBox(label).Padding(4)
}
func menuItems(m *model, row, column string) []*ui.MenuItemView {
	if row == "" {
		return []*ui.MenuItemView{ui.MenuItem("background / "+column, nil).Enabled(false)}
	}
	return []*ui.MenuItemView{ui.MenuItem(row+" / "+column, nil).Enabled(false), ui.MenuItem("Activate", func() { fmt.Printf("menu-activate=%s\n", row) }), ui.MenuItem("Delete", func() { m.remove(row) })}
}
func main() {
	guiMode := flag.Bool("gui", false, "use imperative GUI")
	fallback := flag.Bool("fallback-style", false, "compose fallback style")
	appearance := flag.String("appearance", "plain", "plain/zebra/rows/grid/thick")
	flag.Parse()
	look := slices.Index(appearances, *appearance)
	if look < 0 {
		log.Fatal("invalid -appearance")
	}
	var err error
	if *guiMode {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		err = runGUI(*fallback, look)
	} else {
		err = runUI(*fallback, look)
	}
	if err != nil {
		log.Fatal(err)
	}
}
func runUI(fallback bool, appearance int) error {
	m := newModel()
	dark, version := false, 0
	widths := map[string]float32{"name": 240, "note": 200, "action": 100, "size": 140, "description": 420}
	return ui.Run("org.golang-gui.TableViewProbe", func(app ui.App) ui.RootView {
		column := func(id, title string, cell func(widgets.TableRow, record) ui.View) *wui.TableColumnView[record] {
			// Controlled width lives in application state, including Esc rollback.
			return wui.TableColumn(id, title, cell).Width(widths[id]).OnResize(func(w float32) {
				widths[id] = w
				fmt.Printf("resize=%s:%g\n", id, w)
			})
		}
		name := column("name", "Name", func(_ widgets.TableRow, row record) ui.View {
			return textCellUI(fmt.Sprintf("%s [v%d]", row.name, version), false)
		}).MinWidth(80).MaxWidth(480).Sortable(true)
		note := column("note", "Notes", func(row widgets.TableRow, _ record) ui.View {
			return ui.TextInput().Style("table-cell-input").Text(m.note(row.ID)).Padding(4).
				OnText(func(text string) { m.setNote(row.ID, text) })
		}).MinWidth(80)
		action := column("action", "Action", func(row widgets.TableRow, value record) ui.View {
			if value.action {
				return ui.HBox(ui.Button("Inspect").OnClick(func() { fmt.Printf("cell-action=%s\n", row.ID) })).Padding(4)
			}
			if row.Index%7 == 0 {
				return paintedCellUI()
			}
			return nil
		}).MinWidth(80)
		size := column("size", "Size", func(_ widgets.TableRow, row record) ui.View {
			return textCellUI(fmt.Sprintf("%d KB", row.size), false)
		}).Sortable(true)
		description := column("description", "Description", func(_ widgets.TableRow, row record) ui.View {
			return textCellUI(row.description, true)
		})
		table := wui.TableView(m, name, note, action, size, description).ID("table").SelectionMode(widgets.SelectionMultiple).
			OnSelection(func(ids []string) { fmt.Printf("selection=%v\n", ids) }).OnCurrent(func(id string) { fmt.Printf("current=%s\n", id) }).
			OnActivate(func(id string) { fmt.Printf("activate=%s\n", id) }).
			OnSortRequest(func(id string, order widgets.SortOrder) {
				m.sort(id, order)
				app.FindWidget("table-window", "table").(*widgets.TableView).SetSort(id, order)
			}).
			ContextMenu(func(row, column string) []*ui.MenuItemView { return menuItems(m, row, column) }).
			OnContextMenuError(func(err error) { fmt.Printf("menu-error=%v\n", err) })
		verify := func() { assertTable(app.FindWidget("table-window", "table").(*widgets.TableView), m) }
		return ui.Root().StyleSheet(sheet(dark, fallback, appearance)).Windows(ui.Window("table-window").Title("GOUI TableView UI").Size(880, 620).MinSize(400, 240).
			Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(verify)).Content(
			ui.VBox(ui.HBox(
				ui.Button("Rebuild").OnClick(func() { version++; app.RequestUpdate() }),
				ui.Button("Toggle dark").OnClick(func() { dark = !dark; app.RequestUpdate() }),
				ui.Button("Reveal last").OnClick(func() { app.FindWidget("table-window", "table").(*widgets.TableView).Reveal("record-09999") }),
				ui.Button("Snapshot").OnClick(verify),
				ui.Button("Appearance").OnClick(func() {
					appearance = (appearance + 1) % len(appearances)
					fmt.Printf("appearance=%s\n", appearances[appearance])
					app.RequestUpdate()
				}),
			).Spacing(8), ui.Label("Edit Notes / F6 verify; sortable Name / Size; resize edges / Esc; Shift+F10 menu"), table).CrossAlign(layout.CrossStretch).Padding(12).Spacing(8),
		))
	})
}

type cellDelegate struct {
	model   *model
	column  string
	version *int
}
type actionCell struct {
	*gui.LinearBox
	id      string
	button  *gui.Button
	painted *paintedCell
}
type textCell struct {
	*gui.LinearBox
	label *gui.Label
}
type noteCell struct {
	gui.WidgetBase
	id      string
	input   *gui.TextInput
	binding bool
}

// Deliberately paint the full allocated area. The table, not this Widget,
// must allocate a safe content rectangle around arbitrarily thick grid lines.
type paintedCell struct {
	gui.WidgetBase
	reported bool
}

func (*paintedCell) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: c.Max.Width, Height: 24}))
}
func (w *paintedCell) Paint(p gui.Painter) {
	if !w.reported {
		resource, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		if err != nil {
			panic(err)
		}
		fmt.Printf("backend=%T pixel-scale=%g\n", resource, p.PixelScale())
		resource.Destroy()
		w.reported = true
	}
	p.FillRect(geometry.Rectangle{Size: w.Rect().Size}, graphics.ColorOf(color.RGBA{G: 150, B: 170, A: 255}))
}

type paintedCellView struct{ ui.ViewBase[paintedCellView] }

func paintedCellUI() *paintedCellView {
	v := new(paintedCellView)
	v.Self = v
	return v
}
func (v *paintedCellView) Build() ui.View                    { return v }
func (*paintedCellView) Mount(ui.BuildContext) gui.Widget    { return new(paintedCell) }
func (*paintedCellView) Update(ui.BuildContext, gui.Widget)  {}
func (*paintedCellView) Unmount(ui.BuildContext, gui.Widget) {}

func (d *cellDelegate) Setup() gui.Widget {
	if d.column == "note" {
		c := &noteCell{input: gui.NewTextInput()}
		c.SetLayoutManager(layout.NewFillLayout())
		c.WidgetBase.AddChild(c, c.input)
		c.input.SetStyleName("table-cell-input")
		c.input.SetPadding(4)
		c.input.ConnectText(func(text string) {
			if c.id != "" && !c.binding {
				d.model.setNote(c.id, text)
			}
		})
		return c
	}
	if d.column == "action" {
		c := &actionCell{LinearBox: gui.NewLinearBox(layout.DirectionVertical), button: gui.NewButton(), painted: new(paintedCell)}
		c.SetCrossAlign(layout.CrossStretch)
		c.SetMainAlign(layout.MainCenter)
		c.SetPadding(4)
		c.button.SetChild(gui.NewLabel("Inspect"))
		c.button.ConnectClicked(func() { fmt.Printf("cell-action=%s\n", c.id) })
		c.AddChild(c.button)
		c.AddChild(c.painted)
		return c
	}
	label := gui.NewLabel("")
	label.SetStyleName("table-cell-text")
	label.SetMainWeight(1)
	if d.column == "description" {
		label.SetWrapMode(gui.WrapWordChar)
	}
	c := &textCell{LinearBox: gui.NewLinearBox(layout.DirectionHorizontal), label: label}
	c.SetPadding(4)
	c.AddChild(label)
	return c
}
func (d *cellDelegate) Bind(row widgets.TableRow, widget gui.Widget) {
	if c, ok := widget.(*noteCell); ok {
		c.id = row.ID
		c.binding = true
		c.input.SetText(d.model.note(row.ID))
		c.binding = false
		return
	}
	value := d.model.ItemAt(row.Index)
	if c, ok := widget.(*actionCell); ok {
		c.id = row.ID
		painted := !value.action && row.Index%7 == 0
		c.button.SetVisible(value.action)
		c.painted.SetVisible(painted)
		c.SetVisible(value.action || painted)
		return
	}
	text := value.description
	switch d.column {
	case "name":
		text = fmt.Sprintf("%s [v%d]", value.name, *d.version)
	case "size":
		text = fmt.Sprintf("%d KB", value.size)
	}
	widget.(*textCell).label.SetText(text)
}
func (*cellDelegate) Unbind(_ widgets.TableRow, widget gui.Widget) {
	if c, ok := widget.(*actionCell); ok {
		c.id = ""
	}
	if c, ok := widget.(*noteCell); ok {
		c.id = ""
	}
}
func runGUI(fallback bool, appearance int) error {
	app, err := gui.NewApplication("org.golang-gui.TableViewGUIProbe")
	if err != nil {
		return err
	}
	app.SetStyleSheet(sheet(false, fallback, appearance))
	window, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 880, Height: 620}})
	if err != nil {
		return err
	}
	defer window.Destroy()
	if err = window.SetTitle("GOUI TableView GUI"); err != nil {
		return err
	}
	window.SetMinSize(geometry.Size{Width: 400, Height: 240})
	m := newModel()
	table := widgets.NewTableView()
	version, dark := 0, false
	var columns []*widgets.TableColumn
	for i, id := range []string{"name", "note", "action", "size", "description"} {
		c := widgets.NewTableColumn(id, []string{"Name", "Notes", "Action", "Size", "Description"}[i])
		c.SetWidth([]float32{240, 200, 100, 140, 420}[i])
		c.SetDelegate(&cellDelegate{m, id, &version})
		c.SetSortable(id == "name" || id == "size")
		if id == "name" {
			c.SetMinWidth(80)
			c.SetMaxWidth(480)
		}
		c.ConnectResize(func(w float32) { fmt.Printf("resize=%s:%g\n", id, w) })
		columns = append(columns, c)
	}
	table.SetColumns(columns...)
	table.SetModel(m)
	table.SetSelectionMode(widgets.SelectionMultiple)
	table.ConnectSelection(func(ids []string) { fmt.Printf("selection=%v\n", ids) })
	table.ConnectCurrent(func(id string) { fmt.Printf("current=%s\n", id) })
	table.ConnectActivate(func(id string) { fmt.Printf("activate=%s\n", id) })
	table.ConnectSortRequest(func(id string, order widgets.SortOrder) { m.sort(id, order); table.SetSort(id, order) })
	table.ConnectContextMenu(func(row, column string, result *gui.MenuModel) {
		menu := gui.NewMenu()
		menu.Append(row+" / "+column, nil).SetEnabled(false)
		if row != "" {
			menu.Append("Activate", func() { fmt.Printf("menu-activate=%s\n", row) })
			menu.Append("Delete", func() { m.remove(row) })
		}
		*result = menu
	})
	table.ConnectContextMenuError(func(err error) { fmt.Printf("menu-error=%v\n", err) })
	verify := func() { assertTable(table, m) }
	shortcut := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyF6})
	shortcut.ConnectActivate(verify)
	window.Shortcuts().AddShortcut(shortcut)
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
	button("Refresh", func() { version++; table.Refresh() })
	button("Toggle dark", func() { dark = !dark; app.SetStyleSheet(sheet(dark, fallback, appearance)) })
	button("Reveal last", func() { table.Reveal("record-09999") })
	button("Snapshot", verify)
	button("Appearance", func() {
		appearance = (appearance + 1) % len(appearances)
		fmt.Printf("appearance=%s\n", appearances[appearance])
		app.SetStyleSheet(sheet(dark, fallback, appearance))
	})
	root.AddChild(toolbar)
	root.AddChild(gui.NewLabel("Edit Notes / F6 verify; sortable Name / Size; resize edges / Esc; Shift+F10 menu"))
	root.AddChild(table)
	window.SetWidget(root)
	if err = window.Show(); err != nil {
		return err
	}
	app.Run()
	return nil
}
