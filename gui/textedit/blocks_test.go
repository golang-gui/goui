package textedit

import (
	"fmt"
	"strings"
	"testing"
)

// Inspect all storage roots, including cache and the unused stack capacity:
// stale references there also retain memory. This checks ownership without
// depending on the GC's timing or on unrelated process heap allocations.
func textModelRetainedBlocks(m *Model) map[*textBlock]bool {
	blocks := make(map[*textBlock]bool)
	nodes := make(map[*pieceNode]bool)
	var visit func(*pieceNode)
	visit = func(n *pieceNode) {
		if n == nil || nodes[n] {
			return
		}
		nodes[n] = true
		blocks[n.piece.block] = true
		visit(n.left)
		visit(n.right)
		visit(n.parent)
	}
	visit(m.tree.root)
	visit(m.tree.lfCache.node)
	for _, stack := range [][]textStep{m.undoSteps, m.redoSteps} {
		for _, step := range stack[:cap(stack)] {
			for _, refs := range [][]pieceRef{step.removed, step.inserted} {
				for _, ref := range refs[:cap(refs)] {
					if ref.block != nil {
						blocks[ref.block] = true
					}
				}
			}
		}
	}
	return blocks
}

func TestTextModelBlocksReleaseDeletedContent(t *testing.T) {
	for _, inserted := range []bool{false, true} {
		t.Run(fmt.Sprintf("inserted=%t", inserted), func(t *testing.T) {
			text := strings.Repeat("0123456789abcde\n", textPieceMaxSize)
			m := NewModel("")
			if inserted {
				if err := m.ReplaceAtomic(Range{}, text); err != nil {
					t.Fatal(err)
				}
				m.ClearHistory()
			} else {
				m.SetText(text)
			}
			blocks := textModelRetainedBlocks(m)
			if len(blocks) != len(text)/textPieceMaxSize {
				t.Fatalf("content is not independently chunked: %d blocks", len(blocks))
			}
			for block := range blocks {
				if len(block.data) > textPieceMaxSize || cap(block.data) > textPieceMaxSize {
					t.Fatal("unbounded loaded/inserted block")
				}
			}
			// Populate the LF cache in a soon-to-be-deleted block as well.
			_, _ = m.LineRange(100)
			if err := m.ReplaceAtomic(Range{1, m.Len() - 1}, ""); err != nil {
				t.Fatal(err)
			}
			if len(textModelRetainedBlocks(m)) != len(blocks) {
				t.Fatal("history lost deleted blocks before undo")
			}
			if !m.Undo() || m.Text() != text || !m.Redo() || m.Text() != "0\n" {
				t.Fatal("block references did not survive undo/redo")
			}
			m.ClearHistory()
			if got := len(textModelRetainedBlocks(m)); got != 2 {
				t.Fatalf("only two boundary blocks should remain, retained %d", got)
			}
			checkTextModelTree(t, m)
		})
	}
}

func TestTextModelDiscardRedoDropsUnreferencedBlocks(t *testing.T) {
	m := NewModel("")
	if err := m.ReplaceAtomic(Range{}, strings.Repeat("x", 4*textPieceMaxSize)); err != nil {
		t.Fatal(err)
	}
	if !m.Undo() {
		t.Fatal("missing undo")
	}
	if got := len(textModelRetainedBlocks(m)); got != 4 {
		t.Fatalf("redo must retain its four blocks, got %d", got)
	}
	if err := m.Replace(Range{}, "new"); err != nil {
		t.Fatal(err)
	}
	if m.CanRedo() || len(textModelRetainedBlocks(m)) != 1 {
		t.Fatal("a new branch kept the abandoned redo blocks")
	}
}

func TestTextModelTypingHistoryCoalescesBlockRanges(t *testing.T) {
	m := NewModel("")
	const count = textPieceMaxSize + 128
	for range count {
		if err := m.Replace(Range{m.Len(), m.Len()}, "x"); err != nil {
			t.Fatal(err)
		}
	}
	if len(m.undoSteps) != 1 || len(m.undoSteps[0].inserted) != 2 {
		t.Fatal("typing history must scale with blocks, not keystrokes")
	}
	if !m.Undo() || m.Len() != 0 || !m.Redo() || m.Text() != strings.Repeat("x", count) {
		t.Fatal("merged block history did not restore typing")
	}
	m.ClearHistory()
	for range count {
		if err := m.Replace(Range{m.Len() - 1, m.Len()}, ""); err != nil {
			t.Fatal(err)
		}
	}
	if len(m.undoSteps) != 1 || len(m.undoSteps[0].removed) != 2 {
		t.Fatal("backward deletion must coalesce contiguous block references")
	}
	if !m.Undo() || m.Text() != strings.Repeat("x", count) {
		t.Fatal("backward deletion history has incorrect text order")
	}
	checkTextModelTree(t, m)
}

func TestTextModelRepeatedReplaceAndClearHistoryBoundedStorage(t *testing.T) {
	m := NewModel("seed")
	for i := range 1000 {
		text := fmt.Sprintf("edit %04d\n", i)
		if err := m.ReplaceAtomic(Range{0, m.Len()}, text); err != nil {
			t.Fatal(err)
		}
		m.ClearHistory()
		blocks := textModelRetainedBlocks(m)
		if len(blocks) != 1 {
			t.Fatalf("iteration %d retained %d blocks", i, len(blocks))
		}
		for block := range blocks {
			if cap(block.data) > 32 {
				t.Fatalf("iteration %d retained a growing append buffer", i)
			}
		}
		if m.Text() != text {
			t.Fatal("replacement content mismatch")
		}
	}
}

func BenchmarkTextModelEditCycles(b *testing.B) {
	for _, megabytes := range []int{1, 10} {
		b.Run(fmt.Sprintf("%dMiB", megabytes), func(b *testing.B) {
			m := NewModel(strings.Repeat("0123456789abcde\n", megabytes*1024*1024/16))
			position := m.Len() / 2
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := m.ReplaceAtomic(Range{position, position + 1}, "X"); err != nil {
					b.Fatal(err)
				}
				if i%64 == 63 {
					m.ClearHistory()
				}
			}
			b.StopTimer()
			retained := 0
			for block := range textModelRetainedBlocks(m) {
				retained += cap(block.data)
			}
			// One short insertion per edit, with at most 64 edits retained in
			// this workload. No claimed bound when users keep all history.
			if retained > megabytes*1024*1024+64*32 {
				b.Fatalf("retained storage grew with discarded edits: %d", retained)
			}
			b.ReportMetric(float64(retained), "retained-B")
		})
	}
}
