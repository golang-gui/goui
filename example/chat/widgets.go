package main

import (
	"image/color"
	"math"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/ui"
)

// avatar is a round initial over a diagonal gradient of the member's color.
func avatar(name string, color, size int) ui.View {
	edge := float32(size)
	disc := newDisc(name, avatarColor(color)).MinSize(edge, edge).MaxSize(edge, edge)
	return ui.Overlay(disc, ui.OverlayItem(ui.Label(initial(name)).Style(avatarTextStyle(size))).
		OnPosition(func(a, s geometry.Size, p *geometry.Point) {
			*p = geometry.Point{X: (a.Width - s.Width) / 2, Y: (a.Height - s.Height) / 2}
		}))
}

// discWidget paints a circle shaded from a lighter to a deeper tone of one
// color, top-left to bottom-right. It is decoration; the name is its text.
type discWidget struct {
	gui.WidgetBase
	name  string
	color color.NRGBA
}

func (w *discWidget) Paint(p gui.Painter) {
	size := w.Rect().Size
	if size.Width <= 0 || size.Height <= 0 {
		return
	}
	light := mix(w.color, rgb(0xFFFFFF), .22)
	deep := mix(w.color, rgb(0x000000), .12)
	r := min(size.Width, size.Height) / 2
	p.FillEllipse(geometry.Point{X: size.Width / 2, Y: size.Height / 2}, r, r, graphics.LinearGradient{
		End:        graphics.Point{X: size.Width, Y: size.Height},
		StartColor: graphics.ColorOf(light), EndColor: graphics.ColorOf(deep),
	})
}

func (w *discWidget) Snapshot() gui.WidgetInfo {
	info := w.WidgetBase.Snapshot()
	info.Role = gui.RoleImage
	info.Text = w.name
	return info
}

type discView struct {
	ui.ViewBase[discView]
	name  string
	color color.NRGBA
}

func newDisc(name string, c color.NRGBA) *discView {
	v := &discView{name: name, color: c}
	v.Self = v
	return v
}

func (*discView) Mount(ui.BuildContext) gui.Widget { return new(discWidget) }
func (v *discView) Update(_ ui.BuildContext, widget gui.Widget) {
	w := widget.(*discWidget)
	if w.name != v.name || w.color != v.color {
		w.name, w.color = v.name, v.color
		w.RequestPaint()
	}
}
func (*discView) Unmount(ui.BuildContext, gui.Widget) {}

// shadeWidget fades its style's foreground color from transparent at the top
// to the color at the bottom. Placed above the composer it reads as a soft
// shadow cast upwards over the messages.
type shadeWidget struct{ gui.WidgetBase }

func (w *shadeWidget) Measure(c layout.Constraint) layout.Measurement {
	if !w.Visible() {
		return layout.Measurement{}
	}
	width := float32(0)
	if c.Max.Width < layout.Inf {
		width = c.Max.Width
	}
	return layout.Measured(c.Clamp(geometry.Size{Width: width, Height: 10}))
}

func (w *shadeWidget) Paint(p gui.Painter) {
	ink, ok := gui.WidgetStyle(w, styleShade, "").ForegroundColor()
	size := w.Rect().Size
	if !ok || ink == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	p.FillRect(geometry.Rectangle{Size: size}, graphics.LinearGradient{
		End:      graphics.Point{Y: size.Height},
		EndColor: graphics.ColorOf(ink),
	})
}

type shadeView struct{ ui.ViewBase[shadeView] }

func shade() *shadeView                                { v := new(shadeView); v.Self = v; return v }
func (*shadeView) Mount(ui.BuildContext) gui.Widget    { return new(shadeWidget) }
func (*shadeView) Update(ui.BuildContext, gui.Widget)  {}
func (*shadeView) Unmount(ui.BuildContext, gui.Widget) {}

// presence adds the online dot to the bottom-right of an avatar.
func presence(child ui.View, online bool) ui.View {
	if !online {
		return child
	}
	dot := ui.VBox().Style(styleDot).MinSize(12, 12).MaxSize(12, 12)
	return ui.Overlay(child, ui.OverlayItem(dot).OnPosition(func(a, s geometry.Size, p *geometry.Point) {
		*p = geometry.Point{X: a.Width - s.Width + 1, Y: a.Height - s.Height + 1}
	}))
}

// typingWidget draws three dots whose opacity breathes in a staggered wave.
// It follows the window's frame clock only while it is mounted and drawable.
type typingWidget struct {
	gui.WidgetBase
	started time.Time
	elapsed time.Duration
	handles signal.Handles
}

const typingPeriod = 1200 * time.Millisecond

func newTypingWidget() *typingWidget {
	w := &typingWidget{}
	w.ConnectMount(func() {
		root := w.Root()
		if root == nil {
			return
		}
		w.started = time.Now()
		w.handles = signal.Handles{
			root.ConnectFrame(func(now time.Time) {
				w.elapsed = max(0, now.Sub(w.started))
				w.RequestPaint()
			}),
			root.ConnectAfterPaint(func(gui.Painter, geometry.Rectangle) {
				if gui.IsDrawable(w) {
					_ = root.RequestPaint()
				}
			}),
		}
		_ = root.RequestPaint()
	})
	w.ConnectUnmount(func() {
		w.handles.Disconnect()
		w.handles = nil
	})
	return w
}

func (w *typingWidget) Measure(c layout.Constraint) layout.Measurement {
	if !w.Visible() {
		return layout.Measurement{}
	}
	return layout.Measured(c.Clamp(geometry.Size{Width: 34, Height: 20}))
}

func (w *typingWidget) Paint(p gui.Painter) {
	ink, ok := gui.WidgetStyle(w, styleTyping, "").ForegroundColor()
	if !ok || ink == nil {
		return
	}
	base := graphics.ColorOf(ink)
	size := w.Rect().Size
	phase := float64(w.elapsed) / float64(typingPeriod) * 2 * math.Pi
	for i := range 3 {
		wave := .5 + .5*math.Sin(phase-float64(i)*.9)
		a := float32(.3 + .7*wave)
		c := graphics.Color{R: base.R * a, G: base.G * a, B: base.B * a, A: base.A * a}
		p.FillEllipse(geometry.Point{X: size.Width/2 + float32(i-1)*11, Y: size.Height / 2}, 3, 3, c)
	}
}

func (w *typingWidget) Snapshot() gui.WidgetInfo {
	info := w.WidgetBase.Snapshot()
	info.Role = gui.RoleImage
	info.Text = "正在输入"
	return info
}

type typingView struct{ ui.ViewBase[typingView] }

func typing() *typingView                               { v := new(typingView); v.Self = v; return v }
func (*typingView) Mount(ui.BuildContext) gui.Widget    { return newTypingWidget() }
func (*typingView) Update(ui.BuildContext, gui.Widget)  {}
func (*typingView) Unmount(ui.BuildContext, gui.Widget) {}

// afterView runs fn once the enclosing update has been reconciled, without
// adding a widget or layout boundary.
type afterView struct {
	ui.ComposeBase[afterView]
	child ui.View
	after func()
}

func afterUpdate(child ui.View, fn func()) *afterView {
	v := &afterView{child: child, after: fn}
	v.Self = v
	return v
}

func (v *afterView) Build(ctx ui.BuildContext) ui.View {
	if v.after != nil {
		ctx.AfterUpdate(v.after)
	}
	return v.child
}
