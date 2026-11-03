package textedit

import (
	"fmt"
	"unicode"

	"github.com/rivo/uniseg"
)

// Input is a borrowed user edit. Range addresses the unchanged document in
// UTF-8 bytes; Text is the proposed insertion. Read methods are valid only
// during filtering. A filter must not retain Input or modify its source.
type Input struct {
	Range    Range
	Text     string
	model    *Model
	revision uint64
}

// NewInput borrows a valid edit without copying the document. Text must already
// be normalized. The caller must discard the input after changing the model.
func NewInput(model *Model, edit Edit) (Input, error) {
	if model == nil {
		return Input{}, fmt.Errorf("text input: nil model")
	}
	if err := model.validateRange(edit.Range); err != nil {
		return Input{}, err
	}
	return Input{Range: edit.Range, Text: edit.Text, model: model, revision: model.Revision()}, nil
}

// CurrentSlice reads the unchanged document. A stale input returns an error.
func (in Input) CurrentSlice(rng Range) (string, error) {
	if in.model == nil || in.model.Revision() != in.revision {
		return "", fmt.Errorf("text input: expired source")
	}
	return in.model.Slice(rng)
}

// CurrentText explicitly materializes the unchanged document.
func (in Input) CurrentText() (string, error) {
	if in.model == nil {
		return "", fmt.Errorf("text input: missing source")
	}
	return in.CurrentSlice(Range{End: in.model.Len()})
}

// ResultText explicitly materializes the proposed document. Character-only
// filters can inspect Text instead and avoid this allocation.
func (in Input) ResultText() (string, error) {
	before, after, err := in.surrounding()
	if err != nil {
		return "", err
	}
	return before + in.Text + after, nil
}

func (in Input) surrounding() (string, string, error) {
	if in.model == nil {
		return "", "", fmt.Errorf("text input: missing source")
	}
	before, err := in.CurrentSlice(Range{End: in.Range.Start})
	if err != nil {
		return "", "", err
	}
	after, err := in.CurrentSlice(Range{Start: in.Range.End, End: in.model.Len()})
	return before, after, err
}

// InputFilter accepts, transforms or rejects one user insertion/replacement.
// Return normalized insertion text and true to accept, or false to reject the
// entire edit. The replacement range is fixed. Filters are synchronous pure
// computations, independent of widgets, IME, history and programmatic changes.
type InputFilter func(Input) (text string, accept bool)

// Chain applies filters in order. Each sees the preceding insertion result
// against the same original range and source. A rejection stops the chain.
// Nil filters are ignored; a chain with no filters is nil.
func Chain(filters ...InputFilter) InputFilter {
	items := make([]InputFilter, 0, len(filters))
	for _, f := range filters {
		if f != nil {
			items = append(items, f)
		}
	}
	if len(items) == 0 {
		return nil
	}
	return func(in Input) (string, bool) {
		for _, f := range items {
			text, ok := f(in)
			if !ok {
				return "", false
			}
			in.Text = Normalize(text)
		}
		return in.Text, true
	}
}

// AllowRunes rejects an edit containing a disallowed inserted rune. Empty
// insertion (deletion) is accepted. A nil predicate imposes no restriction.
func AllowRunes(allow func(rune) bool) InputFilter {
	if allow == nil {
		return nil
	}
	return func(in Input) (string, bool) {
		for _, r := range in.Text {
			if !allow(r) {
				return "", false
			}
		}
		return in.Text, true
	}
}

// DigitsOnly accepts ASCII 0-9 and deletion. Other Unicode numbers are rejected.
func DigitsOnly() InputFilter {
	return AllowRunes(func(r rune) bool { return r >= '0' && r <= '9' })
}

// LimitLength limits the resulting text in extended grapheme clusters. It
// retains a fitting whole-grapheme prefix of the insertion, leaving surrounding
// text untouched. A nonempty insertion with no fitting prefix is rejected.
// Deletion is always accepted. Existing over-limit text may be edited without
// increasing its grapheme count. Nonpositive limits impose no restriction.
// This explicit global constraint scans document text; it stores no model data.
func LimitLength(limit int) InputFilter {
	if limit <= 0 {
		return nil
	}
	return func(in Input) (string, bool) {
		if in.Text == "" {
			return "", true
		}
		before, after, err := in.surrounding()
		if err != nil {
			return "", false
		}
		current, err := in.CurrentText()
		if err != nil {
			return "", false
		}
		allowed := max(limit, uniseg.GraphemeClusterCount(current))
		if uniseg.GraphemeClusterCount(before+in.Text+after) <= allowed {
			return in.Text, true
		}
		budget := max(0, allowed-uniseg.GraphemeClusterCount(before)-uniseg.GraphemeClusterCount(after))
		// Inspect the budget and two extra clusters for merges at either join.
		// Verify each candidate against the complete result, rather than adding
		// independent string counts. Only a fixed number of full scans is needed.
		g := uniseg.NewGraphemes(in.Text)
		best, count := 0, 0
		for g.Next() {
			count++
			_, end := g.Positions()
			if count >= budget && uniseg.GraphemeClusterCount(before+in.Text[:end]+after) <= allowed {
				best = end
			}
			if count >= budget+2 {
				break
			}
		}
		return in.Text[:best], best > 0
	}
}

// ASCIIUsername accepts ASCII letters, digits, underscore, dot and hyphen,
// followed by the optional grapheme limit. It allows an empty intermediate
// value; application-specific naming and availability checks belong to submit.
func ASCIIUsername(limit int) InputFilter {
	return Chain(AllowRunes(func(r rune) bool {
		return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '.' || r == '-'
	}), LimitLength(limit))
}

// Password accepts Unicode including spaces and emoji, rejects control runes,
// and applies the optional grapheme limit. It does not obscure the widget or
// require minimum length or particular character classes.
func Password(limit int) InputFilter {
	return Chain(AllowRunes(func(r rune) bool { return !unicode.IsControl(r) }), LimitLength(limit))
}
