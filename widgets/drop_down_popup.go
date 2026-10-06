package widgets

import (
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

const dropDownPopupPadding = 6

// Content owns a finite viewport, not an eagerly constructed option tree.
// Its ListView remains a layout/recycling mechanism; selection stays in owner.
type dropDownContent struct {
	gui.WidgetBase
	owner      *DropDown
	list       *gui.ListView
	scroll     *gui.ScrollView
	padding    float32
	primeWidth float32
	primeDirty bool
}

func newDropDownContent(owner *DropDown) *dropDownContent {
	c := &dropDownContent{owner: owner}
	c.SetFocusable(true)
	c.list = gui.NewListView()
	c.list.SetDelegate(&dropDownListDelegate{owner: owner, delegate: owner.rowDelegate()})
	c.list.SetModel(owner.model)
	c.scroll = gui.NewScrollView()
	c.scroll.SetChild(c.list)
	c.WidgetBase.AddChild(c, c.scroll)
	c.AddEventController(&dropDownPopupInput{
		EventControllerBase: gui.NewEventControllerBase(gui.PhaseCapture), owner: owner,
	})
	keys := gui.NewKeyEventController()
	keys.SetPhase(gui.PhaseCapture)
	keys.ConnectKeyDown(c.keyDown)
	c.AddEventController(keys)
	c.ConnectUnmount(func() { c.list.SetModel(nil) })
	c.ConnectMount(func() {
		if c.list.Model() == nil {
			c.list.SetModel(owner.model)
		}
	})
	return c
}

// The popup is a separate Root. Check its owner's current eligibility before
// ordinary input reaches a custom option widget, then dismiss a stale popup.
type dropDownPopupInput struct {
	gui.EventControllerBase
	owner *DropDown
}

func (c *dropDownPopupInput) HandleEvent(ctx gui.EventContext) {
	if !c.owner.Destroyed() && c.owner.Opened() && gui.IsEnabled(c.owner) {
		return
	}
	ctx.StopPropagation()
	if key, ok := ctx.Event().(events.KeyEvent); ok {
		key.PreventDefault()
	}
	c.owner.Close()
}

func (c *dropDownContent) keyDown(ctx gui.EventContext, event events.KeyEvent) {
	d := c.owner
	if !d.Opened() || !gui.IsEnabled(d) || d.Destroyed() {
		return
	}
	plain := func(key gui.Key) bool { return (gui.KeyGesture{Key: key}).Matches(event) }
	if plain(gui.KeyEnter) || plain(gui.KeySpace) {
		if !event.Repeat {
			d.choose(d.current)
		}
	} else if plain(gui.KeyEscape) {
		d.Close()
	} else if plain(gui.KeyTab) || (gui.KeyGesture{Key: gui.KeyTab, Modifiers: gui.ModShift}).Matches(event) {
		win := d.Window()
		d.Close()
		ctx.StopPropagation()
		event.PreventDefault()
		if win != nil && !d.Destroyed() {
			// The owner's original key was consumed by modal routing. A fresh
			// consumption flag lets its ordinary controller/shortcut/Tab path run.
			handled := false
			event.Handled = &handled
			_ = win.DispatchEvent(event)
		}
		return
	} else {
		index := -1
		switch {
		case plain(gui.KeyArrowDown):
			index = d.neighbor(d.current, 1)
		case plain(gui.KeyArrowUp):
			index = d.neighbor(d.current, -1)
		case plain(gui.KeyHome):
			index = d.neighbor(-1, 1)
		case plain(gui.KeyEnd):
			index = d.neighbor(d.count(), -1)
		default:
			return
		}
		if index >= 0 && index != d.current {
			d.current = index
			c.list.RequestPaint()
			c.list.Reveal(index)
		}
	}
	ctx.StopPropagation()
	event.PreventDefault()
}

func (c *dropDownContent) Measure(constraint layout.Constraint) layout.Measurement {
	s := gui.ResolveStyle("drop-down-popup", style.PartDefault, style.Normal)
	radius, _ := s.Radius()
	border, _ := s.BorderWidth()
	c.padding = max(float32(dropDownPopupPadding), dropDownExtent(border), dropDownExtent(radius)*(1-1/math.Sqrt2)+1)
	width := constraint.Clamp(geometry.Size{Width: c.owner.Rect().Width}).Width
	limit := constraint.Max.Height
	if c.owner.popupMaxHeight > 0 {
		limit = min(limit, c.owner.popupMaxHeight)
	}
	// Prime at most one modest viewport to seed ListView's own height estimate.
	// With no user cap the natural height can be large, but the Popover then
	// remeasures against its work area before constructing the native surface.
	warm := max(float32(0), min(limit, float32(320))-2*c.padding)
	if c.primeDirty || c.primeWidth != width || len(c.list.Children()) == 0 {
		c.primeDirty, c.primeWidth = false, width
		c.list.LayoutVisible(geometry.Size{Width: max(0, width-2*c.padding), Height: warm}, geometry.Point{X: c.scroll.ScrollX(), Y: c.scroll.ScrollY()})
	}
	height := min(c.list.ContentSize().Height+2*c.padding, limit)
	return layout.Measured(constraint.Clamp(geometry.Size{Width: width, Height: height}))
}

func (c *dropDownContent) StyleChanged() { c.primeDirty = true }

func (c *dropDownContent) Arrange(rect geometry.Rectangle) {
	c.WidgetBase.Arrange(rect)
	inner := geometry.Rect(0, 0, rect.Width, rect.Height).Inset(c.padding)
	c.scroll.Measure(layout.Tight(inner.Size))
	c.scroll.Arrange(inner)
	// Keep the option-to-scrollbar gap equal to the leading surface padding.
	// Use the completed content height, since virtualization refines estimates
	// during Arrange. The gutter belongs to the row's scroll extent, outside its
	// painted/pickable bounds, so wide content remains reachable when scrolled.
	gap := float32(0)
	if c.list.ContentSize().Height > inner.Height {
		gap = c.padding
	}
	delegate := c.list.Delegate().(*dropDownListDelegate)
	if delegate.trailingGap != gap {
		delegate.trailingGap = gap
		for _, widget := range c.list.Children() {
			row := widget.(*dropDownRow)
			row.trailingGap = gap
			row.RequestLayout()
		}
		c.list.StyleChanged()
		c.scroll.Measure(layout.Tight(inner.Size))
		c.scroll.Arrange(inner)
	}
}

func (c *dropDownContent) Snapshot() gui.WidgetInfo {
	info := c.WidgetBase.Snapshot()
	if c.owner.Destroyed() || !gui.IsEnabled(c.owner) {
		disableDropDownSnapshot(&info)
	}
	info.Role = RoleListBox
	info.SetAttribute(DropDownInfoKey, DropDownInfo{Count: c.owner.count(), Index: c.owner.selected, Expanded: c.owner.Opened()})
	return info
}

// Detached semantic data must reflect the cross-Root input restriction too.
func disableDropDownSnapshot(info *gui.WidgetInfo) {
	info.Enabled = false
	info.Actions = nil
	info.DragDrop = nil
	for i := range info.Children {
		disableDropDownSnapshot(&info.Children[i])
	}
}

type dropDownListDelegate struct {
	owner       *DropDown
	delegate    gui.ListItemDelegate
	trailingGap float32
}

func (d *dropDownListDelegate) Setup() gui.Widget {
	r := &dropDownRow{owner: d.owner, delegate: d.delegate, index: -1, trailingGap: d.trailingGap}
	r.SetLayoutManager(dropDownRowLayout{})
	if d.delegate != nil {
		r.content = d.delegate.Setup()
	}
	if r.content != nil {
		r.WidgetBase.AddChild(r, r.content)
	}
	r.motion = gui.NewMotionEventController()
	r.motion.ConnectContainsHover(func(hover bool) { r.hovered = hover; r.RequestPaint() })
	r.AddEventController(r.motion)
	r.AddEventController(&dropDownDisabledInput{EventControllerBase: gui.NewEventControllerBase(gui.PhaseCapture), row: r})
	r.click = gui.NewClickEventController()
	r.click.ConnectPressed(func(_ gui.EventContext, pressed bool) { r.pressed = pressed; r.RequestPaint() })
	r.click.ConnectClicked(func(ctx gui.EventContext) {
		if r.bound && !r.disabled {
			ctx.StopPropagation()
			r.owner.choose(r.index)
		}
	})
	r.ConnectUnmount(func() { d.Unbind(0, r) })
	return r
}

func (d *dropDownListDelegate) Bind(index int, widget gui.Widget) {
	r := widget.(*dropDownRow)
	if d.owner.Destroyed() || index >= d.owner.count() {
		return
	}
	if r.bound && r.index != index {
		d.Unbind(r.index, r)
	}
	if d.owner.Destroyed() {
		return
	}
	item := d.owner.model.ItemAt(index)
	r.trailingGap = d.trailingGap
	r.index, r.bound, r.disabled = index, true, item.Disabled
	if item.Disabled {
		r.RemoveEventController(r.click)
	} else {
		r.AddEventController(r.click)
	}
	if r.delegate != nil && r.content != nil {
		r.delegate.Bind(index, r.content)
	}
	r.RequestLayout()
}

func (*dropDownListDelegate) Unbind(_ int, widget gui.Widget) {
	r := widget.(*dropDownRow)
	if !r.bound {
		return
	}
	index := r.index
	r.bound, r.index, r.hovered, r.pressed = false, -1, false, false
	r.click.Reset()
	r.motion.Reset()
	if r.delegate != nil && r.content != nil {
		r.delegate.Unbind(index, r.content)
	}
}

type dropDownRow struct {
	gui.WidgetBase
	owner                             *DropDown
	delegate                          gui.ListItemDelegate
	content                           gui.Widget
	index                             int
	trailingGap                       float32
	bound, disabled, hovered, pressed bool
	motion                            *gui.MotionEventController
	click                             *gui.ClickEventController
}

func (r *dropDownRow) Measure(constraint layout.Constraint) layout.Measurement {
	inner := constraint
	inner.Min.Width = max(0, inner.Min.Width-r.trailingGap)
	inner.Max.Width = max(0, inner.Max.Width-r.trailingGap)
	measured := r.WidgetBase.Measure(inner)
	measured.Width += r.trailingGap
	return measured
}

func (r *dropDownRow) Arrange(rect geometry.Rectangle) {
	// ListView's full-width slot includes the trailing gutter. Keep it outside
	// the actual widget so Paint, input picking and Snapshot share one boundary.
	rect.Width = max(0, rect.Width-r.trailingGap)
	r.WidgetBase.Arrange(rect)
}

// A disabled option also blocks actions in custom content; wheel events still
// bubble normally to the scrolling container.
type dropDownDisabledInput struct {
	gui.EventControllerBase
	row *dropDownRow
}

func (c *dropDownDisabledInput) HandleEvent(ctx gui.EventContext) {
	if event, ok := ctx.Event().(events.PointerEvent); ok && c.row.disabled && (event.EventType == events.PointerDown || event.EventType == events.PointerUp) {
		ctx.StopPropagation()
	}
}

func (r *dropDownRow) Paint(p gui.Painter) {
	if !r.bound {
		return
	}
	state := style.Normal
	if r.disabled {
		state = style.Disabled
	} else if r.pressed {
		state = style.Pressed
	} else if r.hovered || r.owner.current == r.index {
		state = style.Hovered
	}
	part := style.PartDefault
	if r.owner.selected == r.index {
		part = "selected"
	}
	rect := geometry.Rectangle{Size: r.Rect().Size}
	paintStyledBox(p, rect, "drop-down-item", part, state)
	if r.owner.selected == r.index {
		s := gui.ResolveStyle("drop-down-item", "check", state)
		if mark, ok := s.ForegroundColor(); ok && mark != nil {
			x, y := rect.Width-12, rect.Height/2
			color := graphics.ColorOf(mark)
			p.DrawLine(geometry.Point{X: x - 4, Y: y}, geometry.Point{X: x - 1, Y: y + 3}, 1.5, color)
			p.DrawLine(geometry.Point{X: x - 1, Y: y + 3}, geometry.Point{X: x + 4, Y: y - 3}, 1.5, color)
		}
	}
}

func (r *dropDownRow) Snapshot() gui.WidgetInfo {
	info := r.WidgetBase.Snapshot()
	info.Role, info.Enabled, info.Selected = RoleOption, info.Enabled && r.bound && !r.disabled && gui.IsEnabled(r.owner), r.bound && r.owner.selected == r.index
	if r.owner.Destroyed() || !gui.IsEnabled(r.owner) {
		disableDropDownSnapshot(&info)
	}
	if r.bound && r.index < r.owner.count() {
		info.Text = r.owner.model.ItemAt(r.index).Text
	}
	if info.Enabled {
		info.Actions = append(info.Actions, gui.ActionClick)
	}
	info.SetAttribute(DropDownInfoKey, DropDownInfo{Count: r.owner.count(), Index: r.index, Current: r.bound && r.owner.current == r.index})
	return info
}

type dropDownRowLayout struct{}

func (dropDownRowLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	var content layout.Measurement
	if len(children) > 0 {
		content = children[0].Measure(layout.Loose(geometry.Size{Width: max(0, c.Max.Width-30), Height: max(0, c.Max.Height-12)}))
	}
	height := content.Height
	content.Size = c.Clamp(geometry.Size{Width: content.Width + 30, Height: max(float32(28), height+12)})
	content.Baseline += (content.Height - height) / 2
	return content
}
func (dropDownRowLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	if len(children) == 0 {
		return
	}
	area := geometry.Size{Width: max(0, rect.Width-30), Height: max(0, rect.Height-12)}
	content := children[0].Measure(layout.Loose(area))
	children[0].Arrange(geometry.Rect(rect.X+6, rect.Y+(rect.Height-content.Height)/2, area.Width, content.Height))
}
