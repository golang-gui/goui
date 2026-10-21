// Native repaint invalidation regression; requires an interactive desktop.
// Run: GOUI_PLAT_PAINTER=software go run ./tests/window/platform/repaint
// macOS runs on the real main thread. No keyboard/clipboard/files are changed.
// Initial: a small native window. Four successive native Paint events must
// arrive without input; each paint requests the next twice. The final task also
// destroys a second window with a queued repaint and verifies no later Paint
// callback reaches it. Success prints VERIFIED and exits automatically. A lost
// invalidation, callback after destruction, or manual close fails within 4 s.
// Repeat with OpenGL/Direct2D as appropriate; restarting resets the scenario.
package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "NOT VERIFIED:", err)
		os.Exit(1)
	}
}

func run() error {
	p, err := platform.NewPlatform(platform.DefaultName(), "org.golanggui.RepaintRegression")
	if err != nil {
		return err
	}
	defer p.Destroy()
	loop, err := p.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	var win platform.Window
	var painter graphics.Painter
	var size events.SizeEvent
	var paints int
	var done, latePaint bool
	var failure error
	win, err = p.NewWindow(geometry.Size{Width: 320, Height: 220}, func(e events.Event) {
		switch e := e.(type) {
		case events.SizeEvent:
			size = e
		case events.CloseEvent:
			loop.Quit()
		case events.PaintEvent:
			if painter == nil {
				return
			}
			paints++
			painter.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
			painter.Clear(graphics.RGB(byte(40*paints), 90, 170))
			painter.End()
			if paints < 4 {
				failure = win.RequestPaint()
				if failure == nil {
					failure = win.RequestPaint()
				}
			} else if paints == 4 {
				loop.Post(func() {
					ghost, err := p.NewWindow(geometry.Size{Width: 40, Height: 40}, func(e events.Event) {
						if _, paint := e.(events.PaintEvent); paint && done {
							latePaint = true
						}
					}, platform.WindowOptions{})
					if err != nil {
						failure = err
						loop.Quit()
						return
					}
					failure = ghost.Show()
					if failure == nil {
						failure = ghost.RequestPaint()
					}
					ghost.Destroy()
					done = true
					time.AfterFunc(100*time.Millisecond, func() { loop.Post(loop.Quit) })
				})
			}
		}
	}, platform.WindowOptions{})
	if err != nil {
		return err
	}
	defer win.Destroy()
	painter, err = p.NewPainter(win)
	if err != nil {
		return err
	}
	defer painter.Destroy()
	if err := win.Show(); err != nil {
		return err
	}
	timer := time.AfterFunc(4*time.Second, func() { loop.Post(loop.Quit) })
	defer timer.Stop()
	loop.Run()
	if failure != nil {
		return failure
	}
	if paints < 4 || !done || latePaint {
		return fmt.Errorf("paints=%d completed=%v callback after destruction=%v", paints, done, latePaint)
	}
	fmt.Printf("VERIFIED OS=%s painter=%s scale=%g paints=%d\n", runtime.GOOS, painter.Name(), size.PixelWidth/size.Width, paints)
	return nil
}
