package ui

import (
	"github.com/golang-gui/goui/core/bits"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

type TextInputView struct {
	ViewBase[TextInputView]
	text        string
	onText      func(string)
	onSelection func(TextSelection)
	onSubmit    func()
	selection   TextSelection
	readOnly    bool
	padding     float32
	fields      bits.Bitmap[uint64]
}

const (
	textInputText = iota
	textInputSelection
	textInputPadding
)

func TextInput() *TextInputView {
	v := &TextInputView{}
	v.Self = v
	return v
}

// BindText is Text(state.Get()).OnText(state.Set); nil is a no-op.
// A subsequent OnText replaces the writeback callback. Custom BindState
// implementations must arrange UI updates; ui.State already does so.
func (v *TextInputView) BindText(state BindState[string]) *TextInputView {
	if state != nil {
		v.Text(state.Get())
		v.OnText(state.Set)
	}
	return v
}

// BindSelection is Selection(state.Get()).OnSelection(state.Set); nil is a
// no-op. A subsequent OnSelection replaces the writeback callback. Text and
// selection notifications are independent, not an atomic pair.
func (v *TextInputView) BindSelection(state BindState[TextSelection]) *TextInputView {
	if state != nil {
		v.Selection(state.Get()).OnSelection(state.Set)
	}
	return v
}

func (v *TextInputView) Text(text string) *TextInputView {
	v.text = text
	v.fields.Set(textInputText, true)
	return v
}

// Selection controls UTF-8 byte endpoints. Without this modifier the editor
// owns its selection; rebuilding alone does not move the caret.
func (v *TextInputView) Selection(value TextSelection) *TextInputView {
	v.selection = value
	v.fields.Set(textInputSelection, true)
	return v
}
func (v *TextInputView) ReadOnly(value bool) *TextInputView { v.readOnly = value; return v }
func (v *TextInputView) OnSelection(fn func(TextSelection)) *TextInputView {
	v.onSelection = fn
	return v
}
func (v *TextInputView) OnSubmit(fn func()) *TextInputView { v.onSubmit = fn; return v }

func (v *TextInputView) OnText(fn func(string)) *TextInputView {
	v.onText = fn
	return v
}

// Padding sets the text input's inner padding. Unset leaves the built-in
// default (4) untouched.
func (v *TextInputView) Padding(padding float32) *TextInputView {
	v.padding = padding
	v.fields.Set(textInputPadding, true)
	return v
}

func (v *TextInputView) Build() View {
	return v
}

type textInputState struct {
	onText      func(string)
	onSelection func(TextSelection)
	onSubmit    func()
	handles     signal.Handles
	initPadding float32
}

func (v *TextInputView) Mount(ctx BuildContext) gui.Widget {
	input := gui.NewTextInput()
	state := &textInputState{initPadding: input.Padding()}
	state.handles = signal.Handles{input.ConnectText(func(text string) {
		if state.onText != nil {
			state.onText(text)
		}
	}), input.ConnectSelection(func(selection TextSelection) {
		if state.onSelection != nil {
			state.onSelection(selection)
		}
	}), input.ConnectSubmit(func() {
		if state.onSubmit != nil {
			state.onSubmit()
		}
	})}
	ctx.SetState(state)
	return input
}

func (v *TextInputView) Update(ctx BuildContext, widget gui.Widget) {
	input := widget.(*gui.TextInput)
	state := ctx.State().(*textInputState)
	state.handles.Block()
	defer state.handles.Unblock()
	if v.fields.Check(textInputPadding) {
		input.SetPadding(v.padding)
	} else {
		input.SetPadding(state.initPadding)
	}
	if v.fields.Check(textInputText) {
		input.SetText(v.text)
	}
	input.SetReadOnly(v.readOnly)
	if v.fields.Check(textInputSelection) && input.Selection() != v.selection {
		input.SetSelection(v.selection)
	}
	state.onText = v.onText
	state.onSelection, state.onSubmit = v.onSelection, v.onSubmit
}

func (v *TextInputView) Unmount(ctx BuildContext, _ gui.Widget) {
	state, _ := ctx.State().(*textInputState)
	if state != nil {
		state.handles.Disconnect()
	}
}
