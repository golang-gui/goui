package icon_test

import (
	"image"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/icon"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/ui"
)

// 通过公开 Widget API 使用真实图标资源，不让 gui 的包内测试依赖具体 IconSource。
func TestImageWidgetUploadReuse(t *testing.T) {
	source, err := icon.NewImage(
		image.NewRGBA(image.Rect(0, 0, 16, 16)),
		image.NewRGBA(image.Rect(0, 0, 32, 32)),
	)
	if err != nil {
		t.Fatal(err)
	}
	widget := gui.NewIcon(source)
	p := &imagePainter{}
	for _, size := range []float32{24, 25, 30} {
		widget.Arrange(geometry.Rect(0, 0, size, size))
		widget.Paint(p)
	}
	if p.uploads != 1 {
		t.Fatal("same selected bitmap was uploaded again")
	}
	widget.Arrange(geometry.Rect(0, 0, 16, 16))
	widget.Paint(p)
	if p.uploads != 2 {
		t.Fatal("new resolution was not uploaded")
	}
	widget.SetSource(nil)
	if p.destroyed != 2 {
		t.Fatal("native resources leaked")
	}
}

// UI 更新应重新请求图标源；返回同一份不可变像素时继续复用上传资源。
func TestImageViewUpdateUploadReuse(t *testing.T) {
	source, err := icon.NewImage(image.NewRGBA(image.Rect(0, 0, 16, 16)))
	if err != nil {
		t.Fatal(err)
	}
	ctx := &iconBuildContext{}
	counted := &countedSource{IconSource: source}
	widget := ui.Icon(counted).Mount(ctx).(*gui.Icon)
	defer widget.SetSource(nil)
	widget.Arrange(geometry.Rect(0, 0, 16, 16))
	p := &imagePainter{}
	for j := 0; j < 4; j++ {
		ui.Icon(counted).Update(ctx, widget)
		widget.Paint(p)
	}
	if counted.calls != 4 || p.uploads != 1 || p.destroyed != 0 {
		t.Fatalf("UI refresh/reuse: calls=%d uploads=%d destroyed=%d", counted.calls, p.uploads, p.destroyed)
	}
	ui.Icon(nil).Update(ctx, widget)
	if p.destroyed != 1 {
		t.Fatal("clearing the declaration did not release its image")
	}
}

type countedSource struct {
	gui.IconSource
	calls int
}

func (s *countedSource) Image(w, h int, c gui.Color) image.Image {
	s.calls++
	return s.IconSource.Image(w, h, c)
}

type iconBuildContext struct {
	ui.BuildContext
	state any
}

func (c *iconBuildContext) State() any     { return c.state }
func (c *iconBuildContext) SetState(v any) { c.state = v }

type imagePainter struct {
	gui.Painter
	uploads, destroyed int
}

func (*imagePainter) PixelScale() float32 { return 1 }
func (p *imagePainter) NewImage(src image.Image) (graphics.Image, error) {
	p.uploads++
	return &nativeImage{owner: p, bounds: src.Bounds()}, nil
}
func (*imagePainter) DrawImage(geometry.Rectangle, graphics.Image) {}

type nativeImage struct {
	graphics.Image
	owner  *imagePainter
	bounds image.Rectangle
}

func (i *nativeImage) Size() (int, int) { return i.bounds.Dx(), i.bounds.Dy() }
func (i *nativeImage) Destroy() {
	i.owner.destroyed++
}
