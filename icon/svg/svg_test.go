package svg

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/golang-gui/goui/gui"
)

func raster(t *testing.T, body string, size int) *image.RGBA {
	t.Helper()
	source, err := Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	img := source.WithRawColors().Image(size, size, gui.Color{})
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
	// 内部像素的覆盖率为 1；检查绝对预乘分量，保留既有 1/255 量化容差。
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
		source, err := Parse(strings.NewReader(`<svg viewBox="0 0 20 20"><defs>` + definition + `</defs><rect width="20" height="20" fill="url(#g)"/></svg>`))
		if err != nil {
			t.Fatal(err)
		}
		render := func(w, h int) image.Image { return source.WithRawColors().Image(w, h, gui.Color{}) }
		want := render(40, 40)
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
				if render(20+j, 20+j) == nil {
					t.Error("missing image")
				}
				got := render(40, 40)
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
		`<svg viewBox="0 0 0 20"/>`, `<svg width="NaN" height="10"/>`,
		`<svg viewBox="0 0 20 20"><text>Hello</text></svg>`,
		`<svg viewBox="0 0 20 20"><image href="https://example.com/image.png"/></svg>`,
		`<svg viewBox="0 0 20 20"><g opacity="0.5"><rect width="20" height="20"/></g></svg>`,
		`<svg viewBox="0 0 20 20"><rect width="20" height="20" style="clip-path:url(#c)"/></svg>`,
		`<svg viewBox="0 0 20 20"><rect width="20" height="20" fill="url(#missing)"/></svg>`,
		`<svg viewBox="0 0 20 20"><path d="M?"/></svg>`,
		`<!DOCTYPE svg [<!ENTITY external SYSTEM "https://invalid.example/entity">]><svg viewBox="0 0 20 20"><title>&external;</title></svg>`,
	} {
		t.Run(src, func(t *testing.T) {
			source, err := Parse(strings.NewReader(src))
			if err == nil || source != nil {
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

// 使用已知轴对齐矩形的逐像素预期，验证元数据不会改变图形或引入隐藏子树。
func TestXMLCompatibility(t *testing.T) {
	const body = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><rect x="4" y="4" width="12" height="12" fill="blue"/></svg>`
	for _, data := range []string{
		`<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "https://invalid.example/icon.dtd">` + body,
		strings.Replace(body, `viewBox=`, `xml:space="preserve" viewBox=`, 1),
		strings.Replace(body, `fill="blue"`, `fill="blue" clip-rule="evenodd"`, 1),
		strings.Replace(body, `viewBox=`, `xmlns:vendor="urn:private" vendor:fill="red" viewBox=`, 1),
		strings.Replace(body, `<rect`, `<metadata><rect width="20" height="20" fill="red"/></metadata><rect`, 1),
		strings.Replace(body, `<rect`, `<vendor:data xmlns:vendor="urn:private"><rect width="20" height="20" fill="red"/></vendor:data><rect`, 1),
	} {
		img := raster(t, data, 20)
		for y := 0; y < 20; y++ {
			for x := 0; x < 20; x++ {
				want := color.RGBA{}
				if x >= 4 && x < 16 && y >= 4 && y < 16 {
					want = color.RGBA{B: 255, A: 255}
				}
				if got := img.RGBAAt(x, y); got != want {
					t.Fatalf("(%d,%d)=%v，期望 %v；SVG %s", x, y, got, want, data)
				}
			}
		}
	}
}

// 以下是原适配层拒绝、而底层已支持的语法；以独立颜色/透明预期替代旧拒绝断言。
func TestNativeSVGSupport(t *testing.T) {
	for _, test := range []struct {
		name, body string
		x, y       int
		want       color.RGBA
	}{
		{"color 继承", `<g color="blue"><rect width="20" height="20" fill="currentColor"/></g>`, 10, 10, color.RGBA{B: 255, A: 255}},
		{"class 样式", `<style>.blue {fill:blue}</style><rect class="blue" width="20" height="20"/>`, 10, 10, color.RGBA{B: 255, A: 255}},
		{"use 后置定义", `<use href="#shape" fill="blue"/><defs><rect id="shape" width="20" height="20"/></defs>`, 10, 10, color.RGBA{B: 255, A: 255}},
		{"xlink 引用", `<use xmlns:xlink="http://www.w3.org/1999/xlink" xlink:href="#shape"/><defs><rect id="shape" fill="blue" width="20" height="20"/></defs>`, 10, 10, color.RGBA{B: 255, A: 255}},
		{"渐变后置定义", `<rect width="20" height="20" fill="url(#g)"/><linearGradient id="g"><stop offset="0" stop-color="blue"/><stop offset="1" stop-color="blue"/></linearGradient>`, 10, 10, color.RGBA{B: 255, A: 255}},
		{"非均匀描边", `<path stroke="red" fill="none" stroke-width="2" transform="scale(2 1)" d="M2 5H8"/>`, 10, 4, color.RGBA{R: 255, A: 255}},
		{"非均匀描边外部", `<path stroke="red" fill="none" stroke-width="2" transform="scale(2 1)" d="M2 5H8"/>`, 10, 3, color.RGBA{}},
		{"变换的用户空间渐变", `<linearGradient id="g" gradientUnits="userSpaceOnUse" x2="20"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient><rect width="20" height="20" transform="translate(20) rotate(90)" fill="url(#g)"/>`, 10, 1, color.RGBA{R: 236, B: 19, A: 255}},
		{"clip-rule 不代替 fill-rule", `<path fill="blue" clip-rule="evenodd" d="M1 1H19V19H1Z M5 5H15V15H5Z"/>`, 10, 10, color.RGBA{B: 255, A: 255}},
		{"无 stop 渐变透明", `<defs><linearGradient id="g"/></defs><rect width="20" height="20" fill="url(#g)"/>`, 10, 10, color.RGBA{}},
		{"独立 stop 不直接绘制", `<stop offset="0"/>`, 10, 10, color.RGBA{}},
		{"隐藏定义仍可引用", `<defs display="none"><linearGradient id="g"><stop offset="0" stop-color="blue"/></linearGradient></defs><rect width="20" height="20" fill="url(#g)"/>`, 10, 10, color.RGBA{B: 255, A: 255}},
		{"display 隐藏子树", `<g display="none"><rect width="20" height="20" fill="blue"/></g>`, 10, 10, color.RGBA{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			img := raster(t, `<svg viewBox="0 0 20 20">`+test.body+`</svg>`, 20)
			if got := img.RGBAAt(test.x, test.y); got != test.want {
				t.Fatalf("像素 (%d,%d)=%v，期望 %v", test.x, test.y, got, test.want)
			}
		})
	}
}

func TestAspectRatio(t *testing.T) {
	for _, test := range []struct {
		data string
		want float64
	}{
		{`<svg viewBox="10 20 40 20"/>`, 2},
		{`<svg width="2in" height="1in"/>`, 2},
		{`<svg width="30px" height="10"/>`, 3},
		{`<svg viewBox="0 0 10 20" width="100" height="100"/>`, .5},
	} {
		s, err := Parse(strings.NewReader(test.data))
		if err != nil {
			t.Fatal(err)
		}
		if got := s.AspectRatio(); got != test.want {
			t.Fatalf("宽高比=%v，期望 %v", got, test.want)
		}
		s.Image(32, 64, gui.Color{A: 1})
		if s.AspectRatio() != test.want || s.WithRawColors().AspectRatio() != test.want {
			t.Fatal("输出尺寸或颜色配置改变了文档比例")
		}
	}
	if (*Source)(nil).AspectRatio() != 0 || (&Source{}).AspectRatio() != 0 {
		t.Fatal("空 Source 的比例应为零")
	}
}

func TestDocumentAspectRatioModes(t *testing.T) {
	for _, test := range []struct {
		mode string
		y    int
		want color.RGBA
	}{
		{"xMidYMid meet", 1, color.RGBA{}},
		{"xMaxYMin meet", 1, color.RGBA{B: 255, A: 255}},
		{"xMaxYMin meet", 15, color.RGBA{}},
		{"none", 1, color.RGBA{B: 255, A: 255}},
		{"xMidYMid slice", 1, color.RGBA{B: 255, A: 255}},
	} {
		img := raster(t, `<svg viewBox="0 0 20 10" preserveAspectRatio="`+test.mode+`"><rect width="20" height="10" fill="blue"/></svg>`, 20)
		if got := img.RGBAAt(10, test.y); got != test.want {
			t.Fatalf("%s (%d,%d)=%v，期望 %v", test.mode, 10, test.y, got, test.want)
		}
	}
}

// 原始 resvg 素材保留许可与固定来源；直接验证已知内部颜色和轮廓外透明，
// 不通过另一条 oksvg 路径生成期望。128px 采样避开抗锯齿边界。
func TestRasterizerRegressions(t *testing.T) {
	for _, test := range []struct {
		file string
		x, y int
		want color.RGBA
	}{
		{"paint-servers/radialGradient/focal-point-correction.svg", 64, 64, color.RGBA{A: 255}},
		{"paint-servers/radialGradient/focal-point-correction.svg", 120, 120, color.RGBA{}},
		{"painting/stroke/line-as-curve-2.svg", 102, 7, color.RGBA{}},
		{"painting/stroke/line-as-curve-2.svg", 101, 13, color.RGBA{G: 128, A: 255}},
	} {
		data, err := os.ReadFile("testdata/resvg/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		source, err := Parse(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range []struct {
			name   string
			source *Source
			want   color.RGBA
		}{
			{"原色", source.WithRawColors(), test.want},
			{"着色", source, color.RGBA{G: uint8((int(test.want.A) + 1) / 2), A: uint8((int(test.want.A) + 1) / 2)}},
		} {
			img := mode.source.Image(128, 128, gui.Color{G: .5, A: .5})
			if got := color.RGBAModel.Convert(img.At(test.x, test.y)); got != mode.want {
				t.Fatalf("%s/%s (%d,%d)=%v，期望 %v", test.file, mode.name, test.x, test.y, got, mode.want)
			}
		}
	}
}

// 紧邻的两组 A/a 参数构成圆；预期来自解析圆盘，距边界 1.25px 内留给抗锯齿。
func TestRepeatedArcParameters(t *testing.T) {
	for _, path := range []string{
		"M10 4a6 6 0 1 1 0 12 6 6 0 0 1 0-12z",
		"M10 4A6 6 0 1 1 10 16 6 6 0 0 1 10 4z",
	} {
		source, err := Parse(strings.NewReader(`<svg viewBox="0 0 20 20"><path fill="red" d="` + path + `"/></svg>`))
		if err != nil {
			t.Fatal(err)
		}
		for _, size := range []int{40, 80} {
			img := source.WithRawColors().Image(size, size, gui.Color{})
			center, radius := float64(size)/2, float64(size)*.3
			for y := 0; y < size; y++ {
				for x := 0; x < size; x++ {
					distance := math.Hypot(float64(x)+.5-center, float64(y)+.5-center)
					if math.Abs(distance-radius) <= 1.25 {
						continue
					}
					want := color.RGBA{}
					if distance < radius {
						want = color.RGBA{R: 255, A: 255}
					}
					if got := color.RGBAModel.Convert(img.At(x, y)); got != want {
						t.Fatalf("%s size=%d (%d,%d)=%v，期望 %v", path, size, x, y, got, want)
					}
				}
			}
		}
	}
}
