package gui

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/pango"
)

// Observe only the existing Context/TextLayout contract. A decorator must not
// need another optional interface for the editor to retain measured widths.
type nativeWidthContext struct {
	typography.Context
	text  string
	calls nativeWidthCalls
}

type nativeWidthCalls struct{ created, sizes, metrics, resizes, destroyed int }

func (c *nativeWidthContext) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, err := c.Context.NewTextLayout(text, format, width, height)
	if err != nil || text != c.text {
		return l, err
	}
	c.calls.created++
	return &nativeWidthLayout{TextLayout: l, owner: c}, nil
}

type nativeWidthLayout struct {
	typography.TextLayout
	owner *nativeWidthContext
}

func (l *nativeWidthLayout) MeasureSize() (float32, float32) {
	l.owner.calls.sizes++
	return l.TextLayout.MeasureSize()
}
func (l *nativeWidthLayout) MeasureMetrics() ([]typography.TextLine, []typography.TextCluster) {
	l.owner.calls.metrics++
	return l.TextLayout.MeasureMetrics()
}
func (l *nativeWidthLayout) SetSize(width, height float32) {
	l.owner.calls.resizes++
	l.TextLayout.SetSize(width, height)
}
func (l *nativeWidthLayout) Destroy() {
	l.owner.calls.destroyed++
	l.TextLayout.Destroy()
}

func TestTextViewNativeWidthCacheKeepsCompleteContext(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	native, err := pango.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer native.Destroy()
	text := "start \u202e" + strings.Repeat("abc 123 العربية עברית e\u0301 ffi 👩‍💻\t ", 600) + "END\u202c"
	if len(text) < 32768 {
		t.Fatal("fixture must cover the removed long-paragraph branch")
	}
	ctx := &nativeWidthContext{Context: native, text: text}
	setTestApplication(t, ctx)
	v := NewTextView()
	defer v.releaseParagraphs()
	v.SetWrapMode(WrapWordChar)
	v.Model().SetText(text)
	show := func(width, height float32) *textParagraph {
		v.Arrange(geometry.Rect(0, 0, width, height))
		return v.paragraph(0)
	}
	a := show(240, 200)
	if a == nil || len(a.lines) < 100 {
		t.Fatal("missing complete native layout")
	}
	a.editGeometry(v.lineHeight)
	fragment := a.fragment
	position := textedit.Position{Offset: strings.Index(text, "END")}
	wantCaret := a.geometry.Caret(position)
	// The complete native input, at the same effective width, is the reference.
	w, h := a.layout.Size()
	reference, err := native.NewTextLayout(text, a.layout.Format(), w, h)
	if err != nil {
		t.Fatal(err)
	}
	defer reference.Destroy()
	rw, rh := reference.MeasureSize()
	ls, cs := reference.MeasureMetrics()
	want, err := textedit.NewFragment(textedit.Range{End: len(text)}, geometry.Size{Width: rw, Height: rh}, ls, cs)
	if err != nil {
		t.Fatal(err)
	}
	gotLines, gotClusters := fragment.Metrics()
	wantLines, wantClusters := want.Metrics()
	if !reflect.DeepEqual(gotLines, wantLines) || !reflect.DeepEqual(gotClusters, wantClusters) {
		t.Fatal("cached values differ from the complete native paragraph")
	}
	covered := 0
	for _, c := range gotClusters {
		if c.Start < position.Offset+3 && c.Start+c.Length > position.Offset {
			if c.Direction != typography.TextRightToLeft {
				t.Fatal("RLO context was lost across a long paragraph")
			}
			covered += min(c.Start+c.Length, position.Offset+3) - max(c.Start, position.Offset)
		}
	}
	if covered != 3 {
		t.Fatal("RLO target is not completely covered", covered)
	}
	b := show(310, 200)
	if b == a || b.layout == a.layout {
		t.Fatal("different widths shared a mutable native layout")
	}
	before := ctx.calls
	for range 3 {
		if show(240, 280) != a || show(310, 320) != b || ctx.calls != before {
			t.Fatal("restoring measured widths or changing height repeated native work", before, ctx.calls)
		}
	}
	other := NewTextView()
	defer other.releaseParagraphs()
	other.SetWrapMode(WrapWordChar)
	other.Model().SetText(text)
	other.Arrange(geometry.Rect(0, 0, 240, 200))
	if other.paragraph(0).layout == a.layout || ctx.calls.created != 3 {
		t.Fatal("different views shared native resources")
	}
	other.releaseParagraphs()
	v.releaseParagraphs()
	if ctx.calls.created != ctx.calls.destroyed {
		t.Fatal("width caches leaked native resources", ctx.calls)
	}
	g := fragment.Geometry(v.lineHeight)
	if g.Caret(position) != wantCaret {
		t.Fatal("ordinary metrics did not survive native layout destruction")
	}
}
