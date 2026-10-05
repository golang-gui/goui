package ui

import (
	"github.com/golang-gui/goui/core/bits"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/internal/identity"
	"github.com/golang-gui/goui/layout"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	"slices"
)

// TableColumnView is a column declaration, not a Widget View. Its ID preserves
// the imperative column and delegate across rebuilds of its owning TableView.
type TableColumnView[T any] struct {
	id, title                 string
	cell                      func(widgets.TableRow, T) baseui.View
	width, minWidth, maxWidth float32
	resizable, sortable       bool
	fields                    bits.Bitmap[uint8]
	onResize                  func(float32)
}

const tableColumnWidth = 0

func TableColumn[T any](id, title string, cell func(widgets.TableRow, T) baseui.View) *TableColumnView[T] {
	return &TableColumnView[T]{id: id, title: title, cell: cell, minWidth: 40, resizable: true}
}

// Width is controlled when declared. Omission retains user resizing, including
// after a previously controlled declaration is removed.
func (c *TableColumnView[T]) Width(width float32) *TableColumnView[T] {
	c.width = width
	c.fields.Set(tableColumnWidth, true)
	return c
}
func (c *TableColumnView[T]) MinWidth(width float32) *TableColumnView[T] {
	c.minWidth = width
	return c
}
func (c *TableColumnView[T]) MaxWidth(width float32) *TableColumnView[T] {
	c.maxWidth = width
	return c
}
func (c *TableColumnView[T]) Resizable(enabled bool) *TableColumnView[T] {
	c.resizable = enabled
	return c
}
func (c *TableColumnView[T]) Sortable(enabled bool) *TableColumnView[T] {
	c.sortable = enabled
	return c
}
func (c *TableColumnView[T]) OnResize(fn func(float32)) *TableColumnView[T] {
	c.onResize = fn
	return c
}

// TableViewView binds explicit columns and a typed model. Omitted Selection,
// Current and Sort retain user state while a stable model pointer is reused.
type TableViewView[T any] struct {
	baseui.ViewBase[TableViewView[T]]
	model                 widgets.TableData[T]
	columns               []*TableColumnView[T]
	mode                  widgets.SelectionMode
	selection             []string
	current, sortColumn   string
	sortOrder             widgets.SortOrder
	fields                bits.Bitmap[uint8]
	onSelection           func([]string)
	onCurrent, onActivate func(string)
	onSortRequest         func(string, widgets.SortOrder)
	contextMenu           func(string, string) []*baseui.MenuItemView
	onContextMenuError    func(error)
}

const (
	tableSelection = iota
	tableCurrent
	tableSort
)

// TableView declares a self-scrolling table. Reuse a non-nil model pointer to
// retain state. Value models reinstall on every update; typed nil is unsupported.
func TableView[T any](model widgets.TableData[T], columns ...*TableColumnView[T]) *TableViewView[T] {
	v := &TableViewView[T]{model: model, columns: slices.Clone(columns)}
	v.Self = v
	return v
}
func (v *TableViewView[T]) Model(model widgets.TableData[T]) *TableViewView[T] {
	v.model = model
	return v
}
func (v *TableViewView[T]) Columns(columns ...*TableColumnView[T]) *TableViewView[T] {
	v.columns = slices.Clone(columns)
	return v
}
func (v *TableViewView[T]) SelectionMode(mode widgets.SelectionMode) *TableViewView[T] {
	v.mode = mode
	return v
}
func (v *TableViewView[T]) Selection(ids []string) *TableViewView[T] {
	v.selection = slices.Clone(ids)
	v.fields.Set(tableSelection, true)
	return v
}
func (v *TableViewView[T]) Current(id string) *TableViewView[T] {
	v.current = id
	v.fields.Set(tableCurrent, true)
	return v
}
func (v *TableViewView[T]) Sort(id string, order widgets.SortOrder) *TableViewView[T] {
	v.sortColumn = id
	v.sortOrder = order
	v.fields.Set(tableSort, true)
	return v
}
func (v *TableViewView[T]) OnSelection(fn func([]string)) *TableViewView[T] {
	v.onSelection = fn
	return v
}
func (v *TableViewView[T]) OnCurrent(fn func(string)) *TableViewView[T]  { v.onCurrent = fn; return v }
func (v *TableViewView[T]) OnActivate(fn func(string)) *TableViewView[T] { v.onActivate = fn; return v }
func (v *TableViewView[T]) OnSortRequest(fn func(string, widgets.SortOrder)) *TableViewView[T] {
	v.onSortRequest = fn
	return v
}
func (v *TableViewView[T]) ContextMenu(fn func(string, string) []*baseui.MenuItemView) *TableViewView[T] {
	v.contextMenu = fn
	return v
}
func (v *TableViewView[T]) OnContextMenuError(fn func(error)) *TableViewView[T] {
	v.onContextMenuError = fn
	return v
}
func (v *TableViewView[T]) Build() baseui.View { return v }

type tableState[T any] struct {
	ctx                   baseui.BuildContext
	model                 widgets.TableData[T]
	table                 *widgets.TableView
	columns               map[string]*tableColumnState[T]
	handles               signal.Handles
	onSelection           func([]string)
	onCurrent, onActivate func(string)
	onSortRequest         func(string, widgets.SortOrder)
	contextMenu           func(string, string) []*baseui.MenuItemView
	onError               func(error)
}
type tableColumnState[T any] struct {
	owner    *tableState[T]
	column   *widgets.TableColumn
	cell     func(widgets.TableRow, T) baseui.View
	onResize func(float32)
	handle   signal.Handle
}

func (v *TableViewView[T]) Mount(ctx baseui.BuildContext) gui.Widget {
	table := widgets.NewTableView()
	s := &tableState[T]{ctx: ctx, table: table, columns: make(map[string]*tableColumnState[T])}
	s.handles = signal.Handles{
		table.ConnectSelection(func(ids []string) {
			if s.onSelection != nil {
				s.onSelection(ids)
			}
		}),
		table.ConnectCurrent(func(id string) {
			if s.onCurrent != nil {
				s.onCurrent(id)
			}
		}),
		table.ConnectActivate(func(id string) {
			if s.onActivate != nil {
				s.onActivate(id)
			}
		}),
		table.ConnectSortRequest(func(id string, order widgets.SortOrder) {
			if s.onSortRequest != nil {
				s.onSortRequest(id, order)
			}
		}),
		table.ConnectContextMenu(func(row, column string, result *gui.MenuModel) {
			if s.contextMenu != nil {
				*result = baseui.Menu(s.contextMenu(row, column)...)
			}
		}),
		table.ConnectContextMenuError(func(err error) {
			if s.onError != nil {
				s.onError(err)
			}
		}),
	}
	ctx.SetState(s)
	return table
}
func (v *TableViewView[T]) Update(ctx baseui.BuildContext, widget gui.Widget) {
	s, table := ctx.State().(*tableState[T]), widget.(*widgets.TableView)
	seen := make(map[string]bool)
	for _, c := range v.columns {
		if c == nil || c.id == "" || seen[c.id] {
			panic("widgets/ui: invalid or duplicate table column")
		}
		seen[c.id] = true
	}
	s.model = v.model
	s.onSelection = v.onSelection
	s.onCurrent = v.onCurrent
	s.onActivate = v.onActivate
	s.onSortRequest = v.onSortRequest
	s.contextMenu = v.contextMenu
	s.onError = v.onContextMenuError
	s.handles.Block()
	defer s.handles.Unblock()
	var next []*widgets.TableColumn
	var blocked signal.Handles
	defer func() { blocked.Unblock() }()
	for _, declaration := range v.columns {
		c := s.columns[declaration.id]
		if c == nil {
			c = &tableColumnState[T]{owner: s, column: widgets.NewTableColumn(declaration.id, declaration.title)}
			c.column.SetDelegate(c)
			c.handle = c.column.ConnectResize(func(width float32) {
				if c.owner.ctx != nil && c.onResize != nil {
					c.onResize(width)
				}
			})
			s.columns[declaration.id] = c
		}
		c.cell, c.onResize = declaration.cell, declaration.onResize
		c.handle.Block()
		blocked = append(blocked, c.handle)
		c.column.SetTitle(declaration.title)
		c.column.SetMinWidth(declaration.minWidth)
		c.column.SetMaxWidth(declaration.maxWidth)
		c.column.SetResizable(declaration.resizable)
		c.column.SetSortable(declaration.sortable)
		if declaration.fields.Check(tableColumnWidth) {
			c.column.SetWidth(declaration.width)
		}
		next = append(next, c.column)
	}
	table.SetColumns(next...)
	// SetColumns unbinds removed cells before their borrowed contexts expire.
	for id, c := range s.columns {
		if !seen[id] {
			c.handle.Disconnect()
			c.cell, c.onResize = nil, nil
			delete(s.columns, id)
		}
	}
	if !identity.SamePointer(table.Model(), v.model) {
		table.SetModel(v.model)
	}
	table.SetSelectionMode(v.mode)
	if v.fields.Check(tableSelection) {
		table.SetSelection(v.selection)
	}
	if v.fields.Check(tableCurrent) {
		table.SetCurrent(v.current)
	}
	if v.fields.Check(tableSort) {
		table.SetSort(v.sortColumn, v.sortOrder)
	}
	table.Refresh()
}
func (*TableViewView[T]) Unmount(ctx baseui.BuildContext, _ gui.Widget) {
	s := ctx.State().(*tableState[T])
	s.handles.Disconnect()
	for _, c := range s.columns {
		c.handle.Disconnect()
		c.cell, c.onResize = nil, nil
	}
	s.ctx, s.model = nil, nil
	s.onSelection, s.onCurrent, s.onActivate = nil, nil, nil
	s.onSortRequest, s.contextMenu, s.onError = nil, nil, nil
}
func (c *tableColumnState[T]) Setup() gui.Widget { return gui.NewLinearBox(layout.DirectionVertical) }
func (c *tableColumnState[T]) Bind(row widgets.TableRow, widget gui.Widget) {
	s := c.owner
	ctx := s.ctx
	if ctx == nil {
		return
	}
	var view baseui.View
	if c.cell != nil && s.model != nil {
		view = c.cell(row, s.model.ItemAt(row.Index))
	}
	ctx.UpdateChildren(widget.(*gui.LinearBox), []baseui.View{view})
}
func (c *tableColumnState[T]) Unbind(_ widgets.TableRow, widget gui.Widget) {
	if ctx := c.owner.ctx; ctx != nil {
		ctx.UpdateChildren(widget.(*gui.LinearBox), nil)
	}
}
