package ui

import (
	"testing"

	"github.com/golang-gui/goui/gui"
)

func TestStateSetRequestsUpdateWithoutGet(t *testing.T) {
	app := newWindowTestApplication()
	rt := newApp(app, func() RootView {
		return Window("main").Content(Label("main"))
	})
	if err := setActiveApp(rt); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		clearActiveApp(rt)
	})

	state := MakeState("unused")
	state.Set("changed")
	state.Set("changed again")

	if len(app.posts) != 1 {
		t.Fatalf("expected one coalesced update post, got %d", len(app.posts))
	}
}

func TestZeroStateSetRequestsUpdate(t *testing.T) {
	app := newWindowTestApplication()
	rt := newApp(app, func() RootView {
		return Window("main").Content(Label("main"))
	})
	if err := setActiveApp(rt); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		clearActiveApp(rt)
	})

	var state State[int]
	if got := state.Get(); got != 0 {
		t.Fatalf("zero State value = %d, want 0", got)
	}

	state.Set(1)

	if got := state.Get(); got != 1 {
		t.Fatalf("updated zero State value = %d, want 1", got)
	}
	if len(app.posts) != 1 {
		t.Fatalf("expected one update post, got %d", len(app.posts))
	}
}

func TestStateSetRebuildsMountedWindow(t *testing.T) {
	app := newWindowTestApplication()
	title := MakeState("first")
	builds := 0
	rt := newApp(app, func() RootView {
		builds++
		return Window("main").Content(Label(title.Get()))
	})
	if err := setActiveApp(rt); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		clearActiveApp(rt)
	})

	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}
	label := app.windows[0].Widget().(*gui.Label)
	if label.Text() != "first" || builds != 1 {
		t.Fatalf("unexpected initial state: text=%q builds=%d", label.Text(), builds)
	}

	title.Set("second")
	if label.Text() != "first" {
		t.Fatalf("state set should defer rebuild, got %q", label.Text())
	}

	app.runPosted()
	if label.Text() != "second" || builds != 2 {
		t.Fatalf("unexpected rebuilt state: text=%q builds=%d", label.Text(), builds)
	}
}

func TestStateSetWithoutActiveAppUpdatesValue(t *testing.T) {
	state := MakeState(1)

	state.Set(2)

	if got := state.Get(); got != 2 {
		t.Fatalf("state value = %d, want 2", got)
	}
}
