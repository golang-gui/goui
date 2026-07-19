package gui

import (
	"unicode/utf8"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

const (
	defaultTextInputWidth   = 160
	defaultTextInputPadding = 4
	textInputCaretWidth     = 1
)

type TextInput struct {
	WidgetBase
	padding      float32 // self-held: text input self-draws content with an inner inset
	text         string
	caret        int
	preedit      string // active input-method composition, not part of text
	preeditCaret int    // caret byte offset within preedit
	im           IMContext
	key          *KeyEventController
	textSignal   signal.Signal1[string]
}

func NewTextInput() *TextInput {
	input := new(TextInput)
	input.SetFocusable(true)
	input.padding = defaultTextInputPadding
	input.im = NewIMContext()
	input.im.ConnectCommit(input.onCommit)
	input.im.ConnectPreedit(input.onPreedit)
	input.key = NewKeyEventController()
	input.key.ConnectKeyDown(input.handleKeyDown)
	input.AddEventController(input.key)
	return input
}

// IMContext satisfies IMClient: the window binds this context to the native
// input method while the field is focused (see doc/DesignIME.md §6).
func (t *TextInput) IMContext() IMContext { return t.im }

// onCommit inserts input-method-committed text at the caret and clears any
// preedit. This is the field's real text-insertion path.
func (t *TextInput) onCommit(text string) {
	t.setPreedit("", 0)
	t.insertText(text)
}

// onPreedit updates the in-progress composition shown inline before the caret.
func (t *TextInput) onPreedit(text string, caret int) {
	t.setPreedit(text, caret)
}

func (t *TextInput) setPreedit(text string, caret int) {
	if t.preedit == text && t.preeditCaret == caret {
		return
	}
	t.preedit = text
	t.preeditCaret = caret
	t.RequestLayout()
}

// displayText is the committed text with the active preedit spliced in at the
// caret. Text() still returns committed text only.
func (t *TextInput) displayText() string {
	if t.preedit == "" {
		return t.text
	}
	caret := clampCaret(t.text, t.caret)
	return t.text[:caret] + t.preedit + t.text[caret:]
}

// displayCaret is the visible caret byte offset within displayText — inside the
// preedit while composing.
func (t *TextInput) displayCaret() int {
	caret := clampCaret(t.text, t.caret)
	if t.preedit == "" {
		return caret
	}
	return caret + clampCaret(t.preedit, t.preeditCaret)
}

func (t *TextInput) Padding() float32 { return t.padding }

func (t *TextInput) SetPadding(padding float32) {
	if t.padding == padding {
		return
	}
	t.padding = padding
	t.RequestLayout()
}

func (t *TextInput) Text() string {
	return t.text
}

func (t *TextInput) SetText(text string) {
	t.setText(text, len(text))
}

func (t *TextInput) ConnectText(fn func(string)) signal.Handle {
	return t.textSignal.Connect(fn)
}

func (t *TextInput) Measure(c layout.Constraint) geometry.Size {
	if !t.Visible() {
		return geometry.Size{}
	}
	size, _ := t.resolvedStyle().FontSize()
	padding := t.padding
	return t.constrain(c, geometry.Size{
		Width:  defaultTextInputWidth,
		Height: textLineHeight(size) + padding*2,
	})
}

func (t *TextInput) Paint(p Painter) {
	if !t.Visible() {
		return
	}

	s := t.resolvedStyle()
	size := t.Rect().Size
	rect := geometry.Rect(0, 0, size.Width, size.Height)
	paintStyledBox(p, rect, s)

	padding := t.padding
	origin := geometry.Point{X: padding, Y: padding}
	format := t.textFormat(s)
	lineHeight := textLineHeight(format.Font.Size)

	if len(t.displayText()) == 0 {
		if t.Focused() {
			caret := t.defaultCaretRect(padding, lineHeight)
			t.reportCaret(origin, caret)
			if caretColor, ok := t.caretColor(format); ok {
				t.paintCaretRect(p, origin, caret, caretColor)
			}
		}
		return
	}

	textLayout := t.newTextLayout(size.Inset(padding), format)
	if textLayout == nil {
		return
	}
	defer textLayout.Destroy()

	p.DrawTextLayout(origin, textLayout)
	if caretColor, ok := t.caretColor(format); ok {
		if t.preedit != "" {
			t.paintPreeditUnderline(p, origin, textLayout, padding, lineHeight, caretColor)
		}
		if t.Focused() {
			caret := t.caretRect(textLayout, padding, lineHeight)
			t.reportCaret(origin, caret)
			t.paintCaretRect(p, origin, caret, caretColor)
		}
	}
}

// reportCaret hands the caret rectangle (in widget-local coordinates) to the
// input method so it can position the candidate window near the caret.
func (t *TextInput) reportCaret(origin geometry.Point, rect geometry.Rectangle) {
	t.im.SetCaretRect(geometry.Rect(
		origin.X+rect.X,
		origin.Y+rect.Y,
		rect.Width,
		rect.Height,
	))
}

// paintPreeditUnderline underlines the composing region so the user can tell
// uncommitted text apart from committed text.
func (t *TextInput) paintPreeditUnderline(p Painter, origin geometry.Point, layout typography.TextLayout, padding, lineHeight float32, color graphics.Color) {
	start := clampCaret(t.text, t.caret)
	end := start + len(t.preedit)
	from := t.caretRectAt(layout, start, padding, lineHeight)
	to := t.caretRectAt(layout, end, padding, lineHeight)
	y := origin.Y + from.Y + from.Height - textInputCaretWidth
	p.DrawLine(
		geometry.Point{X: origin.X + from.X, Y: y},
		geometry.Point{X: origin.X + to.X, Y: y},
		textInputCaretWidth,
		color,
	)
}

// caretColor returns the caret color taken from the style foreground. When the
// style leaves the foreground unset there is nothing to draw the caret with, so
// ok is false and the caret is skipped (see the unset-skips-drawing rule).
func (t *TextInput) caretColor(format typography.TextFormat) (graphics.Color, bool) {
	if format.TextColor == nil {
		return graphics.Color{}, false
	}
	return graphics.ColorOf(format.TextColor), true
}

func (t *TextInput) Snapshot() WidgetInfo {
	info := t.WidgetBase.Snapshot()
	info.Role = RoleTextInput
	info.Text = t.text
	return info
}

func (t *TextInput) handleKeyDown(ctx EventContext, event events.KeyEvent) {
	if t.handleEditingKey(event) {
		ctx.StopPropagation()
	}
}

func (t *TextInput) handleEditingKey(event events.KeyEvent) bool {
	switch event.Key {
	case events.KeyBackspace:
		return t.deleteBeforeCaret()
	case events.KeyDelete:
		return t.deleteAfterCaret()
	case events.KeyArrowLeft:
		return t.moveCaret(previousRuneIndex(t.text, t.caret))
	case events.KeyArrowRight:
		return t.moveCaret(nextRuneIndex(t.text, t.caret))
	case events.KeyHome:
		return t.moveCaret(0)
	case events.KeyEnd:
		return t.moveCaret(len(t.text))
	}

	text, ok := keyEventText(event)
	if !ok {
		return false
	}
	t.insertText(text)
	return true
}

func (t *TextInput) insertText(text string) {
	if text == "" {
		return
	}
	caret := clampCaret(t.text, t.caret)
	t.setText(t.text[:caret]+text+t.text[caret:], caret+len(text))
}

func (t *TextInput) deleteBeforeCaret() bool {
	caret := clampCaret(t.text, t.caret)
	if caret == 0 {
		return false
	}
	prev := previousRuneIndex(t.text, caret)
	t.setText(t.text[:prev]+t.text[caret:], prev)
	return true
}

func (t *TextInput) deleteAfterCaret() bool {
	caret := clampCaret(t.text, t.caret)
	if caret == len(t.text) {
		return false
	}
	next := nextRuneIndex(t.text, caret)
	t.setText(t.text[:caret]+t.text[next:], caret)
	return true
}

func (t *TextInput) moveCaret(caret int) bool {
	caret = clampCaret(t.text, caret)
	if t.caret == caret {
		return false
	}
	t.caret = caret
	t.requestPaint()
	return true
}

func (t *TextInput) setText(text string, caret int) {
	caret = clampCaret(text, caret)
	if t.text == text && t.caret == caret {
		return
	}
	textChanged := t.text != text
	t.text = text
	t.caret = caret
	if textChanged {
		t.textSignal.Emit(text)
		t.RequestLayout()
		t.requestSemanticUpdate()
		return
	}
	t.requestPaint()
}

func (t *TextInput) requestPaint() {
	// Root is the widget host (window or popover); Window() is nil for a
	// popover-hosted widget, which would drop the repaint request.
	if r := t.Root(); r != nil {
		_ = r.RequestPaint()
	}
}

func (t *TextInput) newTextLayout(size geometry.Size, format typography.TextFormat) typography.TextLayout {
	if App == nil {
		return nil
	}
	typo := App.Typography()
	if typo == nil {
		return nil
	}
	textLayout, err := typo.NewTextLayout(t.displayText(), format, size.Width, size.Height)
	if err != nil {
		return nil
	}
	return textLayout
}

// textFormat builds the single-line text format from the resolved style. Font
// family, size and color come from the style; wrapping and alignment are fixed
// for a single-line field.
func (t *TextInput) textFormat(s style.Style) typography.TextFormat {
	return textFormatFromStyle(s, WrapNone, TextAlignBegin)
}

func (t *TextInput) resolvedStyle() style.Style {
	name := t.StyleName()
	if name == "" {
		name = styleNameTextInput
	}
	return ResolveStyle(name, style.PartDefault, t.styleState())
}

func (t *TextInput) styleState() style.State {
	if t.Focused() {
		return style.Focused
	}
	return style.Normal
}

func (t *TextInput) paintCaretRect(p Painter, origin geometry.Point, rect geometry.Rectangle, caretColor graphics.Color) {
	x := origin.X + rect.X
	y0 := origin.Y + rect.Y
	y1 := y0 + rect.Height
	p.DrawLine(
		geometry.Point{X: x, Y: y0},
		geometry.Point{X: x, Y: y1},
		textInputCaretWidth,
		caretColor,
	)
}

func (t *TextInput) caretRect(layout typography.TextLayout, padding, lineHeight float32) geometry.Rectangle {
	return t.caretRectAt(layout, t.displayCaret(), padding, lineHeight)
}

func (t *TextInput) caretRectAt(layout typography.TextLayout, caret int, padding, lineHeight float32) geometry.Rectangle {
	lines, clusters := layout.MeasureMetrics()
	if len(lines) == 0 {
		return t.defaultCaretRect(padding, lineHeight)
	}

	line := lines[0]
	lineIndex := 0
	for i, current := range lines {
		if caret >= current.Start && caret <= current.Start+current.Length {
			line = current
			lineIndex = i
			break
		}
	}

	x := line.X
	for _, cluster := range clusters {
		if cluster.LineIndex != lineIndex {
			continue
		}
		if caret <= cluster.Start {
			x = cluster.X
			break
		}
		x = cluster.X + cluster.Width
		if cluster.Length > 0 && caret < cluster.Start+cluster.Length {
			break
		}
	}

	height := line.Height
	if height <= 0 {
		height = lineHeight
	}
	return geometry.Rect(x, line.Y, textInputCaretWidth, height)
}

func (t *TextInput) defaultCaretRect(padding, lineHeight float32) geometry.Rectangle {
	height := t.Rect().Size.Inset(padding).Height
	if height <= 0 {
		height = lineHeight
	}
	return geometry.Rect(0, 0, textInputCaretWidth, height)
}

func keyEventText(event events.KeyEvent) (string, bool) {
	if event.Modifiers&(events.ModifierControl|events.ModifierAlt|events.ModifierSuper) != 0 {
		return "", false
	}

	shift := event.Modifiers&events.ModifierShift != 0
	switch {
	case events.KeyA <= event.Key && event.Key <= events.KeyZ:
		ch := byte('a' + event.Key - events.KeyA)
		if shift {
			ch = byte('A' + event.Key - events.KeyA)
		}
		return string([]byte{ch}), true
	case events.Key0 <= event.Key && event.Key <= events.Key9:
		index := event.Key - events.Key0
		if shift {
			return string([]byte{")!@#$%^&*("[index]}), true
		}
		return string([]byte{'0' + byte(index)}), true
	case events.KeyNumpad0 <= event.Key && event.Key <= events.KeyNumpad9:
		return string([]byte{'0' + byte(event.Key-events.KeyNumpad0)}), true
	}

	switch event.Key {
	case events.KeySpace:
		return " ", true
	case events.KeyMinus:
		return shifted("-", "_", shift), true
	case events.KeyEqual:
		return shifted("=", "+", shift), true
	case events.KeyBracketLeft:
		return shifted("[", "{", shift), true
	case events.KeyBracketRight:
		return shifted("]", "}", shift), true
	case events.KeyBackslash:
		return shifted("\\", "|", shift), true
	case events.KeySemicolon:
		return shifted(";", ":", shift), true
	case events.KeyQuote:
		return shifted("'", "\"", shift), true
	case events.KeyComma:
		return shifted(",", "<", shift), true
	case events.KeyPeriod:
		return shifted(".", ">", shift), true
	case events.KeySlash:
		return shifted("/", "?", shift), true
	case events.KeyBackquote:
		return shifted("`", "~", shift), true
	case events.KeyNumpadAdd:
		return "+", true
	case events.KeyNumpadSubtract:
		return "-", true
	case events.KeyNumpadMultiply:
		return "*", true
	case events.KeyNumpadDivide:
		return "/", true
	case events.KeyNumpadDecimal:
		return ".", true
	}
	return "", false
}

func shifted(normal, shifted string, shift bool) string {
	if shift {
		return shifted
	}
	return normal
}

func previousRuneIndex(text string, index int) int {
	index = clampCaret(text, index)
	if index == 0 {
		return 0
	}
	_, size := utf8.DecodeLastRuneInString(text[:index])
	return index - size
}

func nextRuneIndex(text string, index int) int {
	index = clampCaret(text, index)
	if index == len(text) {
		return len(text)
	}
	_, size := utf8.DecodeRuneInString(text[index:])
	return index + size
}

func clampCaret(text string, index int) int {
	if index <= 0 {
		return 0
	}
	if index >= len(text) {
		return len(text)
	}
	for index > 0 && !utf8.RuneStart(text[index]) {
		index--
	}
	return index
}
