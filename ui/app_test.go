package ui

import (
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
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

func TestNewWindowRegisteredForAfterUpdateBeforeShow(t *testing.T) {
	app := newWindowTestApplication()
	probe := &afterUpdateProbe{child: Label("ready").ID("child")}
	probe.Self = probe
	rt := newApp(app, func() RootView { return Window("prepared").Content(probe) })
	called := 0
	probe.callback = func(widget gui.Widget) {
		called++
		mount := rt.windows["prepared"]
		if mount == nil || mount.window.Widget() != widget || mount.root.widget() != widget || app.windows[0].shows != 0 {
			t.Fatal("hidden window not registered/mounted before preparation")
		}
		if gui.FindWidget(mount.window, "child") == nil {
			t.Fatal("preparation cannot find the completed child tree")
		}
	}
	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}
	if called != 1 || app.windows[0].shows != 1 {
		t.Fatal("preparation/show order or counts changed")
	}
}

func TestAfterUpdateCanDiscardHiddenWindowWithoutShowingIt(t *testing.T) {
	app := newWindowTestApplication()
	probe := new(afterUpdateProbe)
	probe.Self = probe
	rt := newApp(app, func() RootView { return Window("discarded").Content(probe) })
	probe.callback = func(gui.Widget) { rt.windows["discarded"].window.Destroy() }
	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}
	if !app.windows[0].destroyed || app.windows[0].shows != 0 || rt.windows["discarded"] != nil {
		t.Fatal("discarded hidden window was shown or registered again")
	}
}

func TestAfterUpdateCannotNestWindowBatch(t *testing.T) {
	application := newWindowTestApplication()
	probe := new(afterUpdateProbe)
	probe.Self = probe
	rt := newApp(application, func() RootView { return Window("prepared").Content(probe) })
	calls := 0
	probe.callback = func(gui.Widget) {
		calls++
		if err := rt.reconcileWindows(nil); err == nil {
			t.Fatal("completion callback entered a nested window batch")
		}
		if rt.windows["prepared"] == nil || application.windows[0].shows != 0 {
			t.Fatal("nested completion removed or showed the pending window")
		}
	}
	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || application.windows[0].shows != 1 || rt.updatingWindows || rt.reconcilingWindows {
		t.Fatal("batch guard prevented normal completion or leaked")
	}
	// The guard applies only to synchronous nesting, not the next update.
	probe.callback = nil
	if err := rt.reconcileWindows(nil); err != nil {
		t.Fatal(err)
	}
}

func TestShowFailurePreservesCompletedWindowUntilCleanup(t *testing.T) {
	application := newWindowTestApplication()
	probe := new(afterUpdateProbe)
	probe.Self = probe
	failure := errors.New("show failed")
	rt := newApp(application, func() RootView { return Window("prepared").Content(probe) })
	probe.callback = func(gui.Widget) { application.windows[0].showErr = failure }
	if err := rt.rebuild(); !errors.Is(err, failure) {
		t.Fatalf("show error lost: %v", err)
	}
	window := application.windows[0]
	if window.destroyed || rt.windows["prepared"] == nil || window.Widget() == nil {
		t.Fatal("show failure destroyed content completed by AfterUpdate")
	}
	rt.destroyAll()
	if !window.destroyed {
		t.Fatal("normal application cleanup leaked the failed window")
	}
}

type transferCompletionView struct {
	ViewBase[transferCompletionView]
	children []View
	complete func(*gui.LinearBox)
}

func (v *transferCompletionView) Build() View { return v }
func (v *transferCompletionView) Mount(BuildContext) gui.Widget {
	return gui.NewLinearBox(layout.DirectionVertical)
}
func (v *transferCompletionView) Update(ctx BuildContext, widget gui.Widget) {
	box := widget.(*gui.LinearBox)
	ctx.UpdateChildren(box, v.children)
	ctx.AfterUpdate(func() { v.complete(box) })
}
func (v *transferCompletionView) Unmount(BuildContext, gui.Widget) {}

func TestWindowCompletionTransferDoesNotReapplyStaleSourceDeclaration(t *testing.T) {
	for _, targetFirst := range []bool{false, true} {
		t.Run(fmt.Sprint("target-first=", targetFirst), func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			application := newWindowTestApplication()
			tracker := new(lifecycleTracker)
			var rt *app
			var original gui.Widget
			pending, moved := false, false
			makeView := func(target bool) *transferCompletionView {
				v := new(transferCompletionView)
				v.Self = v
				if target == moved {
					v.children = []View{transferView(tracker, "retained")}
				}
				v.complete = func(box *gui.LinearBox) {
					if !target || !pending || moved {
						return
					}
					source := rt.windows["source"].window.Widget().(*gui.LinearBox)
					if err := (&Coordinator{owner: rt.windows["source"].root.root}).TransferChild(original, box, func() error {
						source.RemoveChild(original)
						box.AddChild(original)
						return nil
					}, func() { moved = true; rt.RequestUpdate() }); err != nil {
						t.Fatal(err)
					}
				}
				return v
			}
			rt = newApp(application, func() RootView {
				source := Window("source").Content(makeView(false))
				if !pending {
					return source
				}
				target := Window("target").Content(makeView(true))
				if targetFirst {
					return Root().Windows(target, source)
				}
				return Root().Windows(source, target)
			})
			if err := rt.rebuild(); err != nil {
				t.Fatal(err)
			}
			original = rt.windows["source"].window.Widget().Children()[0]
			pending = true
			if err := rt.rebuild(); err != nil {
				t.Fatal(err)
			}
			application.runPosted()
			if !moved || tracker.mounts != 1 || tracker.unmounts != 0 || len(rt.windows["source"].window.Widget().Children()) != 0 || rt.windows["target"].window.Widget().Children()[0] != original {
				t.Fatalf("completion recreated or lost transferred node: moved=%v lifecycle=%+v", moved, tracker)
			}
			rt.destroyAll()
		})
	}
}

func TestAppFindWidgetByScopedID(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	guiApp := newWindowTestApplication()
	rt := newApp(guiApp, func() RootView {
		return Window("main").Content(VBox().Children(
			Label("one").ID("target").Name("peer"),
			Label("two").ID("other").Name("peer"),
		))
	})
	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}
	got := rt.FindWidget("main", "target")
	if got == nil || got.ID() != "target" || got.Name() != "peer" {
		t.Fatalf("scoped lookup: widget=%v", got)
	}
	if missing := rt.FindWidget("other", "target"); missing != nil {
		t.Fatalf("unknown window lookup: widget=%v", missing)
	}
	if missing := rt.FindWidget("main", ""); missing != nil {
		t.Fatalf("empty ID lookup: widget=%v", missing)
	}
	if missing := rt.FindWidget("main", "absent"); missing != nil {
		t.Fatalf("unknown widget lookup: widget=%v", missing)
	}
	if err := rt.reconcileWindows(nil); err != nil {
		t.Fatal(err)
	}
	if missing := rt.FindWidget("main", "target"); missing != nil {
		t.Fatalf("removed window lookup: widget=%v", missing)
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

func TestAppWindowMinSize(t *testing.T) {
	app := newWindowTestApplication()
	mode := 0 // 0: width only, 1: both axes, 2: no declaration
	rt := newApp(app, func() RootView {
		wv := Window("main").Content(Label("main"))
		switch mode {
		case 0:
			wv = wv.MinWidth(320)
		case 1:
			wv = wv.MinSize(320, 240)
		}
		return wv
	})

	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}
	win := app.windows[0]

	// Width-only declaration forwards exactly once.
	want := geometry.Size{Width: 320}
	if len(win.minSizes) != 1 || win.minSizes[0] != want {
		t.Fatalf("MinWidth not forwarded once: %+v", win.minSizes)
	}

	// An identical rebuild must not re-issue the native call.
	rt.RequestUpdate()
	app.runPosted()
	if len(win.minSizes) != 1 {
		t.Fatalf("unchanged rebuild re-forwarded MinWidth: %+v", win.minSizes)
	}

	// Switching to a full MinSize updates both axes.
	mode = 1
	rt.RequestUpdate()
	app.runPosted()
	want = geometry.Size{Width: 320, Height: 240}
	if len(win.minSizes) != 2 || win.minSizes[1] != want {
		t.Fatalf("MinSize update not forwarded: %+v", win.minSizes)
	}

	// Removing all declarations clears back to the unbounded default.
	mode = 2
	rt.RequestUpdate()
	app.runPosted()
	want = geometry.Size{}
	if len(win.minSizes) != 3 || win.minSizes[2] != want {
		t.Fatalf("removed declaration did not clear the hint: %+v", win.minSizes)
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

func TestBeginRunIsOneShot(t *testing.T) {
	// activeRun sets a package-global latch that never clears; reset it so this test
	// does not consume Run for the rest of the package.
	t.Cleanup(func() {
		activeApp.Lock()
		activeApp.ran = false
		activeApp.Unlock()
	})

	if err := activeRun(); err != nil {
		t.Fatalf("first activeRun should succeed, got %v", err)
	}
	if err := activeRun(); !errors.Is(err, ErrAppRunOnce) {
		t.Fatalf("second activeRun should return ErrAppRunOnce, got %v", err)
	}
}

func TestAppHandleRunsThroughRuntime(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
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
	options []gui.WindowOptions
	posts   []func()
	windows []*testWindow
	sheets  []style.StyleSheet
	quits   int
}

func newWindowTestApplication() *windowTestApplication {
	return new(windowTestApplication)
}

func (a *windowTestApplication) NewTimer() *gui.Timer { panic("unexpected NewTimer") }

func (a *windowTestApplication) OpenURL(string) error { panic("unexpected OpenURL") }

func (a *windowTestApplication) OpenPath(string) error { panic("unexpected OpenPath") }

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

func (a *windowTestApplication) FileDialog() gui.FileDialog {
	return &windowTestFileDialog{}
}

func (a *windowTestApplication) NewWindow(request *gui.WindowOptions) (gui.Window, error) {
	var options gui.WindowOptions
	if request != nil {
		options = *request
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	a.options = append(a.options, options)
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

type windowTestFileDialog struct{}

func (f *windowTestFileDialog) OpenFile(owner gui.Window, opts gui.DialogOptions, cb func([]string)) {
	cb(nil)
}

func (f *windowTestFileDialog) OpenDirectory(owner gui.Window, opts gui.DialogOptions, cb func([]string)) {
	cb(nil)
}

func (f *windowTestFileDialog) SaveFile(owner gui.Window, opts gui.DialogOptions, cb func([]string)) {
	cb(nil)
}

func (a *windowTestApplication) runPosted() {
	posts := a.posts
	a.posts = nil
	for _, post := range posts {
		post()
	}
}
