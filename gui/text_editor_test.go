package gui

import (
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/typography"
)

// 共用排版、剪贴板替身与事件入口。

// Fixture typography has 10 DIP cells and 20 DIP lines. It is deliberately
// independent of editor geometry; shaping/bidi contracts have separate tests.
type editorTypography struct{ testTypography }

func (c *editorTypography) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, _ := c.testTypography.NewTextLayout(text, format, width, height)
	p := l.(*testTextLayout)
	line := typography.TextLine{Height: 20, Baseline: 15}
	x, y := float32(0), float32(0)
	for offset, r := range text {
		if x+10 > width && x > 0 && format.WrapMode != WrapNone {
			line.Length = offset - line.Start
			line.Width = x
			p.lines = append(p.lines, line)
			x, y = 0, y+20
			line = typography.TextLine{Start: offset, Y: y, Height: 20, Baseline: y + 15}
		}
		cluster := typography.TextCluster{Start: offset, Length: utf8.RuneLen(r), X: x, Y: y, Width: 10, Height: 20, LineIndex: len(p.lines)}
		p.clusters = append(p.clusters, cluster)
		x += 10
	}
	line.Length = len(text) - line.Start
	line.Width = x
	p.lines = append(p.lines, line)
	p.measureSize = geometry.Size{Width: x, Height: y + 20}
	return p, nil
}

func newEditorFixture(t *testing.T, text string) (*TextView, *window, *editorTypography) {
	t.Helper()
	typo := &editorTypography{}
	setTestApplication(t, typo)
	editor := NewTextView()
	editor.SetModel(NewTextModel(text))
	win := &window{}
	win.SetWidget(editor)
	editor.Arrange(geometry.Rect(0, 0, 209, 108))
	win.SetFocusedWidget(editor)
	t.Cleanup(func() { win.SetWidget(nil) })
	return editor, win, typo
}

func editorKey(t *testing.T, win *window, key events.Key, modifiers events.Modifiers) {
	t.Helper()
	if err := win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: key, Modifiers: modifiers}); err != nil {
		t.Fatal(err)
	}
}

type editorClipboard struct {
	text    string
	pending func(string, bool)
}

func (c *editorClipboard) SetText(s string) { c.text = s }

func (c *editorClipboard) RequestText(fn func(string, bool)) { c.pending = fn }

// 导航、选择、剪贴板与历史。

func TestTextViewKeysSelectionNativeCommitAndReadonly(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "ab\ncd")
	editor.SetSelection(TextSelection{1, 1})
	editorKey(t, win, events.KeyArrowRight, events.ModifierShift)
	if editor.Selection() != (TextSelection{1, 2}) {
		t.Fatalf("shift selection %+v", editor.Selection())
	}
	editor.im.emitCommit(IMCommit{Text: "中"})
	if editor.model.Text() != "a中\ncd" || editor.selection != (TextSelection{4, 4}) {
		t.Fatalf("commit result %q %+v", editor.model.Text(), editor.selection)
	}
	editorKey(t, win, events.KeyDelete, 0)
	if editor.model.Text() != "a中cd" {
		t.Fatal("delete did not join paragraphs")
	}
	editorKey(t, win, events.KeyEnter, 0)
	if editor.model.Text() != "a中\ncd" {
		t.Fatal("enter did not insert LF")
	}
	editor.SetReadOnly(true)
	if editor.IMContext() != nil || win.activeIM != nil {
		t.Fatal("read-only still enables IME")
	}
	editorKey(t, win, events.KeyBackspace, 0)
	editor.im.emitCommit(IMCommit{Text: "NO"})
	if editor.model.Text() != "a中\ncd" {
		t.Fatal("read-only changed document")
	}
	editorKey(t, win, events.KeyHome, textCommandModifier())
	if editor.selection.Caret != 0 {
		t.Fatal("read-only navigation disabled")
	}
	editor.SetReadOnly(false)
	if win.activeIM != editor.im {
		t.Fatal("editable IME not rebound")
	}
}

func TestTextViewPointerCaptureAndVerticalColumn(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "abc\nx\nabc")
	for _, e := range []events.PointerEvent{
		{EventType: events.PointerDown, Position: geometry.Point{X: 6, Y: 8}, Button: events.PointerButtonLeft},
		{EventType: events.PointerMove, Position: geometry.Point{X: 207, Y: 8}, Buttons: events.PointerButtonLeftDown},
		{EventType: events.PointerUp, Position: geometry.Point{X: 207, Y: 8}, Button: events.PointerButtonLeft},
	} {
		if err := win.DispatchEvent(e); err != nil {
			t.Fatal(err)
		}
	}
	if editor.selection != (TextSelection{0, 3}) || editor.input.CapturingPointer() {
		t.Fatalf("drag selection %+v", editor.selection)
	}
	editor.SetSelection(TextSelection{3, 3})
	editorKey(t, win, events.KeyArrowDown, 0)
	if editor.selection.Caret != 5 {
		t.Fatalf("short line caret=%d", editor.selection.Caret)
	}
	editorKey(t, win, events.KeyArrowDown, 0)
	if editor.selection.Caret != 9 {
		t.Fatalf("desired X not retained: %d", editor.selection.Caret)
	}
}

func TestTextViewPasteDropsStaleReply(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "abc")
	clip := &editorClipboard{}
	App.(*application).clipboard = clip
	editor.SetSelection(TextSelection{1, 2})
	editorKey(t, win, events.KeyV, textCommandModifier())
	editor.SetSelection(TextSelection{3, 3})
	clip.pending("wrong", true)
	if editor.model.Text() != "abc" {
		t.Fatal("stale selection received paste")
	}
	editorKey(t, win, events.KeyV, textCommandModifier())
	clip.pending("\r\nX", true)
	if editor.model.Text() != "abc\nX" {
		t.Fatalf("paste not normalized: %q", editor.model.Text())
	}
	editorKey(t, win, events.KeyV, textCommandModifier())
	win.SetWidget(nil)
	win.SetWidget(editor)
	clip.pending("wrong", true)
	if editor.model.Text() != "abc\nX" {
		t.Fatal("paste survived unmount/remount")
	}
}

func TestTextViewUndoRestoresInvokerSelectionAndSplitsEditors(t *testing.T) {
	a, win, _ := newEditorFixture(t, "ab")
	b := NewTextView()
	b.SetModel(a.model)
	other := &window{}
	other.SetWidget(b)
	b.Arrange(geometry.Rect(0, 0, 209, 108))
	t.Cleanup(func() { other.SetWidget(nil) })
	a.SetSelection(TextSelection{2, 2})
	b.SetSelection(TextSelection{2, 2})
	a.im.emitCommit(IMCommit{Text: "x"})
	a.im.emitCommit(IMCommit{Text: "y"})
	b.im.emitCommit(IMCommit{Text: "z"})
	editorKey(t, win, events.KeyZ, textCommandModifier())
	if a.model.Text() != "abxy" || a.selection != (TextSelection{4, 4}) {
		t.Fatalf("undo from other view: %q %+v", a.model.Text(), a.selection)
	}
	b.SetSelection(TextSelection{0, 0})
	editorKey(t, win, events.KeyZ, textCommandModifier())
	if a.model.Text() != "ab" || a.selection != (TextSelection{2, 2}) || b.selection != (TextSelection{}) {
		t.Fatalf("undo restored wrong view: %q %+v %+v", a.model.Text(), a.selection, b.selection)
	}
	if a.model.CanUndo() {
		t.Fatal("editing views should produce exactly two undo steps")
	}
	editorKey(t, win, events.KeyZ, textCommandModifier()|events.ModifierShift)
	if a.model.Text() != "abxy" || a.selection != (TextSelection{4, 4}) {
		t.Fatal("redo did not restore after-selection")
	}
}

func TestTextViewReentrantModelAndSelectionCallbacks(t *testing.T) {
	typo := &editorTypography{}
	setTestApplication(t, typo)
	m := NewTextModel("a")
	m.ConnectChange(func(change TextChange) {
		if change.Revision == 1 {
			_ = m.Replace(TextRange{0, 0}, "Y")
		}
	})
	editor := NewTextView()
	editor.SetModel(m)
	win := &window{}
	win.SetWidget(editor)
	t.Cleanup(func() { win.SetWidget(nil) })
	editor.Arrange(geometry.Rect(0, 0, 209, 108))
	editor.SetSelection(TextSelection{1, 1})
	editor.im.emitCommit(IMCommit{Text: "X"})
	if m.Text() != "YaX" || editor.selection != (TextSelection{3, 3}) {
		t.Fatalf("reentrant model: %q %+v", m.Text(), editor.selection)
	}
	editor.ConnectSelection(func(TextSelection) { win.SetWidget(nil) })
	editor.im.emitCommit(IMCommit{Text: "Z"})
	if editor.modelHandle != nil || !editor.suspended {
		t.Fatal("selection callback unmount was ignored")
	}
}

// 双向文本与视觉导航。

// Explicit geometry, not editor-derived expectations: Hebrew occupies x=10..30
// in reverse byte order. Ordinary text uses the existing 10 DIP fixture.
type bidiEditorTypography struct{ editorTypography }

func (c *bidiEditorTypography) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, err := c.editorTypography.NewTextLayout(text, format, width, height)
	if text != "aאבc" && text != "אב" {
		return l, err
	}
	p := l.(*testTextLayout)
	start, x := 0, float32(0)
	p.clusters = nil
	if text == "aאבc" {
		start, x = 1, 10
		p.clusters = append(p.clusters, typography.TextCluster{Length: 1, Width: 10, Height: 20})
	}
	p.clusters = append(p.clusters,
		typography.TextCluster{Start: start, Length: 2, X: x + 10, Width: 10, Height: 20, Direction: typography.TextRightToLeft},
		typography.TextCluster{Start: start + 2, Length: 2, X: x, Width: 10, Height: 20, Direction: typography.TextRightToLeft})
	if text == "aאבc" {
		p.clusters = append(p.clusters, typography.TextCluster{Start: 5, Length: 1, X: 30, Width: 10, Height: 20})
	}
	return p, err
}

func TestTextEditorCollapseSelectionVisually(t *testing.T) {
	for _, single := range []bool{false, true} {
		name := "multiline"
		if single {
			name = "singleline"
		}
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				name, text  string
				selection   TextSelection
				left, right int
			}{
				{"rtl", "אב", TextSelection{0, 4}, 4, 0},
				{"rtl-reverse-selection", "אב", TextSelection{4, 0}, 4, 0},
				{"embedded-rtl", "aאבc", TextSelection{1, 5}, 5, 1},
				{"disjoint-selection", "aאבc", TextSelection{0, 3}, 0, 3},
				{"ltr", "abcd", TextSelection{1, 3}, 1, 3},
			} {
				t.Run(tc.name, func(t *testing.T) {
					setTestApplication(t, &bidiEditorTypography{})
					var widget Widget
					var editor *textEditor
					if single {
						input := NewTextInput()
						input.SetText(tc.text)
						widget, editor = input, input.textEditor
					} else {
						view := NewTextView()
						view.SetModel(NewTextModel(tc.text))
						widget, editor = view, view.textEditor
					}
					win := &window{}
					win.SetWidget(widget)
					t.Cleanup(func() { win.SetWidget(nil) })
					widget.Arrange(geometry.Rect(0, 0, 200, 80))
					win.SetFocusedWidget(widget)
					for _, direction := range []struct {
						key  events.Key
						want int
					}{{events.KeyArrowLeft, tc.left}, {events.KeyArrowRight, tc.right}} {
						editor.setSelectionValue(tc.selection)
						editorKey(t, win, direction.key, 0)
						if editor.selection != (TextSelection{direction.want, direction.want}) {
							t.Fatalf("key %v collapsed to %+v, want %d", direction.key, editor.selection, direction.want)
						}
					}
				})
			}
		})
	}
}

func TestTextEditorCollapseSelectionAcrossLines(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		selection  TextSelection
	}{
		{"paragraphs", "abcd\ne", TextSelection{3, 6}},
		{"soft-wrap", "abcdef", TextSelection{3, 5}},
		{"soft-wrap-boundary", "abcdefgh", TextSelection{4, 8}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			editor, win, _ := newEditorFixture(t, tc.text)
			editor.Arrange(geometry.Rect(0, 0, 48, 100)) // four 10 DIP cells plus padding
			for _, key := range []events.Key{events.KeyArrowLeft, events.KeyArrowRight} {
				editor.SetSelection(tc.selection)
				editorKey(t, win, key, 0)
				want := tc.selection.Anchor
				if key == events.KeyArrowRight {
					want = tc.selection.Caret
				}
				if editor.Selection() != (TextSelection{want, want}) {
					t.Fatalf("key %v = %+v, want %d", key, editor.Selection(), want)
				}
			}
		})
	}
}

// 预编辑提交与撤销分组。

// XIM can clear/end preedit before delivering its commit in a later key event.
// Keep that commit separate from ordinary typing on both sides, while still
// merging adjacent ordinary keystrokes. Exercise the window's actual IM route.
func TestTextEditorPreeditDoneBeforeCommitUndoGroups(t *testing.T) {
	for _, single := range []bool{false, true} {
		name := "multiline"
		if single {
			name = "singleline"
		}
		t.Run(name, func(t *testing.T) {
			var editor *textEditor
			var win *window
			if single {
				input, w, _ := newInputFixture(t, "")
				editor, win = input.textEditor, w
			} else {
				view, w, _ := newEditorFixture(t, "")
				editor, win = view.textEditor, w
			}
			commit := func(text string) {
				win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: text})
			}
			commit("a")
			commit("b")
			win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "你", Caret: 3})
			win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit})
			if editor.model.Text() != "ab" {
				t.Fatal("preedit changed committed text")
			}
			commit("你")
			commit("x")
			commit("y")
			if editor.model.Text() != "ab你xy" {
				t.Fatal("commit was lost")
			}
			for _, want := range []string{"ab你", "ab", ""} {
				editorKey(t, win, events.KeyZ, textCommandModifier())
				if got := editor.model.Text(); got != want {
					t.Fatalf("undo: %q want %q", got, want)
				}
			}
		})
	}
}

func TestTextEditorCommitWithoutPreeditUndoGroups(t *testing.T) {
	for _, single := range []bool{false, true} {
		name := "multiline"
		if single {
			name = "singleline"
		}
		t.Run(name, func(t *testing.T) {
			var editor *textEditor
			var win *window
			if single {
				input, w, _ := newInputFixture(t, "")
				editor, win = input.textEditor, w
			} else {
				view, w, _ := newEditorFixture(t, "")
				editor, win = view.textEditor, w
			}
			commit := func(text string, composed bool) {
				win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: text, Composed: composed})
			}
			commit("a", false)
			commit("中", false)   // Unicode itself does not imply composition
			commit("word", true) // ASCII can be a native composition/service result
			commit("你", true)
			commit("x", false)
			commit("y", false)
			if editor.model.Text() != "a中word你xy" {
				t.Fatal("native text lost")
			}
			for _, want := range []string{"a中word你", "a中word", "a中", ""} {
				editorKey(t, win, events.KeyZ, textCommandModifier())
				if got := editor.model.Text(); got != want {
					t.Fatalf("undo = %q want %q", got, want)
				}
			}
			for _, want := range []string{"a中", "a中word", "a中word你", "a中word你xy"} {
				editorKey(t, win, events.KeyZ, textCommandModifier()|events.ModifierShift)
				if got := editor.model.Text(); got != want {
					t.Fatalf("redo = %q want %q", got, want)
				}
			}
		})
	}
}

// 光标计时器与共用计时替身。

func timedEditorFixture(t *testing.T) (*TextView, *ScrollView, *window, *timerTestQueue, *editorTypography) {
	t.Helper()
	q := newTimerTestQueue(t)
	typo := &editorTypography{}
	q.app.typo = typo
	old := App
	App = q.app
	t.Cleanup(func() { App = old })
	editor := NewTextView()
	editor.SetModel(NewTextModel(strings.Repeat("row\n", 100)))
	scroll := NewScrollView()
	scroll.SetChild(editor)
	win := &window{}
	win.SetWidget(scroll)
	t.Cleanup(func() { win.SetWidget(nil) })
	box := geometry.Rect(0, 0, 220, 108)
	scroll.Measure(layout.Tight(box.Size))
	scroll.Arrange(box)
	win.SetFocusedWidget(editor)
	if err := win.DispatchEvent(events.FocusEvent{Focused: true}); err != nil {
		t.Fatal(err)
	}
	return editor, scroll, win, q, typo
}

func TestTextViewBlinkOnlyPaintsAndStopsWithFocus(t *testing.T) {
	editor, _, win, q, typo := timedEditorFixture(t)
	if editor.blinkTimer == nil || !editor.blinkTimer.Active() {
		t.Fatal("focused view did not start blink")
	}
	count := len(typo.calls)
	win.layoutDirty, win.paintDirty = false, false
	q.advance(textCaretInterval)
	q.dispatch(t)
	if editor.caretVisible || !win.paintDirty || win.layoutDirty || len(typo.calls) != count {
		t.Fatal("blink changed layout or failed to request paint")
	}
	editorKey(t, win, events.KeyArrowRight, 0)
	if !editor.caretVisible {
		t.Fatal("navigation did not show caret immediately")
	}
	q.advance(textCaretInterval)
	q.dispatch(t)
	if editor.caretVisible {
		t.Fatal("restarted blink did not fire")
	}
	_ = win.DispatchEvent(events.FocusEvent{Focused: false})
	if editor.blinkTimer.Active() {
		t.Fatal("background window kept blinking")
	}
	_ = win.DispatchEvent(events.FocusEvent{Focused: true})
	if !editor.blinkTimer.Active() || !editor.caretVisible {
		t.Fatal("returning window focus did not resume steady-to-blink cycle")
	}
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "compose", Caret: 7})
	if editor.blinkTimer.Active() || !editor.caretVisible {
		t.Fatal("IME caret must remain steady")
	}
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit})
	if !editor.blinkTimer.Active() {
		t.Fatal("cancel did not resume blinking")
	}
	win.SetWidget(nil)
	if editor.blinkTimer != nil || editor.scrollTimer != nil || editor.windowFocusHandle != nil || len(q.s.queue) != 0 {
		t.Fatal("unmount retained timers/connections")
	}
}

// 原生排版共享验证（入口保留平台后缀）。

// Native shaping without a display: verify editing through the normal dispatcher
// using real native clusters. Assertions are byte boundaries and visual ordering,
// not font-dependent pixel widths. This is not a native window/IME acceptance test.
func testTextEditorNativeNavigation(t *testing.T, newContext func() (typography.Context, error)) {
	t.Helper()
	for _, single := range []bool{false, true} {
		name := "multiline"
		if single {
			name = "singleline"
		}
		t.Run(name, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			typo, err := newContext()
			if err != nil {
				t.Fatal(err)
			}
			defer typo.Destroy()
			setTestApplication(t, typo)
			var widget Widget
			var editor *textEditor
			if single {
				input := NewTextInput()
				input.SetText("אבג")
				widget, editor = input, input.textEditor
			} else {
				view := NewTextView()
				view.SetModel(NewTextModel("אבג"))
				widget, editor = view, view.textEditor
			}
			win := &window{}
			win.SetWidget(widget)
			defer win.SetWidget(nil) // all native layout cleanup precedes UnlockOSThread
			widget.Arrange(geometry.Rect(0, 0, 200, 80))
			win.SetFocusedWidget(widget)
			editor.setSelectionValue(TextSelection{0, 6})
			editorKey(t, win, events.KeyArrowLeft, 0)
			if editor.selection != (TextSelection{6, 6}) {
				t.Fatalf("RTL selection collapsed left to %+v, want byte 6", editor.selection)
			}
			before, _ := editor.caretRect()
			editorKey(t, win, events.KeyArrowRight, events.ModifierShift)
			after, _ := editor.caretRect()
			if editor.selection != (TextSelection{6, 4}) || after.X <= before.X {
				t.Fatalf("RTL Shift+Right: selection=%+v x=%g -> %g", editor.selection, before.X, after.X)
			}
			editorKey(t, win, events.KeyBackspace, 0)
			if editor.model.Text() != "אב" {
				t.Fatalf("deleted wrong RTL cluster: %q", editor.model.Text())
			}
			editorKey(t, win, events.KeyZ, textCommandModifier())
			if editor.model.Text() != "אבג" || editor.selection != (TextSelection{6, 4}) {
				t.Fatal("undo did not restore RTL text and directional selection")
			}
			editor.model.SetText("e\u0301x")
			editor.setSelectionValue(TextSelection{0, 0})
			editorKey(t, win, events.KeyArrowRight, 0)
			if editor.selection.Caret != 3 {
				t.Fatalf("navigation split native combining cluster: %+v", editor.selection)
			}
			editorKey(t, win, events.KeyBackspace, 0)
			if editor.model.Text() != "x" {
				t.Fatalf("deletion split native combining cluster: %q", editor.model.Text())
			}
		})
	}
}
