package textedit

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/golang-gui/goui/core/signal"
)

// Range is a half-open [Start, End) range of UTF-8 byte offsets within a
// Model. Positions are always rune boundaries of the normalized document.
type Range struct {
	Start, End int
}

// Selection preserves selection direction. Anchor and Caret are UTF-8 byte
// offsets in the model; an empty selection has equal endpoints.
type Selection struct {
	Anchor int
	Caret  int
}

func (s Selection) Range() Range {
	return Range{min(s.Anchor, s.Caret), max(s.Anchor, s.Caret)}
}

// Edit is one edit of a change: the bytes in Range (in the document as it
// stands before this edit runs) are replaced by Text.
type Edit struct {
	Range Range
	Text  string
}

// Change reports one committed model revision. Edits apply in order: each
// range refers to the text after the previous edit executed, so applying them
// sequentially to the old document yields the new one. Reset marks a reload
// (SetText with different content): Edits is empty and readers must re-read
// the whole document. Revision increases monotonically with every change.
type Change struct {
	Revision uint64
	Edits    []Edit
	Reset    bool
}

// Model is a shareable plain-text document: normalized UTF-8 content with
// byte positions, a logical line index, a monotonic revision and undo/redo
// history (doc/DesignTextEditing.md §3). A model owns no native resources and
// needs no Destroy; views own their signal handles.
//
// A model is sequential: bound to a GUI it must only be touched on the GUI
// thread. Change handlers run synchronously after each mutation and may edit
// the model again. Notifications retain revision order even under reentry;
// querying the model in a handler may see a newer committed revision. Edits
// should be applied in notification order, not inferred from current content.
type Model struct {
	tree       pieceTree
	revision   uint64
	change     signal.Signal1[Change]
	undoSteps  []textStep
	redoSteps  []textStep
	groupOpen  bool // the top undo step still absorbs consecutive same-kind edits
	notifying  bool
	pending    []Change
	lastEditor *EditSource
}

// NewModel creates a model holding the normalized form of text: invalid
// UTF-8 sequences are replaced with U+FFFD and CR/CRLF line breaks become LF.
func NewModel(text string) *Model {
	m := &Model{}
	m.tree.build(Normalize(text))
	return m
}

// Text returns the whole document.
func (m *Model) Text() string {
	return m.tree.text()
}

// EqualText compares against normalized text without copying the document.
func (m *Model) EqualText(text string) bool {
	return m.tree.equalText(Normalize(text))
}

// Len returns the document size in bytes.
func (m *Model) Len() int {
	return m.tree.length()
}

// Revision returns the current revision; it starts at zero and increases with
// every change signal.
func (m *Model) Revision() uint64 {
	return m.revision
}

// LineCount returns the number of logical lines. An empty document has one
// line and every LF at the end starts an empty line.
func (m *Model) LineCount() int {
	return m.tree.lineCount()
}

// LineRange returns the byte range of line, excluding the line separator.
func (m *Model) LineRange(line int) (Range, error) {
	if line < 0 || line >= m.LineCount() {
		return Range{}, fmt.Errorf("text model: line %d out of range [0, %d)", line, m.LineCount())
	}
	start := 0
	if line > 0 {
		off, ok := m.tree.findLF(line - 1)
		if !ok {
			return Range{}, errors.New("text model: inconsistent line index")
		}
		start = off + 1
	}
	end := m.Len()
	if off, ok := m.tree.findLF(line); ok {
		end = off
	}
	return Range{Start: start, End: end}, nil
}

// Slice returns the bytes in rng. Like Replace it validates the range and rune
// boundaries and refuses invalid ranges instead of clamping.
func (m *Model) Slice(rng Range) (string, error) {
	if err := m.validateRange(rng); err != nil {
		return "", err
	}
	if rng.Start == rng.End {
		return "", nil
	}
	return m.tree.sliceRange(rng.Start, rng.End), nil
}

// SetText reloads the model with the normalized form of text and clears the
// history, emitting a Reset change. Loading equal content is a no-op: no
// signal, revision or history change.
func (m *Model) SetText(text string) {
	text = Normalize(text)
	if m.tree.equalText(text) {
		return
	}
	m.tree.build(text)
	m.undoSteps, m.redoSteps = nil, nil
	m.groupOpen = false
	m.revision++
	m.notify(Change{Revision: m.revision, Reset: true})
}

// Replace replaces the bytes in rng with the normalized replacement. This is
// the ordinary typing/editing path: consecutive input at the end of the last
// edit and same-side consecutive deletes merge into one undo step. Replacing
// an empty range with empty text is a no-op.
func (m *Model) Replace(rng Range, replacement string) error {
	return m.replace(rng, replacement, false, nil, nil)
}

// ReplaceAtomic applies an edit as a standalone undo step that neither merges
// with the previous group nor absorbs later edits: one paste or one IME commit
// is one undo step.
func (m *Model) ReplaceAtomic(rng Range, replacement string) error {
	return m.replace(rng, replacement, true, nil, nil)
}

// BreakUndoGroup closes the open merge group so the next Replace starts a new
// undo step. Views call it on caret/selection movement and when the editing
// view switches.
func (m *Model) BreakUndoGroup() {
	m.groupOpen = false
}

// Undo reverts the most recent undo step and reports whether one existed.
func (m *Model) Undo() bool { return m.UndoWithSelection().Applied }

// UndoWithSelection also returns the selection recorded before the edit.
// The result belongs to this operation, even if change callbacks edit again.
func (m *Model) UndoWithSelection() HistoryResult {
	if len(m.undoSteps) == 0 {
		return HistoryResult{}
	}
	step := m.undoSteps[len(m.undoSteps)-1]
	m.undoSteps[len(m.undoSteps)-1] = textStep{}
	m.undoSteps = m.undoSteps[:len(m.undoSteps)-1]
	m.groupOpen = false
	insLen := pieceRefsLen(step.inserted)
	var removedNow []pieceRef
	if insLen > 0 {
		removedNow = m.tree.deleteRange(step.start, step.start+insLen)
	}
	m.tree.insertPiecesAt(step.start, step.removed)
	m.redoSteps = append(m.redoSteps, textStep{
		start:     step.start,
		oldEnd:    step.start + pieceRefsLen(step.removed),
		removed:   step.removed,
		inserted:  removedNow,
		kind:      step.kind,
		side:      step.side,
		selection: step.selection,
	})
	m.revision++
	result := HistoryResult{Applied: true, Revision: m.revision}
	if step.selection != nil {
		selection := step.selection.before
		result.Selection = &selection
	}
	m.notify(Change{
		Revision: m.revision,
		Edits:    []Edit{{Range: Range{Start: step.start, End: step.start + insLen}, Text: m.tree.textOfRefs(step.removed)}},
	})
	return result
}

// Redo reapplies the most recently undone step and reports whether one existed.
func (m *Model) Redo() bool { return m.RedoWithSelection().Applied }

// RedoWithSelection also returns the selection recorded after the edit.
func (m *Model) RedoWithSelection() HistoryResult {
	if len(m.redoSteps) == 0 {
		return HistoryResult{}
	}
	step := m.redoSteps[len(m.redoSteps)-1]
	m.redoSteps[len(m.redoSteps)-1] = textStep{}
	m.redoSteps = m.redoSteps[:len(m.redoSteps)-1]
	m.groupOpen = false
	var removedNow []pieceRef
	if step.oldEnd > step.start {
		removedNow = m.tree.deleteRange(step.start, step.oldEnd)
	}
	m.tree.insertPiecesAt(step.start, step.inserted)
	m.undoSteps = append(m.undoSteps, textStep{
		start:     step.start,
		oldEnd:    step.start + pieceRefsLen(step.inserted),
		removed:   removedNow,
		inserted:  step.inserted,
		kind:      step.kind,
		side:      step.side,
		selection: step.selection,
	})
	m.revision++
	result := HistoryResult{Applied: true, Revision: m.revision}
	if step.selection != nil {
		selection := step.selection.after
		result.Selection = &selection
	}
	m.notify(Change{
		Revision: m.revision,
		Edits:    []Edit{{Range: Range{Start: step.start, End: step.oldEnd}, Text: m.tree.textOfRefs(step.inserted)}},
	})
	return result
}

// CanUndo reports whether an undo step exists.
func (m *Model) CanUndo() bool {
	return len(m.undoSteps) != 0
}

// CanRedo reports whether a redo step exists.
func (m *Model) CanRedo() bool {
	return len(m.redoSteps) != 0
}

// ClearHistory drops the undo and redo history. Content, revision and the
// change signal are unaffected.
func (m *Model) ClearHistory() {
	m.undoSteps, m.redoSteps = nil, nil
	m.groupOpen = false
}

// ConnectChange subscribes to document changes; disconnect via the handle.
func (m *Model) ConnectChange(fn func(Change)) signal.Handle {
	return m.change.Connect(fn)
}

// EditOptions describes the origin and history policy of an Apply operation.
// Source may be nil for edits not associated with a view. Selection is copied;
// it records the pre-edit selection, with the post-edit caret at inserted text's end.
type EditOptions struct {
	Source    *EditSource
	Selection *Selection
	Atomic    bool
}

// HistoryResult describes a completed undo/redo. Selection is nil if the edit
// had no selection metadata; a non-nil selection is an independent copy.
// Revision identifies this operation, not any reentrant edit made by a listener.
type HistoryResult struct {
	Applied   bool
	Revision  uint64
	Selection *Selection
}

// Apply performs an edit with explicit source and history metadata. Like
// Replace, it normalizes text and validates UTF-8 byte ranges.
func (m *Model) Apply(edit Edit, options EditOptions) error {
	return m.replace(edit.Range, edit.Text, options.Atomic, options.Source, options.Selection)
}

func (m *Model) replace(rng Range, replacement string, atomic bool, owner *EditSource, selection *Selection) error {
	if err := m.validateRange(rng); err != nil {
		return err
	}
	text := Normalize(replacement)
	if rng.Start == rng.End && text == "" {
		return nil
	}
	if owner != m.lastEditor {
		m.BreakUndoGroup()
	}
	m.lastEditor = owner
	var removed []pieceRef
	if rng.End > rng.Start {
		removed = m.tree.deleteRange(rng.Start, rng.End)
	}
	var inserted []pieceRef
	if text != "" {
		inserted = m.tree.insertText(rng.Start, text)
	}
	kind := stepReplace
	switch {
	case text == "":
		kind = stepDelete
	case rng.Start == rng.End:
		kind = stepInsert
	}
	step := textStep{start: rng.Start, oldEnd: rng.End, removed: removed, inserted: inserted, kind: kind}
	if selection != nil {
		caret := rng.Start + len(text)
		step.selection = &textStepSelection{before: *selection, after: Selection{caret, caret}}
	}
	if atomic || !m.tryMergeStep(&step) {
		m.undoSteps = append(m.undoSteps, step)
		m.redoSteps = nil
		m.groupOpen = !atomic
	}
	m.revision++
	m.notify(Change{
		Revision: m.revision,
		Edits:    []Edit{{Range: rng, Text: text}},
	})
	return nil
}

// Reentrant edits commit immediately, but must not deliver revision N+1 to a
// later listener before it has seen N. A listener querying the model can see a
// newer revision; views defer layout reads until change.Revision == Revision().
func (m *Model) notify(change Change) {
	m.pending = append(m.pending, change)
	if m.notifying {
		return
	}
	m.notifying = true
	defer func() { m.notifying = false }()
	for len(m.pending) > 0 {
		next := m.pending[0]
		m.pending[0] = Change{}
		m.pending = m.pending[1:]
		m.change.Emit(next)
	}
	m.pending = nil
}

// LineAt returns the zero-based logical paragraph containing a byte position. Counting the
// LF prefix uses subtree aggregates and scans only the final piece.
func (m *Model) LineAt(offset int) int {
	offset = min(max(0, offset), m.Len())
	n, lines := m.tree.root, 0
	for n != nil {
		left := nodeSize(n.left)
		if offset < left {
			n = n.left
			continue
		}
		lines += nodeLF(n.left)
		offset -= left
		if offset <= n.piece.length {
			for _, b := range m.tree.buffer(n.piece)[n.piece.start : n.piece.start+offset] {
				if b == '\n' {
					lines++
				}
			}
			return lines
		}
		lines += n.lfCount
		offset -= n.piece.length
		n = n.right
	}
	return lines
}

func (m *Model) validateRange(rng Range) error {
	if rng.Start < 0 || rng.End < rng.Start || rng.End > m.Len() {
		return fmt.Errorf("text model: invalid range [%d, %d) for length %d", rng.Start, rng.End, m.Len())
	}
	if !m.IsRuneBoundary(rng.Start) || !m.IsRuneBoundary(rng.End) {
		return fmt.Errorf("text model: range [%d, %d) does not start and end at rune boundaries", rng.Start, rng.End)
	}
	return nil
}

// IsRuneBoundary reports whether pos is a valid UTF-8 boundary in the document.
// Document endpoints are boundaries; out-of-range positions are not.
func (m *Model) IsRuneBoundary(pos int) bool {
	if pos <= 0 || pos >= m.Len() {
		return pos == 0 || pos == m.Len()
	}
	node, _, idx := m.tree.nodeAt(pos)
	return utf8.RuneStart(m.tree.buffer(node.piece)[node.piece.start+idx])
}

// tryMergeStep folds step into the open top undo step when the edit continues
// it: input appended to the previous input, or a delete extending the previous
// delete toward the same side. Opposite-side deletes never merge.
func (m *Model) tryMergeStep(step *textStep) bool {
	if !m.groupOpen || len(m.undoSteps) == 0 {
		return false
	}
	top := &m.undoSteps[len(m.undoSteps)-1]
	switch {
	case top.kind == stepInsert && step.kind == stepInsert &&
		step.start == top.start+pieceRefsLen(top.inserted):
		top.inserted = appendPieceRefs(top.inserted, step.inserted)
	case top.kind == stepDelete && step.kind == stepDelete:
		backward := step.oldEnd == top.start
		forward := step.start == top.start
		switch top.side {
		case sideNone:
			if backward {
				top.side = sideBackward
			} else if forward {
				top.side = sideForward
			} else {
				return false
			}
		case sideBackward:
			if !backward {
				return false
			}
		case sideForward:
			if !forward {
				return false
			}
		}
		if backward {
			top.start = step.start
			top.removed = prependPieceRefs(top.removed, step.removed)
		} else {
			top.oldEnd += step.oldEnd - step.start
			top.removed = appendPieceRefs(top.removed, step.removed)
		}
	default:
		return false
	}
	if top.selection != nil && step.selection != nil {
		top.selection.after = step.selection.after
	}
	return true
}

// EditSource is an identity token for an editing view. Allocate a distinct
// &EditSource{} per view; reuse it for that view's edits. The model retains only
// this token, never the view or a closure. Switching sources breaks undo merging.
type EditSource struct{ marker byte }
type textStepSelection struct{ before, after Selection }

// textStep is one undo step: the edit replaced the pieces covering
// [start, oldEnd) of the pre-step document with the pieces covering
// [start, start+len(inserted)) afterwards. Piece references keep the content
// alive in the immutable buffers; no document snapshot is stored.
type textStep struct {
	start     int
	oldEnd    int
	removed   []pieceRef
	inserted  []pieceRef
	kind      textStepKind
	side      textStepSide
	selection *textStepSelection
}

type textStepKind uint8

const (
	stepReplace textStepKind = iota
	stepInsert
	stepDelete
)

type textStepSide uint8

const (
	sideNone     textStepSide = iota
	sideBackward              // deletes extend toward smaller offsets
	sideForward               // deletes extend toward larger offsets
)

// Normalize returns the model's canonical form of s: valid UTF-8 (invalid
// sequences become U+FFFD) with LF as the only line separator (CR and CRLF
// collapse to LF). Positions passed to the model always refer to this form.
func Normalize(s string) string {
	if utf8.ValidString(s) && !strings.ContainsRune(s, '\r') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			b.WriteRune(utf8.RuneError)
		case r == '\r':
			b.WriteByte('\n')
			if i+size < len(s) && s[i+size] == '\n' {
				i += size // also skip the LF of a CRLF pair
			}
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}
