package ui

import (
	"github.com/golang-gui/goui/core/optional"
	"github.com/golang-gui/goui/gui"
)

// Painter, Color and IconDrawFunc share GUI's synchronous drawing contract.
type Painter = gui.Painter
type Color = gui.Color
type IconDrawFunc = gui.IconDrawFunc

type IconView struct {
	ViewBase[IconView]
	draw IconDrawFunc
	size optional.Optional[float32]
}

// Icon declares drawing content whose foreground comes from its own style.
func Icon(draw IconDrawFunc) *IconView {
	v := &IconView{draw: draw}
	v.Self = v
	return v
}

func (v *IconView) DrawFunc(draw IconDrawFunc) *IconView {
	v.draw = draw
	return v
}

// Size sets the preferred square edge in DIP. Removing the modifier restores
// the constructor default; allocation remains subject to layout constraints.
func (v *IconView) Size(size float32) *IconView {
	v.size.SetValue(size)
	return v
}

func (v *IconView) Build() View { return v }

func (v *IconView) Mount(ctx BuildContext) gui.Widget {
	icon := gui.NewIcon(v.draw)
	ctx.SetState(icon.Size())
	return icon
}

func (v *IconView) Update(ctx BuildContext, widget gui.Widget) {
	icon := widget.(*gui.Icon)
	icon.SetDrawFunc(v.draw)
	if v.size.HasValue() {
		icon.SetSize(v.size.Value())
	} else {
		icon.SetSize(ctx.State().(float32))
	}
}

func (v *IconView) Unmount(BuildContext, gui.Widget) {}
