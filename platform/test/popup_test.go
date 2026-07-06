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
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

// TestPopup creates a borderless popup on an owner window, positions and sizes
// it (owner-window-local coordinates), shows it, and verifies it receives a
// paint event and can be drawn to.
func TestPopup(t *testing.T) {
	skipWithoutDisplay(t)

	plat, err := getPlatform()
	if err != nil {
		t.Fatal(err)
	}

	var (
		eventLoop platform.EventLoop
		owner     platform.Window
		popup     platform.Popup
		handle    uintptr
		painted   bool
		drawErr   error
	)

	runOnMainThread(func() {
		eventLoop, err = plat.NewEventLoop()
		if err != nil {
			return
		}

		owner, err = plat.NewWindow(800, 600, func(platform.Event) {})
		if err != nil {
			return
		}
		if err = owner.Show(); err != nil {
			return
		}

		var once sync.Once
		popup, err = plat.NewPopup(owner, 120, 80, func(event platform.Event) {
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
		})
		if err != nil {
			return
		}
		handle = popup.NativeHandle()
		popup.SetSize(120, 80)
		popup.SetPosition(20, 20)
		err = popup.Show()
	})

	if eventLoop != nil {
		defer runOnMainThread(eventLoop.Destroy)
	}
	if err != nil {
		runOnMainThread(func() {
			if popup != nil {
				popup.Destroy()
			}
			if owner != nil {
				owner.Destroy()
			}
		})
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

	runOnMainThread(func() {
		if popup != nil {
			popup.Destroy()
		}
		if owner != nil {
			owner.Destroy()
		}
	})

	if handle == 0 {
		t.Fatal("popup NativeHandle is 0")
	}
	if !painted {
		t.Fatal("popup did not receive a paint event")
	}
	if drawErr != nil {
		t.Fatal(drawErr)
	}
}

// TestPopupPainter verifies a Painter can be created for a Popup (not only a
// Window) and drives one paint frame through the popup's native surface. This
// exercises the widened NewPainter(Surface) contract end-to-end.
func TestPopupPainter(t *testing.T) {
	skipWithoutDisplay(t)

	plat, err := getPlatform()
	if err != nil {
		t.Fatal(err)
	}

	var (
		eventLoop platform.EventLoop
		owner     platform.Window
		popup     platform.Popup
		painter   graphics.Painter
		typo      typography.Context
		painted   bool
	)

	runOnMainThread(func() {
		eventLoop, err = plat.NewEventLoop()
		if err != nil {
			return
		}
		typo, err = plat.NewTypography()
		if err != nil {
			return
		}

		owner, err = plat.NewWindow(800, 600, func(platform.Event) {})
		if err != nil {
			return
		}
		if err = owner.Show(); err != nil {
			return
		}

		var once sync.Once
		popup, err = plat.NewPopup(owner, 120, 80, func(event platform.Event) {
			if _, ok := event.(events.PaintEvent); ok {
				once.Do(func() {
					painter.Begin(120, 80, 1)
					painter.Clear(graphics.RGB(0x30, 0x30, 0x30))
					painter.End()
					painted = true
					eventLoop.Quit()
				})
			}
		})
		if err != nil {
			return
		}

		// The point of this test: NewPainter accepts a Popup, not only a Window.
		painter, err = plat.NewPainter(popup, typo)
		if err != nil {
			return
		}

		popup.SetSize(120, 80)
		popup.SetPosition(20, 20)
		err = popup.Show()
	})

	if eventLoop != nil {
		defer runOnMainThread(eventLoop.Destroy)
	}
	if err != nil {
		runOnMainThread(func() {
			if painter != nil {
				painter.Destroy()
			}
			if popup != nil {
				popup.Destroy()
			}
			if owner != nil {
				owner.Destroy()
			}
		})
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

	runOnMainThread(func() {
		if painter != nil {
			painter.Destroy()
		}
		if popup != nil {
			popup.Destroy()
		}
		if owner != nil {
			owner.Destroy()
		}
	})

	if !painted {
		t.Fatal("popup painter did not run a paint frame")
	}
}
