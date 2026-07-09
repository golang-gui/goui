package ui

import (
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

type ButtonView struct {
	ViewBase[ButtonView]
	child   View
	onClick func()
}

type buttonState struct {
	onClick func()
	click   signal.Handle
}

func Button(text ...string) *ButtonView {
	if len(text) > 1 {
		panic("ui: Button accepts at most one text argument")
	}
	v := &ButtonView{}
	v.Self = v
	if len(text) == 1 {
		v.child = Label(text[0])
	}
	return v
}

func (v *ButtonView) Text(text string) *ButtonView {
	return v.Content(Label(text))
}

func (v *ButtonView) Content(content View) *ButtonView {
	v.child = content
	return v
}

func (v *ButtonView) Child(child View) *ButtonView {
	return v.Content(child)
}

func (v *ButtonView) OnClick(fn func()) *ButtonView {
	v.onClick = fn
	return v
}

func (v *ButtonView) Build() View {
	return v
}

func (v *ButtonView) Mount(ctx BuildContext) gui.Widget {
	button := gui.NewButton()
	state := &buttonState{}
	state.click = button.ConnectClicked(func() {
		if state.onClick != nil {
			state.onClick()
		}
	})
	ctx.SetState(state)
	return button
}

func (v *ButtonView) Update(ctx BuildContext, widget gui.Widget) {
	button := widget.(*gui.Button)
	state := ctx.State().(*buttonState)
	state.onClick = v.onClick
	if v.child == nil {
		ctx.UpdateChildren(button, nil)
	} else {
		ctx.UpdateChildren(button, []View{v.child})
	}
}

func (v *ButtonView) Unmount(ctx BuildContext, _ gui.Widget) {
	state, _ := ctx.State().(*buttonState)
	if state != nil && state.click != nil {
		state.click.Disconnect()
	}
}
