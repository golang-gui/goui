package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
)

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
