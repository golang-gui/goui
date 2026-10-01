package widgets

import (
	"image"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

func TestAllocateTabWidth(t *testing.T) {
	for _, tc := range []struct {
		count                             int
		available, minimum, maximum, want float32
		overflow                          bool
	}{
		{0, 0, 96, 200, 0, false},
		{1, 500, 96, 200, 200, false},
		{3, 800, 96, 200, 200, false},
		{3, 608, 96, 200, 200, false},
		{3, 458, 96, 200, 150, false},
		{3, 296, 96, 200, 96, false},
		{3, 295, 96, 200, 96, true},
		{3, 296.75, 96, 200, 96.25, false},
		{3, 0, 96, 200, 96, true},
		{1, 95, 96, 200, 96, true},
		{3, 608, 200, 200, 200, false},
		{3, 607, 200, 200, 200, true},
	} {
		width, overflow := allocateTabWidth(tc.count, tc.available, tc.minimum, tc.maximum)
		if width != tc.want || overflow != tc.overflow {
			t.Fatalf("%+v: got width=%g overflow=%v", tc, width, overflow)
		}
	}
}

func TestTabWidthThresholdStableAndReveal(t *testing.T) {
	bar, view, pages := dragTestBar()
	bar.SetTabWidthRange(0, 0) // restore defaults after the fixed-width input fixture
	view.SetCurrent(pages[2])
	for _, available := range []float32{800, 458, 344, 343, 344, 343, 458} {
		for repeat := 0; repeat < 3; repeat++ {
			bar.Arrange(geometry.Rect(0, 0, available, 40))
			wantWidth := float32(112)
			if available >= 344 {
				wantWidth = min(240, (available-8)/3)
			}
			if bar.overflow != (available < 344) {
				t.Fatalf("threshold oscillated at width=%g", available)
			}
			for i, page := range pages {
				got := bar.items[page].Rect()
				if got.Width != wantWidth || got.X != float32(i)*(wantWidth+4)-bar.scroll {
					t.Fatalf("slots disagree with allocation: %v", got)
				}
			}
			last := bar.items[pages[2]].Rect()
			if last.X < 0 || last.X+last.Width > bar.viewportWidth {
				t.Fatal("resize lost current tab visibility")
			}
			if !bar.overflow && bar.scroll != 0 {
				t.Fatal("shrink mode retained scroll")
			}
		}
	}
	bar.Arrange(geometry.Rect(0, 0, 343, 40))
	bar.setScroll(0)
	bar.Arrange(bar.Rect())
	if bar.scroll != 0 {
		t.Fatal("ordinary layout undid manual scroll")
	}
	bar.beginDrag(pages[0], geometry.Point{X: 44, Y: 20})
	bar.Arrange(geometry.Rect(0, 0, 458, 40))
	if bar.dragPage != nil || bar.lifted != nil {
		t.Fatal("resize retained stale drag geometry")
	}
	bar.beginDrag(pages[0], geometry.Point{X: 20, Y: 20})
	bar.SetTabWidthRange(100, 100)
	if bar.dragPage != nil || bar.lifted != nil {
		t.Fatal("range change retained drag")
	}
}

type tabTestIcon struct{}

func (tabTestIcon) Image(int, int, gui.Color) image.Image { return nil }

func TestTabWidthRangeNormalizationAndAccessoryFloor(t *testing.T) {
	if minimum, maximum := NewTabBar().TabWidthRange(); minimum != 112 || maximum != 240 {
		t.Fatalf("constructor defaults: %g..%g", minimum, maximum)
	}
	bar, _, pages := dragTestBar()
	for _, tc := range []struct{ min, max, wantMin, wantMax float32 }{
		{0, 0, 112, 240}, {-1, -1, 112, 240},
		{float32(math.NaN()), float32(math.Inf(1)), 112, 240},
		{240, 100, 240, 240},
	} {
		bar.SetTabWidthRange(tc.min, tc.max)
		if minimum, maximum := bar.TabWidthRange(); minimum != tc.wantMin || maximum != tc.wantMax {
			t.Fatalf("normalization: %g..%g", minimum, maximum)
		}
	}
	pages[0].SetIcon(tabTestIcon{})
	pages[1].SetClosable(false)
	pages[2].SetTitle("A much longer title must not alter equal widths")
	bar.SetTabWidthRange(10, 10)
	bar.Arrange(geometry.Rect(0, 0, 300, 40))
	for _, page := range pages {
		if width := bar.items[page].Rect().Width; width != 70 {
			t.Fatalf("mixed accessories/titles broke common safety floor: %g", width)
		}
	}
	// A single slot wider than an extremely small viewport retains its minimum;
	// arrow rectangles must not overlap or extend outside the host.
	bar.Arrange(geometry.Rect(0, 0, 30, 40))
	if bar.previous.Rect().Width != 15 || bar.next.Rect().X != 15 || bar.viewportWidth != 0 {
		t.Fatal("tiny viewport produced invalid arrow geometry")
	}
	bar.SetTabWidthRange(100, 220)
	if got := bar.Measure(layout.Unbounded()).Width; got != 668 {
		t.Fatalf("preferred strip width did not follow max: %g", got)
	}
}
