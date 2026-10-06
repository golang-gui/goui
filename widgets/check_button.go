package widgets

import (
	"math"
	"weak"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

// CheckState describes a selection, independently of hover or press feedback.
type CheckState uint8

const (
	CheckUnchecked CheckState = iota
	CheckChecked
	CheckMixed
)

// CheckAppearance selects an indicator with content or a checkable button.
// Grouped indicators are round (radio); independent indicators are square.
type CheckAppearance uint8

const (
	CheckAppearanceIndicator CheckAppearance = iota
	CheckAppearanceButton
)

const CheckInfoKey = "goui.check"

const checkIndicatorSize float32 = 18

// CheckInfo is detached, JSON-serializable selection data, not keyboard focus.
type CheckInfo struct {
	State      CheckState      `json:"state"`
	Appearance CheckAppearance `json:"appearance"`
	Grouped    bool            `json:"grouped"`
}

// CheckGroup enforces at most one checked member. It owns neither widgets nor
// layout. An empty group is valid; removing a selection does not choose another.
// Like CheckButton, all operations belong to the GUI thread. The weak reference
// lets a long-lived group outlive its members without keeping a widget tree alive.
type CheckGroup struct {
	checked  weak.Pointer[CheckButton]
	revision uint64
}

func NewCheckGroup() *CheckGroup { return new(CheckGroup) }

// Checked returns the selected live member, or nil. Destroying the selected
// widget leaves the group empty, even if the application retains that widget.
func (g *CheckGroup) Checked() *CheckButton {
	if b := g.checked.Value(); b != nil && !b.Destroyed() && b.group == g && b.state == CheckChecked {
		return b
	}
	return nil
}

// CheckButton supplies checkbox, radio and toggle-button behavior with one
// optional content child. Programmatic setters never emit Change. Content keeps
// its own style and input handling; a child that wins a click prevents toggling.
type CheckButton struct {
	gui.WidgetBase
	child      gui.Widget
	state      CheckState
	appearance CheckAppearance
	group      *CheckGroup
	padding    float32
	hovered    bool
	pressed    bool
	revision   uint64
	change     signal.Signal2[CheckState, func() bool]
	motion     *gui.MotionEventController
	click      *gui.ClickEventController
	keys       *gui.KeyEventController
}

// NewCheckButton creates an unchecked, focusable square indicator with 6 DIP
// padding, an 18 DIP indicator and an 8 DIP content gap. It has no implicit label.
func NewCheckButton() *CheckButton {
	b := &CheckButton{padding: 6}
	b.WidgetBase.SetFocusable(true)
	b.SetLayoutManager(checkLayout{button: b})
	b.ConnectFocused(func(bool) { b.RequestPaint() })
	b.motion = gui.NewMotionEventController()
	b.motion.ConnectContainsHover(func(v bool) { b.hovered = v; b.RequestPaint() })
	b.click = gui.NewClickEventController()
	b.click.ConnectPressed(func(_ gui.EventContext, v bool) { b.pressed = v; b.RequestPaint() })
	b.click.ConnectClicked(func(ctx gui.EventContext) { ctx.StopPropagation(); b.activate() })
	b.keys = gui.NewKeyEventController()
	b.keys.ConnectKeyDown(func(ctx gui.EventContext, e events.KeyEvent) {
		for _, key := range []gui.Key{gui.KeySpace, gui.KeyEnter} {
			if (gui.KeyGesture{Key: key}).Matches(e) {
				ctx.StopPropagation()
				e.PreventDefault()
				if !e.Repeat {
					b.activate()
				}
				return
			}
		}
	})
	b.addControllers()
	return b
}

func (b *CheckButton) Child() gui.Widget { return b.child }

func (b *CheckButton) SetChild(child gui.Widget) {
	if b.Destroyed() || child == b || b.child == child {
		return
	}
	if old := b.child; old != nil {
		b.child = nil
		b.WidgetBase.RemoveChild(old)
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

func (b *CheckButton) CheckState() CheckState { return b.state }
func (b *CheckButton) Checked() bool          { return b.state == CheckChecked }

// SetCheckState synchronizes selection without Change. Unknown values become
// Unchecked. Mixed is unsupported in a group and becomes Unchecked there.
func (b *CheckButton) SetCheckState(state CheckState) { b.setState(state, false) }

func (b *CheckButton) SetChecked(checked bool) {
	state := CheckUnchecked
	if checked {
		state = CheckChecked
	}
	b.SetCheckState(state)
}

func (b *CheckButton) Group() *CheckGroup { return b.group }

// SetGroup assigns explicit exclusivity; parentage does not imply a group.
// A checked member joining a group replaces the selection silently. Detaching
// keeps its state. Group identity survives unmount/remount and window transfers.
func (b *CheckButton) SetGroup(group *CheckGroup) {
	if b.Destroyed() || b.group == group {
		return
	}
	if old := b.group; old != nil && old.Checked() == b {
		old.checked = weak.Pointer[CheckButton]{}
		old.revision++
	}
	b.group = group
	b.revision++
	if group != nil {
		if b.state == CheckMixed {
			b.commit(CheckUnchecked)
		} else if b.state == CheckChecked {
			// Joining must also resolve an already-checked incoming member.
			b.setState(CheckChecked, false)
		}
	}
	b.RequestPaint()
}

func (b *CheckButton) Appearance() CheckAppearance { return b.appearance }

func (b *CheckButton) SetAppearance(appearance CheckAppearance) {
	if appearance != CheckAppearanceButton {
		appearance = CheckAppearanceIndicator
	}
	if !b.Destroyed() && b.appearance != appearance {
		b.appearance = appearance
		b.RequestLayout()
	}
}

func (b *CheckButton) Padding() float32 { return b.padding }

// SetPadding sets equal inner padding in DIP; invalid values become zero.
func (b *CheckButton) SetPadding(padding float32) {
	if padding < 0 || math.IsNaN(float64(padding)) || math.IsInf(float64(padding), 0) {
		padding = 0
	}
	if !b.Destroyed() && b.padding != padding {
		b.padding = padding
		b.RequestLayout()
	}
}

func (b *CheckButton) addControllers() {
	b.AddEventController(b.motion)
	b.AddEventController(b.click)
	b.AddEventController(b.keys)
}

// ConnectChange reports user-caused transitions, including the previous member
// becoming Unchecked in a group. All member states are committed first. Later
// notifications are skipped if a callback destroys or supersedes the transition.
func (b *CheckButton) ConnectChange(fn func(CheckState)) signal.Handle {
	return b.change.Connect(func(state CheckState, valid func() bool) {
		if valid() {
			fn(state)
		}
	})
}

func (b *CheckButton) activate() {
	if b.Destroyed() || !gui.IsEnabled(b) || !b.Visible() {
		return
	}
	state := CheckChecked
	if b.state == CheckChecked {
		if b.group != nil {
			return
		}
		state = CheckUnchecked
	}
	b.setState(state, true)
}

func (b *CheckButton) commit(state CheckState) {
	if b.state != state {
		b.state = state
		b.revision++
		b.RequestPaint()
	}
}

func (b *CheckButton) setState(state CheckState, notify bool) {
	if b.Destroyed() {
		return
	}
	if state > CheckMixed || b.group != nil && state == CheckMixed {
		state = CheckUnchecked
	}
	g := b.group
	var old *CheckButton
	if g != nil && state == CheckChecked {
		old = g.Checked()
		if old == b {
			return
		}
		if old != nil {
			old.commit(CheckUnchecked)
		}
		g.checked = weak.Make(b)
		g.revision++
	} else if b.state == state {
		return
	} else if g != nil && g.Checked() == b {
		g.checked = weak.Pointer[CheckButton]{}
		g.revision++
	}
	b.commit(state)
	if !notify {
		return
	}
	groupRevision, revision := uint64(0), b.revision
	if g != nil {
		groupRevision = g.revision
	}
	valid := func() bool {
		return !b.Destroyed() && b.revision == revision && b.group == g &&
			(g == nil || g.revision == groupRevision)
	}
	if old != nil {
		oldRevision := old.revision
		old.change.Emit(CheckUnchecked, func() bool { return valid() && !old.Destroyed() && old.revision == oldRevision })
	}
	b.change.Emit(state, valid)
}

func (b *CheckButton) Snapshot() gui.WidgetInfo {
	info := b.WidgetBase.Snapshot()
	info.Role = RoleCheckBox
	if b.appearance == CheckAppearanceButton {
		info.Role = RoleToggleButton
	} else if b.group != nil {
		info.Role = RoleRadioButton
	}
	if info.Enabled {
		info.Actions = append(info.Actions, gui.ActionClick)
	}
	info.SetAttribute(CheckInfoKey, CheckInfo{State: b.state, Appearance: b.appearance, Grouped: b.group != nil})
	return info
}

func (b *CheckButton) Paint(p gui.Painter) {
	if !b.Visible() {
		return
	}
	name := b.StyleName()
	if name == "" {
		name = "check-button"
	}
	state := style.Normal
	if !gui.IsEnabled(b) {
		state = style.Disabled
	} else if b.pressed {
		state = style.Pressed
	} else if b.hovered {
		state = style.Hovered
	}
	rect := geometry.Rect(0, 0, b.Rect().Width, b.Rect().Height)
	part, indicator := "", b.appearance == CheckAppearanceIndicator
	if indicator {
		inner := rect.Inset(b.padding)
		size := min(checkIndicatorSize, inner.Width, inner.Height)
		rect = geometry.Rect(inner.X, inner.Y+(inner.Height-size)/2, size, size)
		part = "indicator"
		if b.group != nil {
			part = "radio"
		}
	} else {
		part = "button"
	}
	if b.state == CheckChecked {
		part += "-checked"
	} else if b.state == CheckMixed {
		part += "-mixed"
	}
	s := gui.ResolveStyle(name, part, state)
	if indicator && b.group != nil {
		// Radio geometry is circular regardless of a square-indicator radius.
		if bg, ok := s.BackgroundColor(); ok && bg != nil {
			p.FillEllipse(rect.Center(), rect.Width/2, rect.Height/2, graphics.ColorOf(bg))
		}
		width, _ := s.BorderWidth()
		if border, ok := s.BorderColor(); ok && border != nil && width > 0 {
			p.DrawEllipse(rect.Center(), max(0, (rect.Width-width)/2), max(0, (rect.Height-width)/2), width, graphics.ColorOf(border))
		}
	} else {
		paintStyledBox(p, rect, name, part, state)
	}
	if fg, ok := s.ForegroundColor(); indicator && ok && fg != nil && rect.Width > 0 {
		brush, size := graphics.ColorOf(fg), rect.Width
		point := func(x, y float32) geometry.Point { return geometry.Point{X: rect.X + size*x, Y: rect.Y + size*y} }
		if b.state == CheckMixed {
			p.FillRoundRect(geometry.Rect(rect.X+size*.22, rect.Y+size*.4375, size*.56, size*.125), size*.0625, brush)
		} else if b.state == CheckChecked {
			if b.group != nil {
				p.FillEllipse(rect.Center(), size*2/9, size*2/9, brush)
			} else {
				a, c, d := point(.22, .50), point(.43, .70), point(.78, .29)
				p.DrawPath(graphics.MoveTo(a.X, a.Y).LineTo(c.X, c.Y).LineTo(d.X, d.Y), size*.12, brush)
			}
		}
	}
	if b.FocusVisible() {
		paintStyledBorder(p, geometry.Rect(0, 0, b.Rect().Width, b.Rect().Height), gui.ResolveStyle(name, "focus", style.FocusVisible))
	}
}

// The manager lets WidgetBase enforce min/max and parent hard constraints.
// Measure and Arrange use the same reserved indicator/gap and centered baseline.
type checkLayout struct{ button *CheckButton }

func (l checkLayout) content(children []layout.Child, inner geometry.Size) (layout.Measurement, float32) {
	reserve := float32(0)
	if l.button.appearance == CheckAppearanceIndicator {
		reserve = checkIndicatorSize
		if len(children) > 0 {
			reserve += 8
		}
	}
	reserve = min(reserve, inner.Width)
	var child layout.Measurement
	if len(children) > 0 {
		width := inner.Width
		if width < layout.Inf {
			width = max(0, width-reserve)
		}
		child = children[0].Measure(layout.Loose(geometry.Size{Width: width, Height: inner.Height}))
	}
	return child, reserve
}

func (l checkLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	b := l.button
	child, reserve := l.content(children, c.Inset(b.padding).Max)
	height := max(checkIndicatorSize, child.Height)
	width := child.Width + reserve
	if b.appearance == CheckAppearanceButton {
		name := b.StyleName()
		if name == "" {
			name = "check-button"
		}
		fontSize, _ := gui.ResolveStyle(name, "button", style.Normal).FontSize()
		// Match Button's font-derived minimum instead of a fixed 20 DIP floor.
		floor := fontSize * 96 / 72
		width, height = max(width, floor), max(child.Height, floor)
	}
	size := c.Clamp(geometry.Size{Width: width + 2*b.padding, Height: height + 2*b.padding})
	inner := geometry.Rect(0, 0, size.Width, size.Height).Inset(b.padding)
	return layout.Measurement{Size: size, HasBaseline: child.HasBaseline, Baseline: inner.Y + (inner.Height-child.Height)/2 + child.Baseline}
}

func (l checkLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	if len(children) == 0 {
		return
	}
	inner := rect.Inset(l.button.padding)
	child, reserve := l.content(children, inner.Size)
	x := inner.X + reserve
	if l.button.appearance == CheckAppearanceButton {
		x = inner.X + (inner.Width-child.Width)/2
	}
	children[0].Arrange(geometry.Rect(x, inner.Y+(inner.Height-child.Height)/2, child.Width, child.Height))
}

var _ gui.Bin = (*CheckButton)(nil)
