package main

import (
	"errors"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
)

// 检查实际控件上传的位图尺寸，而不是只调用 SVG Source 验证其本身。
func TestWordmarkUsesPhysicalResolution(t *testing.T) {
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	defer d.stopActivities()
	view := newWordmark(d.wordmark)
	w := view.Mount(nil).(*wordmarkWidget)
	view.Update(nil, w)
	defer w.releaseImage()
	if got := w.Measure(layout.Unbounded()).Size; got != (geometry.Size{Width: 305, Height: 128}) {
		t.Fatalf("wordmark measured %v, want 305x128 DIP", got)
	}
	w.Arrange(geometry.Rect(7, 11, 305, 128))
	p := new(wordmarkTestPainter)
	for _, tc := range []struct {
		scale  float32
		pixels image.Point
	}{{1, image.Pt(305, 128)}, {1.25, image.Pt(382, 160)}, {1.5, image.Pt(458, 192)}, {2, image.Pt(610, 256)}} {
		p.scale = tc.scale
		previous := w.native
		w.Paint(p)
		if p.uploaded.Bounds().Size() != tc.pixels || p.rect != geometry.Rect(0, 0, 305, 128) {
			t.Fatalf("scale %g uploaded %v, drawn %v; want %v pixels at 305x128 DIP", tc.scale, p.uploaded.Bounds(), p.rect, tc.pixels)
		}
		// 透明外角及原色内部点，按预乘 RGBA8 精确比较，不采样抗锯齿边缘。
		for _, sample := range []struct {
			x, y int
			want color.RGBA
		}{{0, 0, color.RGBA{}}, {61, 32, color.RGBA{G: 173, B: 216, A: 255}}, {285, 64, color.RGBA{R: 91, G: 122, B: 143, A: 255}}} {
			x, y := int(float32(sample.x)*tc.scale), int(float32(sample.y)*tc.scale)
			if got := color.RGBAModel.Convert(p.uploaded.At(x, y)).(color.RGBA); got != sample.want {
				t.Fatalf("scale %g at (%d,%d): got %v, want original %v", tc.scale, x, y, got, sample.want)
			}
		}
		if previous != nil && !previous.(*wordmarkTestImage).destroyed {
			t.Fatal("scale change retained obsolete native image")
		}
		uploads, native := p.uploads, w.native
		view.Update(nil, w)
		w.Paint(p)
		if p.uploads != uploads || w.native != native {
			t.Fatal("unchanged scale or declarative update replaced native image")
		}
	}
	info := w.Snapshot()
	if info.Role != gui.RoleImage || info.Text != "GOUI" || info.Bounds != w.Rect() || len(info.Actions) != 0 {
		t.Fatalf("unexpected wordmark semantics: %+v", info)
	}
}

func TestWordmarkFitsNarrowAllocation(t *testing.T) {
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	defer d.stopActivities()
	w := &wordmarkWidget{source: d.wordmark}
	defer w.releaseImage()
	w.Arrange(geometry.Rect(0, 0, 152.5, 128))
	p := &wordmarkTestPainter{scale: 1.5}
	w.Paint(p)
	if p.rect != geometry.Rect(0, 0, 152.5, 64) || p.uploaded.Bounds().Size() != image.Pt(229, 96) {
		t.Fatalf("narrow wordmark stretched or overflowed: rect %v, pixels %v", p.rect, p.uploaded.Bounds())
	}
	for _, scale := range []float32{0, -1, float32(math.NaN()), float32(math.Inf(1)), 100} {
		p.scale = scale
		uploads, draws := p.uploads, p.draws
		w.Paint(p)
		if p.uploads != uploads || p.draws != draws {
			t.Fatalf("invalid scale %g submitted an image", scale)
		}
	}
	p.scale = 1.5
	w.SetVisible(false)
	draws := p.draws
	w.Paint(p)
	if p.draws != draws || w.Measure(layout.Unbounded()).Size != (geometry.Size{}) {
		t.Fatal("hidden wordmark was drawn or measured")
	}
}

func TestWordmarkResourceLifetimeAndRetry(t *testing.T) {
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	defer d.stopActivities()
	view := newWordmark(d.wordmark)
	w := view.Mount(nil).(*wordmarkWidget)
	view.Update(nil, w)
	w.Arrange(geometry.Rect(0, 0, 305, 128))
	p := &wordmarkTestPainter{scale: 1.5, uploadErr: errors.New("upload failed")}
	w.Paint(p)
	if p.draws != 0 || w.native != nil {
		t.Fatal("failed upload drew invalid resources")
	}
	p.uploadErr = nil
	w.Paint(p)
	if p.draws != 1 || p.uploads != 2 {
		t.Fatal("native image creation did not retry")
	}
	first := w.native.(*wordmarkTestImage)
	other := &wordmarkWidget{source: d.wordmark}
	other.Arrange(w.Rect())
	other.Paint(p)
	defer other.releaseImage()
	if other.native == w.native {
		t.Fatal("widgets shared mutable native resources")
	}
	newWordmark(d.wordmark.WithRawColors()).Update(nil, w)
	if !first.destroyed || other.native.(*wordmarkTestImage).destroyed {
		t.Fatal("source replacement leaked or destroyed another widget's resource")
	}
	w.Paint(p)
	last := w.native.(*wordmarkTestImage)
	w.releaseImage()
	w.releaseImage()
	if !last.destroyed || w.native != nil {
		t.Fatal("release did not clear the native image")
	}
	newWordmark(nil).Update(nil, w)
	draws := p.draws
	w.Paint(p)
	if p.draws != draws || w.Measure(layout.Unbounded()).Size != (geometry.Size{}) {
		t.Fatal("removed source was drawn or measured")
	}
}

type wordmarkTestPainter struct {
	gui.Painter
	scale          float32
	uploaded       image.Image
	uploads, draws int
	uploadErr      error
	rect           geometry.Rectangle
}

func (p *wordmarkTestPainter) PixelScale() float32 { return p.scale }
func (p *wordmarkTestPainter) NewImage(img image.Image) (graphics.Image, error) {
	p.uploads++
	p.uploaded = img
	if p.uploadErr != nil {
		return nil, p.uploadErr
	}
	return &wordmarkTestImage{size: img.Bounds().Size()}, nil
}
func (p *wordmarkTestPainter) DrawImage(rect geometry.Rectangle, _ graphics.Image) {
	p.draws++
	p.rect = rect
}

type wordmarkTestImage struct {
	size      image.Point
	destroyed bool
}

func (i *wordmarkTestImage) Size() (int, int)         { return i.size.X, i.size.Y }
func (i *wordmarkTestImage) Update(image.Image) error { return nil }
func (i *wordmarkTestImage) Destroy()                 { i.destroyed = true }
