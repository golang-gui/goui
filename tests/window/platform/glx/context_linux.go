package main

import (
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/opengl"
)

func checkGLXMakeCurrentReuse(plat platform.Platform) {
	var testErr error
	func() {
		win, err := plat.NewWindow(geometry.Size{Width: 320, Height: 200}, func(platform.Event) {}, platform.WindowOptions{})
		if err != nil {
			testErr = err
			return
		}
		defer win.Destroy()
		ctx, err := opengl.NewContext(win, nil, opengl.DefaultConfig)
		if err != nil {
			testErr = err
			return
		}
		defer ctx.Destroy()
		defer ctx.ClearCurrent()
		for frame := 0; frame < 16; frame++ {
			for bind := 0; bind < 2; bind++ {
				if err := ctx.MakeCurrent(); err != nil {
					testErr = fmt.Errorf("frame %d, bind %d: %w", frame, bind, err)
					return
				}
				runtime.Gosched()
			}
			if err := ctx.ClearCurrent(); err != nil {
				testErr = err
				return
			}
		}
	}()
	if testErr != nil {
		panic(testErr)
	}
}

func checkGLXWindowMaximizePaint(plat platform.Platform) {
	const cycles = 12
	const framesPerState = 3
	targets := []platform.WindowState{platform.WindowStateNormal}
	for range cycles {
		targets = append(targets, platform.WindowStateMaximized, platform.WindowStateNormal)
	}
	var (
		loop         platform.EventLoop
		win          platform.Window
		desktop      platform.DesktopWindow
		painter      graphics.Painter
		size         events.SizeEvent
		maxSize      events.SizeEvent
		testErr      error
		step         int
		frames       int
		stableFrames int
	)
	defer func() {
		if painter != nil {
			painter.Destroy()
		}
		if win != nil {
			win.Destroy()
		}
		if loop != nil {
			loop.Destroy()
		}
	}()
	func() {
		loop, testErr = plat.NewEventLoop()
		if testErr != nil {
			return
		}
		win, testErr = plat.NewWindow(geometry.Size{Width: 320, Height: 200}, func(event platform.Event) {
			switch event := event.(type) {
			case events.SizeEvent:
				size = event
			case events.StateEvent:
				if win != nil {
					testErr = win.RequestPaint()
				}
			case events.PaintEvent:
				if painter == nil || step == len(targets) || testErr != nil ||
					size.Width <= 0 || size.PixelWidth <= 0 || size.PixelHeight <= 0 {
					return
				}
				painter.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
				runtime.Gosched()
				painter.Clear(graphics.RGB(180, 180, 180))
				painter.FillRect(graphics.Rect(10, 10, 80, 50), graphics.RGB(40, 120, 210))
				painter.End()
				frames++
				if desktop.State() != targets[step] {
					stableFrames = 0
					return
				}
				stableFrames++
				if stableFrames < framesPerState {
					testErr = win.RequestPaint()
					return
				}
				if targets[step] == platform.WindowStateMaximized {
					maxSize = size
				}
				step++
				stableFrames = 0
				if step == len(targets) {
					loop.Quit()
					return
				}
				target := targets[step]
				loop.Post(func() {
					testErr = desktop.RequestState(target)
					if testErr != nil {
						loop.Quit()
					}
				})
			}
		}, platform.WindowOptions{})
		if testErr != nil {
			return
		}
		desktop = win.(platform.DesktopWindow)
		painter, testErr = opengl.NewPainter(win)
		if testErr != nil {
			return
		}
		if testErr = win.(platform.WindowManagement).SetTitle("goui GLX maximize test"); testErr != nil {
			return
		}
		testErr = win.(platform.WindowManagement).Show()
	}()
	if testErr != nil {
		panic(testErr)
	}
	timer := time.AfterFunc(12*time.Second, loop.Quit)
	loop.Run()
	timer.Stop()
	if errors.Is(testErr, platform.ErrUnsupported) {
		panic(fmt.Errorf("unsupported native transition: %v", testErr))
	}
	if testErr != nil {
		panic(testErr)
	}
	if step != len(targets) {
		panic(fmt.Errorf("stalled at transition %d/%d after %d frames", step, len(targets), frames))
	}
	if maxSize.Width <= 320 || maxSize.Height <= 200 {
		panic(fmt.Errorf("maximization did not enlarge the rendered surface: %+v", maxSize))
	}
	fmt.Printf("%d maximize/normal cycles, %d rendered frames; maximized %.0fx%.0f DIP / %.0fx%.0f pixels\n",
		cycles, frames, maxSize.Width, maxSize.Height, maxSize.PixelWidth, maxSize.PixelHeight)
}
