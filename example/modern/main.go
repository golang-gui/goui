// Modern theme showcase.
// Environment: a desktop session; Linux requires X11. Choose the backend and
// application scale with GOUI_PLAT_PAINTER and GOUI_PLAT_SCALE.
// Run: go run ./example/modern. Initially Modern follows the system mode and accent.
// Steps and expected results:
//  1. Switch Mode between Light and Dark: text, backgrounds and semantic colors
//     follow the mode. Changing Accent leaves Warning/Error/Success unchanged.
//  2. Hover and press Flat action and Menu: both use the ordinary button's neutral
//     background and 1 DIP border without shifting text. Flat action updates the input.
//  3. Disabled flat has no background and accepts neither clicks nor Tab focus.
//     Keyboard focus on Flat action shows a 2 DIP border.
//  4. Enable Bare GUI: clear custom StyleName values and retain text with GUI defaults.
//  5. Open and scroll a long menu, then resize the window. Check for clipping and
//     stale transparent pixels in menus and controls.
//  6. Accent buttons use a solid accent and explicit PrimaryContent: white text
//     on purple/green, black on yellow/white. Normal, hover and pressed states add
//     no ordinary border; black still gives interaction feedback. Disabled primary
//     uses a neutral background and subdued text. Its 2 DIP inset focus border is
//     #414147 in light mode and #D8D8DE in dark mode; fill and text stay unchanged.
//     Ordinary and Flat buttons retain their existing focus colors and geometry.
//
// See example/demo for Card/Sidebar surfaces; layout containers do not paint
// a background merely because a StyleName is set.
// Side effects: no system settings, clipboard or files are changed.
// Reset: restart. Exit: close the window.
package main

import (
	"fmt"
	"image/color"

	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func main() {
	mode, accentIndex := 0, 0
	text := "Hello, GOUI"
	modes := []string{"System", "Light", "Dark", "Bare GUI"}
	accents := []color.Color{nil, color.NRGBA{R: 140, G: 70, B: 220, A: 255}, color.NRGBA{R: 20, G: 160, B: 90, A: 255}, color.NRGBA{R: 255, G: 220, B: 70, A: 255}, color.White, color.Black}
	accentNames := []string{"System", "Purple", "Green", "Yellow", "White", "Black"}
	err := ui.Run("com.xugtek.goui", func(app ui.App) ui.RootView {
		settings := app.Settings()
		dark := mode == 2 || mode == 0 && settings.ColorScheme() == ui.ColorSchemeDark
		accent := accents[accentIndex]
		if accent == nil {
			accent = settings.AccentColor()
		}
		var sheet style.StyleSheet
		primary, primaryContent, muted, flat, warning, errorStyle, success := "", "", "", "", "", "", ""
		if mode != 3 {
			if mode == 0 && accentIndex == 0 {
				sheet = modern.FromSettings(settings)
			} else {
				sheet = modern.Sheet(modern.Options{
					Dark: dark, AccentColor: accent,
					FontFamily: settings.FontFamily(), FontSize: settings.FontSize(),
				})
			}
			primary, primaryContent, muted = modern.Primary, modern.PrimaryContent, modern.MutedText
			flat, warning, errorStyle, success = modern.Flat, modern.Warning, modern.Error, modern.Success
		}
		rows := make([]ui.View, 30)
		menuRows := make([]*ui.MenuItemView, 30)
		for i := range rows {
			rows[i] = ui.Label(fmt.Sprintf("Scrollable row %02d — rounded scrollbar", i+1))
			menuRows[i] = ui.MenuItem(fmt.Sprintf("Menu item %02d", i+1), nil)
		}
		edgeMenuRows := make([]*ui.MenuItemView, 120)
		for i := range edgeMenuRows {
			edgeMenuRows[i] = ui.MenuItem(fmt.Sprintf("Work-area constrained item %03d", i+1), nil)
		}
		return ui.Root().StyleSheet(sheet).Windows(
			ui.Window("modern").Title("GOUI Modern").Size(680, 560).
				Chrome(ui.WindowChromeIntegrated).Content(
				ui.VBox(
					ui.HeaderBar(ui.Label("GOUI Modern")),
					ui.VBox(
						ui.Label("Hover and press buttons to inspect their appearance.").Style(muted),
						ui.HBox(
							ui.Button("Mode: "+modes[mode]).OnClick(func() { mode = (mode + 1) % len(modes); app.RequestUpdate() }),
							ui.Button().Content(ui.Label("Accent: "+accentNames[accentIndex]).Style(primaryContent)).Style(primary).
								OnClick(func() { accentIndex = (accentIndex + 1) % len(accents); app.RequestUpdate() }),
							ui.Button().Content(ui.Label("Disabled primary").Style(primaryContent)).Style(primary).Enabled(false),
						).Spacing(12),
						ui.HWrap(
							ui.Button("Flat action").Style(flat).OnClick(func() { text = "Flat action clicked"; app.RequestUpdate() }),
							ui.Button("Disabled flat").Style(flat).Enabled(false),
							ui.Label("Warning").Style(warning), ui.Label("Error").Style(errorStyle), ui.Label("Success").Style(success),
						).Spacing(12).LineSpacing(8),
						ui.HBox(
							ui.MenuButton("Menu").Style(flat).Menu(
								ui.MenuItem("New document", func() { text = "New document"; app.RequestUpdate() }),
								ui.MenuItem("Unavailable command", nil).Enabled(false),
								ui.MenuSeparator(),
								ui.MenuItem("Clear text", func() { text = ""; app.RequestUpdate() }),
							),
							ui.MenuButton("Long menu").Menu(menuRows...),
						).Spacing(12),
						ui.TextInput().Text(text).OnText(func(value string) { text = value; app.RequestUpdate() }),
						ui.Label("System mode follows settings automatically; Bare GUI restores the fallback.").Style(muted),
						ui.ScrollView(ui.VBox(rows...).Spacing(10).Padding(8)),
					).Padding(16).Spacing(16).CrossAlign(layout.CrossStretch).MainWeight(1),
					ui.HBox(
						ui.Label("Move near screen edges, then open →").Style(muted),
						ui.MenuButton("Edge / long menu").Menu(edgeMenuRows...),
					).MainAlign(layout.MainEnd).Padding(8).Spacing(8),
				).CrossAlign(layout.CrossStretch),
			),
		)
	})
	if err != nil {
		panic(err)
	}
}
