package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/platform/events"
)

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
