package textedit

import (
	"bytes"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func TestBlockLineIndexAtUint16Boundary(t *testing.T) {
	b := &textBlock{data: bytes.Repeat([]byte{'\n'}, textPieceMaxSize)}
	b.indexTail(0)
	if len(b.lf) != 65536 || b.lf[65535] != 65535 || b.lfBefore(65536) != 65536 {
		t.Fatal("full block LF index overflowed")
	}
	p := pieceRef{block: b, start: 65530, length: 6}
	for length := 0; length <= p.length; length++ {
		if got := pieceLFCount(p, length); got != length {
			t.Fatal("piece prefix count", length, got)
		}
	}
	if pieceLFIndex(p, 5) != 5 || pieceLFIndex(p, 6) != -1 || pieceLFIndex(p, -1) != -1 {
		t.Fatal("indexed LF escaped the piece")
	}
}

func TestBlockLineIndexFollowsTailGrowth(t *testing.T) {
	b := &textBlock{data: []byte("a\n中文\nz")}
	b.indexTail(0)
	old := pieceRef{block: b, start: 2, length: len(b.data) - 2}
	oldIndex := append([]uint16(nil), b.lf...)
	start := len(b.data)
	b.data = append(b.data, []byte("\nnew\n\n")...)
	b.indexTail(start)
	if !reflect.DeepEqual(b.lf[:len(oldIndex)], oldIndex) || pieceLFCount(old, old.length) != 1 || pieceLFIndex(old, 1) != -1 {
		t.Fatal("tail growth changed a retained piece's LF range")
	}
	if pieceLFCount(pieceRef{block: b, length: len(b.data)}, len(b.data)) != 5 {
		t.Fatal("new tail was not indexed exactly once")
	}
}

func assertIndexedLines(t *testing.T, m *Model, want string) {
	t.Helper()
	checkTextModelTree(t, m)
	starts := []int{0}
	for at, b := range []byte(want) {
		if b == '\n' {
			starts = append(starts, at+1)
		}
	}
	if m.LineCount() != len(starts) {
		t.Fatal("line count", m.LineCount(), len(starts))
	}
	// Deliberately reverse/interleave the order so the sequential cache cannot
	// conceal a wrong block index or stale cache after undo and redo.
	for n := range starts {
		row := len(starts) - n - 1
		r, err := m.LineRange(row)
		end := len(want)
		if row+1 < len(starts) {
			end = starts[row+1] - 1
		}
		if err != nil || r != (Range{Start: starts[row], End: end}) || m.LineAt(starts[row]) != row || m.LineAt(end) != row {
			t.Fatalf("row %d: %+v / %v, want [%d,%d)", row, r, err, starts[row], end)
		}
	}
}

func TestIndexedLinesSurviveSplitsHistoryAndBranching(t *testing.T) {
	want := strings.Repeat("a\n中e\u0301\n", 15000)
	m := NewModel(want)
	assertIndexedLines(t, m, want)
	random := rand.New(rand.NewSource(17))
	for i := 0; i < 80; i++ {
		row := random.Intn(m.LineCount())
		r, err := m.LineRange(row)
		if err != nil {
			t.Fatal(err)
		}
		old := want
		insert := "new\n\n界"
		if err := m.ReplaceAtomic(r, insert); err != nil {
			t.Fatal(err)
		}
		want = old[:r.Start] + insert + old[r.End:]
		assertIndexedLines(t, m, want)
		if !m.Undo() {
			t.Fatal("undo failed")
		}
		assertIndexedLines(t, m, old)
		if i%2 == 0 {
			if !m.Redo() {
				t.Fatal("redo failed")
			}
			assertIndexedLines(t, m, want)
		} else {
			// Append after undo while old history still references the block.
			if err := m.ReplaceAtomic(Range{len(old), len(old)}, "\ntail"); err != nil {
				t.Fatal(err)
			}
			want = old + "\ntail"
			assertIndexedLines(t, m, want)
		}
	}
}

func TestIndexedNextLineAcrossLongParagraph(t *testing.T) {
	text := "first\n" + strings.Repeat("x", 20*textPieceMaxSize) + "\nlast"
	m := NewModel(text)
	for range 4 {
		for row, want := range []Range{{0, 5}, {6, len(text) - 5}, {len(text) - 4, len(text)}} {
			got, err := m.LineRange(row)
			if err != nil || got != want {
				t.Fatal(row, got, want, err)
			}
		}
	}
}
