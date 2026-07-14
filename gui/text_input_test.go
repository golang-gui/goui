package gui

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

func TestTextInputSnapshotAndFocusability(t *testing.T) {
	input := NewTextInput()
	input.SetID("name")
	input.SetText("hello")
	input.Arrange(geometry.Rect(1, 2, 160, 24))

	info := input.Snapshot()
	if info.ID != "name" {
		t.Fatalf("unexpected snapshot id: %q", info.ID)
	}
	if info.Role != RoleTextInput {
		t.Fatalf("unexpected snapshot role: %q", info.Role)
	}
	if info.Text != "hello" {
		t.Fatalf("unexpected snapshot text: %q", info.Text)
	}
	if !info.Focusable {
		t.Fatal("text input should be focusable")
	}
	if info.Bounds != geometry.Rect(1, 2, 160, 24) {
		t.Fatalf("unexpected snapshot bounds: %+v", info.Bounds)
	}
}

func TestTextInputSetTextRequestsLayoutAndEmitsSignal(t *testing.T) {
	win := &window{}
	input := NewTextInput()
	win.SetWidget(input)

	var texts []string
	input.ConnectText(func(text string) {
		texts = append(texts, text)
	})

	win.layoutDirty = false
	win.paintDirty = false
	input.SetText("")
	if win.layoutDirty || win.paintDirty || len(texts) != 0 {
		t.Fatal("setting unchanged text should not request layout or emit")
	}

	input.SetText("abc")
	if input.Text() != "abc" || input.caret != len("abc") {
		t.Fatalf("unexpected text state: text=%q caret=%d", input.Text(), input.caret)
	}
	if !win.layoutDirty || !win.paintDirty {
		t.Fatal("setting text did not request layout and paint")
	}
	if len(texts) != 1 || texts[0] != "abc" {
		t.Fatalf("unexpected text signal calls: %v", texts)
	}
}

func TestTextInputPaintDrawsChromeTextAndCaret(t *testing.T) {
	typo := &testTypography{
		lines: []typography.TextLine{
			{Start: 0, Length: 3, X: 0, Y: 1, Width: 30, Height: 18, Baseline: 14},
		},
		clusters: []typography.TextCluster{
			{Start: 0, Length: 1, X: 0, Y: 1, Width: 10, Height: 18, LineIndex: 0},
			{Start: 1, Length: 1, X: 10, Y: 1, Width: 10, Height: 18, LineIndex: 0},
			{Start: 2, Length: 1, X: 20, Y: 1, Width: 10, Height: 18, LineIndex: 0},
		},
	}
	setTestApplication(t, typo)

	win := &window{}
	input := NewTextInput()
	input.SetText("abc")
	input.caret = len("ab")
	input.Arrange(geometry.Rect(0, 0, 100, 24))
	win.SetWidget(input)
	if !win.SetFocusedWidget(input) {
		t.Fatal("text input should accept focus")
	}

	painter := new(testTextInputPainter)
	input.Paint(painter)

	if painter.fillRect != geometry.Rect(0, 0, 100, 24) || painter.fillBrush != graphics.RGB(255, 255, 255) {
		t.Fatalf("unexpected fill: rect=%+v brush=%+v", painter.fillRect, painter.fillBrush)
	}
	if painter.drawRect != geometry.Rect(0, 0, 100, 24) || painter.drawRectStrokeWidth != 1 || painter.drawRectBrush != graphics.RGB(70, 130, 220) {
		t.Fatalf("unexpected border: rect=%+v width=%v brush=%+v", painter.drawRect, painter.drawRectStrokeWidth, painter.drawRectBrush)
	}
	if len(typo.calls) != 1 {
		t.Fatalf("expected one text layout call, got %d", len(typo.calls))
	}
	call := typo.calls[0]
	if call.text != "abc" || call.width != 92 || call.height != 16 {
		t.Fatalf("unexpected text layout call: %+v", call)
	}
	if painter.textOrigin != (geometry.Point{X: 4, Y: 4}) {
		t.Fatalf("unexpected text origin: %+v", painter.textOrigin)
	}
	if painter.textLayout != typo.layouts[0] {
		t.Fatal("painter did not receive text input layout")
	}
	if !typo.layouts[0].destroyed {
		t.Fatal("paint did not destroy text layout")
	}
	if painter.drawLines != 1 {
		t.Fatalf("expected one caret draw, got %d", painter.drawLines)
	}
	if painter.lineP0 != (geometry.Point{X: 24, Y: 5}) || painter.lineP1 != (geometry.Point{X: 24, Y: 23}) {
		t.Fatalf("unexpected caret line: p0=%+v p1=%+v", painter.lineP0, painter.lineP1)
	}
}

func TestTextInputPaintSkipsCaretWhenNotFocused(t *testing.T) {
	typo := &testTypography{}
	setTestApplication(t, typo)

	input := NewTextInput()
	input.SetText("abc")
	input.Arrange(geometry.Rect(0, 0, 100, 24))

	painter := new(testTextInputPainter)
	input.Paint(painter)

	if painter.drawRectBrush != graphics.RGB(180, 180, 180) {
		t.Fatalf("unexpected unfocused border brush: %+v", painter.drawRectBrush)
	}
	if painter.drawLines != 0 {
		t.Fatalf("unfocused text input should not draw caret, got %d", painter.drawLines)
	}
}

func TestTextInputPaintEmptyFocusedSkipsTextLayoutAndDrawsCaret(t *testing.T) {
	typo := &testTypography{}
	setTestApplication(t, typo)

	win := &window{}
	input := NewTextInput()
	input.Arrange(geometry.Rect(0, 0, 100, 24))
	win.SetWidget(input)
	if !win.SetFocusedWidget(input) {
		t.Fatal("text input should accept focus")
	}

	painter := new(testTextInputPainter)
	input.Paint(painter)

	if len(typo.calls) != 0 {
		t.Fatalf("empty text should not create text layout, got %d calls", len(typo.calls))
	}
	if painter.textLayout != nil {
		t.Fatal("empty text should not draw a text layout")
	}
	if painter.drawLines != 1 {
		t.Fatalf("expected one caret draw, got %d", painter.drawLines)
	}
	// Empty input arranged 100x24 with default padding 4: caret spans the
	// content box, y from padding (4) to height-padding (24-4=20).
	if painter.lineP0 != (geometry.Point{X: 4, Y: 4}) ||
		painter.lineP1 != (geometry.Point{X: 4, Y: 20}) {
		t.Fatalf("unexpected caret line: p0=%+v p1=%+v", painter.lineP0, painter.lineP1)
	}
}

func TestTextInputUsesLocalStyleForChromeAndText(t *testing.T) {
	typo := &testTypography{}
	setTestApplication(t, typo)

	background := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	border := color.RGBA{R: 40, G: 50, B: 60, A: 255}
	foreground := color.RGBA{R: 70, G: 80, B: 90, A: 255}

	input := NewTextInput()
	input.SetPadding(6) // padding is a layout field now, not style
	input.SetStyleRules(
		style.Default().
			BackgroundColor(background).
			BorderColor(border).
			BorderWidth(2).
			Radius(3).
			ForegroundColor(foreground).
			FontFamily("Mono").
			FontSize(18),
	)
	input.SetText("abc")
	input.Arrange(geometry.Rect(0, 0, 100, 24))

	painter := new(testTextInputPainter)
	input.Paint(painter)

	if painter.fillRect != geometry.Rect(0, 0, 100, 24) || painter.fillRadius != 3 || painter.fillBrush != graphics.ColorOf(background) {
		t.Fatalf("unexpected styled fill: rect=%+v radius=%v brush=%+v", painter.fillRect, painter.fillRadius, painter.fillBrush)
	}
	if painter.drawRect != geometry.Rect(0, 0, 100, 24) || painter.drawRadius != 3 || painter.drawRectStrokeWidth != 2 || painter.drawRectBrush != graphics.ColorOf(border) {
		t.Fatalf("unexpected styled border: rect=%+v radius=%v width=%v brush=%+v", painter.drawRect, painter.drawRadius, painter.drawRectStrokeWidth, painter.drawRectBrush)
	}
	if len(typo.calls) != 1 {
		t.Fatalf("expected one text layout call, got %d", len(typo.calls))
	}
	call := typo.calls[0]
	if call.width != 88 || call.height != 12 {
		t.Fatalf("unexpected styled text bounds: %gx%g", call.width, call.height)
	}
	if painter.textOrigin != (geometry.Point{X: 6, Y: 6}) {
		t.Fatalf("unexpected styled text origin: %+v", painter.textOrigin)
	}
	if call.format.Font.Family != "Mono" || call.format.Font.Size != 18 {
		t.Fatalf("unexpected styled text font: %+v", call.format.Font)
	}
	if !colors.Equal(call.format.TextColor, foreground) {
		t.Fatalf("unexpected styled text color: %v", call.format.TextColor)
	}
}

func TestTextInputEditsFocusedWidgetFromKeyEvents(t *testing.T) {
	win := &window{}
	root := newTestWidget()
	input := NewTextInput()
	root.AddChild(input)
	win.SetWidget(root)
	if !win.SetFocusedWidget(input) {
		t.Fatal("text input should accept focus")
	}

	dispatchKey(t, win, events.KeyH, 0)
	dispatchKey(t, win, events.KeyI, events.ModifierShift)
	dispatchKey(t, win, events.KeySpace, 0)
	dispatchKey(t, win, events.KeyNumpad1, 0)

	if input.Text() != "hI 1" {
		t.Fatalf("unexpected text: %q", input.Text())
	}
	if input.caret != len(input.Text()) {
		t.Fatalf("unexpected caret: %d", input.caret)
	}
}

func TestTextInputEditingKeysMoveAndDeleteByRune(t *testing.T) {
	win := &window{}
	input := NewTextInput()
	input.SetText("a世b")
	win.SetWidget(input)
	if !win.SetFocusedWidget(input) {
		t.Fatal("text input should accept focus")
	}

	dispatchKey(t, win, events.KeyArrowLeft, 0)
	if input.caret != len("a世") {
		t.Fatalf("arrow left did not move by rune: %d", input.caret)
	}

	dispatchKey(t, win, events.KeyBackspace, 0)
	if input.Text() != "ab" || input.caret != len("a") {
		t.Fatalf("backspace did not delete previous rune: text=%q caret=%d", input.Text(), input.caret)
	}

	dispatchKey(t, win, events.KeyDelete, 0)
	if input.Text() != "a" || input.caret != len("a") {
		t.Fatalf("delete did not delete next rune: text=%q caret=%d", input.Text(), input.caret)
	}

	dispatchKey(t, win, events.KeyHome, 0)
	if input.caret != 0 {
		t.Fatalf("home did not move caret to start: %d", input.caret)
	}

	dispatchKey(t, win, events.KeyEnd, 0)
	if input.caret != len(input.Text()) {
		t.Fatalf("end did not move caret to end: %d", input.caret)
	}
}

func TestTextInputIgnoresShortcutModifiers(t *testing.T) {
	win := &window{}
	input := NewTextInput()
	win.SetWidget(input)
	if !win.SetFocusedWidget(input) {
		t.Fatal("text input should accept focus")
	}

	if err := win.DispatchEvent(events.KeyEvent{
		EventType: events.KeyDown,
		Key:       events.KeyA,
		Modifiers: events.ModifierControl,
	}); err != nil {
		t.Fatal(err)
	}

	if input.Text() != "" {
		t.Fatalf("shortcut key should not insert text: %q", input.Text())
	}
}

func TestTextInputStopsPropagationWhenEditing(t *testing.T) {
	win := &window{}
	root := newTestWidget()
	input := NewTextInput()
	root.AddChild(input)
	win.SetWidget(root)
	if !win.SetFocusedWidget(input) {
		t.Fatal("text input should accept focus")
	}

	var calls []string
	root.AddEventController(newRecordingController("root-bubble", PhaseBubble, &calls, nil))

	dispatchKey(t, win, events.KeyA, 0)
	assertStrings(t, calls, nil)

	if err := win.DispatchEvent(events.KeyEvent{
		EventType: events.KeyDown,
		Key:       events.KeyA,
		Modifiers: events.ModifierControl,
	}); err != nil {
		t.Fatal(err)
	}
	assertStrings(t, calls, []string{"root-bubble phase=2 type=9"})
}

func TestKeyEventTextMapsAsciiKeys(t *testing.T) {
	tests := []struct {
		name      string
		event     events.KeyEvent
		want      string
		wantFound bool
	}{
		{name: "letter", event: events.KeyEvent{Key: events.KeyA}, want: "a", wantFound: true},
		{name: "shift letter", event: events.KeyEvent{Key: events.KeyA, Modifiers: events.ModifierShift}, want: "A", wantFound: true},
		{name: "digit", event: events.KeyEvent{Key: events.Key1}, want: "1", wantFound: true},
		{name: "shift digit", event: events.KeyEvent{Key: events.Key1, Modifiers: events.ModifierShift}, want: "!", wantFound: true},
		{name: "punctuation", event: events.KeyEvent{Key: events.KeySlash, Modifiers: events.ModifierShift}, want: "?", wantFound: true},
		{name: "numpad", event: events.KeyEvent{Key: events.KeyNumpadDecimal}, want: ".", wantFound: true},
		{name: "shortcut", event: events.KeyEvent{Key: events.KeyA, Modifiers: events.ModifierControl}, wantFound: false},
		{name: "function", event: events.KeyEvent{Key: events.KeyF1}, wantFound: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := keyEventText(tt.event)
			if ok != tt.wantFound || got != tt.want {
				t.Fatalf("keyEventText() = %q, %v; want %q, %v", got, ok, tt.want, tt.wantFound)
			}
		})
	}
}

func dispatchKey(t *testing.T, win Window, key events.Key, modifiers events.Modifiers) {
	t.Helper()
	if err := win.DispatchEvent(events.KeyEvent{
		EventType: events.KeyDown,
		Key:       key,
		Modifiers: modifiers,
	}); err != nil {
		t.Fatal(err)
	}
}

type testTextInputPainter struct {
	testLabelPainter
	fillRect            geometry.Rectangle
	fillRadius          float32
	fillBrush           graphics.Brush
	drawRect            geometry.Rectangle
	drawRadius          float32
	drawRectStrokeWidth float32
	drawRectBrush       graphics.Brush
	drawLines           int
	lineP0              geometry.Point
	lineP1              geometry.Point
	lineStrokeWidth     float32
	lineBrush           graphics.Brush
}

func (p *testTextInputPainter) FillRect(rect geometry.Rectangle, brush graphics.Brush) {
	p.fillRect = rect
	p.fillBrush = brush
}

func (p *testTextInputPainter) FillRoundRect(rect geometry.Rectangle, radius float32, brush graphics.Brush) {
	p.fillRect = rect
	p.fillRadius = radius
	p.fillBrush = brush
}

func (p *testTextInputPainter) DrawRect(rect geometry.Rectangle, strokeWidth float32, brush graphics.Brush) {
	p.drawRect = rect
	p.drawRectStrokeWidth = strokeWidth
	p.drawRectBrush = brush
}

func (p *testTextInputPainter) DrawRoundRect(rect geometry.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
	p.drawRect = rect
	p.drawRadius = radius
	p.drawRectStrokeWidth = strokeWidth
	p.drawRectBrush = brush
}

func (p *testTextInputPainter) DrawLine(p0, p1 geometry.Point, strokeWidth float32, brush graphics.Brush) {
	p.drawLines++
	p.lineP0 = p0
	p.lineP1 = p1
	p.lineStrokeWidth = strokeWidth
	p.lineBrush = brush
}
