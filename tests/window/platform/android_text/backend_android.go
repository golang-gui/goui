package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	ffi "github.com/goexlib/cgo"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/platform/android/libs/jni"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/androidtext"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/goregular"
)

type backendCheck struct {
	Name   string
	Pass   bool
	Detail string
}
type backendResult struct {
	Checks              []backendCheck
	Failures            int
	CachedMetricsNS     int64
	CachedMetricsAllocs float64
}

func (r *backendResult) check(name string, pass bool, detail any) {
	r.Checks = append(r.Checks, backendCheck{name, pass, fmt.Sprint(detail)})
	if !pass {
		r.Failures++
	}
}
func (r *backendResult) run(name string, fn func()) {
	defer func() {
		if err := recover(); err != nil {
			r.check(name+":panic", false, err)
		}
	}()
	fn()
}

func verifyBackend(directory string) backendResult {
	var result backendResult
	result.run("jni", func() {
		vm := jni.VM(ffi.CallRet[uintptr](symbol("goui_android_java_vm")))
		e, status := vm.GetEnv()
		if status != 0 {
			panic(status)
		}
		if e.PushLocalFrame(16) < 0 {
			panic(e.Exception())
		}
		defer e.PopLocalFrame()
		text := "a\x00😀"
		value := e.NewString(text)
		result.check("jni-utf16-roundtrip", e.String(value) == text, nil)
		array := e.IntArray([]int32{-1, 0, 42, math.MaxInt32})
		result.check("jni-int-array", reflect.DeepEqual(e.Ints(array), []int32{-1, 0, 42, math.MaxInt32}), nil)
		class := e.ObjectClass(value)
		method := e.Method(class, "gouiMissingMethod", "()V", false)
		err := e.Exception()
		result.check("jni-exception-converted", method == 0 && err != nil && !e.ExceptionCheck(), err)
		after := e.NewString("after exception")
		result.check("jni-after-exception", e.String(after) == "after exception" && e.Exception() == nil, nil)
	})
	result.run("create", func() {
		ctx, err := androidtext.NewContext()
		if err != nil {
			panic(err)
		}
		defer ctx.Destroy()
		for name, data := range map[string][]byte{"regular": goregular.TTF, "bold": gobold.TTF, "italic": goitalic.TTF} {
			path := filepath.Join(directory, name+".ttf")
			if err := os.WriteFile(path, data, 0600); err != nil {
				panic(err)
			}
			if err := ctx.AddFont(path); err != nil {
				panic(err)
			}
			result.check("font-duplicate:"+name, ctx.AddFont(path) == nil, path)
		}
		format := typography.TextFormat{Font: typography.FontInfo{Family: "Go", Size: 18}, TextColor: color.Black, WrapMode: typography.WrapWordChar}
		create := func(text string, width float32) typography.TextLayout {
			t, err := ctx.NewTextLayout(text, format, width, 300)
			if err != nil {
				panic(err)
			}
			return t
		}
		result.run("native-reference", func() {
			if ffi.CallRet[int32](symbol("probe_reference")) != 0 {
				panic("独立原生参考生成失败")
			}
			var references []struct {
				Text   string
				Lines  []struct{ Y, Height, Baseline, Width float32 }
				Carets []struct {
					Offset, Line int
					X            float32
				}
				Rasters []struct {
					Scale float32
					File  string
				}
			}
			data, err := os.ReadFile(filepath.Join(directory, "backend-reference.json"))
			if err != nil {
				panic(err)
			}
			if err = json.Unmarshal(data, &references); err != nil {
				panic(err)
			}
			result.check("native-reference-cases", len(references) == 4, len(references))
			for _, reference := range references {
				t := create(reference.Text, 220)
				lines, clusters := t.MeasureMetrics()
				g := textedit.NewGeometry(lines, clusters, len(reference.Text), 1)
				result.check("native-line-count", len(lines) == len(reference.Lines), reference.Text)
				for i, want := range reference.Lines {
					if i < len(lines) {
						got := lines[i]
						result.check("native-line-geometry", closeX(got.Y, want.Y) && closeX(got.Height, want.Height) && closeX(got.Baseline, want.Baseline) && closeX(got.Width, want.Width), reference.Text)
					}
				}
				for _, want := range reference.Carets {
					matched := false
					for _, upstream := range []bool{false, true} {
						pos := textedit.Position{Offset: want.Offset, Upstream: upstream}
						matched = matched || (g.LineIndex(pos) == want.Line && closeX(g.Caret(pos).X, want.X))
					}
					result.check("native-caret", matched, fmt.Sprintf("%s %+v", reference.Text, want))
				}
				for _, raster := range reference.Rasters {
					want, err := os.ReadFile(filepath.Join(directory, raster.File))
					if err != nil {
						panic(err)
					}
					got, err := t.Rasterize(raster.Scale, image.Rect(0, 0, 384, 192), nil)
					if err != nil {
						panic(err)
					}
					result.check("native-raster-exact", bytes.Equal(got.Pixels, want), fmt.Sprintf("%s %g", reference.Text, raster.Scale))
				}
				t.Destroy()
			}
		})
		result.run("metrics", func() {
			for _, text := range []string{"", "A\nB\n", "ABC אבג DEF", "a\u0301😀👩‍💻🇨🇳", "a\u200b\u200bB", "ffi office", "किक्ष", "العربية"} {
				t := create(text, 140.375)
				lines, clusters := t.MeasureMetrics()
				result.check("line-present:"+text, len(lines) > 0, lines)
				covered := 0
				for _, c := range clusters {
					result.check("range:"+text, c.Start >= covered && c.Length > 0 && c.Start+c.Length <= len(text), c)
					covered = c.Start + c.Length
				}
				if text == "a\u0301😀👩‍💻🇨🇳" {
					result.check("atomic-metrics", len(clusters) == 4, clusters)
				}
				if text == "a\u200b\u200bB" {
					result.check("zero-width-ranges", len(clusters) == 4 && clusters[1].Length == 3 && clusters[1].Width == 0 && clusters[2].Length == 3, clusters)
				}
				if text == "A\nB\n" {
					result.check("hard-newlines", len(lines) == 3 && lines[0].Length == 2 && lines[2].Start == 4, lines)
				}
				t.Destroy()
			}
		})
		result.run("format", func() {
			t := create("Hello ffi", 500)
			defer t.Destroy()
			region := image.Rect(0, 0, 500, 120)
			original, err := t.Rasterize(2, region, nil)
			if err != nil {
				panic(err)
			}
			for _, font := range []typography.FontInfo{{Family: "Go", Size: 18, Weight: typography.FontWeightBold}, {Family: "Go", Size: 18, Style: typography.FontStyleItalic}} {
				f := format
				f.Font = font
				fresh, err := ctx.NewTextLayout(t.Text(), f, 500, 120)
				if err != nil {
					panic(err)
				}
				want, err := fresh.Rasterize(2, region, nil)
				fresh.Destroy()
				if err != nil {
					panic(err)
				}
				t.SetTextFont(0, len(t.Text()), font)
				got, err := t.Rasterize(2, region, nil)
				if err != nil {
					panic(err)
				}
				result.check("range-font-matches-base", bytes.Equal(got.Pixels, want.Pixels) && !bytes.Equal(got.Pixels, original.Pixels), font)
				t.SetTextFont(0, len(t.Text()), format.Font)
			}
			beforeLines, beforeClusters := t.MeasureMetrics()
			t.SetTextColor(0, len(t.Text()), color.NRGBA{R: 255, A: 128})
			bmp, err := t.Rasterize(2, region, nil)
			if err != nil {
				panic(err)
			}
			seen := false
			valid := true
			for i := 0; i < len(bmp.Pixels); i += 4 {
				a := bmp.Pixels[i+3]
				seen = seen || a > 0
				valid = valid && a <= 129 && bmp.Pixels[i] == a && bmp.Pixels[i+1] == 0 && bmp.Pixels[i+2] == 0
			}
			result.check("premultiplied-foreground", seen && valid, nil)
			// 装饰线可与字形重叠并按 source-over 增加 alpha；不能用单次覆盖的
			// alpha 上限检验重复合成。先独立验证纯文字，再验证装饰及度量。
			t.SetUnderline(0, len(t.Text()), true)
			t.SetStrikethrough(0, len(t.Text()), true)
			afterLines, afterClusters := t.MeasureMetrics()
			result.check("appearance-keeps-metrics", reflect.DeepEqual(beforeLines, afterLines) && reflect.DeepEqual(beforeClusters, afterClusters), nil)
			t.SetUnderline(0, len(t.Text()), false)
			t.SetStrikethrough(0, len(t.Text()), false)
			t.SetTextColor(0, len(t.Text()), color.Black)
			restored, err := t.Rasterize(2, region, nil)
			if err != nil {
				panic(err)
			}
			result.check("format-restored", bytes.Equal(original.Pixels, restored.Pixels), nil)
		})
		result.run("tabs", func() {
			t := create("\tX\t\tY", 900)
			defer t.Destroy()
			changes := 0
			t.ConnectChanged(func() { changes++ })
			t.SetTabWidth(80)
			t.SetTabWidth(80)
			_, cs := t.MeasureMetrics()
			result.check("tab-stops", len(cs) == 5 && math.Abs(float64(cs[1].X-80)) < .02 && math.Abs(float64(cs[4].X-240)) < .02, cs)
			result.check("tab-notification", changes == 1, changes)
			t.SetTabWidth(40.25)
			_, cs = t.MeasureMetrics()
			result.check("fractional-tab-stops", len(cs) == 5 && math.Abs(float64(cs[1].X-40.25)) < .02 && math.Abs(float64(cs[4].X-120.75)) < .02, cs)
			for _, align := range []typography.TextAlignment{typography.TextAlignCenter, typography.TextAlignEnd} {
				t.SetTextAlignment(align)
				lines, aligned := t.MeasureMetrics()
				result.check("aligned-tab-stops", len(aligned) == 5 && math.Abs(float64(aligned[1].X-lines[0].X-40.25)) < .02 && math.Abs(float64(aligned[4].X-lines[0].X-120.75)) < .02, aligned)
			}
		})
		result.run("native-decorations", func() {
			t := create("M   M", 220)
			defer t.Destroy()
			t.SetTextColor(0, 5, color.NRGBA{R: 255, A: 128})
			for _, kind := range []string{"underline", "strike"} {
				if kind == "underline" {
					t.SetUnderline(0, 5, true)
				} else {
					t.SetStrikethrough(0, 5, true)
				}
				want, err := os.ReadFile(filepath.Join(directory, "reference-"+kind+".rgba"))
				if err != nil {
					panic(err)
				}
				got, err := t.Rasterize(1.5, image.Rect(0, 0, 330, 96), nil)
				if err != nil {
					panic(err)
				}
				result.check("native-decoration:"+kind, bytes.Equal(got.Pixels, want), nil)
				if kind == "underline" {
					t.SetUnderline(0, 5, false)
				} else {
					t.SetStrikethrough(0, 5, false)
				}
			}
		})
		result.run("cross-style-grapheme", func() {
			t := create("a\u0301b", 200)
			defer t.Destroy()
			region := image.Rect(0, 0, 200, 80)
			original, err := t.Rasterize(1, region, nil)
			if err != nil {
				panic(err)
			}
			t.SetTextFont(1, 2, typography.FontInfo{Family: "Go", Size: 24})
			t.SetTextColor(1, 2, color.NRGBA{B: 255, A: 255})
			_, cs := t.MeasureMetrics()
			result.check("style-does-not-split-grapheme", len(cs) == 2 && cs[0].Start == 0 && cs[0].Length == 3, cs)
			t.SetTextFont(0, len(t.Text()), format.Font)
			t.SetTextColor(0, len(t.Text()), format.TextColor)
			restored, err := t.Rasterize(1, region, nil)
			if err != nil {
				panic(err)
			}
			result.check("overlapping-style-replacement", bytes.Equal(original.Pixels, restored.Pixels), nil)
		})
		result.run("placeholder", func() {
			for _, text := range []string{"\uFFFC", "\uFFFC\uFFFC", "A\uFFFCB", "אב\uFFFCגד"} {
				t := create(text, 80)
				for i, r := range text {
					if r == '\uFFFC' {
						t.SetPlaceholder(i, &typography.PlaceholderMetrics{Width: 37.25, Height: 42.5, Baseline: 28.25})
					}
				}
				lines, cs := t.MeasureMetrics()
				for _, c := range cs {
					if text[c.Start:c.Start+c.Length] == "\uFFFC" {
						line := lines[c.LineIndex]
						result.check("placeholder-geometry", c.Length == 3 && c.Width == 37.25 && c.Height == 42.5 && c.Y+28.25 == line.Baseline && line.Height >= 42.5, c)
					}
				}
				if text == "\uFFFC" || text == "\uFFFC\uFFFC" {
					bmp, err := t.Rasterize(1.5, image.Rect(-5, -5, 120, 80), nil)
					if err != nil {
						panic(err)
					}
					empty := true
					for _, v := range bmp.Pixels {
						empty = empty && v == 0
					}
					result.check("placeholder-no-glyph", empty, text)
				}
				t.Destroy()
			}
		})
		result.run("raster", func() {
			t := create("office a\u0301\nABC אבג", 260)
			defer t.Destroy()
			beforeLines, beforeClusters := t.MeasureMetrics()
			for _, scale := range []float32{1, 1.5, 2} {
				full, err := t.Rasterize(scale, image.Rect(0, 0, 600, 300), nil)
				if err != nil {
					panic(err)
				}
				for _, region := range []image.Rectangle{image.Rect(10, 5, 180, 70), image.Rect(-7, -11, 210, 90), image.Rect(610, 0, 650, 60)} {
					buf := make([]byte, region.Dx()*region.Dy()*4)
					part, err := t.Rasterize(scale, region, buf)
					if err != nil {
						panic(err)
					}
					equal := true
					for y := 0; y < part.Height; y++ {
						for x := 0; x < part.Width; x++ {
							sx, sy := region.Min.X+x, region.Min.Y+y
							var expected [4]byte
							if sx >= 0 && sy >= 0 && sx < full.Width && sy < full.Height {
								copy(expected[:], full.Pixels[sy*full.Stride+sx*4:])
							}
							equal = equal && bytes.Equal(expected[:], part.Pixels[y*part.Stride+x*4:y*part.Stride+x*4+4])
						}
					}
					result.check(fmt.Sprintf("raster-region:%g:%v", scale, region), equal, nil)
					result.check("raster-reuses-buffer", &part.Pixels[0] == &buf[0], nil)
				}
			}
			afterLines, afterClusters := t.MeasureMetrics()
			result.check("raster-keeps-metrics", reflect.DeepEqual(beforeLines, afterLines) && reflect.DeepEqual(beforeClusters, afterClusters), nil)
			result.CachedMetricsAllocs = testing.AllocsPerRun(100, func() { t.MeasureMetrics() })
			start := time.Now()
			for i := 0; i < 10000; i++ {
				t.MeasureMetrics()
			}
			result.CachedMetricsNS = time.Since(start).Nanoseconds() / 10000
			result.check("cached-metrics-bounded-allocs", result.CachedMetricsAllocs <= 2, result.CachedMetricsAllocs)
			copyLines, copyClusters := t.MeasureMetrics()
			oldX := copyClusters[0].X
			copyClusters[0].X += 100
			copyLines[0].X += 100
			freshLines, freshClusters := t.MeasureMetrics()
			result.check("caller-metrics-isolated", freshClusters[0].X == oldX && freshLines[0].X == beforeLines[0].X, nil)
		})
		result.run("lifetime", func() {
			t := create("peer", 200)
			peer := create("peer", 200)
			defer peer.Destroy()
			region := image.Rect(0, 0, 200, 80)
			original, _ := peer.Rasterize(1, region, nil)
			destroyed := 0
			t.ConnectDestroy(func() { destroyed++ })
			t.SetTextColor(0, 4, color.NRGBA{R: 255, A: 255})
			t.SetTextFont(0, 4, typography.FontInfo{Size: 30})
			t.Destroy()
			t.Destroy()
			after, _ := peer.Rasterize(1, region, nil)
			result.check("independent-peer", bytes.Equal(original.Pixels, after.Pixels) && destroyed == 1, destroyed)
			_, err := ctx.NewTextLayout("char", typography.TextFormat{WrapMode: typography.WrapChar}, 100, 100)
			result.check("wrapchar-explicit-unsupported", err != nil, err)
		})
		result.run("alignment", func() {
			for _, text := range []string{"ABC", "אבג", "a very long line without automatic wrapping"} {
				for _, align := range []typography.TextAlignment{typography.TextAlignBegin, typography.TextAlignEnd, typography.TextAlignCenter} {
					f := format
					f.WrapMode = typography.WrapNone
					f.TextAlign = align
					t, err := ctx.NewTextLayout(text, f, 100.375, 300)
					if err != nil {
						panic(err)
					}
					lines, _ := t.MeasureMetrics()
					line := lines[0]
					want := float32(0)
					rtl := text == "אבג"
					if align == typography.TextAlignCenter {
						want = (100.375 - line.Width) / 2
					} else if (align == typography.TextAlignBegin && rtl) || (align == typography.TextAlignEnd && !rtl) {
						want = 100.375 - line.Width
					}
					result.check("fractional-alignment", len(lines) == 1 && math.Abs(float64(line.X-want)) < .02, fmt.Sprintf("%s %d got=%g want=%g", text, align, line.X, want))
					t.Destroy()
				}
			}
			f := format
			f.TextAlign = typography.TextAlignFill
			t, err := ctx.NewTextLayout("one two three four five six", f, 160, 300)
			if err != nil {
				panic(err)
			}
			defer t.Destroy()
			lines, cs := t.MeasureMetrics()
			right := float32(0)
			for _, c := range cs {
				if c.LineIndex == 0 && t.Text()[c.Start:c.Start+c.Length] != " " {
					right = max(right, c.X+c.Width)
				}
			}
			result.check("native-justification", len(lines) > 1 && math.Abs(float64(right-160)) < .02, fmt.Sprintf("right=%g lines=%v", right, lines))
			want, err := os.ReadFile(filepath.Join(directory, "reference-fill.rgba"))
			if err != nil {
				panic(err)
			}
			got, err := t.Rasterize(1.5, image.Rect(0, 0, 240, 42), nil)
			if err != nil {
				panic(err)
			}
			result.check("justification-native-pixels", bytes.Equal(got.Pixels, want), nil)
		})
		result.run("font-collection", func() {
			fonts := [][]byte{goregular.TTF, gobold.TTF, goitalic.TTF}
			data := make([]byte, 12+len(fonts)*4)
			copy(data, "ttcf")
			binary.BigEndian.PutUint32(data[4:8], 0x10000)
			binary.BigEndian.PutUint32(data[8:12], uint32(len(fonts)))
			for i, font := range fonts {
				for len(data)%4 != 0 {
					data = append(data, 0)
				}
				base := len(data)
				binary.BigEndian.PutUint32(data[12+i*4:16+i*4], uint32(base))
				data = append(data, font...)
				for j := 0; j < int(binary.BigEndian.Uint16(font[4:6])); j++ {
					at := base + 12 + j*16 + 8
					binary.BigEndian.PutUint32(data[at:at+4], binary.BigEndian.Uint32(data[at:at+4])+uint32(base))
				}
			}
			path := filepath.Join(directory, "Go.ttc")
			if err := os.WriteFile(path, data, 0600); err != nil {
				panic(err)
			}
			collection, err := androidtext.NewContext()
			if err != nil {
				panic(err)
			}
			defer collection.Destroy()
			if err = collection.AddFont(path); err != nil {
				panic(err)
			}
			for _, font := range []typography.FontInfo{format.Font, {Family: "Go", Size: 18, Weight: typography.FontWeightBold}, {Family: "Go", Size: 18, Style: typography.FontStyleItalic}} {
				f := format
				f.Font = font
				expected, err := ctx.NewTextLayout("Hello", f, 200, 80)
				if err != nil {
					panic(err)
				}
				actual, err := collection.NewTextLayout("Hello", f, 200, 80)
				if err != nil {
					panic(err)
				}
				a, err := expected.Rasterize(1.5, image.Rect(0, 0, 300, 120), nil)
				if err != nil {
					panic(err)
				}
				b, err := actual.Rasterize(1.5, image.Rect(0, 0, 300, 120), nil)
				if err != nil {
					panic(err)
				}
				result.check("ttc-face-matches-ttf", bytes.Equal(a.Pixels, b.Pixels), font)
				expected.Destroy()
				actual.Destroy()
			}
		})
	})
	return result
}
