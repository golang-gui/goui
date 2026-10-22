// Style-driven vector/custom Icon window checks, with Software bitmap
// generation/caching and no DevServer.
//
// Environment: desktop and rendering libraries. Record OS, Linux desktop, backend
// and scale. Run: go run ./tests/window/ui/icon
// Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows),
// GOUI_PLAT_SCALE=1 or 2. Call ui.Run from main on the platform startup thread.
// Initially light mode, blue accent, 24 DIP: search, Path check, two-color marker
// and icon button.
// Steps and expected results:
//  1. Light/Dark changes ordinary search foreground; Accent check remains visible.
//     The marker's orange stays fixed while its other color follows foreground,
//     without stale colors.
//  2. Accent cycles blue/purple/green. Accent icons and Primary fill follow it;
//     ordinary icons stay unchanged.
//  3. Size cycles 24/40/16 DIP. Circles stay circular, Paths keep proportions;
//     repeat at 2x without low-resolution enlargement blur on diagonals/curves.
//  4. Replace toggles the last icon plus/minus, updating the same source closure.
//  5. Search-button clicks increment once. Hover/press changes the button
//     background; the icon uses explicit PrimaryContent rather than inheriting
//     button style. Standalone icons have neither click behavior nor focus borders.
//  6. Resize without overflowing icon bounds and repeat after theme changes.
//
// Platform differences: native fonts/decorations vary. All backends retain icon
// geometry/color with minor edge rasterization differences. See
// tests/window/graphics/alpha for translucent-brush checks.
// Side effects: no settings, clipboard or files changed. Reset: restart.
// Exit: close the window.
package main

import (
	"fmt"
	"image/color"

	"github.com/golang-gui/goui/core/geometry"
	drawicon "github.com/golang-gui/goui/icon/draw"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func search(p graphics.Painter, size graphics.Size, c graphics.Color) {
	s := size.Width
	p.DrawEllipse(geometry.Point{X: s * .42, Y: s * .42}, s*.25, s*.25, s*.08, c)
	p.DrawLine(geometry.Point{X: s * .60, Y: s * .60}, geometry.Point{X: s * .88, Y: s * .88}, s*.08, c)
}

var checkPath = graphics.MoveTo(3, 12).LineTo(9, 18).LineTo(21, 5)

func check(p graphics.Painter, size graphics.Size, c graphics.Color) {
	p.SetTransform(geometry.Scale(size.Width/24, size.Height/24))
	p.DrawPath(checkPath, 2, c)
}

func mark(p graphics.Painter, size graphics.Size, c graphics.Color) {
	p.FillEllipse(geometry.Point{X: size.Width / 2, Y: size.Height / 2}, size.Width*.45, size.Height*.45,
		ui.Color{R: 1, G: .5, A: 1})
	p.FillRect(geometry.Rect(size.Width*.4, size.Height*.2, size.Width*.2, size.Height*.6), c)
}

func symbol(plus bool) *drawicon.Source {
	return &drawicon.Source{Draw: func(p graphics.Painter, size graphics.Size, c graphics.Color) {
		p.FillRect(geometry.Rect(size.Width*.2, size.Height*.45, size.Width*.6, size.Height*.1), c)
		if plus {
			p.FillRect(geometry.Rect(size.Width*.45, size.Height*.2, size.Width*.1, size.Height*.6), c)
		}
	}}
}

func main() {
	dark, plus := false, true
	searchSource, checkSource, markSource := &drawicon.Source{Draw: search}, &drawicon.Source{Draw: check}, &drawicon.Source{Draw: mark}
	plusSource, minusSource := symbol(true), symbol(false)
	accentIndex, sizeIndex, clicks := 0, 0, 0
	accents := []color.Color{color.RGBA{R: 40, G: 120, B: 220, A: 255}, color.RGBA{R: 160, G: 60, B: 200, A: 255}, color.RGBA{R: 20, G: 150, B: 80, A: 255}}
	sizes := []float32{24, 40, 16}
	if err := ui.Run("org.golang-gui.IconTest", func(app ui.App) ui.RootView {
		size := sizes[sizeIndex]
		symbolSource := minusSource
		if plus {
			symbolSource = plusSource
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{Dark: dark, AccentColor: accents[accentIndex]})).Windows(
			ui.Window("icons").Title("GOUI Icon validation").Size(720, 320).Chrome(ui.WindowChromeIntegrated).Content(
				ui.VBox(
					ui.HeaderBar(ui.Label("Drawing icons")),
					ui.VBox(
						ui.Label(fmt.Sprintf("Dark: %v | Accent: %d | Size: %.0f DIP | Clicks: %d", dark, accentIndex+1, size, clicks)),
						ui.HBox(
							ui.Button("Light/Dark").OnClick(func() { dark = !dark; app.RequestUpdate() }),
							ui.Button("Accent").OnClick(func() { accentIndex = (accentIndex + 1) % len(accents); app.RequestUpdate() }),
							ui.Button("Size").OnClick(func() { sizeIndex = (sizeIndex + 1) % len(sizes); app.RequestUpdate() }),
							ui.Button("Replace").OnClick(func() { plus = !plus; app.RequestUpdate() }),
						).Spacing(12),
						ui.HBox(
							ui.Icon(searchSource).Size(size),
							ui.Icon(checkSource).Size(size).Style(modern.AccentIcon),
							ui.Icon(markSource).Size(size),
							ui.Button().Content(ui.Icon(searchSource).Size(size).Style(modern.PrimaryContent)).Style(modern.Primary).OnClick(func() { clicks++; app.RequestUpdate() }),
							ui.Icon(symbolSource).Size(size),
						).Spacing(24).CrossAlign(layout.CrossCenter),
						ui.Label("Search | Accent path | Two-color | Icon button | Closure"),
					).Padding(16).Spacing(20),
				).CrossAlign(layout.CrossStretch),
			),
		)
	}); err != nil {
		panic(err)
	}
}
