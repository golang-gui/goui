package textedit

import (
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTextModelEmptyDocument(t *testing.T) {
	m := NewModel("")
	if m.Text() != "" || m.Len() != 0 || m.Revision() != 0 || m.LineCount() != 1 {
		t.Fatalf("unexpected empty model state: text=%q len=%d rev=%d lines=%d", m.Text(), m.Len(), m.Revision(), m.LineCount())
	}
	rng, err := m.LineRange(0)
	if err != nil || rng != (Range{0, 0}) {
		t.Fatalf("empty document line 0 = %+v, %v", rng, err)
	}
	if _, err := m.LineRange(1); err == nil {
		t.Fatal("empty document has no line 1")
	}
	if text, err := m.Slice(Range{0, 0}); err != nil || text != "" {
		t.Fatalf("empty slice = %q, %v", text, err)
	}
	if m.Undo() || m.Redo() || m.CanUndo() || m.CanRedo() {
		t.Fatal("empty model must not have history")
	}
	if err := m.Replace(Range{0, 0}, "hi"); err != nil {
		t.Fatal(err)
	}
	if m.Text() != "hi" {
		t.Fatalf("insert into empty model: %q", m.Text())
	}
	if !m.Undo() || m.Text() != "" {
		t.Fatalf("undo of first insert: %q", m.Text())
	}
}

func TestTextModelNormalizesInput(t *testing.T) {
	m := NewModel("a\r\nb\rc\r\r\n")
	if want := "a\nb\nc\n\n"; m.Text() != want {
		t.Fatalf("normalized text = %q, want %q", m.Text(), want)
	}
	if m.LineCount() != 5 {
		t.Fatalf("lines = %d, want 5", m.LineCount())
	}
	m = NewModel("a\xff\xfe€")
	if want := "a\uFFFD\uFFFD€"; m.Text() != want {
		t.Fatalf("invalid UTF-8 = %q, want %q", m.Text(), want)
	}
	m = NewModel("ok")
	if err := m.Replace(Range{2, 2}, "x\r\ny\xff"); err != nil {
		t.Fatal(err)
	}
	if want := "okx\ny\uFFFD"; m.Text() != want {
		t.Fatalf("replacement normalization = %q, want %q", m.Text(), want)
	}
}

func TestTextModelLineRanges(t *testing.T) {
	for _, tc := range []struct {
		text  string
		lines []string
	}{
		{"", []string{""}},
		{"\n", []string{"", ""}},
		{"no-newline", []string{"no-newline"}},
		{"one\ntwo\n\n三\n", []string{"one", "two", "", "三", ""}},
		{"中\n文\na\n", []string{"中", "文", "a", ""}},
	} {
		m := NewModel(tc.text)
		if m.LineCount() != len(tc.lines) {
			t.Fatalf("%q: lines = %d, want %d", tc.text, m.LineCount(), len(tc.lines))
		}
		for i, want := range tc.lines {
			rng, err := m.LineRange(i)
			if err != nil {
				t.Fatalf("%q: line %d: %v", tc.text, i, err)
			}
			text, err := m.Slice(rng)
			if err != nil || text != want {
				t.Fatalf("%q: line %d = %q (%+v, %v), want %q", tc.text, i, text, rng, err, want)
			}
		}
		if _, err := m.LineRange(-1); err == nil {
			t.Fatalf("%q: negative line accepted", tc.text)
		}
		if _, err := m.LineRange(len(tc.lines)); err == nil {
			t.Fatalf("%q: line past end accepted", tc.text)
		}
	}
}

func TestTextModelRangeValidation(t *testing.T) {
	m := NewModel("héllo") // h(1) é(2) l(3) l(4) o(5), length 6
	invalid := []Range{
		{-1, 0}, {2, 1}, {0, 7}, {7, 7}, {6, 7},
		{2, 4}, // starts inside the two-byte é
		{1, 3}, // ends inside the two-byte é? [1,3) is exactly é, valid
	}
	for _, rng := range invalid {
		if rng.Start == 1 && rng.End == 3 {
			continue
		}
		before := m.Text()
		if err := m.Replace(rng, "x"); err == nil {
			t.Fatalf("Replace accepted invalid range %+v", rng)
		}
		if _, err := m.Slice(rng); err == nil {
			t.Fatalf("Slice accepted invalid range %+v", rng)
		}
		if m.Text() != before || m.Revision() != 0 || m.CanUndo() {
			t.Fatalf("invalid range %+v changed state", rng)
		}
	}
	if err := m.Replace(Range{1, 3}, "e"); err != nil {
		t.Fatalf("rune-aligned replace failed: %v", err)
	}
	if m.Text() != "hello" {
		t.Fatalf("rune-aligned replace = %q", m.Text())
	}
}

func TestTextModelReplaceNoOp(t *testing.T) {
	m := NewModel("abc")
	changes := 0
	handle := m.ConnectChange(func(Change) { changes++ })
	defer handle.Disconnect()
	if err := m.Replace(Range{1, 1}, ""); err != nil {
		t.Fatal(err)
	}
	if m.Revision() != 0 || changes != 0 || m.CanUndo() {
		t.Fatal("empty replace must be a no-op")
	}
}

func TestTextModelChangeSignalProtocol(t *testing.T) {
	m := NewModel("hello")
	var first, second []Change
	h1 := m.ConnectChange(func(c Change) { first = append(first, c) })
	h2 := m.ConnectChange(func(c Change) { second = append(second, c) })
	defer func() { h1.Disconnect(); h2.Disconnect() }()

	steps := []struct {
		do    func()
		reset bool
	}{
		{do: func() { _ = m.Replace(Range{5, 5}, " w") }},
		{do: func() { _ = m.Replace(Range{0, 5}, "HELLO") }},
		{do: func() { m.Undo() }},
		{do: func() { m.Redo() }},
		{do: func() { m.SetText("x") }, reset: true},
		{do: func() { m.SetText("x") }}, // same content: no change
	}
	before := m.Text()
	for i, step := range steps {
		step.do()
		if len(first) != len(second) {
			t.Fatalf("step %d: listeners diverged", i)
		}
		if step.reset {
			c := first[len(first)-1]
			if !c.Reset || len(c.Edits) != 0 || c.Revision != m.Revision() {
				t.Fatalf("reset change = %+v", c)
			}
			before = m.Text()
			continue
		}
		if i == len(steps)-1 {
			if m.Revision() != uint64(len(first)) {
				t.Fatalf("same-content SetText changed revision or emitted: rev=%d changes=%d", m.Revision(), len(first))
			}
			continue
		}
		c := first[len(first)-1]
		if c.Reset || c.Revision != m.Revision() {
			t.Fatalf("step %d: change = %+v", i, c)
		}
		if got := applyTextEdits(before, c.Edits); got != m.Text() {
			t.Fatalf("step %d: edits %v applied to %q = %q, want %q", i, c.Edits, before, got, m.Text())
		}
		before = m.Text()
	}
	if m.Revision() != 5 || len(first) != 5 {
		t.Fatalf("revision = %d, changes = %d", m.Revision(), len(first))
	}
}

func applyTextEdits(text string, edits []Edit) string {
	for _, e := range edits {
		text = text[:e.Range.Start] + e.Text + text[e.Range.End:]
	}
	return text
}

func TestTextModelUndoRedoTypingMerge(t *testing.T) {
	m := NewModel("")
	for _, s := range []string{"a", "b", "c"} {
		if err := m.Replace(Range{m.Len(), m.Len()}, s); err != nil {
			t.Fatal(err)
		}
	}
	if m.Text() != "abc" {
		t.Fatalf("text = %q", m.Text())
	}
	if !m.Undo() {
		t.Fatal("undo rejected")
	}
	if m.Text() != "" || m.Undo() {
		t.Fatalf("consecutive typing must be one undo step: %q", m.Text())
	}
	if !m.Redo() || m.Text() != "abc" {
		t.Fatalf("redo = %q", m.Text())
	}
	if m.Redo() {
		t.Fatal("redo beyond history")
	}
}

func TestTextModelBreakUndoGroup(t *testing.T) {
	m := NewModel("")
	_ = m.Replace(Range{0, 0}, "a")
	m.BreakUndoGroup()
	_ = m.Replace(Range{1, 1}, "b")
	if m.Text() != "ab" {
		t.Fatalf("text = %q", m.Text())
	}
	if !m.Undo() || m.Text() != "a" {
		t.Fatalf("undo after break = %q", m.Text())
	}
	if !m.Undo() || m.Text() != "" {
		t.Fatalf("second undo = %q", m.Text())
	}
}

func TestTextModelDeleteMergeBackwardAndForward(t *testing.T) {
	m := NewModel("abcdef")
	_ = m.Replace(Range{5, 6}, "")
	_ = m.Replace(Range{4, 5}, "")
	if m.Text() != "abcd" {
		t.Fatalf("backspaces = %q", m.Text())
	}
	if !m.Undo() || m.Text() != "abcdef" || m.Undo() {
		t.Fatalf("same-side backspaces must be one undo step: %q", m.Text())
	}

	m = NewModel("abcdef")
	_ = m.Replace(Range{0, 1}, "")
	_ = m.Replace(Range{0, 1}, "")
	if m.Text() != "cdef" {
		t.Fatalf("delete keys = %q", m.Text())
	}
	if !m.Undo() || m.Text() != "abcdef" || m.Undo() {
		t.Fatalf("same-side delete keys must be one undo step: %q", m.Text())
	}
}

func TestTextModelDeleteOppositeSidesDoNotMerge(t *testing.T) {
	// Backspaces remove "f" and "e" (caret at 6), then a Delete key removes
	// "g" on the other side of the caret. Merging those would corrupt the
	// combined restore range, so they must stay separate steps.
	m := NewModel("abcdefgh")
	_ = m.Replace(Range{5, 6}, "")
	_ = m.Replace(Range{4, 5}, "")
	_ = m.Replace(Range{4, 5}, "")
	if m.Text() != "abcdh" {
		t.Fatalf("text = %q", m.Text())
	}
	if !m.Undo() || m.Text() != "abcdgh" {
		t.Fatalf("undo of opposite-side delete = %q", m.Text())
	}
	if !m.Undo() || m.Text() != "abcdefgh" {
		t.Fatalf("undo of backspace group = %q", m.Text())
	}
	if m.Undo() {
		t.Fatal("history must be empty")
	}
	if !m.Redo() || m.Text() != "abcdgh" {
		t.Fatalf("redo = %q", m.Text())
	}
}

func TestTextModelReplaceAtomicStandalone(t *testing.T) {
	m := NewModel("")
	_ = m.Replace(Range{0, 0}, "a")
	if err := m.ReplaceAtomic(Range{1, 1}, "PASTE"); err != nil {
		t.Fatal(err)
	}
	_ = m.Replace(Range{6, 6}, "z")
	if m.Text() != "aPASTEz" {
		t.Fatalf("text = %q", m.Text())
	}
	for i, want := range []string{"aPASTE", "a", ""} {
		if !m.Undo() || m.Text() != want {
			t.Fatalf("undo %d = %q, want %q", i, m.Text(), want)
		}
	}
	if m.Undo() {
		t.Fatal("typing after a paste must not merge into the paste step")
	}
	for i, want := range []string{"a", "aPASTE", "aPASTEz"} {
		if !m.Redo() || m.Text() != want {
			t.Fatalf("redo %d = %q, want %q", i, m.Text(), want)
		}
	}
}

func TestTextModelRedoInvalidatedByEdit(t *testing.T) {
	m := NewModel("")
	_ = m.Replace(Range{0, 0}, "a")
	m.Undo()
	_ = m.Replace(Range{0, 0}, "b")
	if m.CanRedo() || m.Redo() {
		t.Fatal("new edit must invalidate redo history")
	}
	if m.Text() != "b" {
		t.Fatalf("text = %q", m.Text())
	}
}

func TestTextModelSetTextSemantics(t *testing.T) {
	m := NewModel("abc")
	_ = m.Replace(Range{3, 3}, "x")
	revision, changes := m.Revision(), 0
	var last Change
	handle := m.ConnectChange(func(c Change) { changes++; last = c })
	defer handle.Disconnect()

	m.SetText("abcx") // same content
	if m.Revision() != revision || changes != 0 || !m.CanUndo() {
		t.Fatal("same-content SetText must not touch revision, signals or history")
	}
	if !m.Undo() || m.Text() != "abc" {
		t.Fatalf("history survived a no-op SetText: %q", m.Text())
	}
	m.Redo()

	// Undo and redo above emitted changes and bumped the revision; recapture.
	revision, changes = m.Revision(), 0
	m.SetText("new")
	if m.Revision() != revision+1 || changes != 1 || m.CanUndo() || m.CanRedo() {
		t.Fatal("SetText must reload and clear history")
	}
	if !last.Reset || len(last.Edits) != 0 || last.Revision != m.Revision() {
		t.Fatalf("reload change = %+v", last)
	}
	if m.Text() != "new" {
		t.Fatalf("reloaded text = %q", m.Text())
	}

	revision = m.Revision()
	m.SetText("another")
	m.ClearHistory()
	if m.CanUndo() || m.CanRedo() || m.Revision() != revision+1 || changes != 2 || m.Text() != "another" {
		t.Fatal("ClearHistory must only drop history")
	}
}

func TestTextModelReentrantChange(t *testing.T) {
	m := NewModel("")
	calls := 0
	m.ConnectChange(func(Change) {
		calls++
		if calls == 1 {
			if m.Text() != "first" {
				t.Errorf("handler ran before state update: %q", m.Text())
			}
			if err := m.Replace(Range{5, 5}, "+more"); err != nil {
				t.Error(err)
			}
		}
	})
	if err := m.Replace(Range{0, 0}, "first"); err != nil {
		t.Fatal(err)
	}
	if m.Text() != "first+more" || m.Revision() != 2 || calls != 2 {
		t.Fatalf("reentrant edit: text=%q rev=%d calls=%d", m.Text(), m.Revision(), calls)
	}
	checkTextModelTree(t, m)
}

func TestTextModelMiddleTypingUsesMergedPiece(t *testing.T) {
	m := NewModel("ab")
	_ = m.Replace(Range{1, 1}, "X")
	m.BreakUndoGroup()
	_ = m.Replace(Range{2, 2}, "Y")
	if m.Text() != "aXYb" {
		t.Fatalf("text = %q", m.Text())
	}
	// The tree may merge X and Y into one add-buffer piece; undo must still
	// remove exactly the second step's bytes.
	if !m.Undo() || m.Text() != "aXb" {
		t.Fatalf("undo after mid-document typing = %q", m.Text())
	}
	if !m.Redo() || m.Text() != "aXYb" {
		t.Fatalf("redo = %q", m.Text())
	}
	checkTextModelTree(t, m)
}

func TestTextModelLargeDocument(t *testing.T) {
	const lines = 16384
	line := strings.Repeat("a", 63) + "\n" // 64 bytes
	text := strings.Repeat(line, lines)
	m := NewModel(text)
	if m.LineCount() != lines+1 {
		t.Fatalf("lines = %d, want %d", m.LineCount(), lines+1)
	}
	if m.Len() != len(text) || m.Text() != text {
		t.Fatal("large document round-trip failed")
	}
	rng, err := m.LineRange(lines / 2)
	if err != nil || rng.Start != (lines/2)*64 || rng.End != (lines/2)*64+63 {
		t.Fatalf("middle line = %+v, %v", rng, err)
	}
	if s, _ := m.Slice(rng); s != strings.Repeat("a", 63) {
		t.Fatalf("middle line content = %q", s)
	}
	// Sequential line iteration must stay linear: walk every line via the
	// public API and compare against the reference split.
	split := strings.Split(text, "\n")
	for i, want := range split {
		rng, err := m.LineRange(i)
		if err != nil {
			t.Fatal(err)
		}
		if s, err := m.Slice(rng); err != nil || s != want {
			t.Fatalf("line %d = %q, want %q", i, s, want)
		}
	}
	mid := (lines / 2) * 64
	if err := m.Replace(Range{mid + 10, mid + 20}, "中\n文"); err != nil {
		t.Fatal(err)
	}
	want := text[:mid+10] + "中\n文" + text[mid+20:]
	if m.Text() != want {
		t.Fatal("large document edit mismatch")
	}
	if !m.Undo() || m.Text() != text {
		t.Fatal("large document undo mismatch")
	}
	if !m.Redo() || m.Text() != want {
		t.Fatal("large document redo mismatch")
	}
	checkTextModelTree(t, m)
}

// shadowModel is an independent plain-string reference for the stress test: it
// mirrors the documented grouping rules without any piece tree.
type shadowModel struct {
	text      string
	undoStack []shadowStep
	redoStack []shadowStep
	groupOpen bool
}

type shadowStep struct {
	start, oldEnd int
	kind          int
	side          int
	oldText       string
	newText       string
}

const (
	shadowInsert = iota
	shadowDelete
	shadowReplace
)

const (
	shadowSideNone = iota
	shadowSideBackward
	shadowSideForward
)

func (s *shadowModel) replace(rng Range, replacement string, atomic bool) {
	kind := shadowReplace
	switch {
	case replacement == "":
		kind = shadowDelete
	case rng.Start == rng.End:
		kind = shadowInsert
	}
	step := shadowStep{start: rng.Start, oldEnd: rng.End, kind: kind, oldText: s.text[rng.Start:rng.End], newText: replacement}
	s.text = s.text[:rng.Start] + replacement + s.text[rng.End:]
	s.redoStack = nil
	if atomic || !s.tryMerge(&step) {
		s.undoStack = append(s.undoStack, step)
		s.groupOpen = !atomic
	}
}

func (s *shadowModel) tryMerge(step *shadowStep) bool {
	if !s.groupOpen || len(s.undoStack) == 0 {
		return false
	}
	top := &s.undoStack[len(s.undoStack)-1]
	switch {
	case top.kind == shadowInsert && step.kind == shadowInsert && step.start == top.start+len(top.newText):
		top.newText += step.newText
	case top.kind == shadowDelete && step.kind == shadowDelete:
		backward := step.oldEnd == top.start
		forward := step.start == top.start
		switch top.side {
		case shadowSideNone:
			if backward {
				top.side = shadowSideBackward
			} else if forward {
				top.side = shadowSideForward
			} else {
				return false
			}
		case shadowSideBackward:
			if !backward {
				return false
			}
		case shadowSideForward:
			if !forward {
				return false
			}
		}
		if backward {
			top.start = step.start
			top.oldText = step.oldText + top.oldText
		} else {
			top.oldEnd += step.oldEnd - step.start
			top.oldText += step.oldText
		}
	default:
		return false
	}
	return true
}

func (s *shadowModel) undo() bool {
	if len(s.undoStack) == 0 {
		return false
	}
	step := s.undoStack[len(s.undoStack)-1]
	s.undoStack = s.undoStack[:len(s.undoStack)-1]
	s.groupOpen = false
	s.text = s.text[:step.start] + step.oldText + s.text[step.start+len(step.newText):]
	s.redoStack = append(s.redoStack, shadowStep{
		start: step.start, oldEnd: step.start + len(step.oldText),
		kind: step.kind, side: step.side, oldText: step.oldText, newText: step.newText,
	})
	return true
}

func (s *shadowModel) redo() bool {
	if len(s.redoStack) == 0 {
		return false
	}
	step := s.redoStack[len(s.redoStack)-1]
	s.redoStack = s.redoStack[:len(s.redoStack)-1]
	s.groupOpen = false
	s.text = s.text[:step.start] + step.newText + s.text[step.oldEnd:]
	s.undoStack = append(s.undoStack, shadowStep{
		start: step.start, oldEnd: step.start + len(step.newText),
		kind: step.kind, side: step.side, oldText: step.oldText, newText: step.newText,
	})
	return true
}

func (s *shadowModel) setText(text string) {
	if s.text == text {
		return
	}
	s.text = text
	s.undoStack, s.redoStack = nil, nil
	s.groupOpen = false
}

func (s *shadowModel) clearHistory() {
	s.undoStack, s.redoStack = nil, nil
	s.groupOpen = false
}

func TestTextModelRandomEditsAgainstStringReference(t *testing.T) {
	for _, seed := range []int64{1, 2, 7} {
		t.Run("seed"+strconv.FormatInt(seed, 10), func(t *testing.T) {
			random := rand.New(rand.NewSource(seed))
			initial := randomDocument(random, 6000)
			m := NewModel(initial)
			shadow := &shadowModel{text: initial}
			var changes []Change
			m.ConnectChange(func(c Change) { changes = append(changes, c) })

			for op := 0; op < 2500; op++ {
				before := shadow.text
				expectedRevision := m.Revision()
				priorChanges := len(changes)
				switch pick := random.Intn(20); {
				case pick < 12: // edit
					if random.Intn(6) == 0 {
						m.BreakUndoGroup()
						shadow.groupOpen = false
					}
					tr := randomRange(random, shadow.text)
					text := randomSnippet(random, pick == 5)
					if tr.Start == tr.End && text == "" {
						continue
					}
					atomic := random.Intn(5) == 0
					var err error
					if atomic {
						err = m.ReplaceAtomic(tr, text)
					} else {
						err = m.Replace(tr, text)
					}
					if err != nil {
						t.Fatalf("op %d: %v", op, err)
					}
					shadow.replace(tr, text, atomic)
					expectedRevision++
				case pick < 16: // undo
					did := m.Undo()
					if did != shadow.undo() {
						t.Fatalf("op %d: undo availability diverged", op)
					}
					if did {
						expectedRevision++
					}
				case pick < 19: // redo
					did := m.Redo()
					if did != shadow.redo() {
						t.Fatalf("op %d: redo availability diverged", op)
					}
					if did {
						expectedRevision++
					}
				default: // reload or clear
					if random.Intn(2) == 0 {
						text := randomDocument(random, random.Intn(2)*6000+3)
						m.SetText(text)
						shadow.setText(text)
						if text != before {
							expectedRevision++
						}
					} else {
						m.ClearHistory()
						shadow.clearHistory()
					}
				}

				if m.Text() != shadow.text {
					t.Fatalf("op %d: text = %q, want %q", op, m.Text(), shadow.text)
				}
				if m.Len() != len(shadow.text) {
					t.Fatalf("op %d: length mismatch", op)
				}
				if m.Revision() != expectedRevision {
					t.Fatalf("op %d: revision = %d, want %d", op, m.Revision(), expectedRevision)
				}
				if m.CanUndo() != (len(shadow.undoStack) != 0) {
					t.Fatalf("op %d: undo availability = %v, shadow has %d steps", op, m.CanUndo(), len(shadow.undoStack))
				}
				if m.CanRedo() != (len(shadow.redoStack) != 0) {
					t.Fatalf("op %d: redo availability = %v, shadow has %d steps", op, m.CanRedo(), len(shadow.redoStack))
				}
				if lineCount := strings.Count(shadow.text, "\n") + 1; m.LineCount() != lineCount {
					t.Fatalf("op %d: lines = %d, want %d", op, m.LineCount(), lineCount)
				}
				if len(changes) > priorChanges {
					// The op emitted; its edits applied to the pre-op text must
					// reproduce the post-op text. Ops that emit nothing (failed
					// undo/redo, ClearHistory, no-op reload) keep the last signal.
					c := changes[len(changes)-1]
					if c.Revision != m.Revision() {
						t.Fatalf("op %d: signal revision = %d, want %d", op, c.Revision, m.Revision())
					}
					if !c.Reset {
						if got := applyTextEdits(before, c.Edits); got != m.Text() {
							t.Fatalf("op %d: signal edits %v produce %q, want %q", op, c.Edits, got, m.Text())
						}
					}
				}
				if op%16 == 0 {
					lines := strings.Split(shadow.text, "\n")
					for i, want := range lines {
						rng, err := m.LineRange(i)
						if err != nil {
							t.Fatal(err)
						}
						if s, err := m.Slice(rng); err != nil || s != want {
							t.Fatalf("op %d: line %d = %q, want %q (%v)", op, i, s, want, err)
						}
					}
				} else {
					line := random.Intn(m.LineCount())
					rng, err := m.LineRange(line)
					if err != nil {
						t.Fatal(err)
					}
					want := strings.Split(shadow.text, "\n")[line]
					if s, err := m.Slice(rng); err != nil || s != want {
						t.Fatalf("op %d: line %d = %q, want %q (%v)", op, line, s, want, err)
					}
				}
				if op%32 == 0 {
					checkTextModelTree(t, m)
				}
			}
			checkTextModelTree(t, m)
		})
	}
}

func randomRange(random *rand.Rand, text string) Range {
	starts := runeStarts(text)
	a := starts[random.Intn(len(starts))]
	b := starts[random.Intn(len(starts))]
	if a > b {
		a, b = b, a
	}
	return Range{a, b}
}

func runeStarts(text string) []int {
	starts := make([]int, 0, utf8.RuneCountInString(text)+1)
	for i := range text {
		starts = append(starts, i)
	}
	return append(starts, len(text))
}

func randomSnippet(random *rand.Rand, large bool) string {
	letters := []string{"a", "z", "0", "中", "😀", "e\u0301", "ß", "\n", "\n"}
	count := 1 + random.Intn(6)
	if large {
		count = 100 + random.Intn(200)
	}
	var b strings.Builder
	for i := 0; i < count; i++ {
		b.WriteString(letters[random.Intn(len(letters))])
	}
	return b.String()
}

func randomDocument(random *rand.Rand, lines int) string {
	var b strings.Builder
	for i := 0; i < lines; i++ {
		b.WriteString(randomSnippet(random, false))
		b.WriteByte('\n')
	}
	return b.String()
}

// checkTextModelTree verifies the piece tree invariants: AVL balance, parent
// links, subtree aggregates, piece bounds and contiguous in-order coverage.
func checkTextModelTree(t *testing.T, m *Model) {
	t.Helper()
	if m.tree.root != nil && m.tree.root.parent != nil {
		t.Fatal("tree root has a parent")
	}
	var check func(n *pieceNode, parent *pieceNode) (size, lf, height int)
	check = func(n *pieceNode, parent *pieceNode) (int, int, int) {
		if n == nil {
			return 0, 0, 0
		}
		if n.parent != parent {
			t.Fatalf("node parent link wrong")
		}
		if n.piece.length <= 0 {
			t.Fatalf("non-positive piece length %d", n.piece.length)
		}
		buf := m.tree.buffer(n.piece)
		if n.piece.start < 0 || n.piece.start+n.piece.length > len(buf) {
			t.Fatalf("piece %+v escapes its buffer", n.piece)
		}
		lf := pieceLFCount(buf, n.piece, n.piece.length)
		if n.lfCount != lf {
			t.Fatalf("piece LF count = %d, want %d", n.lfCount, lf)
		}
		leftSize, leftLF, leftHeight := check(n.left, n)
		rightSize, rightLF, rightHeight := check(n.right, n)
		if n.size != leftSize+n.piece.length+rightSize {
			t.Fatalf("subtree size = %d, want %d", n.size, leftSize+n.piece.length+rightSize)
		}
		if n.lineFeed != leftLF+lf+rightLF {
			t.Fatalf("subtree LF = %d, want %d", n.lineFeed, leftLF+lf+rightLF)
		}
		height := max(leftHeight, rightHeight) + 1
		if int(n.height) != height {
			t.Fatalf("node height = %d, want %d", n.height, height)
		}
		if abs(leftHeight-rightHeight) > 1 {
			t.Fatalf("AVL balance violated: %d vs %d", leftHeight, rightHeight)
		}
		return n.size, n.lineFeed, height
	}
	totalSize, totalLF, _ := check(m.tree.root, nil)
	if totalSize != m.Len() {
		t.Fatalf("tree size = %d, want %d", totalSize, m.Len())
	}
	if totalLF+1 != m.LineCount() {
		t.Fatalf("tree LF = %d, want %d", totalLF, m.LineCount()-1)
	}
	pos := 0
	for node := m.tree.leftmost(); node != nil; node = m.tree.successor(node) {
		pos += node.piece.length
	}
	if pos != m.Len() {
		t.Fatalf("in-order coverage = %d, want %d", pos, m.Len())
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
