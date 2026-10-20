package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/platform/typography"
)

func TestTextEditorSelectionCullsWrappedLines(t *testing.T) {
	const count = 1000
	lines := make([]typography.TextLine, count)
	clusters := make([]typography.TextCluster, count)
	for i := range lines {
		lines[i] = typography.TextLine{Start: i, Length: 1, Y: float32(i * 20), Height: 20}
		clusters[i] = typography.TextCluster{Start: i, Length: 1, Width: 10, LineIndex: i}
	}
	paragraph := &textParagraph{
		geometry: textedit.NewGeometry(lines, clusters, count, 20), geometryValid: true,
	}
	editor := &textEditor{viewport: geometry.Size{Width: 200, Height: 48}, padding: 5}
	rects := editor.visibleSelection(paragraph, TextRange{End: count}, geometry.Point{Y: 5 - 10001})
	if len(rects) != 2 {
		t.Fatalf("selection produced %d rectangles, want 2", len(rects))
	}
	for i, rect := range rects {
		want := geometry.Rect(0, float32(10000+i*20), 10, 20)
		if rect != want {
			t.Fatalf("rectangle %d = %v, want %v", i, rect, want)
		}
	}
}
