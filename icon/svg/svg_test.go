package svg

import (
	"bytes"
	"fmt"
	"github.com/golang-gui/goui/gui"
	"image"
	"image/color"
	"strings"
	"sync"
	"testing"
)

func raster(t *testing.T, body string, size int) *image.RGBA {
	t.Helper()
	doc, err := parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	img := doc.rasterize(size, size)
	if img == nil {
		t.Fatal("missing image")
	}
	return img.(*image.RGBA)
}

func TestPublicSourceImageCache(t *testing.T) {
	s, err := Parse(strings.NewReader(`<svg viewBox="0 0 10 10"><rect width="10" height="10" fill="blue"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	var source gui.IconSource = s
	a := source.Image(20, 20, gui.Color{R: .5, A: .5})
	if a == nil {
		t.Fatal("missing image")
	}
	raw := s.raw
	b := source.Image(20, 20, gui.Color{G: 1, A: 1})
	if raw != s.raw {
		t.Fatal("foreground change re-rasterized SVG")
	}
	if a == b || color.RGBAModel.Convert(a.At(10, 10)) != (color.RGBA{R: 128, A: 128}) {
		t.Fatal("retained output mutated")
	}
	original := s.WithRawColors()
	c := original.Image(20, 20, gui.Color{R: 1, A: 1})
	d := original.Image(20, 20, gui.Color{})
	if c != d || color.RGBAModel.Convert(c.At(10, 10)) != (color.RGBA{B: 255, A: 255}) {
		t.Fatal("original colors changed")
	}
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if source.Image(10+n, 10+n, gui.Color{A: 1}) == nil {
				t.Error("missing image")
			}
		}(n)
	}
	wg.Wait()
}

func TestViewBoxAndAspectRatio(t *testing.T) {
	for _, size := range []int{20, 40} {
		img := raster(t, `<svg viewBox="10 20 20 10"><rect x="10" y="20" width="20" height="10" fill="#ff0000"/></svg>`, size)
		for _, sample := range []struct {
			x, y int
			want color.RGBA
		}{
			{size / 2, size / 8, color.RGBA{}}, {size / 2, size / 2, color.RGBA{R: 255, A: 255}},
			{0, size / 2, color.RGBA{R: 255, A: 255}}, {size / 2, size - 1, color.RGBA{}},
		} {
			if got := img.RGBAAt(sample.x, sample.y); got != sample.want {
				t.Fatalf("size %d (%d,%d): %v != %v", size, sample.x, sample.y, got, sample.want)
			}
		}
	}
}

func TestCompoundFillRules(t *testing.T) {
	for _, rule := range []string{"evenodd", "nonzero"} {
		img := raster(t, fmt.Sprintf(`<svg viewBox="0 0 20 20"><path fill-rule="%s" d="M1 1H19V19H1Z M5 5H15V15H5Z"/></svg>`, rule), 20)
		want := uint8(255)
		if rule == "evenodd" {
			want = 0
		}
		if img.RGBAAt(10, 10).A != want || img.RGBAAt(2, 2).A != 255 {
			t.Fatalf("%s hole/outer alpha: %v/%v", rule, img.RGBAAt(10, 10), img.RGBAAt(2, 2))
		}
	}
}

func TestStrokeScalesWithViewportAndLocalTransform(t *testing.T) {
	const svg = `<svg viewBox="0 0 20 20"><g transform="translate(2 2) scale(2)"><path d="M1 4H7" fill="none" stroke="red" stroke-width="2" stroke-linecap="round"/></g></svg>`
	for _, size := range []int{20, 40, 80} {
		img := raster(t, svg, size)
		s := size / 20
		// The transformed line is y=10, width=4 DIP, then scaled by viewport.
		if img.RGBAAt(10*s, 8*s).A != 255 || img.RGBAAt(10*s, 7*s).A != 0 || img.RGBAAt(3*s, 10*s).A < 240 {
			t.Fatalf("size %d stroke/cap did not scale: %v %v %v", size, img.RGBAAt(10*s, 8*s), img.RGBAAt(10*s, 7*s), img.RGBAAt(3*s, 10*s))
		}
	}
}

func TestOpacityOverrideAndInlineStyle(t *testing.T) {
	img := raster(t, `<svg viewBox="0 0 10 10"><g fill-opacity="0.25"><rect width="10" height="10" fill="blue" fill-opacity="0.5" style="fill:red"/></g></svg>`, 10)
	// Third-party opacity quantizes 0.5 to 127; sample an interior pixel with
	// complete coverage, checking absolute premultiplied components, ±1 byte.
	p := img.RGBAAt(5, 5)
	if p.R < 127 || p.R > 128 || p.A != p.R || p.G != 0 || p.B != 0 {
		t.Fatalf("premultiplied opacity/override: %v", p)
	}
}

func TestDashScalesWithViewport(t *testing.T) {
	const source = `<svg viewBox="0 0 20 20"><path d="M2 10H18" fill="none" stroke="black" stroke-width="2" stroke-dasharray="4 4"/></svg>`
	for _, size := range []int{20, 40, 80} {
		img := raster(t, source, size)
		s := size / 20
		// Interior samples: first dash [2,6], gap [6,10], second dash [10,14].
		if img.RGBAAt(3*s, 10*s).A != 255 || img.RGBAAt(8*s, 10*s).A != 0 || img.RGBAAt(12*s, 10*s).A != 255 {
			t.Fatalf("size %d: dash pattern does not scale", size)
		}
	}
}

func TestGradientsAndConcurrentRasterization(t *testing.T) {
	for _, definition := range []string{
		`<linearGradient id="g"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient>`,
		`<radialGradient id="g"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></radialGradient>`,
	} {
		doc, err := parse(strings.NewReader(`<svg viewBox="0 0 20 20"><defs>` + definition + `</defs><rect width="20" height="20" fill="url(#g)"/></svg>`))
		if err != nil {
			t.Fatal(err)
		}
		want := doc.rasterize(40, 40)
		if want == nil {
			t.Fatal("missing image")
		}
		img := want.(*image.RGBA)
		if strings.HasPrefix(definition, "<linear") {
			if img.RGBAAt(2, 20).R < 200 || img.RGBAAt(37, 20).B < 200 {
				t.Fatal("linear gradient endpoints incorrect")
			}
		} else if img.RGBAAt(20, 20).R < 200 || img.RGBAAt(0, 0).B < 200 {
			t.Fatal("radial gradient incorrect")
		}
		var wg sync.WaitGroup
		for j := 0; j < 8; j++ {
			wg.Add(1)
			go func(j int) {
				defer wg.Done()
				if doc.rasterize(20+j, 20+j) == nil {
					t.Error("missing image")
				}
				got := doc.rasterize(40, 40)
				if got == nil {
					t.Error("missing image")
					return
				}
				if !bytes.Equal(got.(*image.RGBA).Pix, img.Pix) {
					t.Error("shared document changed")
				}
			}(j)
		}
		wg.Wait()
	}
}

func TestUnsupportedAndMalformedSVG(t *testing.T) {
	for _, src := range []string{
		``, `<g/>`, `<svg/>`, `<svg width="1" height="1"/><svg/>`,
		`<!DOCTYPE svg><svg width="1" height="1"/>`,
		`<svg viewBox="0 0 0 20"/>`, `<svg width="NaN" height="10"/>`,
		`<svg viewBox="0 0 20 20"><text>Hello</text></svg>`,
		`<svg viewBox="0 0 20 20"><image href="https://example.com/image.png"/></svg>`,
		`<svg viewBox="0 0 20 20"><g opacity="0.5"><rect width="20" height="20"/></g></svg>`,
		`<svg viewBox="0 0 20 20"><rect width="20" height="20" style="clip-path:url(#c)"/></svg>`,
		`<svg viewBox="0 0 20 20"><rect width="20" height="20" fill="url(#missing)"/></svg>`,
		`<svg viewBox="0 0 20 20" color="red"><path fill="currentColor" d="M0 0H20V20Z"/></svg>`,
		`<svg viewBox="0 0 20 20"><path stroke="red" transform="scale(2 1)" d="M1 1L5 5"/></svg>`,
		`<svg viewBox="0 0 20 20"><defs><linearGradient id="g"/></defs></svg>`,
		`<svg viewBox="0 0 20 20"><path d="M?"/></svg>`,
		`<svg viewBox="0 0 20 20"><linearGradient id="g"><stop offset="0"/></linearGradient></svg>`,
		`<svg viewBox="0 0 20 20"><stop offset="0"/></svg>`,
	} {
		t.Run(src, func(t *testing.T) {
			_, err := Parse(strings.NewReader(src))
			if err == nil {
				t.Fatal("accepted unsupported/malformed SVG")
			}
		})
	}
	if _, err := Parse(nil); err == nil {
		t.Fatal("nil reader accepted")
	}
	if _, err := Parse(strings.NewReader(strings.Repeat(" ", (4<<20)+1))); err == nil {
		t.Fatal("unbounded XML accepted")
	}
}

func TestCurrentColorAndShapes(t *testing.T) {
	img := raster(t, `<svg width="20" height="20"><circle cx="5" cy="5" r="3" fill="currentColor"/><polygon points="12,2 18,2 18,8 12,8" fill="blue"/><ellipse cx="10" cy="15" rx="5" ry="3" fill="red"/></svg>`, 20)
	if img.RGBAAt(5, 5) != (color.RGBA{A: 255}) || img.RGBAAt(15, 5) != (color.RGBA{B: 255, A: 255}) || img.RGBAAt(10, 15) != (color.RGBA{R: 255, A: 255}) {
		t.Fatal("basic shapes/currentColor incorrect")
	}
}

func FuzzParse(f *testing.F) {
	f.Add(`<svg viewBox="0 0 10 10"><path d="M0 0L10 10"/></svg>`)
	f.Add(`<svg width="10" height="10"><g transform="scale(2)"><rect width="4" height="4"/></g></svg>`)
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 16384 {
			return
		}
		_, _ = Parse(strings.NewReader(s))
	})
}

func TestPreparedSourceInvalidRequests(t *testing.T) {
	s, err := Parse(strings.NewReader(`<svg viewBox="0 0 10 10"/>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{-1, 0, 4097} {
		if s.Image(size, size, gui.Color{}) != nil {
			t.Fatal("invalid size allocated pixels", size)
		}
	}
	if s.raw != nil {
		t.Fatal("invalid request populated cache")
	}
	if s.Image(16, 16, gui.Color{}) == nil {
		t.Fatal("valid empty SVG must supply transparent pixels")
	}
	if (*Source)(nil).Image(16, 16, gui.Color{}) != nil || (&Source{}).Image(16, 16, gui.Color{}) != nil {
		t.Fatal("unprepared source must be missing")
	}
}
