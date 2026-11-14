package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
)

const (
	treeRowMinHeight float32 = 28
	treeExpanderSize float32 = 20
)

var treeRowPadding = geometry.UniformInsets(4)

func treeContentStart(depth int, indentation float32) float32 {
	return treeRowPadding.Left + float32(max(0, depth))*indentation + treeExpanderSize
}

type treeListDelegate struct {
	view     *TreeView
	delegate TreeItemDelegate
}

func (d *treeListDelegate) Setup() gui.Widget {
	r := &treeRow{view: d.view, delegate: d.delegate}
	r.SetMinSize(geometry.Size{Height: treeRowMinHeight})
	r.SetLayoutManager(&treeRowLayout{row: r})
	if d.delegate != nil {
		r.content = d.delegate.Setup()
	}
	if r.content != nil {
		r.WidgetBase.AddChild(r, r.content)
	}
	r.motion = gui.NewMotionEventController()
	r.AddEventController(r.motion)
	r.click = gui.NewClickEventController()
	r.click.ConnectPressedChanged(func(pressed bool) { r.SetState(style.Pressed, pressed) })
	r.click.ConnectClicked(func(ctx gui.EventContext, inputInfo gui.ClickInfo) {
		count := inputInfo.Count
		if r.bound.ID == "" || r.view.Destroyed() {
			return
		}
		ctx.StopPropagation()
		id := r.bound.ID
		version := r.view.modelRevision
		primary, shift := treeModifiers(inputInfo.Modifiers)
		r.view.choose(id, primary, shift)
		if !r.view.validModel(version) {
			return
		}
		if count == 2 {
			r.view.activate.Emit(id, version)
		}
	})
	r.AddEventController(r.click)
	// Rows can leave a host without scrolling (window teardown or subtree
	// transfer). Release the last binding then too. Ordinary recycling already
	// clears bound before detaching, so this never calls Unbind twice.
	r.ConnectUnmount(func() { d.Unbind(0, r) })
	return r
}

func (d *treeListDelegate) ArrangeItem(_ int, widget gui.Widget, bounds geometry.Rectangle, viewport geometry.Size, offset geometry.Point) {
	r := widget.(*treeRow)
	r.scrollX = offset.X
	left := min(d.view.rowInsetLeft, max(0, viewport.Width))
	r.Arrange(geometry.Rect(left, bounds.Y, max(0, viewport.Width-left-d.view.rowInsetRight), bounds.Height))
}
func (d *treeListDelegate) Bind(index int, widget gui.Widget) {
	r := widget.(*treeRow)
	v := d.view
	if v.Destroyed() || index < 0 || index >= len(v.rows) {
		return
	}
	flat := v.rows[index]
	row := v.rowState(flat)
	version := v.revision
	if r.bound.ID != "" && r.bound.ID != row.ID {
		d.Unbind(index, widget)
		if !v.valid(version) || r.Destroyed() {
			return
		}
	}
	r.binding++
	binding := r.binding
	r.bound, r.flat = row, flat
	r.SetState(style.Selected, row.Selected)
	v.realized[row.ID] = r
	current := func() bool {
		return v.valid(version) && !r.Destroyed() && r.binding == binding &&
			r.bound == row && v.realized[row.ID] == r && v.rowState(flat) == row
	}
	name, depth := "", row.Depth
	if v.rowStyle != nil {
		name = v.rowStyle(row)
		if !current() {
			return
		}
	}
	if v.rowContentDepth != nil {
		if value, override := v.rowContentDepth(row); override {
			depth = max(0, value)
		}
		if !current() {
			return
		}
	}
	r.SetStyleName(name)
	r.contentDepth = depth
	r.RequestPaint()
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
	r.binding++
	r.SetStyleName("")
	r.contentDepth = 0
	r.scrollX = 0
	r.SetState(style.Selected, false)
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
	row := TreeRow{
		ID: flat.id, Depth: flat.depth, Expandable: v.model.Expandable(flat.id),
		Expanded: v.expanded[flat.id], Selected: v.selected[flat.id], Current: v.current == flat.id,
	}
	if index, ok := v.indices[flat.id]; ok {
		if index > 0 {
			row.PreviousSelected = v.selected[v.rows[index-1].id]
		}
		if index+1 < len(v.rows) {
			row.NextSelected = v.selected[v.rows[index+1].id]
		}
	}
	return row
}

type treeRow struct {
	gui.WidgetBase
	view         *TreeView
	delegate     TreeItemDelegate
	bound        TreeRow
	flat         flatTreeRow
	content      gui.Widget
	motion       *gui.MotionEventController
	click        *gui.ClickEventController
	binding      uint64
	contentDepth int
	scrollX      float32
}

// The row is a viewport-sized clip and interaction surface. Its child retains
// natural width and scrolls inside it; the list's extent includes outer margins.
type treeRowLayout struct{ row *treeRow }

func (l *treeRowLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	insets := geometry.Insets{Left: l.row.view.rowInsetLeft, Right: l.row.view.rowInsetRight}
	size := geometry.Size{}
	if len(children) > 0 {
		size = children[0].Measure(layout.Loose(c.Insets(insets).Max)).Size
	}
	size.Width += insets.Horizontal()
	return layout.Measured(c.Clamp(size))
}

func (l *treeRowLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	if len(children) > 0 {
		size := children[0].Measure(layout.Unbounded()).Size
		children[0].Arrange(geometry.Rect(rect.X-l.row.scrollX, rect.Y, max(rect.Width, size.Width), rect.Height))
	}
}

func (r *treeRow) Paint(p gui.Painter) {
	if r.bound.ID == "" {
		return
	}
	state := r.State()
	focused := !state.Has(style.Disabled) && r.view.current == r.bound.ID && r.view.Focused()
	if focused {
		state |= style.Focused
		if r.view.FocusVisible() {
			state |= style.FocusVisible
		}
	}
	name := r.StyleName()
	if name == "" {
		name = "tree-item"
	}
	part := ""
	if state.Has(style.Selected) {
		part = "selected"
	}
	rect := geometry.Rect(0, 0, r.Rect().Width, r.Rect().Height)
	appearance := gui.ResolveWidgetStyle(r, name, part, state)
	radius, _ := appearance.CornerRadius()
	radius = radius.Normalize(rect.Size)
	joined := radius
	if state.Has(style.Selected) {
		if r.bound.PreviousSelected {
			joined.TopLeft, joined.TopRight = 0, 0
		}
		if r.bound.NextSelected {
			joined.BottomLeft, joined.BottomRight = 0, 0
		}
	}
	if joined == radius {
		gui.PaintStyledBox(p, rect, appearance)
	} else {
		// Join only the selection background. The original radius and edges
		// still describe this row's independent border, including drag feedback.
		background := (style.Rule{Style: appearance}).CornerRadius(joined).Style
		// Round shared edges on the same device grid even when one member
		// uses a path and its neighbor uses the rectangular fill fast path.
		gui.PaintStyledBackground(p, p.SnapRect(rect), background)
		gui.PaintStyledBorder(p, rect, appearance)
	}
	if focused {
		paintStyledBorder(p, rect, gui.ResolveWidgetStyle(r, name, "current", state))
	}
	// The content part decorates the full-height strip after the disclosure
	// slot, including application icons. It does not move or style children.
	start := treeContentStart(r.contentDepth, r.view.indentation) - r.scrollX
	if start < rect.Width {
		paintStyledBox(r, p, geometry.Rect(start, 0, rect.Width-start, rect.Height), name, "content", state)
	}
}
func (r *treeRow) Snapshot() gui.WidgetInfo {
	info := r.WidgetBase.Snapshot()
	info.Role = RoleTreeItem
	info.Selected = r.State().Has(style.Selected)
	info.SetAttribute(HierarchyInfoKey, HierarchyInfo{
		NodeID: r.bound.ID, ParentID: r.flat.parent, Level: r.flat.depth + 1,
		PositionInSet: r.flat.position, SetSize: r.flat.siblings,
		Expandable: r.bound.Expandable, Expanded: r.view.expanded[r.bound.ID], Current: r.view.current == r.bound.ID,
	})
	if info.Enabled {
		info.Actions = append(info.Actions, gui.ActionClick)
	}
	return info
}
