// CheckButton tristate, exclusive-group, button-appearance and reconciliation
// window checks, without DevServer.
// Environment: interactive Linux X11, Windows or macOS. Record OS/WM, backend
// and scale. Run: go run ./tests/window/widgets/check_button
// Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows) and
// GOUI_PLAT_SCALE=2. -fallback-style uses explicit defaults. Separately run
// -destroy-on-change: the old member's first Radio B/C change destroys the window.
// Initially Integrated 900x640 DIP, light/blue, Independent unchecked, Mixed mixed,
// Radio A/Mode A checked, Toggle unchecked beside an ordinary Button with the
// same text, and Disabled inactive. Chrome may fall back to Native. Modern uses
// Settings UI font/size; explicit fallback keeps fixed defaults.
// Steps and expected results:
//  1. Both box and label toggle Independent on release, not press; release
//     outside cancels. Clicking Mixed checks it; Mixed silently restores partial
//     state without adding Changes.
//  2. Radio B/C unchecks the old member before checking the new one; reselecting
//     it does not clear it. Clear empties both groups, which remain usable.
//     Toggle is independent; Mode A/B are exclusive button-style choices.
//  3. Tab/Shift+Tab navigate; Space/Enter toggle once even when held. Keyboard
//     focus alone has a softened 2 DIP border. Disabled neither interacts nor
//     enters Tab order. Disable stops Independent; re-enable restores participation.
//  4. Child increments ChildClicks without toggling Parent; Parent's label toggles
//     it. Rebuild retains identity/state and one Change per operation.
//  5. Dark/Accent changes light/dark and blue/red while preserving selection and
//     readable content. Resize/repeat 1x/2x: circular shapes stay round and marks
//     and borders remain clear within bounds. Ordinary Button and unchecked
//     Toggle share size, fill, border and radius; selected Toggle changes only
//     to neutral gray, unaffected by Accent. Indicators are 18 DIP; boxes have
//     4 DIP corners and a solid selected fill with white check/dash. Selected
//     radio uses the same-state solid fill and a white dot without a stroke.
//     Modern uses accent, Basic fixed blue. Unselected radios are neutral outlines;
//     selected fills darken on hover/press. Disabled selection is neutral with
//     gray marks. Basic selected boxes/radios use one fill; inspect smooth 1x
//     corners and stable hover/press geometry. Button-style controls add no sudden
//     accent border, size or content shift. Basic ignores Dark/Accent.
//  6. Check/F6 asserts state, groups, snapshots and identity, printing ASSERT PASS.
//     BOUNDS is arranged window-local DIP for locating controls, not an action
//     API. Inspect appearance/gestures separately. Closing prints DESTROY PASS;
//     both groups have no selected member.
//  7. Separately -destroy-on-change: F6 then click Radio B. Only the old member's
//     one Change runs; later callbacks access no destroyed tree. DESTROY PASS,
//     normal exit, no panic.
//
// Platform differences: fonts, decoration and antialiasing vary; group selection
// semantics agree. Group arrow navigation is not implemented.
// Side effects: window/stdout only, no settings/files/clipboard. Reset: Reset
// selections/counts, restart for theme/all state. Exit: close the main window.
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

func main() {
	fallback := flag.Bool("fallback-style", false, "使用显式组合的 widgets 兜底样式")
	destroyOnChange := flag.Bool("destroy-on-change", false, "旧选中成员的 Change 回调销毁窗口")
	flag.Parse()
	radioGroup, modeGroup := widgets.NewCheckGroup(), widgets.NewCheckGroup()
	independent, mixed, parent := wui.CheckUnchecked, wui.CheckMixed, wui.CheckUnchecked
	radio, mode := 1, 1
	toggle, enabled, dark, red := false, true, false, false
	changes, childClicks, ordinaryClicks := 0, 0, 0
	lateChanges := 0
	previous := make(map[string]gui.Widget)
	if err := ui.Run("org.golang-gui.CheckButtonTest", func(app ui.App) ui.RootView {
		gui.App.SetQuitOnLastWindowClosed(false)
		update := func() { app.RequestUpdate() }
		reset := func() {
			independent, mixed, parent = wui.CheckUnchecked, wui.CheckMixed, wui.CheckUnchecked
			radio, mode, toggle, enabled, changes, childClicks = 1, 1, false, true, 0, 0
			ordinaryClicks = 0
			update()
		}
		change := func(value *wui.CheckState) func(wui.CheckState) {
			return func(s wui.CheckState) { *value = s; changes++; update() }
		}
		choice := func(value *int, index int) func(wui.CheckState) {
			return func(s wui.CheckState) {
				if s == wui.CheckChecked {
					*value = index
				}
				changes++
				if *destroyOnChange && value == &radio && index == 1 && s == wui.CheckUnchecked {
					app.FindWindow("checks").Destroy()
					return
				}
				update()
			}
		}
		check := func() {
			win := app.FindWindow("checks")
			settings := app.Settings()
			fmt.Printf("FONT family=%q size=%g\n", settings.FontFamily(), settings.FontSize())
			want := map[string]wui.CheckState{"independent": independent, "mixed": mixed, "parent": parent, "disabled": wui.CheckChecked}
			want["toggle"] = wui.CheckUnchecked
			if toggle {
				want["toggle"] = wui.CheckChecked
			}
			for i := 1; i <= 3; i++ {
				want[fmt.Sprintf("radio-%d", i)] = wui.CheckUnchecked
				if radio == i {
					want[fmt.Sprintf("radio-%d", i)] = wui.CheckChecked
				}
			}
			for i := 1; i <= 2; i++ {
				want[fmt.Sprintf("mode-%d", i)] = wui.CheckUnchecked
				if mode == i {
					want[fmt.Sprintf("mode-%d", i)] = wui.CheckChecked
				}
			}
			for id, state := range want {
				b, ok := gui.FindWidget(win, id).(*widgets.CheckButton)
				if !ok || b.CheckState() != state || previous[id] != nil && previous[id] != b {
					panic("state/identity incorrect: " + id)
				}
				if b.Padding() != 6 {
					panic("default padding incorrect: " + id)
				}
				if *destroyOnChange && previous[id] == nil && (id == "radio-1" || id == "radio-2") {
					// Raw GUI connections are deliberately not owned by UI teardown:
					// safety must come from the transition's lifetime check.
					b.ConnectChange(func(widgets.CheckState) { lateChanges++ })
				}
				previous[id] = b
				info := b.Snapshot()
				data := info.Attributes[widgets.CheckInfoKey].(widgets.CheckInfo)
				if info.Focused {
					fmt.Printf("FOCUS %s visible=%v\n", id, info.FocusVisible)
				}
				if data.State != state || len(info.Children) != 1 {
					panic("snapshot incorrect: " + id)
				}
				if id == "disabled" && (b.Enabled() || gui.IsEnabled(b) || !b.Focusable()) {
					panic("disabled is focusable")
				}
				if id == "independent" && b.Enabled() != enabled {
					panic("enabled mismatch")
				}
				if id[:min(len(id), 6)] == "radio-" && (b.Group() != radioGroup || info.Role != widgets.RoleRadioButton) {
					panic("radio group/role mismatch")
				}
				if id == "toggle" && info.Role != widgets.RoleToggleButton {
					panic("toggle role mismatch")
				}
			}
			if (radioGroup.Checked() == nil) != (radio == 0) || (modeGroup.Checked() == nil) != (mode == 0) {
				panic("group selection mismatch")
			}
			if gui.FindWidget(win, "ordinary").Rect().Size != gui.FindWidget(win, "toggle").Rect().Size {
				panic("ordinary Button and Toggle text/padding allocation differs")
			}
			for _, id := range []string{"independent", "mixed", "radio-1", "radio-2", "radio-3", "ordinary", "toggle", "mode-1", "mode-2", "parent", "child", "disabled", "mixed-reset", "clear", "disable", "dark", "accent", "rebuild", "check", "reset"} {
				b := gui.FindWidget(win, id).Snapshot().Bounds
				fmt.Printf("BOUNDS %s %.2f %.2f %.2f %.2f\n", id, b.X, b.Y, b.Width, b.Height)
			}
			fmt.Printf("ASSERT PASS independent=%d mixed=%d radio=%d toggle=%v mode=%d parent=%d changes=%d childClicks=%d ordinaryClicks=%d enabled=%v dark=%v red=%v retained=true\n", independent, mixed, radio, toggle, mode, parent, changes, childClicks, ordinaryClicks, enabled, dark, red)
		}
		accent := color.RGBA{R: 70, G: 130, B: 220, A: 255}
		if red {
			accent = color.RGBA{R: 210, G: 48, B: 48, A: 255}
		}
		settings := app.Settings()
		sheet := modern.Sheet(modern.Options{
			Dark: dark, AccentColor: accent,
			FontFamily: settings.FontFamily(), FontSize: settings.FontSize(),
		})
		if *fallback {
			sheet = style.Sheet(append(gui.DefaultStyleRules(), widgetstyle.Rules()...)...)
		}
		return ui.Root().StyleSheet(sheet).Windows(
			ui.Window("checks").Title("GOUI CheckButton").Size(900, 640).Chrome(ui.WindowChromeIntegrated).
				OnDestroy(func() {
					app.Post(func() {
						if radioGroup.Checked() != nil || modeGroup.Checked() != nil {
							panic("destroyed selection retained")
						}
						if *destroyOnChange && (changes != 1 || lateChanges != 0) {
							panic("destroyed transition continued notification")
						}
						fmt.Println("DESTROY PASS")
						app.Quit()
					})
				}).Content(
				ui.VBox(
					ui.HeaderBar(ui.Label("CheckButton")),
					ui.VBox(
						ui.Label(fmt.Sprintf("Changes %d | ChildClicks %d | ButtonClicks %d | Dark %v | Accent red %v", changes, childClicks, ordinaryClicks, dark, red)),
						ui.HBox(
							ui.Button("Mixed").ID("mixed-reset").OnClick(func() { mixed = wui.CheckMixed; update() }),
							ui.Button("Clear").ID("clear").OnClick(func() { radio, mode = 0, 0; update() }),
							ui.Button("Disable").ID("disable").OnClick(func() { enabled = !enabled; update() }),
							ui.Button("Dark").ID("dark").OnClick(func() { dark = !dark; update() }),
							ui.Button("Accent").ID("accent").OnClick(func() { red = !red; update() }),
							ui.Button("Rebuild").ID("rebuild").OnClick(update),
							ui.Button("Check").ID("check").OnClick(check),
							ui.Button("Reset").ID("reset").OnClick(reset),
						).Spacing(8),
						wui.CheckButton("Independent").ID("independent").CheckState(independent).Enabled(enabled).OnChange(change(&independent)),
						wui.CheckButton("Mixed").ID("mixed").CheckState(mixed).OnChange(change(&mixed)),
						ui.HBox(
							wui.CheckButton("Radio A").ID("radio-1").Group(radioGroup).Checked(radio == 1).OnChange(choice(&radio, 1)),
							wui.CheckButton("Radio B").ID("radio-2").Group(radioGroup).Checked(radio == 2).OnChange(choice(&radio, 2)),
							wui.CheckButton("Radio C").ID("radio-3").Group(radioGroup).Checked(radio == 3).OnChange(choice(&radio, 3)),
						).Spacing(12),
						ui.HBox(
							ui.Button("Toggle").ID("ordinary").OnClick(func() { ordinaryClicks++; update() }),
							wui.CheckButton("Toggle").ID("toggle").Appearance(wui.CheckAppearanceButton).Checked(toggle).OnChange(func(s wui.CheckState) { toggle = s == wui.CheckChecked; changes++; update() }),
							wui.CheckButton("Mode A").ID("mode-1").Appearance(wui.CheckAppearanceButton).Group(modeGroup).Checked(mode == 1).OnChange(choice(&mode, 1)),
							wui.CheckButton("Mode B").ID("mode-2").Appearance(wui.CheckAppearanceButton).Group(modeGroup).Checked(mode == 2).OnChange(choice(&mode, 2)),
						).Spacing(12),
						wui.CheckButton().ID("parent").CheckState(parent).OnChange(change(&parent)).Content(
							ui.HBox(ui.Label("Parent"), ui.Button("Child").ID("child").OnClick(func() { childClicks++; update() })).Spacing(12),
						),
						wui.CheckButton("Disabled").ID("disabled").Checked(true).Enabled(false),
						ui.Label("Tab / Shift+Tab, Space / Enter; click Child without toggling Parent."),
					).Padding(20).Spacing(18).CrossAlign(layout.CrossStart),
				).CrossAlign(layout.CrossStretch).Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(check)),
			),
		)
	}); err != nil {
		panic(err)
	}
}
