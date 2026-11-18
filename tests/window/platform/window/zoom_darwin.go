package main

import (
	"errors"
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	"github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/events"
)

func checkMacOSZoomState(plat platform.Platform) {
	var err error
	for _, mode := range []platform.WindowChrome{platform.WindowChromeNative, platform.WindowChromeIntegrated} {
		func() {
			fmt.Println("CHECK", fmt.Sprint(mode))
			func() { foundation.AutoReleasePool(func() { err = checkZoomState(plat, mode) }) }()
			if err != nil {
				panic(err)
			}
		}()
	}
}

func checkZoomState(plat platform.Platform, mode platform.WindowChrome) error {
	var seen []platform.WindowState
	var desktop platform.DesktopWindow
	var destroyOnMax bool
	var mismatch bool
	win, err := plat.NewWindow(geometry.Size{Width: 420, Height: 280}, func(e platform.Event) {
		if state, ok := e.(events.StateEvent); ok {
			seen = append(seen, state.State)
			if desktop != nil && desktop.State() != state.State {
				mismatch = true
			}
			if destroyOnMax && state.State == platform.WindowStateMaximized {
				desktop.Destroy()
			}
		}
	}, platform.WindowOptions{Chrome: mode})
	if err != nil {
		return err
	}
	defer win.Destroy()
	desktop = win.(platform.DesktopWindow)
	native := foundation.Cast[appkit.NSWindow](foundation.ID(win.NativeHandle()))
	if err := desktop.Show(); err != nil {
		return err
	}
	initial := native.Frame()
	for _, want := range []platform.WindowState{platform.WindowStateMaximized, platform.WindowStateNormal} {
		if err := desktop.RequestState(want); err != nil {
			return err
		}
		if desktop.State() != want || native.IsZoomed() != (want == platform.WindowStateMaximized) {
			return fmt.Errorf("zoom state = %v, want %v", desktop.State(), want)
		}
		if len(seen) == 0 || seen[len(seen)-1] != want || mismatch {
			return fmt.Errorf("missing or predicted state notification: %v", seen)
		}
		frame, count := native.Frame(), len(seen)
		if err := desktop.RequestState(want); err != nil {
			return err
		}
		if native.Frame() != frame || len(seen) != count {
			return fmt.Errorf("duplicate request toggled zoom or emitted state")
		}
	}
	if native.Frame() != initial {
		return fmt.Errorf("unzoom did not restore original frame: %v -> %v", initial, native.Frame())
	}

	native.Zoom(0)
	if desktop.State() != platform.WindowStateMaximized || seen[len(seen)-1] != platform.WindowStateMaximized {
		return fmt.Errorf("native zoom was not observed: %v", seen)
	}
	if err := desktop.Hide(); err != nil {
		return err
	}
	if desktop.State() != platform.WindowStateHidden {
		return fmt.Errorf("hidden zoomed window reported visible state")
	}
	if err := desktop.RequestState(platform.WindowStateNormal); err != nil {
		return err
	}
	if desktop.State() != platform.WindowStateNormal || native.IsZoomed() {
		return fmt.Errorf("hidden zoomed window did not normalize")
	}
	destroyOnMax = true
	if err := desktop.RequestState(platform.WindowStateMaximized); !errors.Is(err, platform.ErrUnavailable) {
		return fmt.Errorf("destroyed zoom request: %v", err)
	}
	if desktop.State() != platform.WindowStateUnknown {
		return fmt.Errorf("destroyed state was not Unknown")
	}
	return nil
}
