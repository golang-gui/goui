package gui

import (
	"image/color"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform"
)

// Fallback defaults used when the platform cannot report a setting. These are
// gui-owned on purpose: the gui layer hides platform differences behind usable
// values, so callers never deal with per-platform availability. Inventing a
// default is policy and thus not the platform layer's job.
var (
	defaultColorScheme             = platform.ColorSchemeLight
	defaultAccentColor color.Color = color.RGBA{R: 70, G: 130, B: 220, A: 255}
	defaultFontFamily              = "" // empty = renderer uses the platform default font
	defaultFontSize    float32     = 14
)

// Settings exposes system settings (appearance, and later locale/timezone/etc.)
// as directly usable values, substituting gui defaults when the platform cannot
// report a value.
type Settings struct {
	platform platform.Settings // may be nil; getters then always fall back
	changed  signal.Signal0
}

func newSettings() *Settings {
	return &Settings{}
}

func (s *Settings) ColorScheme() platform.ColorScheme {
	if s.platform != nil {
		if v, err := s.platform.ColorScheme(); err == nil {
			return v
		}
	}
	return defaultColorScheme
}

func (s *Settings) AccentColor() color.Color {
	if s.platform != nil {
		if v, err := s.platform.AccentColor(); err == nil && v != nil {
			return v
		}
	}
	return defaultAccentColor
}

func (s *Settings) FontFamily() string {
	if s.platform != nil {
		if v, err := s.platform.FontFamily(); err == nil && v != "" {
			return v
		}
	}
	return defaultFontFamily
}

func (s *Settings) FontSize() float32 {
	if s.platform != nil {
		if v, err := s.platform.FontSize(); err == nil && v > 0 {
			return v
		}
	}
	return defaultFontSize
}

// ConnectChanged registers a listener fired when a system setting changes. The
// listener runs on the UI thread.
func (s *Settings) ConnectChanged(fn func()) signal.Handle {
	return s.changed.Connect(fn)
}

// emitChanged is invoked on the UI thread after the platform reports a change.
func (s *Settings) emitChanged() {
	s.changed.Emit()
}
