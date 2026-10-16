package typographytest

import (
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/platform/typography"
)

// Tabs exercises native metrics rather than expanding the input to spaces.
func Tabs(t *testing.T, ctx typography.Context) {
	t.Helper()
	format := typography.TextFormat{Font: typography.FontInfo{Family: "monospace", Size: 12}, TabWidth: 80}
	layout, err := ctx.NewTextLayout("\tX\t\tY", format, 900, 200)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()
	xAt := func(start int) float32 {
		lines, clusters := layout.MeasureMetrics()
		if len(clusters) == 0 {
			for _, line := range lines {
				clusters = append(clusters, line.Clusters...)
			}
		}
		for _, c := range clusters {
			if c.Start == start {
				return c.X
			}
		}
		t.Fatalf("no UTF-8 cluster at %d: %+v", start, clusters)
		return 0
	}
	near := func(got, want float32) {
		t.Helper()
		if math.Abs(float64(got-want)) > 0.1 {
			t.Fatalf("tab coordinate %g, want %g", got, want)
		}
	}
	near(xAt(1), 80)
	near(xAt(4), 240)
	if layout.Text() != "\tX\t\tY" || layout.Format().TabWidth != 80 {
		t.Fatal("tab changed text or format")
	}
	changes := 0
	layout.ConnectChanged(func() { changes++ })
	layout.SetTabWidth(40)
	layout.SetTabWidth(40)
	if changes != 1 {
		t.Fatal("tab invalidation", changes)
	}
	near(xAt(1), 40)
	near(xAt(4), 120)
	layout.SetTextColor(1, 1, color.NRGBA{R: 255, A: 255})
	near(xAt(1), 40)
	near(xAt(4), 120)
	layout.SetTabWidth(0)
	defaultLayout, err := ctx.NewTextLayout(layout.Text(), typography.TextFormat{Font: format.Font}, 900, 200)
	if err != nil {
		t.Fatal(err)
	}
	defer defaultLayout.Destroy()
	w, h := layout.MeasureSize()
	dw, dh := defaultLayout.MeasureSize()
	near(w, dw)
	near(h, dh)
	for _, invalid := range []float32{-1, float32(math.NaN()), float32(math.Inf(1)), 1048576} {
		if bad, err := ctx.NewTextLayout("x", typography.TextFormat{TabWidth: invalid}, 100, 100); err == nil {
			bad.Destroy()
			t.Fatalf("accepted invalid tab %g", invalid)
		}
	}
}
