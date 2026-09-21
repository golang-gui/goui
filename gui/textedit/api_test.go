package textedit_test

import (
	"reflect"
	"testing"

	"github.com/golang-gui/goui/gui/textedit"
)

func TestEditSourcesAndSelectionHistory(t *testing.T) {
	m := textedit.NewModel("ab")
	a, b := &textedit.EditSource{}, &textedit.EditSource{}
	apply := func(source *textedit.EditSource, text string) {
		t.Helper()
		before := textedit.Selection{Anchor: m.Len(), Caret: m.Len()}
		err := m.Apply(textedit.Edit{Range: before.Range(), Text: text},
			textedit.EditOptions{Source: source, Selection: &before})
		if err != nil {
			t.Fatal(err)
		}
		before.Anchor = 999 // history must not borrow the supplied metadata.
	}
	apply(a, "x")
	apply(a, "y")
	apply(b, "z")
	for _, tc := range []struct {
		text  string
		caret int
	}{{"abxy", 4}, {"ab", 2}} {
		result := m.UndoWithSelection()
		want := textedit.Selection{Anchor: tc.caret, Caret: tc.caret}
		if !result.Applied || result.Revision != m.Revision() || result.Selection == nil ||
			*result.Selection != want || m.Text() != tc.text {
			t.Fatalf("undo: text=%q result=%+v selection=%v", m.Text(), result, result.Selection)
		}
		result.Selection.Anchor = 999 // returned metadata must not mutate history.
	}
	if m.CanUndo() || m.UndoWithSelection().Applied {
		t.Fatal("expected exactly two groups")
	}
	for _, tc := range []struct {
		text  string
		caret int
	}{{"abxy", 4}, {"abxyz", 5}} {
		result := m.RedoWithSelection()
		want := textedit.Selection{Anchor: tc.caret, Caret: tc.caret}
		if !result.Applied || result.Selection == nil || *result.Selection != want || m.Text() != tc.text {
			t.Fatalf("redo: text=%q result=%+v", m.Text(), result)
		}
	}
	if m.RedoWithSelection().Applied {
		t.Fatal("unexpected redo")
	}
	result := m.UndoWithSelection()
	if result.Selection == nil || *result.Selection != (textedit.Selection{Anchor: 4, Caret: 4}) {
		t.Fatal("returned selection mutated stored history")
	}
}

func TestHistoryResultSurvivesReentrantEdit(t *testing.T) {
	m := textedit.NewModel("a")
	selection := textedit.Selection{Anchor: 1, Caret: 1}
	if err := m.Apply(textedit.Edit{Range: selection.Range(), Text: "b"},
		textedit.EditOptions{Selection: &selection, Atomic: true}); err != nil {
		t.Fatal(err)
	}
	m.ConnectChange(func(change textedit.Change) {
		if change.Revision == 2 {
			m.ClearHistory()
			if err := m.Replace(textedit.Range{Start: 1, End: 1}, "!"); err != nil {
				t.Fatal(err)
			}
		}
	})
	result := m.UndoWithSelection()
	if !result.Applied || result.Revision != 2 || m.Revision() != 3 ||
		m.Text() != "a!" || result.Selection == nil || *result.Selection != selection {
		t.Fatalf("result must describe original undo: %+v current=%d text=%q", result, m.Revision(), m.Text())
	}
}

func TestApplyAtomicAndPlainHistory(t *testing.T) {
	m := textedit.NewModel("")
	source := &textedit.EditSource{}
	for i, text := range []string{"a", "b", "c"} {
		if err := m.Apply(textedit.Edit{Range: textedit.Range{Start: i, End: i}, Text: text},
			textedit.EditOptions{Source: source, Atomic: i == 1}); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []string{"ab", "a", ""} {
		result := m.UndoWithSelection()
		if !result.Applied || result.Selection != nil || m.Text() != want {
			t.Fatalf("undo: %+v text=%q", result, m.Text())
		}
	}
}

func TestPositionHelpers(t *testing.T) {
	if got := textedit.NormalizeSingleLine("a\r\n中\r\xff"); got != "a 中 �" {
		t.Fatalf("normalize=%q", got)
	}
	for index, want := range map[int]int{-1: 0, 0: 0, 1: 1, 2: 1, 3: 1, 4: 4, 10: 4} {
		if got := textedit.ClampOffset("a中", index); got != want {
			t.Fatalf("clamp(%d)=%d want %d", index, got, want)
		}
	}
	edit := textedit.Edit{Range: textedit.Range{Start: 2, End: 5}, Text: "中"}
	for index, want := range map[int]int{0: 0, 2: 5, 3: 5, 5: 5, 8: 8} {
		if got := textedit.MapOffset(index, edit); got != want {
			t.Fatalf("map(%d)=%d want %d", index, got, want)
		}
	}
	m := textedit.NewModel("hello world\n")
	for _, tc := range []struct {
		offset int
		want   textedit.Range
	}{
		{0, textedit.Range{Start: 0, End: 5}},
		{4, textedit.Range{Start: 0, End: 5}},
		{5, textedit.Range{Start: 5, End: 6}},
		{8, textedit.Range{Start: 6, End: 11}},
		{12, textedit.Range{Start: 12, End: 12}},
	} {
		if got := m.WordAt(tc.offset); got != tc.want {
			t.Fatalf("word(%d)=%v want %v", tc.offset, got, tc.want)
		}
	}
	if !m.EqualText("hello world\r\n") || m.EqualText("other") || m.Revision() != 0 {
		t.Fatal("equal-text comparison changed the model or failed normalization")
	}
}

func TestProjectionMappingAndIsolation(t *testing.T) {
	m := textedit.NewModel("ab\ncd\nef")
	p := textedit.NewProjection(m, textedit.Selection{Anchor: 1, Caret: 7})
	if p.Paragraph(m, 0) != "af" || p.LineCount(m) != 1 {
		t.Fatal("initial projection not usable")
	}
	p.Update("中\nx", 4)
	if p.FirstLine() != 0 || p.LastLine() != 2 || p.ParagraphCount() != 2 ||
		p.Caret() != 4 || p.Replacement() != (textedit.Range{Start: 1, End: 7}) {
		t.Fatalf("projection metadata: %+v", p)
	}
	if got := []string{p.Paragraph(m, 0), p.Paragraph(m, 1)}; !reflect.DeepEqual(got, []string{"a中", "xf"}) {
		t.Fatalf("paragraphs=%q", got)
	}
	if p.LineRange(m, 0) != (textedit.Range{Start: 0, End: 4}) ||
		p.LineRange(m, 1) != (textedit.Range{Start: 5, End: 7}) || p.LineAt(m, 5) != 1 {
		t.Fatal("projection line offsets")
	}
	if p.ModelOffset(2, false) != 1 || p.ModelOffset(2, true) != 7 || p.ModelOffset(6, false) != 7 {
		t.Fatal("projection to model mapping")
	}
	p.SetCaret(2)
	if p.Caret() != 0 {
		t.Fatal("caret split UTF-8 rune")
	}
	other := textedit.NewProjection(m, textedit.Selection{})
	other.Update("!", 1)
	if p.Text() != "中\nx" || m.Text() != "ab\ncd\nef" || m.Revision() != 0 || m.CanUndo() {
		t.Fatal("preedit leaked into another projection or document history")
	}
}
