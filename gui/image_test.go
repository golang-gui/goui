package gui

import (
	"errors"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
)

func TestImageSnapshot(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 8))
	view := NewImage(img)
	view.SetID("logo")
	view.Arrange(geometry.Rect(1, 2, 30, 40))

	info := view.Snapshot()
	if info.ID != "logo" {
		t.Fatalf("unexpected snapshot id: %q", info.ID)
	}
	if info.Role != RoleImage {
		t.Fatalf("unexpected snapshot role: %q", info.Role)
	}
	if info.Bounds != geometry.Rect(1, 2, 30, 40) {
		t.Fatalf("unexpected snapshot bounds: %+v", info.Bounds)
	}
}

func TestImageMeasureUsesNaturalSize(t *testing.T) {
	img := image.NewRGBA(image.Rect(2, 3, 18, 12))
	view := NewImage(img)

	size := view.Measure(layout.Loose(geometry.Size{Width: 100, Height: 100}))

	if size.Size != (geometry.Size{Width: 16, Height: 9}) {
		t.Fatalf("unexpected measured size: %+v", size)
	}
}

func TestImageMeasureSkipsNilAndHiddenImage(t *testing.T) {
	view := NewImage(nil)
	if size := view.Measure(layout.Loose(geometry.Size{Width: 100, Height: 100})); size.Size != (geometry.Size{}) {
		t.Fatalf("nil image measured non-zero size: %+v", size)
	}

	view.SetImage(image.NewRGBA(image.Rect(0, 0, 16, 8)))
	view.SetVisible(false)
	if size := view.Measure(layout.Loose(geometry.Size{Width: 100, Height: 100})); size.Size != (geometry.Size{}) {
		t.Fatalf("hidden image measured non-zero size: %+v", size)
	}
}

func TestImageSetImageRequestsLayout(t *testing.T) {
	win := &window{}
	view := NewImage(nil)
	win.SetWidget(view)

	win.layoutDirty = false
	win.paintDirty = false
	view.SetImage(nil)
	if win.layoutDirty || win.paintDirty {
		t.Fatal("setting nil image to nil should not request layout")
	}

	img := image.NewRGBA(image.Rect(0, 0, 32, 16))
	view.SetImage(img)
	if view.Image() != img {
		t.Fatal("image was not updated")
	}
	if !win.layoutDirty || !win.paintDirty {
		t.Fatal("setting image did not request layout and paint")
	}
}

func TestImagePaintDrawsImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 8))
	view := NewImage(img)
	view.Arrange(geometry.Rect(10, 20, 80, 30))

	painter := new(testImagePainter)
	view.Paint(painter)

	if painter.drawImages != 1 {
		t.Fatalf("expected one image draw, got %d", painter.drawImages)
	}
	if painter.imageRect != geometry.Rect(0, 0, 16, 8) {
		t.Fatalf("unexpected image rect: %+v", painter.imageRect)
	}
	if painter.source != img || painter.image == nil {
		t.Fatal("painter did not create and draw the image resource")
	}
}

// 显式内容尺寸同时用于测量和绘制；父约束只改变分配，不隐含缩放或适配。
func TestImageContentSize(t *testing.T) {
	view := NewImage(image.NewRGBA(image.Rect(2, 3, 602, 288)))
	view.SetContentSize(geometry.Size{Width: 400, Height: 190})
	if got := view.Measure(layout.Unbounded()).Size; got != (geometry.Size{Width: 400, Height: 190}) {
		t.Fatalf("logical content measured %v, want 400x190 DIP", got)
	}
	view.SetMaxSize(geometry.Size{Width: 100, Height: 80})
	if got := view.Measure(layout.Tight(geometry.Size{Width: 90, Height: 70})).Size; got != (geometry.Size{Width: 90, Height: 70}) {
		t.Fatalf("parent constraint lost: %v", got)
	}
	view.Arrange(geometry.Rect(10, 20, 90, 70))
	p := new(testImagePainter)
	view.Paint(p)
	if p.imageRect != geometry.Rect(0, 0, 400, 190) {
		t.Fatalf("drawn content %v, want explicit 400x190 DIP", p.imageRect)
	}
	if w, h := p.image.Size(); w != 600 || h != 285 {
		t.Fatalf("source resolution changed to %dx%d", w, h)
	}
	info := view.Snapshot()
	if info.Role != RoleImage || info.Bounds != geometry.Rect(10, 20, 90, 70) || len(info.Actions) != 0 {
		t.Fatalf("unexpected image semantics: %+v", info)
	}
	view.SetImage(nil)
	if got := view.Measure(layout.Unbounded()).Size; got != (geometry.Size{}) {
		t.Fatalf("nil image measured %v", got)
	}
	view.SetImage(image.NewRGBA(image.Rect(0, 0, 600, 285)))
	view.SetVisible(false)
	if got := view.Measure(layout.Unbounded()).Size; got != (geometry.Size{}) {
		t.Fatalf("hidden image measured %v", got)
	}
}

func TestImageContentSizeInvalidationAndResourceReuse(t *testing.T) {
	win := &window{}
	view := NewImage(image.NewRGBA(image.Rect(0, 0, 600, 285)))
	win.SetWidget(view)
	defer win.SetWidget(nil)
	p := new(testImagePainter)
	view.Paint(p)
	native := p.image.(*testNativeImage)
	win.layoutDirty, win.paintDirty = false, false
	size := geometry.Size{Width: 400, Height: 190}
	view.SetContentSize(size)
	if view.ContentSize() != size || !win.layoutDirty || !win.paintDirty {
		t.Fatal("content size did not invalidate layout and paint")
	}
	view.Paint(p)
	if native.destroyed || p.newImages != 1 || p.image != native {
		t.Fatal("changing only logical size replaced pixel resources")
	}
	win.layoutDirty, win.paintDirty = false, false
	view.SetContentSize(size)
	if win.layoutDirty || win.paintDirty {
		t.Fatal("unchanged content size invalidated layout")
	}
	view.SetImage(image.NewRGBA(image.Rect(0, 0, 1200, 570)))
	if view.ContentSize() != size || !native.destroyed {
		t.Fatal("source replacement lost logical size or retained old resources")
	}
	view.SetContentSize(geometry.Size{})
	if got := view.Measure(layout.Unbounded()).Size; got != (geometry.Size{Width: 1200, Height: 570}) {
		t.Fatalf("reset did not restore pixel dimensions: %v", got)
	}
}

func TestImageInvalidContentSizeRestoresDefault(t *testing.T) {
	for _, invalid := range []geometry.Size{
		{}, {Width: 40}, {Height: 19}, {Width: -1, Height: 19},
		{Width: 40, Height: -1}, {Width: float32(math.NaN()), Height: 19},
		{Width: 40, Height: float32(math.Inf(1))}, {Width: float32(math.Inf(-1)), Height: 19},
	} {
		view := NewImage(image.NewRGBA(image.Rect(0, 0, 60, 30)))
		view.SetContentSize(geometry.Size{Width: 40, Height: 19})
		view.SetContentSize(invalid)
		if view.ContentSize() != (geometry.Size{}) || view.Measure(layout.Unbounded()).Size != (geometry.Size{Width: 60, Height: 30}) {
			t.Fatalf("invalid size %v did not restore source dimensions", invalid)
		}
	}
}

// 同一 24x12 DIP 控件在不同缩放下截图并按原逻辑尺寸预览。
// 在不透明红、蓝区域内部采样预乘 RGBA8，精确比较，不涉及字体和抗锯齿边缘。
func TestImageContentSizePreservesRenderWidgetPreview(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		p, err := software.NewPainter(&widgetRenderSurface{})
		if err != nil {
			t.Fatal(err)
		}
		win := &window{rootBase: rootBase{painter: p, width: 100, pixelWidth: 100 * scale}}
		source := newPainterTestWidget(func(p Painter) {
			p.FillRect(geometry.Rect(0, 0, 24, 12), graphics.RGB(255, 0, 0))
			p.FillRect(geometry.Rect(12, 0, 12, 12), graphics.RGB(0, 0, 255))
		})
		win.SetWidget(source)
		source.Arrange(geometry.Rect(4, 6, 24, 12))
		captured, err := RenderWidget(source, 0)
		if err != nil {
			t.Fatal(err)
		}
		wantBounds := image.Rect(0, 0, int(24*scale), int(12*scale))
		if captured.Bounds() != wantBounds {
			t.Fatalf("scale %g captured %v, want %v", scale, captured.Bounds(), wantBounds)
		}
		preview := NewImage(captured)
		preview.SetContentSize(source.Rect().Size)
		win.SetWidget(preview)
		preview.Arrange(geometry.Rect(0, 0, 24, 12))
		shown, err := RenderWidget(preview, 0)
		if err != nil {
			t.Fatal(err)
		}
		if shown.Bounds() != wantBounds {
			t.Fatalf("scale %g preview bounds %v, want %v", scale, shown.Bounds(), wantBounds)
		}
		for _, sample := range []struct {
			x    int
			want color.RGBA
		}{{6, color.RGBA{R: 255, A: 255}}, {14, color.RGBA{B: 255, A: 255}}} {
			if got := color.RGBAModel.Convert(shown.At(int(float32(sample.x)*scale), int(6*scale))).(color.RGBA); got != sample.want {
				t.Fatalf("scale %g at %d DIP: got %v, want %v", scale, sample.x, got, sample.want)
			}
		}
		win.SetWidget(nil)
		p.Destroy()
	}
}

func TestImagePaintReusesResourceAndReleasesOnReplaceAndUnmount(t *testing.T) {
	first := image.NewRGBA(image.Rect(0, 0, 16, 8))
	second := image.NewRGBA(image.Rect(0, 0, 8, 4))
	view := NewImage(first)
	painter := new(testImagePainter)

	view.Paint(painter)
	resource := painter.image.(*testNativeImage)
	view.Paint(painter)
	if painter.newImages != 1 || painter.drawImages != 2 {
		t.Fatalf("repeated paint created/drew %d/%d images, want 1/2", painter.newImages, painter.drawImages)
	}

	view.SetImage(second)
	if !resource.destroyed {
		t.Fatal("replaced image resource was not destroyed")
	}
	view.Paint(painter)
	secondResource := painter.image.(*testNativeImage)
	if painter.newImages != 2 || secondResource == resource {
		t.Fatal("replacement did not create a new resource")
	}

	win := &window{}
	win.SetWidget(view)
	win.SetWidget(nil)
	if !secondResource.destroyed {
		t.Fatal("unmount did not destroy image resource")
	}
}

func TestImageRetriesResourceCreationAfterFailure(t *testing.T) {
	view := NewImage(image.NewRGBA(image.Rect(0, 0, 4, 4)))
	painter := &testImagePainter{newImageErr: errors.New("upload failed")}
	view.Paint(painter)
	if painter.newImages != 1 || painter.drawImages != 0 {
		t.Fatalf("failed creation counts = %d/%d, want 1/0", painter.newImages, painter.drawImages)
	}
	painter.newImageErr = nil
	view.Paint(painter)
	if painter.newImages != 2 || painter.drawImages != 1 {
		t.Fatalf("retry counts = %d/%d, want 2/1", painter.newImages, painter.drawImages)
	}
}

func TestImagePaintSkipsNilAndHiddenImage(t *testing.T) {
	painter := new(testImagePainter)
	NewImage(nil).Paint(painter)
	if painter.drawImages != 0 {
		t.Fatal("nil image should not be painted")
	}

	view := NewImage(image.NewRGBA(image.Rect(0, 0, 16, 8)))
	view.SetVisible(false)
	view.Paint(painter)
	if painter.drawImages != 0 {
		t.Fatal("hidden image should not be painted")
	}
}

type testImagePainter struct {
	testLabelPainter
	drawImages  int
	newImages   int
	newImageErr error
	imageRect   geometry.Rectangle
	source      image.Image
	image       graphics.Image
}

func (p *testImagePainter) NewImage(src image.Image) (graphics.Image, error) {
	p.newImages++
	p.source = src
	if p.newImageErr != nil {
		return nil, p.newImageErr
	}
	p.image = newTestNativeImage(src)
	return p.image, nil
}

func (p *testImagePainter) DrawImage(rect geometry.Rectangle, img graphics.Image) {
	p.drawImages++
	p.imageRect = rect
	p.image = img
}
func (p *testImagePainter) SetTransform(matrix geometry.Transform) {}

type testNativeImage struct {
	width     int
	height    int
	destroyed bool
}

func newTestNativeImage(src image.Image) *testNativeImage {
	if src == nil {
		return new(testNativeImage)
	}
	bounds := src.Bounds()
	return &testNativeImage{width: bounds.Dx(), height: bounds.Dy()}
}

func (i *testNativeImage) Size() (width, height int) { return i.width, i.height }
func (i *testNativeImage) Update(src image.Image) error {
	if src == nil {
		return errors.New("nil image")
	}
	bounds := src.Bounds()
	if bounds.Dx() != i.width || bounds.Dy() != i.height {
		return errors.New("image size changed")
	}
	return nil
}
func (i *testNativeImage) Destroy() { i.destroyed = true }
