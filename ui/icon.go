package ui

import (
	"github.com/golang-gui/goui/core/optional"
	"github.com/golang-gui/goui/gui"
)

// Icon content shares GUI's drawing and immutable-resource contracts.
type Painter = gui.Painter
type Color = gui.Color
type IconSource = gui.IconSource

type IconView struct {
	ViewBase[IconView]
	source IconSource
	size   optional.Optional[float32]
}

// Icon declares drawing content whose foreground comes from its own style.
func Icon(source IconSource) *IconView {
	v := &IconView{source: source}
	v.Self = v
	return v
}

func (v *IconView) Source(source IconSource) *IconView {
	v.source = source
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
	icon := gui.NewIcon(v.source)
	ctx.SetState(icon.Size())
	return icon
}

func (v *IconView) Update(ctx BuildContext, widget gui.Widget) {
	icon := widget.(*gui.Icon)
	icon.SetSource(v.source)
	if v.size.HasValue() {
		icon.SetSize(v.size.Value())
	} else {
		icon.SetSize(ctx.State().(float32))
	}
}

func (v *IconView) Unmount(BuildContext, gui.Widget) {}

var _ WidgetView = (*IconView)(nil)
