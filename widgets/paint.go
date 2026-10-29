package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"
)

// Keep part selection local to compound widgets; reuse the common drawing path.
func paintStyledBox(widget gui.Widget, p gui.Painter, rect geometry.Rectangle, name, part string, state style.State) {
	gui.PaintStyledBox(p, rect, gui.ResolveWidgetStyle(widget, name, part, state))
}

func paintStyledBorder(p gui.Painter, rect geometry.Rectangle, s style.Style) {
	gui.PaintStyledBorder(p, rect, s)
}
