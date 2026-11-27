package main

import (
	"math"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/ui"
)

// canvasWidget paints with the window's Painter. While live reports true it
// follows the window's frame clock and repaints every frame; otherwise it
// stays idle until its declaration changes.
type canvasWidget struct {
	gui.WidgetBase
	label   string
	draw    gui.DrawFunc
	live    func() bool
	frame   func(time.Time)
	handles signal.Handles
}

func newCanvasWidget() *canvasWidget {
	w := new(canvasWidget)
	w.ConnectMount(func() {
		root := w.Root()
		if root == nil {
			return
		}
		w.handles = signal.Handles{
			root.ConnectFrame(func(now time.Time) {
				if w.isLive() {
					if w.frame != nil {
						w.frame(now)
					}
					w.RequestPaint()
				}
			}),
			root.ConnectAfterPaint(func(gui.Painter, geometry.Rectangle) {
				if w.isLive() && gui.IsDrawable(w) {
					_ = root.RequestPaint()
				}
			}),
		}
	})
	w.ConnectUnmount(func() {
		w.handles.Disconnect()
		w.handles = nil
	})
	return w
}

func (w *canvasWidget) isLive() bool { return w.live != nil && w.live() }

func (w *canvasWidget) Paint(p gui.Painter) {
	if size := w.Rect().Size; w.Visible() && w.draw != nil && size.Width > 0 && size.Height > 0 {
		w.draw(p, size)
	}
}

func (w *canvasWidget) Snapshot() gui.WidgetInfo {
	info := w.WidgetBase.Snapshot()
	info.Role = gui.RoleImage
	info.Text = w.label
	return info
}

// canvasView declares a drawn surface with no natural size; set its size with
// layout modifiers. live and frame may be nil for static drawings.
type canvasView struct {
	ui.ViewBase[canvasView]
	label string
	draw  gui.DrawFunc
	live  func() bool
	frame func(time.Time)
}

func canvas(label string, draw gui.DrawFunc) *canvasView {
	v := &canvasView{label: label, draw: draw}
	v.Self = v
	return v
}

func (v *canvasView) Live(live func() bool) *canvasView      { v.live = live; return v }
func (v *canvasView) OnFrame(fn func(time.Time)) *canvasView { v.frame = fn; return v }

func (*canvasView) Mount(ui.BuildContext) gui.Widget { return newCanvasWidget() }
func (v *canvasView) Update(_ ui.BuildContext, widget gui.Widget) {
	w := widget.(*canvasWidget)
	w.label, w.draw, w.live, w.frame = v.label, v.draw, v.live, v.frame
	w.RequestPaint()
}
func (*canvasView) Unmount(ui.BuildContext, gui.Widget) {}

func brush(c interface{ RGBA() (r, g, b, a uint32) }) graphics.Color { return graphics.ColorOf(c) }

// drawAmbient washes the window in the album's colors: a pale diagonal
// gradient with two large, heavily blurred glows.
func drawAmbient(p gui.Painter, size geometry.Size, c albumColors) {
	p.FillRect(geometry.Rectangle{Size: size}, graphics.LinearGradient{
		End:        graphics.Point{X: size.Width, Y: size.Height},
		StartColor: brush(mix(c.a, rgb(0xFFFFFF), .86)),
		EndColor:   brush(mix(c.b, rgb(0xFFFFFF), .88)),
	})
	glow := func(x, y, r float32, tone [3]uint8) {
		rect := geometry.Rect(x-r, y-r, 2*r, 2*r)
		p.DrawBoxShadow(rect, geometry.UniformCornerRadius(r), graphics.BoxShadow{
			Color: graphics.RGBA(tone[0], tone[1], tone[2], 92), BlurRadius: r * 1.2,
		})
	}
	h := size.Height
	glow(size.Width*.2, h*.3, h*.32, [3]uint8{c.a.R, c.a.G, c.a.B})
	glow(size.Width*.78, h*.82, h*.3, [3]uint8{c.b.R, c.b.G, c.b.B})
}

// drawRecord paints a vinyl record whose label turns with angle (degrees).
// margin leaves room for the soft colored shadow inside the widget bounds.
func drawRecord(p gui.Painter, size geometry.Size, c albumColors, angle, margin float32) {
	r := min(size.Width, size.Height)/2 - margin
	if r <= 0 {
		return
	}
	center := geometry.Point{X: size.Width / 2, Y: size.Height / 2}
	disc := geometry.Rect(center.X-r, center.Y-r, 2*r, 2*r)
	if margin > 0 {
		// A blur reaches about 1.5 × BlurRadius; offset plus reach stays within
		// margin so the widget's clip never cuts the shadow's edge.
		p.DrawBoxShadow(disc, geometry.UniformCornerRadius(r), graphics.BoxShadow{
			Color: brush(withAlpha(c.b, 110)), Offset: graphics.Point{Y: margin * .18}, BlurRadius: margin * .52,
		})
		p.DrawBoxShadow(disc, geometry.UniformCornerRadius(r), graphics.BoxShadow{
			Color: graphics.RGBA(0, 0, 0, 40), Offset: graphics.Point{Y: margin * .08}, BlurRadius: margin * .2,
		})
	}
	p.FillEllipse(center, r, r, graphics.LinearGradient{
		Start: graphics.Point{X: disc.X, Y: disc.Y}, End: graphics.Point{X: disc.X + disc.Width, Y: disc.Y + disc.Height},
		StartColor: brush(rgb(0x2C2C31)), EndColor: brush(vinylColor),
	})
	label := r * .36
	if r > 40 {
		for g := label + 6; g < r-3; g += 4.5 {
			p.DrawEllipse(center, g, g, .6, graphics.RGBA(255, 255, 255, 9))
		}
	}
	p.DrawEllipse(center, r-.5, r-.5, 1, graphics.RGBA(255, 255, 255, 22))

	// The label's gradient axis and its mark rotate with the record.
	rad := float64(angle) * math.Pi / 180
	dx, dy := label*float32(math.Cos(rad)), label*float32(math.Sin(rad))
	p.FillEllipse(center, label, label, graphics.LinearGradient{
		Start: graphics.Point{X: center.X - dx, Y: center.Y - dy}, End: graphics.Point{X: center.X + dx, Y: center.Y + dy},
		StartColor: brush(c.a), EndColor: brush(c.b),
	})
	p.DrawEllipse(center, label*.72, label*.72, max(1, label*.02), graphics.RGBA(255, 255, 255, 60))
	mark := geometry.Point{X: center.X + dx*.72, Y: center.Y + dy*.72}
	p.FillEllipse(mark, max(1.5, label*.06), max(1.5, label*.06), graphics.RGBA(255, 255, 255, 220))
	hole := max(1.5, label*.09)
	p.FillEllipse(center, hole*1.8, hole*1.8, brush(spindleTone))
	p.FillEllipse(center, hole, hole, brush(vinylColor))
}

// drawSpectrum paints mirrored bars that breathe around the center line.
// energy in [0,1] scales the motion; at zero the bars rest as a flat line.
func drawSpectrum(p gui.Painter, size geometry.Size, c albumColors, bars int, energy float32, t float64) {
	if bars < 2 {
		return
	}
	const gap = 4
	width := (size.Width - gap*float32(bars-1)) / float32(bars)
	if width <= 0 {
		return
	}
	rest := min(width, 4)
	for i := range bars {
		f := float64(i)
		level := .5 + .24*math.Sin(t*2.3+f*.55) + .16*math.Sin(t*3.9-f*1.27) + .1*math.Sin(t*6.7+f*.31)
		envelope := .5 + .5*math.Sin(math.Pi*(f+.5)/float64(bars))
		h := max(rest, float32(level*envelope)*size.Height*energy)
		x := float32(i) * (width + gap)
		rect := geometry.Rect(x, (size.Height-h)/2, width, h)
		p.FillRoundRect(rect, width/2, brush(mix(c.a, c.b, float32(i)/float32(bars-1))))
	}
}

// seekWidget is the playback position: a thin track that thickens and shows
// its knob on hover. Click or drag to move the position.
type seekWidget struct {
	gui.WidgetBase
	m        *music
	hover    bool
	dragging bool
	preview  float64
	handles  signal.Handles
}

func newSeekWidget() *seekWidget {
	w := new(seekWidget)
	w.SetCursor(gui.CursorPointing)
	fraction := func(x float32) float64 {
		width := w.Rect().Width - 2*seekInset
		if width <= 0 {
			return 0
		}
		return min(max(float64((x-seekInset)/width), 0), 1)
	}
	motion := gui.NewMotionEventController()
	motion.ConnectHover(func(hover bool) { w.hover = hover; w.RequestPaint() })
	drag := gui.NewDragEventController()
	drag.SetThreshold(2)
	drag.ConnectBegin(func(_ gui.EventContext, info gui.DragInfo) {
		w.dragging, w.preview = true, fraction(info.Position.X)
		w.RequestPaint()
	})
	drag.ConnectUpdate(func(_ gui.EventContext, info gui.DragInfo) {
		w.preview = fraction(info.Position.X)
		w.RequestPaint()
	})
	drag.ConnectEnd(func(_ gui.EventContext, info gui.DragInfo) {
		w.dragging = false
		if w.m != nil {
			w.m.seek(fraction(info.Position.X))
		}
	})
	drag.ConnectCanceled(func(gui.GestureCancelReason) { w.dragging = false; w.RequestPaint() })
	click := gui.NewClickEventController()
	click.ConnectClicked(func(ctx gui.EventContext, info gui.ClickInfo) {
		ctx.StopPropagation()
		if w.m != nil {
			w.m.seek(fraction(info.Position.X))
		}
	})
	w.AddEventController(motion)
	w.AddEventController(drag)
	w.AddEventController(click)
	w.ConnectMount(func() {
		root := w.Root()
		if root == nil {
			return
		}
		live := func() bool { return w.m != nil && w.m.player.playing && !w.dragging }
		w.handles = signal.Handles{
			root.ConnectFrame(func(time.Time) {
				if live() {
					w.RequestPaint()
				}
			}),
			root.ConnectAfterPaint(func(gui.Painter, geometry.Rectangle) {
				if live() && gui.IsDrawable(w) {
					_ = root.RequestPaint()
				}
			}),
		}
	})
	w.ConnectUnmount(func() {
		w.handles.Disconnect()
		w.handles = nil
		w.dragging = false
	})
	return w
}

// seekInset keeps the enlarged knob inside the widget at either end.
const seekInset = 8

func (w *seekWidget) Paint(p gui.Painter) {
	size := w.Rect().Size
	if w.m == nil || size.Width <= 2*seekInset || size.Height <= 0 {
		return
	}
	c := w.m.colors()
	f := w.m.fraction(time.Now())
	if w.dragging {
		f = w.preview
	}
	active := w.hover || w.dragging
	thickness := float32(4)
	if active {
		thickness = 6
	}
	width := size.Width - 2*seekInset
	y := (size.Height - thickness) / 2
	p.FillRoundRect(geometry.Rect(seekInset, y, width, thickness), thickness/2, brush(withAlpha(inkColor, 26)))
	filled := width * float32(f)
	if filled > 0 {
		p.FillRoundRect(geometry.Rect(seekInset, y, max(filled, thickness), thickness), thickness/2, graphics.LinearGradient{
			Start: graphics.Point{X: seekInset}, End: graphics.Point{X: seekInset + width},
			StartColor: brush(c.a), EndColor: brush(c.b),
		})
	}
	if active {
		knob := geometry.Point{X: seekInset + filled, Y: size.Height / 2}
		const r = 7
		p.DrawBoxShadow(geometry.Rect(knob.X-r, knob.Y-r, 2*r, 2*r), geometry.UniformCornerRadius(r),
			graphics.BoxShadow{Color: graphics.RGBA(0, 0, 0, 50), Offset: graphics.Point{Y: 1}, BlurRadius: 4})
		p.FillEllipse(knob, r, r, brush(rgb(0xFFFFFF)))
	}
}

func (w *seekWidget) Snapshot() gui.WidgetInfo {
	info := w.WidgetBase.Snapshot()
	info.Text = "Playback position"
	return info
}

type seekView struct {
	ui.ViewBase[seekView]
	m *music
}

func seek(m *music) *seekView { v := &seekView{m: m}; v.Self = v; return v }

func (*seekView) Mount(ui.BuildContext) gui.Widget { return newSeekWidget() }
func (v *seekView) Update(_ ui.BuildContext, widget gui.Widget) {
	w := widget.(*seekWidget)
	w.m = v.m
	w.RequestPaint()
}
func (*seekView) Unmount(ui.BuildContext, gui.Widget) {}
