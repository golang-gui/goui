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
	accents := []color.Color{nil, color.NRGBA{R: 140, G: 70, B: 220, A: 255}, color.NRGBA{R: 20, G: 160, B: 90, A: 255}, color.White, color.Black}
	accentNames := []string{"System", "Purple", "Green", "White", "Black"}
	err := ui.Run(func(app ui.App) ui.RootView {
		settings := app.Settings()
		dark := mode == 2 || mode == 0 && settings.ColorScheme() == ui.ColorSchemeDark
		accent := accents[accentIndex]
		if accent == nil {
			accent = settings.AccentColor()
		}
		var sheet style.StyleSheet
		primary, muted := "", ""
		if mode != 3 {
			sheet = modern.Sheet(modern.Options{
				Dark: dark, AccentColor: accent,
				FontFamily: settings.FontFamily(), FontSize: settings.FontSize(),
			})
			primary, muted = modern.Primary, modern.MutedText
		}
		rows := make([]ui.View, 30)
		for i := range rows {
			rows[i] = ui.Label(fmt.Sprintf("Scrollable row %02d — rounded scrollbar", i+1))
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
							ui.Button("Accent: "+accentNames[accentIndex]).Style(primary).
								OnClick(func() { accentIndex = (accentIndex + 1) % len(accents); app.RequestUpdate() }),
						).Spacing(12),
						ui.TextInput().Text(text).OnText(func(value string) { text = value; app.RequestUpdate() }),
						ui.Label("System mode follows settings automatically; Bare GUI restores the fallback.").Style(muted),
						ui.ScrollView(ui.VBox(rows...).Spacing(10).Padding(8)),
					).Padding(16).Spacing(16).CrossAlign(layout.CrossStretch).MainWeight(1),
				).CrossAlign(layout.CrossStretch),
			),
		)
	})
	if err != nil {
		panic(err)
	}
}
