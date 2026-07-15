package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
)

type Button struct {
	WidgetBase
	padding float32 // self-held: button overrides Measure/Arrange (skeleton floor)
	hovered bool
	pressed bool
	clicked signal.Signal0
	motion  *MotionEventController
	click   *ClickEventController
}

const defaultButtonPadding = 6

func NewButton() *Button {
	button := new(Button)
	button.SetFocusable(true)
	button.padding = defaultButtonPadding
	button.SetLayoutManager(layout.NewFillLayout())

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

func (b *Button) AddChild(child Widget) {
	b.WidgetBase.AddChild(b, child)
}

func (b *Button) Padding() float32 { return b.padding }

func (b *Button) SetPadding(padding float32) {
	if b.padding == padding {
		return
	}
	b.padding = padding
	b.RequestLayout()
}

func (b *Button) Measure(c layout.Constraint) geometry.Size {
	if !b.Visible() {
		return geometry.Size{}
	}
	padding := b.padding

	var content geometry.Size
	if manager := b.LayoutManager(); manager != nil {
		content = manager.Measure(b.visibleChildren(), layout.Loose(c.Max.Inset(padding))).Inset(-padding)
	}

	// A button keeps a font-derived skeleton (one line-height square) so an empty
	// or icon-only button stays a visible, clickable box instead of collapsing to
	// zero. This is the widget's own intrinsic floor, not a user override; a
	// user's SetMin/MaxSize still wins because constrain runs last (DesignLayout).
	fontSize, _ := b.resolvedStyle().FontSize()
	floor := textLineHeight(fontSize) + padding*2
	content.Width = max(content.Width, floor)
	content.Height = max(content.Height, floor)
	return b.constrain(c, content)
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
	b.PaintChildren(p)
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
