package test

import (
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

func TestWindowChromeCreation(t *testing.T) {
	skipWithoutDisplay(t)
	plat, err := getPlatform()
	if err != nil {
		t.Fatal(err)
	}
	for _, chrome := range []platform.WindowChrome{platform.WindowChromeNative, platform.WindowChromeNone, platform.WindowChromeIntegrated} {
		t.Run(fmt.Sprint(chrome), func(t *testing.T) {
			var win platform.Window
			actual := platform.WindowChromeUnknown
			state := platform.WindowStateUnknown
			var createErr, queryErr, staleErr error
			var size events.SizeEvent
			notified := 0
			runOnMainThread(func() {
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
				desktop, ok := win.(platform.DesktopWindow)
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
			})
			if chrome == platform.WindowChromeIntegrated && runtime.GOOS == "linux" {
				if win != nil || !errors.Is(createErr, platform.ErrUnsupported) || notified != 0 {
					t.Fatalf("unsupported integration: %v %v %d", win, createErr, notified)
				}
				return
			}
			if chrome == platform.WindowChromeIntegrated && runtime.GOOS == "windows" &&
				errors.Is(createErr, platform.ErrUnsupported) && win == nil && notified == 0 {
				t.Skipf("native composition unavailable: %v", createErr)
			}
			if createErr != nil || queryErr != nil {
				t.Fatalf("create %v; query %v", createErr, queryErr)
			}
			if actual != platform.WindowChromeUnknown && actual != chrome {
				t.Fatalf("chrome %v want %v", actual, chrome)
			}
			if chrome == platform.WindowChromeIntegrated {
				if actual != chrome {
					t.Fatalf("integrated creation did not establish the native frame: %v", actual)
				}
				if size.PixelWidth <= 0 || size.PixelHeight <= 0 || size.Width <= 0 || size.Height <= 0 {
					t.Fatalf("missing initial client extent: %+v", size)
				}
				scale := float64(size.PixelWidth / size.Width)
				if math.Abs(float64(size.Width-320))*scale > 1 || math.Abs(float64(size.Height-200))*scale > 1 {
					t.Fatalf("requested client 320x200 DIP, got %+v", size)
				}
			}
			if state != platform.WindowStateHidden {
				t.Fatalf("unshown window state: %v", state)
			}
			if !errors.Is(staleErr, platform.ErrUnavailable) {
				t.Fatalf("dead controls query: %v", staleErr)
			}
		})
	}
}

// Run on a real desktop with GOUI_TEST_WINDOW_CHROME=1. Select the renderer with
// GOUI_PLAT_PAINTER and repeat with GOUI_PLAT_SCALE=1 / 2. This checks native
// geometry and rendering/lifetime integration, not pixels in the compositor.
func TestWindowChromeIntegrated(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		t.Skip("no native Integrated chrome on this backend")
	}
	if os.Getenv("GOUI_TEST_WINDOW_CHROME") == "" {
		t.Skip("set GOUI_TEST_WINDOW_CHROME=1 for visible native integration")
	}
	plat, err := getPlatform()
	if err != nil {
		t.Fatal(err)
	}
	var loop platform.EventLoop
	var win platform.Window
	var desktop platform.DesktopWindow
	var painter graphics.Painter
	var size events.SizeEvent
	var frames []events.SizeEvent
	var controls []geometry.Rectangle
	var testErr error
	var painterName string
	targets := []platform.WindowState{platform.WindowStateNormal, platform.WindowStateMaximized, platform.WindowStateNormal}
	step, closeRequests := 0, 0
	runOnMainThread(func() {
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
						testErr = win.RequestClose()
					} else {
						testErr = desktop.RequestState(targets[step])
					}
					if testErr != nil {
						loop.Quit()
					}
				})
			case events.CloseEvent:
				closeRequests++
				// Veto the first close and verify that the native window survives.
				if closeRequests == 1 {
					if desktop.State() != platform.WindowStateNormal || desktop.Chrome() != platform.WindowChromeIntegrated {
						testErr = fmt.Errorf("close request altered native window state/chrome")
					}
					loop.Post(func() {
						if closeErr := win.RequestClose(); closeErr != nil {
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
		desktop = win.(platform.DesktopWindow)
		desktop.SetMinSize(320, 200)
		painter, err = plat.NewPainter(win)
		if err != nil {
			return
		}
		painterName = painter.Name()
		err = win.Show()
	})
	defer runOnMainThread(func() {
		if painter != nil {
			painter.Destroy()
		}
		if win != nil {
			win.Destroy()
		}
		if loop != nil {
			loop.Destroy()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	timer := time.AfterFunc(8*time.Second, loop.Quit)
	runOnMainThread(loop.Run)
	timer.Stop()
	if testErr != nil {
		t.Fatal(testErr)
	}
	if step != len(targets) || closeRequests != 2 {
		t.Fatalf("native integration stalled: frames=%v close requests=%d", frames, closeRequests)
	}
	for i, r := range controls {
		if runtime.GOOS == "windows" {
			if r != (geometry.Rectangle{}) {
				t.Fatalf("Integrated has no native Windows caption controls: %v", r)
			}
		} else if r.X < 0 || r.Y < 0 || r.Width <= 0 || r.Height <= 0 ||
			r.X+r.Width > frames[i].Width || r.Y+r.Height > frames[i].Height {
			t.Fatalf("AppKit buttons outside full-size client: controls=%v size=%+v", r, frames[i])
		}
	}
	if frames[0] != frames[len(frames)-1] {
		t.Fatalf("restored client differs: before=%+v after=%+v", frames[0], frames[len(frames)-1])
	}
	if runtime.GOOS == "darwin" {
		i := 0
		scale := float64(frames[i].PixelWidth / frames[i].Width)
		if math.Abs(float64(frames[i].Width-640))*scale > 1 || math.Abs(float64(frames[i].Height-400))*scale > 1 {
			t.Fatalf("normal client after transition: %+v, want 640x400 DIP", frames[i])
		}
	}
	t.Logf("painter=%s; client extents=%+v; native controls=%v", painterName, frames, controls)
}

func TestWindowInteractionCapabilities(t *testing.T) {
	skipWithoutDisplay(t)
	plat, err := getPlatform()
	if err != nil {
		t.Fatal(err)
	}
	var moveErr, resizeErr, deadMoveErr, deadResizeErr, deadHitErr error
	runOnMainThread(func() {
		win, createErr := plat.NewWindow(geometry.Size{Width: 320, Height: 200},
			func(platform.Event) {}, platform.WindowOptions{})
		err = createErr
		if err != nil {
			return
		}
		desktop := win.(platform.DesktopWindow)
		moveErr = desktop.BeginMove()
		resizeErr = desktop.BeginResize(platform.WindowEdgeBottomRight)
		win.Destroy()
		deadMoveErr = desktop.BeginMove()
		deadResizeErr = desktop.BeginResize(platform.WindowEdgeBottomRight)
		deadHitErr = desktop.SetHitTest(nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	wantMove, wantResize := platform.ErrUnavailable, platform.ErrUnsupported
	if runtime.GOOS == "windows" {
		wantMove = platform.ErrUnsupported
	} else if runtime.GOOS == "linux" {
		wantResize = platform.ErrUnavailable
	}
	if !errors.Is(moveErr, wantMove) || !errors.Is(resizeErr, wantResize) {
		t.Fatalf("without input: move=%v want %v, resize=%v want %v", moveErr, wantMove, resizeErr, wantResize)
	}
	for _, err := range []error{deadMoveErr, deadResizeErr, deadHitErr} {
		if !errors.Is(err, platform.ErrUnavailable) {
			t.Fatalf("destroyed window: %v", err)
		}
	}
}

func TestWindowStateMinimizeTransitions(t *testing.T) {
	testWindowStateTransitions(t, []platform.WindowState{platform.WindowStateNormal, platform.WindowStateMinimized,
		platform.WindowStateNormal, platform.WindowStateHidden, platform.WindowStateNormal})
}
func TestWindowStateMaximizeTransitions(t *testing.T) {
	// Normal must exit maximization even when minimizing preserved a native
	// restore-to-maximized flag. It is not a Restore command.
	testWindowStateTransitions(t, []platform.WindowState{platform.WindowStateNormal, platform.WindowStateMaximized,
		platform.WindowStateMinimized, platform.WindowStateNormal, platform.WindowStateHidden, platform.WindowStateNormal})
}
func TestWindowStateFullscreenTransitions(t *testing.T) {
	testWindowStateTransitions(t, []platform.WindowState{platform.WindowStateNormal, platform.WindowStateFullscreen,
		platform.WindowStateMaximized, platform.WindowStateNormal})
}

func testWindowStateTransitions(t *testing.T, targets []platform.WindowState) {
	t.Helper()
	skipWithoutDisplay(t)
	if os.Getenv("GOUI_TEST_WINDOW_STATE") == "" {
		t.Skip("set GOUI_TEST_WINDOW_STATE=1 for native transitions")
	}
	plat, err := getPlatform()
	if err != nil {
		t.Fatal(err)
	}
	var loop platform.EventLoop
	var win platform.Window
	var desktop platform.DesktopWindow
	var commandErr error
	var observed []platform.WindowState
	step := 0
	runOnMainThread(func() {
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
		desktop = win.(platform.DesktopWindow)
		err = win.Show()
	})
	defer runOnMainThread(func() {
		if win != nil {
			win.Destroy()
		}
		if loop != nil {
			loop.Destroy()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	timer := time.AfterFunc(8*time.Second, loop.Quit)
	runOnMainThread(loop.Run)
	timer.Stop()
	if errors.Is(commandErr, platform.ErrUnsupported) {
		t.Skipf("unsupported native transition: %v", commandErr)
	}
	if commandErr != nil {
		t.Fatal(commandErr)
	}
	if step != len(targets) {
		t.Fatalf("stalled at %d/%d; observed %v", step, len(targets), observed)
	}
	var controls geometry.Rectangle
	var controlsErr error
	runOnMainThread(func() { controls, controlsErr = desktop.ControlsRect() })
	if !errors.Is(controlsErr, platform.ErrUnsupported) {
		if controlsErr != nil || controls.Width <= 0 || controls.Height <= 0 {
			t.Fatalf("visible Native controls: %v, %v", controls, controlsErr)
		}
		t.Logf("native controls in client DIP: %v", controls)
	}
	t.Logf("observed native states: %v", observed)
}
