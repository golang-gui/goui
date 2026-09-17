package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

// Color is a premultiplied drawing color, shared with the graphics backend.
type Color = graphics.Color

// IconDrawFunc draws an icon in rect, a centered square in Widget-local DIP.
// foreground is the icon's own resolved style color (transparent when unset).
// The function runs synchronously during Paint: do not retain p, load assets,
// or mutate the Widget tree. Pair every Painter.Save with Painter.Restore.
// Reusable paths and other immutable drawing data may be captured in closures.
type IconDrawFunc func(p Painter, rect geometry.Rectangle, foreground Color)

// Icon displays resolution-independent, application-supplied drawing content.
// It owns no native drawing resources and does not inherit its parent's style.
type Icon struct {
	WidgetBase
	draw IconDrawFunc
	size float32
}

// NewIcon creates an icon with a preferred size of 16 DIP. A nil draw function
// supplies no content and has zero natural size.
func NewIcon(draw IconDrawFunc) *Icon { return &Icon{draw: draw, size: 16} }

func (i *Icon) DrawFunc() IconDrawFunc { return i.draw }

// SetDrawFunc replaces the drawing strategy and requests repainting. Changing
// whether content is present also invalidates measurement.
func (i *Icon) SetDrawFunc(draw IconDrawFunc) {
	contentChanged := (i.draw == nil) != (draw == nil)
	i.draw = draw
	if contentChanged {
		i.RequestLayout()
	}
	i.RequestPaint()
}

// Size returns the preferred square edge in DIP, not the allocated size.
func (i *Icon) Size() float32 { return i.size }

// SetSize sets the preferred edge. Negative and non-finite values become zero.
// Parent constraints and Widget min/max sizes still determine allocation.
func (i *Icon) SetSize(size float32) {
	size = normalizeLayoutValue(size)
	if i.size == size {
		return
	}
	i.size = size
	i.RequestLayout()
}

func (i *Icon) Measure(c layout.Constraint) layout.Measurement {
	if !i.Visible() {
		return layout.Measurement{}
	}
	var natural geometry.Size
	if i.draw != nil {
		natural = geometry.Size{Width: i.size, Height: i.size}
	}
	return layout.Measured(i.constrain(c, natural))
}

func (i *Icon) Paint(p Painter) {
	if !i.Visible() || i.draw == nil {
		return
	}
	bounds := i.Rect()
	edge := min(bounds.Width, bounds.Height)
	if edge <= 0 {
		return
	}
	name := i.StyleName()
	if name == "" {
		name = styleNameIcon
	}
	s := ResolveStyle(name, "", style.Normal)
	var foreground Color
	if c, ok := s.ForegroundColor(); ok && c != nil {
		foreground = graphics.ColorOf(c)
	}
	i.draw(p, geometry.Rect((bounds.Width-edge)/2, (bounds.Height-edge)/2, edge, edge), foreground)
}

func (i *Icon) Snapshot() WidgetInfo {
	info := i.WidgetBase.Snapshot()
	info.Role = RoleImage
	return info
}
