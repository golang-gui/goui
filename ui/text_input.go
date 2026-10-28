package ui

import (
	"github.com/golang-gui/goui/core/bits"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

type TextInputView struct {
	ViewBase[TextInputView]
	text            string
	textSource      BindState[string]
	selectionSource BindState[TextSelection]
	placeholder     string
	onText          func(string)
	onSelection     func(TextSelection)
	onSubmit        func()
	selection       TextSelection
	readOnly        bool
	padding         float32
	fields          bits.Bitmap[uint64]
}

const (
	textInputText = iota
	textInputSelection
	textInputPadding
	textInputWriteText
	textInputWriteSelection
)

func TextInput() *TextInputView {
	v := &TextInputView{}
	v.Self = v
	return v
}

// BindText reads current values on the UI thread and writes user edits back.
// A subsequent OnText replaces writeback; Text removes the read binding and
// its generated writeback. nil is a no-op.
func (v *TextInputView) BindText(source BindState[string]) *TextInputView {
	if source != nil {
		validateObservable(source)
		v.textSource = source
		v.fields.Set(textInputText, true)
		v.onText = source.Set
		v.fields.Set(textInputWriteText, true)
	}
	return v
}

// BindSelection follows selection changes independently of text. A subsequent
// OnSelection replaces writeback; Selection removes the binding. nil is a no-op.
func (v *TextInputView) BindSelection(source BindState[TextSelection]) *TextInputView {
	if source != nil {
		validateObservable(source)
		v.selectionSource = source
		v.fields.Set(textInputSelection, true)
		v.onSelection = source.Set
		v.fields.Set(textInputWriteSelection, true)
	}
	return v
}

func (v *TextInputView) Text(text string) *TextInputView {
	v.text = text
	v.textSource = nil
	if v.fields.Check(textInputWriteText) {
		v.onText = nil
		v.fields.Set(textInputWriteText, false)
	}
	v.fields.Set(textInputText, true)
	return v
}

// Placeholder sets the empty-field hint. Omitted restores an empty hint.
func (v *TextInputView) Placeholder(text string) *TextInputView {
	v.placeholder = text
	return v
}

// Selection controls UTF-8 byte endpoints. Without this modifier the editor
// owns its selection; rebuilding alone does not move the caret.
func (v *TextInputView) Selection(value TextSelection) *TextInputView {
	v.selection = value
	v.selectionSource = nil
	if v.fields.Check(textInputWriteSelection) {
		v.onSelection = nil
		v.fields.Set(textInputWriteSelection, false)
	}
	v.fields.Set(textInputSelection, true)
	return v
}
func (v *TextInputView) ReadOnly(value bool) *TextInputView { v.readOnly = value; return v }
func (v *TextInputView) OnSelection(fn func(TextSelection)) *TextInputView {
	v.onSelection = fn
	v.fields.Set(textInputWriteSelection, false)
	return v
}
func (v *TextInputView) OnSubmit(fn func()) *TextInputView { v.onSubmit = fn; return v }

func (v *TextInputView) OnText(fn func(string)) *TextInputView {
	v.onText = fn
	v.fields.Set(textInputWriteText, false)
	return v
}

// Padding sets the text input's inner padding. Unset leaves the built-in
// default (4) untouched.
func (v *TextInputView) Padding(padding float32) *TextInputView {
	v.padding = padding
	v.fields.Set(textInputPadding, true)
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
	if v.textSource != nil {
		BindValue(ctx, "text", v.textSource, func(text string) {
			state.handles.Block()
			defer state.handles.Unblock()
			input.SetText(text)
		})
	} else if v.fields.Check(textInputText) {
		input.SetText(v.text)
	}
	input.SetReadOnly(v.readOnly)
	input.SetPlaceholder(v.placeholder)
	if v.selectionSource != nil {
		BindValue(ctx, "selection", v.selectionSource, func(selection TextSelection) {
			state.handles.Block()
			defer state.handles.Unblock()
			if input.Selection() != selection {
				input.SetSelection(selection)
			}
		})
	} else if v.fields.Check(textInputSelection) && input.Selection() != v.selection {
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
