package gui

import (
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
)

type Button struct {
	WidgetBase
	pressed bool
	clicked signal.Signal0
}

func NewButton() *Button {
	button := new(Button)
	button.Init(button)
	button.SetLayoutManager(layout.NewFillLayout())
	button.AddEventController(&buttonClickController{
		button: button,
		phase:  PhaseTarget,
	})
	button.AddEventController(&buttonClickController{
		button: button,
		phase:  PhaseBubble,
	})
	return button
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

func (b *Button) setPressed(pressed bool) {
	if b.pressed == pressed {
		return
	}
	b.pressed = pressed
	if b.window != nil {
		b.window.requestPaint()
	}
}

type buttonClickController struct {
	button *Button
	phase  PropagationPhase
	widget Widget
}

func (c *buttonClickController) Phase() PropagationPhase {
	return c.phase
}

func (c *buttonClickController) Widget() Widget {
	return c.widget
}

func (c *buttonClickController) SetWidget(widget Widget) {
	c.widget = widget
}

func (c *buttonClickController) HandleEvent(ctx *EventContext, event events.Event) {
	pointerEvent, ok := event.(events.PointerEvent)
	if !ok {
		return
	}
	if pointerEvent.Button != events.PointerButtonLeft {
		return
	}

	switch pointerEvent.EventType {
	case events.PointerDown:
		c.button.setPressed(true)
		ctx.StopPropagation()
	case events.PointerUp:
		if c.button.pressed {
			c.button.setPressed(false)
			c.button.emitClicked()
			ctx.StopPropagation()
		}
	}
}
