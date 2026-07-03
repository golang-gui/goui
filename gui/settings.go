package gui

import (
	"image/color"
	"time"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform"
)

// ColorScheme re-exports the platform color scheme so gui consumers use gui
// types only, without importing the platform package.
type ColorScheme = platform.ColorScheme

const (
	ColorSchemeLight = platform.ColorSchemeLight
	ColorSchemeDark  = platform.ColorSchemeDark
)

// Fallback defaults used when the platform cannot report a setting. These are
// gui-owned on purpose: the gui layer hides platform differences behind usable
// values, so callers never deal with per-platform availability. Inventing a
// default is policy and thus not the platform layer's job.
var (
	defaultColorScheme             = ColorSchemeLight
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

	// Change detection state, accessed only on the UI thread (see watch).
	snapPrev  settingsSnapshot
	snapReady bool
}

func newSettings() *Settings {
	return &Settings{}
}

func (s *Settings) ColorScheme() ColorScheme {
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

// settingsSnapshot is a comparable digest of the resolved settings.
type settingsSnapshot struct {
	scheme         ColorScheme
	ar, ag, ab, aa uint32
	family         string
	size           float32
}

func (s *Settings) snapshot() settingsSnapshot {
	ar, ag, ab, aa := s.AccentColor().RGBA()
	return settingsSnapshot{
		scheme: s.ColorScheme(),
		ar:     ar,
		ag:     ag,
		ab:     ab,
		aa:     aa,
		family: s.FontFamily(),
		size:   s.FontSize(),
	}
}

// watch drives system-setting change detection.
// posts checkChanged onto the event loop so every read happens on the UI thread.
func (s *Settings) watch(loop platform.EventLoop) {
	if s.platform == nil {
		return // nothing to observe; getters always fall back
	}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			loop.Post(s.checkChanged)
		}
	}()
}

// checkChanged runs on the UI thread: snapshot, compare, emit on change.
func (s *Settings) checkChanged() {
	next := s.snapshot()
	changed := s.snapReady && next != s.snapPrev
	s.snapPrev = next
	s.snapReady = true
	if changed {
		s.changed.Emit()
	}
}
