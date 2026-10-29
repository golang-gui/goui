package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

type Button struct {
	WidgetBase
	content Widget          // single child (0 or 1); Button is not a Container
	padding geometry.Insets // self-held: button overrides Measure/Arrange (skeleton floor)
	clicked signal.Signal1[uint64]
	motion  *MotionEventController
	click   *ClickEventController
}

const defaultButtonPadding = 6

// NewButton creates a focusable button with centered content and 6 DIP padding on every edge.
// A custom LayoutManager can override the content policy, for example
// layout.NewFillLayout to stretch the child across the content area.
func NewButton() *Button {
	button := new(Button)
	button.SetFocusable(true)
	button.ConnectFocused(func(bool) { button.RequestPaint() })
	button.padding = geometry.UniformInsets(defaultButtonPadding)
	button.SetLayoutManager(buttonContentLayout{})

	button.motion = NewMotionEventController()
	button.AddEventController(button.motion)

	button.click = NewClickEventController()
	button.click.ConnectPressed(func(_ EventContext, pressed bool) {
		button.setPressed(pressed)
	})
	button.click.ConnectClicked(func(_ EventContext) {
		button.emitClicked()
	})
	button.AddEventController(button.click)
	addButtonKeys(button, button.emitClicked)

	return button
}

// Match the activation policy of CheckButton and Switch: unmodified Space or
// Enter activates once on key-down. Only the focused button owns this binding;
// an editable child must retain its own keyboard input.
func addButtonKeys(button Widget, activate func()) {
	keys := NewKeyEventController()
	keys.ConnectKeyDown(func(ctx EventContext, event events.KeyEvent) {
		if !button.Focused() || button.base().destroyed || !IsEnabled(button) {
			return
		}
		if !(KeyGesture{Key: KeySpace}).Matches(event) && !(KeyGesture{Key: KeyEnter}).Matches(event) {
			return
		}
		ctx.StopPropagation()
		event.PreventDefault()
		if !event.Repeat {
			activate()
		}
	})
	button.AddEventController(keys)
}

// SetChild sets the button's single child, replacing any previous one.
// The default layout centers the child inside the padding.
func (b *Button) SetChild(child Widget) {
	if b.content == child {
		return
	}
	if b.content != nil {
		b.WidgetBase.RemoveChild(b.content)
	}
	b.content = child
	if child != nil {
		b.WidgetBase.AddChild(b, child)
	}
	b.RequestLayout()
}

// Child returns the button's single child, or nil.
func (b *Button) Child() Widget {
	return b.content
}

func (b *Button) Padding() geometry.Insets { return b.padding }

// SetPadding sets the per-edge inner padding in DIP. Negative and non-finite
// edges become 0.
func (b *Button) SetPadding(padding geometry.Insets) {
	padding = normalizeInsets(padding)
	if b.padding == padding {
		return
	}
	b.padding = padding
	b.RequestLayout()
}

func (b *Button) Measure(c layout.Constraint) layout.Measurement {
	if !b.Visible() {
		return layout.Measurement{}
	}
	// A button keeps a font-derived skeleton (one line-height square) so an empty
	// or icon-only button stays a visible, clickable box instead of collapsing to
	// zero. This is the widget's own intrinsic floor, not a user override; a
	// user's SetMin/MaxSize and the parent's hard limits still bound this floor.
	fontSize, _ := b.resolvedStyle().FontSize()
	floor := textLineHeight(fontSize) + b.padding.Vertical()
	return measureButtonContent(&b.WidgetBase, c, b.padding, floor)
}

// measureButtonContent lets Button and MenuButton measure their content with
// the same constraints used for placement. The optional skeleton floor is
// clamped by user preferences and parent limits before reaching the manager.
func measureButtonContent(w *WidgetBase, c layout.Constraint, padding geometry.Insets, floor float32) layout.Measurement {
	c = w.layoutConstraint(c)
	c.Min = c.Clamp(geometry.Size{Width: floor, Height: floor})
	var measured layout.Measurement
	if manager := w.LayoutManager(); manager != nil {
		measured = manager.Measure(w.visibleChildren(), c.Insets(padding))
	}
	measured.Size = c.Clamp(measured.Size.Insets(-padding.Left, -padding.Top, -padding.Right, -padding.Bottom))
	if measured.HasBaseline {
		// Rectangle.Insets also handles a parent allocation smaller than the
		// summed horizontal or vertical padding.
		inner := geometry.Rect(0, 0, measured.Width, measured.Height).
			Insets(padding.Left, padding.Top, padding.Right, padding.Bottom)
		measured.Baseline += inner.Y
	}
	return measured
}

// buttonContentLayout centers a Bin's single content without linearly packing
// it. Both axes remain bounded: a button's maximum width must still reach
// wrapping content, independently of LinearBox's natural main-axis basis.
type buttonContentLayout struct{}

func (buttonContentLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	var measured layout.Measurement
	if len(children) > 0 {
		measured = children[0].Measure(layout.Loose(c.Max)).Constrain(layout.Loose(c.Max))
	}
	size := c.Clamp(measured.Size)
	if measured.HasBaseline {
		measured.Baseline += (size.Height - measured.Height) / 2
	}
	measured.Size = size
	return measured
}

func (buttonContentLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	if len(children) == 0 {
		return
	}
	size := children[0].Measure(layout.Loose(rect.Size)).Constrain(layout.Loose(rect.Size)).Size
	children[0].Arrange(geometry.Rect(
		rect.X+(rect.Width-size.Width)/2, rect.Y+(rect.Height-size.Height)/2,
		size.Width, size.Height,
	))
}

func (b *Button) Arrange(rect geometry.Rectangle) {
	b.rect = rect
	manager := b.LayoutManager()
	if manager == nil {
		return
	}
	manager.Arrange(b.visibleChildren(), geometry.Rect(0, 0, rect.Width, rect.Height).
		Insets(b.padding.Left, b.padding.Top, b.padding.Right, b.padding.Bottom))
}

func (b *Button) Paint(p Painter) {
	if !b.Visible() {
		return
	}
	rect := geometry.Rect(0, 0, b.Rect().Width, b.Rect().Height)
	PaintStyledBox(p, rect, b.resolvedStyle())
	paintButtonFocus(p, rect, &b.WidgetBase)
}

func (b *Button) Snapshot() WidgetInfo {
	info := b.WidgetBase.Snapshot()
	info.Role = RoleButton
	if info.Enabled {
		info.Actions = append(info.Actions, ActionClick)
	}
	return info
}

func (b *Button) ConnectClicked(fn func()) signal.Handle {
	return b.clicked.Connect(func(revision uint64) {
		if !b.Destroyed() && IsEnabled(b) && b.enabledRevision == revision {
			fn()
		}
	})
}

func (b *Button) emitClicked() {
	if !b.Destroyed() && IsEnabled(b) {
		b.clicked.Emit(b.enabledRevision)
	}
}

func (b *Button) setPressed(value bool) { b.SetState(style.Pressed, value) }

func (b *Button) resolvedStyle() style.Style {
	return ResolveWidgetStyle(b, buttonStyleName(&b.WidgetBase), style.PartDefault, b.State())
}
