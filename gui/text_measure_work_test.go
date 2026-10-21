package gui

import (
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/layout"
)

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
