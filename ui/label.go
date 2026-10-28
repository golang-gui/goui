package ui

import (
	"github.com/golang-gui/goui/gui"
)

type LabelView struct {
	ViewBase[LabelView]
	text       string
	textSource ReadState[string]
	wrapMode   gui.WrapMode
}

// Label declares plain text. Omission means empty text; at most one argument is allowed.
func Label(text ...string) *LabelView {
	if len(text) > 1 {
		panic("ui: Label accepts at most one text argument")
	}
	v := &LabelView{}
	if len(text) == 1 {
		v.text = text[0]
	}
	v.Self = v
	return v
}

func (v *LabelView) Text(text string) *LabelView {
	v.text = text
	v.textSource = nil
	return v
}

// BindText follows a readable value without rebuilding the containing View.
// A later Text replaces the binding. nil is a no-op.
func (v *LabelView) BindText(source ReadState[string]) *LabelView {
	if source != nil {
		validateObservable(source)
		v.textSource = source
	}
	return v
}

func (v *LabelView) WrapMode(mode gui.WrapMode) *LabelView {
	v.wrapMode = mode
	return v
}

func (v *LabelView) Mount(BuildContext) gui.Widget {
	return gui.NewLabel(v.text)
}

func (v *LabelView) Update(ctx BuildContext, widget gui.Widget) {
	label := widget.(*gui.Label)
	if v.textSource != nil {
		BindValue(ctx, "text", v.textSource, label.SetText)
	} else {
		label.SetText(v.text)
	}
	label.SetWrapMode(v.wrapMode)
}

func (v *LabelView) Unmount(BuildContext, gui.Widget) {}
