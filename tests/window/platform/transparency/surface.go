package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

func checkTransparentSurfaceCreation(plat platform.Platform) {
	var err error
	func() {
		owner, createErr := plat.NewWindow(geometry.Size{Width: 160, Height: 120}, func(platform.Event) {}, platform.WindowOptions{})
		if createErr != nil {
			err = createErr
			return
		}
		defer owner.Destroy()
		if owner.Transparent() {
			err = fmt.Errorf("default window is transparent")
			return
		}
		win, windowErr := plat.NewWindow(geometry.Size{Width: 120, Height: 80}, func(platform.Event) {},
			platform.WindowOptions{Chrome: platform.WindowChromeNone, Transparent: true})
		popup, popupErr := plat.NewPopup(owner, geometry.Size{Width: 120, Height: 80}, func(platform.Event) {}, platform.PopupOptions{Transparent: true})
		if win != nil {
			defer win.Destroy()
		}
		if popup != nil {
			defer popup.Destroy()
		}
		if windowErr != nil || popupErr != nil {
			unavailable := func(e error) bool {
				return errors.Is(e, platform.ErrUnsupported) || errors.Is(e, platform.ErrUnavailable)
			}
			if win != nil || popup != nil || !unavailable(windowErr) || !unavailable(popupErr) {
				err = fmt.Errorf("window=%v popup=%v", windowErr, popupErr)
			}
			return
		}
		if !win.Transparent() || !popup.Transparent() {
			err = fmt.Errorf("alpha surface not reported")
		}
	}()
	if err != nil {
		panic(err)
	}
}

func checkTransparentSurfacePaint(plat platform.Platform) {
	backends := []string{"software", "opengl"}
	if runtime.GOOS == "windows" {
		backends = append(backends, "direct2d")
	}
	for _, backend := range backends {
		for _, role := range []string{"window", "popup"} {
			func() {
				fmt.Println("CHECK", backend+"/"+role)
				if err := os.Setenv("GOUI_PLAT_PAINTER", backend); err != nil {
					panic(err)
				}
				var loop platform.EventLoop
				var owner, win platform.Window
				var popup platform.Popup
				var painter graphics.Painter
				var surface platform.Surface
				var size events.SizeEvent
				var testErr error
				frames := 0
				defer func() {
					if painter != nil {
						painter.Destroy()
					}
					if popup != nil {
						popup.Destroy()
					}
					if win != nil {
						win.Destroy()
					}
					if owner != nil {
						owner.Destroy()
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
					handler := func(event platform.Event) {
						switch e := event.(type) {
						case events.SizeEvent:
							size = e
						case events.PaintEvent:
							if painter == nil || size.Width <= 0 || size.Height <= 0 {
								return
							}
							painter.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
							painter.Clear(graphics.Color{})
							if frames == 0 {
								painter.FillRoundRect(graphics.Rect(12, 12, 80, 48), 16, graphics.Color{R: .5, G: .125, A: .5})
							}
							painter.End()
							frames++
							if frames >= 2 {
								loop.Quit()
							} else {
								loop.Post(func() { surface.RequestPaint() })
							}
						}
					}
					if role == "window" {
						win, testErr = plat.NewWindow(geometry.Size{Width: 140, Height: 100}, handler,
							platform.WindowOptions{Chrome: platform.WindowChromeNone, Transparent: true})
						surface = win
					} else {
						owner, testErr = plat.NewWindow(geometry.Size{Width: 300, Height: 200}, func(platform.Event) {}, platform.WindowOptions{})
						if testErr != nil {
							return
						}
						if testErr = owner.(platform.WindowManagement).Show(); testErr != nil {
							return
						}
						popup, testErr = plat.NewPopup(owner, geometry.Size{Width: 140, Height: 100}, handler, platform.PopupOptions{Transparent: true})
						if testErr == nil {
							popup.SetPosition(20, 20)
						}
						surface = popup
					}
					if testErr != nil {
						return
					}
					painter, testErr = plat.NewPainter(surface)
					if testErr != nil {
						return
					}
					want := map[string]string{"software": "Software", "opengl": "OpenGL", "direct2d": "Direct2D"}[backend]
					if painter.Name() != want {
						testErr = fmt.Errorf("requested %s, got %s", want, painter.Name())
						return
					}
					if popup != nil {
						testErr = popup.Show()
					} else {
						testErr = win.(platform.WindowManagement).Show()
					}
					if testErr != nil {
						return
					}
					surface.RequestPaint()
				}()
				if testErr != nil {
					panic(testErr)
				}
				done := make(chan struct{})
				stopped := make(chan struct{})
				timer := time.NewTimer(5 * time.Second)
				defer timer.Stop()
				go func() {
					defer close(stopped)
					select {
					case <-done:
					case <-timer.C:
						loop.Quit()
					}
				}()
				loop.Run()
				close(done)
				<-stopped
				if frames < 2 {
					panic(fmt.Errorf("only %d frames presented", frames))
				}
			}()
		}
	}
}
