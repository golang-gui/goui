package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

const (
	captionButtonWidth      float32 = 44
	captionButtonHeight     float32 = 32
	circularControlSize     float32 = 28
	circularControlDiameter float32 = 24
	circularControlGap      float32 = 8
	circularControlInset    float32 = 12
)

// windowControls belongs exclusively to Window. Only custom presentation has
// GUI children; native controls retain OS input, painting and accessibility.
type windowControls struct {
	WidgetBase
	window      *window
	info        ChromeInfo
	buttons     [3]*captionButton
	connections signal.Handles
}

func newWindowControls(win *window) *windowControls {
	c := &windowControls{window: win}
	c.SetID("window-controls")
	c.hidden = true
	c.attachRoot(win, c)
	c.connections = signal.Handles{
		win.Chrome().ConnectInfo(c.changed),
		win.Chrome().ConnectQueryRegion(c.queryRegion),
		win.ConnectState(func(WindowState) { c.RequestPaint() }),
	}
	return c
}

func (c *windowControls) changed(info ChromeInfo) {
	c.info = info
	if info.Controls == ChromeControlsCustom && c.buttons[0] == nil {
		for i := range c.buttons {
			button := &captionButton{Button: NewButton(), circular: c.window.clientChrome}
			button.SetID([]string{"window-minimize", "window-maximize", "window-close"}[i])
			button.SetPadding(0)
			if c.window.clientChrome {
				button.SetStyleName(styleNameWindowControl)
			}
			button.SetFocusable(false) // caption buttons do not steal document focus
			icon := &captionIcon{controls: c, index: i}
			icon.SetMinSize(geometry.Size{Width: 12, Height: 12})
			// Attach to the wrapper identity, not the embedded Button: picking,
			// propagation and ancestor tests must all see the same parent.
			button.content = icon
			button.WidgetBase.AddChild(button, icon)
			button.ConnectClicked(func() { c.activate(i) })
			c.buttons[i] = button
			c.WidgetBase.AddChild(c, button)
		}
	}
}

// Only caption buttons separate their visible background from their hit box.
// The embedded Button still owns the existing click/hover state and signals.
type captionButton struct {
	*Button
	circular bool
}

func (b *captionButton) Paint(p Painter) {
	if !b.circular {
		b.Button.Paint(p)
		return
	}
	if b.Visible() {
		inset := (circularControlSize - circularControlDiameter) / 2
		paintStyledBox(p, geometry.Rect(0, 0, b.Rect().Width, b.Rect().Height).Inset(inset), b.resolvedStyle())
	}
}

// Window arranges its tree from Chrome's resolved observation. Chrome never
// reaches into this object; query and information signals carry the geometry.
func (c *windowControls) layout() {
	bounds := c.info.ControlsBounds
	visible := c.info.Controls == ChromeControlsCustom && !emptyRect(bounds)
	if !visible {
		if !c.hidden {
			if target := pathTarget(c.window.dispatcher.hoverPath); target != nil &&
				target.base().isDescendant(target, c) {
				c.window.dispatcher.clearHover(events.PointerEvent{EventType: events.PointerLeave})
			}
			for _, button := range c.buttons {
				button.click.Reset()
				button.setPressed(false)
				button.setHovered(false)
			}
		}
		c.hidden = true
		c.rect = geometry.Rectangle{}
		return
	}
	c.hidden = false
	c.rect = bounds
	for i, button := range c.buttons {
		rect := geometry.Rect(float32(i)*bounds.Width/3, 0, bounds.Width/3, bounds.Height)
		if c.window.clientChrome {
			rect = geometry.Rect(float32(i)*(circularControlSize+circularControlGap), 0, circularControlSize, circularControlSize)
		}
		measureWidget(button, layout.Tight(rect.Size))
		button.Arrange(rect)
	}
}

func (c *windowControls) queryRegion(p geometry.Point, result *ChromeRegion) {
	if c.info.Controls == ChromeControlsNative {
		// Native buttons own their input; no synthetic GUI hit role is needed.
		return
	}
	target := hitTest(c, p)
	if target == nil {
		return
	}
	*result = ChromeRegionClient
	for i, button := range c.buttons {
		if target.base().isDescendant(target, button) {
			*result = [...]ChromeRegion{ChromeRegionMinimize, ChromeRegionMaximize, ChromeRegionClose}[i]
			return
		}
	}
}

func (c *windowControls) activate(index int) {
	win := c.window
	if win == nil || win.destroyed || c.hidden {
		return
	}
	switch index {
	case 0:
		win.RequestState(WindowStateMinimized)
	case 1:
		if win.State() == WindowStateMaximized {
			win.RequestState(WindowStateNormal)
		} else {
			win.RequestState(WindowStateMaximized)
		}
	case 2:
		_ = win.RequestClose()
	}
}

func (c *windowControls) Snapshot() WidgetInfo {
	info := c.WidgetBase.Snapshot()
	info.Role = RoleHBox
	for i, text := range []string{"Minimize", "Maximize", "Close"} {
		if i == 1 && c.window.State() == WindowStateMaximized {
			text = "Restore"
		}
		if i < len(info.Children) {
			info.Children[i].Text = text
			info.Children[i].Children = nil // the icon is decorative
		}
	}
	return info
}

func (c *windowControls) release() {
	c.connections.Disconnect()
	c.connections = nil
	c.WidgetBase.destroy(c)
	c.window = nil
}

// Vector icons avoid font-dependent caption glyphs.
type captionIcon struct {
	WidgetBase
	controls *windowControls
	index    int
}

func (i *captionIcon) Paint(p Painter) {
	brush := graphics.RGB(0, 0, 0)
	if color, ok := i.controls.buttons[i.index].resolvedStyle().ForegroundColor(); ok {
		brush = graphics.ColorOf(color)
	}
	switch i.index {
	case 0:
		p.DrawLine(geometry.Point{X: 1, Y: 6}, geometry.Point{X: 11, Y: 6}, 1, brush)
	case 1:
		if i.controls.window.State() == WindowStateMaximized {
			p.DrawLine(geometry.Point{X: 3, Y: 1}, geometry.Point{X: 11, Y: 1}, 1, brush)
			p.DrawLine(geometry.Point{X: 11, Y: 1}, geometry.Point{X: 11, Y: 9}, 1, brush)
			p.DrawRect(geometry.Rect(1, 3, 8, 8), 1, brush)
		} else {
			p.DrawRect(geometry.Rect(1, 1, 10, 10), 1, brush)
		}
	case 2:
		p.DrawLine(geometry.Point{X: 1, Y: 1}, geometry.Point{X: 11, Y: 11}, 1, brush)
		p.DrawLine(geometry.Point{X: 11, Y: 1}, geometry.Point{X: 1, Y: 11}, 1, brush)
	}
}
