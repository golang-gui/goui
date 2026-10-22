package gui

import (
	"image"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
)

// Image displays immutable pixels at a logical size. By default, each source
// pixel contributes one DIP to the content's width and height.
type Image struct {
	WidgetBase
	img         image.Image
	contentSize geometry.Size
	paintImage  graphics.Image
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

// ContentSize returns the explicitly configured content size in DIP.
// A zero size means that the source pixel dimensions are used as DIP.
func (i *Image) ContentSize() geometry.Size { return i.contentSize }

// SetContentSize sets the content's logical width and height independently of
// its pixel resolution. Both dimensions must be finite and positive; otherwise
// the size resets to zero. It does not preserve aspect ratio or fit the content
// to the widget's allocation. Min/max sizes remain layout constraints only.
func (i *Image) SetContentSize(size geometry.Size) {
	if normalizeLayoutValue(size.Width) <= 0 || normalizeLayoutValue(size.Height) <= 0 {
		size = geometry.Size{}
	}
	if i.contentSize == size {
		return
	}
	i.contentSize = size
	i.RequestLayout()
}

func (i *Image) naturalSize() geometry.Size {
	if i.contentSize != (geometry.Size{}) {
		return i.contentSize
	}
	bounds := i.img.Bounds()
	return geometry.Size{Width: float32(bounds.Dx()), Height: float32(bounds.Dy())}
}

func (i *Image) Measure(c layout.Constraint) layout.Measurement {
	if !i.Visible() || i.img == nil {
		return layout.Measurement{}
	}
	return layout.Measured(i.constrain(c, i.naturalSize()))
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
			p.DrawImage(geometry.Rectangle{Size: i.naturalSize()}, i.paintImage)
		}
	}
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
