package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/layout"
)

const (
	defaultTextInputWidth  = 160
	textInputMeasureExtent = 1 << 20
	// Measure a stable mixed-script line, independent of field contents.
	textInputHeightSample = "Ag中"
)

// TextInput is a single-line editor with its own private document/history.
// It shares editing operations with TextView, not a child Widget or model.
// Secondary click or Shift+F10 opens its standard editing context menu.
type TextInput struct {
	WidgetBase
	*textEditor
	textSignal signal.Signal1[string]
}

func NewTextInput() *TextInput {
	t := &TextInput{}
	t.textEditor = newTextEditor(t, true)
	t.connectChange(func(TextChange) { t.textSignal.Emit(t.Text()) })
	return t
}

func (t *TextInput) Text() string { return t.model.Text() }

// SetText reloads normalized single-line text: CR/CRLF become LF, each LF
// becomes a space, and invalid UTF-8 is replaced. A changed value cancels IME,
// clears history and puts the caret at the end. Equal text is a complete no-op.
func (t *TextInput) SetText(text string) {
	text = textedit.NormalizeSingleLine(text)
	if t.model.EqualText(text) {
		return
	}
	model, epoch := t.model, t.mountEpoch
	t.cancelPreedit(true)
	if t.destroyed || t.model != model || t.mountEpoch != epoch {
		return
	}
	model.SetText(text)
	if t.suspended && t.model == model && t.mountEpoch == epoch {
		// The private model is disconnected on unmount. Explicit setters still
		// update the field and emit notifications while detached.
		t.selection = TextSelection{len(text), len(text)}
		t.seenRevision = model.Revision()
		t.resetParagraphs()
		t.textSignal.Emit(text)
	}
}

func (t *TextInput) ConnectText(fn func(string)) signal.Handle { return t.textSignal.Connect(fn) }

// ConnectSubmit reports an unconsumed Enter key. It does not insert a newline.
func (t *TextInput) ConnectSubmit(fn func()) signal.Handle { return t.submitSignal.Connect(fn) }
func (t *TextInput) Selection() TextSelection              { return t.selection }

// SetSelection uses UTF-8 byte positions, clamped to complete Clusters.
func (t *TextInput) SetSelection(s TextSelection) { t.setSelectionValue(s) }
func (t *TextInput) ConnectSelection(fn func(TextSelection)) signal.Handle {
	return t.connectSelection(fn)
}
func (t *TextInput) ReadOnly() bool         { return t.readOnly }
func (t *TextInput) SetReadOnly(value bool) { t.setReadOnly(value) }
func (t *TextInput) Padding() float32       { return t.padding }

// SetPadding sets the content inset in DIP; invalid values become zero.
func (t *TextInput) SetPadding(value float32) { t.setPadding(value) }
func (t *TextInput) IMContext() IMContext     { return t.imContext() }
func (t *TextInput) StyleChanged()            { t.styleChanged() }

func (t *TextInput) Measure(c layout.Constraint) layout.Measurement {
	if !t.Visible() {
		return layout.Measurement{}
	}
	t.ensureFormat()
	m := layout.Measurement{Size: t.constrain(c, geometry.Size{
		Width: defaultTextInputWidth, Height: t.lineHeight + 2*t.padding,
	}), Baseline: t.padding + t.baseline, HasBaseline: t.hasBaseline}
	return m
}

func (t *TextInput) Arrange(rect geometry.Rectangle) {
	t.WidgetBase.Arrange(rect)
	t.layoutSingleLine(rect.Size)
}
func (t *TextInput) Paint(p Painter) {
	t.layoutSingleLine(t.Rect().Size)
	t.paint(p)
}
func (t *TextInput) Snapshot() WidgetInfo {
	info := t.snapshot()
	info.Role = RoleTextInput
	info.Text = t.Text()
	return info
}
