// Switch state, click/drag, short animation and reconciliation window checks,
// without DevServer.
// Environment: interactive Linux X11, Windows or macOS. Record OS/WM, backend
// and scale. Run: go run ./tests/window/widgets/switch
// Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows),
// GOUI_PLAT_SCALE=2; -fallback-style uses explicit defaults. Separately use
// -destroy-on-change to destroy on the first user change and suppress later callbacks.
// Initially Integrated 900x540 DIP, Modern light/blue: Wi-Fi off, Instant on,
// Parent off, Disabled on. Three enabled controls enter Tab order; chrome may
// fall back to Native.
// Steps and expected results:
//  1. Click Wi-Fi track/label: toggle on release, not press; releasing outside
//     cancels. The thumb transitions over about 120 ms, redirecting from its
//     current position on rapid clicks without jumping. Set silently changes
//     Wi-Fi without Changes; after completion Check/F6 prints ASSERT PASS.
//  2. Drag the thumb horizontally: state/Changes stay fixed until release while
//     the thumb follows. Crossing the midpoint toggles once; staying on the same
//     side retains state without an extra click. A won drag can release outside;
//     Esc/disable/hide/focus loss cancels to committed state. Vertical or label
//     drags do not become thumb drags. Parent gesture yielding is covered by
//     normal-dispatch package tests, since this program has no scroll container.
//  3. Tab/Shift+Tab navigate; Space/Enter toggle once even when held. Only keyboard
//     focus has a softened 2 DIP border. Disabled neither interacts nor takes Tab.
//     Disable/re-enable retains Wi-Fi state. Animated=false removes automatic
//     transitions while still allowing pointer tracking during drag.
//  4. Instant has no animation. Child increments ChildClicks without changing
//     Parent; Parent's track/label toggles it. Rebuild retains identity and one
//     Change connection.
//  5. Dark/Accent changes mode and blue/red, not state. Off uses a subtle neutral
//     outline, on an accent fill, with a white unstroked thumb. Modern on uses
//     fill only; hover/press affects track color, not thumb. Disabled remains
//     distinguishable by position with readable text. At 1x/2x and resizing,
//     tracks stay capsules, thumbs round and content vertically centered within
//     bounds. Inspect on-state blue 1x end curves for smooth partial coverage,
//     not hardened overlapping fill/stroke. Dark/disabled/hover/press retain soft
//     edges. Explicit fallback retains its existing stroke and is outside this
//     Modern appearance check.
//  6. Check/F6 asserts state/snapshot/identity and arranged window-local DIP
//     BOUNDS. After 150 ms ActiveTimers=0; inspect motion separately. Closing
//     checks timers stopped and widgets destroyed, then prints DESTROY PASS.
//  7. -destroy-on-change: F6, then click Wi-Fi. One Change, LateChanges=0,
//     DESTROY PASS and normal exit, with no timer leak or panic.
//
// Platform differences: fonts/decorations/antialiasing vary; animation is not
// promised to synchronize with vertical refresh. Side effects: window/stdout only,
// no files/settings/clipboard. Reset: Reset state/counts and enable Wi-Fi;
// restart restores theme/all options. Exit: close; assertion failures panic.
package main

import (
	"flag"
	"fmt"
	"image/color"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	widgetstyle "github.com/golang-gui/goui/widgets/style"
	wui "github.com/golang-gui/goui/widgets/ui"
)

type switchApplication struct {
	gui.Application
	timers []*gui.Timer
}

func (a *switchApplication) NewTimer() *gui.Timer {
	timer := a.Application.NewTimer()
	a.timers = append(a.timers, timer)
	return timer
}
func (a *switchApplication) activeTimers() int {
	active := 0
	for _, timer := range a.timers {
		if timer.Active() {
			active++
		}
	}
	return active
}

func main() {
	fallback := flag.Bool("fallback-style", false, "使用 widgets 显式兜底样式")
	destroyOnChange := flag.Bool("destroy-on-change", false, "首次用户切换后销毁窗口")
	flag.Parse()
	wifi, instant, parent := false, true, false
	enabled, animated, visible, dark, red := true, true, true, false, false
	changes, childClicks, lateChanges := 0, 0, 0
	previous := make(map[string]*widgets.Switch)
	var monitor *switchApplication
	var lateConnected bool
	if err := ui.Run("org.golang-gui.SwitchTest", func(app ui.App) ui.RootView {
		if monitor == nil {
			monitor = &switchApplication{Application: gui.App}
			gui.App = monitor
			gui.App.SetQuitOnLastWindowClosed(false)
		}
		update := func() { app.RequestUpdate() }
		change := func(value *bool) func(bool) {
			return func(checked bool) {
				*value = checked
				changes++
				if *destroyOnChange {
					app.FindWindow("switches").Destroy()
				} else {
					update()
				}
			}
		}
		check := func() {
			win := app.FindWindow("switches")
			for id, want := range map[string]bool{"wifi": wifi, "instant": instant, "parent": parent, "disabled": true} {
				b, ok := gui.FindWidget(win, id).(*widgets.Switch)
				if !ok || b.Checked() != want || previous[id] != nil && previous[id] != b {
					panic("state/identity mismatch: " + id)
				}
				previous[id] = b
				info := b.Snapshot()
				if info.Role != widgets.RoleSwitch || info.Attributes[widgets.SwitchInfoKey].(widgets.SwitchInfo).Checked != want {
					panic("snapshot mismatch: " + id)
				}
				if b.Enabled() != (id != "disabled" && (id != "wifi" || enabled)) {
					panic("enabled mismatch: " + id)
				}
				if b.Padding() != 6 || b.Animated() != (id != "instant" && animated) {
					panic("default/animation mismatch: " + id)
				}
			}
			if !lateConnected {
				previous["wifi"].ConnectChange(func(bool) { lateChanges++ })
				lateConnected = true
			}
			for _, id := range []string{"wifi", "instant", "parent", "child", "disabled", "set", "disable", "animated", "hide", "dark", "accent", "rebuild", "check", "reset"} {
				i := gui.FindWidget(win, id).Snapshot()
				fmt.Printf("BOUNDS %s %.2f %.2f %.2f %.2f\n", id, i.Bounds.X, i.Bounds.Y, i.Bounds.Width, i.Bounds.Height)
				if i.Focused {
					fmt.Printf("FOCUS %s visible=%v\n", id, i.FocusVisible)
				}
			}
			fmt.Printf("ASSERT PASS wifi=%v instant=%v parent=%v changes=%d childClicks=%d enabled=%v animated=%v visible=%v dark=%v red=%v activeTimers=%d retained=true\n", wifi, instant, parent, changes, childClicks, enabled, animated, visible, dark, red, monitor.activeTimers())
		}
		reset := func() {
			wifi, instant, parent = false, true, false
			changes, childClicks = 0, 0
			enabled, visible = true, true
			update()
		}
		accent := color.RGBA{R: 70, G: 130, B: 220, A: 255}
		if red {
			accent = color.RGBA{R: 210, G: 48, B: 48, A: 255}
		}
		settings := app.Settings()
		sheet := modern.Sheet(modern.Options{Dark: dark, AccentColor: accent, FontFamily: settings.FontFamily(), FontSize: settings.FontSize()})
		if *fallback {
			sheet = style.Sheet(append(gui.DefaultStyleRules(), widgetstyle.Rules()...)...)
		}
		return ui.Root().StyleSheet(sheet).Windows(
			ui.Window("switches").Title("GOUI Switch").Size(900, 540).Chrome(ui.WindowChromeIntegrated).
				OnDestroy(func() {
					app.Post(func() {
						if monitor.activeTimers() != 0 {
							panic("destroy retained timers")
						}
						for _, b := range previous {
							if !b.Destroyed() {
								panic("window retained live widget")
							}
						}
						if *destroyOnChange && (changes != 1 || lateChanges != 0) {
							panic("destroyed transition continued")
						}
						fmt.Println("DESTROY PASS")
						app.Quit()
					})
				}).Content(
				ui.VBox(
					ui.HeaderBar(ui.Label("Switch")),
					ui.VBox(
						ui.Label(fmt.Sprintf("Changes %d | ChildClicks %d | Wi-Fi %v | Dark %v", changes, childClicks, wifi, dark)),
						ui.HBox(
							ui.Button("Set").ID("set").OnClick(func() { wifi = !wifi; update() }),
							ui.Button("Disable").ID("disable").OnClick(func() { enabled = !enabled; update() }),
							ui.Button("Animated").ID("animated").OnClick(func() { animated = !animated; update() }),
							ui.Button("Hide").ID("hide").OnClick(func() { visible = !visible; update() }),
							ui.Button("Dark").ID("dark").OnClick(func() { dark = !dark; update() }),
							ui.Button("Accent").ID("accent").OnClick(func() { red = !red; update() }),
							ui.Button("Rebuild").ID("rebuild").OnClick(update),
							ui.Button("Check").ID("check").OnClick(check),
							ui.Button("Reset").ID("reset").OnClick(reset),
						).Spacing(8),
						wui.Switch("Wi-Fi").ID("wifi").Checked(wifi).Enabled(enabled).Animated(animated).Visible(visible).OnChange(change(&wifi)),
						wui.Switch("Instant").ID("instant").Checked(instant).Animated(false).OnChange(change(&instant)),
						wui.Switch().ID("parent").Checked(parent).Animated(animated).OnChange(change(&parent)).Content(
							ui.HBox(ui.Label("Parent"), ui.Button("Child").ID("child").OnClick(func() { childClicks++; update() })).Spacing(12)),
						wui.Switch("Disabled").ID("disabled").Checked(true).Enabled(false).Animated(animated),
						ui.Label("Click / drag horizontally; Tab / Shift+Tab, Space / Enter; Esc cancels drag."),
					).Padding(20).Spacing(18).CrossAlign(layout.CrossStart),
				).CrossAlign(layout.CrossStretch).Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(check))),
		)
	}); err != nil {
		panic(err)
	}
}
