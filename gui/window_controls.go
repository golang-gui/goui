package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
	"image/color"
)

const (
	captionButtonWidth      float32 = 46
	captionButtonHeight     float32 = 32
	circularControlSize     float32 = 28
	circularControlDiameter float32 = 24
	circularControlGap      float32 = 8
	circularControlInset    float32 = 6 // visible circle is another 2 DIP inside its hit box
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
	c.SetName("window-controls")
	c.hidden = true
	c.attachRoot(win, c)
	c.connections = signal.Handles{
		win.Chrome().ConnectInfo(c.changed),
		win.Chrome().ConnectQueryRegion(c.queryRegion),
		win.ConnectState(func(WindowState) { c.RequestPaint() }),
		win.ConnectFocus(func(bool) { c.RequestPaint() }),
	}
	return c
}

func (c *windowControls) changed(info ChromeInfo) {
	c.info = info
	if info.Controls == ChromeControlsCustom && c.buttons[0] == nil {
		for i := range c.buttons {
			button := &captionButton{Button: NewButton(), window: c.window, circular: c.window.clientChrome, close: i == 2}
			button.SetName([]string{"window-minimize", "window-maximize", "window-close"}[i])
			button.SetPadding(0)
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
	window   *window
	circular bool
	close    bool
}

func (b *captionButton) Measure(c layout.Constraint) layout.Measurement {
	// Chrome supplies tight allocations; the icon has a fixed intrinsic size.
	// Do not inherit Button's application-font-derived measurement floor.
	return b.WidgetBase.Measure(c)
}

func (b *captionButton) decorationStyle() style.Style {
	palette := b.window.decorationPalette(b.circular)
	if !b.circular {
		// Windows caption controls blend into the titlebar at rest. Their
		// rectangular state fills and close warning color are decoration policy,
		// independent of application Button styles and accent colors.
		var background color.Color = color.Transparent
		var foreground color.Color = palette.foreground
		if b.close && (b.hovered || b.pressed) {
			background = color.NRGBA{R: 196, G: 43, B: 28, A: 255}
			foreground = color.White
			if b.pressed {
				background = color.NRGBA{R: 176, G: 37, B: 26, A: 255}
			}
		} else {
			ink := uint8(0)
			if palette.foreground.Y > 128 {
				ink = 255
			}
			if b.hovered || b.pressed {
				alpha := uint8(26)
				if b.pressed {
					alpha = 51
				}
				background = color.NRGBA{R: ink, G: ink, B: ink, A: alpha}
			} else if !b.window.Focused() {
				foreground = color.NRGBA{R: ink, G: ink, B: ink, A: 102}
			}
		}
		return style.Default().BackgroundColor(background).ForegroundColor(foreground).Radius(0).Style
	}
	background := palette.button
	if b.pressed {
		background = palette.pressed
	} else if b.hovered {
		background = palette.hovered
	}
	return style.Default().BackgroundColor(background).ForegroundColor(palette.foreground).
		Radius(circularControlDiameter / 2).Style
}

func (b *captionButton) Paint(p Painter) {
	if !b.Visible() {
		return
	}
	rect := geometry.Rect(0, 0, b.Rect().Width, b.Rect().Height)
	if b.circular {
		inset := (circularControlSize - circularControlDiameter) / 2
		rect = rect.Inset(inset)
		paintStyledBox(p, rect, b.decorationStyle())
		return
	}
	background, _ := b.decorationStyle().BackgroundColor()
	p.FillRect(rect, b.decorationBrush(background))
}

func (b *captionButton) decorationBrush(c color.Color) graphics.Color {
	if b.circular {
		return graphics.ColorOf(c) // Preserve Linux's existing decoration path.
	}
	// Graphics brushes use straight alpha. ColorOf currently returns
	// premultiplied RGB; feeding it translucent white applies alpha twice.
	// Keep this correction local until the shared color contract is reconciled.
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	return graphics.RGBA(n.R, n.G, n.B, n.A)
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
	foreground, _ := i.controls.buttons[i.index].decorationStyle().ForegroundColor()
	brush := i.controls.buttons[i.index].decorationBrush(foreground)
	// Keep the 12 DIP icon allocation and 1 DIP stroke. Only the Linux
	// circular presentation uses a smaller glyph; button hit boxes are unchanged.
	left, right := float32(1), float32(11)
	if i.controls.buttons[i.index].circular {
		left, right = 2, 10
	}
	span := right - left
	switch i.index {
	case 0:
		p.DrawLine(geometry.Point{X: left, Y: 6}, geometry.Point{X: right, Y: 6}, 1, brush)
	case 1:
		if i.controls.window.State() == WindowStateMaximized {
			p.DrawLine(geometry.Point{X: left + 2, Y: left}, geometry.Point{X: right, Y: left}, 1, brush)
			p.DrawLine(geometry.Point{X: right, Y: left}, geometry.Point{X: right, Y: right - 2}, 1, brush)
			p.DrawRect(geometry.Rect(left, left+2, span-2, span-2), 1, brush)
		} else {
			p.DrawRect(geometry.Rect(left, left, span, span), 1, brush)
		}
	case 2:
		p.DrawLine(geometry.Point{X: left, Y: left}, geometry.Point{X: right, Y: right}, 1, brush)
		p.DrawLine(geometry.Point{X: right, Y: left}, geometry.Point{X: left, Y: right}, 1, brush)
	}
}
