package textedit

import (
	"reflect"
	"testing"
)

func TestBatchHistoryAndNotifications(t *testing.T) {
	m := NewModel("ab\ncd")
	before, after := Selection{5, 0}, Selection{7, 0}
	var changes []Change
	h := m.ConnectChange(func(c Change) { changes = append(changes, c) })
	defer h.Disconnect()
	if err := m.ApplyBatch([]Edit{{Range{0, 0}, " "}, {Range{3, 3}, " "}}, BatchOptions{BeforeSelection: &before, AfterSelection: &after}); err != nil {
		t.Fatal(err)
	}
	before.Anchor = 0
	after.Anchor = 0
	if m.Text() != " ab\n cd" || m.Revision() != 1 || len(changes) != 1 {
		t.Fatal(m.Text(), m.Revision(), changes)
	}
	if !reflect.DeepEqual(changes[0].Edits, []Edit{{Range{3, 3}, " "}, {Range{0, 0}, " "}}) {
		t.Fatal(changes)
	}
	result := m.UndoWithSelection()
	if !result.Applied || m.Text() != "ab\ncd" || *result.Selection != (Selection{5, 0}) {
		t.Fatal(m.Text(), result)
	}
	result.Selection.Anchor = 999
	result = m.RedoWithSelection()
	if !result.Applied || m.Text() != " ab\n cd" || *result.Selection != (Selection{7, 0}) || len(changes) != 3 {
		t.Fatal(m.Text(), result, changes)
	}
	if !m.Undo() || m.Undo() {
		t.Fatal("batch is not one undo item")
	}
}

func TestBatchInvalidIsAtomic(t *testing.T) {
	for _, edits := range [][]Edit{
		{{Range{0, 0}, "x"}, {Range{1, 2}, "y"}},
		{{Range{0, 2}, "x"}, {Range{1, 3}, "y"}},
		{{Range{2, 2}, "x"}, {Range{0, 0}, "y"}},
		{{Range{0, 0}, "x"}, {Range{0, 0}, "y"}},
	} {
		m := NewModel("ébc")
		count := 0
		h := m.ConnectChange(func(Change) { count++ })
		if err := m.ApplyBatch(edits, BatchOptions{}); err == nil {
			t.Fatal("accepted", edits)
		}
		if m.Text() != "ébc" || m.Revision() != 0 || m.CanUndo() || count != 0 {
			t.Fatal("partial mutation")
		}
		h.Disconnect()
	}
}

func TestBatchSelectionValidationAndNormalization(t *testing.T) {
	m := NewModel("a")
	bad := Selection{1, 1}
	if err := m.ApplyBatch([]Edit{{Range{0, 1}, "é"}}, BatchOptions{AfterSelection: &bad}); err == nil {
		t.Fatal("accepted partial rune")
	}
	if m.Text() != "a" || m.Revision() != 0 {
		t.Fatal("invalid selection mutated")
	}
	end := Selection{4, 4}
	if err := m.ApplyBatch([]Edit{{Range{0, 1}, "é\r\nx"}}, BatchOptions{AfterSelection: &end}); err != nil {
		t.Fatal(err)
	}
	if m.Text() != "é\nx" {
		t.Fatal(m.Text())
	}
	if !m.Undo() || !m.Redo() || m.Text() != "é\nx" {
		t.Fatal("history")
	}
}

func TestBatchReentryResultRevision(t *testing.T) {
	m := NewModel("x")
	h := m.ConnectChange(func(c Change) {
		if c.Revision == 2 {
			if err := m.Replace(Range{0, 0}, "z"); err != nil {
				t.Fatal(err)
			}
		}
	})
	defer h.Disconnect()
	if err := m.ApplyBatch([]Edit{{Range{0, 1}, "y"}}, BatchOptions{}); err != nil {
		t.Fatal(err)
	}
	r := m.UndoWithSelection()
	if r.Revision != 2 || m.Revision() != 3 || m.Text() != "zx" {
		t.Fatal(r, m.Revision(), m.Text())
	}
}
