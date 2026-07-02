package ui

import "github.com/golang-gui/goui/gui"

type LabelView struct {
	name   string
	hidden bool
	text   string
}

func Label(text string) LabelView {
	return LabelView{text: text}
}

func (v LabelView) Name(name string) LabelView {
	v.name = name
	return v
}

func (v LabelView) Hidden(hidden bool) LabelView {
	v.hidden = hidden
	return v
}

func (v LabelView) Text(text string) LabelView {
	v.text = text
	return v
}

func (v LabelView) Build() View {
	return v
}

func (v LabelView) Mount(BuildContext) gui.Widget {
	return gui.NewLabel(v.text)
}

func (v LabelView) Update(_ BuildContext, widget gui.Widget) {
	label := widget.(*gui.Label)
	label.SetID(v.name)
	label.SetVisible(!v.hidden)
	label.SetText(v.text)
}

func (v LabelView) Unmount(BuildContext, gui.Widget) {}
