package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
)

type Button struct {
	WidgetBase
	hovered bool
	pressed bool
	clicked signal.Signal0
	motion  *MotionEventController
	click   *ClickEventController
}

func NewButton() *Button {
	button := new(Button)
	button.SetFocusable(true)
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

func (b *Button) Paint(p Painter) {
	if !b.Visible() {
		return
	}
	p.FillRoundRect(geometry.Rect(0, 0, b.Rect().Width, b.Rect().Height), 4, b.backgroundColor())
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
	if win := b.Window(); win != nil {
		_ = win.RequestPaint()
	}
}

func (b *Button) backgroundColor() graphics.Color {
	switch {
	case b.pressed:
		return graphics.RGB(180, 180, 180)
	case b.hovered:
		return graphics.RGB(230, 230, 230)
	default:
		return graphics.RGB(210, 210, 210)
	}
}
