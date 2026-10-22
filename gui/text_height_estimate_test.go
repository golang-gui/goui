package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"strings"
	"testing"
)

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
