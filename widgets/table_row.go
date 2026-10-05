package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
	"slices"
)

type tableListDelegate struct{ view *TableView }

func (d *tableListDelegate) Setup() gui.Widget {
	r := &tableRow{view: d.view}
	r.SetLayoutManager(&tableRowLayout{row: r})
	r.motion = gui.NewMotionEventController()
	r.motion.ConnectContainsHover(func(hover bool) { r.hover = hover; r.RequestPaint() })
	r.AddEventController(r.motion)
	r.click = gui.NewClickEventController()
	r.click.ConnectPressed(func(_ gui.EventContext, pressed bool) { r.pressed = pressed; r.RequestPaint() })
	r.click.ConnectClick(func(ctx gui.EventContext, count int) {
		if r.bound.ID == "" || r.view.Destroyed() {
			return
		}
		ctx.StopPropagation()
		id, version := r.bound.ID, r.view.revision
		if e, ok := ctx.Event().(events.PointerEvent); ok {
			primary, shift := treeModifiers(e.Modifiers)
			r.view.choose(id, primary, shift)
			if count == 2 && r.view.valid(version) && r.bound.ID == id {
				r.view.activate.Emit(id, version)
			}
		}
	})
	r.AddEventController(r.click)
	r.ConnectUnmount(func() { d.Unbind(0, r) })
	return r
}
func (d *tableListDelegate) Bind(index int, widget gui.Widget) {
	v, r := d.view, widget.(*tableRow)
	if v.Destroyed() || index < 0 || index >= len(v.rowIDs) {
		return
	}
	version := v.revision
	row := v.rowState(index)
	if r.bound.ID != "" && r.bound.ID != row.ID {
		d.Unbind(index, r)
	}
	if !v.valid(version) {
		return
	}
	r.syncCells()
	if !v.valid(version) {
		return
	}
	r.bound = row
	v.realized[row.ID] = r
	for _, cell := range slices.Clone(r.cells) {
		cell.bound = row
		if cell.delegate != nil && cell.content != nil {
			cell.delegate.Bind(row, cell.content)
		}
		if !v.valid(version) || r.bound.ID != row.ID {
			return
		}
		cell.RequestLayout()
	}
	r.RequestLayout()
}
func (d *tableListDelegate) Unbind(_ int, widget gui.Widget) {
	r := widget.(*tableRow)
	old := r.bound
	if old.ID == "" {
		return
	}
	r.bound = TableRow{}
	if d.view.realized[old.ID] == r {
		delete(d.view.realized, old.ID)
	}
	// Focus must not follow a shell into a different business record.
	tableRelinquishFocus(r, d.view)
	r.motion.Reset()
	r.click.Reset()
	for _, cell := range slices.Clone(r.cells) {
		cell.unbind()
	}
}

type tableRow struct {
	gui.WidgetBase
	view           *TableView
	bound          TableRow
	cells          []*tableCell
	motion         *gui.MotionEventController
	click          *gui.ClickEventController
	hover, pressed bool
}

func (r *tableRow) syncCells() {
	v := r.view
	version := v.revision
	for _, cell := range slices.Clone(r.cells) {
		if !slices.Contains(v.columns, cell.column) {
			r.cells = slices.DeleteFunc(r.cells, func(previous *tableCell) bool { return previous == cell })
			cell.unbind()
			r.RemoveChild(cell)
			if !v.valid(version) {
				return
			}
		}
	}
	next := make([]*tableCell, 0, len(v.columns))
	for _, c := range v.columns {
		var cell *tableCell
		for _, previous := range r.cells {
			if previous.column == c {
				cell = previous
				break
			}
		}
		if cell == nil {
			cell = &tableCell{row: r, column: c, delegate: c.delegate}
			cell.SetLayoutManager(&tableCellLayout{cell: cell})
			if cell.delegate != nil {
				cell.content = cell.delegate.Setup()
			}
			if !v.valid(version) {
				return
			}
			if cell.content != nil {
				dead, _ := cell.content.(interface{ Destroyed() bool })
				if dead != nil && dead.Destroyed() || cell.content.Parent() != nil || cell.content.Root() != nil {
					panic("widgets: table cell Setup must return a live detached widget")
				}
				cell.WidgetBase.AddChild(cell, cell.content)
			}
			r.cells = append(r.cells, cell)
			r.WidgetBase.AddChild(r, cell)
			if !v.valid(version) {
				return
			}
		}
		r.MoveChildBefore(cell, nil)
		next = append(next, cell)
	}
	r.cells = next
	r.RequestLayout()
}
func (r *tableRow) Paint(p gui.Painter) {
	if r.bound.ID == "" {
		return
	}
	state := style.Normal
	if r.hover {
		state = style.Hovered
	}
	if r.pressed {
		state = style.Pressed
	}
	part := ""
	if r.view.selected[r.bound.ID] {
		part = "selected"
	} else if state == style.Normal && r.bound.Index%2 == 1 {
		part = "alternate"
	}
	rect := geometry.Rect(0, 0, r.Rect().Width, r.Rect().Height)
	paintStyledBox(p, rect, "table-row", part, state)
	r.paintGrid(p, rect)
	if r.view.current == r.bound.ID && r.view.Focused() {
		paintStyledBorder(p, rect, gui.ResolveStyle("table-row", "current", state))
	}
}
func (r *tableRow) Snapshot() gui.WidgetInfo {
	info := r.WidgetBase.Snapshot()
	info.Role = RoleTableRow
	info.Selected = r.view.selected[r.bound.ID]
	info.Table = &gui.TableInfo{RowID: r.bound.ID, RowIndex: r.bound.Index + 1, Current: r.view.current == r.bound.ID}
	info.Actions = append(info.Actions, gui.ActionClick)
	return info
}

type tableCell struct {
	gui.WidgetBase
	row      *tableRow
	column   *TableColumn
	delegate TableCellDelegate
	content  gui.Widget
	bound    TableRow
}

func (c *tableCell) unbind() {
	old := c.bound
	if old.ID == "" {
		return
	}
	c.bound = TableRow{}
	tableRelinquishFocus(c, c.row.view)
	if c.delegate != nil && c.content != nil {
		c.delegate.Unbind(old, c.content)
	}
}

func tableRelinquishFocus(widget gui.Widget, view *TableView) {
	if view.Destroyed() {
		return
	}
	if host, ok := widget.Root().(gui.EventTarget); ok {
		// Query actual focus; ContainsFocus is a crossing notification and may
		// not yet have been delivered for a programmatic focus change.
		for focused := host.FocusedWidget(); focused != nil; focused = focused.Parent() {
			if focused == widget {
				host.SetFocusedWidget(view)
				return
			}
		}
	}
}
func (c *tableCell) Snapshot() gui.WidgetInfo {
	info := c.WidgetBase.Snapshot()
	info.Role = RoleTableCell
	index := slices.Index(c.row.view.columns, c.column)
	info.Table = &gui.TableInfo{RowID: c.bound.ID, RowIndex: c.bound.Index + 1, ColumnID: c.column.id, ColumnIndex: index + 1, Current: c.row.view.current == c.bound.ID}
	info.Selected = c.row.view.selected[c.bound.ID]
	return info
}
