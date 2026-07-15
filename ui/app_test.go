package ui

import (
	"errors"
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

func TestAppMountsAndUpdatesWindow(t *testing.T) {
	app := newWindowTestApplication()
	text := "first"
	builds := 0
	rt := newApp(app, func() RootView {
		builds++
		return Window("main").
			Title("Main").
			Content(Label(text))
	})

	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}

	if len(app.windows) != 1 {
		t.Fatalf("expected one created window, got %d", len(app.windows))
	}
	win := app.windows[0]
	if win.id != "main" || win.title != "Main" || win.shows != 1 {
		t.Fatalf("unexpected window state: id=%q title=%q shows=%d", win.id, win.title, win.shows)
	}
	label := win.widget.(*gui.Label)
	if label.Text() != "first" || builds != 1 {
		t.Fatalf("unexpected initial content: text=%q builds=%d", label.Text(), builds)
	}

	text = "second"
	rt.RequestUpdate()
	rt.RequestUpdate()

	if len(app.posts) != 1 {
		t.Fatalf("expected one coalesced post, got %d", len(app.posts))
	}
	if label.Text() != "first" {
		t.Fatalf("update should be deferred, got %q", label.Text())
	}

	app.runPosted()
	if label.Text() != "second" || builds != 2 {
		t.Fatalf("unexpected updated content: text=%q builds=%d", label.Text(), builds)
	}
	if len(app.windows) != 1 || app.windows[0] != win {
		t.Fatal("same window id should reuse the existing gui.Window")
	}
}

func TestAppReplacesWindowWhenIDChanges(t *testing.T) {
	app := newWindowTestApplication()
	id := "first"
	rt := newApp(app, func() RootView {
		return Window(id).Content(Label(id))
	})

	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}
	first := app.windows[0]

	id = "second"
	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}

	if len(app.windows) != 2 {
		t.Fatalf("expected two created windows, got %d", len(app.windows))
	}
	if !first.destroyed {
		t.Fatal("old window was not destroyed after id changed")
	}
	second := app.windows[1]
	if second.id != "second" || second.destroyed {
		t.Fatalf("unexpected replacement window: id=%q destroyed=%v", second.id, second.destroyed)
	}
	if _, exists := rt.windows["first"]; exists {
		t.Fatal("old window mount was not removed")
	}
	if rt.windows["second"].window != second {
		t.Fatal("new window mount was not recorded")
	}
}

func TestAppCloseRequestCanPreventDestroy(t *testing.T) {
	app := newWindowTestApplication()
	preventClose := true
	destroys := 0
	rt := newApp(app, func() RootView {
		return Window("main").
			Content(Label("main")).
			OnCloseRequest(func(allow *bool) {
				if preventClose {
					*allow = false
				}
			}).
			OnDestroy(func() {
				destroys++
			})
	})

	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}
	win := app.windows[0]

	if err := win.RequestClose(); err != nil {
		t.Fatal(err)
	}
	if win.destroyed {
		t.Fatal("close request was not prevented")
	}
	if destroys != 0 || app.quits != 0 {
		t.Fatalf("unexpected destroy side effects: destroys=%d quits=%d", destroys, app.quits)
	}

	preventClose = false
	rt.RequestUpdate()
	app.runPosted()

	if err := win.RequestClose(); err != nil {
		t.Fatal(err)
	}
	if !win.destroyed {
		t.Fatal("window was not destroyed after close request was allowed")
	}
	// The ui runtime destroys the window but does not decide to quit — that is
	// gui.Application's QuitOnLastWindowClosed policy, so app.quits stays 0.
	if destroys != 1 || app.quits != 0 {
		t.Fatalf("unexpected destroy handling: destroys=%d quits=%d", destroys, app.quits)
	}
	if len(rt.windows) != 0 {
		t.Fatal("destroyed window mount was not removed")
	}
}

func TestAppRejectsInvalidWindowIDs(t *testing.T) {
	app := newWindowTestApplication()
	rt := newApp(app, func() RootView {
		return Window("").Content(Label("bad"))
	})

	if err := rt.rebuild(); !errors.Is(err, ErrWindowIDEmpty) {
		t.Fatalf("expected ErrWindowIDEmpty, got %v", err)
	}
}

func TestAppAppliesRootStyleSheetOnChange(t *testing.T) {
	app := newWindowTestApplication()
	sheetA := style.Sheet(style.Name("button").Radius(4))
	sheetB := style.Sheet(style.Name("button").Radius(8))
	current := sheetA
	rt := newApp(app, func() RootView {
		return Root().
			StyleSheet(current).
			Windows(Window("main").Content(Label("x")))
	})

	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}
	if len(app.sheets) != 1 {
		t.Fatalf("expected the root sheet applied once, got %d", len(app.sheets))
	}
	if r, ok := app.sheets[0].Resolve(style.Sel{Name: "button"}).Radius(); !ok || r != 4 {
		t.Fatalf("applied the wrong sheet: radius=%v ok=%v", r, ok)
	}

	// An identical sheet on the next rebuild must not re-apply: that would trigger a
	// redundant app-wide relayout every rebuild.
	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}
	if len(app.sheets) != 1 {
		t.Fatalf("identical sheet should not re-apply, got %d applications", len(app.sheets))
	}

	// Swapping to a different sheet applies the new one (runtime re-skin).
	current = sheetB
	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}
	if len(app.sheets) != 2 {
		t.Fatalf("swapped sheet should re-apply, got %d applications", len(app.sheets))
	}
	if r, ok := app.sheets[1].Resolve(style.Sel{Name: "button"}).Radius(); !ok || r != 8 {
		t.Fatalf("applied the wrong swapped sheet: radius=%v ok=%v", r, ok)
	}
}

func TestAppHandleRunsThroughRuntime(t *testing.T) {
	app := newWindowTestApplication()
	builds := 0
	// The App handle passed into the build closure is the runtime itself.
	var handle App = newApp(app, func() RootView {
		builds++
		return Window("main").Content(Label("main"))
	})

	posted := false
	handle.Post(func() {
		posted = true
	})
	if posted {
		t.Fatal("Post should not run synchronously")
	}
	app.runPosted()
	if !posted {
		t.Fatal("Post did not run through the runtime")
	}

	synced := false
	handle.Sync(func() {
		synced = true
	})
	if !synced {
		t.Fatal("Sync should run inline on the UI goroutine")
	}

	handle.RequestUpdate()
	handle.RequestUpdate()
	if len(app.posts) != 1 {
		t.Fatalf("expected one coalesced update post, got %d", len(app.posts))
	}
	app.runPosted()
	if builds != 1 {
		t.Fatalf("expected one rebuild through RequestUpdate, got %d", builds)
	}
}

type windowTestApplication struct {
	posts   []func()
	windows []*testWindow
	sheets  []style.StyleSheet
	quits   int
}

func newWindowTestApplication() *windowTestApplication {
	return new(windowTestApplication)
}

func (a *windowTestApplication) Platform() platform.Platform {
	return nil
}

func (a *windowTestApplication) Typography() typography.Context {
	return nil
}

func (a *windowTestApplication) StyleSheet() style.StyleSheet {
	return nil
}

func (a *windowTestApplication) SetStyleSheet(sheet style.StyleSheet) {
	a.sheets = append(a.sheets, sheet)
}

func (a *windowTestApplication) Clipboard() gui.Clipboard {
	return nil
}

func (a *windowTestApplication) Settings() gui.Settings {
	return nil
}

func (a *windowTestApplication) NewWindow() (gui.Window, error) {
	win := newTestWindow()
	a.windows = append(a.windows, win)
	return win, nil
}

func (a *windowTestApplication) Run() {}

func (a *windowTestApplication) Quit() {
	a.quits++
}

func (a *windowTestApplication) QuitOnLastWindowClosed() bool { return true }

func (a *windowTestApplication) SetQuitOnLastWindowClosed(bool) {}

func (a *windowTestApplication) Post(task func()) {
	a.posts = append(a.posts, task)
}

func (a *windowTestApplication) Windows() []gui.Window {
	windows := make([]gui.Window, 0, len(a.windows))
	for _, win := range a.windows {
		if !win.destroyed {
			windows = append(windows, win)
		}
	}
	return windows
}

func (a *windowTestApplication) Snapshot() gui.ApplicationInfo {
	return gui.ApplicationInfo{}
}

func (a *windowTestApplication) DispatchWindowEvent(string, events.Event) error {
	return nil
}

func (a *windowTestApplication) runPosted() {
	posts := a.posts
	a.posts = nil
	for _, post := range posts {
		post()
	}
}
