package ui

import (
	"errors"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/mlang"
)

func TestLanguageAutomaticallyReconcilesExistingWidgets(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	previous := mlang.Language()
	t.Cleanup(func() { _ = mlang.SetLanguage(previous) })
	_ = mlang.SetLanguage("en")
	text := mlang.String(mlang.En("Save"), mlang.ZhCN("保存"), mlang.Text("ja", "保存する"))
	a := &settingsTestApplication{windowTestApplication: newWindowTestApplication()}
	builds := 0
	rt := newApp(a, func() RootView {
		builds++
		return Window("main").Content(VBox(Label(text()).ID("text"), TextInput().ID("input")))
	})
	a.run = func() {
		root := a.windows[0].widget
		label := gui.FindWidget(a.windows[0], "text").(*gui.Label)
		input := gui.FindWidget(a.windows[0], "input").(*gui.TextInput)
		input.SetText("user-owned text")
		input.SetSelection(gui.TextSelection{Anchor: 1, Caret: 5})
		a.windows[0].SetFocusedWidget(input)
		if err := mlang.SetLanguage("zh-CN"); err != nil {
			t.Fatal(err)
		}
		_ = mlang.SetLanguage("ja")
		rt.RequestUpdate()
		if builds != 1 || len(a.posts) != 1 {
			t.Fatal("language changes did not share the coalesced update queue")
		}
		a.runPosted()
		if builds != 2 || label.Text() != "保存する" || len(a.windows) != 1 || a.windows[0].widget != root ||
			gui.FindWidget(a.windows[0], "text") != label || gui.FindWidget(a.windows[0], "input") != input ||
			input.Text() != "user-owned text" || input.Selection() != (gui.TextSelection{Anchor: 1, Caret: 5}) ||
			a.windows[0].FocusedWidget() != input {
			t.Fatal("language change replaced widgets, lost edited state, or failed to update text")
		}
		_ = mlang.SetLanguage("ja")
		if len(a.posts) != 0 {
			t.Fatal("same language requested a rebuild")
		}
	}
	if err := rt.run(); err != nil {
		t.Fatal(err)
	}
	_ = mlang.SetLanguage("de")
	if len(a.posts) != 0 || rt.updatePending {
		t.Fatal("language notification revived the stopped application")
	}
	rt.destroyAll()
}

func TestLanguageUpdateStopsAfterBuildFailureOrQuit(t *testing.T) {
	for _, mode := range []string{"initial failure", "update failure", "quit"} {
		t.Run(mode, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			previous := mlang.Language()
			t.Cleanup(func() { _ = mlang.SetLanguage(previous) })
			_ = mlang.SetLanguage("en")
			a := &settingsTestApplication{windowTestApplication: newWindowTestApplication()}
			builds := 0
			rt := newApp(a, func() RootView {
				builds++
				if mode == "initial failure" || mode == "update failure" && builds > 1 {
					return Window("")
				}
				return Window("main")
			})
			a.run = func() {
				if mode == "initial failure" {
					t.Fatal("failed initial build entered the native event loop")
				}
				_ = mlang.SetLanguage("ja")
				if mode == "quit" {
					rt.Quit()
				}
				a.runPosted()
			}
			err := rt.run()
			if mode != "quit" && !errors.Is(err, ErrWindowIDEmpty) || mode == "quit" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			last := builds
			_ = mlang.SetLanguage("fr")
			a.runPosted()
			if builds != last || len(a.posts) != 0 || rt.updatePending {
				t.Fatal("language change rebuilt after failure/quit")
			}
			rt.destroyAll()
		})
	}
}
