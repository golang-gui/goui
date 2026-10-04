package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

// Shared box painting for compound widgets; style resolution stays local to
// the widget/part, never inherited from its parent.
func paintStyledBox(p gui.Painter, rect geometry.Rectangle, name, part string, state style.State) {
	s := gui.ResolveStyle(name, part, state)
	radius, _ := s.Radius()
	if bg, ok := s.BackgroundColor(); ok && bg != nil {
		if radius > 0 {
			p.FillRoundRect(rect, radius, graphics.ColorOf(bg))
		} else {
			p.FillRect(rect, graphics.ColorOf(bg))
		}
	}
	paintStyledBorder(p, rect, s)
}

// A focus/current indicator must not repaint the underlying selected fill.
func paintStyledBorder(p gui.Painter, rect geometry.Rectangle, s style.Style) {
	radius, _ := s.Radius()
	width, hasWidth := s.BorderWidth()
	border, hasColor := s.BorderColor()
	if hasWidth && hasColor && width > 0 && border != nil {
		inside := rect.Inset(width / 2)
		if radius > 0 {
			p.DrawRoundRect(inside, max(0, radius-width/2), width, graphics.ColorOf(border))
		} else {
			p.DrawRect(inside, width, graphics.ColorOf(border))
		}
	}
}
