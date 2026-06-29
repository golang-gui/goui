package gui

import (
	"image"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
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

	size := view.Measure(geometry.Size{Width: 100, Height: 100})

	if size != (geometry.Size{Width: 16, Height: 9}) {
		t.Fatalf("unexpected measured size: %+v", size)
	}
}

func TestImageMeasureSkipsNilAndHiddenImage(t *testing.T) {
	view := NewImage(nil)
	if size := view.Measure(geometry.Size{Width: 100, Height: 100}); size != (geometry.Size{}) {
		t.Fatalf("nil image measured non-zero size: %+v", size)
	}

	view.SetImage(image.NewRGBA(image.Rect(0, 0, 16, 8)))
	view.SetVisible(false)
	if size := view.Measure(geometry.Size{Width: 100, Height: 100}); size != (geometry.Size{}) {
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
	if painter.image != img {
		t.Fatal("painter did not receive image")
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
	drawImages int
	imageRect  geometry.Rectangle
	image      image.Image
}

func (p *testImagePainter) DrawImage(rect geometry.Rectangle, img image.Image) {
	p.drawImages++
	p.imageRect = rect
	p.image = img
}
