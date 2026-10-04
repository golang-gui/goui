package widgets

import (
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

// TreeExpander composes indentation, a disclosure button and one content child.
// It has no model or TreeView ownership. Toggle requests an application update;
// SetExpanded synchronizes state without emitting Toggle. All methods run on
// the GUI thread. Content keeps its own StyleName.
type TreeExpander struct {
	gui.WidgetBase
	child                gui.Widget
	button               *treeDisclosure
	depth                int
	indentation          float32
	expandable, expanded bool
	toggle               signal.Signal0
}

func NewTreeExpander() *TreeExpander {
	e := &TreeExpander{indentation: 16}
	e.SetMinSize(geometry.Size{Height: treeRowMinHeight})
	e.SetLayoutManager(&treeExpanderLayout{e: e})
	e.button = &treeDisclosure{expander: e}
	e.button.click = gui.NewClickEventController()
	e.button.motion = gui.NewMotionEventController()
	e.button.click.ConnectPressed(func(_ gui.EventContext, v bool) { e.button.pressed = v; e.button.RequestPaint() })
	e.button.click.ConnectClicked(func(ctx gui.EventContext) {
		ctx.StopPropagation()
		if !e.Destroyed() && e.expandable {
			e.toggle.Emit()
		}
	})
	e.button.motion.ConnectHover(func(v bool) { e.button.hover = v; e.button.RequestPaint() })
	e.WidgetBase.AddStructuralChild(e, e.button)
	return e
}

func (e *TreeExpander) Child() gui.Widget {
	return e.child
}
func (e *TreeExpander) SetChild(w gui.Widget) {
	if e.Destroyed() || e.Child() == w || w == e || w == e.button {
		return
	}
	if old := e.Child(); old != nil {
		e.child = nil
		e.WidgetBase.RemoveChild(old)
	}
	if e.Destroyed() {
		return
	}
	e.child = w
	if w != nil {
		e.WidgetBase.AddChild(e, w)
		if w.Parent() != e {
			e.child = nil
		}
	}
	e.RequestLayout()
}
func (e *TreeExpander) Depth() int { return e.depth }
func (e *TreeExpander) SetDepth(v int) {
	v = max(0, v)
	if e.depth != v {
		e.depth = v
		e.RequestLayout()
	}
}
func (e *TreeExpander) Indentation() float32 { return e.indentation }
func (e *TreeExpander) SetIndentation(v float32) {
	if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
		return
	}
	v = max(0, v)
	if e.indentation != v {
		e.indentation = v
		e.RequestLayout()
	}
}
func (e *TreeExpander) Expandable() bool { return e.expandable }
func (e *TreeExpander) SetExpandable(v bool) {
	if e.Destroyed() || e.expandable == v {
		return
	}
	e.expandable = v
	if v {
		e.button.AddEventController(e.button.click)
		e.button.AddEventController(e.button.motion)
	} else {
		e.button.RemoveEventController(e.button.click)
		e.button.RemoveEventController(e.button.motion)
		e.button.pressed, e.button.hover = false, false
	}
	e.button.RequestPaint()
}
func (e *TreeExpander) Expanded() bool { return e.expanded }
func (e *TreeExpander) SetExpanded(v bool) {
	if e.expanded != v {
		e.expanded = v
		e.button.RequestPaint()
	}
}
func (e *TreeExpander) ConnectToggle(fn func()) signal.Handle {
	return e.toggle.Connect(func() {
		if !e.Destroyed() {
			fn()
		}
	})
}

type treeDisclosure struct {
	gui.WidgetBase
	expander       *TreeExpander
	click          *gui.ClickEventController
	motion         *gui.MotionEventController
	hover, pressed bool
}

func (b *treeDisclosure) Paint(p gui.Painter) {
	e := b.expander
	if !e.expandable {
		return
	}
	name := e.StyleName()
	if name == "" {
		name = "tree-expander"
	}
	state := style.Normal
	if b.hover {
		state = style.Hovered
	}
	if b.pressed {
		state = style.Pressed
	}
	paintStyledBox(p, geometry.Rect(0, 0, b.Rect().Width, b.Rect().Height), name, "", state)
	fg, ok := gui.ResolveStyle(name, "", state).ForegroundColor()
	if !ok || fg == nil {
		return
	}
	center := geometry.Point{X: b.Rect().Width / 2, Y: b.Rect().Height / 2}
	a, c, d := geometry.Point{X: -2, Y: -4}, geometry.Point{X: 2}, geometry.Point{X: -2, Y: 4}
	if e.expanded {
		a, c, d = geometry.Point{X: -4, Y: -2}, geometry.Point{Y: 2}, geometry.Point{X: 4, Y: -2}
	}
	p.DrawLine(center.Add(a), center.Add(c), 1.5, graphics.ColorOf(fg))
	p.DrawLine(center.Add(c), center.Add(d), 1.5, graphics.ColorOf(fg))
}
func (b *treeDisclosure) Snapshot() gui.WidgetInfo {
	info := b.WidgetBase.Snapshot()
	if b.expander.expandable {
		info.Role = gui.RoleButton
		info.Name = "展开"
		if b.expander.expanded {
			info.Name = "折叠"
		}
		info.Actions = append(info.Actions, gui.ActionClick)
	}
	return info
}
func (e *TreeExpander) Snapshot() gui.WidgetInfo {
	info := e.WidgetBase.Snapshot()
	info.Role = gui.RoleBox
	if !e.expandable && len(info.Children) > 0 {
		info.Children = info.Children[1:]
	}
	return info
}

type treeExpanderLayout struct{ e *TreeExpander }

func (l *treeExpanderLayout) prefix() float32 {
	return treeRowPadding + float32(l.e.depth)*l.e.indentation + treeExpanderSize
}
func (l *treeExpanderLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	w, h := l.prefix()+treeRowPadding, treeRowMinHeight
	if len(children) > 1 {
		m := children[1].Measure(layout.Loose(geometry.Size{Width: max(0, c.Max.Width-w), Height: max(0, c.Max.Height-2*treeRowPadding)}))
		w += m.Width
		h = max(h, m.Height+2*treeRowPadding)
	}
	return layout.Measured(c.Clamp(geometry.Size{Width: w, Height: h}))
}
func (l *treeExpanderLayout) Arrange(children []layout.Child, r geometry.Rectangle) {
	if len(children) == 0 {
		return
	}
	prefix := l.prefix()
	h := min(treeExpanderSize, r.Height)
	children[0].Arrange(geometry.Rect(r.X+prefix-treeExpanderSize, r.Y+(r.Height-h)/2, min(treeExpanderSize, max(0, r.Width-prefix+treeExpanderSize)), h))
	if len(children) > 1 {
		m := children[1].Measure(layout.Loose(geometry.Size{Width: max(0, r.Width-prefix-treeRowPadding), Height: max(0, r.Height-2*treeRowPadding)}))
		children[1].Arrange(geometry.Rect(r.X+prefix, r.Y+(r.Height-m.Height)/2, max(0, r.Width-prefix-treeRowPadding), m.Height))
	}
}
