// Declarative ProgressBar window checks, without DevServer.
// Environment: interactive Linux, Windows or macOS. Record OS, Linux WM, backend
// and scale. Run: go run ./tests/window/widgets/ui_progress_bar
// Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows),
// GOUI_PLAT_SCALE=2. Initially Integrated 900x560 DIP, Modern light/blue, 25%
// determinate progress and two busy indicators below.
// Steps and expected results:
//  1. +25% cycles 50/75/100/0%, Reset returns 25%. Bar/ring values agree; the ring
//     grows clockwise from the top, closes at 100% and shows only track at 0%,
//     with stable layout.
//  2. The busy capsule exits right and re-enters left without reversing/jumping.
//     A fixed 90-degree round-cap arc rotates clockwise uniformly every 1.4 seconds,
//     without stretching or cycle gaps. Rebuild retains identity and animation
//     phase. Check inspects four widget IDs, form, value/mode and snapshots;
//     inspect motion and pixels independently. See the legacy timer-check note below.
//  3. Busy toggles the lower controls to saved determinate values and stops
//     animation. Hide toggles their ancestor; showing resumes without remnants.
//     The upper buttons remain usable.
//  4. Dark/Accent changes mode and blue/red without restarting activity; tracks
//     stay neutral. Resizing keeps rings round and drawing within bounds.
//  5. Repeat 1x/2x: 4 DIP spans 4/8 pixels, without cap seams or darkening.
//
// Known test limitation: Check still expects two active Timer objects while busy
// and visible, although ProgressBar now uses Root frame subscriptions. That
// legacy assertion must be updated before treating that Check mode as a passed
// regression. Hidden or determinate mode expects zero timers. Startup alone
// does not verify these cases.
// Platform differences: fonts/decorations and edge antialiasing vary; refresh
// synchronization is not promised. Side effects: window/logs only, no settings,
// clipboard or files. Reset: Reset value, restart all state. Exit: close, check
// timers stopped before loop exit and DESTROY PASS. Assertion failures panic.
package main

import (
	"fmt"
	"image/color"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

type progressApplication struct {
	gui.Application
	timers []*gui.Timer
}

func (a *progressApplication) NewTimer() *gui.Timer {
	timer := a.Application.NewTimer()
	a.timers = append(a.timers, timer)
	return timer
}
func (a *progressApplication) checkTimers(want int) {
	active := 0
	for _, timer := range a.timers {
		if timer.Active() {
			active++
		}
	}
	if active != want {
		panic(fmt.Sprintf("active timers: got %d, want %d", active, want))
	}
}

func main() {
	value, busy, visible, dark, red := float32(.25), true, true, false, false
	ids := []string{"linear", "circular", "busy-linear", "spinner"}
	previous := make(map[string]gui.Widget)
	var monitor *progressApplication
	if err := ui.Run("org.golang-gui.ProgressBarUITest", func(app ui.App) ui.RootView {
		if monitor == nil {
			monitor = &progressApplication{Application: gui.App}
			gui.App = monitor
			gui.App.SetQuitOnLastWindowClosed(false)
		}
		accent := color.RGBA{R: 70, G: 130, B: 220, A: 255}
		if red {
			accent = color.RGBA{R: 210, G: 48, B: 48, A: 255}
		}
		check := func() {
			win := app.FindWindow("progress")
			for i, id := range ids {
				widget := gui.FindWidget(win, id)
				p, ok := widget.(*widgets.ProgressBar)
				if !ok || previous[id] != nil && previous[id] != widget {
					panic("progress widget replaced: " + id)
				}
				previous[id] = widget
				info := p.Snapshot()
				progress := info.Attributes[widgets.ProgressInfoKey].(widgets.ProgressInfo)
				wantBusy, wantShape := i >= 2 && busy, widgets.ProgressLinear
				if i%2 == 1 {
					wantShape = widgets.ProgressCircular
				}
				if info.Role != widgets.RoleProgressBar || info.Range != nil || len(info.Actions) != 0 ||
					p.Value() != value || progress.Shape != wantShape || progress.Indeterminate != wantBusy ||
					wantBusy && progress.Value != nil || !wantBusy && (progress.Value == nil || *progress.Value != value) {
					panic("progress snapshot incorrect: " + id)
				}
			}
			want := 0
			if busy && visible {
				want = 2
			}
			monitor.checkTimers(want)
			fmt.Printf("ASSERT PASS value=%.2f busy=%v visible=%v dark=%v retained=true\n", value, busy, visible, dark)
		}
		row := func(text string, p *wui.ProgressBarView) ui.View {
			return ui.HBox(ui.Label(text).MinSize(160, 0), p).Spacing(12)
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{Dark: dark, AccentColor: accent})).Windows(
			ui.Window("progress").Title("GOUI ProgressBar UI").Size(900, 560).Chrome(ui.WindowChromeIntegrated).
				OnDestroy(func() {
					app.Post(func() {
						monitor.checkTimers(0) // Check host teardown before quitting the application.
						fmt.Println("DESTROY PASS")
						app.Quit()
					})
				}).Content(
				ui.VBox(
					ui.HeaderBar(ui.Label("Progress indicators")),
					ui.VBox(
						ui.Label(fmt.Sprintf("Value %.0f%% | Busy %v | Visible %v | Dark %v | Red %v", value*100, busy, visible, dark, red)),
						ui.HBox(
							ui.Button("+25%").OnClick(func() {
								value += .25
								if value > 1 {
									value = 0
								}
								app.RequestUpdate()
							}),
							ui.Button("Reset").OnClick(func() { value = .25; app.RequestUpdate() }),
							ui.Button("Busy").OnClick(func() { busy = !busy; app.RequestUpdate() }),
							ui.Button("Hide").OnClick(func() { visible = !visible; app.RequestUpdate() }),
							ui.Button("Dark").OnClick(func() { dark = !dark; app.RequestUpdate() }),
							ui.Button("Accent").OnClick(func() { red = !red; app.RequestUpdate() }),
							ui.Button("Rebuild").OnClick(func() {
								count := len(monitor.timers)
								app.RequestUpdate()
								app.Post(func() {
									if len(monitor.timers) != count {
										panic("rebuild allocated new timers")
									}
								})
							}),
							ui.Button("Check").OnClick(check),
						).Spacing(8),
						row("Linear value", wui.ProgressBar(value).ID(ids[0]).MainWeight(1)),
						row("Circular value", wui.ProgressBar(value).ID(ids[1]).Shape(wui.ProgressCircular).MinSize(48, 48).MaxSize(48, 48)),
						ui.VBox(
							row("Linear activity", wui.ProgressBar(value).ID(ids[2]).Indeterminate(busy).MainWeight(1)),
							row("Circular / Spinner", wui.ProgressBar(value).ID(ids[3]).Shape(wui.ProgressCircular).Indeterminate(busy).MinSize(48, 48).MaxSize(48, 48)),
						).CrossAlign(layout.CrossStretch).Spacing(24).Visible(visible),
					).CrossAlign(layout.CrossStretch).Padding(20).Spacing(24),
				).CrossAlign(layout.CrossStretch),
			),
		)
	}); err != nil {
		panic(err)
	}
}
