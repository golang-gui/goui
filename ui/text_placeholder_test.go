package ui

import (
	"testing"

	"github.com/golang-gui/goui/gui"
)

func TestTextPlaceholderUpdateAndOmissionPreserveEditing(t *testing.T) {
	t.Run("input", func(t *testing.T) {
		r := newRoot()
		t.Cleanup(r.unmountWindow)
		w := r.update(TextInput().Text("abc").Placeholder("first")).(*gui.TextInput)
		w.SetSelection(TextSelection{Anchor: 1, Caret: 2})
		changes := 0
		w.ConnectText(func(string) { changes++ })
		w.ConnectSelection(func(TextSelection) { changes++ })
		r.update(TextInput().Placeholder("second\nline"))
		if w.Placeholder() != "second line" || w.Text() != "abc" || w.Selection() != (TextSelection{Anchor: 1, Caret: 2}) || changes != 0 {
			t.Fatal("placeholder update changed input state or did not normalize")
		}
		r.update(TextInput())
		if w.Placeholder() != "" || w.Text() != "abc" || w.Selection() != (TextSelection{Anchor: 1, Caret: 2}) || changes != 0 {
			t.Fatal("omitted placeholder did not restore default independently of editing")
		}
	})
	t.Run("view", func(t *testing.T) {
		r := newRoot()
		t.Cleanup(r.unmountWindow)
		m := NewTextModel("abc")
		w := r.update(TextView().Model(m).Placeholder("first")).(*gui.TextView)
		w.SetSelection(TextSelection{Anchor: 1, Caret: 2})
		if err := m.Replace(TextRange{}, "x"); err != nil {
			t.Fatal(err)
		}
		revision, selection := m.Revision(), w.Selection()
		changes := 0
		w.ConnectChange(func(TextChange) { changes++ })
		w.ConnectSelection(func(TextSelection) { changes++ })
		r.update(TextView().Model(m).Placeholder("second\r\nline"))
		if w.Placeholder() != "second\nline" || w.Model() != m || m.Revision() != revision || !m.CanUndo() || w.Selection() != selection || changes != 0 {
			t.Fatal("placeholder update changed shared model, history or selection")
		}
		r.update(TextView().Model(m))
		if w.Placeholder() != "" || m.Revision() != revision || !m.CanUndo() || w.Selection() != selection || changes != 0 {
			t.Fatal("omitted placeholder changed editing state")
		}
	})
}
