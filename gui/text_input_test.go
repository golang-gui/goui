package gui

import (
	"fmt"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
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

// 用可推进 GUI 计时队列验证遮蔽，不等待真实时间、不创建原生窗口。
func obscuredInputFixture(t *testing.T) (*TextInput, *window, *timerTestQueue) {
	t.Helper()
	_, _, win, q, _ := timedEditorFixture(t)
	input := NewTextInput()
	input.SetObscured(true)
	input.SetObscureDelay(600 * time.Millisecond)
	box := NewLinearBox(layout.DirectionVertical)
	box.AddChild(input)
	win.SetWidget(box)
	input.Arrange(geometry.Rect(0, 0, 240, 36))
	win.SetFocusedWidget(input)
	return input, win, q
}

func obscuredLayoutText(input *TextInput) string { return input.paragraph(0).layout.Text() }

func advanceObscureTimer(t *testing.T, q *timerTestQueue, delay time.Duration) {
	t.Helper()
	q.advance(delay)
	for len(q.posts) > 0 {
		q.dispatch(t)
	}
}

func TestTextInputObscureDelayAndRestart(t *testing.T) {
	input, _, q := obscuredInputFixture(t)
	input.SetText("é")
	input.im.emitCommit(IMCommit{Text: "中"})
	if got := obscuredLayoutText(input); got != "•中" {
		t.Fatal("最后一个提交字素没有显露", got)
	}
	info := input.Snapshot()
	if !info.TextEditing.Obscured || info.Text != "" || info.TextEditing.VisibleText != "" {
		t.Fatal("短暂显露泄露语义正文", info)
	}
	revision, selection := input.model.Revision(), input.Selection()
	timer := input.obscureHint.timer
	advanceObscureTimer(t, q, 599*time.Millisecond)
	input.SetObscureDelay(600 * time.Millisecond)
	input.SetObscured(true)
	input.SetObscureChar('•')
	if got := obscuredLayoutText(input); got != "•中" {
		t.Fatal("相同配置提前隐藏提示", got)
	}
	advanceObscureTimer(t, q, time.Millisecond)
	if got := obscuredLayoutText(input); got != "••" || timer.Active() {
		t.Fatal("相同配置重新启动了倒计时，或到期未遮蔽", got)
	}
	if input.model.Revision() != revision || input.Selection() != selection || !input.model.CanUndo() {
		t.Fatal("视觉计时改变了正文、选区或历史")
	}
	input.im.emitCommit(IMCommit{Text: "a"})
	q.advance(600 * time.Millisecond) // 原到期通知已经排队，但尚未派发。
	input.im.emitCommit(IMCommit{Text: "👨‍👩‍👧‍👦"})
	for len(q.posts) > 0 {
		q.dispatch(t)
	}
	if got := obscuredLayoutText(input); got != "•••👨‍👩‍👧‍👦" || input.obscureHint.timer != timer {
		t.Fatal("旧通知遮蔽了新输入，或没有复用计时器", got)
	}
	advanceObscureTimer(t, q, 599*time.Millisecond)
	if got := obscuredLayoutText(input); got != "•••👨‍👩‍👧‍👦" {
		t.Fatal("新输入没有获得完整显示时间", got)
	}
	advanceObscureTimer(t, q, time.Millisecond)
	if got := obscuredLayoutText(input); got != "••••" {
		t.Fatal("新输入到期未遮蔽", got)
	}
}

func TestTextInputObscureDelayCancellation(t *testing.T) {
	for _, action := range []string{"selection", "arrow", "focus", "window_focus", "disable", "parent_disable", "hide", "unmount", "destroy", "read_only", "zero_delay", "changed_delay", "character", "toggle", "model", "preedit"} {
		t.Run(action, func(t *testing.T) {
			input, win, q := obscuredInputFixture(t)
			input.SetText("a")
			input.im.emitCommit(IMCommit{Text: "中"})
			if obscuredLayoutText(input) != "•中" {
				t.Fatal("取消前没有显露")
			}
			timer := input.obscureHint.timer
			switch action {
			case "selection":
				input.SetSelection(TextSelection{})
			case "arrow":
				editorKey(t, win, events.KeyArrowLeft, 0)
			case "focus":
				win.SetFocusedWidget(nil)
			case "window_focus":
				_ = win.DispatchEvent(events.FocusEvent{Focused: false})
			case "disable":
				input.SetEnabled(false)
			case "parent_disable":
				input.Parent().SetEnabled(false)
			case "hide":
				input.SetVisible(false)
			case "unmount":
				win.SetWidget(nil)
			case "destroy":
				input.WidgetBase.destroy(input)
			case "read_only":
				input.SetReadOnly(true)
			case "zero_delay":
				input.SetObscureDelay(-time.Second)
			case "changed_delay":
				input.SetObscureDelay(time.Second)
			case "character":
				input.SetObscureChar('*')
			case "toggle":
				input.SetObscured(false)
				input.SetObscured(true)
			case "model":
				input.SetText("other")
			case "preedit":
				input.im.emitPreedit("候选", 3)
			}
			if timer.Active() || input.obscureHint != nil && input.obscureHint.span != (TextRange{}) {
				t.Fatal("取消后仍保留计时或可见范围")
			}
			advanceObscureTimer(t, q, time.Second)
			if !input.Destroyed() && !input.suspended {
				got := obscuredLayoutText(input)
				if strings.ContainsAny(got, "a中候选other") {
					t.Fatal("取消后仍显示正文", got)
				}
			}
		})
	}
}

func TestTextInputObscureDelayEditOrigins(t *testing.T) {
	for _, action := range []string{"default", "multi", "paste", "programmatic", "history", "reject", "transform_many", "truncate_multi"} {
		t.Run(action, func(t *testing.T) {
			input, win, _ := obscuredInputFixture(t)
			switch action {
			case "default":
				input.SetObscureDelay(0)
				input.im.emitCommit(IMCommit{Text: "x"})
			case "multi":
				input.im.emitCommit(IMCommit{Text: "xy", Composed: true})
			case "paste":
				clip := &editorClipboard{}
				App.(*application).clipboard = clip
				editorKey(t, win, events.KeyV, textCommandModifier())
				clip.pending("x", nil)
			case "programmatic":
				input.SetText("x")
			case "history":
				input.im.emitCommit(IMCommit{Text: "x"})
				editorKey(t, win, events.KeyZ, textCommandModifier())
				editorKey(t, win, events.KeyZ, textCommandModifier()|events.ModifierShift)
			case "reject":
				input.SetInputFilter(textedit.DigitsOnly())
				input.im.emitCommit(IMCommit{Text: "x"})
			case "transform_many":
				input.SetInputFilter(func(textedit.Input) (string, bool) { return "xy", true })
				input.im.emitCommit(IMCommit{Text: "x"})
			case "truncate_multi":
				input.SetInputFilter(textedit.LimitLength(1))
				input.im.emitCommit(IMCommit{Text: "xy"})
			}
			if input.obscureHint != nil && input.obscureHint.span != (TextRange{}) {
				t.Fatal("非单字素用户提交触发了显露")
			}
			if action == "default" && input.obscureHint != nil {
				t.Fatal("默认关闭仍分配了反馈计时器")
			}
		})
	}
}

func TestTextInputObscureDelayCompositionReentryAndUnavailableTimer(t *testing.T) {
	input, _, q := obscuredInputFixture(t)
	input.SetText("e")
	input.im.emitCommit(IMCommit{Text: "́"})
	if obscuredLayoutText(input) != "é" {
		t.Fatal("连接到前一个字符的组合符没有显露完整字素")
	}
	input.im.emitPreedit("候", 3)
	if obscuredLayoutText(input) != "••" {
		t.Fatal("预编辑没有遮蔽")
	}
	input.im.emitCommit(IMCommit{Text: "中", Composed: true})
	if obscuredLayoutText(input) != "•中" {
		t.Fatal("单字素 IME 最终提交没有显露")
	}
	input.ConnectText(func(string) { input.SetSelection(TextSelection{}) })
	input.im.emitCommit(IMCommit{Text: "x"})
	if input.obscureHint.span != (TextRange{}) {
		t.Fatal("选区回调取消后又显露了过期输入")
	}
	// 新控件使用已停止的应用计时器，尽力反馈失败应保持立即遮蔽。
	q.s.close()
	other := NewTextInput()
	input.Parent().(*LinearBox).AddChild(other)
	other.SetObscured(true)
	other.SetObscureDelay(time.Second)
	other.Arrange(geometry.Rect(0, 40, 240, 36))
	input.Window().SetFocusedWidget(other)
	other.im.emitCommit(IMCommit{Text: "x"})
	if other.obscureHint.timer.Active() || obscuredLayoutText(other) != "•" {
		t.Fatal("计时器失败后仍显露输入")
	}
}

func TestTextInputObscureTimeoutKeepsPendingPasteAndHandlesDestruction(t *testing.T) {
	t.Run("pending_paste", func(t *testing.T) {
		input, win, q := obscuredInputFixture(t)
		input.im.emitCommit(IMCommit{Text: "x"})
		clip := &editorClipboard{}
		App.(*application).clipboard = clip
		editorKey(t, win, events.KeyV, textCommandModifier())
		if clip.pending == nil {
			t.Fatal("未发起剪贴板读取")
		}
		advanceObscureTimer(t, q, 600*time.Millisecond)
		clip.pending("y", nil)
		if input.Text() != "xy" || obscuredLayoutText(input) != "••" {
			t.Fatal("遮蔽计时使正常粘贴失效，或粘贴显露正文")
		}
	})
	t.Run("destroy_callback", func(t *testing.T) {
		input, _, q := obscuredInputFixture(t)
		input.im.emitCommit(IMCommit{Text: "x"})
		timer := input.obscureHint.timer
		input.paragraph(0).layout.ConnectDestroy(func() { input.WidgetBase.destroy(input) })
		advanceObscureTimer(t, q, 600*time.Millisecond)
		if !input.Destroyed() || timer.Active() || input.obscureHint != nil {
			t.Fatal("超时释放排版的回调销毁控件后仍保留计时或显露状态")
		}
	})
}

func TestTextInputObscureCancelPreeditStopsAfterLayoutCallback(t *testing.T) {
	for _, readOnly := range []bool{false, true} {
		t.Run(fmt.Sprint(readOnly), func(t *testing.T) {
			input, _, _ := obscuredInputFixture(t)
			input.im.emitCommit(IMCommit{Text: "x"})
			input.paragraph(0).layout.ConnectDestroy(func() {
				if readOnly {
					input.SetReadOnly(true)
				} else {
					input.SetEnabled(false)
				}
			})
			input.im.emitPreedit("候选", 3)
			if input.preedit != nil || input.obscureHint.timer.Active() {
				t.Fatal("停止显露的销毁回调改变可编辑状态后，继续了过期预编辑")
			}
		})
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
	if got := input.Padding(); got != (geometry.Insets{Left: 8, Top: 4, Right: 8, Bottom: 4}) {
		t.Fatalf("default text input padding = %+v", got)
	}

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
	input.SetSelection(TextSelection{Anchor: len("ab"), Caret: len("ab")})
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
	if call.text != "abc" || call.width != textInputMeasureExtent || call.height != textInputMeasureExtent || painter.clipRect != geometry.Rect(8, 4, 84, 16) {
		t.Fatalf("unexpected text layout call: %+v", call)
	}
	if painter.textOrigin != (geometry.Point{X: 8, Y: 4}) {
		t.Fatalf("unexpected text origin: %+v", painter.textOrigin)
	}
	if painter.textLayout != typo.layouts[1] {
		t.Fatal("painter did not receive text input layout")
	}
	// With caching, the layout is NOT destroyed after Paint — it lives until unmount/setter.
	if typo.layouts[1].destroyed {
		t.Fatal("paint should cache text layout for reuse")
	}
	if len(painter.fills) != 2 || painter.fills[1] != geometry.Rect(28, 5, 1, 18) {
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
	if len(painter.fills) != 2 || painter.fills[1] != geometry.Rect(8, 4, 1, input.lineHeight) || painter.clipRect != geometry.Rect(8, 4, 84, 16) {
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
	input.SetPadding(geometry.UniformInsets(6)) // padding is a layout field, not style
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
	input.SetSelection(TextSelection{Anchor: len("a"), Caret: len("a")})

	input.onCommit(IMCommit{Text: "b"})

	if input.Text() != "abc" {
		t.Fatalf("unexpected text: %q", input.Text())
	}
	if input.Selection().Caret != len("ab") {
		t.Fatalf("unexpected caret: %d", input.Selection().Caret)
	}
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
	input.SetSelection(TextSelection{Anchor: 1, Caret: 3})
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "中\n文", Caret: 4})
	composition, rev := input.preedit, input.model.Revision()
	input.SetText("a\r\nb\rc\nd")
	if input.preedit != composition || input.Selection() != (TextSelection{Anchor: 1, Caret: 3}) || input.model.Revision() != rev {
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
	if input.Text() != "a b c d" || input.Selection() != (TextSelection{Anchor: 1, Caret: 3}) {
		t.Fatal("single undo did not restore replacement and selection")
	}
	input.SetText("new")
	if input.model.CanUndo() || input.model.CanRedo() || input.Selection() != (TextSelection{Anchor: 3, Caret: 3}) {
		t.Fatal("changed SetText did not reset history/caret")
	}
}

func TestTextInputHorizontalRevealAndStableGeometry(t *testing.T) {
	input, win, typo := newInputFixture(t, strings.Repeat("x", 100))
	before := input.Measure(layout.Unbounded())
	caret, _ := input.caretRect()
	if input.offset.X <= 0 || caret.X-input.offset.X < input.padding.Left || caret.X+caret.Width-input.offset.X > input.Rect().Width-input.padding.Right {
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
	if info.Role != RoleTextInput || info.Text != "abc" || info.TextEditing == nil || !info.TextEditing.ReadOnly || info.TextEditing.Selection != (TextSelection{Anchor: 0, Caret: 3}) {
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
	input.SetSelection(TextSelection{Anchor: 1, Caret: 2})
	dispatchKey(t, win, events.KeyV, textCommandModifier())
	clip.pending("1\r\n2\n3", nil)
	if input.Text() != "a1 2 3c" {
		t.Fatalf("paste = %q", input.Text())
	}
	dispatchKey(t, win, events.KeyZ, textCommandModifier())
	if input.Text() != "abc" || input.Selection() != (TextSelection{Anchor: 1, Caret: 2}) {
		t.Fatal("paste undo split or lost selection")
	}
	dispatchKey(t, win, events.KeyV, textCommandModifier())
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "x", Caret: 1})
	clip.pending("stale", nil)
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
	input.SetSelection(TextSelection{Anchor: 1, Caret: 1})
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
			input.SetSelection(TextSelection{Anchor: 1, Caret: 1})
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

// Input focus chrome.

func TestTextInputFocusVisibleKeepsOwnBorder(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			_, win, _ := newEditorFixture(t, "")
			backend, err := software.NewPainter(&widgetRenderSurface{})
			if err != nil {
				t.Fatal(err)
			}
			defer backend.Destroy()
			win.painter = backend
			input := NewTextInput()
			win.SetWidget(input)
			defer win.SetWidget(nil)
			input.Arrange(geometry.Rect(0, 0, 80, 30))
			border := color.RGBA{R: 70, G: 130, B: 220, A: 255}
			App.SetStyleSheet(style.Sheet(append(DefaultStyleRules(),
				style.Name("text-input").State(style.Focused).BorderColor(border).BorderWidth(2),
				// Deliberately conflicting rules must not add a second focus pass
				// or replace the editor's existing Focused border.
				style.Name("text-input").State(style.FocusVisible).BorderColor(color.Black).BorderWidth(6),
				style.Name("text-input").Part("focus").State(style.FocusVisible).BorderColor(color.Black).BorderWidth(6),
			)...))
			win.SetFocusedWidget(input)
			for _, keyboard := range []bool{false, true} {
				if keyboard {
					editorKey(t, win, events.KeyTab, 0)
				}
				if input.FocusVisible() != keyboard {
					t.Fatalf("unexpected input mode: keyboard=%v hint=%v", keyboard, input.FocusVisible())
				}
				img, err := RenderWidget(input, scale)
				if err != nil {
					t.Fatal(err)
				}
				// Straight top edge at 0/1 DIP must be exactly the opaque 2 DIP
				// editor border; 2 DIP and the center must stay white. No glyphs,
				// corners or caret are sampled; premultiplied sRGB bytes, no tolerance.
				for _, y := range []int{0, 1, 2, 15} {
					want := border
					if y >= 2 {
						want = color.RGBA{R: 255, G: 255, B: 255, A: 255}
					}
					got := color.RGBAModel.Convert(img.At(40*int(scale), y*int(scale))).(color.RGBA)
					if got != want {
						t.Fatalf("keyboard=%v y=%d DIP pixel=%v want=%v", keyboard, y, got, want)
					}
				}
			}
		})
	}
}

// Horizontal caret reveal.

func TestTextInputCaretRevealReusesMetrics(t *testing.T) {
	_, _, win, typo := longWrappedEditor(t)
	input := NewTextInput()
	input.SetText(strings.Repeat("abcdefghij", 100))
	win.SetWidget(input)
	win.SetFocusedWidget(input)
	settleEditorLayout(t, win)
	p := input.paragraphs[0].layout.(*testTextLayout)
	calls, layouts := p.metricsCalls, len(typo.calls)
	for _, key := range []events.Key{events.KeyHome, events.KeyEnd, events.KeyArrowLeft} {
		editorKey(t, win, key, 0)
		if win.layoutDirty || p.metricsCalls != calls || len(typo.calls) != layouts {
			t.Fatal("single-line navigation remeasured text")
		}
		rect, ok := input.caretRect()
		if !ok || rect.X < input.offset.X || rect.X+rect.Width > input.offset.X+input.viewport.Width {
			t.Fatal("single-line caret reveal lost horizontal scrolling")
		}
	}
}

func TestTextInputAsymmetricPadding(t *testing.T) {
	typo := &testTypography{
		measureSize: geometry.Size{Width: 30, Height: 18},
		lines: []typography.TextLine{
			{Start: 0, Length: len(textInputHeightSample), Width: 30, Height: 18, Baseline: 14},
		},
	}
	setTestApplication(t, typo)
	input := NewTextInput()
	input.SetPadding(geometry.Insets{Left: 10, Top: 2, Right: 6, Bottom: 5})
	input.SetText("ab")

	measured := input.Measure(layout.Loose(geometry.Size{Width: 300, Height: 100}))
	if measured.Size.Height != 25 || !measured.HasBaseline || measured.Baseline != 16 {
		t.Fatalf("asymmetric text input measure = %+v", measured)
	}
	input.Arrange(geometry.Rect(0, 0, 100, 25))

	painter := new(testTextInputPainter)
	input.Paint(painter)
	if painter.textOrigin != (geometry.Point{X: 10, Y: 2}) {
		t.Fatalf("asymmetric text origin = %+v", painter.textOrigin)
	}
	if painter.clipRect != geometry.Rect(10, 2, 84, 18) {
		t.Fatalf("asymmetric text clip = %+v", painter.clipRect)
	}
}

// 掩码排版只包含替代字符，几何使用原文 UTF-8 坐标而非掩码字节坐标。
func TestTextInputObscuredGeometryHistoryAndSnapshot(t *testing.T) {
	typo := &editorTypography{}
	setTestApplication(t, typo)
	input := NewTextInput()
	input.SetObscureChar('*')
	input.SetObscured(true)
	secret := "é中👨‍👩‍👧‍👦"
	input.SetText(secret)
	win := &window{}
	win.SetWidget(input)
	t.Cleanup(func() { win.SetWidget(nil) })
	input.Arrange(geometry.Rect(0, 0, 160, 32))
	win.SetFocusedWidget(input)
	input.SetSelection(TextSelection{})
	input.Paint(&testTextInputPainter{})
	p := input.paragraph(0)
	if p.layout.Text() != "***" || input.Text() != secret {
		t.Fatal("原文送入了排版，或模型被替换")
	}
	for _, end := range []int{3, 6, len(secret)} {
		editorKey(t, win, events.KeyArrowRight, 0)
		if input.Selection().Caret != end {
			t.Fatal("掩码命中未映射回原文边界", input.Selection(), end)
		}
	}
	editorKey(t, win, events.KeyBackspace, 0)
	if input.Text() != "é中" {
		t.Fatal("删除拆开了 Emoji 字素", input.Text())
	}
	editorKey(t, win, events.KeyZ, textCommandModifier())
	if input.Text() != secret {
		t.Fatal("掩码编辑历史没有保存原文")
	}
	input.SetSelection(TextSelection{Anchor: 3, Caret: 6})
	snapshot := input.Snapshot()
	if snapshot.Text != "" || snapshot.TextEditing.VisibleText != "" || !snapshot.TextEditing.Obscured || snapshot.TextEditing.Length != len(secret) || snapshot.TextEditing.Selection != input.Selection() {
		t.Fatal("掩码快照泄露正文或破坏源坐标", snapshot)
	}
	selection := input.Selection()
	revision := input.model.Revision()
	native := input.paragraph(0).layout.(*testTextLayout)
	input.SetObscured(false)
	if !native.destroyed || input.model.Revision() != revision || input.Selection() != selection || !input.model.CanRedo() {
		t.Fatal("切换模式未释放排版，或改变模型／历史／选区")
	}
	input.Paint(&testTextInputPainter{})
	if input.paragraph(0).layout.Text() != secret || input.Snapshot().Text != secret {
		t.Fatal("退出遮蔽未恢复普通语义")
	}
}

// 密码模式禁止复制和剪切；预编辑同样遮蔽且候选位置来自映射几何。
func TestTextInputObscuredClipboardAndPreedit(t *testing.T) {
	setTestApplication(t, &editorTypography{})
	clip := &editorClipboard{text: "keep"}
	App.(*application).clipboard = clip
	input := NewTextInput()
	input.SetObscured(true)
	input.SetText("é中")
	win := &window{}
	win.SetWidget(input)
	t.Cleanup(func() { win.SetWidget(nil) })
	input.Arrange(geometry.Rect(0, 0, 160, 32))
	win.SetFocusedWidget(input)
	input.SetSelection(TextSelection{Anchor: 0, Caret: len(input.Text())})
	for _, key := range []events.Key{events.KeyC, events.KeyX} {
		editorKey(t, win, key, textCommandModifier())
		if clip.text != "keep" || input.Text() != "é中" {
			t.Fatal("遮蔽控件执行了复制／剪切")
		}
	}
	input.SetSelection(TextSelection{Anchor: 6, Caret: 6})
	input.im.emitPreedit("候选", len("候"))
	input.Paint(&testTextInputPainter{})
	if input.paragraph(0).layout.Text() != "••••" {
		t.Fatal("预编辑没有遮蔽")
	}
	caret, ok := input.caretRect()
	if !ok || caret.X != input.Padding().Left+30 {
		t.Fatal("原生候选位置不在掩码预编辑光标处", caret)
	}
	info := input.Snapshot().TextEditing
	if info.Preedit == nil || info.Preedit.Text != "" || info.Preedit.Caret != 3 || info.VisibleText != "" {
		t.Fatal("快照暴露预编辑文本或偏移错误", info)
	}
	input.im.emitCommit(IMCommit{Text: "文", Composed: true})
	if input.Text() != "é中文" {
		t.Fatal("组合提交未保留真实文本")
	}
	input.SetInputFilter(textedit.DigitsOnly())
	editorKey(t, win, events.KeyV, textCommandModifier())
	clip.pending("12", nil)
	if input.Text() != "é中文12" {
		t.Fatal("遮蔽控件不能粘贴")
	}
}
