package main

import (
	"errors"
	"fmt"
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	"github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	"github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/events"
)

func checkWindowChromeMouseDownCannotMove(plat platform.Platform) {
	var err error
	for _, chrome := range []platform.WindowChrome{platform.WindowChromeNative, platform.WindowChromeIntegrated, platform.WindowChromeNone} {
		func() {
			fmt.Println("CHECK", fmt.Sprint(chrome))
			var canMove bool
			func() {
				var win platform.Window
				win, err = plat.NewWindow(geometry.Size{Width: 320, Height: 200}, func(platform.Event) {}, platform.WindowOptions{Chrome: chrome})
				if err != nil {
					return
				}
				defer win.Destroy()
				native := foundation.Cast[appkit.NSWindow](foundation.ID(win.NativeHandle()))
				canMove = native.ContentView().MouseDownCanMoveWindow()
			}()
			if err != nil {
				panic(err)
			}
			if canMove {
				panic("content view still opts into implicit native dragging")
			}
		}()
	}
}

func checkWindowControls(plat platform.Platform) {
	var err error
	func() {
		foundation.AutoReleasePool(func() { err = checkWindowControlsPosition(plat) })
	}()
	if err != nil {
		panic(err)
	}
}

func checkWindowControlsPosition(plat platform.Platform) error {
	var size events.SizeEvent
	win, err := plat.NewWindow(geometry.Size{Width: 640, Height: 400}, func(event platform.Event) {
		if event, ok := event.(events.SizeEvent); ok {
			size = event
		}
	}, platform.WindowOptions{Chrome: platform.WindowChromeIntegrated})
	if err != nil {
		return err
	}
	defer win.Destroy()
	desktop := win.(platform.DesktopWindow)
	native := foundation.Cast[appkit.NSWindow](foundation.ID(win.NativeHandle()))
	pointsPerDIP := float64(size.PixelWidth/size.Width) / float64(native.BackingScaleFactor())
	defaults := nativeControlRects(native, pointsPerDIP)
	if defaults[0].Width <= 0 || defaults[1].Width <= 0 || defaults[2].Width <= 0 {
		return fmt.Errorf("missing initial native buttons: %v", defaults)
	}
	defaultHeight := native.StandardWindowButton(appkit.NSWindowCloseButton).Superview().Superview().Frame().Size.Height
	defaultGroup := controlsUnion(defaults)

	other, err := plat.NewWindow(geometry.Size{Width: 500, Height: 300}, func(platform.Event) {},
		platform.WindowOptions{Chrome: platform.WindowChromeIntegrated})
	if err != nil {
		return err
	}
	defer other.Destroy()
	otherNative := foundation.Cast[appkit.NSWindow](foundation.ID(other.NativeHandle()))
	otherDefaults := nativeControlRects(otherNative, 1)

	check := func(want geometry.Point) error {
		got := nativeControlRects(native, pointsPerDIP)
		group := controlsUnion(got)
		for i, rect := range got {
			expected := geometry.Rect(want.X+defaults[i].X-defaultGroup.X,
				want.Y+defaults[i].Y-defaultGroup.Y, defaults[i].Width, defaults[i].Height)
			if !controlsRectClose(rect, expected) {
				return fmt.Errorf("button %d: got %v, want %v (group %v)", i, rect, expected, group)
			}
		}
		return nil
	}
	for _, position := range []geometry.Point{{}, {X: 16.5, Y: 18.25}, {X: 44, Y: 26}, {X: 44, Y: 26}} {
		p := position
		if err := desktop.SetControlsPosition(&p); err != nil {
			return fmt.Errorf("set %v: %w", position, err)
		}
		p.X, p.Y = 100, 100
		if err := desktop.SetTitle("Controls position regression"); err != nil {
			return err
		}
		if err := check(position); err != nil {
			return err
		}
	}
	before := nativeControlRects(native, pointsPerDIP)
	for _, invalid := range []geometry.Point{{X: -1}, {Y: float32(math.NaN())}, {X: float32(math.Inf(1))}, {Y: 10000}} {
		if err := desktop.SetControlsPosition(&invalid); err == nil {
			return fmt.Errorf("accepted invalid/unfit position: %v", invalid)
		}
		if after := nativeControlRects(native, pointsPerDIP); after != before {
			return fmt.Errorf("rejected position changed native layout: %v -> %v", before, after)
		}
	}

	native.SetContentSize(foundation.NSSize{Width: core_graphics.CGFloat(720 * pointsPerDIP), Height: core_graphics.CGFloat(480 * pointsPerDIP)})
	if err := check(geometry.Point{X: 44, Y: 26}); err != nil {
		return fmt.Errorf("after resize: %w", err)
	}
	{
		for range 2 {
			if err := desktop.Show(); err != nil {
				return err
			}
			if err := check(geometry.Point{X: 44, Y: 26}); err != nil {
				return fmt.Errorf("after show: %w", err)
			}
			got, err := desktop.ControlsRect()
			want := controlsUnion(nativeControlRects(native, pointsPerDIP))
			if err != nil || !controlsRectClose(got, want) {
				return fmt.Errorf("ControlsRect = %v, %v; actual native union %v", got, err, want)
			}
			if err := desktop.Hide(); err != nil {
				return err
			}
		}
	}
	for range 2 {
		if err := desktop.SetControlsPosition(nil); err != nil {
			return fmt.Errorf("reset: %w", err)
		}
		if err := check(geometry.Point{X: defaultGroup.X, Y: defaultGroup.Y}); err != nil {
			return fmt.Errorf("reset: %w", err)
		}
		if got := native.StandardWindowButton(appkit.NSWindowCloseButton).Superview().Superview().Frame().Size.Height; got != defaultHeight {
			return fmt.Errorf("reset container height: got %v, want %v", got, defaultHeight)
		}
	}
	win.Destroy()
	for _, position := range []*geometry.Point{nil, {X: 16, Y: 18}} {
		if err := desktop.SetControlsPosition(position); !errors.Is(err, platform.ErrUnavailable) {
			return fmt.Errorf("destroyed window setter: %v", err)
		}
	}
	if got := nativeControlRects(otherNative, 1); got != otherDefaults {
		return fmt.Errorf("another window's controls changed: %v -> %v", otherDefaults, got)
	}
	return nil
}

func nativeControlRects(window appkit.NSWindow, pointsPerDIP float64) (rects [3]geometry.Rectangle) {
	view := window.ContentView()
	bounds := view.Bounds()
	for i, kind := range [...]appkit.NSWindowButton{appkit.NSWindowCloseButton, appkit.NSWindowMiniaturizeButton, appkit.NSWindowZoomButton} {
		button := window.StandardWindowButton(kind)
		r := button.ConvertRectToView(button.Bounds(), view)
		rects[i] = geometry.Rect(float32(float64(r.Origin.X-bounds.Origin.X)/pointsPerDIP),
			float32(float64(bounds.Origin.Y+bounds.Size.Height-r.Origin.Y-r.Size.Height)/pointsPerDIP),
			float32(float64(r.Size.Width)/pointsPerDIP), float32(float64(r.Size.Height)/pointsPerDIP))
	}
	return rects
}

func controlsUnion(rects [3]geometry.Rectangle) geometry.Rectangle {
	r := rects[0]
	for _, next := range rects[1:] {
		x, y := min(r.X, next.X), min(r.Y, next.Y)
		r = geometry.Rect(x, y, max(r.X+r.Width, next.X+next.Width)-x, max(r.Y+r.Height, next.Y+next.Height)-y)
	}
	return r
}

func controlsRectClose(a, b geometry.Rectangle) bool {
	return math.Abs(float64(a.X-b.X)) < 0.01 && math.Abs(float64(a.Y-b.Y)) < 0.01 &&
		math.Abs(float64(a.Width-b.Width)) < 0.01 && math.Abs(float64(a.Height-b.Height)) < 0.01
}

func checkNativeGeometry(plat platform.Platform) {
	checkWindowChromeMouseDownCannotMove(plat)
	checkWindowControls(plat)
	checkMacOSZoomState(plat)
}
