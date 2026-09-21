package textedit

import (
	"reflect"
	"testing"
)

func TestTextModelReentrantNotificationOrder(t *testing.T) {
	m := NewModel("")
	var revisions []uint64
	m.ConnectChange(func(change Change) {
		if change.Revision == 1 {
			_ = m.Replace(Range{1, 1}, "b")
		}
	})
	m.ConnectChange(func(change Change) { revisions = append(revisions, change.Revision) })
	_ = m.Replace(Range{}, "a")
	if !reflect.DeepEqual(revisions, []uint64{1, 2}) {
		t.Fatalf("revision order %v", revisions)
	}
	if m.Text() != "ab" {
		t.Fatal("reentrant edit missing")
	}
}

func TestTextModelLineAt(t *testing.T) {
	m := NewModel("a\n中\n\n")
	for offset, want := range []int{0, 0, 1, 1, 1, 1, 2, 3} {
		if got := m.LineAt(offset); got != want {
			t.Fatalf("lineAt(%d)=%d want %d", offset, got, want)
		}
	}
}
