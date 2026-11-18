package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"sync"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

func checkWindow(plat platform.Platform) {
	var err error

	var (
		eventLoop platform.EventLoop
		window    platform.Window
		paintErr  error
		painted   bool
		closeSent int
		closeSeen int
		destroyed bool
		title     string
	)

	func() {
		eventLoop, err = plat.NewEventLoop()
		if err != nil {
			return
		}

		var finishPaint sync.Once
		window, err = plat.NewWindow(geometry.Size{Width: 800, Height: 600}, func(event platform.Event) {
			switch event.(type) {
			case events.PaintEvent:
				finishPaint.Do(func() {
					img := image.NewRGBA(image.Rect(0, 0, 64, 64))
					draw.Draw(img, img.Bounds(), &image.Uniform{
						C: color.RGBA{R: 0x28, G: 0x78, B: 0xd4, A: 0xff},
					}, image.Point{}, draw.Src)
					paintErr = window.Draw(img)
					painted = true

					eventLoop.Post(func() {
						closeSent++
						if requestErr := window.(platform.WindowManagement).RequestClose(); requestErr != nil {
							paintErr = requestErr
							window.Destroy()
							destroyed = true
							eventLoop.Quit()
						}
					})
				})
			case events.CloseEvent:
				closeSeen++
				if closeSeen == 1 {
					eventLoop.Post(func() {
						closeSent++
						if requestErr := window.(platform.WindowManagement).RequestClose(); requestErr != nil {
							paintErr = requestErr
							window.Destroy()
							destroyed = true
							eventLoop.Quit()
						}
					})
					return
				}
				window.Destroy()
				destroyed = true
				eventLoop.Quit()
			}
		}, platform.WindowOptions{})
		if err != nil {
			return
		}

		if err = window.(platform.WindowManagement).SetTitle("goui platform test"); err != nil {
			return
		}
		title = window.(platform.WindowManagement).Title()
		err = window.(platform.WindowManagement).Show()
	}()
	if eventLoop != nil {
		defer func() {
			eventLoop.Destroy()
		}()
	}
	if err != nil {
		if window != nil {
			window.Destroy()
		}
		panic(err)
	}

	runFinished := make(chan struct{})
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	go func() {
		select {
		case <-timeout.C:
			eventLoop.Quit()
		case <-runFinished:
		}
	}()

	eventLoop.Run()
	close(runFinished)

	if !destroyed {
		window.Destroy()
	}
	if title != "goui platform test" {
		panic(fmt.Errorf("unexpected window title: %q", title))
	}
	if !painted {
		panic("window did not receive a paint event")
	}
	if closeSent != 2 {
		panic(fmt.Errorf("expected 2 window close requests, got %d", closeSent))
	}
	if closeSeen != 2 {
		panic(fmt.Errorf("expected 2 close request events, got %d", closeSeen))
	}
	if !destroyed {
		panic("window was not destroyed after the close request")
	}
	if paintErr != nil {
		panic(paintErr)
	}
}

func checkWindowRequestPaint(plat platform.Platform) {
	var err error

	var (
		eventLoop         platform.EventLoop
		window            platform.Window
		paintErr          error
		destroyed         bool
		requestScheduled  bool
		requestSent       bool
		paintAfterRequest int
	)

	func() {
		eventLoop, err = plat.NewEventLoop()
		if err != nil {
			return
		}

		window, err = plat.NewWindow(geometry.Size{Width: 800, Height: 600}, func(event platform.Event) {
			switch event.(type) {
			case events.PaintEvent:
				img := image.NewRGBA(image.Rect(0, 0, 64, 64))
				draw.Draw(img, img.Bounds(), &image.Uniform{
					C: color.RGBA{R: 0x20, G: 0x90, B: 0x50, A: 0xff},
				}, image.Point{}, draw.Src)
				if drawErr := window.Draw(img); drawErr != nil {
					paintErr = drawErr
					window.Destroy()
					destroyed = true
					eventLoop.Quit()
					return
				}

				if requestSent {
					paintAfterRequest++
					window.Destroy()
					destroyed = true
					eventLoop.Quit()
					return
				}

				if !requestScheduled {
					requestScheduled = true
					eventLoop.Post(func() {
						if requestErr := window.RequestPaint(); requestErr != nil {
							paintErr = requestErr
							window.Destroy()
							destroyed = true
							eventLoop.Quit()
							return
						}
						requestSent = true
					})
				}

			case events.CloseEvent:
				window.Destroy()
				destroyed = true
				eventLoop.Quit()
			}
		}, platform.WindowOptions{})
		if err != nil {
			return
		}

		if err = window.(platform.WindowManagement).SetTitle("goui request paint test"); err != nil {
			return
		}
		err = window.(platform.WindowManagement).Show()
	}()
	if eventLoop != nil {
		defer func() {
			eventLoop.Destroy()
		}()
	}
	if err != nil {
		if window != nil {
			window.Destroy()
		}
		panic(err)
	}

	runFinished := make(chan struct{})
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	go func() {
		select {
		case <-timeout.C:
			eventLoop.Quit()
		case <-runFinished:
		}
	}()

	eventLoop.Run()
	close(runFinished)

	if !destroyed {
		window.Destroy()
	}
	if paintErr != nil {
		panic(paintErr)
	}
	if !requestScheduled {
		panic("window did not receive an initial paint event")
	}
	if !requestSent {
		panic("RequestPaint was not sent")
	}
	if paintAfterRequest == 0 {
		panic("window did not receive a paint event after RequestPaint")
	}
}
