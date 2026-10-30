package gui

import "github.com/golang-gui/goui/layout"

// MenuButtonGroup arranges local menu buttons horizontally. Descendant
// MenuButtons use their nearest group for pointer and keyboard switching.
// Ordinary buttons can share the layout without joining menu navigation.
// Nested groups are independent. Styling each button remains explicit.
type MenuButtonGroup struct{ LinearBox }

// NewMenuButtonGroup creates a horizontal group with no spacing or padding.
func NewMenuButtonGroup() *MenuButtonGroup {
	return &MenuButtonGroup{LinearBox: *NewLinearBox(layout.DirectionHorizontal)}
}

func (g *MenuButtonGroup) AddChild(child Widget) { g.WidgetBase.AddChild(g, child) }

func (g *MenuButtonGroup) Paint(p Painter) { PaintWidgetStyle(p, g, "menu-button-group") }
