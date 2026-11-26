// GOUI system monitor: a project demo that favors a finished look over API
// coverage. All activity is simulated; it never reads the real system.
//
// Environment: a desktop session; Linux requires X11. Choose the backend and
// scale with GOUI_PLAT_PAINTER and GOUI_PLAT_SCALE.
// Run: go run ./example/monitor. It starts in the dark palette.
// Steps and expected results:
//  1. Every second the headline values glide to new readings, the CPU chart
//     and network sparkline slide left by one sample, and the process table
//     updates. The CPU ring and chart turn red above 80%.
//  2. Click the CPU, memory or name column headers to sort; selection follows
//     the process, not the row position.
//  3. Click the sun/moon button in the header to switch between dark and light;
//     charts redraw in the new colors without losing history.
//  4. Resize the window: the cards share the width, the chart stretches and
//     the table takes the remaining height.
//
// Side effects: none. Reset: restart. Exit: close the window.
package main

import (
	"log"
	"time"

	"github.com/golang-gui/goui/animation"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// sampleInterval is the simulated sampling period and the duration of every
// transition between two samples.
const sampleInterval = time.Second

// monitor owns the simulation and every piece of presentation state.
type monitor struct {
	app       ui.App
	icons     themeIcons
	sim       *simulator
	dark      bool
	shown     ui.State[readings]
	values    *animation.Animation[readings]
	scroll    float32
	slide     *animation.Animation[float32]
	processes processModel
	sortBy    ui.State[string]
	sortOrder ui.State[widgets.SortOrder]
	selection ui.State[[]string]
	ticker    *ui.Timer
}

func newMonitor() (*monitor, error) {
	icons, err := loadThemeIcons()
	if err != nil {
		return nil, err
	}
	m := &monitor{icons: icons, sim: newSimulator(), dark: true, scroll: 1}
	m.shown = ui.MakeState(m.sim.current)
	m.values = animation.New(m.sim.current, lerpReadings)
	m.values.ConnectUpdate(m.shown.Set)
	m.slide = animation.New(float32(1), animation.Float32)
	m.slide.ConnectUpdate(func(v float32) { m.scroll = v })
	m.sortBy = ui.MakeState("cpu")
	m.sortOrder = ui.MakeState(widgets.SortDescending)
	m.processes = processModel{gui.NewSliceListModel(m.sim.sortedProcesses("cpu", widgets.SortDescending))}
	return m, nil
}

func (m *monitor) palette() palette {
	if m.dark {
		return darkPalette
	}
	return lightPalette
}

// tick advances the simulation and starts the transitions towards it.
func (m *monitor) tick() {
	m.sim.step()
	m.processes.SetItems(m.sim.sortedProcesses(m.sortBy.Peek(), m.sortOrder.Peek()))
	window := m.app.FindWindow("monitor")
	if window == nil {
		m.values.SetValue(m.sim.current)
		m.slide.SetValue(1)
		return
	}
	// Both transitions last a whole sample interval at constant speed. An
	// easing curve that settles early makes the display stop and restart
	// every second, which reads as stutter.
	m.slide.SetValue(0)
	if err := m.slide.AnimateTo(window, 1, sampleInterval, animation.Linear); err != nil {
		m.slide.SetValue(1)
	}
	if err := m.values.AnimateTo(window, m.sim.current, sampleInterval, animation.Linear); err != nil {
		m.values.SetValue(m.sim.current)
	}
}

func (m *monitor) stop() {
	if m.ticker != nil {
		m.ticker.Stop()
		m.ticker = nil
	}
	m.values.Stop()
	m.slide.Stop()
}

func (m *monitor) toggleTheme() {
	m.dark = !m.dark
	m.app.RequestUpdate() // The root owns the style sheet.
}

func (m *monitor) build(app ui.App) ui.RootView {
	m.app = app
	if m.ticker == nil {
		m.ticker = app.TickFunc(sampleInterval, m.tick)
	}
	rules := styleRules(m.palette(), m.dark, app.Settings().FontFamily())
	window := ui.Window("monitor").Title("GOUI Monitor").Chrome(ui.WindowChromeIntegrated).
		Size(1080, 780).MinSize(820, 620).Center(true).
		OnDestroy(func() { m.stop(); app.Quit() }).
		Content(m.view())
	return ui.Root().StyleSheet(style.Sheet(rules...)).Windows(window)
}

func main() {
	m, err := newMonitor()
	if err != nil {
		log.Fatal(err)
	}
	defer m.stop()
	if err := ui.Run("org.golanggui.Monitor", m.build); err != nil {
		log.Fatal(err)
	}
}
