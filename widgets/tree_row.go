package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
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
	r.SetLayoutManager(&treeRowLayout{row: r})
	r.expander = &treeExpander{row: r}
	r.expander.click = gui.NewClickEventController()
	r.expander.click.ConnectPressed(func(_ gui.EventContext, pressed bool) {
		r.expander.pressed = pressed
		r.expander.RequestPaint()
	})
	r.expander.motion = gui.NewMotionEventController()
	r.expander.motion.ConnectHover(func(hover bool) {
		r.expander.hover = hover
		r.expander.RequestPaint()
	})
	r.expander.click.ConnectClicked(func(ctx gui.EventContext) {
		ctx.StopPropagation()
		if r.bound.ID != "" {
			r.view.SetExpanded(r.bound.ID, !r.view.Expanded(r.bound.ID))
		}
	})
	r.WidgetBase.AddChild(r, r.expander)
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
	r.expander.setExpandable(row.Expandable)
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
	r.expander.click.Reset()
	r.expander.motion.Reset()
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
	expander       *treeExpander
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
	info.Hierarchy = &gui.HierarchyInfo{
		NodeID: r.bound.ID, ParentID: r.flat.parent, Level: r.flat.depth + 1,
		PositionInSet: r.flat.position, SetSize: r.flat.siblings,
		Expandable: r.bound.Expandable, Expanded: r.view.expanded[r.bound.ID], Current: r.view.current == r.bound.ID,
	}
	info.Actions = append(info.Actions, gui.ActionClick)
	if !r.bound.Expandable && len(info.Children) != 0 {
		info.Children = info.Children[1:]
	}
	return info
}

type treeExpander struct {
	gui.WidgetBase
	row            *treeRow
	click          *gui.ClickEventController
	motion         *gui.MotionEventController
	expandable     bool
	hover, pressed bool
}

func (e *treeExpander) setExpandable(value bool) {
	if value == e.expandable {
		return
	}
	e.expandable = value
	if value {
		e.AddEventController(e.click)
		e.AddEventController(e.motion)
	} else {
		e.RemoveEventController(e.click)
		e.RemoveEventController(e.motion)
	}
}
func (e *treeExpander) Paint(p gui.Painter) {
	if !e.expandable {
		return
	}
	name := e.StyleName()
	if name == "" {
		name = "tree-expander"
	}
	state := style.Normal
	if e.hover {
		state = style.Hovered
	}
	if e.pressed {
		state = style.Pressed
	}
	paintStyledBox(p, geometry.Rect(0, 0, e.Rect().Width, e.Rect().Height), name, "", state)
	s := gui.ResolveStyle(name, "", state)
	color, ok := s.ForegroundColor()
	if !ok || color == nil {
		return
	}
	center := geometry.Point{X: e.Rect().Width / 2, Y: e.Rect().Height / 2}
	a, b, c := geometry.Point{X: -2, Y: -4}, geometry.Point{X: 2}, geometry.Point{X: -2, Y: 4}
	if e.row.view.expanded[e.row.bound.ID] {
		a, b, c = geometry.Point{X: -4, Y: -2}, geometry.Point{Y: 2}, geometry.Point{X: 4, Y: -2}
	}
	p.DrawLine(center.Add(a), center.Add(b), 1.5, graphics.ColorOf(color))
	p.DrawLine(center.Add(b), center.Add(c), 1.5, graphics.ColorOf(color))
}
func (e *treeExpander) Snapshot() gui.WidgetInfo {
	info := e.WidgetBase.Snapshot()
	if e.expandable {
		info.Role, info.Name = gui.RoleButton, "展开"
		if e.row.view.expanded[e.row.bound.ID] {
			info.Name = "折叠"
		}
		info.Actions = append(info.Actions, gui.ActionClick)
	}
	return info
}

// Layout receives GUI's cached/style-aware Child adapters; it does not bypass
// Widget measurement. The disclosure slot remains reserved for leaves.
type treeRowLayout struct{ row *treeRow }

func (l *treeRowLayout) prefix() float32 {
	return treeRowPadding + float32(l.row.bound.Depth)*l.row.view.indentation + treeExpanderSize
}
func (l *treeRowLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	width, height := l.prefix()+treeRowPadding, treeRowMinHeight
	if len(children) > 1 {
		content := children[1].Measure(layout.Loose(geometry.Size{Width: max(0, c.Max.Width-width), Height: max(0, c.Max.Height-2*treeRowPadding)}))
		width += content.Width
		height = max(height, content.Height+2*treeRowPadding)
	}
	return layout.Measured(c.Clamp(geometry.Size{Width: width, Height: height}))
}
func (l *treeRowLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	if len(children) == 0 {
		return
	}
	prefix := l.prefix()
	h := min(treeExpanderSize, rect.Height)
	children[0].Arrange(geometry.Rect(rect.X+prefix-treeExpanderSize, rect.Y+(rect.Height-h)/2, min(treeExpanderSize, max(0, rect.Width-prefix+treeExpanderSize)), h))
	if len(children) > 1 {
		content := children[1].Measure(layout.Loose(geometry.Size{Width: max(0, rect.Width-prefix-treeRowPadding), Height: max(0, rect.Height-2*treeRowPadding)}))
		children[1].Arrange(geometry.Rect(rect.X+prefix, rect.Y+(rect.Height-content.Height)/2, content.Width, content.Height))
	}
}
