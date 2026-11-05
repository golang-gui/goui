//go:build android || ios

package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	ffi "github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

func verifyGraphics(p graphics.Painter, ctx typography.Context, gpu bool) {
	pixels := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			pixels.SetRGBA(x, y, color.RGBA{B: 255, A: 255})
		}
	}
	texture, err := p.NewImage(pixels)
	must(err)
	defer texture.Destroy()
	text, err := ctx.NewTextLayout("Go", typography.TextFormat{Font: typography.FontInfo{Family: "Go", Size: 12}, TextColor: color.Black}, 80, 26)
	must(err)
	defer text.Destroy()
	for _, scale := range []float32{1, 2} {
		result, err := p.RenderImage(int(128*scale), int(96*scale), scale, func() {
			if gpu {
				get := nativeGLSymbol("glGetString")
				version := ffi.GoString(ffi.Pointer(ffi.CallRet[uintptr](get, uint32(0x1F02))))
				expect("实际 OpenGL ES 3 上下文", strings.HasPrefix(version, "OpenGL ES 3"))
			}
			p.Clear(graphics.RGB(255, 255, 255))
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					pixels.SetRGBA(x, y, color.RGBA{R: 32, G: 160, B: 96, A: 255})
				}
			}
			must(texture.Update(pixels))
			p.DrawImage(graphics.Rect(8, 8, 16, 16), texture)
			p.FillRect(graphics.Rect(40, 8, 16, 16), graphics.ColorOf(color.NRGBA{R: 255, A: 128}))
			p.SetTransform(geometry.Translate(72, 8))
			p.FillRect(graphics.Rect(0, 0, 16, 16), graphics.RGB(80, 96, 112))
			p.SetTransform(geometry.Identity())
			p.SetClipRect(graphics.Rect(8, 40, 16, 16))
			p.FillRect(graphics.Rect(0, 32, 48, 32), graphics.RGB(224, 96, 48))
			p.SetClipRect(graphics.Rectangle{})
			p.ClearPath(graphics.MoveTo(72, 40).LineTo(88, 40).LineTo(88, 56).LineTo(72, 56).Close())
			p.DrawTextLayout(graphics.Point{X: 8, Y: 68}, text)
		})
		must(err)
		for _, sample := range []struct {
			x, y int
			want color.RGBA
			name string
		}{
			{16, 16, color.RGBA{32, 160, 96, 255}, "Image.Update 后绘制"},
			{48, 16, color.RGBA{255, 127, 127, 255}, "半透明合成"},
			{80, 16, color.RGBA{80, 96, 112, 255}, "Transform"},
			{16, 48, color.RGBA{224, 96, 48, 255}, "clip 内部"},
			{28, 48, color.RGBA{255, 255, 255, 255}, "clip 外部"},
			{80, 48, color.RGBA{}, "ClearPath 擦除"},
		} {
			actual := color.RGBAModel.Convert(result.At(int(float32(sample.x)*scale), int(float32(sample.y)*scale))).(color.RGBA)
			ok := math.Abs(float64(actual.R)-float64(sample.want.R)) <= 1 && math.Abs(float64(actual.G)-float64(sample.want.G)) <= 1 && math.Abs(float64(actual.B)-float64(sample.want.B)) <= 1 && math.Abs(float64(actual.A)-float64(sample.want.A)) <= 1
			if !ok {
				panic(fmt.Sprintf("%s %gx 像素=%v，预期=%v", sample.name, scale, actual, sample.want))
			}
			expect(fmt.Sprintf("%s %gx", sample.name, scale), ok)
		}
		ink := 0
		for y := int(68 * scale); y < int(94*scale); y++ {
			for x := int(8 * scale); x < int(50*scale); x++ {
				r, g, b, _ := result.At(x, y).RGBA()
				if r < 30000 && g < 30000 && b < 30000 {
					ink++
				}
			}
		}
		expect(fmt.Sprintf("文字上传绘制 %gx", scale), ink > 10)
	}
}
