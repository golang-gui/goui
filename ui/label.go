package ui

import (
	"github.com/golang-gui/goui/gui"
)

type LabelView struct {
	ViewBase[LabelView]
	text     string
	wrapMode gui.WrapMode
}

func Label(text string) *LabelView {
	v := &LabelView{text: text}
	v.Self = v
	return v
}

func (v *LabelView) Text(text string) *LabelView {
	v.text = text
	return v
}

func (v *LabelView) WrapMode(mode gui.WrapMode) *LabelView {
	v.wrapMode = mode
	return v
}

func (v *LabelView) Build() View {
	return v
}

func (v *LabelView) Mount(BuildContext) gui.Widget {
	return gui.NewLabel(v.text)
}

func (v *LabelView) Update(_ BuildContext, widget gui.Widget) {
	label := widget.(*gui.Label)
	label.SetText(v.text)
	label.SetWrapMode(v.wrapMode)
}

func (v *LabelView) Unmount(BuildContext, gui.Widget) {}
