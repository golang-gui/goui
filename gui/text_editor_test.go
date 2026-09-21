package gui

import (
	"fmt"
	"image/color"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

// 共用排版、剪贴板替身与事件入口。

// Fixture typography has 10 DIP cells and 20 DIP lines. It is deliberately
// independent of editor geometry; shaping/bidi contracts have separate tests.
type editorTypography struct{ testTypography }

func (c *editorTypography) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, _ := c.testTypography.NewTextLayout(text, format, width, height)
	p := l.(*testTextLayout)
	p.resized = func() { layoutEditorFixture(p) }
	layoutEditorFixture(p)
	return p, nil
}

func layoutEditorFixture(p *testTextLayout) {
	text, format, width := p.text, p.format, p.width
	p.lines, p.clusters = nil, nil
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

func checkParagraphCache(t *testing.T, c *textParagraphCache) {
	t.Helper()
	count, size := 0, 0
	var previous *textParagraph
	for p := c.oldest; p != nil; p = p.next {
		if p.previous != previous || c.paragraphs[p.index] != p {
			t.Fatal("cache links and paragraph map disagree")
		}
		previous = p
		count++
		size += p.textBytes
		if count > len(c.paragraphs) {
			t.Fatal("cache cycle")
		}
	}
	if count != len(c.paragraphs) || size != c.textBytes || previous != c.newest {
		t.Fatal("cache accounting mismatch")
	}
}

func TestTextParagraphCacheRecencyAndBudgets(t *testing.T) {
	var c textParagraphCache
	for i := 0; i < textParagraphCacheLimit; i++ {
		c.add(i, 10, &textParagraph{layout: &testTextLayout{}})
	}
	retained, evicted := c.paragraphs[0], c.paragraphs[1]
	c.touch(retained)
	c.add(textParagraphCacheLimit, 10, &textParagraph{layout: &testTextLayout{}})
	c.trim(textParagraphCacheLimit, textParagraphCacheLimit+1)
	if c.paragraphs[0] != retained || c.paragraphs[1] != nil || !evicted.layout.(*testTextLayout).destroyed {
		t.Fatal("eviction did not respect access recency")
	}
	checkParagraphCache(t, &c)
	c.clear()
	if !retained.layout.(*testTextLayout).destroyed {
		t.Fatal("clear did not release retained resources")
	}
	checkParagraphCache(t, &c)
	// An oversized active paragraph must remain usable. Once it leaves the
	// active range, the byte budget evicts it even below the count limit.
	large := &textParagraph{layout: &testTextLayout{}}
	c.add(0, textParagraphCacheBytes+1, large)
	c.trim(0, 1)
	if c.paragraphs[0] != large {
		t.Fatal("evicted oversized active paragraph")
	}
	c.add(1, 10, &textParagraph{layout: &testTextLayout{}})
	c.trim(1, 2)
	if c.paragraphs[0] != nil || c.textBytes != 10 || !large.layout.(*testTextLayout).destroyed {
		t.Fatal("text byte budget not enforced")
	}
	checkParagraphCache(t, &c)
	c.clear()
	// The active viewport can contain more paragraphs than the cache limit.
	for i := 0; i < textParagraphCacheLimit+10; i++ {
		c.add(i, 0, &textParagraph{})
	}
	c.trim(0, textParagraphCacheLimit+10)
	if len(c.paragraphs) != textParagraphCacheLimit+10 {
		t.Fatal("active range was evicted")
	}
	c.trim(textParagraphCacheLimit, textParagraphCacheLimit+10)
	if len(c.paragraphs) != textParagraphCacheLimit {
		t.Fatal("cache did not recover its bound after viewport shrank")
	}
	checkParagraphCache(t, &c)
}

func TestTextViewRecentLayoutsSurviveScrollEditAndUnmount(t *testing.T) {
	var text strings.Builder
	for i := 0; i < 2000; i++ {
		fmt.Fprintf(&text, "line %04d\n", i)
	}
	editor, win, typo := newEditorFixture(t, text.String())
	original := editor.paragraph(3)
	editor.LayoutVisible(editor.viewport, geometry.Point{Y: 2000})
	count := len(typo.calls)
	editor.LayoutVisible(editor.viewport, geometry.Point{})
	if editor.paragraph(3) != original || len(typo.calls) != count {
		t.Fatal("return scroll rebuilt cached paragraphs")
	}
	editor.LayoutVisible(editor.viewport, geometry.Point{Y: 2000})
	if err := editor.model.Replace(TextRange{}, "new\n"); err != nil {
		t.Fatal(err)
	}
	if editor.paragraphs[4] != original || original.index != 4 {
		t.Fatal("edit failed to remap an offscreen cached paragraph")
	}
	checkParagraphCache(t, &editor.textParagraphCache)
	if !editor.model.Undo() {
		t.Fatal("undo failed")
	}
	if editor.paragraphs[3] != original || original.index != 3 {
		t.Fatal("undo failed to restore cached paragraph index")
	}
	// A standalone fixture has no ScrollView to acknowledge anchor requests.
	editor.LayoutVisible(editor.viewport, editor.offset)
	editor.LayoutVisible(editor.viewport, editor.offset)
	for y := float32(4000); y < 36000; y += 300 {
		editor.LayoutVisible(editor.viewport, geometry.Point{Y: y})
		checkParagraphCache(t, &editor.textParagraphCache)
		if len(editor.paragraphs) > textParagraphCacheLimit {
			t.Fatal("scroll cache grew without bound")
		}
	}
	if !original.layout.(*testTextLayout).destroyed {
		t.Fatal("old cached layout was never evicted")
	}
	win.SetWidget(nil)
	checkParagraphCache(t, &editor.textParagraphCache)
	for _, p := range typo.layouts {
		if !p.destroyed {
			t.Fatal("unmount leaked a native layout")
		}
	}
}

func TestTextViewOffscreenCacheInvalidation(t *testing.T) {
	editor, _, typo := newEditorFixture(t, strings.Repeat("abcdefghijklmnopqrst\n", 2000))
	p := editor.paragraph(3)
	editor.LayoutVisible(editor.viewport, geometry.Point{Y: 2000})
	count := len(typo.calls)
	app := App.(*application)
	foreground := color.RGBA{R: 80, A: 255}
	app.style = style.Sheet(append(DefaultStyleRules(), style.Name(styleNameTextView).ForegroundColor(foreground))...)
	editor.StyleChanged()
	if p.layout.(*testTextLayout).destroyed || !colors.Equal(p.layout.Format().TextColor, foreground) || len(typo.calls) != count {
		t.Fatal("color change failed to update retained layouts in place")
	}
	editor.LayoutVisible(geometry.Size{Width: 109, Height: 108}, editor.offset)
	if p.measured {
		t.Fatal("offscreen measurement remained valid after width change")
	}
	if got := editor.paragraph(3); got != p || got.height != 40 {
		t.Fatal("offscreen paragraph was not lazily resized on access")
	}
	// Width A -> B -> A resets the height index twice. Reusing a cached native
	// layout must still restore its height into the current index.
	editor.LayoutVisible(geometry.Size{Width: 209, Height: 108}, geometry.Point{})
	if editor.paragraph(3) != p || p.height != 20 || editor.heights.Top(4)-editor.heights.Top(3) != 20 {
		t.Fatal("returning to original width left stale paragraph heights")
	}
	app.style = style.Sheet(append(DefaultStyleRules(), style.Name(styleNameTextView).FontSize(30))...)
	editor.StyleChanged()
	if !p.layout.(*testTextLayout).destroyed || len(editor.paragraphs) != 0 {
		t.Fatal("font change did not clear retained layouts")
	}
	checkParagraphCache(t, &editor.textParagraphCache)
}

func TestTextViewMeasuresEditingGeometryOnlyOnDemand(t *testing.T) {
	editor, _, _ := newEditorFixture(t, strings.Repeat("abcdefghijklmnopqrst\n", 30))
	p := editor.paragraph(1)
	native := p.layout.(*testTextLayout)
	if p.geometryValid || native.metricsCalls != 0 {
		t.Fatal("display-only paragraph eagerly built editing geometry")
	}
	editor.Paint(&testLabelPainter{})
	if p.geometryValid || native.metricsCalls != 0 {
		t.Fatal("plain painting built unrelated editing geometry")
	}
	rng, _ := editor.model.LineRange(1)
	if got := editor.snapPosition(rng.Start + 1); got != rng.Start+1 {
		t.Fatalf("lazy snapping returned %d", got)
	}
	editor.snapPosition(rng.Start + 2)
	if !p.geometryValid || native.metricsCalls != 1 {
		t.Fatal("editing geometry was not cached after first use")
	}
	editor.LayoutVisible(geometry.Size{Width: 109, Height: 108}, geometry.Point{})
	p = editor.paragraph(1)
	if p.layout != native || p.geometryValid {
		t.Fatal("reflow did not retain layout and invalidate old geometry")
	}
	g := p.editGeometry(editor.lineHeight)
	if g.LineCount() != 2 || native.metricsCalls != 2 {
		t.Fatal("lazy geometry did not use the new wrapping width")
	}
	p.editGeometry(editor.lineHeight)
	if native.metricsCalls != 2 {
		t.Fatal("unchanged geometry was remeasured")
	}
}

func TestTextViewPaintSelectionDoesNotMeasureUnselectedParagraphs(t *testing.T) {
	editor, _, _ := newEditorFixture(t, strings.Repeat("row\n", 30))
	editor.SetSelection(TextSelection{4, 6})
	editor.Paint(&testLabelPainter{})
	if !editor.paragraph(1).geometryValid || editor.paragraph(2).geometryValid {
		t.Fatal("selection painting measured an unrelated paragraph")
	}
}

func TestTextViewWidthChangeReusesLayoutAndRefreshesHeight(t *testing.T) {
	editor, _, typo := newEditorFixture(t, "abcdefghijklmnopqrst")
	original := editor.paragraphs[0].layout
	count := len(typo.calls)
	for _, width := range []float32{109, 209, 89, 209} {
		editor.LayoutVisible(geometry.Size{Width: width, Height: 108}, geometry.Point{})
		p := editor.paragraph(0)
		if p.layout != original || original.(*testTextLayout).destroyed {
			t.Fatal("width change replaced the native layout")
		}
		nativeWidth, _ := p.layout.Size()
		if nativeWidth != width-9 {
			t.Fatalf("stale native width: %g, want %g", nativeWidth, width-9)
		}
		cols := int((width - 9) / 10)
		wantHeight := float32((20+cols-1)/cols) * 20
		if p.height != wantHeight || editor.heights.Total() != wantHeight {
			t.Fatalf("stale height at width %g: paragraph=%g index=%g want=%g", width, p.height, editor.heights.Total(), wantHeight)
		}
	}
	if len(typo.calls) != count {
		t.Fatal("width-only reflow created layouts")
	}
}

func TestTextViewReflowUpdatesOnlyOneSharedModelView(t *testing.T) {
	left, _, _ := newEditorFixture(t, "abcdefghijklmnopqrst")
	right := NewTextView()
	right.SetModel(left.model)
	win := &window{}
	win.SetWidget(right)
	t.Cleanup(func() { win.SetWidget(nil) })
	right.Arrange(geometry.Rect(0, 0, 209, 108))
	left.LayoutVisible(geometry.Size{Width: 109, Height: 108}, geometry.Point{})
	if left.paragraph(0).height != 40 || right.paragraph(0).height != 20 {
		t.Fatal("reflow did not preserve independent view layouts")
	}
}
