// TabBar/TabView interaction and appearance window checks, without DevServer.
//
// Environment: interactive Linux/X11, Windows or macOS. Record OS, WM, actual
// backend and scale. Run from the repository root:
//
//	go run ./tests/window/widgets/tabs
//
// Initially request Integrated 800x480 DIP, minimum 400x320, Modern light.
// Document one/two/three start with one selected, empty page inputs and a reorderable
// HeaderBar tab bar. Cross-window transfer is off; see ../tab_detach for new windows.
// Steps and expected results:
//  1. Type different text on one/two and switch tabs: each retains its text,
//     caret and selection, including after Rebuild.
//  2. Drag over 4 DIP inside the bar: the whole tab tracks its original hotspot,
//     neighbors animate aside and reversal does not jump. Release commits one
//     order/moved update. A short click selects; release outside or Esc cancels
//     without reordering or creating a window.
//  3. Add repeatedly and narrow: equal tabs shrink from 240 toward 112 DIP, with
//     scrolling only below the minimum fit. Arrows/wheel reveal hidden tabs and
//     selection stays visible. Widen: arrows disappear when all fit; equal tab
//     widths grow up to 240 DIP.
//  4. Move into content: unselected text/icons do not look disabled. Selected
//     tabs always show close; others show it only on hover or close-button focus.
//     Hover leaves titles/neighbors fixed. Close removes that page without first
//     selecting it.
//  5. Integrated tab drags reorder, while space after tabs, 4 DIP gaps and
//     HeaderBar padding move the window. Close/scroll buttons remain clickable.
//  6. Right-click an unselected tab or its close button: menu targets it without
//     changing selection, reordering or closing. Close issues the same close
//     request as its button; switch-to selects that target. Esc/outside click
//     only closes the menu. Shift+F10 uses the focused tab; also test macOS
//     Control+click. Blank/gap areas have no tab menu and still act as caption.
//
// Optional appearance/regressions; choose relevant cases without exhaustive combinations:
//  1. Click tab body, then arrows/Home/End: selected content agrees. Toggle Dark
//     preserves readable text/icons, bounds and hits. Selection uses a neutral
//     fill/thin border; Integrated resize edges do not clip the top. Close/scroll
//     hover/press leaves titles fixed.
//  2. Text size switches 14/24 pt: text fits vertically, HeaderBar grows with
//     row height, close stays centered. Narrow text clips visually without
//     ellipsis or font shrinking.
//  3. Width mode switches 112..240/240..240 DIP. Fixed widths do not shrink and
//     still scroll. Adjacent unselected/unhovered tabs have centered 1x16 DIP
//     separators, hidden beside a hovered tab and throughout drag/return animation,
//     then restored in the new order.
//  4. Drag/release an unselected tab then leave the bar: it stays unselected and
//     close hides. Esc/outside release is identical. Body focus does not retain
//     close, but close-button focus does. No invisible clickable area may remain.
//  5. Restart -fallback-style: explicit GUI+widget defaults ignore Toggle Dark/Text
//     size but retain readable text, state fills, separators, icons and selection/reorder.
//
// Logical 2x: set GOUI_PLAT_SCALE=2 (PowerShell: $env:GOUI_PLAT_SCALE='2') and
// restore afterward; this does not verify mixed DPI. Activate macOS windows first.
// Linux Integrated needs composition; record Native fallback. Windows supports
// Direct2D/OpenGL/Software independently of decoration choice. Inspect manually
// or with Computer Use: Selected/rebuilds/order status and current/close/moved logs
// support per-step pass/fail records; normal exit alone is not an automatic PASS.
// Side effects: this program's windows/in-memory text, no application files,
// clipboard or settings. Reset: restart. Exit: close the window.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"slices"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/icon"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	widgetstyle "github.com/golang-gui/goui/widgets/style"
	wui "github.com/golang-gui/goui/widgets/ui"
)

func main() {
	fallbackStyle := flag.Bool("fallback-style", false, "use explicit GUI + widgets fallback rules instead of Modern")
	flag.Parse()
	bitmap := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 2; y < 14; y++ {
		for x := 2; x < 14; x++ {
			bitmap.SetRGBA(x, y, color.RGBA{A: 255})
		}
	}
	iconSource, err := icon.NewImage(bitmap)
	if err != nil {
		log.Fatal(err)
	}
	keys := []string{"one", "two", "three"}
	current := "one"
	dark, serial, rebuilds := false, 3, 0
	fontSize := float32(14)
	fixedWidth := false
	err = ui.Run("org.golang-gui.TabsProbe", func(app ui.App) ui.RootView {
		minWidth := float32(112)
		if fixedWidth {
			minWidth = 240
		}
		pages := make([]*wui.TabPageView, 0, len(keys))
		for _, key := range keys {
			pages = append(pages, wui.TabPage(key,
				ui.VBox(ui.Label("Page: "+key), ui.TextInput().ID("input-"+key)).Spacing(8).Padding(16),
			).Title("Document "+key).Icon(iconSource).Closable(true))
		}
		sheet := modern.Sheet(modern.Options{Dark: dark, FontSize: fontSize})
		if *fallbackStyle {
			sheet = style.Sheet(append(gui.DefaultStyleRules(), widgetstyle.Rules()...)...)
		}
		return ui.Root().StyleSheet(sheet).Windows(
			ui.Window("tabs").Title("GOUI tabs acceptance").Chrome(ui.WindowChromeIntegrated).Size(800, 480).MinSize(400, 320).Content(
				ui.VBox(
					ui.HeaderBar(wui.TabBar("documents").TabWidthRange(minWidth, 240).Reorderable(true).
						ContextMenu(func(key string) []*ui.MenuItemView {
							fmt.Printf("menu=%s current=%s\n", key, current)
							return []*ui.MenuItemView{
								ui.MenuItem("Document "+key, nil).Enabled(false),
								ui.MenuSeparator(),
								ui.MenuItem("切换到此页", func() { current = key; app.RequestUpdate() }).Enabled(current != key),
								ui.MenuItem("关闭", func() {
									// Route through the same GUI request as the close button.
									if page, ok := app.FindWidget("tabs", key).(*widgets.TabPage); ok {
										if view, ok := app.FindWidget("tabs", "documents").(*widgets.TabView); ok {
											view.RequestClose(page)
										}
									}
								}),
							}
						}).
						OnContextMenuError(func(err error) { fmt.Printf("menu-error=%v\n", err) })).Padding(6),
					ui.HBox(
						ui.Button("Add").OnClick(func() {
							serial++
							key := fmt.Sprintf("page-%d", serial)
							keys = append(keys, key)
							current = key
							app.RequestUpdate()
						}),
						ui.Button("Rebuild").OnClick(func() { rebuilds++; app.RequestUpdate() }),
						ui.Button("Toggle Dark").OnClick(func() { dark = !dark; app.RequestUpdate() }),
						ui.Button("Text size").OnClick(func() {
							if fontSize == 14 {
								fontSize = 24
							} else {
								fontSize = 14
							}
							app.RequestUpdate()
						}),
					).Spacing(8).Padding(8),
					ui.HBox(
						ui.Button("Width mode").OnClick(func() { fixedWidth = !fixedWidth; app.RequestUpdate() }),
						ui.Label(fmt.Sprintf("Tab widths: %.0f..240 DIP", minWidth)),
					).Spacing(8).Padding(8),
					ui.Label(fmt.Sprintf("Selected=%s | rebuilds=%d | order=%v", current, rebuilds, keys)),
					wui.TabView(pages...).ID("documents").Current(current).
						OnCurrent(func(key string) { current = key; fmt.Printf("current=%s\n", key); app.RequestUpdate() }).
						OnCloseRequest(func(key string) {
							if i := slices.Index(keys, key); i >= 0 {
								if current == key {
									if i+1 < len(keys) {
										current = keys[i+1]
									} else if i > 0 {
										current = keys[i-1]
									} else {
										current = ""
									}
								}
								keys = slices.Delete(keys, i, i+1)
							}
							fmt.Printf("close=%s current=%s\n", key, current)
							app.RequestUpdate()
						}).
						OnMoved(func(key string, _, to int) {
							from := slices.Index(keys, key)
							if from >= 0 && to >= 0 && to < len(keys) {
								keys = slices.Delete(keys, from, from+1)
								keys = slices.Insert(keys, to, key)
								fmt.Printf("moved=%s %d->%d order=%v\n", key, from, to, keys)
								app.RequestUpdate()
							}
						}),
				).CrossAlign(layout.CrossStretch),
			),
		)
	})
	if err != nil {
		log.Fatal(err)
	}
}
