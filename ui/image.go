package ui

import (
	"image"

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
	imageWidget.SetImage(v.img)
}

func (v *ImageView) Unmount(BuildContext, gui.Widget) {}
