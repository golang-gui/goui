package ui

import (
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

type ButtonView struct {
	name    string
	hidden  bool
	child   View
	onClick func()
}

type buttonState struct {
	onClick func()
	click   signal.Handle
}

func Button(child View) ButtonView {
	return ButtonView{child: child}
}

func (v ButtonView) Name(name string) ButtonView {
	v.name = name
	return v
}

func (v ButtonView) Hidden(hidden bool) ButtonView {
	v.hidden = hidden
	return v
}

func (v ButtonView) Child(child View) ButtonView {
	v.child = child
	return v
}

func (v ButtonView) OnClick(fn func()) ButtonView {
	v.onClick = fn
	return v
}

func (v ButtonView) Mount(ctx BuildContext) gui.Widget {
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

func (v ButtonView) Update(ctx BuildContext, widget gui.Widget) {
	button := widget.(*gui.Button)
	state := ctx.State().(*buttonState)
	state.onClick = v.onClick
	button.SetID(v.name)
	button.SetVisible(!v.hidden)
	if v.child == nil {
		ctx.UpdateChildren(button, nil)
	} else {
		ctx.UpdateChildren(button, []View{v.child})
	}
}

func (v ButtonView) Unmount(ctx BuildContext, _ gui.Widget) {
	state, _ := ctx.State().(*buttonState)
	if state != nil && state.click != nil {
		state.click.Disconnect()
	}
}
