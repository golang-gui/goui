package coretext

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_foundation"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_text"
	"github.com/golang-gui/goui/platform/typography"
)

// A retained typesetter can create a finite line from the complete attributed
// paragraph. Unlike an independently allocated substring it retains native
// paragraph analysis. This probe does not establish safe arbitrary cut points.
func TestContextualSubstringShaping(t *testing.T) {
	for _, tc := range []struct{ text, target string }{
		{"start \u202e" + strings.Repeat("abcd 1234 ", 410) + "TARGET text\u202c end", "TARGET"},
		{"אבג \u202d" + strings.Repeat("abcd 1234 ", 410) + "TARGET text\u202c סוף", "TARGET"},
		{"start \u2067" + strings.Repeat("עברית العربية 123 ", 180) + "TARGET text\u2069 end", "TARGET"},
		{strings.Repeat("中文 e\u0301 ffi 👩‍💻 العربية עברית\t ", 130) + "TARGET text", "TARGET"},
		{strings.Repeat("العربية 中文 ", 300) + "العربية", "العربية"},
		{strings.Repeat("office ", 600) + "office", "ffi"},
		{strings.Repeat("text ", 900) + "e\u0301 end", "e\u0301"},
	} {
		t.Run(fmt.Sprintf("%q", tc.text[:min(len(tc.text), 16)]), func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Destroy()
			l, err := ctx.NewTextLayout(tc.text, typography.TextFormat{Font: typography.FontInfo{Family: "Arial", Size: 12}}, 1<<24, 1<<24)
			if err != nil {
				t.Fatal(err)
			}
			defer l.Destroy()
			whole := l.(*TextLayout)
			l.MeasureSize()
			if len(whole.lines) != 1 {
				t.Fatalf("reference lines=%d", len(whole.lines))
			}
			setter := CTTypesetterCreateWithAttributedString(whole.attrString)
			if setter == 0 {
				t.Fatal("create typesetter failed")
			}
			defer CFRelease(setter)
			start := strings.LastIndex(tc.text, tc.target)
			r := CFRangeMake(whole.position.ToUtf16(start), whole.position.ToUtf16(start+len(tc.target))-whole.position.ToUtf16(start))
			line := CTTypesetterCreateLine(setter, r)
			if line == 0 {
				t.Fatal("create range line failed")
			}
			defer CFRelease(line)
			got, want := nativeGlyphFacts(line, r), nativeGlyphFacts(whole.lines[0].line, r)
			if len(got) == 0 || !reflect.DeepEqual(got, want) {
				t.Fatalf("range glyph facts=%+v full paragraph=%+v", got, want)
			}
		})
	}
}

type glyphFact struct {
	glyph   uint16
	index   CFIndex
	advance CGSize
	rtl     bool
}

func nativeGlyphFacts(line CTLineRef, r CFRange) []glyphFact {
	var result []glyphFact
	runs := CTLineGetGlyphRuns(line)
	for i := 0; i < CFArrayGetCount(runs); i++ {
		run := CFArrayGetValueAtIndex(runs, i)
		n := CTRunGetGlyphCount(run)
		glyphs := make([]uint16, n)
		advances := make([]CGSize, n)
		indices := make([]CFIndex, n)
		CTRunGetGlyphs(run, CFRange{}, glyphs)
		CTRunGetAdvances(run, CFRange{}, advances)
		CTRunGetStringIndices(run, CFRange{}, indices)
		for j, index := range indices {
			if int(index) >= r.Location && int(index) < r.Location+r.Length {
				result = append(result, glyphFact{glyphs[j], index, advances[j], CTRunGetStatus(run)&KCTRunStatusRightToLeft != 0})
			}
		}
	}
	return result
}
