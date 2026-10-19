package gui

import (
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/typography"
)

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

type sizingBaselineTypography struct{ editorTypography }

func (c *sizingBaselineTypography) NewTextLayout(text string, f typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, err := c.editorTypography.NewTextLayout(text, f, width, height)
	if text != textInputHeightSample {
		p := l.(*testTextLayout)
		for i := range p.lines {
			p.lines[i].Baseline -= 3
		}
	}
	return l, err
}

func TestTextFieldsShareBaselineDrawingAndHitGeometry(t *testing.T) {
	setTestApplication(t, &sizingBaselineTypography{})
	v, input := NewTextView(), NewTextInput()
	v.Model().SetText("abc")
	input.SetText("abc")
	t.Cleanup(v.releaseParagraphs)
	t.Cleanup(input.releaseParagraphs)
	c := layout.Tight(geometry.Size{Width: 209, Height: 31})
	for _, field := range []Widget{v, input} {
		m := field.Measure(c)
		if m.Height != 31 || m.Baseline != 19 || !m.HasBaseline {
			t.Fatalf("shared baseline: %+v", m)
		}
		field.Arrange(geometry.Rectangle{Size: m.Size})
		p := &testLabelPainter{}
		field.Paint(p)
		if p.textOrigin.Y != 7 {
			t.Fatalf("native baseline displacement: %+v", p.textOrigin)
		}
	}
	if got := v.hitText(geometry.Point{X: 19, Y: 12}); got.Offset != 2 {
		t.Fatalf("hit did not use painted origin: %+v", got)
	}
	if caret, ok := v.caretRect(); !ok || caret.Y != 7 {
		t.Fatalf("caret did not use painted origin: %+v", caret)
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

type replacingMeasuredContent struct {
	mockScrollContent
	replace func()
}

func (c *replacingMeasuredContent) Measure(cs layout.Constraint) layout.Measurement {
	c.replace()
	return c.mockScrollContent.Measure(cs)
}

func TestScrollViewStopsWhenMeasurementReplacesContent(t *testing.T) {
	scroll := NewScrollView()
	content := &replacingMeasuredContent{replace: func() { scroll.SetChild(nil) }}
	scroll.SetChild(content)
	if got := scroll.Measure(layout.Loose(geometry.Size{Width: 100, Height: 100})); got != (layout.Measurement{}) || scroll.Child() != nil {
		t.Fatalf("measurement retained replaced content: %+v", got)
	}
}
