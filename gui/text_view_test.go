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
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

// 多行视图、模型与布局。

// 首末块采用不同字符宽度；修正末块后不能保留旧的最大估宽。
// 宿主走正常 SetScrollX/Arrange；字形替身提供 W=20、i=10 DIP 的独立坐标。
func TestTextViewLongWidthCorrectionReachesTextEnd(t *testing.T) {
	for _, tc := range []struct {
		name, prefix, tail     string
		prefixWidth, tailWidth int
		browse                 bool
	}{
		{"Shrink", "W", "i", 20, 10, false},
		{"Grow", "i", "W", 10, 20, false},
		{"Browse", "i", "W", 10, 20, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, scroll, _, q, _ := timedEditorFixture(t)
			q.app.typo = &tailWidthTypography{}
			v.SetWrapMode(WrapNone)
			length := 20*paragraphBlockBytes + 128
			v.Model().SetText(strings.Repeat(tc.prefix, 20*paragraphBlockBytes) + strings.Repeat(tc.tail, 128))
			box := geometry.Rect(0, 0, 800, 200)
			scroll.Measure(layout.Tight(box.Size))
			scroll.Arrange(box)
			p := v.paragraphs[0]
			if p == nil || p.paged == nil {
				t.Fatal("未进入长段落路径")
			}
			before := v.ContentSize().Width
			target := float32(1 << 30)
			if tc.browse {
				target = before - v.viewport.Width - 300
			}
			scroll.SetScrollX(target)
			initial := scroll.ScrollX()
			for range 6 {
				v.longBudget = 1
				scroll.Arrange(box)
			}
			want := float32(20*paragraphBlockBytes*tc.prefixWidth+128*tc.tailWidth) + 2*v.Padding() + 1
			if v.ContentSize().Width != want {
				t.Fatal("内容宽度不等于字符宽度的独立总和", v.ContentSize(), want)
			}
			if tc.browse {
				if scroll.ScrollX() != initial {
					t.Fatal("普通浏览位置被拉向末尾", initial, scroll.ScrollX())
				}
				return
			}
			end := p.paged.GeometryAt(length)
			if end == nil {
				t.Fatal("末尾几何未就绪")
			}
			caret := end.Caret(textedit.Position{Offset: length, Upstream: true})
			x := float64(v.Padding()) + end.X + float64(caret.X) - float64(scroll.ScrollX())
			if x < float64(v.Padding()) || x > float64(v.viewport.Width-v.Padding()) {
				t.Fatalf("末尾落在视口外：x=%g，offset=%g，content=%v", x, scroll.ScrollX(), v.ContentSize())
			}
		})
	}
}

// 原生窗口淘汰不删除标量宽度；非相交行的插删重映射索引，模型重置释放尺寸元数据。
func TestTextViewLongWidthMetadataLifetime(t *testing.T) {
	v, scroll, win, _, _ := timedEditorFixture(t)
	v.SetWrapMode(WrapNone)
	text := "header\n" + strings.Repeat("a", 17*paragraphBlockBytes+128) + "\nfooter"
	v.Model().SetText(text)
	box := geometry.Rect(0, 0, 800, 200)
	for range 6 {
		v.longBudget = 1
		scroll.Arrange(box)
	}
	width := v.ContentSize().Width
	p := v.paragraphs[1]
	if p == nil || p.paged == nil || v.longWidths[1] <= 0 {
		t.Fatal("长行宽度未记录")
	}
	v.textParagraphCache.remove(p)
	if v.ContentSize().Width != width {
		t.Fatal("淘汰原生窗口改变内容宽度")
	}
	if err := v.Model().Replace(TextRange{}, "\n"); err != nil {
		t.Fatal(err)
	}
	if len(v.longWidths) != 1 || v.longWidths[2] <= 0 || v.ContentSize().Width != width {
		t.Fatal("前缀插入丢失或错配未相交长行", v.longWidths, v.ContentSize())
	}
	line, _ := v.Model().LineRange(2)
	if err := v.Model().Replace(TextRange{Start: line.Start, End: line.End + 1}, ""); err != nil {
		t.Fatal(err)
	}
	if len(v.longWidths) != 0 || v.longWidth != 0 {
		t.Fatal("删除长行后遗留宽度", v.longWidths, v.longWidth)
	}
	v.Model().SetText(text)
	for range 6 {
		v.longBudget = 1
		scroll.Arrange(box)
	}
	if len(v.longWidths) != 1 {
		t.Fatal("模型重载未重建尺寸元数据")
	}
	win.SetWidget(nil)
	if len(v.longWidths) != 0 || v.longWidth != 0 {
		t.Fatal("卸载后遗留尺寸元数据")
	}
}

// W 宽 20 DIP，其余字符宽 10 DIP；不依赖真实字体，也不改动被测布局状态。
type tailWidthTypography struct{ editorTypography }

func (c *tailWidthTypography) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	native, err := c.editorTypography.NewTextLayout(text, format, width, height)
	if err != nil || format.WrapMode != WrapNone {
		return native, err
	}
	p := native.(*testTextLayout)
	x := float32(0)
	for i := range p.clusters {
		c := &p.clusters[i]
		c.X = x
		if text[c.Start] == 'W' {
			c.Width = 20
		}
		x += c.Width
	}
	p.lines[0].Width, p.measureSize.Width = x, x
	return p, nil
}

func TestTextViewTabConsumptionPrecedesNavigation(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "")
	root, next := newTestWidget(), newTestWidget()
	next.SetFocusable(true)
	win.SetWidget(nil)
	root.AddChild(editor)
	root.AddChild(next)
	win.SetWidget(root)
	editor.Arrange(geometry.Rect(0, 0, 209, 108))
	win.SetFocusedWidget(editor)
	editor.SetAcceptsTab(true)
	editorKey(t, win, events.KeyTab, 0)
	if editor.Model().Text() != "\t" || win.FocusedWidget() != editor || editor.FocusVisible() || win.focusVisible {
		t.Fatal("AcceptsTab did not consume Tab before navigation")
	}
	editor.SetAcceptsTab(false)
	editorKey(t, win, events.KeyTab, 0)
	if editor.Model().Text() != "\t" || win.FocusedWidget() != next || !next.Focused() || !next.FocusVisible() {
		t.Fatal("unconsumed Tab did not move focus")
	}
	editorKey(t, win, events.KeyTab, events.ModifierShift)
	if win.FocusedWidget() != editor || !editor.Focused() {
		t.Fatal("Shift+Tab did not restore editor focus")
	}
}

func TestTextViewViewportCacheAndLocalEdit(t *testing.T) {
	editor, _, typo := newEditorFixture(t, strings.Repeat("row\n", 50000))
	if len(editor.paragraphs) > 20 || len(typo.calls) > 25 {
		t.Fatalf("initial layout shaped the document: %d layouts", len(typo.calls))
	}
	untouched := editor.paragraphs[3].layout
	editor.SetSelection(TextSelection{Anchor: 1, Caret: 1})
	count := len(typo.calls)
	editor.Paint(&testLabelPainter{})
	editor.Paint(&testLabelPainter{})
	if len(typo.calls) != count {
		t.Fatal("selection/paint rebuilt text")
	}
	if err := editor.model.Replace(TextRange{Start: 0, End: 0}, "X"); err != nil {
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
	if len(editor.paragraphs) > textParagraphCacheLimit {
		t.Fatalf("scroll retained %d paragraph layouts", len(editor.paragraphs))
	}
	for _, p := range typo.layouts {
		if p == untouched && p.destroyed {
			t.Fatal("recent offscreen layout not retained")
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
	a.SetSelection(TextSelection{Anchor: 1, Caret: 1})
	b.SetSelection(TextSelection{Anchor: 3, Caret: 3})
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

// 使用固定排版的基准，统计编辑器工作量，不代表原生排版或真实桌面操作耗时。

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
				_ = editor.model.Replace(TextRange{Start: 1, End: 2}, "X")
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

// 编译期接口契约。

var _ Scrollable = (*TextView)(nil)

var _ ScrollIntoViewRequester = (*TextView)(nil)

var _ IMClient = (*TextView)(nil)

// 多行预编辑投影与生命周期。

func TestTextViewPreeditProjectionAcrossParagraphs(t *testing.T) {
	editor, win, typo := newEditorFixture(t, "aa\nbb\ncc\ndd")
	editor.SetSelection(TextSelection{Anchor: 7, Caret: 1})
	untouched := editor.paragraphs[3].layout
	count := len(typo.calls)
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "中\nX", Caret: 4})
	if editor.model.Text() != "aa\nbb\ncc\ndd" || editor.model.Revision() != 0 || editor.model.CanUndo() {
		t.Fatal("preedit mutated shared model/history")
	}
	if editor.Selection() != (TextSelection{Anchor: 7, Caret: 1}) {
		t.Fatal("preedit replaced model-space selection")
	}
	if editor.displayLineCount() != 3 || editor.displayParagraph(0) != "a中" || editor.displayParagraph(1) != "Xc" || editor.displayParagraph(2) != "dd" {
		t.Fatal("preedit paragraph projection is wrong")
	}
	if editor.displayLineRange(1) != (TextRange{Start: 5, End: 7}) || editor.displayLineRange(2) != (TextRange{Start: 8, End: 10}) {
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
	if state.Preedit == nil || state.Preedit.Replacement != (TextRange{Start: 1, End: 7}) || state.Preedit.Text != "中\nX" || state.Preedit.Caret != 4 {
		t.Fatalf("preedit snapshot %+v", state.Preedit)
	}
	committed, _ := editor.model.Slice(state.VisibleRange)
	if state.VisibleText != committed {
		t.Fatal("snapshot mixed display offsets with committed content")
	}
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "文"})
	if editor.preedit != nil || editor.model.Text() != "a文c\ndd" || editor.Selection() != (TextSelection{Anchor: 4, Caret: 4}) {
		t.Fatalf("commit %q %+v", editor.model.Text(), editor.Selection())
	}
	editorKey(t, win, events.KeyZ, textCommandModifier())
	if editor.model.Text() != "aa\nbb\ncc\ndd" || editor.Selection() != (TextSelection{Anchor: 7, Caret: 1}) {
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
	a.SetSelection(TextSelection{Anchor: 1, Caret: 8})
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "NEW", Caret: 3})
	if b.preedit != nil || b.displayParagraph(0) != "one" || b.displayLineCount() != 3 {
		t.Fatal("preedit leaked into other view")
	}
	_ = a.model.Replace(TextRange{Start: 0, End: 0}, "x\n")
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
	clip.pending("stale", nil)
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
	editor.SetSelection(TextSelection{Anchor: 3, Caret: 3})
	if editor.preedit != nil || editor.model.Text() != "abc" || editor.Selection() != (TextSelection{Anchor: 3, Caret: 3}) {
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
	_ = editor.model.Replace(TextRange{Start: 0, End: 0}, "new\n")
	for range 3 {
		scroll.Arrange(scroll.Rect())
	}
	if editor.anchor.offset != old.offset+4 || editor.anchor.within != old.within || scroll.ScrollY() != 1025 {
		t.Fatalf("insertion moved anchored text: old=%+v new=%+v scroll=%g", old, editor.anchor, scroll.ScrollY())
	}
	_ = editor.model.Replace(TextRange{Start: 0, End: 4}, "")
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
	if err := editor.model.Replace(TextRange{Start: 0, End: 0}, "new\n"); err != nil {
		t.Fatal(err)
	}
	arrange(220)
	// One inserted paragraph adds 20 DIP, without losing byte 20 in the old
	// paragraph while it was in the middle of the wider display line.
	if scroll.ScrollY() != 41 || editor.anchor.offset != 24 {
		t.Fatalf("edit lost retained model position: scroll=%g anchor=%+v", scroll.ScrollY(), editor.anchor)
	}
}

type fractionalEditorTypography struct {
	editorTypography
	factor float32
}

func (c *fractionalEditorTypography) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, err := c.editorTypography.NewTextLayout(text, format, width, height)
	if err != nil {
		return nil, err
	}
	p := l.(*testTextLayout)
	factor := c.factor
	if factor == 0 {
		factor = 1.025
	}
	// 调宽后也保留小数行高；横向几何仍来自独立的 10 DIP 字格。
	p.resized = func() {
		layoutEditorFixture(p)
		for i := range p.lines {
			p.lines[i].Y *= factor
			p.lines[i].Height *= factor
			p.lines[i].Baseline *= factor
		}
		for i := range p.clusters {
			p.clusters[i].Y *= factor
			p.clusters[i].Height *= factor
		}
		p.measureSize.Height *= factor
	}
	p.resized()
	return p, nil
}

func TestTextViewResizeKeepsAnchorRoundedAboveParagraph(t *testing.T) {
	setTestApplication(t, &fractionalEditorTypography{factor: 1.0125})
	editor := NewTextView()
	editor.SetModel(NewTextModel(strings.Repeat(strings.Repeat("x", 80)+"\n", 100)))
	scroll := NewScrollView()
	scroll.SetChild(editor)
	win := &window{}
	win.SetWidget(scroll)
	defer win.SetWidget(nil)
	arrange := func(width float32) {
		box := geometry.Rect(0, 0, width, 108)
		scroll.Measure(layout.Tight(box.Size))
		for range 8 {
			scroll.Arrange(box)
		}
	}
	arrange(220)
	// 每行 20.25 DIP；第一段四行高 81 DIP，锚点是第二段首字节 81。
	scroll.SetScrollY(81)
	for _, step := range []struct{ width, want float32 }{{320, 61}, {820, 20}, {220, 81}, {820, 20}, {220, 81}} {
		arrange(step.width)
		// 宽 820 时段顶为 20.25 DIP，宿主取整为 20，落在上一段内。
		// 仍应保留第二段的字符；改锚定上一段会使后续调宽丢失整段位置。
		if scroll.ScrollY() != step.want || editor.anchor.offset != 81 || editor.anchor.within != 0 || !editor.anchor.valid {
			t.Fatalf("width=%g scroll=%g want=%g anchor=%+v", step.width, scroll.ScrollY(), step.want, editor.anchor)
		}
		if editor.anchorPending || scroll.revealPending {
			t.Fatal("段落边界的锚点修正未收敛")
		}
	}
	// 主动滚动 1 DIP 仍须重新采样，不能锁死保留的行内偏移。
	scroll.SetScrollY(82)
	arrange(820)
	if scroll.ScrollY() != 21 || editor.anchor.offset != 81 || editor.anchor.within != 1 {
		t.Fatalf("主动滚动未替换锚点：scroll=%g anchor=%+v", scroll.ScrollY(), editor.anchor)
	}
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
	editor.SetSelection(TextSelection{Anchor: editor.model.Len(), Caret: editor.model.Len()})
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
	editor.SetSelection(TextSelection{Anchor: editor.model.Len(), Caret: editor.model.Len()})
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

func TestTextViewFastScrollCoversViewport(t *testing.T) {
	testTextViewFastScrollCoverage(t, &editorTypography{})
}

func testTextViewFastScrollCoverage(t *testing.T, ctx typography.Context) {
	t.Helper()
	setTestApplication(t, ctx)
	var text strings.Builder
	for i := 0; i < 1500; i++ {
		fmt.Fprintf(&text, "[%04d] %s\n", i, strings.Repeat("abc 中文 ffi ", 1+i%9))
	}
	editor := NewTextView()
	editor.SetModel(NewTextModel(text.String()))
	scroll := NewScrollView()
	scroll.SetChild(editor)
	win := &window{}
	win.SetWidget(scroll)
	defer win.SetWidget(nil)
	for _, size := range []geometry.Size{{Width: 220, Height: 108}, {Width: 499, Height: 590}, {Width: 1206, Height: 1200}} {
		box := geometry.Rect(0, 0, size.Width, size.Height)
		scroll.Measure(layout.Tight(size))
		scroll.Arrange(box)
		for _, y := range []float32{20000, 1100, 50000, 0, 35000, 9500, 1e8, 1000} {
			scroll.SetScrollY(y)
			assertTextViewViewportMeasured(t, editor)
			for range 20 {
				if !editor.anchorPending && !scroll.revealPending && !editor.revealCaret {
					break
				}
				scroll.Arrange(box)
				assertTextViewViewportMeasured(t, editor)
			}
			if editor.anchorPending || scroll.revealPending || editor.revealCaret || editor.offset.Y != scroll.ScrollY() {
				t.Fatalf("scroll did not settle: size=%v y=%g host=%g editor=%g", size, y, scroll.ScrollY(), editor.offset.Y)
			}
			assertTextViewViewportMeasured(t, editor)
		}
	}
}

func assertTextViewViewportMeasured(t *testing.T, editor *TextView) {
	t.Helper()
	first := editor.heights.At(max(0, editor.offset.Y-editor.padding))
	last := editor.heights.At(max(0, editor.offset.Y+editor.viewport.Height-editor.padding-1))
	for index := first; index <= last; index++ {
		p := editor.paragraphs[index]
		if p == nil || !p.measured {
			t.Fatalf("unmeasured visible paragraph: viewport=%v offset=%g index=%d visible=%d..%d", editor.viewport, editor.offset.Y, index, first, last)
		}
	}
}

func TestTextViewTrailingOverscanIsBounded(t *testing.T) {
	editor, _, _ := newEditorFixture(t, strings.Repeat("row\n", 5000))
	for _, height := range []float32{400, 1200} {
		editor.LayoutVisible(geometry.Size{Width: 1200, Height: height}, geometry.Point{})
		count := 0
		for index := range editor.paragraphs {
			if editor.heights.Top(index) > height-editor.padding {
				count++
			}
		}
		if count > 4 {
			t.Fatalf("height %g measured %d trailing paragraphs", height, count)
		}
	}
}

// Paragraph cache ownership, invalidation and local work.

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
	// Paragraph sizing now caches native metrics for baseline alignment, while
	// the more expensive editing Geometry remains independent and lazy.
	if p.geometryValid || native.metricsCalls != 1 {
		t.Fatal("display-only paragraph eagerly built editing geometry")
	}
	editor.Paint(&testLabelPainter{})
	if p.geometryValid || native.metricsCalls != 1 {
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
	editor.SetSelection(TextSelection{Anchor: 4, Caret: 6})
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

// Content measurement and ScrollView integration.

func TestTextViewMeasureFollowsDisplayedContent(t *testing.T) {
	v, _, _ := newEditorFixture(t, "") // deterministic 10x20 DIP typography
	for _, tc := range []struct {
		text                   string
		width, maximum, height float32
	}{
		{"", 109, 200, 28}, {"a", 109, 200, 28},
		{"a\n", 109, 200, 48}, {"\n\n", 109, 200, 68},
		{"abcdefghijklmnopqrst", 109, 200, 48},
		{"abcdefghijklmnopqrst", 209, 200, 28},
		{strings.Repeat("row\n", 20), 109, 65, 65},
	} {
		v.Model().SetText(tc.text)
		v.SetPlaceholder(strings.Repeat("placeholder\n", 100))
		before := v.offset
		m := v.Measure(layout.Loose(geometry.Size{Width: tc.width, Height: tc.maximum}))
		if m.Height != tc.height || !m.HasBaseline || m.Baseline != 19 {
			t.Fatalf("text=%q width=%g: got %+v, want height=%g baseline=19", tc.text, tc.width, m, tc.height)
		}
		if v.offset != before {
			t.Fatal("measurement moved the viewport")
		}
	}
}

func TestTextViewMeasureIncludesPreeditWithoutChangingModel(t *testing.T) {
	v, _, _ := newEditorFixture(t, "")
	v.onPreedit("a\nb", 3)
	c := layout.Loose(geometry.Size{Width: 209, Height: 200})
	if m := v.Measure(c); m.Height != 48 || v.Model().Len() != 0 || v.Model().CanUndo() {
		t.Fatalf("preedit measurement changed model or lost LF: %+v", m)
	}
	v.cancelPreedit(false)
	if m := v.Measure(c); m.Height != 28 {
		t.Fatalf("cancelled composition retained height: %+v", m)
	}
}

func TestTextViewBoundedMeasurePreservesExtentAndCacheLimit(t *testing.T) {
	v, _, typo := newEditorFixture(t, strings.Repeat("row\n", 10000))
	v.releaseParagraphs()
	before := len(typo.calls)
	m := v.Measure(layout.Loose(geometry.Size{Width: 209, Height: 65}))
	if m.Height != 65 || len(typo.calls)-before > 4 || v.ContentSize().Height < 200008 {
		t.Fatalf("bounded measurement lost virtualization/extent: size=%v calls=%d content=%v", m, len(typo.calls)-before, v.ContentSize())
	}
	v.SetModel(NewTextModel(strings.Repeat("row\n", 600)))
	m = v.Measure(layout.Unbounded())
	if m.Height != 12028 || len(v.paragraphs) > textParagraphCacheLimit {
		t.Fatalf("unbounded measurement: size=%v retained=%d", m, len(v.paragraphs))
	}
}

func TestScrollViewMeasuresTextHeightAndClampsAfterShrink(t *testing.T) {
	v, win, _ := newEditorFixture(t, "a")
	win.SetWidget(nil)
	scroll := NewScrollView()
	scroll.SetDirection(ScrollVertical)
	scroll.SetMainWeight(0)
	scroll.SetMaxSize(geometry.Size{Height: 65})
	scroll.SetChild(v)
	win.SetWidget(scroll)
	for _, tc := range []struct {
		text   string
		height float32
	}{
		{"a", 28}, {"a\nb", 48}, {"a\nb\nc\nd", 65}, {"a", 28},
	} {
		v.Model().SetText(tc.text)
		m := scroll.Measure(layout.Loose(geometry.Size{Width: 209, Height: 1000}))
		if m.Height != tc.height {
			t.Fatalf("%q: height=%g, want %g", tc.text, m.Height, tc.height)
		}
		scroll.Arrange(geometry.Rectangle{Size: m.Size})
		scroll.SetScrollY(10000)
		if tc.height < 65 && (scroll.ScrollY() != 0 || scroll.vbar.Visible()) {
			t.Fatal("short content retained a scroll offset or bar")
		}
	}
}

func TestTextViewMeasureDoesNotReflowSharedModelPeer(t *testing.T) {
	a, _, _ := newEditorFixture(t, "abcdefghijklmnopqrst")
	b := NewTextView()
	b.SetModel(a.Model())
	t.Cleanup(b.releaseParagraphs)
	t.Cleanup(b.disconnectModel)
	wide := a.Measure(layout.Loose(geometry.Size{Width: 209, Height: 200}))
	narrow := b.Measure(layout.Loose(geometry.Size{Width: 109, Height: 200}))
	if wide.Height != 28 || narrow.Height != 48 || a.ContentSize().Height != 28 {
		t.Fatalf("shared model geometry leaked: wide=%v narrow=%v content=%v", wide, narrow, a.ContentSize())
	}
}

// Wrapped caret reveal.

func TestTextViewCaretMovesReuseWrappedLayout(t *testing.T) {
	v, scroll, win, typo := longWrappedEditor(t)
	scroll.SetScrollY(200)
	p := v.paragraphs[0]
	native := p.layout.(*testTextLayout)
	metrics, layouts := native.metricsCalls, len(typo.calls)
	for _, tc := range []struct {
		name string
		act  func()
		want TextSelection
	}{
		// Width 220 minus the bar, padding and caret reserve gives 20 cells.
		// At scroll Y=200, window (29,14) hits cell 3 on visual line 10.
		{"click", func() {
			editorPointer(t, win, events.PointerDown, geometry.Point{X: 29, Y: 14}, 0)
			editorPointer(t, win, events.PointerUp, geometry.Point{X: 29, Y: 14}, 0)
		}, TextSelection{Anchor: 203, Caret: 203}},
		{"shift click", func() {
			editorPointer(t, win, events.PointerDown, geometry.Point{X: 69, Y: 14}, events.ModifierShift)
			editorPointer(t, win, events.PointerUp, geometry.Point{X: 69, Y: 14}, events.ModifierShift)
		}, TextSelection{Anchor: 203, Caret: 207}},
		{"collapse selection", func() { editorKey(t, win, events.KeyArrowRight, 0) },
			TextSelection{Anchor: 207, Caret: 207}},
		{"arrow", func() { editorKey(t, win, events.KeyArrowRight, 0) },
			TextSelection{Anchor: 208, Caret: 208}},
		{"document end", func() { editorKey(t, win, events.KeyEnd, textCommandModifier()) },
			TextSelection{Anchor: 10000, Caret: 10000}},
		{"document start", func() { editorKey(t, win, events.KeyHome, textCommandModifier()) },
			TextSelection{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.act()
			if got := v.Selection(); got != tc.want {
				t.Fatalf("selection=%+v, want %+v", got, tc.want)
			}
			if win.layoutDirty {
				t.Fatal("caret movement requested content measurement")
			}
			settleEditorLayout(t, win)
			if v.paragraphs[0] != p || native.metricsCalls != metrics || len(typo.calls) != layouts {
				t.Fatal("caret movement rebuilt wrapped paragraph metrics")
			}
			rect, ok := v.caretRect()
			if !ok || rect.Y < scroll.ScrollY() || rect.Y+rect.Height > scroll.ScrollY()+v.viewport.Height {
				t.Fatalf("caret not revealed: rect=%v offset=%g", rect, scroll.ScrollY())
			}
		})
	}
	if scroll.ScrollY() != 0 {
		t.Fatal("document start retained the old scroll offset")
	}
	editorPointer(t, win, events.PointerDown, geometry.Point{X: 29, Y: 14}, 0)
	editorPointer(t, win, events.PointerMove, geometry.Point{X: 89, Y: 54}, 0)
	editorPointer(t, win, events.PointerUp, geometry.Point{X: 89, Y: 54}, 0)
	if v.Selection() != (TextSelection{Anchor: 3, Caret: 49}) || win.layoutDirty || native.metricsCalls != metrics {
		t.Fatalf("drag selection invalidated layout: %+v dirty=%v", v.Selection(), win.layoutDirty)
	}
}

// Measured fragment ownership.

func TestTextViewOwnsMeasuredFragmentAcrossNativeResize(t *testing.T) {
	v, _, _ := newEditorFixture(t, strings.Repeat("abcdefghijklmnopqrst", 200))
	p := v.paragraph(0)
	p.editGeometry(v.lineHeight)
	f := p.fragment
	if f.LineCount() < 100 {
		t.Fatal("fixture did not wrap across many pages", f.LineCount())
	}
	before := f.Geometry(v.lineHeight)
	pos := textedit.Position{Offset: 201, Upstream: false}
	want := before.Caret(pos)
	// Native objects can be resized independently; retained values must not
	// silently change into the new width's geometry through borrowed slices.
	p.layout.SetSize(49, 1<<20)
	p.layout.MeasureMetrics()
	after := f.Geometry(v.lineHeight)
	if got := after.Caret(pos); got != want || got.Width != 1 {
		t.Fatal("snapshot changed with native layout", got, want)
	}
	view, err := f.LineRange(32, 34)
	if err != nil {
		t.Fatal(err)
	}
	g := view.Geometry(v.lineHeight)
	if g.Hit(geometry.Point{X: 10, Y: 1}).Offset < 0 || view.Range().End <= view.Range().Start {
		t.Fatal("invalid retained line fragment")
	}
}

// Bounded initial height estimation.

func TestTextViewInitialWrappedEstimateUsesOnlyBoundedSample(t *testing.T) {
	for _, text := range []string{strings.Repeat("abc ", 3000), strings.Repeat("中文 e\u0301 👩‍💻 ", 1000)} {
		typo := &editorTypography{}
		setTestApplication(t, typo)
		v := NewTextView()
		v.SetModel(NewTextModel(text))
		v.ensureFormat()
		calls := len(typo.calls)
		v.Measure(layout.Tight(geometry.Size{Width: 220, Height: 108}))
		if len(typo.calls) != calls+1 || len(typo.calls[calls].text) > 512 || v.heights.Total() <= 108 || len(v.paragraphs) != 0 {
			t.Fatal("natural measure did not use a bounded, provisional wrapped extent")
		}
		if !typo.layouts[calls].destroyed {
			t.Fatal("provisional sample retained native resources")
		}
		v.Measure(layout.Tight(geometry.Size{Width: 220, Height: 108}))
		if len(typo.calls) != calls+1 {
			t.Fatal("repeated natural measurement repeated the sample")
		}
		scroll := NewScrollView()
		scroll.SetChild(v)
		scroll.Measure(layout.Tight(geometry.Size{Width: 220, Height: 108}))
		scroll.Arrange(geometry.Rect(0, 0, 220, 108))
		full := 0
		for _, call := range typo.calls {
			if call.text == text {
				full++
			}
		}
		if full != 1 || v.paragraph(0).height != v.heights.Total() {
			t.Fatal("initial scrollbar allocation repeated full layout or kept estimated geometry")
		}
		v.releaseParagraphs()
	}
}

// Exact measurement reuse across editing history.

func TestTextViewHistoryReusesExactParagraphMeasurements(t *testing.T) {
	v, scroll, win, typo := longWrappedEditor(t)
	defer win.SetWidget(nil)
	original := v.paragraphs[0]
	if err := v.model.ReplaceAtomic(textedit.Range{}, "x"); err != nil {
		t.Fatal(err)
	}
	scroll.Arrange(scroll.Rect())
	edited := v.paragraphs[0]
	created := len(typo.calls)
	for range 5 {
		if !v.model.Undo() {
			t.Fatal("undo failed")
		}
		scroll.Arrange(scroll.Rect())
		if v.paragraphs[0] != original {
			t.Fatal("undo remeasured an identical complete input")
		}
		if !v.model.Redo() {
			t.Fatal("redo failed")
		}
		scroll.Arrange(scroll.Rect())
		if v.paragraphs[0] != edited {
			t.Fatal("redo remeasured an identical complete input")
		}
	}
	if len(typo.calls) != created {
		t.Fatal("history created new native layouts")
	}
	checkParagraphCache(t, &v.textParagraphCache)
}

func TestTextViewReplacedParagraphCacheIsBoundedAndReleased(t *testing.T) {
	v, scroll, win, typo := longWrappedEditor(t)
	for range 12 {
		if err := v.model.ReplaceAtomic(textedit.Range{}, "x"); err != nil {
			t.Fatal(err)
		}
		scroll.Arrange(scroll.Rect())
		if len(v.replacedParagraphs) > textReplacedCacheLimit || v.replacedBytes > textParagraphCacheBytes {
			t.Fatal("replaced input cache exceeded its bound")
		}
		for _, old := range v.replacedParagraphs {
			for _, live := range v.paragraphs {
				if old == live || old.layout == live.layout {
					t.Fatal("native layout has two owners")
				}
			}
		}
	}
	if len(v.replacedParagraphs) == 0 {
		t.Fatal("fixture did not retain replaced input")
	}
	win.SetWidget(nil)
	if len(v.replacedParagraphs) != 0 || v.replacedBytes != 0 {
		t.Fatal("unmount retained history resources")
	}
	for _, native := range typo.layouts {
		if !native.destroyed {
			t.Fatal("unmount leaked a native history layout")
		}
	}
}

func TestTextViewReplacedParagraphsDoNotSurviveColorChange(t *testing.T) {
	v, scroll, win, _ := longWrappedEditor(t)
	defer win.SetWidget(nil)
	original := v.paragraphs[0]
	if err := v.model.ReplaceAtomic(textedit.Range{}, "x"); err != nil {
		t.Fatal(err)
	}
	scroll.Arrange(scroll.Rect())
	app := App.(*application)
	app.style = style.Sheet(append(DefaultStyleRules(), style.Name(styleNameTextView).ForegroundColor(color.RGBA{R: 120, A: 255}))...)
	v.StyleChanged()
	if len(v.replacedParagraphs) != 0 || !original.layout.(*testTextLayout).destroyed {
		t.Fatal("color change retained obsolete history colors")
	}
}

type historyDestroyLayout struct {
	typography.TextLayout
	after func()
}

func (l *historyDestroyLayout) Destroy() {
	l.TextLayout.Destroy()
	if l.after != nil {
		fn := l.after
		l.after = nil
		fn()
	}
}

func TestTextViewHistoryEvictionReentryPreservesReplacementModel(t *testing.T) {
	v, scroll, win, _ := longWrappedEditor(t)
	defer win.SetWidget(nil)
	for range 4 {
		if err := v.model.ReplaceAtomic(textedit.Range{}, "x"); err != nil {
			t.Fatal(err)
		}
		scroll.Arrange(scroll.Rect())
	}
	replacement := textedit.NewModel(strings.Repeat("new model\n", 5))
	old := v.replacedParagraphs[0]
	old.layout = &historyDestroyLayout{TextLayout: old.layout, after: func() { v.SetModel(replacement) }}
	if err := v.model.ReplaceAtomic(textedit.Range{}, "x"); err != nil {
		t.Fatal(err)
	}
	scroll.Arrange(geometry.Rectangle{Size: scroll.Rect().Size})
	if v.model != replacement || v.seenRevision != replacement.Revision() || v.heights.Count() != replacement.LineCount() {
		t.Fatal("interrupted edit overwrote replacement model metadata")
	}
	for row, p := range v.paragraphs {
		r, _ := replacement.LineRange(row)
		text, _ := replacement.Slice(r)
		if p.layout.Text() != text {
			t.Fatal("interrupted edit published a stale paragraph")
		}
	}
}

// Bounded measurement and retained viewport geometry.

func TestTextViewLargeMeasureDoesNotWalkDocument(t *testing.T) {
	v, _, typo := newEditorFixture(t, strings.Repeat("row\n", 100000))
	v.releaseParagraphs()
	before := len(typo.calls)
	m := v.Measure(layout.Unbounded())
	if len(typo.calls) != before || len(v.paragraphs) != 0 {
		t.Fatalf("natural sizing measured document paragraphs: %d new layouts", len(typo.calls)-before)
	}
	if m.Height != 2000028 || m.Baseline != 19 || !m.HasBaseline {
		t.Fatalf("natural sizing lost paragraph extent or baseline: %+v", m)
	}
}

func TestTextViewLargePreeditMeasurePreservesGeometry(t *testing.T) {
	v, _, typo := newEditorFixture(t, "short text")
	v.preedit = textedit.NewProjection(v.model, textedit.Selection{})
	v.preedit.Update(strings.Repeat("a", 32768), 0)
	width, before := v.layoutWidth, len(typo.calls)
	v.Measure(layout.Unbounded())
	if v.layoutWidth != width || len(typo.calls) != before {
		t.Fatal("size query synchronously measured the large preedit projection")
	}
}

func TestTextViewLargeMeasurePreservesAllocatedGeometry(t *testing.T) {
	v, _, _ := newEditorFixture(t, strings.Repeat("a", 32768))
	p := v.paragraphs[0]
	native := p.layout.(*testTextLayout)
	width, metrics := v.layoutWidth, native.metricsCalls
	before := v.heights.Total()
	for _, c := range []layout.Constraint{
		layout.Unbounded(),
		layout.Loose(geometry.Size{Width: 109, Height: 300}),
		layout.Tight(geometry.Size{Width: 209, Height: 200}),
	} {
		v.Measure(c)
		if v.layoutWidth != width || !p.measured || native.metricsCalls != metrics || v.heights.Total() != before {
			t.Fatal("size-only query invalidated the allocated paragraph")
		}
	}
}

func TestTextViewHeightResizeDoesNotMeasureOuterScrollWidth(t *testing.T) {
	v, scroll, win, _ := longWrappedEditor(t)
	p := v.paragraphs[0]
	native := p.layout.(*testTextLayout)
	metrics := native.metricsCalls
	for _, height := range []float32{208, 80, 108} {
		size := geometry.Size{Width: 220, Height: height}
		// A linear container first requests a natural height, then allocates
		// the viewport. Its outer width includes the vertical scrollbar.
		scroll.Measure(layout.Loose(geometry.Size{Width: 220, Height: layout.Inf}))
		scroll.Measure(layout.Tight(size))
		scroll.Arrange(geometry.Rectangle{Size: size})
		if native.metricsCalls != metrics || v.paragraphs[0] != p || v.layoutWidth != 203 {
			t.Fatalf("height-only allocation reflowed text: metrics=%d/%d width=%g", native.metricsCalls, metrics, v.layoutWidth)
		}
	}
	win.SetWidget(nil)
}

func TestTextViewParagraphEditPreservesWrappedExtentUntilLayout(t *testing.T) {
	v, scroll, win, _ := longWrappedEditor(t)
	defer win.SetWidget(nil)
	height := v.heights.Total()
	if err := v.model.ReplaceAtomic(textedit.Range{}, "x"); err != nil {
		t.Fatal(err)
	}
	if v.heights.Total() != height || v.paragraphs[0] != nil {
		t.Fatal("edit collapsed the wrapped height estimate or retained stale text")
	}
	scroll.Arrange(scroll.Rect())
	p := v.paragraphs[0]
	if p == nil || p.layout.(*testTextLayout).metricsCalls != 1 || v.layoutWidth != 203 {
		t.Fatal("one edit measured the entire paragraph at multiple scrollbar widths")
	}
	if err := v.model.ReplaceAtomic(textedit.Range{End: v.model.Len()}, ""); err != nil {
		t.Fatal(err)
	}
	// The retained extent is an estimate, not a minimum. Exact empty-content
	// measurement must remove the scrollbar and finish at the small height.
	for range 3 {
		scroll.Arrange(scroll.Rect())
	}
	if v.ContentSize().Height != 28 || v.heights.Total() != 20 {
		t.Fatal("deleting the whole paragraph retained a stale scroll extent", v.ContentSize())
	}
}

// Coalesced resize and displayed hit geometry.

func resizeEditorFixture(t *testing.T) (*TextView, *ScrollView, *window, *timerTestQueue, *editorTypography) {
	t.Helper()
	v, scroll, win, q, typo := timedEditorFixture(t)
	v.Model().SetText(strings.Repeat("abcdefghij ", 3200))
	scroll.Arrange(geometry.Rect(0, 0, 220, 200))
	return v, scroll, win, q, typo
}

func TestTextViewResizeCoalescesLatestWidth(t *testing.T) {
	v, scroll, _, q, typo := resizeEditorFixture(t)
	old := v.paragraph(0)
	old.editGeometry(v.lineHeight)
	width, before := v.layoutWidth, len(typo.calls)
	for _, w := range []float32{250, 270, 300} {
		scroll.Arrange(geometry.Rect(0, 0, w, 200))
		q.advance(20 * time.Millisecond)
		if v.paragraph(0) != old || v.layoutWidth != width || len(typo.calls) != before {
			t.Fatal("intermediate allocation synchronously remeasured the paragraph")
		}
		if !old.geometryValid || v.heights.Total() != old.height {
			t.Fatal("pending resize lost its displayed geometry or height")
		}
		if v.ContentSize().Width != v.viewport.Width {
			t.Fatal("retained old width introduced a horizontal scrollbar")
		}
		meta, ok := v.Snapshot().Attributes["goui.text"].(map[string]any)
		if !ok || meta["layoutPending"] != true || meta["layoutWidth"] != width {
			t.Fatal("snapshot presented an old layout as the new width", meta)
		}
	}
	q.advance(textResizeDelay)
	q.dispatch(t)
	if !v.resizeReady || len(typo.calls) != before {
		t.Fatal("timer measured text instead of requesting layout")
	}
	scroll.Arrange(geometry.Rect(0, 0, 300, 200))
	if v.layoutWidth != 283 || v.paragraph(0) == old || len(typo.calls) != before+1 || v.resizeWidth != 0 {
		t.Fatal("settled allocation did not measure only the latest width")
	}
	before = len(typo.calls)
	scroll.Arrange(geometry.Rect(0, 0, 220, 280))
	if v.paragraph(0) != old || len(typo.calls) != before || v.resizeWidth != 0 {
		t.Fatal("cached width restoration was delayed or remeasured")
	}
}

func TestTextViewResizeCancelsObsoleteWork(t *testing.T) {
	for _, action := range []string{"return", "newer", "edit", "model", "unmount"} {
		t.Run(action, func(t *testing.T) {
			v, scroll, win, q, typo := resizeEditorFixture(t)
			old := v.paragraph(0)
			scroll.Arrange(geometry.Rect(0, 0, 300, 200))
			q.advance(textResizeDelay) // An old wake has already been posted.
			before := len(typo.calls)
			switch action {
			case "return":
				scroll.Arrange(geometry.Rect(0, 0, 220, 200))
			case "newer":
				scroll.Arrange(geometry.Rect(0, 0, 320, 200))
			case "edit":
				if err := v.Model().ReplaceAtomic(textedit.Range{}, "x"); err != nil {
					t.Fatal(err)
				}
			case "model":
				v.SetModel(NewTextModel("replacement"))
			case "unmount":
				win.SetWidget(nil)
			}
			q.dispatch(t)
			if len(typo.calls) != before || v.resizeReady {
				t.Fatal("obsolete posted timer measured or published a width")
			}
			if action == "newer" {
				q.advance(textResizeDelay)
				q.dispatch(t)
				scroll.Arrange(geometry.Rect(0, 0, 320, 200))
				if v.layoutWidth != 303 || len(typo.calls) != before+1 {
					t.Fatal("new allocation did not supersede the posted width")
				}
			} else if v.resizeWidth != 0 || v.resizeTimer != nil && v.resizeTimer.Active() {
				t.Fatal("cancelled allocation retained pending work")
			}
			if action == "return" && v.paragraph(0) != old {
				t.Fatal("return to displayed width discarded its exact geometry")
			}
		})
	}
}

func TestTextViewPendingResizeUsesDisplayedHitGeometry(t *testing.T) {
	v, scroll, win, q, typo := resizeEditorFixture(t)
	old := v.paragraph(0)
	old.editGeometry(v.lineHeight)
	width, before := v.layoutWidth, len(typo.calls)
	scroll.Arrange(geometry.Rect(0, 0, 300, 200))
	point := geometry.Point{X: 29, Y: 14}
	want := old.geometry.Hit(geometry.Point{X: point.X - v.padding, Y: point.Y - v.padding}).Offset
	editorPointer(t, win, events.PointerDown, point, 0)
	editorPointer(t, win, events.PointerUp, point, 0)
	if v.Selection() != (TextSelection{Anchor: want, Caret: want}) || v.paragraph(0) != old || len(typo.calls) != before {
		t.Fatal("click did not use the actually displayed layout", v.Selection(), want)
	}
	q.advance(textResizeDelay / 2)
	scroll.Arrange(geometry.Rect(0, 0, 300, 240))
	q.advance(textResizeDelay / 2)
	q.dispatch(t)
	if !v.resizeReady || v.layoutWidth != width {
		t.Fatal("height-only allocation restarted the width wait or measured inside the timer")
	}
	scroll.Arrange(geometry.Rect(0, 0, 300, 240))
	if v.layoutWidth != 283 || v.Selection().Caret != want {
		t.Fatal("new width lost the position selected in the retained layout")
	}
}

// Width cache reuse, bounds and reentry.

func TestTextViewLongWidthsReuseExactMeasurements(t *testing.T) {
	v, scroll, win, typo := longWrappedEditor(t)
	a := v.paragraph(0)
	a.editGeometry(v.lineHeight)
	show := func(width float32) *textParagraph {
		scroll.Arrange(geometry.Rect(0, 0, width, 108))
		return v.paragraph(0)
	}
	b := show(310)
	if a == b || a.layout == b.layout {
		t.Fatal("different long-paragraph widths need independent native resources")
	}
	created := len(typo.calls)
	am, bm := a.layout.(*testTextLayout).metricsCalls, b.layout.(*testTextLayout).metricsCalls
	for range 5 {
		if show(220) != a || show(310) != b {
			t.Fatal("restoring a width did not reuse its exact paragraph")
		}
		if v.heights.Total() != b.height || !a.geometryValid {
			t.Fatal("restored height or retained editing geometry was lost")
		}
		checkParagraphCache(t, &v.textParagraphCache)
	}
	if len(typo.calls) != created || a.layout.(*testTextLayout).metricsCalls != am || b.layout.(*testTextLayout).metricsCalls != bm {
		t.Fatal("a measured width performed new native measurement")
	}
	if err := v.model.ReplaceAtomic(textedit.Range{}, "changed "); err != nil {
		t.Fatal(err)
	}
	if p := show(220); p == a || p.layout.Text() != v.model.Text() {
		t.Fatal("width restoration resurrected obsolete text")
	}
	if !v.model.Undo() || show(220) != a {
		t.Fatal("history could not reuse the original text and width")
	}
	win.SetWidget(nil)
	for _, l := range typo.layouts {
		if !l.destroyed {
			t.Fatal("unmount leaked a width or history resource")
		}
	}
}

func TestTextViewLongWidthCacheBoundsAndReentry(t *testing.T) {
	v, scroll, win, typo := longWrappedEditor(t)
	for width := float32(230); width < 250; width++ {
		scroll.Arrange(geometry.Rect(0, 0, width, 108))
		if len(v.replacedParagraphs) > textReplacedCacheLimit || v.replacedBytes > textParagraphCacheBytes {
			t.Fatal("continuous resize exceeded retained-input bounds")
		}
		for _, old := range v.replacedParagraphs {
			if old == v.paragraphs[0] || old.layout.(*testTextLayout).destroyed {
				t.Fatal("retired width is shared with the live paragraph or already destroyed")
			}
		}
	}
	old := v.replacedParagraphs[0]
	replacement := NewTextModel("replacement during width eviction")
	old.layout = &historyDestroyLayout{TextLayout: old.layout, after: func() { v.SetModel(replacement) }}
	scroll.Arrange(geometry.Rect(0, 0, 270, 108))
	if v.model != replacement || v.heights.Count() != 1 {
		t.Fatal("width eviction overwrote a reentrant replacement model")
	}
	for _, p := range v.paragraphs {
		if p.layout.Text() != replacement.Text() {
			t.Fatal("width eviction published obsolete text after reentry")
		}
	}
	win.SetWidget(nil)
	for _, l := range typo.layouts {
		if !l.destroyed {
			t.Fatal("eviction or unmount leaked a native width")
		}
	}
}

// 其他行撑大的滚动范围不能让当前已经选至末尾的短行持续向右滚动。
func TestTextViewShortLineDragStopsHorizontalScroll(t *testing.T) {
	v, scroll, win, q, _ := timedEditorFixture(t)
	v.SetWrapMode(WrapNone)
	v.Model().SetText(strings.Repeat("x", 120) + "\nshort")
	win.width, win.height = 220, 108
	win.RequestLayout()
	settleEditorLayout(t, win)
	editorPointer(t, win, events.PointerDown, geometry.Point{X: 15, Y: 30}, 0)
	editorPointer(t, win, events.PointerMove, geometry.Point{X: 300, Y: 30}, 0)
	if v.scrollTimer != nil && v.scrollTimer.Active() {
		t.Fatal("短行行尾仍启动自动滚动时钟")
	}
	for range 4 {
		q.advance(textScrollInterval)
		for len(q.posts) > 0 {
			q.dispatch(t)
		}
	}
	if scroll.ScrollX() != 0 || v.Selection().Range().End != 126 {
		t.Fatalf("短行末尾继续横滚：offset=%g selection=%v", scroll.ScrollX(), v.Selection())
	}
	editorPointer(t, win, events.PointerUp, geometry.Point{X: 300, Y: 30}, 0)
	editorPointer(t, win, events.PointerDown, geometry.Point{X: 15, Y: 10}, 0)
	editorPointer(t, win, events.PointerMove, geometry.Point{X: 300, Y: 10}, 0)
	q.advance(textScrollInterval)
	q.dispatch(t)
	if scroll.ScrollX() <= 0 || v.Selection().Caret <= 20 {
		t.Fatal("长行拖选无法继续横滚", scroll.ScrollX(), v.Selection())
	}
}
