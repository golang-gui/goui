// Imperative ProgressBar window checks, without DevServer.
// Environment: interactive Linux, Windows or macOS. Record OS, Linux WM, backend
// and scale. Run: go run ./tests/window/widgets/progress_bar
// Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows),
// GOUI_PLAT_SCALE=2. Initially Integrated 900x560 DIP, Modern light/blue, 25%
// determinate progress and two busy indicators below.
// Steps and expected results:
//  1. +25% cycles 50/75/100/0%; Reset returns 25%. Bar length is proportional;
//     the ring grows clockwise from the top, closes seamlessly at 100% and shows
//     only a track at 0%. Changes leave layout stable.
//  2. The lower capsule enters from the left and exits right, repeating without
//     reversing or jumping. A fixed 90-degree round-cap arc rotates clockwise
//     uniformly every 1.4 seconds without stretching, reversal or cycle gaps.
//     Busy toggles to determinate at the saved value and back to animation.
//     Waiting changes neither the upper value nor layout.
//  3. Hide toggles their shared ancestor; showing restores animation without
//     hidden remnants.
//  4. Dark/Accent changes mode and blue/red while tracks stay neutral, with
//     unchanged size/value/mode. Resizing leaves rings circular and within bounds.
//  5. Check prints ASSERT PASS for role, form, value or omitted busy value,
//     without adjustable Range/click actions. After mode/hide changes wait
//     100 ms before Check. Root frames drive busy animation without a per-widget
//     Timer; this assertion does not verify visual motion.
//  6. Repeat at 1x/2x: 4 DIP equals 4/8 pixels, with no cap seams/darkening.
//
// Platform differences: font/decorations vary, with minor edge antialiasing
// differences; refresh synchronization is not promised. Side effects: window/logs
// only, no settings/clipboard/files. Reset: Reset values, restart all state.
// Exit: close; check all timers stopped before event-loop exit and DESTROY PASS.
// Assertion failures panic.
package main

import (
	"fmt"
	"image/color"
	"log"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

// Only observe public Timer contracts; no control/scheduler private state.
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
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	app, err := gui.NewApplication("org.golang-gui.ProgressBarTest")
	if err != nil {
		log.Fatal(err)
	}
	monitor := &progressApplication{Application: app}
	gui.App = monitor
	app.SetQuitOnLastWindowClosed(false)
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 900, Height: 560}, Chrome: gui.WindowChromeIntegrated})
	if err != nil {
		log.Fatal(err)
	}
	if err := win.SetTitle("GOUI ProgressBar GUI"); err != nil {
		log.Fatal(err)
	}
	win.ConnectDestroy(func() {
		app.Post(func() {
			monitor.checkTimers(0) // Before Quit: host teardown, not app-wide cancellation.
			fmt.Println("DESTROY PASS")
			app.Quit()
		})
	})
	value, busy, visible, dark, red := float32(.25), true, true, false, false
	all := []*widgets.ProgressBar{widgets.NewProgressBar(), widgets.NewProgressBar(), widgets.NewProgressBar(), widgets.NewProgressBar()}
	for i, p := range all {
		p.SetID(fmt.Sprintf("progress-%d", i))
		if i%2 == 1 {
			p.SetShape(widgets.ProgressCircular)
			p.SetMinSize(geometry.Size{Width: 48, Height: 48})
			p.SetMaxSize(geometry.Size{Width: 48, Height: 48})
		} else {
			p.SetMainWeight(1)
		}
	}
	row := func(text string, p *widgets.ProgressBar) *gui.LinearBox {
		b := gui.NewLinearBox(layout.DirectionHorizontal)
		b.SetSpacing(12)
		label := gui.NewLabel(text)
		label.SetMinSize(geometry.Size{Width: 160})
		b.AddChild(label)
		b.AddChild(p)
		return b
	}
	activity := gui.NewLinearBox(layout.DirectionVertical)
	activity.SetCrossAlign(layout.CrossStretch)
	activity.SetSpacing(24)
	activity.AddChild(row("Linear activity", all[2]))
	activity.AddChild(row("Circular / Spinner", all[3]))
	status := gui.NewLabel("")
	refresh := func() {
		accent := color.RGBA{R: 70, G: 130, B: 220, A: 255}
		if red {
			accent = color.RGBA{R: 210, G: 48, B: 48, A: 255}
		}
		app.SetStyleSheet(modern.Sheet(modern.Options{Dark: dark, AccentColor: accent}))
		for i, p := range all {
			p.SetValue(value)
			p.SetIndeterminate(i >= 2 && busy)
		}
		activity.SetVisible(visible)
		status.SetText(fmt.Sprintf("Value %.0f%% | Busy %v | Visible %v | Dark %v | Red %v", value*100, busy, visible, dark, red))
	}
	check := func() {
		for i, p := range all {
			info := p.Snapshot()
			progress := info.Attributes[widgets.ProgressInfoKey].(widgets.ProgressInfo)
			wantBusy, wantShape := i >= 2 && busy, widgets.ProgressLinear
			if i%2 == 1 {
				wantShape = widgets.ProgressCircular
			}
			if info.Role != widgets.RoleProgressBar || info.Focusable || len(info.Actions) != 0 || info.Range != nil ||
				progress.Shape != wantShape || progress.Indeterminate != wantBusy ||
				wantBusy && progress.Value != nil || !wantBusy && (progress.Value == nil || *progress.Value != value) {
				panic(fmt.Sprintf("progress %d incorrect: %+v", i, info))
			}
		}
		if all[0].Rect().Width > 0 { // Startup Check precedes the first layout.
			monitor.checkTimers(0)
		}
		fmt.Printf("ASSERT PASS value=%.2f busy=%v visible=%v dark=%v\n", value, busy, visible, dark)
	}
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetSpacing(8)
	button := func(text string, fn func()) {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		b.ConnectClicked(fn)
		toolbar.AddChild(b)
	}
	button("+25%", func() {
		value += .25
		if value > 1 {
			value = 0
		}
		refresh()
	})
	button("Reset", func() { value = .25; refresh() })
	button("Busy", func() { busy = !busy; refresh() })
	button("Hide", func() { visible = !visible; refresh() })
	button("Dark", func() { dark = !dark; refresh() })
	button("Accent", func() { red = !red; refresh() })
	button("Check", check)
	body := gui.NewLinearBox(layout.DirectionVertical)
	body.SetCrossAlign(layout.CrossStretch)
	body.SetPadding(20)
	body.SetSpacing(24)
	for _, w := range []gui.Widget{status, toolbar, row("Linear value", all[0]), row("Circular value", all[1]), activity} {
		body.AddChild(w)
	}
	header := gui.NewHeaderBar()
	header.SetChild(gui.NewLabel("Progress indicators"))
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	root.AddChild(header)
	root.AddChild(body)
	refresh()
	win.SetWidget(root)
	if err := win.Show(); err != nil {
		log.Fatal(err)
	}
	check()
	app.Run()
}
