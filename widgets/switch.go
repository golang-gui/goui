package widgets

import (
	"math"
	"time"

	"github.com/golang-gui/goui/animation"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

const SwitchInfoKey = "goui.switch"

// SwitchInfo reports committed selection, independently of thumb movement.
type SwitchInfo struct {
	Checked bool `json:"checked"`
}

const switchDuration = 120 * time.Millisecond

// Switch is a two-state switch with optional, independently styled content.
// Setters synchronize state silently; Change reports user-caused transitions.
// Like other widgets, its methods belong to the GUI thread.
type Switch struct {
	gui.WidgetBase
	child                         gui.Widget
	checked, animated             bool
	padding                       float32
	hovered, pressed, mounted     bool
	revision                      uint64
	change                        signal.Signal2[bool, func() bool]
	motion                        *gui.MotionEventController
	click                         *gui.ClickEventController
	keys                          *gui.KeyEventController
	drag                          *switchDrag
	position, dragPosition, dragX float32
	transition                    *animation.Animation[float32]
}

// NewSwitch creates an enabled, focusable switch, initially off. The track is
// 36x20 DIP with a 16 DIP thumb, 6 DIP padding and an 8 DIP content gap.
func NewSwitch() *Switch {
	b := &Switch{animated: true, padding: 6}
	b.transition = animation.New(float32(0), animation.Float32)
	b.transition.ConnectUpdate(func(position float32) {
		if b.Destroyed() {
			b.transition.Stop()
			return
		}
		b.position = position
		if !b.canAnimate() {
			b.transition.Stop()
			b.position = b.target()
		}
		b.RequestPaint()
	})
	b.WidgetBase.SetFocusable(true)
	b.SetLayoutManager(switchLayout{b})
	b.ConnectMount(func() { b.mounted = true })
	b.ConnectUnmount(func() { b.mounted = false; b.revision++; b.cancelInput(); b.finishAnimation() })
	b.ConnectFocused(func(bool) { b.RequestPaint() })
	b.motion = gui.NewMotionEventController()
	b.motion.ConnectContainsHover(func(v bool) { b.hovered = v; b.RequestPaint() })
	b.click = gui.NewClickEventController()
	b.click.ConnectPressed(func(_ gui.EventContext, v bool) { b.pressed = v; b.RequestPaint() })
	b.click.ConnectClicked(func(ctx gui.EventContext) {
		ctx.StopPropagation()
		b.setChecked(!b.checked, true)
	})
	b.drag = &switchDrag{DragEventController: gui.NewDragEventController(), owner: b}
	b.drag.SetThreshold(4)
	b.drag.ConnectBegin(func(p geometry.Point, _ events.Modifiers) {
		b.stopAnimation()
		b.dragX, b.dragPosition = p.X, b.position
		b.RequestPaint()
	})
	b.drag.ConnectUpdate(func(p geometry.Point, _ events.Modifiers) { b.dragTo(p) })
	b.drag.ConnectEnd(func(p geometry.Point, _ events.Modifiers) {
		b.dragTo(p)
		checked := b.checked
		if b.position != .5 {
			checked = b.position > .5
		}
		b.setChecked(checked, true)
	})
	b.drag.ConnectCancel(func() { b.moveToTarget(); b.RequestPaint() })
	b.keys = gui.NewKeyEventController()
	b.keys.ConnectKeyDown(func(ctx gui.EventContext, e events.KeyEvent) {
		if (gui.KeyGesture{Key: gui.KeyEscape}).Matches(e) && (b.drag.armed || b.pressed) {
			b.cancelInput()
			b.moveToTarget()
		} else if (gui.KeyGesture{Key: gui.KeySpace}).Matches(e) || (gui.KeyGesture{Key: gui.KeyEnter}).Matches(e) {
			if !e.Repeat {
				b.cancelInput()
				b.setChecked(!b.checked, true)
			}
		} else {
			return
		}
		ctx.StopPropagation()
		e.PreventDefault()
	})
	b.addControllers()
	return b
}

func (b *Switch) Child() gui.Widget { return b.child }

// SetChild detaches the previous content; it does not destroy it.
func (b *Switch) SetChild(child gui.Widget) {
	if b.Destroyed() || child == b || b.child == child {
		return
	}
	if old := b.child; old != nil {
		b.child = nil
		b.RemoveChild(old)
	}
	if b.Destroyed() {
		return
	}
	b.child = child
	if child != nil {
		b.WidgetBase.AddChild(b, child)
		if child.Parent() != b {
			b.child = nil
		}
	}
	b.RequestLayout()
}

func (b *Switch) Checked() bool { return b.checked }

// SetChecked silently synchronizes selection. An unchanged value preserves
// any drag or transition, including during a declarative rebuild.
func (b *Switch) SetChecked(checked bool) {
	if b.Destroyed() || b.checked == checked {
		return
	}
	b.cancelInput()
	b.setChecked(checked, false)
}

func (b *Switch) setChecked(checked, notify bool) {
	if b.Destroyed() || !gui.IsEnabled(b) && notify {
		return
	}
	changed := b.checked != checked
	if changed {
		b.stopAnimation()
		b.checked = checked
		b.revision++
	}
	b.moveToTarget()
	b.RequestPaint()
	if changed && notify {
		revision := b.revision
		b.change.Emit(checked, func() bool { return !b.Destroyed() && b.revision == revision })
	}
}

// ConnectChange reports a completed user transition immediately, before the
// visual transition ends. Superseded or destroyed transitions stop notifying.
func (b *Switch) ConnectChange(fn func(bool)) signal.Handle {
	return b.change.Connect(func(checked bool, valid func() bool) {
		if valid() {
			fn(checked)
		}
	})
}

func (b *Switch) Padding() float32 { return b.padding }
func (b *Switch) SetPadding(padding float32) {
	if padding < 0 || math.IsNaN(float64(padding)) || math.IsInf(float64(padding), 0) {
		padding = 0
	}
	if !b.Destroyed() && b.padding != padding {
		b.cancelInput()
		b.padding = padding
		b.RequestLayout()
	}
}

func (b *Switch) Animated() bool { return b.animated }

// SetAnimated disables automatic transitions, not pointer-driven thumb motion.
func (b *Switch) SetAnimated(animated bool) {
	if b.Destroyed() || b.animated == animated {
		return
	}
	b.animated = animated
	if !animated {
		b.stopAnimation()
		if !b.drag.Dragging() {
			b.position = b.target()
		}
	}
	b.RequestPaint()
}

func (b *Switch) SetVisible(visible bool) {
	if !visible {
		b.cancelInput()
		b.finishAnimation()
	}
	b.WidgetBase.SetVisible(visible)
}

func (b *Switch) addControllers() {
	b.AddEventController(b.motion)
	b.AddEventController(b.drag)
	b.AddEventController(b.click)
	b.AddEventController(b.keys)
}

func (b *Switch) cancelInput() {
	b.drag.Reset()
	b.click.Reset()
	b.pressed = false
}

func (b *Switch) target() float32 {
	if b.checked {
		return 1
	}
	return 0
}

func (b *Switch) canAnimate() bool {
	if !b.animated || !gui.IsEnabled(b) || !b.mounted || b.Root() == nil || b.Destroyed() {
		return false
	}
	if track := b.trackRect(); track.Width <= 0 || track.Height <= 0 {
		return false
	}
	for w := gui.Widget(b); w != nil; w = w.Parent() {
		if !w.Visible() {
			return false
		}
	}
	if host, ok := b.Root().(interface{ Visible() bool }); ok && !host.Visible() {
		return false
	}
	return true
}

func (b *Switch) stopAnimation() {
	b.transition.Stop()
}

func (b *Switch) finishAnimation() { b.stopAnimation(); b.position = b.target() }

func (b *Switch) moveToTarget() {
	b.stopAnimation()
	if b.position == b.target() {
		return
	}
	if !b.canAnimate() {
		b.position = b.target()
		return
	}
	b.transition.SetValue(b.position)
	if b.transition.AnimateTo(b.Root(), b.target(), switchDuration, animation.EaseOutCubic) != nil {
		b.finishAnimation()
	}
}

func (b *Switch) trackRect() geometry.Rectangle {
	inner := geometry.Rect(0, 0, b.Rect().Width, b.Rect().Height).Inset(b.padding)
	scale := max(0, min(float32(1), min(inner.Width/36, inner.Height/20)))
	return geometry.Rect(inner.X, inner.Y+(inner.Height-20*scale)/2, 36*scale, 20*scale)
}

func (b *Switch) dragTo(p geometry.Point) {
	travel := b.trackRect().Width * 16 / 36
	if travel > 0 {
		b.position = min(1, max(0, b.dragPosition+(p.X-b.dragX)/travel))
	}
	b.RequestPaint()
}

func (b *Switch) Paint(p gui.Painter) {
	if !b.Visible() {
		return
	}
	name := b.StyleName()
	if name == "" {
		name = "switch"
	}
	state := style.Normal
	if !gui.IsEnabled(b) {
		state = style.Disabled
	} else if b.pressed || b.drag.Dragging() {
		state = style.Pressed
	} else if b.hovered {
		state = style.Hovered
	}
	suffix := ""
	if b.checked {
		suffix = "-checked"
	}
	track := b.trackRect()
	paintStyledBox(p, track, name, "track"+suffix, state)
	diameter, gap := track.Height*.8, track.Height*.1
	x := track.X + gap + diameter/2 + b.position*(track.Width-2*gap-diameter)
	center := geometry.Point{X: x, Y: track.Y + track.Height/2}
	s := gui.ResolveStyle(name, "thumb"+suffix, state)
	if bg, ok := s.BackgroundColor(); ok && bg != nil {
		p.FillEllipse(center, diameter/2, diameter/2, graphics.ColorOf(bg))
	}
	width, _ := s.BorderWidth()
	width = min(max(0, width), diameter)
	if edge, ok := s.BorderColor(); ok && edge != nil && width > 0 {
		p.DrawEllipse(center, (diameter-width)/2, (diameter-width)/2, width, graphics.ColorOf(edge))
	}
	if b.FocusVisible() {
		paintStyledBorder(p, track.Inset(-min(float32(2), b.padding)), gui.ResolveStyle(name, "focus", style.FocusVisible))
	}
}

func (b *Switch) Snapshot() gui.WidgetInfo {
	info := b.WidgetBase.Snapshot()
	info.Role = RoleSwitch
	if info.Enabled {
		info.Actions = append(info.Actions, gui.ActionClick)
	}
	info.SetAttribute(SwitchInfoKey, SwitchInfo{Checked: b.checked})
	return info
}

// The wrapper limits enrollment and recognition, retaining the standard drag
// controller's arbitration, capture, cancellation and coordinate handling.
type switchDrag struct {
	*gui.DragEventController
	owner *Switch
	armed bool
	start geometry.Point
}

func (d *switchDrag) Reset() { d.armed = false; d.DragEventController.Reset() }
func (d *switchDrag) GestureCanceled(reason gui.GestureCancelReason) {
	d.armed = false
	d.DragEventController.GestureCanceled(reason)
}
func (d *switchDrag) HandleEvent(ctx gui.EventContext) {
	e, ok := ctx.Event().(events.PointerEvent)
	if !ok {
		return
	}
	switch e.EventType {
	case events.PointerDown:
		point, valid := ctx.Position()
		r := d.owner.trackRect()
		if !valid || e.Button != events.PointerButtonLeft || point.X < r.X || point.X >= r.X+r.Width || point.Y < r.Y || point.Y >= r.Y+r.Height {
			return
		}
		d.armed, d.start = true, e.Position
	case events.PointerMove:
		if !d.armed {
			return
		}
		if !d.Dragging() {
			dx, dy := math.Abs(float64(e.Position.X-d.start.X)), math.Abs(float64(e.Position.Y-d.start.Y))
			if dy >= 4 && dy > dx {
				d.Reset()
				return
			}
			if dx < 4 {
				return
			}
		}
	case events.PointerUp:
		if !d.armed {
			return
		}
		d.armed = false
	}
	d.DragEventController.HandleEvent(ctx)
}

type switchLayout struct{ button *Switch }

func (l switchLayout) content(children []layout.Child, inner geometry.Size) (layout.Measurement, float32) {
	reserve := float32(36)
	if len(children) > 0 {
		reserve += 8
	}
	reserve = min(reserve, inner.Width)
	if len(children) == 0 {
		return layout.Measurement{}, reserve
	}
	return children[0].Measure(layout.Loose(geometry.Size{Width: max(0, inner.Width-reserve), Height: inner.Height})), reserve
}
func (l switchLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	b := l.button
	child, reserve := l.content(children, c.Inset(b.padding).Max)
	size := c.Clamp(geometry.Size{Width: reserve + child.Width + 2*b.padding, Height: max(float32(20), child.Height) + 2*b.padding})
	inner := geometry.Rect(0, 0, size.Width, size.Height).Inset(b.padding)
	return layout.Measurement{Size: size, HasBaseline: child.HasBaseline, Baseline: inner.Y + (inner.Height-child.Height)/2 + child.Baseline}
}
func (l switchLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	if len(children) == 0 {
		return
	}
	inner := rect.Inset(l.button.padding)
	child, reserve := l.content(children, inner.Size)
	children[0].Arrange(geometry.Rect(inner.X+reserve, inner.Y+(inner.Height-child.Height)/2, child.Width, child.Height))
}

var _ gui.Bin = (*Switch)(nil)
var _ gui.GestureController = (*switchDrag)(nil)
