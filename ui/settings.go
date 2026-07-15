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

// Settings is the thread-safe UI-layer view of system settings, obtained via
// App.Settings(). Getters run on the UI thread automatically and always return
// usable values (the gui layer applies fallback), so callers need no nil checks or
// thread handling.
type Settings struct {
	current  *app
	settings gui.Settings
}

// Settings returns the system settings view. It is never nil.
func (a *app) Settings() Settings {
	return Settings{
		current:  a,
		settings: a.gui.Settings(),
	}
}

func (s Settings) ColorScheme() (v ColorScheme) {
	if s.current == nil {
		return v
	}
	s.current.Sync(func() { v = s.settings.ColorScheme() })
	return
}

func (s Settings) AccentColor() (v color.Color) {
	if s.current == nil {
		return v
	}
	s.current.Sync(func() { v = s.settings.AccentColor() })
	return
}

func (s Settings) FontFamily() (v string) {
	if s.current == nil {
		return v
	}
	s.current.Sync(func() { v = s.settings.FontFamily() })
	return
}

func (s Settings) FontSize() (v float32) {
	if s.current == nil {
		return v
	}
	s.current.Sync(func() { v = s.settings.FontSize() })
	return
}
