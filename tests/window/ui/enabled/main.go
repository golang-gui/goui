// Disabled widgets, inherited state, focus, input and declarative restoration
// window checks, without DevServer.
// Environment: interactive Linux X11, Windows or macOS. Record OS/WM, backend
// and scale. Run: go run ./tests/window/ui/enabled
// Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows) and
// GOUI_PLAT_SCALE=2 for 2x. Initially Integrated 860x700 DIP, light theme, group
// enabled, ordinary/primary buttons and input enabled, permanently disabled
// button inactive, initial editor text retained and ReadOnly selectable/copyable.
// Steps and expected results:
//  1. Use buttons, checks, switch, drop-down and editor: counts/text update.
//  2. F1 or Toggle group disables the group without changing layout/content.
//     Text/icons dim; interaction, Tab focus and editor context menus stop.
//     Progress animation continues; the external toolbar remains usable.
//  3. F2/F3 change the ordinary button/input's own settings even while their
//     group is disabled. F1 restores each according to its own setting; the
//     permanently disabled button stays disabled. F2/F3 restore both.
//  4. Focus/type in the input, including Chinese preedit, then F1: focus/caret
//     clear, uncommitted preedit cancels and committed text remains. Re-enabling
//     does not take focus automatically.
//  5. Hold the ordinary button, press F1 twice and release: no click count.
//     A fresh click works. Disable/re-enable it with F2 under the pointer:
//     hover recovers; moving away before toggling cannot create false hover.
//     F7 schedules group disabling in two seconds; open the drop-down first.
//     Focus loss may close it; if it stays open, the next pointer/key operation
//     closes it without committing selection. Modal popup keys cannot invoke F1
//     for this scenario.
//  6. Tab/Shift+Tab skip disabled fields. ReadOnly can focus/copy but not edit.
//  7. F4/Dark changes mode with readable neutral disabled colors and stable
//     dimensions. F5 removes only the ordinary button's Enabled declaration,
//     restoring its constructor default while retaining the group's restriction.
//  8. F6/Check asserts real widget state, subtree/content, actions and Focusable,
//     printing ASSERT PASS and window-local DIP bounds. Repeat after F1/F2/F3/F5
//     and rebuilds; window-level F1-F6 still work while the group is disabled.
//
// Side effects: clipboard copying only on request; no file writes. Reset: restart.
// Exit: Escape, Quit or native close prints EXIT PASS. Native close also invokes
// OnDestroy and prints DESTROY PASS; App.Quit need not emit that signal.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"time"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

type squareIcon struct{}

func (squareIcon) Image(width, height int, foreground gui.Color) image.Image {
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	c := color.RGBA{R: uint8(foreground.R * 255), G: uint8(foreground.G * 255), B: uint8(foreground.B * 255), A: uint8(foreground.A * 255)}
	draw.Draw(result, result.Bounds().Inset(max(1, width/6)), image.NewUniform(c), image.Point{}, draw.Src)
	return result
}

func main() {
	groupEnabled, buttonEnabled, inputEnabled := true, true, true
	buttonDeclared, dark, connected := true, false, false
	clicks, changes, submits := 0, 0, 0
	text := "Editable text / 中文"
	checked, switched, selected := false, true, 0
	model := gui.NewSliceListModel([]widgets.DropDownItem{{Text: "First"}, {Text: "Second"}, {Text: "Unavailable", Disabled: true}})
	var previous []gui.Widget
	var verify func()
	err := ui.Run("org.golang-gui.EnabledTest", func(app ui.App) ui.RootView {
		update := app.RequestUpdate
		toggleGroup := func() { groupEnabled = !groupEnabled; update() }
		toggleButton := func() { buttonDeclared = true; buttonEnabled = !buttonEnabled; update() }
		toggleInput := func() { inputEnabled = !inputEnabled; update() }
		toggleDark := func() { dark = !dark; update() }
		removeButton := func() { buttonDeclared = false; update() }
		disableLater := func() {
			app.TimeoutFunc(2*time.Second, func() { groupEnabled = false; update() })
		}
		wantGroup, wantButton, wantInput, wantDeclared := groupEnabled, buttonEnabled, inputEnabled, buttonDeclared
		verify = func() {
			win := app.FindWindow("enabled")
			if win == nil {
				panic("missing window")
			}
			ids := []string{"group", "button", "primary", "input", "check", "switch", "choice", "always-disabled", "readonly", "icon"}
			for i, id := range ids {
				w := gui.FindWidget(win, id)
				if w == nil || len(previous) != 0 && previous[i] != w {
					panic("identity: " + id)
				}
				own := true
				switch id {
				case "group":
					own = wantGroup
				case "button":
					own = !wantDeclared || wantButton
				case "input":
					own = wantInput
				case "always-disabled":
					own = false
				}
				effective := own && (id == "readonly" || wantGroup)
				if id == "icon" {
					effective = effective && (!wantDeclared || wantButton)
				}
				info := w.Snapshot()
				if w.Enabled() != own || gui.IsEnabled(w) != effective || info.Enabled != effective ||
					!effective && (info.Focused || len(info.Actions) != 0) {
					panic("enable state/actions: " + id)
				}
				if id != "group" && id != "icon" && !w.Focusable() {
					panic("lost focus configuration: " + id)
				}
				fmt.Printf("BOUNDS %s %.1f %.1f %.1f %.1f own=%v effective=%v focus=%v\n", id,
					info.Bounds.X, info.Bounds.Y, info.Bounds.Width, info.Bounds.Height, own, effective, info.Focused)
			}
			input := gui.FindWidget(win, "input").(*gui.TextInput)
			// Editing changes the Widget before a queued declarative rebuild.
			// Compare with synchronous OnText writeback, not an old declaration.
			if input.Text() != text {
				panic(fmt.Sprintf("content: got %q, want %q", input.Text(), text))
			}
			previous = previous[:0]
			for _, id := range ids {
				previous = append(previous, gui.FindWidget(win, id))
			}
			fmt.Printf("ASSERT PASS clicks=%d changes=%d submits=%d text=%q\n", clicks, changes, submits, text)
		}
		// Fast input can change pending state before a previous check runs. Compare
		// against the latest completed build, not pending declarations and an older tree.
		check := func() { app.Post(func() { verify() }) }
		if !connected {
			connected = true
			check()
		}
		button := ui.Button().ID("button").Child(ui.HBox(ui.Icon(squareIcon{}).ID("icon").Style(modern.AccentIcon), ui.Label("Normal button")).Spacing(6)).
			OnClick(func() { clicks++; fmt.Printf("CLICK %d\n", clicks); update() })
		if buttonDeclared {
			button.Enabled(buttonEnabled)
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{Dark: dark})).Windows(
			ui.Window("enabled").Title("GOUI Enabled").Size(860, 700).Chrome(ui.WindowChromeIntegrated).
				OnDestroy(func() { fmt.Println("DESTROY PASS"); app.Quit() }).
				Shortcuts(
					ui.Shortcut(ui.KeyF1).OnActivate(toggleGroup), ui.Shortcut(ui.KeyF2).OnActivate(toggleButton),
					ui.Shortcut(ui.KeyF3).OnActivate(toggleInput), ui.Shortcut(ui.KeyF4).OnActivate(toggleDark),
					ui.Shortcut(ui.KeyF5).OnActivate(removeButton), ui.Shortcut(ui.KeyF6).OnActivate(check),
					ui.Shortcut(ui.KeyF7).OnActivate(disableLater),
					ui.Shortcut(ui.KeyEscape).OnActivate(app.Quit),
				).Content(ui.VBox(
				ui.HeaderBar(ui.Label("Widget Enabled · subtree / focus / style")),
				ui.VBox(
					ui.HBox(ui.Button("Toggle group").OnClick(toggleGroup), ui.Button("Toggle button").OnClick(toggleButton),
						ui.Button("Toggle input").OnClick(toggleInput), ui.Button("Dark").OnClick(toggleDark),
						ui.Button("Check").OnClick(check), ui.Button("Quit").OnClick(app.Quit)).Spacing(8),
					ui.Label(fmt.Sprintf("group=%v | button=%v declared=%v | input=%v | clicks=%d changes=%d submits=%d", groupEnabled, buttonEnabled, buttonDeclared, inputEnabled, clicks, changes, submits)),
					ui.VBox(
						ui.HBox(button, ui.Button().Content(ui.Label("Primary").Style(modern.PrimaryContent)).ID("primary").Style(modern.Primary).OnClick(func() { clicks++; update() }),
							ui.Button("Always disabled").ID("always-disabled").Enabled(false).OnClick(func() { panic("disabled click") })).Spacing(12),
						ui.TextInput().ID("input").Text(text).Enabled(inputEnabled).MinWidth(420).
							OnText(func(value string) { text = value; changes++; update() }).OnSubmit(func() { submits++; update() }),
						wui.CheckButton("CheckButton").ID("check").Checked(checked).OnChange(func(s wui.CheckState) { checked = s == wui.CheckChecked; changes++; update() }),
						wui.Switch("Switch").ID("switch").Checked(switched).OnChange(func(value bool) { switched = value; changes++; update() }),
						wui.DropDown(model).ID("choice").Selected(selected).MinWidth(260).OnSelected(func(index int) { selected = index; changes++; update() }).OnOpenError(func(err error) { panic(err) }),
						wui.ProgressBar(0).Indeterminate(true).MinWidth(300),
					).ID("group").Enabled(groupEnabled).Spacing(18).CrossAlign(layout.CrossStart),
					ui.Label("ReadOnly stays interactive outside the disabled group:"),
					ui.TextInput().ID("readonly").Text("Select and copy; editing is read-only").ReadOnly(true).MinWidth(420),
					ui.Label("F1 group · F2 button · F3 input · F4 dark · F5 reset button · F6 check · F7 disable in 2s · Esc quit"),
				).Padding(20).Spacing(18).CrossAlign(layout.CrossStart),
			).CrossAlign(layout.CrossStretch)),
		)
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("EXIT PASS")
}
