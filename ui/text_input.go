package ui

import (
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"
)

type TextInputView struct {
	name   string
	hidden bool
	text   string
	onText func(string)
	rules  []style.Rule
}

func TextInput() TextInputView {
	return TextInputView{}
}

func (v TextInputView) Name(name string) TextInputView {
	v.name = name
	return v
}

func (v TextInputView) Hidden(hidden bool) TextInputView {
	v.hidden = hidden
	return v
}

func (v TextInputView) Text(text string) TextInputView {
	v.text = text
	return v
}

func (v TextInputView) OnText(fn func(string)) TextInputView {
	v.onText = fn
	return v
}

func (v TextInputView) Style(rules ...style.Rule) TextInputView {
	v.rules = rules
	return v
}

func (v TextInputView) Build() View {
	return v
}

type textInputState struct {
	onText func(string)
	text   signal.Handle
}

func (v TextInputView) Mount(ctx BuildContext) gui.Widget {
	input := gui.NewTextInput()
	state := &textInputState{}
	state.text = input.ConnectText(func(text string) {
		if state.onText != nil {
			state.onText(text)
		}
	})
	ctx.SetState(state)
	return input
}

func (v TextInputView) Update(ctx BuildContext, widget gui.Widget) {
	input := widget.(*gui.TextInput)
	state := ctx.State().(*textInputState)
	input.SetID(v.name)
	input.SetVisible(!v.hidden)
	input.SetStyleRules(v.rules...)
	func() {
		state.text.Block()
		defer state.text.Unblock()
		input.SetText(v.text)
	}()
	state.onText = v.onText
}

func (v TextInputView) Unmount(ctx BuildContext, _ gui.Widget) {
	state, _ := ctx.State().(*textInputState)
	if state != nil && state.text != nil {
		state.text.Disconnect()
	}
}
