// TableView 多列记录、虚拟化、声明协调与现代外观验收，不使用 DevServer。
//
// 环境：Linux/X11、Windows 或 macOS 可交互桌面；记录 OS、Linux WM、实际
// 后端与缩放。仓库根目录运行 go run ./tests/window/widgets/table；默认 UI，
// -gui 使用命令式 GUI，-fallback-style 显式组合兜底样式。
// -appearance=plain/zebra/rows/grid/thick 指定初始外观，默认 plain；Appearance
// 按钮循环切换以上模式。thick 为粗线与自定义 Widget 内容避让的专项检查。
// GOUI_PLAT_PAINTER=software/opengl，Windows 另可 direct2d；GOUI_PLAT_SCALE=2
// 请求 2x。初始：Native 880×620 DIP、Modern 亮色、10000 条内存记录、多选、
// 四列固定宽度；名称和大小可排序，说明自动换行，部分操作单元格含 Inspect 按钮。
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
//
// 平台差异：Primary、右键触发时机按现有事件规则转换。首版没有全局 Tab 焦点
// 导航、列重排或表格编辑事务。副作用仅测试窗口、内存数据和终端日志；不读写
// 文件、剪贴板或网络。复位：重启；退出：关闭窗口。断言失败 panic 返回非零。
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
type model struct{ *gui.SliceListModel[record] }

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
	return &model{gui.NewSliceListModel(rows)}
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
func assertTable(table *widgets.TableView) {
	info := table.Snapshot()
	columns := table.Columns()
	if info.Role != widgets.RoleTable || info.Table == nil || info.Table.ColumnCount != 4 || info.Table.RowCount != table.Model().ItemsCount() || len(info.Children) < 4 || len(info.Children) > 90 {
		panic("table extent/virtualization")
	}
	seen := make(map[string]bool)
	for i, header := range info.Children[:4] {
		if header.Role != widgets.RoleColumnHeader || header.Table.ColumnID != columns[i].ID() || header.Table.ColumnIndex != i+1 {
			panic("header semantics")
		}
	}
	for _, row := range info.Children[4:] {
		if row.Role != widgets.RoleTableRow || row.Table == nil || row.Table.RowID == "" || seen[row.Table.RowID] || len(row.Children) != 4 {
			panic("row semantics")
		}
		seen[row.Table.RowID] = true
		if row.Table.RowIndex < 1 || row.Table.RowIndex > info.Table.RowCount || table.Model().RowID(row.Table.RowIndex-1) != row.Table.RowID || row.Table.Current != (row.Table.RowID == table.Current()) || row.Selected != slices.Contains(table.Selection(), row.Table.RowID) {
			panic("row identity/state")
		}
		for i, cell := range row.Children {
			if cell.Role != widgets.RoleTableCell || cell.Table.ColumnID != columns[i].ID() || cell.Table.RowID != row.Table.RowID || cell.Table.ColumnIndex != i+1 || cell.Bounds.Width != columns[i].Width() || cell.Bounds.X != info.Children[i].Bounds.X {
				panic("cell/header geometry")
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
		if painted, ok := widget.(*paintedCell); ok && painted.Visible() {
			r := painted.Snapshot().Bounds
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
					if got := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA); got != (color.RGBA{G: 150, B: 170, A: 255}) {
						panic(fmt.Sprintf("custom cell edge (%d,%d) covered: %v", x, y, got))
					}
				}
			}
		}
		for _, child := range widget.Children() {
			checkContent(child)
		}
	}
	checkContent(table)
	fmt.Printf("ASSERT PASS logical=%d realized=%d current=%s selection=%v scroll=(%g,%g)\n", info.Table.RowCount, len(seen), table.Current(), table.Selection(), info.ScrollX, info.ScrollY)
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
	widths := map[string]float32{"name": 240, "action": 100, "size": 140, "description": 420}
	return ui.Run("org.golang-gui.TableViewProbe", func(app ui.App) ui.RootView {
		column := func(id, title string, cell func(widgets.TableRow, record) ui.View) *wui.TableColumnView[record] {
			// Controlled width lives in application state, including Esc rollback.
			return wui.TableColumn(id, title, cell).Width(widths[id]).OnResize(func(w float32) {
				widths[id] = w
				fmt.Printf("resize=%s:%g\n", id, w)
			})
		}
		name := column("name", "Name", func(_ widgets.TableRow, row record) ui.View {
			return ui.Label(fmt.Sprintf("%s [v%d]", row.name, version)).Style("table-cell-text")
		}).MinWidth(80).MaxWidth(480).Sortable(true)
		action := column("action", "Action", func(row widgets.TableRow, value record) ui.View {
			if value.action {
				return ui.Button("Inspect").OnClick(func() { fmt.Printf("cell-action=%s\n", row.ID) })
			}
			if row.Index%7 == 0 {
				return paintedCellUI()
			}
			return nil
		}).MinWidth(80)
		size := column("size", "Size", func(_ widgets.TableRow, row record) ui.View {
			return ui.Label(fmt.Sprintf("%d KB", row.size)).Style("table-cell-text")
		}).Sortable(true)
		description := column("description", "Description", func(_ widgets.TableRow, row record) ui.View {
			return ui.Label(row.description).WrapMode(gui.WrapWordChar).Style("table-cell-text")
		})
		table := wui.TableView(m, name, action, size, description).ID("table").SelectionMode(widgets.SelectionMultiple).
			OnSelection(func(ids []string) { fmt.Printf("selection=%v\n", ids) }).OnCurrent(func(id string) { fmt.Printf("current=%s\n", id) }).
			OnActivate(func(id string) { fmt.Printf("activate=%s\n", id) }).
			OnSortRequest(func(id string, order widgets.SortOrder) {
				m.sort(id, order)
				app.FindWidget("table-window", "table").(*widgets.TableView).SetSort(id, order)
			}).
			ContextMenu(func(row, column string) []*ui.MenuItemView { return menuItems(m, row, column) }).
			OnContextMenuError(func(err error) { fmt.Printf("menu-error=%v\n", err) })
		return ui.Root().StyleSheet(sheet(dark, fallback, appearance)).Windows(ui.Window("table-window").Title("GOUI TableView UI").Size(880, 620).MinSize(400, 240).Content(
			ui.VBox(ui.HBox(
				ui.Button("Rebuild").OnClick(func() { version++; app.RequestUpdate() }),
				ui.Button("Toggle dark").OnClick(func() { dark = !dark; app.RequestUpdate() }),
				ui.Button("Reveal last").OnClick(func() { app.FindWidget("table-window", "table").(*widgets.TableView).Reveal("record-09999") }),
				ui.Button("Snapshot").OnClick(func() { assertTable(app.FindWidget("table-window", "table").(*widgets.TableView)) }),
				ui.Button("Appearance").OnClick(func() {
					appearance = (appearance + 1) % len(appearances)
					fmt.Printf("appearance=%s\n", appearances[appearance])
					app.RequestUpdate()
				}),
			).Spacing(8), ui.Label("Primary / Shift selection; sortable Name / Size; resize edges / Esc; Shift+F10 menu"), table).CrossAlign(layout.CrossStretch).Padding(12).Spacing(8),
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
	if d.column == "action" {
		c := &actionCell{LinearBox: gui.NewLinearBox(layout.DirectionVertical), button: gui.NewButton(), painted: new(paintedCell)}
		c.SetCrossAlign(layout.CrossStretch)
		c.button.SetChild(gui.NewLabel("Inspect"))
		c.button.ConnectClicked(func() { fmt.Printf("cell-action=%s\n", c.id) })
		c.AddChild(c.button)
		c.AddChild(c.painted)
		return c
	}
	label := gui.NewLabel("")
	label.SetStyleName("table-cell-text")
	if d.column == "description" {
		label.SetWrapMode(gui.WrapWordChar)
	}
	return label
}
func (d *cellDelegate) Bind(row widgets.TableRow, widget gui.Widget) {
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
	widget.(*gui.Label).SetText(text)
}
func (*cellDelegate) Unbind(_ widgets.TableRow, widget gui.Widget) {
	if c, ok := widget.(*actionCell); ok {
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
	for i, id := range []string{"name", "action", "size", "description"} {
		c := widgets.NewTableColumn(id, []string{"Name", "Action", "Size", "Description"}[i])
		c.SetWidth([]float32{240, 100, 140, 420}[i])
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
	button("Snapshot", func() { assertTable(table) })
	button("Appearance", func() {
		appearance = (appearance + 1) % len(appearances)
		fmt.Printf("appearance=%s\n", appearances[appearance])
		app.SetStyleSheet(sheet(dark, fallback, appearance))
	})
	root.AddChild(toolbar)
	root.AddChild(gui.NewLabel("Primary / Shift selection; sortable Name / Size; resize edges / Esc; Shift+F10 menu"))
	root.AddChild(table)
	window.SetWidget(root)
	if err = window.Show(); err != nil {
		return err
	}
	app.Run()
	return nil
}
