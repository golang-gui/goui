package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
)

type Button struct {
	WidgetBase
	content Widget  // single child (0 or 1); Button is not a Container
	padding float32 // self-held: button overrides Measure/Arrange (skeleton floor)
	hovered bool
	pressed bool
	clicked signal.Signal0
	motion  *MotionEventController
	click   *ClickEventController
}

const defaultButtonPadding = 6

// NewButton creates a focusable button with centered content and 6 DIP padding.
// A custom LayoutManager can override the content policy, for example
// layout.NewFillLayout to stretch the child across the content area.
func NewButton() *Button {
	button := new(Button)
	button.SetFocusable(true)
	button.padding = defaultButtonPadding
	button.SetLayoutManager(&layout.LinearLayout{
		Direction:  layout.DirectionHorizontal,
		MainAlign:  layout.MainCenter,
		CrossAlign: layout.CrossCenter,
	})

	button.motion = NewMotionEventController()
	button.motion.ConnectContainsHover(button.setHovered)
	button.AddEventController(button.motion)

	button.click = NewClickEventController()
	button.click.ConnectPressed(func(ctx EventContext, pressed bool) {
		button.setPressed(pressed)
		ctx.StopPropagation()
	})
	button.click.ConnectClicked(func(ctx EventContext) {
		button.emitClicked()
		ctx.StopPropagation()
	})
	button.AddEventController(button.click)

	return button
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

func (b *Button) Padding() float32 { return b.padding }

// SetPadding sets the inner padding. Negative and non-finite values become 0.
func (b *Button) SetPadding(padding float32) {
	padding = normalizeLayoutValue(padding)
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
	floor := textLineHeight(fontSize) + b.padding*2
	return measureButtonContent(&b.WidgetBase, c, b.padding, floor)
}

// measureButtonContent lets Button and MenuButton measure their content with
// the same constraints used for placement. The optional skeleton floor is
// clamped by user preferences and parent limits before reaching the manager.
func measureButtonContent(w *WidgetBase, c layout.Constraint, padding, floor float32) layout.Measurement {
	c = w.layoutConstraint(c)
	c.Min = c.Clamp(geometry.Size{Width: floor, Height: floor})
	var measured layout.Measurement
	if manager := w.LayoutManager(); manager != nil {
		measured = manager.Measure(w.visibleChildren(), c.Inset(padding))
	}
	measured.Size = c.Clamp(measured.Size.Inset(-padding))
	if measured.HasBaseline {
		// Rectangle.Inset also handles a parent allocation smaller than 2*padding.
		inner := geometry.Rect(0, 0, measured.Width, measured.Height).Inset(padding)
		measured.Baseline += inner.Y
	}
	return measured
}

func (b *Button) Arrange(rect geometry.Rectangle) {
	b.rect = rect
	manager := b.LayoutManager()
	if manager == nil {
		return
	}
	manager.Arrange(b.visibleChildren(), geometry.Rect(0, 0, rect.Width, rect.Height).Inset(b.padding))
}

func (b *Button) Paint(p Painter) {
	if !b.Visible() {
		return
	}
	rect := geometry.Rect(0, 0, b.Rect().Width, b.Rect().Height)
	paintStyledBox(p, rect, b.resolvedStyle())
}

func (b *Button) Snapshot() WidgetInfo {
	info := b.WidgetBase.Snapshot()
	info.Role = RoleButton
	info.Actions = append(info.Actions, ActionClick)
	return info
}

func (b *Button) ConnectClicked(fn func()) signal.Handle {
	return b.clicked.Connect(fn)
}

func (b *Button) emitClicked() {
	b.clicked.Emit()
}

func (b *Button) setHovered(hovered bool) {
	if b.hovered == hovered {
		return
	}
	b.hovered = hovered
	if !hovered && b.pressed {
		b.setPressed(false)
		return
	}
	b.requestPaint()
}

func (b *Button) setPressed(pressed bool) {
	if b.pressed == pressed {
		return
	}
	b.pressed = pressed
	b.requestPaint()
}

func (b *Button) requestPaint() {
	// Root is the widget host (a window or a popover); Window() would be nil for a
	// widget hosted in a popover, so a repaint request would be dropped.
	if r := b.Root(); r != nil {
		_ = r.RequestPaint()
	}
}

func (b *Button) resolvedStyle() style.Style {
	name := b.StyleName()
	if name == "" {
		name = styleNameButton
	}
	return ResolveStyle(name, style.PartDefault, b.styleState())
}

func (b *Button) styleState() style.State {
	if b.pressed {
		return style.Pressed
	}
	if b.hovered {
		return style.Hovered
	}
	return style.Normal
}
