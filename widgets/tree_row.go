package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

const (
	treeRowMinHeight float32 = 28
	treeRowPadding   float32 = 4
	treeExpanderSize float32 = 20
)

type treeListDelegate struct {
	view     *TreeView
	delegate TreeItemDelegate
}

func (d *treeListDelegate) Setup() gui.Widget {
	r := &treeRow{view: d.view, delegate: d.delegate}
	r.SetMinSize(geometry.Size{Height: treeRowMinHeight})
	r.SetLayoutManager(layout.NewFillLayout())
	if d.delegate != nil {
		r.content = d.delegate.Setup()
	}
	if r.content != nil {
		r.WidgetBase.AddChild(r, r.content)
	}
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
		id := r.bound.ID
		e, ok := ctx.Event().(events.PointerEvent)
		if !ok {
			return
		}
		primary, shift := treeModifiers(e.Modifiers)
		r.view.choose(id, primary, shift)
		if r.bound.ID != id || r.view.Destroyed() {
			return
		}
		if count == 2 {
			if r.view.model.Expandable(id) {
				r.view.SetExpanded(id, !r.view.Expanded(id))
			} else {
				r.view.activate.Emit(id, r.view.revision)
			}
		}
	})
	r.AddEventController(r.click)
	// Rows can leave a host without scrolling (window teardown or subtree
	// transfer). Release the last binding then too. Ordinary recycling already
	// clears bound before detaching, so this never calls Unbind twice.
	r.ConnectUnmount(func() { d.Unbind(0, r) })
	return r
}
func (d *treeListDelegate) Bind(index int, widget gui.Widget) {
	r := widget.(*treeRow)
	v := d.view
	if v.Destroyed() || index < 0 || index >= len(v.rows) {
		return
	}
	flat := v.rows[index]
	row := v.rowState(flat)
	if r.bound.ID != "" && r.bound.ID != row.ID {
		d.Unbind(index, widget)
	}
	r.bound, r.flat = row, flat
	v.realized[row.ID] = r
	r.RequestLayout()
	if r.delegate != nil && r.content != nil {
		r.delegate.Bind(row, r.content)
	}
}
func (d *treeListDelegate) Unbind(_ int, widget gui.Widget) {
	r := widget.(*treeRow)
	old := r.bound
	if old.ID == "" {
		return
	}
	r.bound = TreeRow{}
	if d.view.realized[old.ID] == r {
		delete(d.view.realized, old.ID)
	}
	r.motion.Reset()
	r.click.Reset()
	if r.delegate != nil && r.content != nil {
		r.delegate.Unbind(old, r.content)
	}
}

func (v *TreeView) rowState(flat flatTreeRow) TreeRow {
	return TreeRow{
		ID: flat.id, Depth: flat.depth, Expandable: v.model.Expandable(flat.id),
		Expanded: v.expanded[flat.id], Selected: v.selected[flat.id], Current: v.current == flat.id,
	}
}

type treeRow struct {
	gui.WidgetBase
	view           *TreeView
	delegate       TreeItemDelegate
	bound          TreeRow
	flat           flatTreeRow
	content        gui.Widget
	motion         *gui.MotionEventController
	click          *gui.ClickEventController
	hover, pressed bool
}

func (r *treeRow) Paint(p gui.Painter) {
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
	name := r.StyleName()
	if name == "" {
		name = "tree-item"
	}
	part := ""
	if r.view.selected[r.bound.ID] {
		part = "selected"
	}
	rect := geometry.Rect(0, 0, r.Rect().Width, r.Rect().Height)
	paintStyledBox(p, rect, name, part, state)
	if r.view.current == r.bound.ID && r.view.Focused() {
		paintStyledBorder(p, rect, gui.ResolveStyle(name, "current", state))
	}
}
func (r *treeRow) Snapshot() gui.WidgetInfo {
	info := r.WidgetBase.Snapshot()
	info.Role = RoleTreeItem
	info.Selected = r.view.selected[r.bound.ID]
	info.SetAttribute(HierarchyInfoKey, HierarchyInfo{
		NodeID: r.bound.ID, ParentID: r.flat.parent, Level: r.flat.depth + 1,
		PositionInSet: r.flat.position, SetSize: r.flat.siblings,
		Expandable: r.bound.Expandable, Expanded: r.view.expanded[r.bound.ID], Current: r.view.current == r.bound.ID,
	})
	info.Actions = append(info.Actions, gui.ActionClick)
	return info
}
