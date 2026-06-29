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

func (v LabelView) Build(ctx BuildContext, old gui.Widget) gui.Widget {
	label, _ := old.(*gui.Label)
	if label == nil {
		label = gui.NewLabel(v.text)
	}
	label.SetID(v.name)
	label.SetVisible(!v.hidden)
	label.SetText(v.text)
	return label
}

type ButtonView struct {
	name    string
	hidden  bool
	child   View
	onClick func()
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

func (v ButtonView) Build(ctx BuildContext, old gui.Widget) gui.Widget {
	button, _ := old.(*gui.Button)
	if button == nil {
		button = gui.NewButton()
	}
	button.SetID(v.name)
	button.SetVisible(!v.hidden)
	if v.child == nil {
		ctx.UpdateChildren(button, nil)
	} else {
		ctx.UpdateChildren(button, []View{v.child})
	}
	ctx.Connect(button, "click", func() signal.Handle {
		if v.onClick == nil {
			return nil
		}
		return button.ConnectClicked(v.onClick)
	})
	return button
}

type TextInputView struct {
	name   string
	hidden bool
	text   string
	onText func(string)
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

func (v TextInputView) Build(ctx BuildContext, old gui.Widget) gui.Widget {
	input, _ := old.(*gui.TextInput)
	if input == nil {
		input = gui.NewTextInput()
	}
	input.SetID(v.name)
	input.SetVisible(!v.hidden)
	ctx.Connect(input, "text", nil)
	input.SetText(v.text)
	ctx.Connect(input, "text", func() signal.Handle {
		if v.onText == nil {
			return nil
		}
		return input.ConnectText(v.onText)
	})
	return input
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

func (v ImageView) Build(ctx BuildContext, old gui.Widget) gui.Widget {
	imageWidget, _ := old.(*gui.Image)
	if imageWidget == nil {
		imageWidget = gui.NewImage(v.img)
	}
	imageWidget.SetID(v.name)
	imageWidget.SetVisible(!v.hidden)
	imageWidget.SetImage(v.img)
	return imageWidget
}

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

func (v BoxView) Build(ctx BuildContext, old gui.Widget) gui.Widget {
	box, _ := old.(*gui.LinearBox)
	if box == nil {
		box = gui.NewLinearBox(v.direction)
	}
	box.SetID(v.name)
	box.SetVisible(!v.hidden)
	box.SetDirection(v.direction)
	box.SetSpacing(v.spacing)
	ctx.UpdateChildren(box, v.children)
	return box
}
