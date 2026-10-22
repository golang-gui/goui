// Shortcut and keyboard-layout window checks.
//
// Environment: Windows/macOS/Linux desktop. Record OS, actual layout, IME,
// backend and scale. Run: go run ./tests/window/ui/shortcuts
// Primary is Command on macOS, Control elsewhere. Initially counts=0, shortcuts
// enabled and input empty.
// Steps and expected results:
//  1. Focus a noneditor button. Primary+S increments Save once, including when
//     held. F6 permits system repeat and increments repeat while held.
//     Primary+Shift+S does not increment Save.
//  2. Type abc in the input. Primary+A selects all without window-A; with button
//     focus it increments window-A. Input F5 increments local-F5, button F5
//     window-F5. Hold each of three F5 presses for 100-200 ms before release:
//     exactly three increments. Same-instant down/up simulation is insufficient.
//     Holding F5 one second adds only one; release/repress adds another. Held
//     input F6 still repeats. On Linux check both Chinese/English IME modes.
//  3. Rebuild repeatedly, then Primary+S: one increment. Disable stops it;
//     Enable restores it.
//  4. Use recorded French AZERTY and German QWERTZ layouts. With button focus,
//     logical Primary+Q/Y increment the corresponding counts independently of
//     US physical positions. Repeat with CapsLock on/off; explicit Shift is an
//     extra modifier and does not match these declarations.
//  5. Input the layout's AltGr/Option characters, dead keys and Chinese IME text:
//     normal commits, no extra commands. Primary+S inserts neither s nor controls.
//  6. Close without leftover windows. Startup alone does not verify these steps.
//
// Side effects: in-program state only. Restore any manually changed keyboard
// layout/environment afterward. Reset: restart. System-reserved combinations
// may be intercepted and are not required to reach the application.
package main

import (
	"fmt"
	"log"

	"github.com/golang-gui/goui/ui"
)

func main() {
	save, repeat, windowA, localF5, windowF5, q, y := 0, 0, 0, 0, 0, 0, 0
	enabled := true
	if err := ui.Run("org.golang-gui.Shortcuts", func(app ui.App) ui.RootView {
		binding := func(key ui.Key, mods ui.KeyModifiers, counter *int) *ui.ShortcutView {
			return ui.Shortcut(key).Modifiers(mods).Enabled(enabled).OnActivate(func() {
				*counter++
				log.Printf("key=%d save=%d local-F5=%d window-F5=%d repeat=%d Q=%d Y=%d", key, save, localF5, windowF5, repeat, q, y)
				app.RequestUpdate()
			})
		}
		return ui.Window("shortcuts").Title("GOUI shortcuts / keyboard layouts").Size(720, 420).
			Shortcuts(binding(ui.KeyS, ui.ModPrimary, &save), binding(ui.KeyA, ui.ModPrimary, &windowA),
				binding(ui.KeyF5, 0, &windowF5), binding(ui.KeyF6, 0, &repeat).AutoRepeat(true),
				binding(ui.KeyQ, ui.ModPrimary, &q), binding(ui.KeyY, ui.ModPrimary, &y)).
			Content(ui.VBox(
				ui.Label(fmt.Sprintf("save=%d repeat=%d window-A=%d\nlocal-F5=%d window-F5=%d Q=%d Y=%d", save, repeat, windowA, localF5, windowF5, q, y)),
				ui.TextInput().Shortcuts(binding(ui.KeyF5, 0, &localF5)),
				ui.Button("Rebuild / focus button").OnClick(func() { app.RequestUpdate() }),
				ui.Button(fmt.Sprintf("Enabled: %t (toggle)", enabled)).OnClick(func() { enabled = !enabled; app.RequestUpdate() }),
			).Spacing(12))
	}); err != nil {
		log.Fatal(err)
	}
}
