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
	"github.com/golang-gui/goui/platform/graphics"
)

type popupOwner struct{ platform.Window }

func checkPopup(plat platform.Platform) {
	var err error

	var (
		eventLoop platform.EventLoop
		owner     platform.Window
		popup     platform.Popup
		handle    uintptr
		painted   bool
		drawErr   error
	)

	func() {
		eventLoop, err = plat.NewEventLoop()
		if err != nil {
			return
		}

		owner, err = plat.NewWindow(geometry.Size{Width: 800, Height: 600}, func(platform.Event) {}, platform.WindowOptions{})
		if err != nil {
			return
		}
		if err = owner.(platform.WindowManagement).Show(); err != nil {
			return
		}

		var once sync.Once
		baseOwner := popupOwner{owner}
		if _, managed := any(baseOwner).(platform.WindowManagement); managed {
			err = fmt.Errorf("Popup owner 不应暴露窗口管理能力")
			return
		}
		popup, err = plat.NewPopup(baseOwner, geometry.Size{Width: 120, Height: 80}, func(event platform.Event) {
			if _, ok := event.(events.PaintEvent); ok {
				once.Do(func() {
					img := image.NewRGBA(image.Rect(0, 0, 120, 80))
					draw.Draw(img, img.Bounds(), &image.Uniform{
						C: color.RGBA{R: 0x30, G: 0x30, B: 0x30, A: 0xff},
					}, image.Point{}, draw.Src)
					drawErr = popup.Draw(img)
					painted = true
					eventLoop.Quit()
				})
			}
		}, platform.PopupOptions{})
		if err != nil {
			return
		}
		handle = popup.NativeHandle()
		popup.SetSize(120, 80)
		popup.SetPosition(20, 20)
		err = popup.Show()
	}()

	if eventLoop != nil {
		defer eventLoop.Destroy()
	}
	if err != nil {
		func() {
			if popup != nil {
				popup.Destroy()
			}
			if owner != nil {
				owner.Destroy()
			}
		}()
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

	func() {
		if popup != nil {
			popup.Destroy()
		}
		if owner != nil {
			owner.Destroy()
		}
	}()

	if handle == 0 {
		panic("popup NativeHandle is 0")
	}
	if !painted {
		panic("popup did not receive a paint event")
	}
	if drawErr != nil {
		panic(drawErr)
	}
}

func checkPopupPainter(plat platform.Platform) {
	var err error

	var (
		eventLoop platform.EventLoop
		owner     platform.Window
		popup     platform.Popup
		painter   graphics.Painter
		size      events.SizeEvent
		painted   bool
	)

	func() {
		eventLoop, err = plat.NewEventLoop()
		if err != nil {
			return
		}
		owner, err = plat.NewWindow(geometry.Size{Width: 800, Height: 600}, func(platform.Event) {}, platform.WindowOptions{})
		if err != nil {
			return
		}
		if err = owner.(platform.WindowManagement).Show(); err != nil {
			return
		}

		var once sync.Once
		popup, err = plat.NewPopup(owner, geometry.Size{Width: 120, Height: 80}, func(event platform.Event) {
			if event, ok := event.(events.SizeEvent); ok {
				size = event
			}
			if _, ok := event.(events.PaintEvent); ok {
				once.Do(func() {
					painter.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
					painter.Clear(graphics.RGB(0x30, 0x30, 0x30))
					painter.End()
					painted = true
					eventLoop.Quit()
				})
			}
		}, platform.PopupOptions{})
		if err != nil {
			return
		}

		painter, err = plat.NewPainter(popup)
		if err != nil {
			return
		}

		popup.SetSize(120, 80)
		popup.SetPosition(20, 20)
		err = popup.Show()
	}()

	if eventLoop != nil {
		defer eventLoop.Destroy()
	}
	if err != nil {
		func() {
			if painter != nil {
				painter.Destroy()
			}
			if popup != nil {
				popup.Destroy()
			}
			if owner != nil {
				owner.Destroy()
			}
		}()
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

	func() {
		if painter != nil {
			painter.Destroy()
		}
		if popup != nil {
			popup.Destroy()
		}
		if owner != nil {
			owner.Destroy()
		}
	}()

	if !painted {
		panic("popup painter did not run a paint frame")
	}
}
