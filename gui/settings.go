package gui

import (
	"image/color"
	"math"
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

// Settings exposes a GUI-thread snapshot with per-field fallback values.
// The initial snapshot is available before Run; later changes are polled while
// the application event loop runs. Getters never query native APIs themselves.
type Settings interface {
	ColorScheme() ColorScheme
	AccentColor() color.Color
	FontFamily() string
	FontSize() float32
	// ConnectChanged runs on the GUI thread after the entire snapshot is updated.
	// Connecting does not emit the initial snapshot; read it through the getters.
	ConnectChanged(fn func()) signal.Handle
}

type settings struct {
	settings platform.Settings // may be nil; the snapshot then contains fallbacks
	changed  signal.Signal0
	current  settingsSnapshot // accessed only on the GUI thread
}

func newSettings(platSettings platform.Settings) (s *settings) {
	s = &settings{settings: platSettings}
	s.current = s.snapshot()
	return
}

func (s *settings) ColorScheme() ColorScheme {
	return s.current.scheme
}

func (s *settings) AccentColor() color.Color {
	return s.current.accent
}

func (s *settings) FontFamily() string {
	return s.current.family
}

func (s *settings) FontSize() float32 {
	return s.current.size
}

// ConnectChanged registers a listener fired when a system setting changes. The
// listener runs on the UI thread.
func (s *settings) ConnectChanged(fn func()) signal.Handle {
	return s.changed.Connect(fn)
}

// settingsSnapshot owns comparable values, including a copy of the accent color.
type settingsSnapshot struct {
	scheme ColorScheme
	accent color.RGBA64
	family string
	size   float32
}

func (s *settings) snapshot() settingsSnapshot {
	next := settingsSnapshot{
		scheme: defaultColorScheme,
		accent: color.RGBA64Model.Convert(defaultAccentColor).(color.RGBA64),
		family: defaultFontFamily,
		size:   defaultFontSize,
	}
	if s.settings == nil {
		return next
	}
	if v, err := s.settings.ColorScheme(); err == nil && (v == ColorSchemeLight || v == ColorSchemeDark) {
		next.scheme = v
	}
	if v, err := s.settings.AccentColor(); err == nil && v != nil {
		next.accent = color.RGBA64Model.Convert(v).(color.RGBA64)
	}
	if v, err := s.settings.FontFamily(); err == nil && v != "" {
		next.family = v
	}
	if v, err := s.settings.FontSize(); err == nil && v > 0 && !math.IsInf(float64(v), 0) {
		next.size = v
	}
	return next
}

// watch belongs to one application Run. The timer only posts work; native
// queries, snapshot updates and notifications all run on the GUI thread.
// Stopping also invalidates checks already queued when Run returns.
func (s *settings) watch(app Application) (stop func()) {
	if s == nil || s.settings == nil {
		return func() {}
	}
	timer := app.NewTimer()
	timer.ConnectTimeout(s.checkChanged)
	// A scheduler already closed by Quit intentionally does not start polling.
	_ = timer.Start(2 * time.Second)
	return timer.Stop
}

// checkChanged runs on the UI thread: snapshot, compare, emit on change.
func (s *settings) checkChanged() {
	next := s.snapshot()
	if next != s.current {
		s.current = next
		s.changed.Emit()
	}
}
