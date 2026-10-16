package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

type record struct {
	ID, Name, Kind, Note string
	Size                 int
	number               int
	noteEdited           bool
}
type recordModel struct{ *gui.SliceListModel[record] }

func (m *recordModel) RowID(i int) string { return m.ItemAt(i).ID }
func newRecordModel() *recordModel {
	items := make([]record, 120)
	for i := range items {
		items[i] = record{ID: fmt.Sprintf("file-%03d", i), number: i + 1, Size: (i * 37) % 2048}
		items[i].translate()
	}
	return &recordModel{gui.NewSliceListModel(items)}
}
func (r *record) translate() {
	r.Name = fmt.Sprintf(resource.RecordName(), r.number)
	r.Kind = []string{resource.RecordDocument(), resource.RecordImage(), resource.RecordCode()}[(r.number-1)%3]
	if !r.noteEdited {
		r.Note = resource.RecordNote()
	}
}
func (m *recordModel) translate() {
	m.Modify(func(items []record) []record {
		for i := range items {
			items[i].translate()
		}
		return items
	})
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
	m.writeNote(id, note, true)
}
func (m *recordModel) resetNote(id string) {
	m.writeNote(id, resource.RecordNote(), false)
}
func (m *recordModel) writeNote(id, note string, edited bool) {
	for i := 0; i < m.ItemsCount(); i++ {
		r := m.ItemAt(i)
		if r.ID == id {
			if r.Note != note || r.noteEdited != edited {
				r.Note, r.noteEdited = note, edited
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
			d.report(fmt.Sprintf(resource.TreeSelected(), strings.Join(ids, ", ")))
		}).
		ContextMenu(func(id string) []*ui.MenuItemView {
			return []*ui.MenuItemView{
				ui.MenuItem(fmt.Sprintf(resource.ViewNode(), d.tree.Item(id)), func() { d.report(fmt.Sprintf(resource.NodeID(), id)) }),
				ui.MenuItem(resource.CopyNode(), func() {
					if err := d.app.Clipboard().SetText(d.tree.Item(id)); err != nil {
						d.result(resource.CopyNode(), err)
						return
					}
					d.report(resource.NodeCopied())
				}),
			}
		}).OnContextMenuError(func(e error) { d.result(resource.TreeMenu(), e) })
	table := wui.TableView(d.table,
		wui.TableColumn("name", resource.Name(), func(_ widgets.TableRow, r record) ui.View { return ui.HBox(ui.Label(r.Name)).Padding(8) }).Width(d.tableWidths[0]).OnResize(func(width float32) { d.tableWidths[0] = width }).MinWidth(100).Resizable(true).Sortable(true),
		wui.TableColumn("kind", resource.Kind(), func(_ widgets.TableRow, r record) ui.View { return ui.HBox(ui.Label(r.Kind)).Padding(8) }).Width(d.tableWidths[1]).OnResize(func(width float32) { d.tableWidths[1] = width }).Resizable(true).Sortable(true),
		wui.TableColumn("size", resource.SizeKB(), func(_ widgets.TableRow, r record) ui.View { return ui.HBox(ui.Label(fmt.Sprint(r.Size))).Padding(8) }).Width(d.tableWidths[2]).OnResize(func(width float32) { d.tableWidths[2] = width }).Resizable(true).Sortable(true),
		wui.TableColumn("note", resource.Note(), func(_ widgets.TableRow, r record) ui.View {
			return ui.TextInput().Style("demo-cell-edit").Text(r.Note).OnText(func(value string) { d.table.setNote(r.ID, value) })
		}).Width(d.tableWidths[3]).OnResize(func(width float32) { d.tableWidths[3] = width }).Resizable(true),
	).ID("records-table").SelectionMode(widgets.SelectionMultiple).Selection(d.tableSelection).Sort(d.sortColumn, d.sortOrder).
		OnSelection(func(ids []string) {
			d.tableSelection = slices.Clone(ids)
			d.report(fmt.Sprintf(resource.TableSelected(), len(ids)))
		}).
		OnSortRequest(func(column string, order widgets.SortOrder) {
			d.sortColumn, d.sortOrder = column, order
			d.table.sort(column, order)
			d.app.RequestUpdate()
		}).
		ContextMenu(func(id, column string) []*ui.MenuItemView {
			return []*ui.MenuItemView{ui.MenuItem(resource.ViewRowID(), func() { d.report(fmt.Sprintf(resource.RowColumn(), id, column)) }), ui.MenuItem(resource.ResetNote(), func() { d.table.resetNote(id) })}
		}).
		OnContextMenuError(func(e error) { d.result(resource.TableMenu(), e) })
	left := ui.VBox(ui.Label(resource.TreeActions()), ui.ScrollView(tree).MainWeight(1)).CrossAlign(layout.CrossStretch).Spacing(8).MinWidth(160)
	right := ui.VBox(ui.Label(resource.TableActions()), table.MainWeight(1)).CrossAlign(layout.CrossStretch).Spacing(8).MinWidth(250)
	return fullPage("data", resource.Data(), resource.DataHint(),
		wui.VSplit(wui.HSplit(left, right).StartSize(d.dataStart).OnResize(func(start, _ float32) { d.dataStart = start }),
			ui.VBox(ui.Label(resource.ListRows()), ui.ScrollView(ui.ListView(d.list, func(i int, text string) ui.View {
				return ui.Button(text).OnClick(func() { d.report(fmt.Sprintf(resource.ListSelected(), i+1)) })
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
