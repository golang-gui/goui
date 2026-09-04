package gui

import (
	"errors"
	"image"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
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

	if size != (geometry.Size{Width: 16, Height: 9}) {
		t.Fatalf("unexpected measured size: %+v", size)
	}
}

func TestImageMeasureSkipsNilAndHiddenImage(t *testing.T) {
	view := NewImage(nil)
	if size := view.Measure(layout.Loose(geometry.Size{Width: 100, Height: 100})); size != (geometry.Size{}) {
		t.Fatalf("nil image measured non-zero size: %+v", size)
	}

	view.SetImage(image.NewRGBA(image.Rect(0, 0, 16, 8)))
	view.SetVisible(false)
	if size := view.Measure(layout.Loose(geometry.Size{Width: 100, Height: 100})); size != (geometry.Size{}) {
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
