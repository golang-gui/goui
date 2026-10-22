// Default Basic and optional Modern theme window checks, without DevServer.
// Environment: interactive Linux X11, Windows or macOS. Record OS/WM, backend
// and scale. Run: go run ./tests/window/ui/themes
// Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows),
// GOUI_PLAT_SCALE=2 for application 2x. Initially Integrated 640x460 DIP, no
// custom sheet, HeaderBar enabled, CheckButton/Switch selected, and two disabled
// controls that cannot interact.
// Steps and expected results:
//  1. Default Basic paints check box/mark, switch track/thumb and labels. Clicking
//     either synchronizes both and HeaderBar enabled state; disabled examples
//     do not change. Selected boxes/on tracks add no same-color stroke; the blue
//     switch is softened, and unchecked controls retain neutral outlines.
//  2. Explicit Basic matches defaults. System Modern uses system mode/accent/font;
//     Dark Modern adapts backgrounds/text. Selection, counts, widget identity and
//     HeaderBar drag behavior survive changes.
//  3. Default Basic restores a nil sheet without adding widgets/style manually;
//     Basic fonts/colors return with visible controls. System Modern responds to
//     system mode/accent through Settings-triggered rebuilds; explicit modes stay fixed.
//  4. Tab/Shift+Tab navigate and Space toggles. Only keyboard focus shows a 2 DIP
//     border; disabled controls cannot focus, and pointer focus adds no keyboard hint.
//  5. F6/Check prints ASSERT PASS and BOUNDS, checking state, snapshots, identity,
//     indicator areas and nonempty labels. Inspect pixels separately. Repeat
//     1x/2x without clipped corners/text/thumbs, then close normally.
//
// Platform differences: fonts, antialiasing and decorations vary; verify
// Windows/macOS separately. Side effects: window/logs only. The tester may change
// system settings for step 3 and must restore them. Reset: Reset or restart to
// restore mode. Exit: close the main window.
package main

import (
	"fmt"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

func main() {
	mode, changes := 0, 0
	enabled := ui.MakeState(true)
	previous := make(map[string]gui.Widget)
	modeNames := []string{"默认 Basic", "显式 Basic", "系统 Modern", "深色 Modern"}
	if err := ui.Run("org.golang-gui.ThemeTest", func(app ui.App) ui.RootView {
		var sheet style.StyleSheet
		switch mode {
		case 1:
			sheet = basic.Sheet()
		case 2:
			sheet = modern.FromSettings(app.Settings())
		case 3:
			s := app.Settings()
			sheet = modern.Sheet(modern.Options{Dark: true, AccentColor: s.AccentColor(), FontFamily: s.FontFamily(), FontSize: s.FontSize()})
		}
		change := func(value bool) { changes++; enabled.Set(value) }
		check := func() {
			win := app.FindWindow("themes")
			if (gui.App.StyleSheet() == nil) != (mode == 0) {
				panic("explicit stylesheet/nil contract changed")
			}
			for _, id := range []string{"header", "check", "switch", "disabled-check", "disabled-switch"} {
				w := gui.FindWidget(win, id)
				if w == nil || w.Rect().Width <= 0 || w.Rect().Height <= 0 || previous[id] != nil && previous[id] != w {
					panic("missing geometry or replaced widget: " + id)
				}
				previous[id] = w
				info := w.Snapshot()
				if info.Enabled != (id != "disabled-check" && id != "disabled-switch" && (id != "header" || enabled.Get())) {
					panic("enabled snapshot mismatch: " + id)
				}
				switch w := w.(type) {
				case *widgets.CheckButton:
					want := enabled.Get() || id == "disabled-check"
					if w.Checked() != want || info.Attributes[widgets.CheckInfoKey].(widgets.CheckInfo).State != w.CheckState() {
						panic("checkbox state/snapshot mismatch")
					}
				case *widgets.Switch:
					want := enabled.Get() || id == "disabled-switch"
					if w.Checked() != want || info.Attributes[widgets.SwitchInfoKey].(widgets.SwitchInfo).Checked != want {
						panic("switch state/snapshot mismatch")
					}
				}
				if len(info.Children) != 1 || info.Children[0].Text == "" || info.Children[0].Bounds.Width <= 0 || info.Children[0].Bounds.Height <= 0 {
					panic("independent content label missing: " + id)
				}
			}
			for _, id := range []string{"mode-0", "mode-1", "mode-2", "mode-3", "check", "switch", "verify", "reset"} {
				info := gui.FindWidget(win, id).Snapshot()
				fmt.Printf("BOUNDS %s %.2f %.2f %.2f %.2f focused=%v focusVisible=%v\n", id, info.Bounds.X, info.Bounds.Y, info.Bounds.Width, info.Bounds.Height, info.Focused, info.FocusVisible)
			}
			fmt.Printf("ASSERT PASS mode=%d enabled=%v changes=%d retained=true\n", mode, enabled.Get(), changes)
		}
		modes := make([]ui.View, len(modeNames))
		for i, name := range modeNames {
			modes[i] = ui.Button(name).ID(fmt.Sprintf("mode-%d", i)).OnClick(func() { mode = i; app.RequestUpdate() })
		}
		return ui.Root().StyleSheet(sheet).Windows(
			ui.Window("themes").Title("GOUI Basic / Modern").Chrome(ui.WindowChromeIntegrated).Size(640, 460).
				Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(check)).
				Content(ui.VBox(
					ui.HeaderBar(ui.Label("HeaderBar")).ID("header").Enabled(enabled.Get()),
					ui.VBox(
						ui.HWrap(modes...).Spacing(8).LineSpacing(8),
						ui.Label(fmt.Sprintf("模式：%s；启用：%v；变化次数：%d", modeNames[mode], enabled.Get(), changes)),
						wui.CheckButton("HeaderBar CheckButton").ID("check").Checked(enabled.Get()).OnChange(func(s wui.CheckState) { change(s != wui.CheckUnchecked) }),
						wui.Switch("HeaderBar Switch").ID("switch").Checked(enabled.Get()).OnChange(change),
						wui.CheckButton("Disabled CheckButton").ID("disabled-check").Checked(true).Enabled(false),
						wui.Switch("Disabled Switch").ID("disabled-switch").Checked(true).Enabled(false),
						ui.HBox(
							ui.Button("检查 / F6").ID("verify").OnClick(check),
							ui.Button("复位").ID("reset").OnClick(func() { mode, changes = 0, 0; enabled.Set(true) }),
						).Spacing(8),
					).Padding(16).Spacing(12),
				).CrossAlign(layout.CrossStretch)),
		)
	}); err != nil {
		panic(err)
	}
}
