package main

import (
	"errors"
	"fmt"
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
)

func checkWindowWorkArea(plat platform.Platform) {
	var queryErr error
	var area geometry.Rectangle
	func() {
		win, e := plat.NewWindow(geometry.Size{Width: 240, Height: 160}, func(platform.Event) {}, platform.WindowOptions{})
		if e != nil {
			queryErr = e
			return
		}
		desktop := win.(platform.DesktopWindow)
		defer win.Destroy()
		area, queryErr = desktop.WorkAreaAt(geometry.Point{X: 120, Y: 80})
		if _, e := desktop.WorkAreaAt(geometry.Point{X: float32(math.NaN())}); e == nil {
			panic("accepted NaN")
		}
		if queryErr == nil {
			if far, e := desktop.WorkAreaAt(geometry.Point{X: 100000, Y: -100000}); e != nil || far.Width <= 0 || far.Height <= 0 {
				panic(fmt.Errorf("nearest off-screen display: %v, %v", far, e))
			}
		}
		win.Destroy()
		if _, e := desktop.WorkAreaAt(geometry.Point{}); !errors.Is(e, platform.ErrUnavailable) {
			panic(fmt.Errorf("destroyed query: %v", e))
		}
	}()
	if errors.Is(queryErr, platform.ErrUnsupported) {
		panic(fmt.Errorf("work area unavailable on this desktop: %v", queryErr))
	}
	if queryErr != nil {
		panic(queryErr)
	}
	if area.Width <= 0 || area.Height <= 0 {
		panic(fmt.Sprint("invalid area", area))
	}
	fmt.Printf("owner-client DIP workarea: %v\n", area)
}
