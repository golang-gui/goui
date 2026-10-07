package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

type record struct {
	ID, Name, Kind, Note string
	Size                 int
}
type recordModel struct{ *gui.SliceListModel[record] }

func (m *recordModel) RowID(i int) string { return m.ItemAt(i).ID }
func newRecordModel() *recordModel {
	items := make([]record, 120)
	for i := range items {
		items[i] = record{ID: fmt.Sprintf("file-%03d", i), Name: fmt.Sprintf("文档 %03d", i+1), Kind: []string{"文档", "图片", "代码"}[i%3], Size: (i * 37) % 2048, Note: "可编辑备注"}
	}
	return &recordModel{gui.NewSliceListModel(items)}
}
func (m *recordModel) sort(column string, order widgets.SortOrder) {
	m.Modify(func(items []record) []record {
		slices.SortStableFunc(items, func(a, b record) int {
			var n int
			switch column {
			case "size":
				n = a.Size - b.Size
			case "kind":
				n = strings.Compare(a.Kind, b.Kind)
			default:
				n = strings.Compare(a.Name, b.Name)
			}
			if order == widgets.SortNone {
				return strings.Compare(a.ID, b.ID)
			}
			if order == widgets.SortDescending {
				return -n
			}
			return n
		})
		return items
	})
}
func (m *recordModel) setNote(id, note string) {
	for i := 0; i < m.ItemsCount(); i++ {
		r := m.ItemAt(i)
		if r.ID == id {
			if r.Note != note {
				r.Note = note
				m.Set(i, r)
			}
			return
		}
	}
}
func (d *demo) dataPage() ui.View {
	tree := wui.TreeView(d.tree, func(row widgets.TreeRow, value string) ui.View {
		return ui.HBox(ui.Icon(d.icons[1]).Size(16).Style("tree-item-icon"), ui.Label(value).Style("tree-item-text")).Spacing(8)
	}).ID("project-tree").Expanded(d.expanded).Selection(d.treeSelection).SelectionMode(widgets.SelectionMultiple).
		OnExpanded(func(id string, expanded bool) {
			d.expanded = setMember(d.expanded, id, expanded)
			d.app.RequestUpdate()
		}).
		OnSelection(func(ids []string) {
			d.treeSelection = slices.Clone(ids)
			d.report("树选中：" + strings.Join(ids, ", "))
		}).
		ContextMenu(func(id string) []*ui.MenuItemView {
			return []*ui.MenuItemView{ui.MenuItem("查看 "+d.tree.Item(id), func() { d.report("节点 ID：" + id) }), ui.MenuItem("复制节点名", func() { d.app.Clipboard().SetText(d.tree.Item(id)); d.report("已请求复制节点名") })}
		}).OnContextMenuError(func(e error) { d.result("树菜单", e) })
	table := wui.TableView(d.table,
		wui.TableColumn("name", "名称", func(_ widgets.TableRow, r record) ui.View { return ui.HBox(ui.Label(r.Name)).Padding(8) }).Width(d.tableWidths[0]).OnResize(func(width float32) { d.tableWidths[0] = width }).MinWidth(100).Resizable(true).Sortable(true),
		wui.TableColumn("kind", "类型", func(_ widgets.TableRow, r record) ui.View { return ui.HBox(ui.Label(r.Kind)).Padding(8) }).Width(d.tableWidths[1]).OnResize(func(width float32) { d.tableWidths[1] = width }).Resizable(true).Sortable(true),
		wui.TableColumn("size", "大小 / KB", func(_ widgets.TableRow, r record) ui.View { return ui.HBox(ui.Label(fmt.Sprint(r.Size))).Padding(8) }).Width(d.tableWidths[2]).OnResize(func(width float32) { d.tableWidths[2] = width }).Resizable(true).Sortable(true),
		wui.TableColumn("note", "备注（直接编辑）", func(_ widgets.TableRow, r record) ui.View {
			return ui.TextInput().Style("demo-cell-edit").Text(r.Note).OnText(func(value string) { d.table.setNote(r.ID, value) })
		}).Width(d.tableWidths[3]).OnResize(func(width float32) { d.tableWidths[3] = width }).Resizable(true),
	).ID("records-table").SelectionMode(widgets.SelectionMultiple).Selection(d.tableSelection).Sort(d.sortColumn, d.sortOrder).
		OnSelection(func(ids []string) {
			d.tableSelection = slices.Clone(ids)
			d.report(fmt.Sprintf("表格已选 %d 行", len(ids)))
		}).
		OnSortRequest(func(column string, order widgets.SortOrder) {
			d.sortColumn, d.sortOrder = column, order
			d.table.sort(column, order)
			d.app.RequestUpdate()
		}).
		ContextMenu(func(id, column string) []*ui.MenuItemView {
			return []*ui.MenuItemView{ui.MenuItem("查看行 ID", func() { d.report("行 " + id + " / 列 " + column) }), ui.MenuItem("重置备注", func() { d.table.setNote(id, "可编辑备注") })}
		}).
		OnContextMenuError(func(e error) { d.result("表格菜单", e) })
	left := ui.VBox(ui.Label("TreeView · 展开 / 多选 / 右键"), ui.ScrollView(tree).MainWeight(1)).CrossAlign(layout.CrossStretch).Spacing(8).MinWidth(160)
	right := ui.VBox(ui.Label("TableView · 排序 / 列宽 / 单元格编辑"), table.MainWeight(1)).CrossAlign(layout.CrossStretch).Spacing(8).MinWidth(250)
	return fullPage("数据浏览", "所有数据都在内存中。表格可以直接编辑备注；左下列表按需创建可见行，不创建 1000 个控件。",
		wui.VSplit(wui.HSplit(left, right).StartSize(d.dataStart).OnResize(func(start, _ float32) { d.dataStart = start }),
			ui.VBox(ui.Label("ListView · 1000 行"), ui.ScrollView(ui.ListView(d.list, func(i int, text string) ui.View {
				return ui.Button(text).OnClick(func() { d.report(fmt.Sprintf("虚拟列表第 %d 行", i+1)) })
			})).MainWeight(1)).CrossAlign(layout.CrossStretch).Spacing(8)).Ratio(d.dataRatio).
			OnResize(func(start, end float32) {
				if start+end > 0 {
					d.dataRatio = start / (start + end)
				}
			}).MainWeight(1),
	)
}
func setMember(ids []string, id string, present bool) []string {
	out := slices.Clone(ids)
	i := slices.Index(out, id)
	if present && i < 0 {
		return append(out, id)
	}
	if !present && i >= 0 {
		return slices.Delete(out, i, i+1)
	}
	return out
}
