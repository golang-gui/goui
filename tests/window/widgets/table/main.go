// TableView 多列记录、虚拟化、声明协调与现代外观验收，不使用 DevServer。
//
// 环境：Linux/X11、Windows 或 macOS 可交互桌面；记录 OS、Linux WM、实际
// 后端与缩放。仓库根目录运行 go run ./tests/window/widgets/table；默认 UI，
// -gui 使用命令式 GUI，-fallback-style 显式组合兜底样式。
// -appearance=plain/zebra/rows/grid/thick 指定初始外观，默认 plain；Appearance
// 按钮循环切换以上模式。thick 为粗线与自定义 Widget 内容避让的专项检查。
// GOUI_PLAT_PAINTER=software/opengl，Windows 另可 direct2d；GOUI_PLAT_SCALE=2
// 请求 2x。初始：Native 880×620 DIP、Modern 亮色、10000 条内存记录、多选、
// 五列固定宽度；名称和大小可排序，Notes 为单行编辑器，说明自动换行，部分
// 操作单元格含 Inspect 按钮。正文内容填满单元格，普通文本自行用容器留白。
//
// 操作及预期：
//  1. 点击行体、Primary（macOS Command，其他 Ctrl）多选、Shift 范围选择；
//     上下/Home/End/Page 导航，Primary+方向键仅改变 Current，Primary+A 全选。
//     Enter 或双击行体输出 activate；点击 Inspect 只输出 cell-action，不改变选择。
//  2. 拖动表头各列右边缘改变宽度，光标为水平调整；说明随宽度重新换行，全部
//     行和表头边界一致。名称最小 80、最大 480 DIP。拖动中按 Esc 恢复起始宽度，
//     输出 resize 回退；松手不会排序，也不残留高亮。UI 重建后保留用户宽度。
//  3. 点击 Name 或 Size 表头，先升序后降序，输出 sort，箭头与实际数据一致；
//     选中记录和 Current 按 ID 保留，不因排序改选别的记录。其他表头不可排序。
//  4. 缩窄窗口并横向滚动，表头与正文同步；纵向滚动表头固定。快速滚动万行，
//     无旧文字/按下背景残留。Reveal last 到记录 record-09999，而非假定最后索引。
//  5. 右击行显示行/列 ID；右击选中行保留多选，未选中行改为选中该行。
//     Shift+F10 在 Current 可见位置弹出菜单；macOS 另验 Control+单击。
//     菜单 Delete 删除该记录，Current 邻近回退；正文空白菜单标为 background，
//     表头和滚动条不查询正文菜单。菜单弹出失败输出 menu-error。
//  6. Rebuild（GUI 为 Refresh）更新版本号，保留选择、Current、滚动及列宽。
//     Toggle dark 切换中性亮暗外观；文字独立使用表格样式名，选中不是主题色底。
//  7. Snapshot 输出 ASSERT PASS：真实逻辑行/列数、顺序、当前/选中和虚拟行数量；
//     此断言只验证语义，不代替实际交互和外观验收。缩放 1x/2x 重复主要操作。
//  8. Appearance 循环 plain（无内部线）、zebra（斑马色）、rows（水平线）、
//     grid（完整网格+斑马色）、thick（12 DIP 竖线/6 DIP 横线/独立外框）。
//     外框不得被表头、正文或滚动条盖住；表头与正文竖线对齐，交点不加深。
//     部分 Action 为填满自身矩形的青色自定义 Widget；thick 下其四边仍完整，
//     不被线条覆盖。切换粗细后说明重新换行，滚动/重建/排序后条纹仍按行序交替。
//     Snapshot 还使用 RenderWidget 重绘实际子树并采样青色块四角（无需截图文件），
//     检查自绘内容边缘；这不是帧率基准，也不代替原生窗口外观观察。
//  9. 点击 Notes 列并输入文字，预期整格出现 2 DIP 主题色直角边框，四边没有
//     外部留白；文字保留自己的内边距。点击其他位置后边框消失，不能出现双框。
//     长说明撑高整行时编辑器仍填满高度、文字垂直居中；编辑不改变行选择。
//     按 F6 在不转移输入焦点的情况下执行 Snapshot，断言编辑器边界和文本。
//     F6 还离屏采样当前编辑器的四角，检查焦点边框颜色和直角；可见桌面的
//     四边、裁剪和 HiDPI 效果仍需实际观察，不能用该离屏检查代替桌面验收。
//     排序、滚动到远处再返回、Rebuild/Refresh、Toggle dark 后，编辑值仍跟随
//     原 RowID，不能串到其他记录。Notes 内右键/Shift+F10 使用文本编辑菜单，
//     不应弹出行菜单。grid/thick 下编辑器与网格线相邻，线条不得遮挡编辑器。
//
// 平台差异：Primary、右键触发时机按现有事件规则转换。默认 Tab 仅遍历已挂载
// 控件，不保证跨虚拟记录导航；首版没有列重排或表格编辑事务。副作用仅测试窗口、
// 内存数据和终端日志；不读写
// 文件或网络；文本编辑菜单的复制/剪切/粘贴由用户主动触发并会读写剪贴板。
// 复位：重启；退出：关闭窗口。断言失败 panic 返回非零。
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
type paintedCell struct{ gui.WidgetBase }

func (*paintedCell) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: c.Max.Width, Height: 24}))
}
func (w *paintedCell) Paint(p gui.Painter) {
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
