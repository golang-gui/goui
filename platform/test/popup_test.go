package test

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"sync"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

// popupOwner 只转发基础宿主，不向 Popup 暴露窗口管理接口。
type popupOwner struct{ platform.Window }

// TestPopup 验证不具备 WindowManagement 的 owner 也能创建、定位、显示和绘制 Popup。
// 使用真实窗口的基础接口视图，避免把原生承载能力与主动管理权限混为一谈。
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
		size      events.SizeEvent
		painted   bool
	)

	runOnMainThread(func() {
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

		// The point of this test: NewPainter accepts a Popup, not only a Window.
		painter, err = plat.NewPainter(popup)
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
