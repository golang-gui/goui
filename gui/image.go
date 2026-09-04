package gui

import (
	"image"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
)

type Image struct {
	WidgetBase
	img        image.Image
	paintImage graphics.Image
}

func NewImage(img image.Image) *Image {
	widget := &Image{img: img}
	widget.ConnectUnmount(widget.releaseImage)
	return widget
}

func (i *Image) Image() image.Image {
	return i.img
}

func (i *Image) SetImage(img image.Image) {
	if i.img == nil && img == nil {
		return
	}
	i.releaseImage()
	i.img = img
	i.RequestLayout()
	i.requestSemanticUpdate()
}

func (i *Image) Measure(c layout.Constraint) geometry.Size {
	if !i.Visible() || i.img == nil {
		return geometry.Size{}
	}
	bounds := i.img.Bounds()
	return i.constrain(c, geometry.Size{
		Width:  float32(bounds.Dx()),
		Height: float32(bounds.Dy()),
	})
}

func (i *Image) Paint(p Painter) {
	if !i.Visible() {
		return
	}
	if i.img != nil {
		if i.paintImage == nil {
			i.paintImage, _ = p.NewImage(i.img)
		}
		if i.paintImage != nil {
			width, height := i.paintImage.Size()
			p.DrawImage(geometry.Rect(0, 0, float32(width), float32(height)), i.paintImage)
		}
	}
	i.PaintChildren(p)
}

func (i *Image) releaseImage() {
	if i.paintImage != nil {
		i.paintImage.Destroy()
		i.paintImage = nil
	}
}

func (i *Image) Snapshot() WidgetInfo {
	info := i.WidgetBase.Snapshot()
	info.Role = RoleImage
	return info
}
