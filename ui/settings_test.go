package ui

import (
	"errors"
	"image/color"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"
)

type testSettings struct {
	changed                signal.Signal0
	scheme                 gui.ColorScheme
	family                 string
	connections, listeners int
}

func (s *testSettings) ColorScheme() gui.ColorScheme { return s.scheme }
func (s *testSettings) AccentColor() color.Color     { return color.Black }
func (s *testSettings) FontFamily() string           { return s.family }
func (s *testSettings) FontSize() float32            { return 14 }
func (s *testSettings) ConnectChanged(fn func()) signal.Handle {
	s.connections++
	s.listeners++
	return &testSettingsHandle{Handle: s.changed.Connect(fn), settings: s}
}

type testSettingsHandle struct {
	signal.Handle
	settings *testSettings
}

func (h *testSettingsHandle) Disconnect() {
	h.Handle.Disconnect()
	if h.settings != nil {
		h.settings.listeners--
		h.settings = nil
	}
}

type settingsTestApplication struct {
	*windowTestApplication
	settings testSettings
	run      func()
}

func (a *settingsTestApplication) Settings() gui.Settings { return &a.settings }
func (a *settingsTestApplication) Run()                   { a.run() }

func TestSettingsAutomaticallyRebuildAndDeduplicateSheet(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	a := &settingsTestApplication{windowTestApplication: newWindowTestApplication()}
	builds := 0
	var rt *app
	rt = newApp(a, func() RootView {
		builds++
		background := color.White
		if rt.Settings().ColorScheme() == ColorSchemeDark {
			background = color.Black
		}
		// A new sheet is built each time; unrelated settings must not invalidate
		// all GUI style caches when the resolved rules remain equal.
		return Root().StyleSheet(style.Sheet(style.Name("window").BackgroundColor(background))).
			Windows(Window("main").Content(Label(rt.Settings().FontFamily())))
	})
	a.run = func() {
		if builds != 1 || a.settings.connections != 1 || a.settings.listeners != 1 {
			t.Fatal("missing initial build or unique subscription")
		}
		label := a.windows[0].widget.(*gui.Label)
		a.settings.scheme, a.settings.family = ColorSchemeDark, "Changed"
		a.settings.changed.Emit()
		a.settings.changed.Emit()
		rt.RequestUpdate() // explicit requests share the same scheduler
		if builds != 1 || len(a.posts) != 1 {
			t.Fatal("settings updates were not deferred/coalesced")
		}
		a.runPosted()
		if builds != 2 || label.Text() != "Changed" || len(a.sheets) != 2 || len(a.windows) != 1 {
			t.Fatal("settings did not reconcile existing content and changed sheet")
		}
		a.settings.family = "Unrelated to fixed font style"
		a.settings.changed.Emit()
		a.runPosted()
		if builds != 3 || len(a.sheets) != 2 || a.settings.connections != 1 {
			t.Fatal("equal rules reapplied the sheet or rebuild reconnected settings")
		}
		// Simulate native exit without ui.App.Quit, leaving an update queued.
		a.settings.changed.Emit()
	}
	if err := rt.run(); err != nil {
		t.Fatal(err)
	}
	if a.settings.listeners != 0 || !rt.isStopping() {
		t.Fatal("native exit retained listener or running state")
	}
	a.runPosted()
	a.settings.changed.Emit()
	rt.RequestUpdate()
	rt.Sync(func() { t.Fatal("Sync ran after exit") })
	if builds != 3 || len(a.posts) != 0 || rt.updatePending {
		t.Fatal("late settings/update revived the stopped UI")
	}
	rt.destroyAll()
}

func TestSettingsListenerCleanupOnFailureAndQuit(t *testing.T) {
	for _, mode := range []string{"initial failure", "update failure", "quit"} {
		t.Run(mode, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			a := &settingsTestApplication{windowTestApplication: newWindowTestApplication()}
			builds := 0
			var rt *app
			rt = newApp(a, func() RootView {
				builds++
				if a.settings.listeners != 1 {
					t.Fatal("listener was not established before building")
				}
				if mode == "initial failure" || mode == "update failure" && builds > 1 {
					a.settings.changed.Emit()
					return Window("")
				}
				return Window("main")
			})
			a.run = func() {
				if mode == "initial failure" {
					t.Fatal("initial failure entered the native loop")
				}
				a.settings.changed.Emit()
				if mode == "quit" {
					rt.Quit()
				}
				a.runPosted()
			}
			err := rt.run()
			if mode != "quit" && !errors.Is(err, ErrWindowIDEmpty) || mode == "quit" && err != nil {
				t.Fatalf("unexpected run error: %v", err)
			}
			if a.settings.listeners != 0 || a.settings.connections != 1 {
				t.Fatal("listener was leaked or duplicated")
			}
			previous := builds
			a.runPosted()
			a.settings.changed.Emit()
			if builds != previous || len(a.posts) != 0 {
				t.Fatal("queued work rebuilt after failure/quit")
			}
			rt.destroyAll()
		})
	}
}
