package main

import (
	"image/color"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/ui"
)

// chartWidget paints with the window's own Painter. The chart slides on every
// frame, so an offscreen DrawingArea would rasterize and upload each frame.
type chartWidget struct {
	gui.WidgetBase
	label string
	draw  gui.DrawFunc
}

func (w *chartWidget) Paint(p gui.Painter) {
	if size := w.Rect().Size; w.Visible() && w.draw != nil && size.Width > 0 && size.Height > 0 {
		w.draw(p, size)
	}
}

func (w *chartWidget) Snapshot() gui.WidgetInfo {
	info := w.WidgetBase.Snapshot()
	info.Role = gui.RoleImage
	info.Text = w.label
	return info
}

// chartView declares a chart with no natural size; set its layout constraints.
type chartView struct {
	ui.ViewBase[chartView]
	label string
	draw  gui.DrawFunc
}

func newChart(label string, draw gui.DrawFunc) *chartView {
	v := &chartView{label: label, draw: draw}
	v.Self = v
	return v
}

func (*chartView) Mount(ui.BuildContext) gui.Widget { return new(chartWidget) }
func (v *chartView) Update(_ ui.BuildContext, widget gui.Widget) {
	w := widget.(*chartWidget)
	w.label, w.draw = v.label, v.draw
	w.RequestPaint()
}
func (*chartView) Unmount(ui.BuildContext, gui.Widget) {}

// series describes one line on a chart.
type series struct {
	values []float32
	color  color.NRGBA
	fill   bool
}

// drawSeries plots samples oldest first. scroll in [0,1] slides the newest
// sample in from the right edge, so the line moves continuously between ticks.
func drawSeries(p gui.Painter, size geometry.Size, ceiling, scroll float32, s series) {
	n := len(s.values)
	if n < 2 || size.Width <= 0 || size.Height <= 0 || ceiling <= 0 {
		return
	}
	const top, stroke = 6, 1.5
	step := size.Width / float32(historyLength-2)
	point := func(i int) geometry.Point {
		x := size.Width - float32(n-1-i)*step + (1-scroll)*step
		y := size.Height - stroke - clamp(s.values[i]/ceiling, 0, 1)*(size.Height-top-stroke)
		return geometry.Point{X: x, Y: y}
	}
	first := point(0)
	line := graphics.MoveTo(first.X, first.Y)
	prev := first
	for i := 1; i < n; i++ {
		next := point(i)
		// Horizontal tangents at every sample give a smooth curve that never
		// overshoots the data, unlike a general spline.
		mid := (prev.X + next.X) / 2
		line = line.BezierTo(mid, prev.Y, mid, next.Y, next.X, next.Y)
		prev = next
	}
	if s.fill {
		area := line.LineTo(prev.X, size.Height).LineTo(first.X, size.Height).Close()
		p.FillPath(area, graphics.ColorOf(withAlpha(s.color, 26)))
	}
	p.DrawPath(line, stroke, graphics.ColorOf(s.color))
}

// drawGrid draws quiet horizontal guides at quarters of the chart height.
func drawGrid(p gui.Painter, size geometry.Size, c color.NRGBA) {
	brush := graphics.ColorOf(c)
	for i := 1; i <= 3; i++ {
		y := float32(int(size.Height*float32(i)/4)) + .5
		p.DrawLine(geometry.Point{Y: y}, geometry.Point{X: size.Width, Y: y}, 1, brush)
	}
}

// drawLatest marks where the line meets the right edge with a dot ringed by
// the card surface. It follows the same curve segment as drawSeries.
func drawLatest(p gui.Painter, size geometry.Size, ceiling, scroll float32, values []float32, dot, ring color.NRGBA) {
	n := len(values)
	if n < 2 || size.Width <= 0 || size.Height <= 0 || ceiling <= 0 {
		return
	}
	const top, stroke, radius = 6, 1.5, 5
	y := func(v float32) float32 { return size.Height - stroke - clamp(v/ceiling, 0, 1)*(size.Height-top-stroke) }
	step := size.Width / float32(historyLength-2)
	x0 := size.Width - step + (1-scroll)*step
	x := size.Width - radius
	// Solve the segment's Bezier x(u) for the dot position, then evaluate y(u).
	target := clamp((x-x0)/step, 0, 1)
	lo, hi := float32(0), float32(1)
	for range 16 {
		u := (lo + hi) / 2
		if 1.5*u-1.5*u*u+u*u*u < target {
			lo = u
		} else {
			hi = u
		}
	}
	u := (lo + hi) / 2
	y0, y1 := y(values[n-2]), y(values[n-1])
	center := geometry.Point{X: x, Y: y0 + (y1-y0)*(3*u*u-2*u*u*u)}
	p.FillEllipse(center, radius, radius, graphics.ColorOf(ring))
	p.FillEllipse(center, 3, 3, graphics.ColorOf(dot))
}
