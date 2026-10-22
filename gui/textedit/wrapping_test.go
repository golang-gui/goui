package textedit

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/golang-gui/goui/platform/typography"
)

func wrapFixture(t testing.TB, text string) WrapMeasurements {
	t.Helper()
	var clusters []typography.TextCluster
	for at, r := range text {
		clusters = append(clusters, typography.TextCluster{Start: at, Length: utf8.RuneLen(r), Width: 1})
	}
	m, err := NewWrapMeasurements(text, clusters)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestWrappingUsesMeasuredAdvancesAndLegalBoundaries(t *testing.T) {
	for _, tc := range []struct {
		text  string
		width float32
		word  bool
		want  []string
	}{
		{"abcdefg", 3, false, []string{"abc", "def", "g"}},
		{"alpha beta gamma", 8, true, []string{"alpha ", "beta ", "gamma"}},
		{"alpha beta", 3, true, []string{"alp", "ha ", "bet", "a"}},
		{"a\u0301b", 1, false, []string{"a\u0301", "b"}},
		{"👩‍💻x", 1, true, []string{"👩‍💻", "x"}},
		{"אבג العربية", 3, false, []string{"אבג", " ال", "عرب", "ية"}},
		{"abc", 0, false, []string{"a", "b", "c"}},
		{"", 1, true, []string{""}},
	} {
		t.Run(tc.text, func(t *testing.T) {
			m := wrapFixture(t, tc.text)
			it := m.Iterator()
			var got []string
			for n := 0; ; n++ {
				if n > utf8.RuneCountInString(tc.text)+1 {
					t.Fatal("non-progressing iterator")
				}
				line, ok := it.Next(tc.width, 0, tc.word)
				if !ok {
					break
				}
				got = append(got, m.Text()[line.Range.Start:line.Range.End])
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWrappingPreservesShapingClustersAndOwnsMeasurements(t *testing.T) {
	cs := []typography.TextCluster{
		{Start: 3, Length: 1, Width: 5, Direction: typography.TextRightToLeft},
		{Start: 0, Length: 3, Width: 17}, // ffi ligature
	}
	m, err := NewWrapMeasurements("ffix", cs)
	if err != nil {
		t.Fatal(err)
	}
	cs[0].Width, cs[1].Width = 0, 0
	first, second := m.Iterator(), m.Iterator()
	a, ok := first.Next(10, 0, true)
	if !ok || a.Range != (Range{0, 3}) || a.Width != 17 {
		t.Fatal(a, ok)
	}
	b, ok := first.Next(10, 0, true)
	if !ok || b.Range != (Range{3, 4}) || b.Width != 5 {
		t.Fatal(b, ok)
	}
	whole, ok := second.Next(22, 0, true)
	if !ok || whole.Range != (Range{0, 4}) || whole.Width != 22 {
		t.Fatal(whole, ok)
	}
}

func TestWrappingExpandsTabsFromEachLineOrigin(t *testing.T) {
	m := wrapFixture(t, "a\tb\tcd")
	it := m.Iterator()
	want := []WrapLine{{Range{0, 3}, 5}, {Range{3, 6}, 6}}
	for _, w := range want {
		got, ok := it.Next(6, 4, false)
		if !ok || got != w {
			t.Fatalf("got %+v/%v want %+v", got, ok, w)
		}
	}
	if _, ok := it.Next(6, 4, false); ok {
		t.Fatal("extra line")
	}
	// A non-fitting tab remains indivisible and must advance the iterator.
	it = m.Iterator()
	for range 6 {
		_, _ = it.Next(0, 4, false)
	}
	if _, ok := it.Next(0, 4, false); ok {
		t.Fatal("zero-width tab failed to progress")
	}
}

func TestWrappingRejectsIncompleteOrInvalidMeasurements(t *testing.T) {
	for _, cs := range [][]typography.TextCluster{
		nil,
		{{Start: 1, Length: 1, Width: 1}},
		{{Start: 0, Length: 1, Width: 1}},
		{{Start: 0, Length: 2, Width: -1}},
		{{Start: 0, Length: 2, Width: float32(math.NaN())}},
		{{Start: 0, Length: 2, Width: float32(math.Inf(1))}},
		{{Start: 0, Length: 1, Width: 1}, {Start: 0, Length: 2, Width: 1}},
	} {
		if _, err := NewWrapMeasurements("ab", cs); err == nil {
			t.Fatal("accepted", cs)
		}
	}
	if _, err := NewWrapMeasurements("é", []typography.TextCluster{{Length: 1, Width: 1}, {Start: 1, Length: 1, Width: 1}}); err == nil {
		t.Fatal("accepted a split UTF-8 rune")
	}
}

func TestWrappingRejectsForcedLineBreaks(t *testing.T) {
	for _, text := range []string{"a\nb", "a\rb", "a\vb", "a\fb", "a\u0085b", "a\u2028b", "a\u2029b"} {
		var clusters []typography.TextCluster
		for at, r := range text {
			clusters = append(clusters, typography.TextCluster{Start: at, Length: utf8.RuneLen(r), Width: 1})
		}
		if _, err := NewWrapMeasurements(text, clusters); err == nil {
			t.Fatal("accepted forced line break", text)
		}
	}
}

func BenchmarkWrappingMeasuredLongParagraph(b *testing.B) {
	m := wrapFixture(b, strings.Repeat("alpha beta gamma ", 8192))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		it := m.Iterator()
		for {
			if _, ok := it.Next(120, 0, true); !ok {
				break
			}
		}
	}
}

func TestWrappingLeadingZeroWidthControlsStayWithVisibleCluster(t *testing.T) {
	m, err := NewWrapMeasurements("\u202eab", []typography.TextCluster{{Start: 0, Length: 3}, {Start: 3, Length: 1, Width: 9}, {Start: 4, Length: 1, Width: 9}})
	if err != nil {
		t.Fatal(err)
	}
	it := m.Iterator()
	line, ok := it.Next(1, 0, true)
	if !ok || line.Range != (Range{0, 4}) || line.Width != 9 {
		t.Fatal(line, ok)
	}
	line, ok = it.Next(1, 0, true)
	if !ok || line.Range != (Range{4, 5}) || line.Width != 9 {
		t.Fatal(line, ok)
	}
}
