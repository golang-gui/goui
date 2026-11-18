package main

import (
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

// 绘制可辨认的原生 Surface，不引入 GUI 层事件处理。
type nativeCanvas struct {
	painter graphics.Painter
	size    events.SizeEvent
	color   graphics.Color
}

func (c *nativeCanvas) init(p platform.Platform, w platform.Window, source bool) error {
	var err error
	c.painter, err = p.NewPainter(w)
	if err != nil {
		return err
	}
	c.color = graphics.RGB(40, 150, 90)
	if source {
		c.color = graphics.RGB(40, 100, 200)
	}
	fmt.Printf("actual painter=%s; drag in the colored client area\n", c.painter.Name())
	return nil
}

func (c *nativeCanvas) event(event events.Event) {
	switch e := event.(type) {
	case events.SizeEvent:
		c.size = e
		if e.Width > 0 {
			fmt.Printf("client=%gx%g DIP; scale=%g\n", e.Width, e.Height, e.PixelWidth/e.Width)
		}
	case events.PaintEvent:
		s := c.size
		if c.painter == nil || s.Width <= 0 || s.Height <= 0 || s.PixelWidth <= 0 || s.PixelHeight <= 0 {
			return
		}
		c.painter.Begin(s.PixelWidth, s.PixelHeight, s.PixelWidth/s.Width)
		c.painter.Clear(graphics.RGB(240, 240, 240))
		c.painter.FillRoundRect(geometry.Rect(16, 16, s.Width-32, s.Height-32), 12, c.color)
		c.painter.End()
	}
}
