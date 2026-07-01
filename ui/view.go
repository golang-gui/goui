package ui

import (
	"image"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

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

type ButtonView struct {
	name    string
	hidden  bool
	child   View
	onClick func()
}

type buttonState struct {
	onClick func()
	click   signal.Handle
}

func Button(child View) ButtonView {
	return ButtonView{child: child}
}

func (v ButtonView) Name(name string) ButtonView {
	v.name = name
	return v
}

func (v ButtonView) Hidden(hidden bool) ButtonView {
	v.hidden = hidden
	return v
}

func (v ButtonView) Child(child View) ButtonView {
	v.child = child
	return v
}

func (v ButtonView) OnClick(fn func()) ButtonView {
	v.onClick = fn
	return v
}

func (v ButtonView) Mount(ctx BuildContext) gui.Widget {
	button := gui.NewButton()
	state := &buttonState{}
	state.click = button.ConnectClicked(func() {
		if state.onClick != nil {
			state.onClick()
		}
	})
	ctx.SetState(state)
	return button
}

func (v ButtonView) Update(ctx BuildContext, widget gui.Widget) {
	button := widget.(*gui.Button)
	state := ctx.State().(*buttonState)
	state.onClick = v.onClick
	button.SetID(v.name)
	button.SetVisible(!v.hidden)
	if v.child == nil {
		ctx.UpdateChildren(button, nil)
	} else {
		ctx.UpdateChildren(button, []View{v.child})
	}
}

func (v ButtonView) Unmount(ctx BuildContext, _ gui.Widget) {
	state, _ := ctx.State().(*buttonState)
	if state != nil && state.click != nil {
		state.click.Disconnect()
	}
}

type TextInputView struct {
	name   string
	hidden bool
	text   string
	onText func(string)
}

type textInputState struct {
	onText func(string)
	text   signal.Handle
}

func TextInput() TextInputView {
	return TextInputView{}
}

func (v TextInputView) Name(name string) TextInputView {
	v.name = name
	return v
}

func (v TextInputView) Hidden(hidden bool) TextInputView {
	v.hidden = hidden
	return v
}

func (v TextInputView) Text(text string) TextInputView {
	v.text = text
	return v
}

func (v TextInputView) OnText(fn func(string)) TextInputView {
	v.onText = fn
	return v
}

func (v TextInputView) Mount(ctx BuildContext) gui.Widget {
	input := gui.NewTextInput()
	state := &textInputState{}
	state.text = input.ConnectText(func(text string) {
		if state.onText != nil {
			state.onText(text)
		}
	})
	ctx.SetState(state)
	return input
}

func (v TextInputView) Update(ctx BuildContext, widget gui.Widget) {
	input := widget.(*gui.TextInput)
	state := ctx.State().(*textInputState)
	input.SetID(v.name)
	input.SetVisible(!v.hidden)
	func() {
		state.text.Block()
		defer state.text.Unblock()
		input.SetText(v.text)
	}()
	state.onText = v.onText
}

func (v TextInputView) Unmount(ctx BuildContext, _ gui.Widget) {
	state, _ := ctx.State().(*textInputState)
	if state != nil && state.text != nil {
		state.text.Disconnect()
	}
}

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

type BoxView struct {
	name      string
	hidden    bool
	direction layout.Direction
	spacing   float32
	children  []View
}

func HBox(children ...View) BoxView {
	return BoxView{
		direction: layout.DirectionHorizontal,
		children:  compactViews(children),
	}
}

func VBox(children ...View) BoxView {
	return BoxView{
		direction: layout.DirectionVertical,
		children:  compactViews(children),
	}
}

func (v BoxView) Name(name string) BoxView {
	v.name = name
	return v
}

func (v BoxView) Hidden(hidden bool) BoxView {
	v.hidden = hidden
	return v
}

func (v BoxView) Spacing(spacing float32) BoxView {
	v.spacing = spacing
	return v
}

func (v BoxView) Children(children ...View) BoxView {
	v.children = compactViews(children)
	return v
}

func (v BoxView) Mount(BuildContext) gui.Widget {
	return gui.NewLinearBox(v.direction)
}

func (v BoxView) Update(ctx BuildContext, widget gui.Widget) {
	box := widget.(*gui.LinearBox)
	box.SetID(v.name)
	box.SetVisible(!v.hidden)
	box.SetDirection(v.direction)
	box.SetSpacing(v.spacing)
	ctx.UpdateChildren(box, v.children)
}

func (v BoxView) Unmount(BuildContext, gui.Widget) {}
