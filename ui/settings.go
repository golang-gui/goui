package ui

import (
	"image/color"

	"github.com/golang-gui/goui/gui"
)

// ColorScheme re-exports the gui color scheme so ui consumers use ui types only,
// without importing the gui or platform packages.
type ColorScheme = gui.ColorScheme

const (
	ColorSchemeLight = gui.ColorSchemeLight
	ColorSchemeDark  = gui.ColorSchemeDark
)

// Settings is the thread-safe UI-layer view of system settings. Getters run on
// the UI thread automatically and always return usable values (the gui layer
// applies fallback), so callers need no nil checks or thread handling.
type Settings struct{}

// Settings returns the system settings. It is never nil.
func (app) Settings() Settings {
	return Settings{}
}

func (Settings) ColorScheme() ColorScheme {
	var v ColorScheme
	App.Sync(func() { v = guiSettings().ColorScheme() })
	return v
}

func (Settings) AccentColor() color.Color {
	var v color.Color
	App.Sync(func() { v = guiSettings().AccentColor() })
	return v
}

func (Settings) FontFamily() string {
	var v string
	App.Sync(func() { v = guiSettings().FontFamily() })
	return v
}

func (Settings) FontSize() float32 {
	var v float32
	App.Sync(func() { v = guiSettings().FontSize() })
	return v
}

// guiSettings returns the gui settings, or a default (all-fallback) instance
// when there is no active runtime, so getters are always usable.
func guiSettings() *gui.Settings {
	if rt := currentAppRuntime(); rt != nil && rt.gui != nil {
		if s := rt.gui.Settings(); s != nil {
			return s
		}
	}
	return &gui.Settings{}
}
