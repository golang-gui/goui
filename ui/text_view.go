package ui

import (
	"github.com/golang-gui/goui/core/bits"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

// The UI layer uses GUI's document and UTF-8 editing values directly.
type TextModel = gui.TextModel
type TextRange = gui.TextRange
type TextEdit = gui.TextEdit
type TextChange = gui.TextChange
type TextSelection = gui.TextSelection
type WrapMode = gui.WrapMode

const (
	WrapNone     = gui.WrapNone
	WrapChar     = gui.WrapChar
	WrapWordChar = gui.WrapWordChar
)

func NewTextModel(text string) *TextModel { return gui.NewTextModel(text) }

type TextViewView struct {
	ViewBase[TextViewView]
	model           *TextModel
	text            string
	textSource      BindState[string]
	selectionSource BindState[TextSelection]
	placeholder     string
	selection       TextSelection
	padding         float32
	wrap            WrapMode
	readOnly        bool
	acceptsTab      bool
	onChange        func(TextChange)
	onText          func(string)
	onSelection     func(TextSelection)
	fields          bits.Bitmap[uint64]
}

const (
	textViewText = iota
	textViewSelection
	textViewPadding
	textViewWrap
	textViewWriteText
	textViewWriteSelection
)

// TextView declares a multiline editor. Compose with ScrollView for scrolling;
// create shared models outside Build so rebuilding does not reload a document.
func TextView() *TextViewView {
	v := &TextViewView{}
	v.Self = v
	return v
}

// Model selects a shared document. nil/unset restores the widget's original
// private model, which is retained across declarative rebuilds.
// A non-nil model cannot be combined with Text or BindText.
func (v *TextViewView) Model(model *TextModel) *TextViewView { v.model = model; return v }

// Text declares the private document's content. Unset preserves its content;
// an explicit empty string clears it. Different normalized text reloads the
// document, clearing history and resetting selection, scrolling and IME.
// Equal text is a complete no-op. Selection, when declared, is applied next.
func (v *TextViewView) Text(text string) *TextViewView {
	v.text = text
	v.textSource = nil
	if v.fields.Check(textViewWriteText) {
		v.onText = nil
		v.fields.Set(textViewWriteText, false)
	}
	v.fields.Set(textViewText, true)
	return v
}

// Placeholder sets the empty-document hint. Omitted restores an empty hint.
func (v *TextViewView) Placeholder(text string) *TextViewView {
	v.placeholder = text
	return v
}

// OnText receives the current committed document text, excluding IME preedit.
// Declarative updates do not invoke it. Use OnChange for incremental updates
// without reading the whole document; reentrant edits may advance the model
// beyond the revision of the notification that triggered this callback.
func (v *TextViewView) OnText(fn func(string)) *TextViewView {
	v.onText = fn
	v.fields.Set(textViewWriteText, false)
	return v
}

// BindText reads current values on the UI thread and writes user edits back.
// A subsequent OnText replaces writeback; Text removes the read binding and
// its generated writeback. nil is a no-op.
func (v *TextViewView) BindText(source BindState[string]) *TextViewView {
	if source != nil {
		validateObservable(source)
		v.textSource = source
		v.fields.Set(textViewText, true)
		v.onText = source.Set
		v.fields.Set(textViewWriteText, true)
	}
	return v
}

// BindSelection follows selection changes independently of text. A subsequent
// OnSelection replaces writeback; Selection removes the binding. nil is a no-op.
func (v *TextViewView) BindSelection(source BindState[TextSelection]) *TextViewView {
	if source != nil {
		validateObservable(source)
		v.selectionSource = source
		v.fields.Set(textViewSelection, true)
		v.onSelection = source.Set
		v.fields.Set(textViewWriteSelection, true)
	}
	return v
}

// Selection declares UTF-8 byte endpoints in normalized text. GUI corrects
// them to valid document/Cluster boundaries without writing back during Update.
// Unset leaves the selection under the editor's control.
func (v *TextViewView) Selection(value TextSelection) *TextViewView {
	v.selection = value
	v.selectionSource = nil
	if v.fields.Check(textViewWriteSelection) {
		v.onSelection = nil
		v.fields.Set(textViewWriteSelection, false)
	}
	v.fields.Set(textViewSelection, true)
	return v
}

func (v *TextViewView) OnSelection(fn func(TextSelection)) *TextViewView {
	v.onSelection = fn
	v.fields.Set(textViewWriteSelection, false)
	return v
}

func (v *TextViewView) Padding(value float32) *TextViewView {
	v.padding = value
	v.fields.Set(textViewPadding, true)
	return v
}
func (v *TextViewView) WrapMode(value WrapMode) *TextViewView {
	v.wrap = value
	v.fields.Set(textViewWrap, true)
	return v
}
func (v *TextViewView) ReadOnly(value bool) *TextViewView          { v.readOnly = value; return v }
func (v *TextViewView) AcceptsTab(value bool) *TextViewView        { v.acceptsTab = value; return v }
func (v *TextViewView) OnChange(fn func(TextChange)) *TextViewView { v.onChange = fn; return v }

type textViewState struct {
	model       *TextModel
	padding     float32
	wrap        WrapMode
	onChange    func(TextChange)
	onText      func(string)
	onSelection func(TextSelection)
	handles     signal.Handles
}

func (v *TextViewView) Mount(ctx BuildContext) gui.Widget {
	v.validateContent()
	w := gui.NewTextView()
	s := &textViewState{model: w.Model(), padding: w.Padding(), wrap: w.WrapMode()}
	s.handles = signal.Handles{
		// Separate slots let unmount inside OnText disconnect OnChange before
		// it runs; neither callback needs to access the widget after user code.
		w.ConnectChange(func(TextChange) {
			if s.onText != nil {
				s.onText(w.Model().Text())
			}
		}),
		w.ConnectChange(func(change TextChange) {
			if s.onChange != nil {
				s.onChange(change)
			}
		}),
		w.ConnectSelection(func(selection TextSelection) {
			if s.onSelection != nil {
				s.onSelection(selection)
			}
		}),
	}
	ctx.SetState(s)
	return w
}

func (v *TextViewView) Update(ctx BuildContext, widget gui.Widget) {
	v.validateContent()
	w, s := widget.(*gui.TextView), ctx.State().(*textViewState)
	s.handles.Block()
	defer s.handles.Unblock()
	model := v.model
	if model == nil {
		model = s.model
	}
	w.SetModel(model)
	w.SetPlaceholder(v.placeholder)
	if v.textSource != nil {
		BindValue(ctx, "text", v.textSource, func(text string) {
			s.handles.Block()
			defer s.handles.Unblock()
			w.Model().SetText(text)
		})
	} else if v.fields.Check(textViewText) {
		w.Model().SetText(v.text)
	}
	if v.fields.Check(textViewPadding) {
		w.SetPadding(v.padding)
	} else {
		w.SetPadding(s.padding)
	}
	if v.fields.Check(textViewWrap) {
		w.SetWrapMode(v.wrap)
	} else {
		w.SetWrapMode(s.wrap)
	}
	w.SetReadOnly(v.readOnly)
	w.SetAcceptsTab(v.acceptsTab)
	if v.selectionSource != nil {
		BindValue(ctx, "selection", v.selectionSource, func(selection TextSelection) {
			s.handles.Block()
			defer s.handles.Unblock()
			if w.Selection() != selection {
				w.SetSelection(selection)
			}
		})
	} else if v.fields.Check(textViewSelection) && w.Selection() != v.selection {
		w.SetSelection(v.selection)
	}
	s.onChange, s.onSelection = v.onChange, v.onSelection
	s.onText = v.onText
}

func (v *TextViewView) validateContent() {
	if v.model != nil && v.fields.Check(textViewText) {
		panic("ui: TextView Model and Text/BindText are mutually exclusive")
	}
}

func (v *TextViewView) Unmount(ctx BuildContext, _ gui.Widget) {
	if s, ok := ctx.State().(*textViewState); ok {
		s.handles.Disconnect()
	}
}

var _ WidgetView = (*TextViewView)(nil)
