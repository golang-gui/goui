package gui

import (
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

type placeholderEditor interface {
	Widget
	Placeholder() string
	SetPlaceholder(string)
}

func placeholderFixture(t *testing.T, singleLine bool) (placeholderEditor, *textEditor, *window, *editorTypography) {
	t.Helper()
	if singleLine {
		w, win, typo := newInputFixture(t, "")
		return w, w.textEditor, win, typo
	}
	w, win, typo := newEditorFixture(t, "")
	return w, w.textEditor, win, typo
}

func TestPlaceholderConfigurationDoesNotEdit(t *testing.T) {
	for _, singleLine := range []bool{false, true} {
		t.Run(map[bool]string{true: "input", false: "view"}[singleLine], func(t *testing.T) {
			w, e, win, _ := placeholderFixture(t, singleLine)
			if w.Placeholder() != "" {
				t.Fatal("default hint is not empty")
			}
			if err := e.model.Replace(TextRange{}, "abc"); err != nil {
				t.Fatal(err)
			}
			e.setSelectionValue(TextSelection{Anchor: 1, Caret: 2})
			win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "候选", Caret: 3})
			preedit, selection, revision := e.preedit, e.selection, e.model.Revision()
			changes, selections := 0, 0
			e.connectChange(func(TextChange) { changes++ })
			e.connectSelection(func(TextSelection) { selections++ })
			w.SetPlaceholder("hint\r\n中\xff")
			want := "hint\n中�"
			if singleLine {
				want = "hint 中�"
			}
			if w.Placeholder() != want || e.model.Text() != "abc" || e.model.Revision() != revision ||
				e.selection != selection || e.preedit != preedit || preedit == nil || !e.model.CanUndo() || changes != 0 || selections != 0 {
				t.Fatal("setting hint changed editing state or failed normalization")
			}
			info := w.Snapshot()
			if info.TextEditing.Placeholder != want || info.TextEditing.Length != 3 || info.TextEditing.Preedit == nil || info.TextEditing.VisibleText != "abc" {
				t.Fatalf("hint leaked into document semantics: %+v", info.TextEditing)
			}
			win.layoutDirty, win.paintDirty = false, false
			w.SetPlaceholder(want)
			if win.layoutDirty || win.paintDirty {
				t.Fatal("equal normalized hint invalidated the view")
			}
			w.SetPlaceholder("")
			if win.layoutDirty || !win.paintDirty || e.preedit != preedit || changes != 0 || selections != 0 {
				t.Fatal("clearing hint changed layout, IME or emitted edit signals")
			}
		})
	}
}

func TestPlaceholderPaintTracksRealInputAndPreedit(t *testing.T) {
	for _, singleLine := range []bool{false, true} {
		t.Run(map[bool]string{true: "input", false: "view"}[singleLine], func(t *testing.T) {
			w, e, win, _ := placeholderFixture(t, singleLine)
			w.SetPlaceholder("Hint")
			paint := func(want string) {
				t.Helper()
				w.Arrange(geometry.Rect(0, 0, 100, 60))
				p := &testTextInputPainter{}
				w.Paint(p)
				got := ""
				if p.textLayout != nil {
					got = p.textLayout.Text()
				}
				if got != want {
					t.Fatalf("painted %q, want %q", got, want)
				}
			}
			paint("Hint") // Focused, with caret on top of the hint.
			win.SetFocusedWidget(nil)
			paint("Hint")
			win.SetFocusedWidget(w)
			win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "中", Caret: 3})
			paint("中")
			if e.model.Len() != 0 || w.Snapshot().TextEditing.VisibleText != "" {
				t.Fatal("preedit or hint entered the model")
			}
			win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit})
			paint("Hint")
			win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: " "})
			paint(" ")
			editorKey(t, win, events.KeyZ, textCommandModifier())
			paint("Hint")
			editorKey(t, win, events.KeyZ, textCommandModifier()|events.ModifierShift)
			paint(" ")
			editorKey(t, win, events.KeyBackspace, 0)
			paint("Hint")
			e.setReadOnly(true)
			paint("Hint")
			w.SetEnabled(false)
			paint("Hint")
			w.SetPlaceholder("")
			paint("")
		})
	}
}

func TestPlaceholderSharedModelKeepsViewStateIndependent(t *testing.T) {
	a, win, _ := newEditorFixture(t, "")
	b := NewTextView()
	b.SetModel(a.Model())
	b.SetPlaceholder("second\r\nline")
	a.SetPlaceholder("first")
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "中", Caret: 3})
	if a.showsPlaceholder() || !b.showsPlaceholder() || b.Placeholder() != "second\nline" {
		t.Fatal("shared document shared hint/preedit presentation")
	}
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "x"})
	if a.showsPlaceholder() || b.showsPlaceholder() || b.Model().Text() != "x" {
		t.Fatal("shared commit did not hide both hints")
	}
	a.SetModel(NewTextModel(""))
	if a.Placeholder() != "first" || !a.showsPlaceholder() || b.showsPlaceholder() {
		t.Fatal("model replacement lost or shared view configuration")
	}
	b.Model().SetText("\n")
	if b.showsPlaceholder() {
		t.Fatal("a newline is real document content")
	}
}

func TestPlaceholderBaselineClipAndMeasurement(t *testing.T) {
	setTestApplication(t, &baselineInputTypography{})
	w := NewTextInput()
	before := w.Measure(layout.Unbounded())
	w.SetPlaceholder("中文 emoji 👩‍💻 " + strings.Repeat("long ", 100))
	if after := w.Measure(layout.Unbounded()); before != after {
		t.Fatalf("hint changed measurement: %+v => %+v", before, after)
	}
	for _, extra := range []float32{0, 20} {
		w.Arrange(geometry.Rect(0, 0, 100, before.Height+extra))
		size, offset := w.contentSize(), w.offset
		p := &testTextInputPainter{}
		w.Paint(p)
		lines, _ := p.textLayout.MeasureMetrics()
		if got := p.textOrigin.Y + lines[0].Baseline; got != before.Baseline+extra/2 {
			t.Fatalf("hint baseline %g, want %g", got, before.Baseline+extra/2)
		}
		if p.clipRect != geometry.Rect(4, 4, 92, before.Height+extra-8) || w.contentSize() != size || w.offset != offset {
			t.Fatal("hint changed content geometry or escaped content clipping")
		}
	}
}

func TestPlaceholderCacheStyleWidthAndUnmount(t *testing.T) {
	w, win, typo := newEditorFixture(t, "")
	w.SetStyleName("custom-editor")
	setStyle := func(size float32, fg color.Color) {
		App.SetStyleSheet(style.Sheet(append(DefaultStyleRules(),
			style.Name("custom-editor").FontSize(size),
			style.Name("custom-editor").Part("placeholder").ForegroundColor(fg),
		)...))
		w.StyleChanged()
	}
	setStyle(14, color.Black)
	w.SetPlaceholder(strings.Repeat("word ", 20))
	w.Arrange(geometry.Rect(0, 0, 100, 60))
	size := w.ContentSize()
	w.Paint(&testTextInputPainter{})
	first, calls := w.placeholderLayout, len(typo.calls)
	if first == nil || first.format.WrapMode != WrapWordChar || first.width != 91 || w.ContentSize() != size {
		t.Fatal("hint did not wrap to the view without changing document size")
	}
	w.Paint(&testTextInputPainter{})
	if w.placeholderLayout != first || len(typo.calls) != calls {
		t.Fatal("ordinary repaint reshaped hint")
	}
	setStyle(14, color.White)
	w.Paint(&testTextInputPainter{})
	if w.placeholderLayout != first || !colors.Equal(first.layout.(*testTextLayout).format.TextColor, color.White) {
		t.Fatal("color change failed or recreated layout")
	}
	w.Arrange(geometry.Rect(0, 0, 200, 60))
	w.Paint(&testTextInputPainter{})
	second := w.placeholderLayout
	if second == first || !first.layout.(*testTextLayout).destroyed || second.width != 191 {
		t.Fatal("wrapped width change retained stale hint geometry")
	}
	w.SetWrapMode(WrapNone)
	w.Arrange(geometry.Rect(0, 0, 200, 60))
	w.Paint(&testTextInputPainter{})
	if w.placeholderLayout.format.WrapMode != WrapNone || w.placeholderLayout.width != textInputMeasureExtent {
		t.Fatal("hint ignored WrapNone")
	}
	third := w.placeholderLayout
	setStyle(20, color.White)
	w.Paint(&testTextInputPainter{})
	if w.placeholderLayout == third || !third.layout.(*testTextLayout).destroyed {
		t.Fatal("font change retained native hint layout")
	}
	last := w.placeholderLayout
	win.SetWidget(nil)
	if w.placeholderLayout != nil || !last.layout.(*testTextLayout).destroyed {
		t.Fatal("unmount retained hint layout")
	}
	win.SetWidget(w)
	w.Arrange(geometry.Rect(0, 0, 200, 60))
	w.Paint(&testTextInputPainter{})
	if w.placeholderLayout == nil || w.placeholderLayout == last {
		t.Fatal("remount did not rebuild hint")
	}
}

func TestPlaceholderColorNotificationCanUnmount(t *testing.T) {
	w, win, _ := newEditorFixture(t, "")
	w.SetPlaceholder("hint")
	w.Paint(&testTextInputPainter{})
	old := w.placeholderLayout
	old.layout.ConnectChanged(func() { win.SetWidget(nil) })
	App.SetStyleSheet(style.Sheet(append(DefaultStyleRules(),
		style.Name("text-view").Part("placeholder").ForegroundColor(color.White),
	)...))
	w.Paint(&testTextInputPainter{})
	if w.placeholderLayout != nil || !old.layout.(*testTextLayout).destroyed {
		t.Fatal("unmount inside layout notification retained resources")
	}
}

func TestPlaceholderLayoutsBelongToEachView(t *testing.T) {
	setTestApplication(t, &editorTypography{})
	a, b := NewTextView(), NewTextView()
	b.SetModel(a.Model())
	box := NewLinearBox(layout.DirectionHorizontal)
	box.AddChild(a)
	box.AddChild(b)
	win := &window{}
	win.SetWidget(box)
	t.Cleanup(func() { win.SetWidget(nil) })
	for _, w := range []*TextView{a, b} {
		w.SetPlaceholder("same hint")
		w.Arrange(geometry.Rect(0, 0, 100, 60))
		w.Paint(&testTextInputPainter{})
	}
	first, second := a.placeholderLayout.layout.(*testTextLayout), b.placeholderLayout.layout.(*testTextLayout)
	if first == second {
		t.Fatal("shared model/text also shared mutable native hint layouts")
	}
	a.SetPlaceholder("")
	if !first.destroyed || second.destroyed || b.placeholderLayout == nil {
		t.Fatal("clearing one view's hint affected the other")
	}
	b.Paint(&testTextInputPainter{})
}

type reentrantPlaceholderTypography struct {
	editorTypography
	created func(typography.TextLayout)
}

func (c *reentrantPlaceholderTypography) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, err := c.editorTypography.NewTextLayout(text, format, width, height)
	if text == "old hint" && c.created != nil {
		c.created(l)
	}
	return l, err
}

func TestPlaceholderCreationRejectsReentrantConfiguration(t *testing.T) {
	typo := &reentrantPlaceholderTypography{}
	setTestApplication(t, typo)
	w := NewTextInput()
	w.Arrange(geometry.Rect(0, 0, 100, 40))
	w.SetPlaceholder("old hint")
	var stale *testTextLayout
	typo.created = func(l typography.TextLayout) {
		stale = l.(*testTextLayout)
		w.SetPlaceholder("new hint")
	}
	w.Paint(&testTextInputPainter{})
	if stale == nil || !stale.destroyed || w.placeholderLayout != nil {
		t.Fatal("stale native layout survived reentrant hint change")
	}
	p := &testTextInputPainter{}
	w.Paint(p)
	if p.textLayout == nil || p.textLayout.Text() != "new hint" {
		t.Fatal("new hint did not render after discarding obsolete result")
	}
}

// Check rendering order independently of native glyph rasterization.
type placeholderOrderPainter struct {
	testTextInputPainter
	ops []string
}

func (p *placeholderOrderPainter) DrawTextLayout(origin geometry.Point, l typography.TextLayout) {
	p.ops = append(p.ops, "hint")
	p.testTextInputPainter.DrawTextLayout(origin, l)
}
func (p *placeholderOrderPainter) FillRect(r geometry.Rectangle, b graphics.Brush) {
	p.ops = append(p.ops, "fill")
	p.testTextInputPainter.FillRect(r, b)
}

func TestPlaceholderCaretPaintedLast(t *testing.T) {
	w, _, _ := newInputFixture(t, "")
	w.SetPlaceholder("hint")
	p := &placeholderOrderPainter{}
	w.Paint(p)
	if strings.Join(p.ops, ",") != "fill,hint,fill" {
		t.Fatalf("paint order: %v", p.ops)
	}
}
