package gui

import (
	"image"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
)

type Image struct {
	WidgetBase
	img image.Image
}

func NewImage(img image.Image) *Image {
	image := &Image{img: img}
	image.SetStyleName(styleNameImage)
	return image
}

func (i *Image) Image() image.Image {
	return i.img
}

func (i *Image) SetImage(img image.Image) {
	if i.img == nil && img == nil {
		return
	}
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
		bounds := i.img.Bounds()
		p.DrawImage(geometry.Rect(0, 0, float32(bounds.Dx()), float32(bounds.Dy())), i.img)
	}
	i.PaintChildren(p)
}

func (i *Image) Snapshot() WidgetInfo {
	info := i.WidgetBase.Snapshot()
	info.Role = RoleImage
	return info
}
