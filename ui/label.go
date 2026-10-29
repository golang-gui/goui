package ui

import (
	"github.com/golang-gui/goui/core/bits"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

type LabelView struct {
	ViewBase[LabelView]
	text            string
	textSource      ReadState[string]
	wrapMode        gui.WrapMode
	selectable      bool
	selection       TextSelection
	selectionSource BindState[TextSelection]
	onSelection     func(TextSelection)
	onCopyError     func(error)
	fields          bits.Bitmap[uint8]
}

const (
	labelSelectionValue = iota
	labelWriteSelection
	labelFocusable
	labelCursor
)

// Label declares display text. A leading \{format\} enables named style ranges
// as documented by gui.Label. Omission means empty text; at most one argument
// is allowed.
func Label(text ...string) *LabelView {
	if len(text) > 1 {
		panic("ui: Label accepts at most one text argument")
	}
	v := &LabelView{}
	if len(text) == 1 {
		v.text = text[0]
	}
	v.Self = v
	return v
}

func (v *LabelView) Text(text string) *LabelView {
	v.text = text
	v.textSource = nil
	return v
}

// BindText follows a readable value without rebuilding the containing View.
// A later Text replaces the binding. nil is a no-op.
func (v *LabelView) BindText(source ReadState[string]) *LabelView {
	if source != nil {
		validateObservable(source)
		v.textSource = source
	}
	return v
}

func (v *LabelView) WrapMode(mode gui.WrapMode) *LabelView {
	v.wrapMode = mode
	return v
}

// Selectable enables plain-text selection and copying. It defaults to false.
func (v *LabelView) Selectable(value bool) *LabelView { v.selectable = value; return v }

// Focusable overrides the focus default implied by Selectable.
func (v *LabelView) Focusable(value bool) *LabelView {
	v.fields.Set(labelFocusable, true)
	return v.ViewBase.Focusable(value)
}

// Cursor overrides the text cursor implied by Selectable.
func (v *LabelView) Cursor(value Cursor) *LabelView {
	v.fields.Set(labelCursor, true)
	return v.ViewBase.Cursor(value)
}

// Selection declares PlainText UTF-8 byte endpoints. Omission preserves the
// widget's selection; a later Selection replaces BindSelection and its writeback.
func (v *LabelView) Selection(value TextSelection) *LabelView {
	v.selection, v.selectionSource = value, nil
	if v.fields.Check(labelWriteSelection) {
		v.onSelection = nil
		v.fields.Set(labelWriteSelection, false)
	}
	v.fields.Set(labelSelectionValue, true)
	return v
}

// BindSelection follows and writes selection changes. A later OnSelection
// replaces writeback. nil is a no-op.
func (v *LabelView) BindSelection(source BindState[TextSelection]) *LabelView {
	if source != nil {
		validateObservable(source)
		v.selectionSource, v.onSelection = source, source.Set
		v.fields.Set(labelSelectionValue, true)
		v.fields.Set(labelWriteSelection, true)
	}
	return v
}

// OnSelection observes user selection changes. Declarative writes are suppressed.
func (v *LabelView) OnSelection(fn func(TextSelection)) *LabelView {
	v.onSelection = fn
	v.fields.Set(labelWriteSelection, false)
	return v
}

// OnCopyError receives clipboard errors from built-in keyboard/menu actions.
func (v *LabelView) OnCopyError(fn func(error)) *LabelView { v.onCopyError = fn; return v }

type labelState struct {
	onSelection func(TextSelection)
	onCopyError func(error)
	handles     signal.Handles
}

func (v *LabelView) Mount(ctx BuildContext) gui.Widget {
	ctx.SetState(&labelState{})
	return gui.NewLabel(v.text)
}

func (s *labelState) connect(label *gui.Label) {
	if s.handles != nil {
		return
	}
	s.handles = signal.Handles{
		label.ConnectSelection(func(value TextSelection) {
			if s.onSelection != nil {
				s.onSelection(value)
			}
		}),
		label.ConnectCopyError(func(err error) {
			if s.onCopyError != nil {
				s.onCopyError(err)
			}
		}),
	}
}

func (v *LabelView) Update(ctx BuildContext, widget gui.Widget) {
	label := widget.(*gui.Label)
	state := ctx.State().(*labelState)
	if v.selectable || v.onSelection != nil || v.onCopyError != nil {
		state.connect(label)
	}
	state.handles.Block()
	defer state.handles.Unblock()
	label.SetSelectable(v.selectable)
	// Shared modifiers are applied after Update. Materialize this View's
	// dynamic defaults locally, without changing Root or BuildContext.
	if !v.fields.Check(labelFocusable) {
		v.ViewBase.Focusable(v.selectable)
	}
	if !v.fields.Check(labelCursor) {
		var cursor Cursor
		if v.selectable {
			cursor = gui.CursorText
		}
		v.ViewBase.Cursor(cursor)
	}
	if v.textSource != nil {
		BindValue(ctx, "text", v.textSource, func(value string) {
			state.handles.Block()
			defer state.handles.Unblock()
			label.SetText(value)
		})
	} else {
		label.SetText(v.text)
	}
	label.SetWrapMode(v.wrapMode)
	if v.selectionSource != nil {
		BindValue(ctx, "selection", v.selectionSource, func(value TextSelection) {
			state.handles.Block()
			defer state.handles.Unblock()
			label.SetSelection(value)
		})
	} else if v.fields.Check(labelSelectionValue) {
		label.SetSelection(v.selection)
	}
	state.onSelection, state.onCopyError = v.onSelection, v.onCopyError
}

func (v *LabelView) Unmount(ctx BuildContext, _ gui.Widget) {
	if state, ok := ctx.State().(*labelState); ok {
		state.handles.Disconnect()
	}
}
