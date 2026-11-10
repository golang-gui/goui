package ui

import (
	"errors"
	"fmt"
	"time"

	"github.com/xuges/gothread"
	"image/color"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"
)

type testSettings struct {
	composited             bool
	changed                signal.Signal0
	scheme                 gui.ColorScheme
	family                 string
	language               string
	connections, listeners int
}

func (s *testSettings) Composited() bool             { return s.composited }
func (s *testSettings) ColorScheme() gui.ColorScheme { return s.scheme }
func (s *testSettings) AccentColor() color.Color     { return color.Black }
func (s *testSettings) FontFamily() string           { return s.family }
func (s *testSettings) FontSize() float32            { return 14 }
func (s *testSettings) Language() string             { return s.language }
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
func (a *settingsTestApplication) Run() error {
	a.activate.Emit()
	if a.quits == 0 {
		a.run()
	}
	a.shutdown.Emit()
	return nil
}

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
			Windows(Window("main").Content(Label(fmt.Sprintf("%s%s/%t", rt.Settings().FontFamily(), rt.Settings().Language(), rt.Settings().Composited()))))
	})
	a.run = func() {
		if builds != 1 || a.settings.connections != 1 || a.settings.listeners != 1 {
			t.Fatal("missing initial build or unique subscription")
		}
		label := a.windows[0].widget.(*gui.Label)
		a.settings.scheme, a.settings.family = ColorSchemeDark, "Changed"
		a.settings.language = "ja"
		a.settings.changed.Emit()
		a.settings.changed.Emit()
		rt.RequestUpdate() // explicit requests share the same scheduler
		if builds != 1 || len(a.posts) != 1 {
			t.Fatal("settings updates were not deferred/coalesced")
		}
		a.runPosted()
		if builds != 2 || label.Text() != "Changedja/false" || len(a.sheets) != 2 || len(a.windows) != 1 {
			t.Fatal("settings did not reconcile existing content and changed sheet")
		}
		a.settings.family = "Unrelated to fixed font style"
		a.settings.changed.Emit()
		a.runPosted()
		if builds != 3 || len(a.sheets) != 2 || a.settings.connections != 1 {
			t.Fatal("equal rules reapplied the sheet or rebuild reconnected settings")
		}
		// 仅合成状态改变也复用同一订阅与更新调度。
		a.settings.composited = true
		a.settings.changed.Emit()
		a.runPosted()
		if builds != 4 || label.Text() != "Unrelated to fixed font styleja/true" || len(a.sheets) != 2 || a.settings.connections != 1 {
			t.Fatal("合成能力变化未更新视图或重复应用样式/连接")
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
	if builds != 4 || len(a.posts) != 0 || rt.updatePending {
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

// 跨线程 getter 仅同步读取 GUI 快照，不直接请求构建；停止后不再读取。
type compositedThreadSettings struct {
	*testSettings
	calls, thread int
}

func (s *compositedThreadSettings) Composited() bool {
	s.calls++
	if gothread.GetId() != s.thread {
		panic("settings read outside the UI thread")
	}
	return s.composited
}

type compositedTestApplication struct {
	*windowTestApplication
	tasks    chan func()
	settings compositedThreadSettings
}

func (a *compositedTestApplication) Post(fn func())         { a.tasks <- fn }
func (a *compositedTestApplication) Settings() gui.Settings { return &a.settings }

func TestSettingsCompositedUsesUIThread(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	g := &compositedTestApplication{windowTestApplication: newWindowTestApplication(), tasks: make(chan func(), 1)}
	g.settings = compositedThreadSettings{testSettings: &testSettings{composited: true}, thread: gothread.GetId()}
	rt := newApp(g, func() RootView { t.Fatal("读取不应重建视图"); return nil })
	settings := rt.Settings()
	if !settings.Composited() || g.settings.calls != 1 {
		t.Fatal("UI 线程内读取失败")
	}
	g.settings.composited = false
	done := make(chan bool, 1)
	go func() { done <- settings.Composited() }()
	select {
	case fn := <-g.tasks:
		fn()
	case <-time.After(time.Second):
		t.Fatal("后台读取没有投递")
	}
	select {
	case got := <-done:
		if got || g.settings.calls != 2 {
			t.Fatal("未读取最新 GUI 快照")
		}
	case <-time.After(time.Second):
		t.Fatal("后台读取没有返回")
	}
	select {
	case <-g.tasks:
		t.Fatal("getter 额外安排了更新")
	default:
	}
	rt.stop()
	if settings.Composited() || g.settings.calls != 2 || (Settings{}).Composited() {
		t.Fatal("停止后或零值仍读取设置")
	}
}
