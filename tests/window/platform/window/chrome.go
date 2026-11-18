package main

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

func checkWindowChromeCreation(plat platform.Platform) {
	for _, chrome := range []platform.WindowChrome{platform.WindowChromeNative, platform.WindowChromeNone, platform.WindowChromeIntegrated} {
		func() {
			fmt.Println("CHECK", fmt.Sprint(chrome))
			var win platform.Window
			actual := platform.WindowChromeUnknown
			state := platform.WindowStateUnknown
			var createErr, queryErr, staleErr error
			var size events.SizeEvent
			notified := 0
			func() {
				win, createErr = plat.NewWindow(geometry.Size{Width: 320, Height: 200},
					func(event platform.Event) {
						notified++
						if event, ok := event.(events.SizeEvent); ok {
							size = event
						}
					}, platform.WindowOptions{Chrome: chrome})
				if createErr != nil {
					return
				}
				defer win.Destroy()
				desktop, ok := win.(platform.WindowManagement)
				if !ok {
					queryErr = fmt.Errorf("desktop backend returned a base-only Window")
					return
				}
				actual, state = desktop.Chrome(), desktop.State()
				var hitErr error
				if runtime.GOOS != "windows" {
					hitErr = platform.ErrUnsupported
				}
				for _, f := range []func(geometry.Point) platform.WindowHit{
					func(geometry.Point) platform.WindowHit { return platform.WindowHitClient }, nil,
				} {
					if err := desktop.SetHitTest(f); !errors.Is(err, hitErr) {
						queryErr = fmt.Errorf("SetHitTest: %v, want %v", err, hitErr)
						return
					}
				}
				if runtime.GOOS != "darwin" || chrome != platform.WindowChromeIntegrated {
					for _, position := range []*geometry.Point{nil, {X: 16, Y: 18}} {
						if err := desktop.SetControlsPosition(position); !errors.Is(err, platform.ErrUnsupported) {
							queryErr = fmt.Errorf("SetControlsPosition: %v, want unsupported", err)
							return
						}
					}
				}
				win.Destroy()
				if desktop.State() != platform.WindowStateUnknown {
					queryErr = fmt.Errorf("dead window reported a state")
				}
				if desktop.Chrome() != platform.WindowChromeUnknown {
					queryErr = fmt.Errorf("dead window reported chrome")
				}
				_, staleErr = desktop.ControlsRect()
				for _, position := range []*geometry.Point{nil, {X: 16, Y: 18}} {
					if err := desktop.SetControlsPosition(position); !errors.Is(err, platform.ErrUnavailable) {
						queryErr = fmt.Errorf("dead SetControlsPosition: %v, want unavailable", err)
					}
				}
			}()
			if chrome == platform.WindowChromeIntegrated && runtime.GOOS == "linux" {
				if win != nil || !errors.Is(createErr, platform.ErrUnsupported) || notified != 0 {
					panic(fmt.Errorf("unsupported integration: %v %v %d", win, createErr, notified))
				}
				return
			}
			if chrome == platform.WindowChromeIntegrated && runtime.GOOS == "windows" &&
				errors.Is(createErr, platform.ErrUnsupported) && win == nil && notified == 0 {
				panic(fmt.Errorf("native composition unavailable: %v", createErr))
			}
			if createErr != nil || queryErr != nil {
				panic(fmt.Errorf("create %v; query %v", createErr, queryErr))
			}
			if actual != platform.WindowChromeUnknown && actual != chrome {
				panic(fmt.Errorf("chrome %v want %v", actual, chrome))
			}
			if chrome == platform.WindowChromeIntegrated {
				if actual != chrome {
					panic(fmt.Errorf("integrated creation did not establish the native frame: %v", actual))
				}
				if size.PixelWidth <= 0 || size.PixelHeight <= 0 || size.Width <= 0 || size.Height <= 0 {
					panic(fmt.Errorf("missing initial client extent: %+v", size))
				}
				scale := float64(size.PixelWidth / size.Width)
				if math.Abs(float64(size.Width-320))*scale > 1 || math.Abs(float64(size.Height-200))*scale > 1 {
					panic(fmt.Errorf("requested client 320x200 DIP, got %+v", size))
				}
			}
			if state != platform.WindowStateHidden {
				panic(fmt.Errorf("unshown window state: %v", state))
			}
			if !errors.Is(staleErr, platform.ErrUnavailable) {
				panic(fmt.Errorf("dead controls query: %v", staleErr))
			}
		}()
	}
}

func checkWindowChromeIntegrated(plat platform.Platform) {
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		panic("no native Integrated chrome on this backend")
	}

	var err error
	var loop platform.EventLoop
	var win platform.Window
	var desktop platform.WindowManagement
	var painter graphics.Painter
	var size events.SizeEvent
	var frames []events.SizeEvent
	var controls []geometry.Rectangle
	var testErr error
	var painterName string
	targets := []platform.WindowState{platform.WindowStateNormal, platform.WindowStateMaximized, platform.WindowStateNormal}
	step, closeRequests := 0, 0
	func() {
		loop, err = plat.NewEventLoop()
		if err != nil {
			return
		}
		win, err = plat.NewWindow(geometry.Size{Width: 640, Height: 400}, func(event platform.Event) {
			switch event := event.(type) {
			case events.SizeEvent:
				size = event
			case events.PaintEvent:
				if painter == nil || size.Width <= 0 || size.Height <= 0 || step >= len(targets) {
					return
				}
				painter.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
				painter.Clear(graphics.RGB(245, 245, 245))
				painter.FillRect(geometry.Rect(0, 0, size.Width, 52), graphics.RGB(50, 80, 110))
				painter.DrawRect(geometry.Rect(8, 60, size.Width-16, size.Height-68), 1, graphics.RGB(30, 140, 120))
				painter.End()
				if desktop.State() != targets[step] {
					return
				}
				r, queryErr := desktop.ControlsRect()
				if queryErr != nil {
					testErr = queryErr
					loop.Quit()
					return
				}
				controls = append(controls, r)
				frames = append(frames, size)
				step++
				loop.Post(func() {
					if step == len(targets) {
						testErr = win.(platform.WindowManagement).RequestClose()
					} else {
						testErr = desktop.RequestState(targets[step])
					}
					if testErr != nil {
						loop.Quit()
					}
				})
			case events.CloseEvent:
				closeRequests++

				if closeRequests == 1 {
					if desktop.State() != platform.WindowStateNormal || desktop.Chrome() != platform.WindowChromeIntegrated {
						testErr = fmt.Errorf("close request altered native window state/chrome")
					}
					loop.Post(func() {
						if closeErr := win.(platform.WindowManagement).RequestClose(); closeErr != nil {
							testErr = closeErr
							loop.Quit()
						}
					})
				} else {
					loop.Quit()
				}
			}
		}, platform.WindowOptions{Chrome: platform.WindowChromeIntegrated})
		if err != nil {
			return
		}
		desktop = win.(platform.WindowManagement)
		desktop.SetMinSize(320, 200)
		painter, err = plat.NewPainter(win)
		if err != nil {
			return
		}
		painterName = painter.Name()
		err = win.(platform.WindowManagement).Show()
	}()
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
	if err != nil {
		panic(err)
	}
	timer := time.AfterFunc(8*time.Second, loop.Quit)
	loop.Run()
	timer.Stop()
	if testErr != nil {
		panic(testErr)
	}
	if step != len(targets) || closeRequests != 2 {
		panic(fmt.Errorf("native integration stalled: frames=%v close requests=%d", frames, closeRequests))
	}
	for i, r := range controls {
		if runtime.GOOS == "windows" {
			if r != (geometry.Rectangle{}) {
				panic(fmt.Errorf("Integrated has no native Windows caption controls: %v", r))
			}
		} else if r.X < 0 || r.Y < 0 || r.Width <= 0 || r.Height <= 0 ||
			r.X+r.Width > frames[i].Width || r.Y+r.Height > frames[i].Height {
			panic(fmt.Errorf("AppKit buttons outside full-size client: controls=%v size=%+v", r, frames[i]))
		}
	}
	if frames[0] != frames[len(frames)-1] {
		panic(fmt.Errorf("restored client differs: before=%+v after=%+v", frames[0], frames[len(frames)-1]))
	}
	if runtime.GOOS == "darwin" {
		i := 0
		scale := float64(frames[i].PixelWidth / frames[i].Width)
		if math.Abs(float64(frames[i].Width-640))*scale > 1 || math.Abs(float64(frames[i].Height-400))*scale > 1 {
			panic(fmt.Errorf("normal client after transition: %+v, want 640x400 DIP", frames[i]))
		}
	}
	fmt.Printf("painter=%s; client extents=%+v; native controls=%v\n", painterName, frames, controls)
}

func checkWindowInteractionCapabilities(plat platform.Platform) {
	var err error
	var moveErr, resizeErr, deadMoveErr, deadResizeErr, deadHitErr error
	func() {
		win, createErr := plat.NewWindow(geometry.Size{Width: 320, Height: 200},
			func(platform.Event) {}, platform.WindowOptions{})
		err = createErr
		if err != nil {
			return
		}
		desktop := win.(platform.WindowManagement)
		moveErr = desktop.BeginMove()
		resizeErr = desktop.BeginResize(platform.WindowEdgeBottomRight)
		win.Destroy()
		deadMoveErr = desktop.BeginMove()
		deadResizeErr = desktop.BeginResize(platform.WindowEdgeBottomRight)
		deadHitErr = desktop.SetHitTest(nil)
	}()
	if err != nil {
		panic(err)
	}
	wantMove, wantResize := platform.ErrUnavailable, platform.ErrUnavailable
	if runtime.GOOS == "windows" {
		wantMove = platform.ErrUnsupported
		wantResize = platform.ErrUnsupported
	}
	if !errors.Is(moveErr, wantMove) || !errors.Is(resizeErr, wantResize) {
		panic(fmt.Errorf("without input: move=%v want %v, resize=%v want %v", moveErr, wantMove, resizeErr, wantResize))
	}
	for _, err := range []error{deadMoveErr, deadResizeErr, deadHitErr} {
		if !errors.Is(err, platform.ErrUnavailable) {
			panic(fmt.Errorf("destroyed window: %v", err))
		}
	}
}

func checkWindowStateMinimizeTransitions(plat platform.Platform) {
	testWindowStateTransitions(plat, []platform.WindowState{platform.WindowStateNormal, platform.WindowStateMinimized,
		platform.WindowStateNormal, platform.WindowStateHidden, platform.WindowStateNormal})
}
func checkWindowStateMaximizeTransitions(plat platform.Platform) {
	testWindowStateTransitions(plat, []platform.WindowState{platform.WindowStateNormal, platform.WindowStateMaximized,
		platform.WindowStateMinimized, platform.WindowStateNormal, platform.WindowStateHidden, platform.WindowStateNormal})
}
func checkWindowStateFullscreenTransitions(plat platform.Platform) {
	testWindowStateTransitions(plat, []platform.WindowState{platform.WindowStateNormal, platform.WindowStateFullscreen,
		platform.WindowStateMaximized, platform.WindowStateNormal})
}

func testWindowStateTransitions(plat platform.Platform, targets []platform.WindowState) {
	func() {
	}()

	var err error
	var loop platform.EventLoop
	var win platform.Window
	var desktop platform.WindowManagement
	var commandErr error
	var observed []platform.WindowState
	step := 0
	func() {
		loop, err = plat.NewEventLoop()
		if err != nil {
			return
		}
		advance := func(state platform.WindowState) {
			if desktop == nil || step >= len(targets) {
				return
			}
			observed = append(observed, state)
			if state != targets[step] {
				return
			}
			step++
			if step == len(targets) {
				loop.Quit()
				return
			}
			target := targets[step]
			loop.Post(func() {
				commandErr = desktop.RequestState(target)
				if commandErr != nil {
					loop.Quit()
				}
			})
		}
		win, err = plat.NewWindow(geometry.Size{Width: 320, Height: 200}, func(event platform.Event) {
			switch event := event.(type) {
			case events.StateEvent:
				advance(event.State)
			case events.PaintEvent:
				if desktop != nil {
					advance(desktop.State())
				}
			}
		}, platform.WindowOptions{})
		if err != nil {
			return
		}
		desktop = win.(platform.WindowManagement)
		err = win.(platform.WindowManagement).Show()
	}()
	defer func() {
		if win != nil {
			win.Destroy()
		}
		if loop != nil {
			loop.Destroy()
		}
	}()
	if err != nil {
		panic(err)
	}
	timer := time.AfterFunc(8*time.Second, loop.Quit)
	loop.Run()
	timer.Stop()
	if errors.Is(commandErr, platform.ErrUnsupported) {
		panic(fmt.Errorf("unsupported native transition: %v", commandErr))
	}
	if commandErr != nil {
		panic(commandErr)
	}
	if step != len(targets) {
		panic(fmt.Errorf("stalled at %d/%d; observed %v", step, len(targets), observed))
	}
	var controls geometry.Rectangle
	var controlsErr error
	controls, controlsErr = desktop.ControlsRect()
	if !errors.Is(controlsErr, platform.ErrUnsupported) {
		if controlsErr != nil || controls.Width <= 0 || controls.Height <= 0 {
			panic(fmt.Errorf("visible Native controls: %v, %v", controls, controlsErr))
		}
		fmt.Printf("native controls in client DIP: %v\n", controls)
	}
	fmt.Printf("observed native states: %v\n", observed)
}
