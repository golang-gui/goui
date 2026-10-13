package ui

import (
	"github.com/golang-gui/goui/core/optional"
	"github.com/golang-gui/goui/gui"
)

// BorderBoxView reconciles the GUI single-child frame. Resizing and cursor
// behavior belong to gui.BorderBox, not the declarative coordinator.
type BorderBoxView struct {
	ViewBase[BorderBoxView]
	child       View
	resizable   optional.Optional[bool]
	resizeWidth optional.Optional[float32]
}

type borderBoxState struct {
	resizable   bool
	resizeWidth float32
}

// BorderBox has no default appearance. At a WindowChromeNone root it supplies
// resize edges; use HeaderBar inside it for movement and StyleName for decoration.
func BorderBox(child View) *BorderBoxView {
	v := &BorderBoxView{child: child}
	v.Self = v
	return v
}

func (v *BorderBoxView) Child(child View) *BorderBoxView {
	v.child = child
	return v
}

func (v *BorderBoxView) Resizable(resizable bool) *BorderBoxView {
	v.resizable.SetValue(resizable)
	return v
}

// ResizeWidth sets the edge hit width and minimum content inset in DIP.
func (v *BorderBoxView) ResizeWidth(width float32) *BorderBoxView {
	v.resizeWidth.SetValue(width)
	return v
}

func (v *BorderBoxView) Build() View { return v }

func (v *BorderBoxView) Mount(ctx BuildContext) gui.Widget {
	b := gui.NewBorderBox()
	ctx.SetState(&borderBoxState{resizable: b.Resizable(), resizeWidth: b.ResizeWidth()})
	return b
}

func (v *BorderBoxView) Update(ctx BuildContext, widget gui.Widget) {
	b := widget.(*gui.BorderBox)
	defaults := ctx.State().(*borderBoxState)
	resizable, width := defaults.resizable, defaults.resizeWidth
	if v.resizable.HasValue() {
		resizable = v.resizable.Value()
	}
	if v.resizeWidth.HasValue() {
		width = v.resizeWidth.Value()
	}
	b.SetResizable(resizable)
	b.SetResizeWidth(width)
	ctx.UpdateChild(b, v.child)
}

func (v *BorderBoxView) Unmount(BuildContext, gui.Widget) {}
