package ui

import (
	"image"

	"github.com/golang-gui/goui/gui"
)

type ImageView struct {
	name   string
	hidden bool
	img    image.Image
}

func Image(img image.Image) ImageView {
	return ImageView{img: img}
}

func (v ImageView) Name(name string) ImageView {
	v.name = name
	return v
}

func (v ImageView) Hidden(hidden bool) ImageView {
	v.hidden = hidden
	return v
}

func (v ImageView) Image(img image.Image) ImageView {
	v.img = img
	return v
}

func (v ImageView) Build() View {
	return v
}

func (v ImageView) Mount(BuildContext) gui.Widget {
	return gui.NewImage(v.img)
}

func (v ImageView) Update(_ BuildContext, widget gui.Widget) {
	imageWidget := widget.(*gui.Image)
	imageWidget.SetID(v.name)
	imageWidget.SetVisible(!v.hidden)
	imageWidget.SetImage(v.img)
}

func (v ImageView) Unmount(BuildContext, gui.Widget) {}
