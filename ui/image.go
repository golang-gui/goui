package ui

import (
	"image"
	"reflect"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
)

type ImageView struct {
	ViewBase[ImageView]
	img         image.Image
	contentSize geometry.Size
}

func Image(img image.Image) *ImageView {
	v := &ImageView{img: img}
	v.Self = v
	return v
}

func (v *ImageView) Image(img image.Image) *ImageView {
	v.img = img
	return v
}

// ContentSize sets the image content's size in DIP, independently of its pixel
// resolution. Both dimensions must be finite and positive; otherwise the source
// dimensions are used. Omitting the modifier also restores that default.
func (v *ImageView) ContentSize(width, height float32) *ImageView {
	v.contentSize = geometry.Size{Width: width, Height: height}
	return v
}

func (v *ImageView) Mount(BuildContext) gui.Widget {
	return gui.NewImage(v.img)
}

func (v *ImageView) Update(_ BuildContext, widget gui.Widget) {
	imageWidget := widget.(*gui.Image)
	if !sameImageSource(imageWidget.Image(), v.img) {
		imageWidget.SetImage(v.img)
	}
	imageWidget.SetContentSize(v.contentSize)
}

func (v *ImageView) Unmount(BuildContext, gui.Widget) {}

func sameImageSource(a, b image.Image) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ta := reflect.TypeOf(a)
	return ta == reflect.TypeOf(b) && ta.Comparable() && a == b
}
