package gui

import (
	"unicode/utf8"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

const (
	defaultTextInputWidth  = 160
	defaultTextInputHeight = 24
	textInputPaddingX      = 4
	textInputPaddingY      = 3
	textInputBorderWidth   = 1
	textInputCaretWidth    = 1
)

type TextInput struct {
	WidgetBase
	text       string
	caret      int
	format     typography.TextFormat
	key        *KeyEventController
	textSignal signal.Signal1[string]
}

func NewTextInput() *TextInput {
	input := &TextInput{
		format: DefaultTextInputTextFormat(),
	}
	input.SetFocusable(true)
	input.key = NewKeyEventController()
	input.key.ConnectKeyDown(input.handleKeyDown)
	input.AddEventController(input.key)
	return input
}

func DefaultTextInputTextFormat() typography.TextFormat {
	return DefaultLabelTextFormat()
}

func (t *TextInput) Text() string {
	return t.text
}

func (t *TextInput) SetText(text string) {
	t.setText(text, len(text))
}

func (t *TextInput) TextFormat() typography.TextFormat {
	return t.format
}

func (t *TextInput) SetTextFormat(format typography.TextFormat) {
	format = normalizeLabelTextFormat(format)
	if sameTextFormat(t.format, format) {
		return
	}
	t.format = format
	t.RequestLayout()
}

func (t *TextInput) ConnectText(fn func(string)) signal.Handle {
	return t.textSignal.Connect(fn)
}

func (t *TextInput) Measure(available geometry.Size) geometry.Size {
	if !t.Visible() {
		return geometry.Size{}
	}
	return geometry.Size{
		Width:  defaultTextInputWidth,
		Height: defaultTextInputHeight,
	}
}

func (t *TextInput) Paint(p Painter) {
	if !t.Visible() {
		return
	}

	size := t.Rect().Size
	rect := geometry.Rect(0, 0, size.Width, size.Height)
	p.FillRect(rect, graphics.RGB(255, 255, 255))
	p.DrawRect(rect, textInputBorderWidth, t.borderColor())

	origin := geometry.Point{X: textInputPaddingX, Y: textInputPaddingY}
	if len(t.text) == 0 {
		if t.Focused() {
			t.paintCaretRect(p, origin, t.defaultCaretRect())
		}
		return
	}

	textLayout := t.newTextLayout(t.textSize(size))
	if textLayout == nil {
		return
	}
	defer textLayout.Destroy()

	p.DrawTextLayout(origin, textLayout)
	if t.Focused() {
		t.paintCaret(p, origin, textLayout)
	}
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
	if win := t.Window(); win != nil {
		_ = win.RequestPaint()
	}
}

func (t *TextInput) newTextLayout(size geometry.Size) typography.TextLayout {
	if App == nil {
		return nil
	}
	typo := App.Typography()
	if typo == nil {
		return nil
	}
	textLayout, err := typo.NewTextLayout(t.text, normalizeLabelTextFormat(t.format), size.Width, size.Height)
	if err != nil {
		return nil
	}
	return textLayout
}

func (t *TextInput) textSize(size geometry.Size) geometry.Size {
	width := size.Width - textInputPaddingX*2
	if width < 0 {
		width = 0
	}
	height := size.Height - textInputPaddingY*2
	if height < 0 {
		height = 0
	}
	return geometry.Size{Width: width, Height: height}
}

func (t *TextInput) borderColor() graphics.Color {
	if t.Focused() {
		return graphics.RGB(70, 130, 220)
	}
	return graphics.RGB(180, 180, 180)
}

func (t *TextInput) paintCaret(p Painter, origin geometry.Point, layout typography.TextLayout) {
	rect := t.caretRect(layout)
	t.paintCaretRect(p, origin, rect)
}

func (t *TextInput) paintCaretRect(p Painter, origin geometry.Point, rect geometry.Rectangle) {
	x := origin.X + rect.X
	y0 := origin.Y + rect.Y
	y1 := y0 + rect.Height
	p.DrawLine(
		geometry.Point{X: x, Y: y0},
		geometry.Point{X: x, Y: y1},
		textInputCaretWidth,
		graphics.RGB(20, 20, 20),
	)
}

func (t *TextInput) caretRect(layout typography.TextLayout) geometry.Rectangle {
	caret := clampCaret(t.text, t.caret)
	lines, clusters := layout.MeasureMetrics()
	if len(lines) == 0 {
		return t.defaultCaretRect()
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
		height = defaultTextInputHeight - textInputPaddingY*2
	}
	return geometry.Rect(x, line.Y, textInputCaretWidth, height)
}

func (t *TextInput) defaultCaretRect() geometry.Rectangle {
	height := t.textSize(t.Rect().Size).Height
	if height <= 0 {
		height = defaultTextInputHeight - textInputPaddingY*2
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
