package gui

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

// 单行控件、绘制与输入契约。

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
	if input.Text() != "abc" || input.Selection().Caret != len("abc") {
		t.Fatalf("unexpected text state: text=%q caret=%d", input.Text(), input.Selection().Caret)
	}
	if !win.layoutDirty || !win.paintDirty {
		t.Fatal("setting text did not request layout and paint")
	}
	if len(texts) != 1 || texts[0] != "abc" {
		t.Fatalf("unexpected text signal calls: %v", texts)
	}
}

func TestTextInputMeasureReportsStableBaseline(t *testing.T) {
	typo := &testTypography{
		measureSize: geometry.Size{Width: 30, Height: 18},
		lines: []typography.TextLine{
			{Start: 0, Length: len(textInputHeightSample), Width: 30, Height: 18, Baseline: 14},
		},
	}
	setTestApplication(t, typo)
	input := NewTextInput()

	measured := input.Measure(layout.Loose(geometry.Size{Width: 300, Height: 100}))
	if measured.Size != (geometry.Size{Width: defaultTextInputWidth, Height: 26}) {
		t.Fatalf("unexpected text input size: %+v", measured.Size)
	}
	if !measured.HasBaseline || measured.Baseline != 18 {
		t.Fatalf("text input baseline should include padding: %+v", measured)
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
	input.SetSelection(TextSelection{len("ab"), len("ab")})
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
	if painter.drawRect != geometry.Rect(0.5, 0.5, 99, 23) || painter.drawRectStrokeWidth != 1 || painter.drawRectBrush != graphics.RGB(70, 130, 220) {
		t.Fatalf("unexpected border: rect=%+v width=%v brush=%+v", painter.drawRect, painter.drawRectStrokeWidth, painter.drawRectBrush)
	}
	if len(typo.calls) != 2 || typo.calls[0].text != textInputHeightSample {
		t.Fatalf("expected a font metrics sample and one display layout, got %+v", typo.calls)
	}
	call := typo.calls[1]
	// A single line is shaped unconstrained; the allocated content rectangle
	// clips rendering and horizontal scrolling, never truncates the layout.
	if call.text != "abc" || call.width != textInputMeasureExtent || call.height != textInputMeasureExtent || painter.clipRect != geometry.Rect(4, 4, 92, 16) {
		t.Fatalf("unexpected text layout call: %+v", call)
	}
	if painter.textOrigin != (geometry.Point{X: 4, Y: 4}) {
		t.Fatalf("unexpected text origin: %+v", painter.textOrigin)
	}
	if painter.textLayout != typo.layouts[1] {
		t.Fatal("painter did not receive text input layout")
	}
	// With caching, the layout is NOT destroyed after Paint — it lives until unmount/setter.
	if typo.layouts[1].destroyed {
		t.Fatal("paint should cache text layout for reuse")
	}
	if len(painter.fills) != 2 || painter.fills[1] != geometry.Rect(24, 5, 1, 18) {
		t.Fatalf("unexpected caret rectangle: %+v", painter.fills)
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
	if len(painter.fills) != 1 || painter.drawLines != 0 {
		t.Fatalf("unfocused text input should draw only the background: %+v", painter.fills)
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

	if len(typo.calls) != 1 || typo.calls[0].text != textInputHeightSample {
		t.Fatalf("empty text should only measure stable font metrics, got %+v", typo.calls)
	}
	if painter.textLayout != nil {
		t.Fatal("empty text should not draw a text layout")
	}
	// Empty-line geometry uses the stable font height. A short allocation
	// clips it to the content box rather than changing the font's metrics.
	if len(painter.fills) != 2 || painter.fills[1] != geometry.Rect(4, 4, 1, input.lineHeight) || painter.clipRect != geometry.Rect(4, 4, 92, 16) {
		t.Fatalf("unexpected empty caret/clip: %+v / %+v", painter.fills, painter.clipRect)
	}
}

func TestTextInputUsesStyleSheetForChromeAndText(t *testing.T) {
	typo := &testTypography{}
	background := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	border := color.RGBA{R: 40, G: 50, B: 60, A: 255}
	foreground := color.RGBA{R: 70, G: 80, B: 90, A: 255}
	useTestApplication(t, &application{
		typo: typo,
		style: style.Sheet(
			style.Name(styleNameTextInput).
				BackgroundColor(background).
				BorderColor(border).
				BorderWidth(2).
				Radius(3).
				ForegroundColor(foreground).
				FontFamily("Mono").
				FontSize(18),
		),
	})

	input := NewTextInput()
	input.SetPadding(6) // padding is a layout field, not style
	input.SetText("abc")
	input.Arrange(geometry.Rect(0, 0, 100, 24))

	painter := new(testTextInputPainter)
	input.Paint(painter)

	if painter.fillRect != geometry.Rect(0, 0, 100, 24) || painter.fillRadius != 3 || painter.fillBrush != graphics.ColorOf(background) {
		t.Fatalf("unexpected styled fill: rect=%+v radius=%v brush=%+v", painter.fillRect, painter.fillRadius, painter.fillBrush)
	}
	if painter.drawRect != geometry.Rect(1, 1, 98, 22) || painter.drawRadius != 2 || painter.drawRectStrokeWidth != 2 || painter.drawRectBrush != graphics.ColorOf(border) {
		t.Fatalf("unexpected styled border: rect=%+v radius=%v width=%v brush=%+v", painter.drawRect, painter.drawRadius, painter.drawRectStrokeWidth, painter.drawRectBrush)
	}
	if len(typo.calls) != 2 || typo.calls[0].text != textInputHeightSample {
		t.Fatalf("expected font sample and display layout, got %+v", typo.calls)
	}
	call := typo.calls[1]
	if call.width != textInputMeasureExtent || call.height != textInputMeasureExtent || painter.clipRect != geometry.Rect(6, 6, 88, 12) {
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

func TestTextInputNativeCommitDoesNotDuplicateKeyEvents(t *testing.T) {
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
	if input.Text() != "" {
		t.Fatalf("physical keys must not guess characters: %q", input.Text())
	}
	// Keep the original ASCII insertion assertion, now through the same
	// InputMethodHandler used by native windows instead of a US key map.
	for _, text := range []string{"h", "I", " ", "1"} {
		win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: text})
	}

	if input.Text() != "hI 1" {
		t.Fatalf("unexpected text: %q", input.Text())
	}
	if input.Selection().Caret != len(input.Text()) {
		t.Fatalf("unexpected caret: %d", input.Selection().Caret)
	}
}

func TestTextInputEditingKeysMoveAndDeleteByCluster(t *testing.T) {
	// These three runes are also three complete native Clusters. Navigation
	// now requires typography, rather than assuming every rune is a Cluster.
	setTestApplication(t, &editorTypography{})
	win := &window{}
	input := NewTextInput()
	input.SetText("a世b")
	win.SetWidget(input)
	if !win.SetFocusedWidget(input) {
		t.Fatal("text input should accept focus")
	}

	dispatchKey(t, win, events.KeyArrowLeft, 0)
	if input.Selection().Caret != len("a世") {
		t.Fatalf("arrow left did not move by rune: %d", input.Selection().Caret)
	}

	dispatchKey(t, win, events.KeyBackspace, 0)
	if input.Text() != "ab" || input.Selection().Caret != len("a") {
		t.Fatalf("backspace did not delete previous rune: text=%q caret=%d", input.Text(), input.Selection().Caret)
	}

	dispatchKey(t, win, events.KeyDelete, 0)
	if input.Text() != "a" || input.Selection().Caret != len("a") {
		t.Fatalf("delete did not delete next rune: text=%q caret=%d", input.Text(), input.Selection().Caret)
	}

	dispatchKey(t, win, events.KeyHome, 0)
	if input.Selection().Caret != 0 {
		t.Fatalf("home did not move caret to start: %d", input.Selection().Caret)
	}

	dispatchKey(t, win, events.KeyEnd, 0)
	if input.Selection().Caret != len(input.Text()) {
		t.Fatalf("end did not move caret to end: %d", input.Selection().Caret)
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
	setTestApplication(t, &editorTypography{})
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

	input.SetText("a")
	dispatchKey(t, win, events.KeyBackspace, 0)
	assertStrings(t, calls, nil)
	if input.Text() != "" {
		t.Fatal("editing key should delete text")
	}

	if err := win.DispatchEvent(events.KeyEvent{
		EventType: events.KeyDown,
		Key:       events.KeyA,
		Modifiers: textCommandModifier(),
	}); err != nil {
		t.Fatal(err)
	}
	// Ctrl+A is now an implemented selection command and is consumed.
	assertStrings(t, calls, nil)
	dispatchKey(t, win, events.KeyF1, 0)
	assertStrings(t, calls, []string{"root-bubble phase=2 type=9"})
}

func TestTextInputNativeCharactersAndCommandKeys(t *testing.T) {
	tests := []struct {
		name  string
		event events.KeyEvent
		want  string
	}{
		{name: "letter", event: events.KeyEvent{Key: events.KeyA}, want: "a"},
		{name: "shift letter", event: events.KeyEvent{Key: events.KeyA, Modifiers: events.ModifierShift}, want: "A"},
		{name: "digit", event: events.KeyEvent{Key: events.Key1}, want: "1"},
		{name: "shift digit", event: events.KeyEvent{Key: events.Key1, Modifiers: events.ModifierShift}, want: "!"},
		{name: "punctuation", event: events.KeyEvent{Key: events.KeySlash, Modifiers: events.ModifierShift}, want: "?"},
		{name: "numpad", event: events.KeyEvent{Key: events.KeyNumpadDecimal}, want: "."},
		{name: "layout", event: events.KeyEvent{Key: events.KeyA}, want: "q"},
		{name: "AltGr", event: events.KeyEvent{Key: events.KeyQ, Modifiers: events.ModifierControl | events.ModifierAlt}, want: "@"},
		{name: "dead key", event: events.KeyEvent{Key: events.KeyE}, want: "é"},
		{name: "emoji", want: "😀"},
		{name: "shortcut", event: events.KeyEvent{Key: events.KeyA, Modifiers: events.ModifierControl}},
		{name: "function", event: events.KeyEvent{Key: events.KeyF1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			win := &window{}
			input := NewTextInput()
			win.SetWidget(input)
			win.SetFocusedWidget(input)
			dispatchKey(t, win, tt.event.Key, tt.event.Modifiers)
			if input.Text() != "" {
				t.Fatal("key event inserted guessed text")
			}
			if tt.want != "" {
				win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: tt.want})
			}
			if input.Text() != tt.want {
				t.Fatalf("native text = %q, want %q", input.Text(), tt.want)
			}
		})
	}
}

func TestTextInputCommitInsertsAtCaret(t *testing.T) {
	input := NewTextInput()
	input.SetText("ac")
	input.SetSelection(TextSelection{len("a"), len("a")})

	input.onCommit(IMCommit{Text: "b"})

	if input.Text() != "abc" {
		t.Fatalf("unexpected text: %q", input.Text())
	}
	if input.Selection().Caret != len("ab") {
		t.Fatalf("unexpected caret: %d", input.Selection().Caret)
	}
}

// 绘制与事件辅助。

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
	fills               []geometry.Rectangle
	clipRect            geometry.Rectangle
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
	if len(p.fills) == 0 {
		p.fillRect, p.fillBrush = rect, brush
	}
	p.fills = append(p.fills, rect)
}

func (p *testTextInputPainter) SetClipRect(rect geometry.Rectangle) { p.clipRect = rect }

func (p *testTextInputPainter) FillRoundRect(rect geometry.Rectangle, radius float32, brush graphics.Brush) {
	p.fillRect = rect
	p.fillRadius = radius
	p.fillBrush = brush
	p.fills = append(p.fills, rect)
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

// 单行模型、Cluster、滚动与 baseline。

func newInputFixture(t *testing.T, text string) (*TextInput, *window, *editorTypography) {
	t.Helper()
	typo := &editorTypography{}
	setTestApplication(t, typo)
	input := NewTextInput()
	input.SetText(text)
	win := &window{}
	win.SetWidget(input)
	t.Cleanup(func() { win.SetWidget(nil) })
	input.Arrange(geometry.Rect(0, 0, 80, 40))
	win.SetFocusedWidget(input)
	return input, win, typo
}

func TestTextInputSingleLineModelAndEqualReload(t *testing.T) {
	input, win, _ := newInputFixture(t, "a\r\nb\rc\nd")
	if input.Text() != "a b c d" || input.model.LineCount() != 1 {
		t.Fatal("line breaks were not normalized to spaces")
	}
	if _, exposesModel := any(input).(interface{ SetModel(*TextModel) }); exposesModel {
		t.Fatal("TextInput must not expose the multiline model setter")
	}
	if _, scrollable := any(input).(Scrollable); scrollable || len(input.Children()) != 0 {
		t.Fatal("TextInput became a nested/scrollable TextView")
	}
	input.SetSelection(TextSelection{1, 3})
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "中\n文", Caret: 4})
	composition, rev := input.preedit, input.model.Revision()
	input.SetText("a\r\nb\rc\nd")
	if input.preedit != composition || input.Selection() != (TextSelection{1, 3}) || input.model.Revision() != rev {
		t.Fatal("equal normalized SetText disturbed selection/composition")
	}
	if composition.Text() != "中 文" || composition.Caret() != 4 || input.displayLineCount() != 1 {
		t.Fatalf("multiline composition escaped the single-line projection: %+v", composition)
	}
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "X\r\nY"})
	if input.Text() != "aX Y c d" {
		t.Fatalf("composition replacement = %q", input.Text())
	}
	dispatchKey(t, win, events.KeyZ, textCommandModifier())
	if input.Text() != "a b c d" || input.Selection() != (TextSelection{1, 3}) {
		t.Fatal("single undo did not restore replacement and selection")
	}
	input.SetText("new")
	if input.model.CanUndo() || input.model.CanRedo() || input.Selection() != (TextSelection{3, 3}) {
		t.Fatal("changed SetText did not reset history/caret")
	}
}

func TestTextInputHorizontalRevealAndStableGeometry(t *testing.T) {
	input, win, typo := newInputFixture(t, strings.Repeat("x", 100))
	before := input.Measure(layout.Unbounded())
	caret, _ := input.caretRect()
	if input.offset.X <= 0 || caret.X-input.offset.X < input.padding || caret.X+caret.Width-input.offset.X > input.Rect().Width-input.padding {
		t.Fatalf("end caret not horizontally visible: caret=%v offset=%v", caret, input.offset)
	}
	count := len(typo.calls)
	for range 5 {
		input.Paint(&testTextInputPainter{})
		input.Measure(layout.Unbounded())
	}
	if len(typo.calls) != count {
		t.Fatal("stable frames reallocated single-line layouts")
	}
	dispatchKey(t, win, events.KeyHome, 0)
	input.Arrange(input.Rect())
	if input.offset.X != 0 {
		t.Fatal("Home did not restore the start")
	}
	input.SetText("中")
	input.Arrange(input.Rect())
	after := input.Measure(layout.Unbounded())
	if input.offset.X != 0 || before != after {
		t.Fatalf("content changed line geometry: before=%+v after=%+v offset=%v", before, after, input.offset)
	}
}

func TestTextInputSubmitReadOnlyAndSnapshot(t *testing.T) {
	input, win, _ := newInputFixture(t, "abc")
	submits := 0
	input.ConnectSubmit(func() { submits++ })
	dispatchKey(t, win, events.KeyEnter, 0)
	if submits != 1 || input.Text() != "abc" {
		t.Fatal("Enter inserted text instead of submitting")
	}
	input.SetReadOnly(true)
	if win.activeIM != nil {
		t.Fatal("readonly field retained native input binding")
	}
	dispatchKey(t, win, events.KeyBackspace, 0)
	dispatchKey(t, win, events.KeyA, textCommandModifier())
	info := input.Snapshot()
	if info.Role != RoleTextInput || info.Text != "abc" || info.TextEditing == nil || !info.TextEditing.ReadOnly || info.TextEditing.Selection != (TextSelection{0, 3}) {
		t.Fatalf("readonly editing state not exposed: %+v", info)
	}
	input.SetReadOnly(false)
	input.ConnectSubmit(func() { win.SetWidget(nil) })
	dispatchKey(t, win, events.KeyEnter, 0)
	if !input.suspended || input.blinkTimer != nil {
		t.Fatal("submit continued after unmount")
	}
}

func TestTextInputPasteAndDetachedSetText(t *testing.T) {
	input, win, _ := newInputFixture(t, "abc")
	clip := &editorClipboard{}
	App.(*application).clipboard = clip
	input.SetSelection(TextSelection{1, 2})
	dispatchKey(t, win, events.KeyV, textCommandModifier())
	clip.pending("1\r\n2\n3", true)
	if input.Text() != "a1 2 3c" {
		t.Fatalf("paste = %q", input.Text())
	}
	dispatchKey(t, win, events.KeyZ, textCommandModifier())
	if input.Text() != "abc" || input.Selection() != (TextSelection{1, 2}) {
		t.Fatal("paste undo split or lost selection")
	}
	dispatchKey(t, win, events.KeyV, textCommandModifier())
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "x", Caret: 1})
	clip.pending("stale", true)
	if input.Text() != "abc" || input.preedit == nil {
		t.Fatal("stale paste changed composition")
	}
	win.SetWidget(nil)
	changes := 0
	input.ConnectText(func(string) { changes++ })
	input.SetText("detached\ntext")
	if changes != 1 || input.Text() != "detached text" {
		t.Fatal("detached setter did not notify")
	}
	win.SetWidget(input)
	input.Arrange(input.Rect())
	if input.Selection().Caret != len(input.Text()) || input.displayParagraph(0) != input.Text() {
		t.Fatal("reattachment did not restore current private model")
	}
}

// The initial e + combining mark form one supplied Cluster, not two runes.
type combinedInputTypography struct{ editorTypography }

func (c *combinedInputTypography) NewTextLayout(text string, f typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, err := c.editorTypography.NewTextLayout(text, f, width, height)
	if text == "e\u0301x" {
		p := l.(*testTextLayout)
		p.clusters = []typography.TextCluster{
			{Start: 0, Length: 3, X: 0, Width: 10, Height: 20},
			{Start: 3, Length: 1, X: 10, Width: 10, Height: 20},
		}
		p.lines[0].Width, p.measureSize.Width = 20, 20
	}
	return l, err
}

func TestTextInputNeverSplitsSuppliedCluster(t *testing.T) {
	setTestApplication(t, &combinedInputTypography{})
	input := NewTextInput()
	input.SetText("e\u0301x")
	win := &window{}
	win.SetWidget(input)
	t.Cleanup(func() { win.SetWidget(nil) })
	win.SetFocusedWidget(input)
	input.Arrange(geometry.Rect(0, 0, 100, 40))
	input.SetSelection(TextSelection{1, 1})
	if input.Selection() != (TextSelection{}) {
		t.Fatal("setter split the combining Cluster")
	}
	dispatchKey(t, win, events.KeyArrowRight, 0)
	if input.Selection().Caret != 3 {
		t.Fatal("arrow split the combining Cluster")
	}
	dispatchKey(t, win, events.KeyBackspace, 0)
	if input.Text() != "x" {
		t.Fatalf("delete split the Cluster: %q", input.Text())
	}
	dispatchKey(t, win, events.KeyZ, textCommandModifier())
	if input.Text() != "e\u0301x" || input.Selection().Caret != 3 {
		t.Fatal("undo lost the Cluster selection")
	}
}

type baselineInputTypography struct{ editorTypography }

func (c *baselineInputTypography) NewTextLayout(text string, f typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, err := c.editorTypography.NewTextLayout(text, f, width, height)
	p := l.(*testTextLayout)
	p.lines[0].Baseline = 14
	if text == textInputHeightSample {
		p.lines[0].Height, p.lines[0].Baseline, p.measureSize.Height = 28, 22, 28
	}
	return l, err
}

func TestTextInputPaintMatchesStableMeasuredBaseline(t *testing.T) {
	setTestApplication(t, &baselineInputTypography{})
	input := NewTextInput()
	input.SetText("abc")
	m := input.Measure(layout.Unbounded())
	if !m.HasBaseline || m.Baseline != 26 {
		t.Fatalf("stable sample baseline = %+v", m)
	}
	for _, extra := range []float32{0, 20} {
		input.Arrange(geometry.Rect(0, 0, m.Width, m.Height+extra))
		p := &testTextInputPainter{}
		input.Paint(p)
		lines, _ := p.textLayout.MeasureMetrics()
		if got := p.textOrigin.Y + lines[0].Baseline; got != m.Baseline+extra/2 {
			t.Fatalf("glyph baseline %g != measured baseline %g + centering %g", got, m.Baseline, extra/2)
		}
	}
}

type changingInputTypography struct {
	editorTypography
	joined bool
}

func (c *changingInputTypography) NewTextLayout(text string, f typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, err := c.editorTypography.NewTextLayout(text, f, width, height)
	if c.joined && text == "fi" {
		p := l.(*testTextLayout)
		p.clusters = []typography.TextCluster{{Start: 0, Length: 2, Width: 10, Height: 20}}
		p.lines[0].Width, p.measureSize.Width = 10, 10
	}
	return l, err
}

func TestTextInputCommandRespectsChangedFontClusters(t *testing.T) {
	for _, unmount := range []bool{false, true} {
		t.Run(fmt.Sprintf("unmount=%t", unmount), func(t *testing.T) {
			typo := &changingInputTypography{}
			setTestApplication(t, typo)
			input := NewTextInput()
			input.SetText("fi")
			win := &window{}
			win.SetWidget(input)
			t.Cleanup(func() { win.SetWidget(nil) })
			win.SetFocusedWidget(input)
			input.SetSelection(TextSelection{1, 1})
			typo.joined = true
			App.(*application).SetStyleSheet(textStyleSheet(20, color.Black))
			input.SetStyleName("text-input")
			if unmount {
				input.ConnectSelection(func(TextSelection) { win.SetWidget(nil) })
				win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "x"})
			} else {
				dispatchKey(t, win, events.KeyBackspace, 0)
			}
			if input.Text() != "fi" || input.Selection() != (TextSelection{}) || input.suspended != unmount {
				t.Fatalf("command split a new Cluster or continued after unmount: text=%q selection=%+v suspended=%t", input.Text(), input.Selection(), input.suspended)
			}
		})
	}
}
