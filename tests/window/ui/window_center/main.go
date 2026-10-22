// Initial window centering and declarative creation-option checks, without DevServer.
//
// Environment: interactive Windows/macOS desktop or a Linux/X11 WM supporting
// work-area queries. Run:
//
//	go run ./tests/window/ui/window_center [-auto] [-chrome=native|integrated|none]
//
// Also use -center=false, -override and -skip-taskbar=true. Repeat with
// GOUI_PLAT_SCALE=1 or 2, recording OS, WM and actual backend; application scale
// alone does not verify native system HiDPI.
// Initially request a 360x300 DIP client area mathematically centered in the
// work area, unlike AppKit's slightly high default. AfterUpdate asserts hidden
// position before first Show. -override then explicitly requests (120,80), which
// must determine the first visible position. -center=false makes no assertion
// about default coordinates.
// Steps and expected results:
//  1. Test all chrome modes: client/work-area centers coincide without a visible
//     jump after show. Native caption size is excluded; Integrated/None follow
//     the same client-coordinate contract.
//  2. F5/Move sets work-area (100,60). F6/Rebuild retains window identity/position.
//  3. F7/Hide-Show retains position without recentering. First drag the caption
//     in Native/Integrated and verify manual position survives F6/F7 too.
//  4. Restart -override: initially (120,80); Center does not override later positioning.
//  5. Restart -center=false: native defaults choose position; F5/F6/F7 still work.
//  6. -skip-taskbar=true retains centering, activation and buttons; see taskbar
//     checks for platform task-list differences.
//
// -auto waits 400 ms per phase for native events, checking hidden position, actual
// size/work-area symmetry, explicit movement, UI identity/position and Hide/Show,
// then prints PASS and exits. The 0.5 DIP tolerance allows at most one physical
// pixel at the specified 1x/2x. A constrained/ignored native request fails the
// assertion and must be recorded. This does not cover multiple/mixed-DPI screens,
// oversized/maximized windows or content minimums larger than the initial Size.
// Known observation: Xfwm4 may reposition a -center=false window without an initial
// position hint on Hide/Show. This assertion also failed before the centering
// change; it is not Center running again. Observed Center=true/-override cases
// retain their initial position hints.
// Side effects: only this program's windows and activation; no settings/clipboard.
// Reset: restart. Exit: Esc, Quit or native close; automatic mode exits itself,
// and failures return nonzero.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	auto := flag.Bool("auto", false, "check placement, rebuild and redisplay, then exit")
	center := flag.Bool("center", true, "request initial work-area centering")
	override := flag.Bool("override", false, "explicitly position the hidden window after initial centering")
	skip := flag.Bool("skip-taskbar", false, "omit a separate taskbar entry")
	chrome := flag.String("chrome", "native", "native, integrated or none")
	flag.Parse()
	mode, ok := map[string]ui.WindowChromeMode{
		"native": ui.WindowChromeNative, "integrated": ui.WindowChromeIntegrated, "none": ui.WindowChromeNone,
	}[*chrome]
	if !ok {
		return fmt.Errorf("unknown chrome %q", *chrome)
	}
	size := geometry.Size{Width: 360, Height: 300}
	var result error
	var original gui.Window
	initialized, rebuilds, phase := false, 0, 0
	err := ui.Run("org.golang-gui.InitialCenter", func(app ui.App) ui.RootView {
		rebuilds++
		check := func(name string, err error) bool {
			if err != nil {
				result = fmt.Errorf("%s: %w", name, err)
				app.Quit()
				return false
			}
			fmt.Println("PASS", name)
			return true
		}
		preserve := func(redisplay bool) {
			win := app.FindWindow("main")
			before, err := win.Position(nil)
			if !check("observe before operation", err) {
				return
			}
			if redisplay {
				if !check("hide", win.PlatformWindow().Hide()) || !check("show", win.Show()) {
					return
				}
			} else {
				app.RequestUpdate()
			}
			app.TimeoutFunc(400*time.Millisecond, func() {
				current := app.FindWindow("main")
				if current != win {
					check("preserved window identity", fmt.Errorf("window replaced"))
					return
				}
				check("preserved position", checkPosition(current, before))
			})
		}
		move := func() {
			check("move to (100,60)", app.FindWindow("main").SetPosition(nil, geometry.Point{X: 100, Y: 60}))
		}
		body := []ui.View{
			ui.HeaderBar(ui.Label("Initial window centering")),
			ui.Label(fmt.Sprintf("Center=%t / rebuild=%d", *center, rebuilds)),
			ui.Button("F5 Move to (100,60)").OnClick(move),
			ui.Button("F6 Rebuild").OnClick(func() { preserve(false) }),
			ui.Button("F7 Hide / Show").OnClick(func() { preserve(true) }),
			ui.Button("Esc Quit").OnClick(app.Quit),
		}
		probe := &inspectionView{children: body}
		probe.Self = probe
		probe.inspect = func() {
			if initialized {
				return
			}
			initialized = true
			original = app.FindWindow("main")
			if original.State() != gui.WindowStateHidden {
				check("hidden creation", fmt.Errorf("state=%v", original.State()))
				return
			}
			if *center && !check("center before first Show", checkCenter(original, size)) {
				return
			}
			if *override && !check("explicit pre-Show override", original.SetPosition(nil, geometry.Point{X: 120, Y: 80})) {
				return
			}
			fmt.Printf("OS=%s painter=%q scale=%q chrome=%s center=%t override=%t skip-taskbar=%t\n",
				runtime.GOOS, os.Getenv("GOUI_PLAT_PAINTER"), os.Getenv("GOUI_PLAT_SCALE"), *chrome, *center, *override, *skip)
			if !*auto {
				return
			}
			var timer *ui.Timer
			timer = app.TickFunc(400*time.Millisecond, func() {
				win := app.FindWindow("main")
				if win != original {
					check("window identity", fmt.Errorf("missing or replaced window"))
					return
				}
				switch phase {
				case 0:
					bounds := win.Snapshot().Bounds
					if math.Abs(float64(bounds.Width-size.Width)) > .5 || math.Abs(float64(bounds.Height-size.Height)) > .5 {
						check("actual client size", fmt.Errorf("bounds=%v want size=%v", bounds, size))
						return
					}
					if *override {
						if !check("override after Show", checkPosition(win, geometry.Point{X: 120, Y: 80})) {
							return
						}
					} else if *center && !check("actual client center after Show", checkCenter(win, bounds.Size)) {
						return
					}
					if !check("explicit move", win.SetPosition(nil, geometry.Point{X: 100, Y: 60})) {
						return
					}
				case 1:
					if !check("observed explicit move", checkPosition(win, geometry.Point{X: 100, Y: 60})) {
						return
					}
					app.RequestUpdate()
				case 2:
					if !check("rebuild preserves position", checkPosition(win, geometry.Point{X: 100, Y: 60})) {
						return
					}
					if !check("hide", win.PlatformWindow().Hide()) || !check("show again", win.Show()) {
						return
					}
				case 3:
					if !check("Hide/Show preserves position", checkPosition(win, geometry.Point{X: 100, Y: 60})) {
						return
					}
					timer.Stop()
					app.Quit()
				}
				phase++
			})
		}
		return ui.Window("main").Title("GOUI initial center").Size(size.Width, size.Height).
			Chrome(mode).Center(*center).SkipTaskbar(*skip).Content(probe).
			Shortcuts(ui.Shortcut(ui.KeyF5).OnActivate(move),
				ui.Shortcut(ui.KeyF6).OnActivate(func() { preserve(false) }),
				ui.Shortcut(ui.KeyF7).OnActivate(func() { preserve(true) }),
				ui.Shortcut(ui.KeyEscape).OnActivate(app.Quit)).OnDestroy(app.Quit)
	})
	if err != nil {
		return err
	}
	return result
}

type inspectionView struct {
	ui.ViewBase[inspectionView]
	children []ui.View
	inspect  func()
}

func (v *inspectionView) Build() ui.View { return v }
func (v *inspectionView) Mount(ui.BuildContext) gui.Widget {
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.SetSpacing(6)
	return box
}
func (v *inspectionView) Update(ctx ui.BuildContext, widget gui.Widget) {
	ctx.UpdateChildren(widget.(ui.Container), v.children)
	ctx.AfterUpdate(v.inspect)
}
func (*inspectionView) Unmount(ui.BuildContext, gui.Widget) {}

func checkCenter(win gui.Window, size geometry.Size) error {
	area, err := win.WorkAreaAt(geometry.Point{})
	if err != nil {
		return err
	}
	// WorkAreaAt is an observation independent of Position or a stored request:
	// in client-local coordinates, opposite margins must have equal lengths.
	left, top := -area.X, -area.Y
	right, bottom := area.X+area.Width-size.Width, area.Y+area.Height-size.Height
	if math.Abs(float64(left-right)) > 1 || math.Abs(float64(top-bottom)) > 1 {
		return fmt.Errorf("unequal work-area margins: left/top=(%g,%g) right/bottom=(%g,%g)", left, top, right, bottom)
	}
	return checkPosition(win, geometry.Point{X: (area.Width - size.Width) / 2, Y: (area.Height - size.Height) / 2})
}

func checkPosition(win gui.Window, want geometry.Point) error {
	got, err := win.Position(nil)
	if err != nil {
		return err
	}
	if math.Abs(float64(got.X-want.X)) > .5 || math.Abs(float64(got.Y-want.Y)) > .5 {
		return fmt.Errorf("position=%v want=%v (tolerance 0.5 DIP)", got, want)
	}
	return nil
}
