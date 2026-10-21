package gui

import (
	"image/color"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

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
