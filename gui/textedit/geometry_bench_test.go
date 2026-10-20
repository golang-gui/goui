package textedit

import (
	"fmt"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
)

func wrappedGeometryFixture(count int) Geometry {
	const columns = 80
	lines := make([]typography.TextLine, count)
	clusters := make([]typography.TextCluster, 0, count*columns)
	for i := range lines {
		lines[i] = typography.TextLine{Start: i * columns, Length: columns, Y: float32(i * 20), Height: 20}
		for j := 0; j < columns; j++ {
			clusters = append(clusters, typography.TextCluster{
				Start: i*columns + j, Length: 1, LineIndex: i,
				X: float32(j * 8), Y: float32(i * 20), Width: 8, Height: 20,
			})
		}
	}
	return NewGeometry(lines, clusters, count*columns, 20)
}

// These benchmarks measure queries on already supplied native metrics. They
// deliberately exclude shaping and index construction, and cannot establish
// first-paint latency or event-loop responsiveness.
func BenchmarkGeometryWrappedQueries(b *testing.B) {
	for _, count := range []int{100, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			g := wrappedGeometryFixture(count)
			b.Run("Caret", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_ = g.Caret(Position{Offset: (count/2+i%20)*80 + 40})
				}
			})
			b.Run("Hit", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_ = g.Hit(geometry.Point{X: 320, Y: float32((count/2+i%20)*20 + 10)})
				}
			})
			b.Run("ShortSelection", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					start := (count/2 + i%20) * 80
					_ = g.Selection(Range{start + 20, start + 40})
				}
			})
		})
	}
}
