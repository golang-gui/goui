package software

import (
	"bytes"
	"image"
	"image/color"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/pango"
)

// Compare the complete premultiplied RGBA surface, on the same native font,
// including italic overhang, transparent edges, partial alpha and nonintegral
// device scales. Splitting must preserve the whole layout's glyph phase.
func TestGlyphRangesMatchWholeTextAtFractionalScale(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := pango.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	l, err := ctx.(typography.GlyphContext).NewGlyphLayout("f    f    f    e\u0301    ffi    ", typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans", Size: 37, Style: typography.FontStyleItalic}, TextColor: color.NRGBA{R: 170, B: 90, A: 180}})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Destroy()
	m, err := l.GlyphMetrics()
	if err != nil {
		t.Fatal(err)
	}
	t.Log("metrics", m.Line, m.Clusters)
	if m.InkPadding.Right < 2 {
		t.Fatal("fixture did not expose italic overhang", m.InkPadding)
	}
	base, err := NewPainter(new(testDrawer))
	if err != nil {
		t.Fatal(err)
	}
	defer base.Destroy()
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		whole, err := base.RenderImage(1200, 180, scale, func() { base.DrawTextLayout(geometry.Point{X: 20, Y: 20}, l) })
		if err != nil {
			t.Fatal(err)
		}
		split, err := base.RenderImage(1200, 180, scale, func() {
			for _, i := range m.VisualOrder {
				c := m.Clusters[i]
				base.DrawTextGlyphs(geometry.Point{X: 20 + c.X, Y: 20}, l, c.Start, c.Length, nil)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		a, b := whole.(*image.RGBA), split.(*image.RGBA)
		if !bytes.Equal(a.Pix, b.Pix) {
			for y := 0; y < a.Rect.Dy(); y++ {
				for x := 0; x < a.Rect.Dx(); x++ {
					if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
						t.Fatalf("scale %v (%d,%d): split %v whole %v", scale, x, y, b.RGBAAt(x, y), a.RGBAAt(x, y))
					}
				}
			}
		}
	}
}
