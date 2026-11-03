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

// defaultTextInputPadding clears the frame and its 1-2 DIP focus border while
// keeping the single-line height compact.
var defaultTextInputPadding = geometry.Insets{Left: 8, Top: 4, Right: 8, Bottom: 4}

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
	t.textEditor = newTextEditor(t, true, defaultTextInputPadding)
	t.connectChange(func(TextChange) { t.textSignal.Emit(t.Text()) })
	return t
}

func (t *TextInput) Text() string { return t.model.Text() }

// Placeholder is the normalized hint shown while the field is empty and has
// no IME preedit, including while focused. It is independent of the text value.
func (t *TextInput) Placeholder() string { return t.placeholder }

// SetPlaceholder sets a single-line hint, normalizing text as SetText does.
// Empty disables it. It does not change content, selection, history or IME.
func (t *TextInput) SetPlaceholder(text string) { t.setPlaceholder(text) }

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
		t.selection = TextSelection{Anchor: len(text), Caret: len(text)}
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
func (t *TextInput) ReadOnly() bool           { return t.readOnly }
func (t *TextInput) InputFilter() InputFilter { return t.inputFilter }

// SetInputFilter replaces the policy for future user edits. Nil removes it;
// programmatic text, history, selection and IME are not changed.
func (t *TextInput) SetInputFilter(filter InputFilter) { t.inputFilter = filter }

// Obscured reports whether display and semantic text are hidden. Application
// Text and ConnectText still use the actual value. Copy and cut are disabled.
func (t *TextInput) Obscured() bool { return t.obscured }

// SetObscured changes view-local presentation without editing text or history.
func (t *TextInput) SetObscured(value bool) {
	if t.Destroyed() || t.obscured == value {
		return
	}
	t.obscured = value
	t.refreshObscuring()
}

// ObscureChar returns the repeated replacement rune, defaulting to U+2022.
func (t *TextInput) ObscureChar() rune {
	if t.obscureChar == 0 {
		return '•'
	}
	return t.obscureChar
}

// SetObscureChar sets the masking rune. Zero restores U+2022. Invalid, control,
// whitespace and combining runes panic. It does not enable obscuring.
func (t *TextInput) SetObscureChar(char rune) {
	if !textedit.ValidObscureChar(char) {
		panic("gui: invalid obscuring character")
	}
	if char == 0 {
		char = '•'
	}
	if t.Destroyed() || t.ObscureChar() == char {
		return
	}
	t.obscureChar = char
	if t.obscured {
		t.refreshObscuring()
	}
}

func (t *TextInput) refreshObscuring() {
	t.inputRevision++
	t.resetParagraphs()
	if t.Destroyed() {
		return
	}
	t.hasDesiredX = false
	t.revealCaret = true
	t.updateContextMenu()
	t.RequestLayout()
	t.requestSemanticUpdate()
}
func (t *TextInput) SetReadOnly(value bool)   { t.setReadOnly(value) }
func (t *TextInput) Padding() geometry.Insets { return t.padding }

// SetPadding sets the per-edge content inset in DIP; invalid edges become zero.
func (t *TextInput) SetPadding(value geometry.Insets) { t.setPadding(value) }
func (t *TextInput) IMContext() IMContext             { return t.imContext() }
func (t *TextInput) StyleChanged()                    { t.styleChanged() }

func (t *TextInput) Measure(c layout.Constraint) layout.Measurement {
	if !t.Visible() {
		return layout.Measurement{}
	}
	t.ensureFormat()
	height, baseline := t.lineHeight, t.baseline
	if p := t.paragraph(0); p != nil {
		height, baseline = p.height, p.baseline
	}
	m := layout.Measurement{Size: t.constrain(c, geometry.Size{
		Width: defaultTextInputWidth, Height: height + t.verticalPadding(),
	}), Baseline: t.padding.Top + baseline, HasBaseline: t.hasBaseline}
	m.Baseline += max(0, (m.Height-t.verticalPadding()-height)/2)
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
	if !t.obscured {
		info.Text = t.Text()
	}
	return info
}
