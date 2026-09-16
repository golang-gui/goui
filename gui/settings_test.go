package gui

import (
	"errors"
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/golang-gui/goui/platform"
)

type settingsTestSource struct {
	scheme ColorScheme
	accent color.Color
	family string
	size   float32
	err    error
	reads  int
}

func (s *settingsTestSource) ColorScheme() (ColorScheme, error) {
	s.reads++
	return s.scheme, s.err
}
func (s *settingsTestSource) AccentColor() (color.Color, error) {
	s.reads++
	return s.accent, s.err
}
func (s *settingsTestSource) FontFamily() (string, error) {
	s.reads++
	return s.family, s.err
}
func (s *settingsTestSource) FontSize() (float32, error) {
	s.reads++
	return s.size, s.err
}

func TestSettingsInitialSnapshotAndFirstChange(t *testing.T) {
	accent := &color.NRGBA{R: 20, G: 40, B: 60, A: 128}
	source := &settingsTestSource{scheme: ColorSchemeDark, accent: accent, family: "First", size: 16}
	s := newSettings(source)
	initial := s.current
	if source.reads != 4 || s.ColorScheme() != ColorSchemeDark || s.FontFamily() != "First" || s.FontSize() != 16 {
		t.Fatalf("initial settings missing: %+v, reads=%d", initial, source.reads)
	}
	if initial.accent != color.RGBA64Model.Convert(accent) {
		t.Fatal("accent precision or premultiplication changed")
	}
	changes := 0
	h := s.ConnectChanged(func() {
		changes++
		if s.ColorScheme() != ColorSchemeLight || s.FontFamily() != "Second" || s.FontSize() != 18 ||
			s.AccentColor() != color.RGBA64Model.Convert(accent) {
			t.Fatal("listener saw a partial/stale snapshot")
		}
	})
	defer h.Disconnect()
	if changes != 0 {
		t.Fatal("connecting emitted an initial change")
	}
	source.scheme, source.family, source.size = ColorSchemeLight, "Second", 18
	accent.R = 200
	for range 10 {
		if s.ColorScheme() != initial.scheme || s.AccentColor() != initial.accent ||
			s.FontFamily() != initial.family || s.FontSize() != initial.size {
			t.Fatal("getter queried native state or retained mutable platform color")
		}
	}
	if source.reads != 4 {
		t.Fatal("getters performed native queries")
	}
	s.checkChanged()
	if changes != 1 || source.reads != 8 {
		t.Fatalf("first poll lost the change: changes=%d reads=%d", changes, source.reads)
	}
	// Equal colors with a different concrete Go type do not cause a notification.
	source.accent = color.RGBA64Model.Convert(accent)
	s.checkChanged()
	if changes != 1 {
		t.Fatal("unchanged resolved values emitted another change")
	}
}

func TestSettingsFallbackSnapshot(t *testing.T) {
	want := newSettings(nil).current
	for _, size := range []float32{0, -1, float32(math.NaN()), float32(math.Inf(1))} {
		source := &settingsTestSource{scheme: ColorScheme(99), size: size}
		s := newSettings(source)
		if s.current != want {
			t.Fatalf("invalid values did not fall back: %+v", s.current)
		}
		changes := 0
		s.ConnectChanged(func() { changes++ })
		s.checkChanged()
		if changes != 0 {
			t.Fatal("invalid native values caused repeated changes")
		}
	}
	source := &settingsTestSource{scheme: ColorSchemeDark, accent: color.Black, family: "System", size: 20}
	s := newSettings(source)
	changes := 0
	s.ConnectChanged(func() { changes++ })
	source.err = errors.New("native setting unavailable")
	s.checkChanged()
	if s.current != want || changes != 1 {
		t.Fatal("native errors did not publish resolved fallback values")
	}
	s.checkChanged()
	if changes != 1 {
		t.Fatal("persistent native errors emitted repeatedly")
	}
	source.err = nil
	s.checkChanged()
	if s.FontFamily() != "System" || changes != 2 {
		t.Fatal("native recovery was not reported")
	}
}

type settingsTestLoop struct {
	platform.EventLoop
	posts chan func()
	run   func()
}

func (l *settingsTestLoop) Post(fn func()) { l.posts <- fn }
func (l *settingsTestLoop) Run()           { l.run() }

func TestSettingsWatchBelongsToApplicationRun(t *testing.T) {
	source := &settingsTestSource{family: "Initial", size: 14}
	s := newSettings(source)
	loop := &settingsTestLoop{posts: make(chan func(), 4)}
	var pending func()
	loop.run = func() {
		select {
		case pending = <-loop.posts:
		case <-time.After(5 * time.Second):
			t.Fatal("Run did not start settings polling")
		}
		if source.reads != 4 {
			t.Fatal("timer queried native settings outside the event loop")
		}
		// Return with this task still queued, as a native loop can do on quit.
	}
	a := &application{settings: s, loop: loop, timers: newTimerScheduler(loop.Post, time.Now)}
	a.Run()
	source.family = "Changed after exit"
	pending()
	if source.reads != 4 || s.FontFamily() != "Initial" {
		t.Fatal("stale poll queried native resources after Run returned")
	}
	// No-platform fallback needs neither a timer nor a usable event loop.
	stop := newSettings(nil).watch(nil)
	stop()
}
