package textedit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func benchmarkModelLineQueries(b *testing.B, text string) {
	m := NewModel(text)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Alternate unrelated positions; do not measure only the sequential LF
		// cache. The same path is used by viewport anchoring and caret input.
		row := int(uint64(i+1) * 2654435761 % uint64(m.LineCount()))
		r, err := m.LineRange(row)
		if err != nil || m.LineAt(r.Start) != row {
			b.Fatal("line mapping", row, r, err)
		}
	}
}

func BenchmarkModelLineQueries(b *testing.B) {
	benchmarkModelLineQueries(b, strings.Repeat("  short text line 中文\n", 400000))
}

// Optional downloaded corpus, never fetched by tests. Pass an existing directory
// explicitly with GOUI_CORPUS_DIR. Input loading/model construction are outside
// timing; benchmarks report only public line/offset queries with an assertion.
func BenchmarkCorpusLineQueries(b *testing.B) {
	dir := os.Getenv("GOUI_CORPUS_DIR")
	if dir == "" {
		b.Skip("optional corpus benchmark: set GOUI_CORPUS_DIR to the downloaded directory")
	}
	for _, name := range []string{"war-and-peace.txt", "github-api.json", "opengl.xml", "war-and-peace.html", "rust-releases.md"} {
		b.Run(name, func(b *testing.B) {
			text, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				b.Fatal(err)
			}
			benchmarkModelLineQueries(b, string(text))
		})
	}
}
