package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/typography"
)

type reflowFixture struct {
	*testTextLayout
	pending, allowed   bool
	targetW, targetH   float32
	advances, requests int
	onAdvance          func()
}

func (l *reflowFixture) Reflow(w, h float32) bool {
	l.pending = true
	l.targetW, l.targetH = w, h
	l.requests++
	return true
}
func (l *reflowFixture) CancelReflow() { l.pending = false }
func (l *reflowFixture) AdvanceReflow(deadline time.Time) (bool, error) {
	l.advances++
	if l.onAdvance != nil {
		f := l.onAdvance
		l.onAdvance = nil
		f()
	}
	if !l.pending {
		return true, nil
	}
	if !l.allowed {
		return false, nil
	}
	l.pending = false
	l.SetSize(l.targetW, l.targetH)
	return true, nil
}

type reflowTypography struct {
	editorTypography
	long *reflowFixture
}

func (c *reflowTypography) NewTextLayout(s string, f typography.TextFormat, w, h float32) (typography.TextLayout, error) {
	l, e := c.editorTypography.NewTextLayout(s, f, w, h)
	if e != nil || len(s) < 8192 {
		return l, e
	}
	r := &reflowFixture{testTextLayout: l.(*testTextLayout)}
	c.long = r
	return r, nil
}

func TestTextViewDeferredResizeKeepsInputAndCancelsStaleWork(t *testing.T) {
	ctx := &reflowTypography{}
	setTestApplication(t, ctx)
	v := NewTextView()
	v.Model().SetText(strings.Repeat("abcdefghij", 1000))
	w := &window{}
	w.SetWidget(v)
	defer w.SetWidget(nil)
	v.Arrange(geometry.Rect(0, 0, 209, 108))
	w.SetFocusedWidget(v)
	native := ctx.long
	p := v.paragraphs[0]
	before := native.metricsCalls
	oldWidth, _ := native.Size()
	for _, width := range []float32{180, 120, 160} {
		v.Arrange(geometry.Rect(0, 0, width, 108))
	}
	if native.metricsCalls != before || v.paragraphs[0] != p {
		t.Fatal("resize synchronously measured the paragraph")
	}
	if native.targetW != 151 {
		t.Fatalf("latest effective width=%g want151", native.targetW)
	}
	if width, _ := native.Size(); width != oldWidth {
		t.Fatal("pending resize changed displayed width")
	}
	editorPointer(t, w, events.PointerDown, geometry.Point{X: 29, Y: 14}, 0)
	editorPointer(t, w, events.PointerUp, geometry.Point{X: 29, Y: 14}, 0)
	if v.Selection() != (TextSelection{Anchor: 3, Caret: 3}) {
		t.Fatalf("pending input used new rather than displayed geometry: %+v", v.Selection())
	}
	native.allowed = true
	w.frames.Emit(time.Now())
	if width, _ := native.Size(); width != 151 || native.metricsCalls != before+1 {
		t.Fatalf("publication width=%g metrics=%d", width, native.metricsCalls)
	}
	if p.cancelReflow != nil || textReflows[w] != nil {
		t.Fatal("completed task retained a root/job")
	}
	v.Arrange(geometry.Rect(0, 0, 140, 108))
	native.allowed = false
	if textReflows[w] == nil {
		t.Fatal("missing pending task")
	}
	v.Model().SetText("edited")
	if !native.destroyed || native.pending || textReflows[w] != nil {
		t.Fatal("edit retained a stale task/native object")
	}
}

func TestTextReflowQueueReentrantPublicationAndTeardown(t *testing.T) {
	v, w, _ := newEditorFixture(t, "abc")
	l := &reflowFixture{testTextLayout: &testTextLayout{text: "abc", width: 100, height: 20}, allowed: true}
	defer l.Destroy()
	callbacks := 0
	l.ConnectChanged(func() {
		if callbacks == 0 {
			callbacks++
			DeferTextLayout(v, l, 300, 20, func(error) { callbacks++ })
		}
	})
	oldReady := false
	_, ok := DeferTextLayout(v, l, 200, 20, func(error) { oldReady = true })
	if !ok {
		t.Fatal("request declined")
	}
	w.frames.Emit(time.Now())
	if oldReady || callbacks != 2 {
		t.Fatalf("stale completion/reentrant work: old=%v callbacks=%d", oldReady, callbacks)
	}
	if width, _ := l.Size(); width != 300 || textReflows[w] != nil {
		t.Fatal("reentrant request was cancelled or leaked")
	}
	l.allowed = false
	cancel, _ := DeferTextLayout(v, l, 400, 20, func(error) { t.Fatal("cancelled request completed") })
	cancel()
	cancel()
	if l.pending || textReflows[w] != nil {
		t.Fatal("cancel did not release pending root")
	}
	DeferTextLayout(v, l, 500, 20, func(error) { t.Fatal("destroyed request completed") })
	l.Destroy()
	if textReflows[w] != nil {
		t.Fatal("native destruction retained queue")
	}
}

func TestDeferredParagraphPublicationFollowsLineSplice(t *testing.T) {
	ctx := &reflowTypography{}
	setTestApplication(t, ctx)
	v := NewTextView()
	v.Model().SetText("prefix\n" + strings.Repeat("a", 10000))
	w := &window{}
	w.SetWidget(v)
	defer w.SetWidget(nil)
	v.Arrange(geometry.Rect(0, 0, 209, 108))
	p := v.paragraphs[1]
	if p == nil {
		t.Fatal("long paragraph not measured")
	}
	v.Arrange(geometry.Rect(0, 0, 160, 108))
	native := ctx.long
	before := native.metricsCalls
	if err := v.Model().Replace(TextRange{}, "new\n"); err != nil {
		t.Fatal(err)
	}
	if p.index != 2 || v.paragraphs[2] != p {
		t.Fatal("unchanged paragraph not remapped")
	}
	native.allowed = true
	w.frames.Emit(time.Now())
	if native.metricsCalls != before+1 || p.cancelReflow != nil || v.heights.Top(3)-v.heights.Top(2) != p.height {
		t.Fatal("publication used obsolete logical index")
	}
}

func TestTextReflowQueueRotatesAndPausesHiddenWork(t *testing.T) {
	v, w, _ := newEditorFixture(t, "abc")
	busy := &reflowFixture{testTextLayout: &testTextLayout{text: "busy", width: 100, height: 20}}
	fast := &reflowFixture{testTextLayout: &testTextLayout{text: "fast", width: 100, height: 20}, allowed: true}
	defer busy.Destroy()
	defer fast.Destroy()
	cancel, _ := DeferTextLayout(v, busy, 200, 20, nil)
	defer cancel()
	done := false
	DeferTextLayout(v, fast, 200, 20, func(error) { done = true })
	w.frames.Emit(time.Now())
	if !done || busy.advances == 0 {
		t.Fatal("slow job starved another layout")
	}
	v.SetVisible(false)
	before := busy.advances
	w.frames.Emit(time.Now())
	if busy.advances != before {
		t.Fatal("hidden owner continued measuring")
	}
	v.SetVisible(true)
	busy.allowed = true
	w.frames.Emit(time.Now())
	if textReflows[w] != nil {
		t.Fatal("resumed work leaked queue")
	}
}

func TestDeferredResizeCancelsOffscreenWidths(t *testing.T) {
	ctx := &reflowTypography{}
	setTestApplication(t, ctx)
	v := NewTextView()
	v.Model().SetText(strings.Repeat("a", 10000) + "\n" + strings.Repeat("b", 10000))
	w := &window{}
	w.SetWidget(v)
	defer w.SetWidget(nil)
	v.Arrange(geometry.Rect(0, 0, 209, 108))
	v.paragraph(1) // Retain a previously visited paragraph outside this viewport.
	v.Arrange(geometry.Rect(0, 0, 180, 108))
	offscreen := v.paragraph(1)
	native := offscreen.layout.(*reflowFixture)
	if offscreen.cancelReflow == nil || !native.pending {
		t.Fatal("fixture did not request an offscreen width")
	}
	v.Arrange(geometry.Rect(0, 0, 160, 108))
	if offscreen.cancelReflow != nil || native.pending || offscreen.measured {
		t.Fatal("offscreen result can publish an obsolete width as measured")
	}
}
