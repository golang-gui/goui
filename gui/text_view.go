package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/layout"
)

// TextModel is a shareable document. Its implementation lives in textedit.
type TextModel = textedit.Model

// TextRange is a half-open range of UTF-8 byte offsets.
type TextRange = textedit.Range

// TextSelection preserves anchor and caret direction.
type TextSelection = textedit.Selection

// TextEdit describes one document replacement.
type TextEdit = textedit.Edit

// TextChange reports one committed model revision.
type TextChange = textedit.Change

// NewTextModel creates a document with normalized UTF-8 text and LF line breaks.
func NewTextModel(text string) *TextModel { return textedit.NewModel(text) }

// TextView is a plain multiline editor. Model and history can be shared;
// selection, input state, scrolling and native layouts belong to each view.
// Place it in a ScrollView for scrolling and scrollbars.
type TextView struct {
	WidgetBase
	*textEditor
}

func NewTextView() *TextView {
	t := &TextView{}
	t.textEditor = newTextEditor(t, false)
	return t
}

func (t *TextView) Model() *TextModel { return t.model }

// SetModel resets selection and scrolling, leaving the old model/history
// intact. nil creates an empty model; passing the current model is a no-op.
func (t *TextView) SetModel(model *TextModel) { t.setModel(model) }
func (t *TextView) Selection() TextSelection  { return t.selection }

// SetSelection clamps UTF-8 byte offsets to the document and whole Cluster
// boundaries when typography is available, preserving selection direction.
func (t *TextView) SetSelection(s TextSelection) { t.setSelectionValue(s) }
func (t *TextView) ReadOnly() bool               { return t.readOnly }
func (t *TextView) SetReadOnly(value bool)       { t.setReadOnly(value) }
func (t *TextView) AcceptsTab() bool             { return t.acceptsTab }
func (t *TextView) SetAcceptsTab(value bool)     { t.setAcceptsTab(value) }
func (t *TextView) WrapMode() WrapMode           { return t.wrap }
func (t *TextView) SetWrapMode(value WrapMode)   { t.setWrapMode(value) }
func (t *TextView) Padding() float32             { return t.padding }
func (t *TextView) SetPadding(value float32)     { t.setPadding(value) }
func (t *TextView) ConnectChange(fn func(TextChange)) signal.Handle {
	return t.connectChange(fn)
}
func (t *TextView) ConnectSelection(fn func(TextSelection)) signal.Handle {
	return t.connectSelection(fn)
}
func (t *TextView) ConnectScrollIntoView(fn func(geometry.Rectangle)) signal.Handle {
	return t.connectScrollIntoView(fn)
}
func (t *TextView) IMContext() IMContext                           { return t.imContext() }
func (t *TextView) StyleChanged()                                  { t.styleChanged() }
func (t *TextView) ContentSize() geometry.Size                     { return t.contentSize() }
func (t *TextView) Measure(c layout.Constraint) layout.Measurement { return t.measure(c) }
func (t *TextView) Arrange(rect geometry.Rectangle)                { t.arrange(rect) }
func (t *TextView) LayoutVisible(viewport geometry.Size, offset geometry.Point) {
	t.layoutVisible(viewport, offset)
}
func (t *TextView) Paint(p Painter)      { t.paint(p) }
func (t *TextView) Snapshot() WidgetInfo { return t.snapshot() }
