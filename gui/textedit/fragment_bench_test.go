package textedit

import (
	"fmt"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
)

// Both paths receive the same already measured native-like values. This measures
// snapshot/index construction plus first-screen queries, not shaping, rendering,
// event-loop latency or the time to open a document. Fragment includes the cost
// of copying the ordinary metrics; Eager borrows its input as before.
func BenchmarkFragmentFirstScreen(b *testing.B) {
	for _, count := range []int{1000, 10000} {
		const columns = 80
		lines := make([]typography.TextLine, count)
		clusters := make([]typography.TextCluster, 0, count*columns)
		for i := range lines {
			lines[i] = typography.TextLine{Start: i * columns, Length: columns, Y: float32(i * 20), Height: 20, Width: 640}
			for j := 0; j < columns; j++ {
				clusters = append(clusters, typography.TextCluster{Start: i*columns + j, Length: 1, X: float32(j * 8), Y: float32(i * 20), Width: 8, Height: 20, LineIndex: i})
			}
		}
		for _, mode := range []string{"Eager", "Fragment"} {
			b.Run(fmt.Sprintf("%d/%s", count, mode), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					var g Geometry
					if mode == "Eager" {
						g = NewGeometry(lines, clusters, count*columns, 20)
					} else {
						f, err := NewFragment(Range{End: count * columns}, geometry.Size{Width: 640, Height: float32(count * 20)}, lines, clusters)
						if err != nil {
							b.Fatal(err)
						}
						g = f.Geometry(20)
					}
					point := geometry.Point{X: 320, Y: float32(count/2*20 + 10)}
					if g.Hit(point).Offset != (count/2*columns + 40) {
						b.Fatal("incorrect first-screen hit")
					}
					if len(g.SelectionLines(Range{End: count * columns}, count/2, count/2+20)) != 20 {
						b.Fatal("incorrect visible selection")
					}
				}
			})
		}
	}
}
