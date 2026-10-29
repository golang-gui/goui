// Package textformat parses Label text into plain UTF-8 text and named style
// ranges. It has no dependency on widgets, themes, native typography or mlang.
package textformat

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

const prefix = `\{format\}`
const maxDepth = 32

// Run is a non-empty half-open UTF-8 byte range in Text.Plain. Styles lists
// names from outermost to innermost. Runs do not overlap.
type Run struct {
	Start, End int
	Styles     []string
}

// Text is an owned parse result without native resources. Plain is the displayed
// text; Formatted reports a successfully parsed format-prefixed source. Runs
// contains only styled ranges. Callers may modify their own result.
type Text struct {
	Plain     string
	Runs      []Run
	Formatted bool
}

// ParseError describes a syntax error at a zero-based byte offset in the source,
// including its format prefix.
type ParseError struct {
	Offset int
	Reason string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("textformat: %s at byte %d", e.Reason, e.Offset)
}

// Parse preserves the source verbatim unless it begins with \{format\}.
// Formatted text supports \{name:body\}, nested up to 32 levels, and \\ for a
// literal backslash. Other backslash sequences and all whitespace are preserved.
// Names are case-sensitive, non-empty, and exclude colon, backslash, braces and
// line breaks. Unknown style names are valid: resolving names is the caller's
// responsibility. A syntax error returns a zero Text and a *ParseError.
func Parse(source string) (Text, error) {
	if !strings.HasPrefix(source, prefix) {
		return Text{Plain: source}, nil
	}
	for i := len(prefix); i < len(source); {
		_, size := utf8.DecodeRuneInString(source[i:])
		if size == 1 && source[i] >= utf8.RuneSelf {
			return Text{}, &ParseError{i, "invalid UTF-8"}
		}
		i += size
	}

	var plain strings.Builder
	var runs []Run
	var names []string
	var starts []int
	start := 0
	flush := func() {
		end := plain.Len()
		if end > start && len(names) > 0 {
			if n := len(runs); n > 0 && runs[n-1].End == start && slices.Equal(runs[n-1].Styles, names) {
				runs[n-1].End = end
			} else {
				runs = append(runs, Run{Start: start, End: end, Styles: slices.Clone(names)})
			}
		}
		start = end
	}
	for i := len(prefix); i < len(source); {
		if source[i] != '\\' {
			end := strings.IndexByte(source[i:], '\\')
			if end < 0 {
				end = len(source) - i
			}
			plain.WriteString(source[i : i+end])
			i += end
			continue
		}
		if i+1 < len(source) {
			switch source[i+1] {
			case '\\':
				plain.WriteByte('\\')
				i += 2
				continue
			case '}':
				if len(names) == 0 {
					return Text{}, &ParseError{i, "unexpected range end"}
				}
				flush()
				names, starts = names[:len(names)-1], starts[:len(starts)-1]
				i += 2
				continue
			case '{':
				end := i + 2
				for end < len(source) && !strings.ContainsRune(":\\{}\r\n", rune(source[end])) {
					end++
				}
				if end == len(source) || source[end] != ':' {
					return Text{}, &ParseError{i, "missing style separator"}
				}
				if end == i+2 {
					return Text{}, &ParseError{i, "empty style name"}
				}
				if len(names) == maxDepth {
					return Text{}, &ParseError{i, "nesting exceeds 32 levels"}
				}
				flush()
				names, starts = append(names, source[i+2:end]), append(starts, i)
				i = end + 1
				continue
			}
		}
		plain.WriteByte('\\')
		i++
	}
	if len(names) > 0 {
		return Text{}, &ParseError{starts[len(starts)-1], "unclosed style range"}
	}
	flush()
	return Text{Plain: plain.String(), Runs: runs, Formatted: true}, nil
}

// PlainText extracts display text from a format source. Invalid syntax returns
// the complete source, matching Label's verbatim fallback. Use an existing
// Label's PlainText method when available to reuse its parsed text. This is not
// a general operation on semantic strings, which may contain literal markup.
func PlainText(source string) string {
	text, err := Parse(source)
	if err != nil {
		return source
	}
	return text.Plain
}

// EscapeArg prepares a literal string argument for %s or %[n]s substitution
// into template. It is a no-op for plain templates and escapes backslashes for
// format-prefixed templates. It does not format arguments or validate template.
func EscapeArg(template, value string) string {
	if !strings.HasPrefix(template, prefix) {
		return value
	}
	return strings.ReplaceAll(value, `\`, `\\`)
}

// Literal returns a source whose displayed text is value, even when value
// itself starts with the format prefix. The returned source may differ from
// value. Use EscapeArg instead when interpolating into another template.
func Literal(value string) string {
	if !strings.HasPrefix(value, prefix) {
		return value
	}
	return prefix + EscapeArg(prefix, value)
}
