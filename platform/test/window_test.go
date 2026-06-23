package test

import (
	"image"
	"image/color"
	"image/draw"
	"sync"
	"testing"
	"time"

	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

func TestWindow(t *testing.T) {
	skipWithoutDisplay(t)

	plat, err := getPlatform()
	if err != nil {
		t.Fatal(err)
	}

	var (
		eventLoop platform.EventLoop
		window    platform.Window
		paintErr  error
		painted   bool
		destroyed bool
		title     string
	)

	runOnMainThread(func() {
		eventLoop, err = plat.NewEventLoop()
		if err != nil {
			return
		}

		var finishPaint sync.Once
		window, err = plat.NewWindow(func(event platform.Event) {
			paintEvent, ok := event.(*events.PaintEvent)
			if !ok {
				return
			}

			finishPaint.Do(func() {
				img := image.NewRGBA(image.Rect(0, 0, 64, 64))
				draw.Draw(img, img.Bounds(), &image.Uniform{
					C: color.RGBA{R: 0x28, G: 0x78, B: 0xd4, A: 0xff},
				}, image.Point{}, draw.Src)
				paintErr = paintEvent.Window.Draw(img)
				painted = true

				eventLoop.Post(func() {
					paintEvent.Window.Destroy()
					destroyed = true
					eventLoop.Quit()
				})
			})
		})
		if err != nil {
			return
		}

		if err = window.SetTitle("goui platform test"); err != nil {
			return
		}
		title = window.Title()
		err = window.Show()
	})
	if eventLoop != nil {
		defer func() {
			runOnMainThread(eventLoop.Destroy)
		}()
	}
	if err != nil {
		if window != nil {
			runOnMainThread(window.Destroy)
		}
		t.Fatal(err)
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

	runOnMainThread(eventLoop.Run)
	close(runFinished)

	if !destroyed {
		runOnMainThread(window.Destroy)
	}
	if title != "goui platform test" {
		t.Fatalf("unexpected window title: %q", title)
	}
	if !painted {
		t.Fatal("window did not receive a paint event")
	}
	if paintErr != nil {
		t.Fatal(paintErr)
	}
}
