package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
	"slices"
)

type tableHeader struct {
	gui.WidgetBase
	view           *TableView
	column         *TableColumn
	label          *gui.Label
	handle         *tableResizeHandle
	hover, pressed bool
}

func newTableHeader(v *TableView, c *TableColumn) *tableHeader {
	h := &tableHeader{view: v, column: c, label: gui.NewLabel(c.title)}
	h.label.SetStyleName("table-header-text")
	h.handle = newTableResizeHandle(h)
	h.WidgetBase.AddChild(h, h.label)
	h.WidgetBase.AddChild(h, h.handle)
	h.SetLayoutManager(&tableHeaderCellLayout{header: h})
	motion := gui.NewMotionEventController()
	motion.ConnectContainsHover(func(hover bool) { h.hover = hover; h.RequestPaint() })
	h.AddEventController(motion)
	click := gui.NewClickEventController()
	click.ConnectPressed(func(_ gui.EventContext, pressed bool) { h.pressed = pressed; h.RequestPaint() })
	click.ConnectClicked(func(ctx gui.EventContext) { ctx.StopPropagation(); v.requestSort(c) })
	h.AddEventController(click)
	keys := gui.NewKeyEventController()
	keys.SetPhase(gui.PhaseTarget)
	keys.ConnectKeyDown(func(ctx gui.EventContext, e events.KeyEvent) {
		if c.sortable && e.Modifiers == 0 && (e.Key == events.KeyEnter || e.Key == events.KeySpace) {
			ctx.StopPropagation()
			e.PreventDefault()
			if !e.Repeat {
				v.requestSort(c)
			}
		}
	})
	h.AddEventController(keys)
	h.update()
	return h
}
func (h *tableHeader) update() {
	h.label.SetText(h.column.title)
	h.SetFocusable(h.column.sortable)
	h.handle.SetVisible(h.column.resizable)
	if !h.column.resizable {
		h.handle.cancel()
	}
	h.RequestLayout()
	h.RequestPaint()
}
func (h *tableHeader) Paint(p gui.Painter) {
	state := style.Normal
	if h.hover && h.column.sortable {
		state = style.Hovered
	}
	if h.pressed && h.column.sortable {
		state = style.Pressed
	}
	r := geometry.Rect(0, 0, h.Rect().Width, h.Rect().Height)
	paintStyledBox(p, r, "table-header", "", state)
	line := min(r.Height, tableHeaderLineWidth())
	w := h.view.columnLineWidth(h.column)
	tablePaintLine(p, geometry.Rect(r.Width-w, 0, w, max(0, r.Height-line)), tableGridStyle("vertical"))
	if color, ok := gui.ResolveStyle("table-header", "separator", state).ForegroundColor(); ok && color != nil {
		p.FillRect(geometry.Rect(0, r.Height-line, r.Width, line), graphics.ColorOf(color))
	}
	if h.view.sortColumn == h.column.id && h.view.sortOrder != SortNone {
		if color, ok := gui.ResolveStyle("table-header", "sort", state).ForegroundColor(); ok && color != nil {
			x, y := max(0, r.Width-w-20), max(0, r.Height-line)/2
			dy := float32(3)
			if h.view.sortOrder == SortDescending {
				dy = -dy
			}
			ink := graphics.ColorOf(color)
			p.DrawLine(geometry.Point{X: x - 3, Y: y + dy/2}, geometry.Point{X: x, Y: y - dy/2}, 1, ink)
			p.DrawLine(geometry.Point{X: x, Y: y - dy/2}, geometry.Point{X: x + 3, Y: y + dy/2}, 1, ink)
		}
	}
}
func (h *tableHeader) Snapshot() gui.WidgetInfo {
	info := h.WidgetBase.Snapshot()
	info.Role, info.Text = RoleColumnHeader, h.column.title
	data := TableInfo{ColumnID: h.column.id, ColumnIndex: slices.Index(h.view.columns, h.column) + 1}
	if h.view.sortColumn == h.column.id {
		if h.view.sortOrder == SortAscending {
			data.Sort = "ascending"
		} else if h.view.sortOrder == SortDescending {
			data.Sort = "descending"
		}
	}
	info.SetAttribute(TableInfoKey, data)
	info.Children = nil
	if h.column.resizable {
		info.Children = append(info.Children, h.handle.Snapshot())
	}
	if h.column.sortable {
		info.Actions = append(info.Actions, gui.ActionClick)
	}
	return info
}

type tableHeaderCellLayout struct{ header *tableHeader }

func (l *tableHeaderCellLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	reserve := 2*tableHeaderPaddingX + l.header.view.columnLineWidth(l.header.column)
	if l.header.column.sortable {
		reserve += 16
	}
	m := children[0].Measure(layout.Loose(geometry.Size{Width: max(0, c.Max.Width-reserve), Height: layout.Inf}))
	return layout.Measured(c.Clamp(geometry.Size{Width: l.header.column.width, Height: max(tableMinHeight, m.Height+2*tableHeaderPaddingY) + tableHeaderLineWidth()}))
}
func (l *tableHeaderCellLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	reserve := 2*tableHeaderPaddingX + l.header.view.columnLineWidth(l.header.column)
	if l.header.column.sortable {
		reserve += 16
	}
	height := max(0, rect.Height-tableHeaderLineWidth())
	available := max(0, rect.Width-l.header.view.columnLineWidth(l.header.column))
	m := children[0].Measure(layout.Loose(geometry.Size{Width: max(0, rect.Width-reserve), Height: max(0, height-2*tableHeaderPaddingY)}))
	children[0].Arrange(geometry.Rect(min(tableHeaderPaddingX, available), max(0, (height-m.Height)/2), m.Width, m.Height))
	l.header.handle.Arrange(geometry.Rect(max(0, rect.Width-8), 0, min(8, rect.Width), rect.Height))
}

type tableResizeHandle struct {
	gui.WidgetBase
	header         *tableHeader
	drag           *gui.DragEventController
	dragging       bool
	before, startX float32
}

func newTableResizeHandle(header *tableHeader) *tableResizeHandle {
	h := &tableResizeHandle{header: header}
	h.SetFocusable(true)
	h.SetCursor(gui.CursorResizeHorizontal)
	h.drag = gui.NewDragEventController()
	h.drag.ConnectBegin(func(point geometry.Point, _ events.Modifiers) {
		if header.view.Destroyed() || header.column.owner != header.view || !header.column.resizable {
			h.drag.Reset()
			return
		}
		h.dragging, h.before, h.startX = true, header.column.width, tablePoint(h, point, header.view).X
	})
	h.drag.ConnectUpdate(func(point geometry.Point, _ events.Modifiers) { h.move(point) })
	h.drag.ConnectEnd(func(point geometry.Point, _ events.Modifiers) { h.move(point); h.dragging = false })
	h.drag.ConnectCancel(h.cancel)
	h.AddEventController(h.drag)
	keys := gui.NewKeyEventController()
	keys.SetPhase(gui.PhaseTarget)
	keys.ConnectKeyDown(func(ctx gui.EventContext, e events.KeyEvent) {
		if e.Key == events.KeyEscape && e.Modifiers == 0 && h.dragging {
			ctx.StopPropagation()
			e.PreventDefault()
			h.cancel()
		}
	})
	h.AddEventController(keys)
	h.ConnectUnmount(h.cancel)
	return h
}
func (h *tableResizeHandle) move(point geometry.Point) {
	if !h.dragging {
		return
	}
	v, c := h.header.view, h.header.column
	if v.Destroyed() || c.owner != v || !c.resizable {
		h.cancel()
		return
	}
	width := c.clamp(h.before + tablePoint(h, point, v).X - h.startX)
	if width == c.width {
		return
	}
	c.SetWidth(width)
	if !v.Destroyed() && c.owner == v && c.width == width {
		c.emitResize()
	}
}
func (h *tableResizeHandle) cancel() {
	if !h.dragging {
		return
	}
	h.dragging = false
	h.drag.Reset()
	v, c := h.header.view, h.header.column
	if v.Destroyed() || c.owner != v {
		return
	}
	width := c.clamp(h.before)
	if c.width == width {
		return
	}
	c.SetWidth(width)
	if !v.Destroyed() && c.owner == v && c.width == width {
		c.emitResize()
	}
}
func (h *tableResizeHandle) Snapshot() gui.WidgetInfo {
	info := h.WidgetBase.Snapshot()
	info.Role = RoleSeparator
	c := h.header.column
	maximum := c.maxWidth
	if maximum == 0 {
		maximum = layout.Inf
	}
	info.Range = &gui.RangeInfo{Value: c.width, Min: c.minWidth, Max: max(c.minWidth, maximum), Direction: layout.DirectionHorizontal}
	info.Actions = []gui.Action{gui.ActionFocus}
	return info
}
func tablePoint(widget gui.Widget, point geometry.Point, table *TableView) geometry.Point {
	for w := widget; w != nil && w != table; w = w.Parent() {
		point = point.Add(w.Rect().Pos)
	}
	return point
}
