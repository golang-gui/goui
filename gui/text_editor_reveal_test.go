package gui

import (
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
)

func longWrappedEditor(t *testing.T) (*TextView, *ScrollView, *window, *editorTypography) {
	t.Helper()
	v, scroll, win, _, typo := timedEditorFixture(t)
	v.Model().SetText(strings.Repeat("abcdefghij", 1000))
	win.width, win.height = 220, 108
	settleEditorLayout(t, win)
	return v, scroll, win, typo
}

func settleEditorLayout(t *testing.T, win *window) {
	t.Helper()
	for range 8 {
		if !win.layoutDirty {
			return
		}
		win.layoutFrame(win.Widget())
	}
	t.Fatal("editor layout did not settle")
}

func editorPointer(t *testing.T, win *window, kind events.EventType, point geometry.Point, mods events.Modifiers) {
	t.Helper()
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: kind, Position: point, Button: events.PointerButtonLeft, Modifiers: mods,
	}); err != nil {
		t.Fatal(err)
	}
}

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

func TestTextInputCaretRevealReusesMetrics(t *testing.T) {
	_, _, win, typo := longWrappedEditor(t)
	input := NewTextInput()
	input.SetText(strings.Repeat("abcdefghij", 100))
	win.SetWidget(input)
	win.SetFocusedWidget(input)
	settleEditorLayout(t, win)
	p := input.paragraphs[0].layout.(*testTextLayout)
	calls, layouts := p.metricsCalls, len(typo.calls)
	for _, key := range []events.Key{events.KeyHome, events.KeyEnd, events.KeyArrowLeft} {
		editorKey(t, win, key, 0)
		if win.layoutDirty || p.metricsCalls != calls || len(typo.calls) != layouts {
			t.Fatal("single-line navigation remeasured text")
		}
		rect, ok := input.caretRect()
		if !ok || rect.X < input.offset.X || rect.X+rect.Width > input.offset.X+input.viewport.Width {
			t.Fatal("single-line caret reveal lost horizontal scrolling")
		}
	}
}

func TestTextEditorRevealPreservesPendingLayout(t *testing.T) {
	v, _, win, _ := longWrappedEditor(t)
	v.SetPadding(8)
	v.SetSelection(TextSelection{Anchor: 50, Caret: 50})
	if !win.layoutDirty {
		t.Fatal("selection consumed padding invalidation")
	}
	settleEditorLayout(t, win)
	v.onPreedit("候选", len("候选"))
	if !win.layoutDirty {
		t.Fatal("composition failed to request layout")
	}
	settleEditorLayout(t, win)
	editorKey(t, win, events.KeyArrowRight, 0)
	if v.preedit != nil || !win.layoutDirty {
		t.Fatal("navigation consumed composition cancellation layout")
	}
	settleEditorLayout(t, win)
	v.Model().SetText("replacement")
	v.SetSelection(TextSelection{Anchor: 3, Caret: 3})
	if !win.layoutDirty {
		t.Fatal("selection consumed model replacement layout")
	}
	settleEditorLayout(t, win)
	if got := v.paragraphs[0].layout.Text(); got != "replacement" {
		t.Fatal("reveal retained stale paragraph")
	}
}

func TestTextEditorUnmeasuredCaretRequestsLayout(t *testing.T) {
	v, scroll, win, _, typo := timedEditorFixture(t)
	win.width, win.height = 220, 108
	settleEditorLayout(t, win)
	last := v.Model().LineCount() - 1
	if v.paragraphs[last] != nil {
		t.Fatal("fixture already measured the distant target")
	}
	calls := len(typo.calls)
	editorKey(t, win, events.KeyEnd, textCommandModifier())
	if !win.layoutDirty || !v.revealCaret || len(typo.calls) != calls {
		t.Fatal("uncached reveal must request layout without shaping in the setter")
	}
	settleEditorLayout(t, win)
	rect, ok := v.caretRect()
	if !ok || v.Selection().Caret != v.Model().Len() || v.offset.Y != scroll.ScrollY() ||
		rect.Y < v.offset.Y || rect.Y+rect.Height > v.offset.Y+v.viewport.Height {
		t.Fatal("layout did not resolve the previously unmeasured caret")
	}
}

func TestTextEditorRevealCallbackSupersedesSelection(t *testing.T) {
	for _, action := range []string{"selection", "model", "unmount", "destroy"} {
		t.Run(action, func(t *testing.T) {
			v, _, win, _ := longWrappedEditor(t)
			reveals, stale := 0, false
			v.ConnectSelection(func(s TextSelection) {
				if s.Caret == 10000 {
					stale = true
				}
			})
			handle := v.ConnectScrollIntoView(func(geometry.Rectangle) {
				reveals++
				if reveals != 1 {
					t.Fatal("reveal listener was called recursively")
				}
				switch action {
				case "selection":
					v.SetSelection(TextSelection{Anchor: 1, Caret: 1})
				case "model":
					v.SetModel(NewTextModel("new"))
				case "unmount":
					win.SetWidget(nil)
				case "destroy":
					win.SetWidget(nil)
					v.WidgetBase.destroy(v)
				}
			})
			editorKey(t, win, events.KeyEnd, textCommandModifier())
			handle.Disconnect()
			if reveals != 1 || stale {
				t.Fatalf("stale selection after reveal callback: calls=%d stale=%v", reveals, stale)
			}
			if action == "selection" {
				settleEditorLayout(t, win)
				if v.Selection().Caret != 1 || v.offset.Y != 0 {
					t.Fatal("nested selection reveal was lost")
				}
			}
		})
	}
}
