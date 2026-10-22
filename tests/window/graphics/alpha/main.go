// Premultiplied-color window checks.
//
// Environment: Linux, Windows or macOS desktop. Record the backend, desktop/WM
// and scale. Run: go run ./tests/window/graphics/alpha
// Use GOUI_PLAT_PAINTER=software or opengl (also direct2d on Windows), and
// GOUI_PLAT_SCALE=1 or 2. -transparent needs desktop composition.
// Initially the background is white. Each row compares native brushes/text
// on the left with reference images/text on the right.
// Steps and expected results:
//  1. Compare Fill, Stroke, Gradient and Shadow. Interior pink fills and shadows
//     match the reference image, strokes match the color strips, and gradient
//     endpoints and midpoint have no extra darkening.
//  2. Compare the two MMMM groups in Text: default translucent and range-specific
//     colors have equal color/opacity. Half/Zero/Half keeps the middle group's
//     space while making it completely invisible.
//  3. Repeat at 1x and 2x. Minor edge rasterization differences are allowed;
//     interior colors must remain unchanged.
//  4. With -transparent, click through reference, translucent Clear, zero-clear
//     plus full-window FillRect, zero-clear plus full-window DrawImage, then
//     reference again. The three solid-color modes must look identical and
//     reveal the desktop equally. The log identifies the active mode.
//
// Platform differences: transparent mode is undecorated and supplies no simulated
// movement/resizing; normal mode uses native decorations.
// Side effects: no system settings, clipboard or files are changed.
// Reset: restart. Exit: Esc, or close the normal decorated window.
package main

import (
	"flag"
	"image"
	"image/color"
	"image/draw"
	"log"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	transparent := flag.Bool("transparent", false, "transparent frameless window; Esc exits")
	flag.Parse()
	if err := run(*transparent); err != nil {
		log.Fatal(err)
	}
}

func run(transparent bool) error {
	plat, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.AlphaTest")
	if err != nil {
		return err
	}
	defer plat.Destroy()
	loop, err := plat.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	ty, err := plat.NewTypography()
	if err != nil {
		return err
	}
	defer ty.Destroy()
	var labels []typography.TextLayout
	defer func() {
		for _, label := range labels {
			label.Destroy()
		}
	}()
	ink := color.NRGBA{R: 255, A: 128}
	for _, text := range []string{"Fill", "Stroke", "Gradient", "Shadow", "Text", "Half/Zero/Half", "MMMM", "MMMM", "MMMMMMMMMMMM"} {
		fg := color.Color(color.Black)
		if len(labels) >= 6 {
			fg = ink
		}
		layout, err := ty.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "sans", Size: 20}, TextColor: fg}, 400, 50)
		if err != nil {
			return err
		}
		labels = append(labels, layout)
	}
	labels[7].SetTextColor(0, 4, ink)
	labels[8].SetTextColor(4, 4, color.Transparent)
	var win platform.Window
	var p graphics.Painter
	var swatch, gradient graphics.Image
	var size events.SizeEvent
	mode := 0
	c := graphics.ColorOf(ink)
	paint := func() {
		if p == nil || size.Width <= 0 || size.Height <= 0 {
			return
		}
		p.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
		defer p.End()
		full := graphics.Rect(0, 0, size.Width, size.Height)
		if mode != 0 {
			p.Clear(graphics.Color{})
			switch mode {
			case 1:
				p.Clear(c)
			case 2:
				p.FillRect(full, c)
			case 3:
				p.DrawImage(full, swatch)
			}
			return
		}
		p.Clear(graphics.RGB(255, 255, 255))
		for i := 0; i < 6; i++ {
			p.DrawTextLayout(graphics.Point{X: 16, Y: float32(16 + i*62)}, labels[i])
		}
		p.FillRect(graphics.Rect(190, 16, 120, 40), c)
		p.DrawImage(graphics.Rect(340, 16, 120, 40), swatch)
		p.DrawRect(graphics.Rect(194, 82, 112, 32), 8, c)
		p.DrawImage(graphics.Rect(340, 78, 120, 8), swatch)
		p.FillRect(graphics.Rect(190, 140, 120, 40), graphics.LinearGradient{Start: graphics.Point{X: 190}, End: graphics.Point{X: 310}, StartColor: graphics.RGBA(255, 0, 0, 64), EndColor: graphics.RGBA(0, 0, 255, 192)})
		p.DrawImage(graphics.Rect(340, 140, 120, 40), gradient)
		p.DrawBoxShadow(graphics.Rect(190, 202, 120, 40), 4, graphics.BoxShadow{Color: c, BlurRadius: 4})
		p.DrawImage(graphics.Rect(340, 202, 120, 40), swatch)
		p.DrawTextLayout(graphics.Point{X: 190, Y: 264}, labels[6])
		p.DrawTextLayout(graphics.Point{X: 340, Y: 264}, labels[7])
		p.DrawTextLayout(graphics.Point{X: 190, Y: 326}, labels[8])
	}
	chrome := platform.WindowChromeNative
	if transparent {
		chrome = platform.WindowChromeNone
	}
	win, err = plat.NewWindow(geometry.Size{Width: 600, Height: 410}, func(e platform.Event) {
		switch e := e.(type) {
		case events.SizeEvent:
			size = e
		case events.CloseEvent:
			loop.Quit()
		case events.KeyEvent:
			if e.Key == events.KeyEscape {
				loop.Quit()
			}
		case events.PointerEvent:
			if transparent && e.EventType == events.PointerDown && win != nil {
				mode = (mode + 1) % 4
				log.Printf("mode=%d (0=compare, 1=Clear, 2=FillRect, 3=Image)", mode)
				if err := win.RequestPaint(); err != nil {
					log.Print(err)
				}
			}
		case events.PaintEvent:
			paint()
		}
	}, platform.WindowOptions{Chrome: chrome, Transparent: transparent})
	if err != nil {
		return err
	}
	defer win.Destroy()
	if err = win.SetTitle("GOUI premultiplied alpha — Esc exits"); err != nil {
		return err
	}
	p, err = plat.NewPainter(win)
	if err != nil {
		return err
	}
	defer p.Destroy()
	src := image.NewNRGBA(image.Rect(0, 0, 120, 40))
	draw.Draw(src, src.Bounds(), image.NewUniform(ink), image.Point{}, draw.Src)
	swatch, err = p.NewImage(src)
	if err != nil {
		return err
	}
	defer swatch.Destroy()
	ref := image.NewRGBA(src.Bounds())
	for y := 0; y < 40; y++ {
		for x := 0; x < 120; x++ {
			t := float64(x) + .5
			t /= 120
			ref.SetRGBA(x, y, color.RGBA{R: byte(64*(1-t) + .5), B: byte(192*t + .5), A: byte(64*(1-t) + 192*t + .5)})
		}
	}
	gradient, err = p.NewImage(ref)
	if err != nil {
		return err
	}
	defer gradient.Destroy()
	log.Printf("Painter=%s transparent=%v; Esc exits", p.Name(), transparent)
	if err = win.Show(); err != nil {
		return err
	}
	loop.Run()
	return nil
}
