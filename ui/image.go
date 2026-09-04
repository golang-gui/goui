package ui

import (
	"image"
	"reflect"

	"github.com/golang-gui/goui/gui"
)

type ImageView struct {
	ViewBase[ImageView]
	img image.Image
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

func (v *ImageView) Build() View {
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
}

func (v *ImageView) Unmount(BuildContext, gui.Widget) {}

func sameImageSource(a, b image.Image) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ta := reflect.TypeOf(a)
	return ta == reflect.TypeOf(b) && ta.Comparable() && a == b
}
