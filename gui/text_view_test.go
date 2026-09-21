package gui

import (
	"fmt"
	"image/color"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

// 多行视图、模型与布局。

func TestTextViewViewportCacheAndLocalEdit(t *testing.T) {
	editor, _, typo := newEditorFixture(t, strings.Repeat("row\n", 50000))
	if len(editor.paragraphs) > 20 || len(typo.calls) > 25 {
		t.Fatalf("initial layout shaped the document: %d layouts", len(typo.calls))
	}
	untouched := editor.paragraphs[3].layout
	editor.SetSelection(TextSelection{1, 1})
	count := len(typo.calls)
	editor.Paint(&testLabelPainter{})
	editor.Paint(&testLabelPainter{})
	if len(typo.calls) != count {
		t.Fatal("selection/paint rebuilt text")
	}
	if err := editor.model.Replace(TextRange{0, 0}, "X"); err != nil {
		t.Fatal(err)
	}
	editor.LayoutVisible(editor.viewport, editor.offset)
	if editor.paragraphs[3].layout != untouched {
		t.Fatal("local edit invalidated an unrelated paragraph")
	}
	if len(typo.calls)-count != 1 {
		t.Fatalf("local edit built %d layouts, want one", len(typo.calls)-count)
	}
	editor.LayoutVisible(editor.viewport, geometry.Point{Y: 20000})
	count = len(typo.calls)
	editor.Paint(&testLabelPainter{})
	if len(typo.calls) != count {
		t.Fatal("paint rebuilt an offscreen caret paragraph")
	}
	if len(editor.paragraphs) > 20 {
		t.Fatalf("scroll retained %d paragraph layouts", len(editor.paragraphs))
	}
	for _, p := range typo.layouts {
		if p == untouched && !p.destroyed {
			t.Fatal("offscreen layout not released")
		}
	}
	info := editor.Snapshot()
	if info.Role != RoleTextView || info.Text != "" || info.TextEditing == nil {
		t.Fatal("missing range-labelled editor snapshot")
	}
	state := info.TextEditing
	want, _ := editor.model.Slice(state.VisibleRange)
	if want != state.VisibleText || state.Length != editor.model.Len() || len(state.VisibleText) > 4096 {
		t.Fatalf("snapshot mismatch %+v", state)
	}
}

func TestTextViewSharedModelAndUnmount(t *testing.T) {
	a, win, _ := newEditorFixture(t, "abc")
	b := NewTextView()
	b.SetModel(a.model)
	other := &window{}
	other.SetWidget(b)
	b.Arrange(geometry.Rect(0, 0, 209, 108))
	t.Cleanup(func() { other.SetWidget(nil) })
	a.SetSelection(TextSelection{1, 1})
	b.SetSelection(TextSelection{3, 3})
	if a.paragraphs[0].layout == b.paragraphs[0].layout {
		t.Fatal("shared native layout")
	}
	a.im.emitCommit(IMCommit{Text: "X"})
	if a.selection.Caret != 2 || b.selection.Caret != 4 {
		t.Fatalf("view positions not independently mapped: %+v %+v", a.selection, b.selection)
	}
	win.SetWidget(nil)
	for _, p := range a.paragraphs {
		if !p.layout.(*testTextLayout).destroyed {
			t.Fatal("unmount retained layout")
		}
	}
	if a.modelHandle != nil {
		t.Fatal("unmount retained model listener")
	}
	selection := a.selection
	a.model.SetText("new")
	if a.selection != selection {
		t.Fatal("unmounted view received callback")
	}
	win.SetWidget(a)
	a.Arrange(geometry.Rect(0, 0, 209, 108))
	if a.seenRevision != a.model.Revision() || a.paragraphs[0].layout.Text() != "new" {
		t.Fatal("remount did not refresh")
	}
}

func TestTextViewColorKeepsLayoutFontInvalidates(t *testing.T) {
	editor, _, _ := newEditorFixture(t, "abc")
	old := editor.paragraphs[0].layout
	app := App.(*application)
	app.style = style.Sheet(append(DefaultStyleRules(), style.Name(styleNameTextView).ForegroundColor(color.RGBA{R: 80, A: 255}))...)
	editor.StyleChanged()
	if editor.paragraphs[0].layout != old {
		t.Fatal("color rebuilt layout")
	}
	app.style = style.Sheet(append(DefaultStyleRules(), style.Name(styleNameTextView).FontSize(30))...)
	editor.StyleChanged()
	if !old.(*testTextLayout).destroyed {
		t.Fatal("font retained stale geometry")
	}
}

func TestTextViewRevealEndAfterEstimatedHeightsBecomeWrapped(t *testing.T) {
	typo := &editorTypography{}
	setTestApplication(t, typo)
	editor := NewTextView()
	editor.SetModel(NewTextModel(strings.Repeat(strings.Repeat("x", 60)+"\n", 1000)))
	scroll := NewScrollView()
	scroll.SetChild(editor)
	win := &window{}
	win.SetWidget(scroll)
	t.Cleanup(func() { win.SetWidget(nil) })
	box := geometry.Rect(0, 0, 220, 108)
	scroll.Measure(layout.Tight(box.Size))
	scroll.Arrange(box)
	win.SetFocusedWidget(editor)
	editorKey(t, win, events.KeyEnd, textCommandModifier())
	for frame := 0; frame < 12; frame++ {
		scroll.Arrange(box)
	}
	caret, ok := editor.caretRect()
	if !ok || caret.Y < scroll.ScrollY() || caret.Y+caret.Height > scroll.ScrollY()+editor.viewport.Height {
		t.Fatalf("caret %v not revealed in viewport y=%g height=%g after height refinement", caret, scroll.ScrollY(), editor.viewport.Height)
	}
}

func TestTextViewRevealEndUsesScheduledLayouts(t *testing.T) {
	typo := &editorTypography{}
	setTestApplication(t, typo)
	editor := NewTextView()
	editor.SetModel(NewTextModel(strings.Repeat(strings.Repeat("x", 110)+"\n", 95000)))
	scroll := NewScrollView()
	scroll.SetChild(editor)
	win := &window{}
	win.width, win.height = 507, 590
	win.SetWidget(scroll)
	t.Cleanup(func() { win.SetWidget(nil) })
	settle := func() {
		for frame := 0; win.layoutDirty && frame < 100; frame++ {
			win.layoutContent()
		}
		if win.layoutDirty {
			t.Fatal("layout requests did not settle")
		}
	}
	settle()
	win.SetFocusedWidget(editor)
	for _, width := range []float32{441, 591, 507} {
		win.width = width
		win.RequestLayout()
		settle()
	}
	editorKey(t, win, events.KeyHome, textCommandModifier())
	editorKey(t, win, events.KeyEnd, textCommandModifier()|events.ModifierShift)
	settle()
	caret, ok := editor.caretRect()
	if !ok || caret.Y < scroll.ScrollY() || caret.Y+caret.Height > scroll.ScrollY()+editor.viewport.Height {
		t.Fatalf("scheduled layouts stopped before revealing caret: caret=%v scroll=%g viewport=%v pending=%t/%t/%t", caret, scroll.ScrollY(), editor.viewport, editor.revealCaret, editor.anchorPending, scroll.revealPending)
	}
	if editor.offset.Y != scroll.ScrollY() || editor.revealCaret || editor.anchorPending || scroll.revealPending {
		t.Fatalf("editor and host disagree after scheduled layouts: editor=%g host=%g pending=%t/%t/%t", editor.offset.Y, scroll.ScrollY(), editor.revealCaret, editor.anchorPending, scroll.revealPending)
	}
}

// 性能基准。

func BenchmarkTextViewLocalEdit(b *testing.B) {
	for _, megabytes := range []int{1, 10} {
		b.Run(fmt.Sprintf("%dMiB", megabytes), func(b *testing.B) {
			typo := &editorTypography{}
			oldApp := App
			App = &application{typo: typo}
			defer func() { App = oldApp }()
			editor := NewTextView()
			defer editor.disconnectModel()
			defer editor.releaseParagraphs()
			editor.SetModel(NewTextModel(strings.Repeat("line0123456789\n", megabytes*1024*1024/15)))
			editor.Arrange(geometry.Rect(0, 0, 500, 400))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				typo.calls = nil
				typo.layouts = nil
				_ = editor.model.Replace(TextRange{1, 2}, "X")
				editor.LayoutVisible(editor.viewport, editor.offset)
				if len(typo.calls) != 1 {
					b.Fatalf("local edit shaped %d paragraphs", len(typo.calls))
				}
				editor.model.ClearHistory()
			}
		})
	}
}

// Same 500x400 DIP viewport and deterministic 10x20 DIP typography for both
// document sizes. Count shaping work independently of wall-clock timing.
func BenchmarkTextViewScrollAndReflow(b *testing.B) {
	for _, megabytes := range []int{1, 10} {
		for _, operation := range []string{"scroll", "reflow"} {
			b.Run(fmt.Sprintf("%dMiB/%s", megabytes, operation), func(b *testing.B) {
				typo := &editorTypography{}
				previous := App
				App = &application{typo: typo}
				defer func() { App = previous }()
				editor := NewTextView()
				text := strings.Repeat("x", 80) + "\n"
				editor.SetModel(NewTextModel(strings.Repeat(text, megabytes*1024*1024/len(text))))
				scroll := NewScrollView()
				scroll.SetChild(editor)
				win := &window{}
				win.SetWidget(scroll)
				defer win.SetWidget(nil)
				box := geometry.Rect(0, 0, 500, 400)
				scroll.Measure(layout.Tight(box.Size))
				scroll.Arrange(box)
				scroll.SetScrollY(10000)
				shaped := 0
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					typo.calls, typo.layouts = nil, nil
					if operation == "scroll" {
						scroll.SetScrollY(10000 + float32(i%100)*100)
					} else {
						box.Width = 400 + float32(i%2)*200
						scroll.Measure(layout.Tight(box.Size))
						scroll.Arrange(box)
					}
					if len(typo.calls) > 120 {
						b.Fatalf("%s shaped %d paragraphs", operation, len(typo.calls))
					}
					shaped += len(typo.calls)
				}
				b.StopTimer()
				b.ReportMetric(float64(shaped)/float64(b.N), "layouts/op")
			})
		}
	}
}

// 接口契约。

var _ Scrollable = (*TextView)(nil)

var _ ScrollIntoViewRequester = (*TextView)(nil)

var _ IMClient = (*TextView)(nil)

// 多行预编辑投影与生命周期。

func TestTextViewPreeditProjectionAcrossParagraphs(t *testing.T) {
	editor, win, typo := newEditorFixture(t, "aa\nbb\ncc\ndd")
	editor.SetSelection(TextSelection{7, 1})
	untouched := editor.paragraphs[3].layout
	count := len(typo.calls)
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "中\nX", Caret: 4})
	if editor.model.Text() != "aa\nbb\ncc\ndd" || editor.model.Revision() != 0 || editor.model.CanUndo() {
		t.Fatal("preedit mutated shared model/history")
	}
	if editor.Selection() != (TextSelection{7, 1}) {
		t.Fatal("preedit replaced model-space selection")
	}
	if editor.displayLineCount() != 3 || editor.displayParagraph(0) != "a中" || editor.displayParagraph(1) != "Xc" || editor.displayParagraph(2) != "dd" {
		t.Fatal("preedit paragraph projection is wrong")
	}
	if editor.displayLineRange(1) != (TextRange{5, 7}) || editor.displayLineRange(2) != (TextRange{8, 10}) {
		t.Fatalf("display ranges wrong: %v %v", editor.displayLineRange(1), editor.displayLineRange(2))
	}
	editor.LayoutVisible(editor.viewport, editor.offset)
	if editor.paragraphs[2].layout != untouched || len(typo.calls)-count != 2 {
		t.Fatal("preedit rebuilt unaffected paragraphs")
	}
	caret, _ := editor.caretRect()
	if caret.X != 4 || caret.Y != 24 {
		t.Fatalf("caret %v not in preedit's second line", caret)
	}
	state := editor.Snapshot().TextEditing
	if state.Preedit == nil || state.Preedit.Replacement != (TextRange{1, 7}) || state.Preedit.Text != "中\nX" || state.Preedit.Caret != 4 {
		t.Fatalf("preedit snapshot %+v", state.Preedit)
	}
	committed, _ := editor.model.Slice(state.VisibleRange)
	if state.VisibleText != committed {
		t.Fatal("snapshot mixed display offsets with committed content")
	}
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "文"})
	if editor.preedit != nil || editor.model.Text() != "a文c\ndd" || editor.Selection() != (TextSelection{4, 4}) {
		t.Fatalf("commit %q %+v", editor.model.Text(), editor.Selection())
	}
	editorKey(t, win, events.KeyZ, textCommandModifier())
	if editor.model.Text() != "aa\nbb\ncc\ndd" || editor.Selection() != (TextSelection{7, 1}) {
		t.Fatal("IME undo did not restore original content and directed selection")
	}
}

func TestTextViewPreeditEmptyAndCaretOnlyUpdates(t *testing.T) {
	editor, win, typo := newEditorFixture(t, "")
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "ab", Caret: 1})
	editor.LayoutVisible(editor.viewport, editor.offset)
	p := editor.paragraphs[0].layout
	count := len(typo.calls)
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "ab", Caret: 2})
	editor.LayoutVisible(editor.viewport, editor.offset)
	if len(typo.calls) != count || editor.paragraphs[0].layout != p {
		t.Fatal("preedit caret-only update reshaped text")
	}
	editorKey(t, win, events.KeyEscape, 0)
	if editor.preedit != nil || editor.model.Len() != 0 || editor.model.CanUndo() {
		t.Fatal("Escape committed preedit")
	}
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "x\r\n中", Caret: 6})
	if editor.preedit.Text() != "x\n中" || editor.preedit.Caret() != 5 {
		t.Fatalf("normalized preedit caret %+v", editor.preedit)
	}
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit})
	if editor.preedit != nil {
		t.Fatal("empty preedit did not cancel projection")
	}
}

func TestTextViewPreeditIsolationAndModelChangeCancels(t *testing.T) {
	a, win, _ := newEditorFixture(t, "one\ntwo\nthree")
	b := NewTextView()
	b.SetModel(a.model)
	other := &window{}
	other.SetWidget(b)
	t.Cleanup(func() { other.SetWidget(nil) })
	b.Arrange(geometry.Rect(0, 0, 209, 108))
	a.SetSelection(TextSelection{1, 8})
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "NEW", Caret: 3})
	if b.preedit != nil || b.displayParagraph(0) != "one" || b.displayLineCount() != 3 {
		t.Fatal("preedit leaked into other view")
	}
	_ = a.model.Replace(TextRange{0, 0}, "x\n")
	if a.preedit != nil || a.displayLineCount() != 4 || a.heights.Count() != 4 {
		t.Fatal("external edit left stale projection/index")
	}
	a.LayoutVisible(a.viewport, a.offset)
	if a.paragraphs[0].layout.Text() != "x" {
		t.Fatal("stale preedit layout survived model change")
	}
}

func TestTextViewPreeditLifecycleAndPasteInvalidation(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "abc")
	clip := &editorClipboard{}
	App.(*application).clipboard = clip
	editorKey(t, win, events.KeyV, textCommandModifier())
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "中", Caret: 3})
	clip.pending("stale", true)
	if editor.model.Text() != "abc" || editor.preedit == nil {
		t.Fatal("preedit did not invalidate pending paste")
	}
	editor.SetSelection(editor.Selection())
	if editor.preedit == nil {
		t.Fatal("same declared selection cancelled composition")
	}
	editor.SetReadOnly(true)
	if editor.preedit != nil || win.activeIM != nil {
		t.Fatal("readonly retains preedit or input method")
	}
	editor.SetReadOnly(false)
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "X", Caret: 1})
	win.SetWidget(nil)
	if editor.preedit != nil || len(editor.paragraphs) != 0 {
		t.Fatal("unmount retained projection/layouts")
	}
}

type textResetProbe struct{ reset func() }

func (p *textResetProbe) SetEnabled(bool) {}

func (p *textResetProbe) SetCaretRect(geometry.Rectangle) {}

func (p *textResetProbe) Reset() {
	if p.reset != nil {
		p.reset()
	}
}

func (p *textResetProbe) Destroy() {}

func TestTextViewPreeditResetCannotCommitStaleText(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "abc")
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "old", Caret: 3})
	win.inputMethod = &textResetProbe{reset: func() {
		win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "stale"})
		win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "stale", Caret: 5})
	}}
	editor.SetSelection(TextSelection{3, 3})
	if editor.preedit != nil || editor.model.Text() != "abc" || editor.Selection() != (TextSelection{3, 3}) {
		t.Fatal("Reset callback changed document/composition")
	}
	win.inputMethod = nil
}

func TestTextViewPreeditResetMayUnmountView(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "abc")
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "old", Caret: 3})
	probe := &textResetProbe{}
	probe.reset = func() { probe.reset = nil; win.SetWidget(nil) }
	win.inputMethod = probe
	editor.SetModel(NewTextModel("replacement"))
	if !editor.suspended || editor.model.Text() != "abc" || len(editor.paragraphs) != 0 {
		t.Fatal("setter continued after Reset unmounted its view")
	}
	win.inputMethod = nil
}

// 视口锚点、重排与滚动收敛。

func anchoredEditorFixture(t *testing.T, text string) (*TextView, *ScrollView, *editorTypography) {
	t.Helper()
	typo := &editorTypography{}
	setTestApplication(t, typo)
	editor := NewTextView()
	editor.SetModel(NewTextModel(text))
	scroll := NewScrollView()
	scroll.SetChild(editor)
	win := &window{}
	win.SetWidget(scroll)
	t.Cleanup(func() { win.SetWidget(nil) })
	scroll.Measure(layout.Tight(geometry.Size{Width: 220, Height: 108}))
	scroll.Arrange(geometry.Rect(0, 0, 220, 108))
	return editor, scroll, typo
}

func TestTextViewAnchorMapsEditsAboveViewport(t *testing.T) {
	editor, scroll, _ := anchoredEditorFixture(t, strings.Repeat("row\n", 1000))
	scroll.SetScrollY(1005)
	old := editor.anchor
	if !old.valid {
		t.Fatal("viewport anchor missing")
	}
	_ = editor.model.Replace(TextRange{0, 0}, "new\n")
	for range 3 {
		scroll.Arrange(scroll.Rect())
	}
	if editor.anchor.offset != old.offset+4 || editor.anchor.within != old.within || scroll.ScrollY() != 1025 {
		t.Fatalf("insertion moved anchored text: old=%+v new=%+v scroll=%g", old, editor.anchor, scroll.ScrollY())
	}
	_ = editor.model.Replace(TextRange{0, 4}, "")
	for range 3 {
		scroll.Arrange(scroll.Rect())
	}
	if editor.anchor.offset != old.offset || editor.anchor.within != old.within || scroll.ScrollY() != 1005 {
		t.Fatalf("deletion lost anchor %+v scroll=%g", editor.anchor, scroll.ScrollY())
	}
}

func TestTextViewResizeKeepsTopWrappedText(t *testing.T) {
	editor, scroll, typo := anchoredEditorFixture(t, strings.Repeat(strings.Repeat("x", 80)+"\n", 1000))
	scroll.SetScrollY(1005)
	for range 3 {
		scroll.Arrange(scroll.Rect())
	}
	old := editor.anchor
	count := len(typo.calls)
	box := geometry.Rect(0, 0, 320, 108)
	scroll.Measure(layout.Tight(box.Size))
	for range 4 {
		scroll.Arrange(box)
	}
	if len(typo.calls)-count > 60 {
		t.Fatal("resize laid out the entire document")
	}
	index := editor.model.LineAt(old.offset)
	p := editor.paragraph(index)
	rng, _ := editor.model.LineRange(index)
	line := p.geometry.LineIndex(textedit.Position{Offset: old.offset - rng.Start, Upstream: false})
	// The anchor is at the inner content clip (padding), not the widget edge;
	// text's padding translation and the clip's padding cancel each other.
	y := editor.heights.Top(index) + p.geometry.LineMetrics(line).Y
	if scroll.ScrollY() != y+old.within {
		t.Fatalf("reflow moved top text: scroll=%g text y=%g residual=%g", scroll.ScrollY(), y, old.within)
	}
	if editor.anchorPending || scroll.revealPending {
		t.Fatal("anchor request did not settle after the host applied it")
	}
}

func TestTextViewResizeAnchorsInsidePaddingClip(t *testing.T) {
	editor, scroll, _ := anchoredEditorFixture(t, strings.Repeat(strings.Repeat("x", 80)+"\n", 20))
	// The fixture uses 10 DIP cells / 20 DIP rows: the old viewport fits 20
	// characters per line. At offset 21, line 0 is completely above the inner
	// padding clip and byte 20 is the first visible line's start, clipped 1 DIP.
	scroll.SetScrollY(21)
	box := geometry.Rect(0, 0, 320, 108) // now fits 30 characters per line
	scroll.Measure(layout.Tight(box.Size))
	for range 8 {
		scroll.Arrange(box)
	}
	// Byte 20 is now on line 0. Preserve its 1 DIP clipping, not the hidden
	// preceding line's position. The former offset-padding anchor yielded 21.
	if scroll.ScrollY() != 1 {
		t.Fatalf("anchored a line hidden by padding: scroll=%g want=1", scroll.ScrollY())
	}
	if editor.anchorPending || scroll.revealPending {
		t.Fatal("clip-relative anchor did not settle")
	}
}

func TestTextViewResizeRoundTripKeepsLogicalAnchor(t *testing.T) {
	editor, scroll, _ := anchoredEditorFixture(t, strings.Repeat(strings.Repeat("x", 80)+"\n", 20))
	scroll.SetScrollY(21) // byte 20, clipped by 1 DIP in the 20-cell viewport
	for _, step := range []struct{ width, want float32 }{
		{320, 1}, // byte 20 joins the first display line
		{250, 1},
		{220, 21}, // the same byte returns to the second display line
		{320, 1},
		{220, 21},
	} {
		r := geometry.Rect(0, 0, step.width, 108)
		scroll.Measure(layout.Tight(r.Size))
		for range 8 {
			scroll.Arrange(r)
		}
		if scroll.ScrollY() != step.want {
			t.Fatalf("width=%g scroll=%g want=%g anchor=%+v", step.width, scroll.ScrollY(), step.want, editor.anchor)
		}
		if editor.anchorPending || scroll.revealPending {
			t.Fatal("round-trip resize did not settle")
		}
	}
}

func TestTextViewScrollReplacesReflowAnchor(t *testing.T) {
	editor, scroll, _ := anchoredEditorFixture(t, strings.Repeat(strings.Repeat("x", 80)+"\n", 20))
	arrange := func(width float32) {
		r := geometry.Rect(0, 0, width, 108)
		scroll.Measure(layout.Tight(r.Size))
		for range 8 {
			scroll.Arrange(r)
		}
	}
	scroll.SetScrollY(21)
	arrange(320)
	scroll.SetScrollY(2) // new browsing position, even within the same line
	arrange(220)
	if scroll.ScrollY() != 2 || editor.anchor.offset != 0 {
		t.Fatalf("scroll reused an obsolete anchor: scroll=%g anchor=%+v", scroll.ScrollY(), editor.anchor)
	}
	scroll.SetScrollY(21)
	arrange(320)
	if err := editor.model.Replace(TextRange{0, 0}, "new\n"); err != nil {
		t.Fatal(err)
	}
	arrange(220)
	// One inserted paragraph adds 20 DIP, without losing byte 20 in the old
	// paragraph while it was in the middle of the wider display line.
	if scroll.ScrollY() != 41 || editor.anchor.offset != 24 {
		t.Fatalf("edit lost retained model position: scroll=%g anchor=%+v", scroll.ScrollY(), editor.anchor)
	}
}

type fractionalEditorTypography struct{ editorTypography }

func (c *fractionalEditorTypography) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, err := c.editorTypography.NewTextLayout(text, format, width, height)
	if err != nil {
		return nil, err
	}
	p := l.(*testTextLayout)
	// 20.5 DIP line metrics are legitimate; width and text boundaries stay
	// identical to the independent 10 DIP-cell fixture.
	for i := range p.lines {
		p.lines[i].Y *= 1.025
		p.lines[i].Height *= 1.025
		p.lines[i].Baseline *= 1.025
	}
	for i := range p.clusters {
		p.clusters[i].Y *= 1.025
		p.clusters[i].Height *= 1.025
	}
	p.measureSize.Height *= 1.025
	return p, nil
}

func TestTextViewFractionalAnchorSettles(t *testing.T) {
	setTestApplication(t, &fractionalEditorTypography{})
	editor := NewTextView()
	editor.SetModel(NewTextModel(strings.Repeat(strings.Repeat("x", 80)+"\n", 100)))
	scroll := NewScrollView()
	scroll.SetChild(editor)
	win := &window{}
	win.SetWidget(scroll)
	defer win.SetWidget(nil)
	arrange := func(width float32) {
		r := geometry.Rect(0, 0, width, 108)
		scroll.Measure(layout.Tight(r.Size))
		for range 8 {
			scroll.Arrange(r)
		}
	}
	arrange(220)
	for offset := float32(100); offset < 130; offset++ {
		scroll.SetScrollY(offset)
		for _, width := range []float32{320, 220} {
			arrange(width)
			if editor.anchorPending || scroll.revealPending {
				t.Fatalf("fractional metrics oscillate: offset=%g width=%g scroll=%g contentOffset=%g anchor=%+v", offset, width, scroll.ScrollY(), editor.offset.Y, editor.anchor)
			}
		}
	}
}

func TestTextViewFractionalRevealEndSettles(t *testing.T) {
	setTestApplication(t, &fractionalEditorTypography{})
	editor := NewTextView()
	editor.SetModel(NewTextModel(strings.Repeat("row\n", 100)))
	scroll := NewScrollView()
	scroll.SetChild(editor)
	win := &window{}
	win.width, win.height = 220, 108
	win.SetWidget(scroll)
	defer win.SetWidget(nil)
	editor.SetSelection(TextSelection{editor.model.Len(), editor.model.Len()})
	for frame := 0; win.layoutDirty && frame < 100; frame++ {
		win.layoutContent()
	}
	// 101 rows of 20.5 DIP plus two 4 DIP padding edges, minus 108 DIP:
	// the host's maximum integer scroll offset is floor(1970.5) = 1970.
	if scroll.ScrollY() != 1970 || editor.offset.Y != 1970 || editor.revealCaret || editor.anchorPending || scroll.revealPending || win.layoutDirty {
		t.Fatalf("fractional reveal did not settle: host=%g editor=%g pending=%t/%t/%t/%t", scroll.ScrollY(), editor.offset.Y, editor.revealCaret, editor.anchorPending, scroll.revealPending, win.layoutDirty)
	}
}

func TestTextViewAnchorAtLastPageSettles(t *testing.T) {
	editor, scroll, _ := anchoredEditorFixture(t, strings.Repeat(strings.Repeat("x", 80)+"\n", 20))
	editor.SetSelection(TextSelection{editor.model.Len(), editor.model.Len()})
	for range 6 {
		scroll.Arrange(scroll.Rect())
	}
	box := geometry.Rect(0, 0, 500, 208)
	scroll.Measure(layout.Tight(box.Size))
	for range 6 {
		scroll.Arrange(box)
	}
	if editor.anchorPending || editor.revealCaret || scroll.revealPending {
		t.Fatalf("last-page reflow kept requesting scroll: anchor=%t caret=%t host=%t", editor.anchorPending, editor.revealCaret, scroll.revealPending)
	}
	if scroll.ScrollY() > editor.ContentSize().Height-editor.viewport.Height {
		t.Fatal("anchor escaped the new document extent")
	}
}

func TestTextViewTypingAtTopDoesNotScrollAwayNewline(t *testing.T) {
	editor, scroll, _ := anchoredEditorFixture(t, "abc\ndef\nghi")
	editor.onCommit(IMCommit{Text: "\n"})
	for range 3 {
		scroll.Arrange(scroll.Rect())
	}
	if scroll.ScrollY() != 0 || editor.Selection().Caret != 1 {
		t.Fatalf("typing scrolled unnecessarily: scroll=%g caret=%d", scroll.ScrollY(), editor.Selection().Caret)
	}
}

// 拖选自动滚动。

func TestTextViewDragAutoScrollAndRelease(t *testing.T) {
	editor, scroll, win, q, _ := timedEditorFixture(t)
	for _, e := range []events.PointerEvent{
		{EventType: events.PointerDown, Position: geometry.Point{X: 8, Y: 8}, Button: events.PointerButtonLeft},
		{EventType: events.PointerMove, Position: geometry.Point{X: 20, Y: 160}, Buttons: events.PointerButtonLeftDown},
	} {
		if err := win.DispatchEvent(e); err != nil {
			t.Fatal(err)
		}
	}
	if editor.scrollTimer == nil || !editor.scrollTimer.Active() {
		t.Fatal("drag beyond viewport did not start timer")
	}
	beforeCaret := editor.selection.Caret
	for i := 0; i < 4; i++ {
		q.advance(textScrollInterval)
		q.dispatch(t)
	}
	if scroll.ScrollY() <= 0 || editor.selection.Caret <= beforeCaret || editor.selection.Anchor != 0 {
		t.Fatalf("drag scroll y=%g selection=%+v", scroll.ScrollY(), editor.selection)
	}
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Position: geometry.Point{X: 20, Y: 160}, Button: events.PointerButtonLeft})
	if editor.scrollTimer.Active() || editor.input.CapturingPointer() {
		t.Fatal("release did not stop autoscroll/capture")
	}
}

func TestTextViewAutoScrollCallbackCanUnmount(t *testing.T) {
	editor, _, win, q, _ := timedEditorFixture(t)
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 8, Y: 8}, Button: events.PointerButtonLeft})
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 20, Y: 160}, Buttons: events.PointerButtonLeftDown})
	handle := editor.ConnectScrollIntoView(func(geometry.Rectangle) { win.SetWidget(nil) })
	defer handle.Disconnect()
	q.advance(textScrollInterval)
	q.dispatch(t)
	if !editor.suspended || editor.scrollTimer != nil || editor.blinkTimer != nil || len(q.s.queue) != 0 {
		t.Fatal("autoscroll continued after unmount callback")
	}
}

// 原生排版的锚定与定位验证。

func testTextViewNativeRevealEnd(t *testing.T, newContext func() (typography.Context, error)) {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	typo, err := newContext()
	if err != nil {
		t.Fatal(err)
	}
	defer typo.Destroy()
	setTestApplication(t, typo)
	var text strings.Builder
	for row := 0; row < 95000; row++ {
		fmt.Fprintf(&text, "[%06d:start] text editing, viewport layout; [%06d:middle] selection and undo; [%06d:end] 中文测试。\n", row, row, row)
	}
	editor := NewTextView()
	editor.SetModel(NewTextModel(text.String()))
	scroll := NewScrollView()
	scroll.SetChild(editor)
	win := &window{}
	win.width, win.height = 507, 590
	win.SetWidget(scroll)
	defer win.SetWidget(nil)
	settle := func() {
		for frame := 0; win.layoutDirty && frame < 100; frame++ {
			win.layoutContent()
		}
		if win.layoutDirty {
			t.Fatal("native layout requests did not settle")
		}
	}
	settle()
	win.SetFocusedWidget(editor)
	scroll.SetScrollY(900)
	settle()
	for _, width := range []float32{441, 591, 507} {
		win.width = width
		win.RequestLayout()
		settle()
	}
	editorKey(t, win, events.KeyHome, textCommandModifier())
	editorKey(t, win, events.KeyEnd, textCommandModifier()|events.ModifierShift)
	settle()
	caret, ok := editor.caretRect()
	if !ok || editor.offset.Y != scroll.ScrollY() || caret.Y < editor.offset.Y || caret.Y+caret.Height > editor.offset.Y+editor.viewport.Height || editor.revealCaret || editor.anchorPending || scroll.revealPending {
		t.Fatalf("native scheduled layouts stopped before revealing caret: caret=%v scroll=%g viewport=%v pending=%t/%t/%t", caret, scroll.ScrollY(), editor.viewport, editor.revealCaret, editor.anchorPending, scroll.revealPending)
	}
}

func testTextViewNativeResizeAnchor(t *testing.T, newContext func() (typography.Context, error)) {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	typo, err := newContext()
	if err != nil {
		t.Fatal(err)
	}
	defer typo.Destroy()
	setTestApplication(t, typo)
	var text strings.Builder
	for row := 0; row < 1000; row++ {
		fmt.Fprintf(&text, "[%06d:start] text editing, viewport layout; [%06d:middle] selection and undo; [%06d:end] 中文测试。\n", row, row, row)
	}
	editor := NewTextView()
	editor.SetModel(NewTextModel(text.String()))
	scroll := NewScrollView()
	scroll.SetChild(editor)
	win := &window{}
	win.SetWidget(scroll)
	defer win.SetWidget(nil)
	arrange := func(width float32) {
		r := geometry.Rect(0, 0, width, 590)
		scroll.Measure(layout.Tight(r.Size))
		for range 8 {
			scroll.Arrange(r)
		}
	}
	arrange(507)
	for offset := float32(890); offset < 918; offset++ {
		arrange(507)
		scroll.SetScrollY(0)
		scroll.SetScrollY(offset)
		// Paint clips at padding, and translates text by padding-offset:
		// the first visible content coordinate is therefore offset, not
		// offset-padding. Derive it from native metrics, not editor.anchor.
		oldIndex := editor.heights.At(scroll.ScrollY())
		oldRange, _ := editor.model.LineRange(oldIndex)
		oldLines, _ := editor.paragraph(oldIndex).layout.MeasureMetrics()
		oldY := scroll.ScrollY() - editor.heights.Top(oldIndex)
		oldLine := 0
		for oldLine+1 < len(oldLines) && oldLines[oldLine+1].Y <= oldY {
			oldLine++
		}
		old := textViewportAnchor{offset: oldRange.Start + oldLines[oldLine].Start, within: oldY - oldLines[oldLine].Y}
		// Retain the original model character for the whole resize sequence.
		// Re-sampling each new display line would reproduce the drift bug in
		// the expected value and miss loss of position on a width round trip.
		for _, width := range []float32{457, 441, 482, 532, 591, 532, 482, 441, 507} {
			arrange(width)
			index := editor.model.LineAt(old.offset)
			rng, _ := editor.model.LineRange(index)
			p := editor.paragraph(index)
			lines, _ := p.layout.MeasureMetrics()
			line := 0
			for line+1 < len(lines) && lines[line+1].Start <= old.offset-rng.Start {
				line++
			}
			want := editor.heights.Top(index) + lines[line].Y + old.within
			// ScrollView rounds to integral DIP. Native line metrics need not be
			// integral, but a whole line/paragraph jump is never rounding error.
			if abs32(scroll.ScrollY()-want) > 1 {
				t.Fatalf("width=%g old=%+v new=%+v scroll=%g want=%g", width, old, editor.anchor, scroll.ScrollY(), want)
			}
			if editor.anchorPending || scroll.revealPending {
				t.Fatalf("resize did not settle: width=%g old=%+v scroll=%g contentOffset=%g anchor=%+v pending=%t/%t", width, old, scroll.ScrollY(), editor.offset.Y, editor.anchor, editor.anchorPending, scroll.revealPending)
			}
		}
	}
}
