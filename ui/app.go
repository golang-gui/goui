package ui

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"

	"github.com/xuges/gothread"
)

// App is the running application handle passed into the Run build closure. It is
// the running *app itself: valid for the whole run and safe to stash,
// capture in handlers, or call from any goroutine (after Quit its methods are safe
// no-ops).
type App interface {
	// Post runs f on the UI thread asynchronously.
	Post(func())
	// Sync runs f on the UI thread and waits, running inline when already on the
	// UI thread.
	Sync(func())
	// Quit asks the running app to stop.
	Quit()
	// RequestUpdate coalesces a declarative rebuild on the UI thread.
	RequestUpdate()
	// Clipboard returns the system clipboard view (never nil).
	Clipboard() Clipboard
	// Settings returns the system settings view (never nil).
	Settings() Settings
}

var (
	ErrAppRunOnce      = errors.New("ui: Run may be called only once per process")
	ErrAppBuildNil     = errors.New("ui app build function is nil")
	ErrWindowIDEmpty   = errors.New("ui window id is empty")
	ErrWindowDuplicate = errors.New("ui window id is duplicated")
)

// Run creates the application, mounts the declarative tree returned by build, and
// runs the event loop until the app quits. build receives the App handle, which it
// may stash, capture in handlers, or call from other goroutines.
func Run(build func(app App) RootView) error {
	if build == nil {
		return ErrAppBuildNil
	}
	if err := activeRun(); err != nil {
		return err
	}

	runtime.LockOSThread()

	guiApp, err := gui.NewApplication()
	if err != nil {
		return err
	}

	var a *app
	a = newApp(guiApp, func() RootView { return build(a) })
	setActiveApp(a)
	defer clearActiveApp()

	if err := a.rebuild(); err != nil {
		a.destroyAll()
		return err
	}

	guiApp.Run()
	return a.error()
}

// Post runs f on the UI thread asynchronously; it is dropped after the app stops.
func (a *app) Post(f func()) {
	if f != nil {
		a.postTask(f)
	}
}

// Sync runs f on the UI thread and waits. It runs f inline when already on the UI
// thread; after the app stops it is a no-op.
func (a *app) Sync(f func()) {
	if f == nil {
		return
	}
	if a.onUI() {
		f()
		return
	}

	var wg sync.WaitGroup
	if !a.postTask(func() {
		defer wg.Done()
		f()
	}) {
		return
	}
	wg.Wait()
}

type app struct {
	mu            sync.Mutex
	gui           gui.Application
	build         func() RootView
	windows       map[string]*windowMount
	appliedSheet  style.StyleSheet
	updatePending bool
	stopping      bool
	err           error
	uiThread      int
}

type windowMount struct {
	runtime       *app
	id            string
	window        gui.Window
	root          *root
	view          WindowView
	closeHandle   signal.Handle
	destroyHandle signal.Handle
	destroying    bool
}

var activeApp struct {
	sync.Mutex
	current *app
	ran     bool // one-shot: set on the first Run, never cleared (the loop is consumed)
}

// activeRun claims the process-wide, one-shot Run. ui.Run may run only once: the
// platform event loop is consumed when Run returns and cannot be restarted, and on
// Wayland/Android/web the app cannot own a restartable loop at all (see DesignApp).
// A second call — concurrent or after the first returned — fails here. To show a
// different UI, change state and let the tree rebuild within the single Run.
func activeRun() error {
	activeApp.Lock()
	defer activeApp.Unlock()

	if activeApp.ran {
		return ErrAppRunOnce
	}
	activeApp.ran = true
	return nil
}

// setActiveApp records the running runtime so State.Set can reach it — the one
// internal caller with no App handle in scope. It does not gate re-runs (activeRun
// does), so tests may install/clear a runtime without tripping the one-shot latch.
func setActiveApp(a *app) {
	activeApp.Lock()
	defer activeApp.Unlock()
	activeApp.current = a
}

// clearActiveApp drops the active runtime after Run returns; the ran latch
// stays set so no later Run can start.
func clearActiveApp() {
	activeApp.Lock()
	defer activeApp.Unlock()
	activeApp.current = nil
}

func currentApp() *app {
	activeApp.Lock()
	defer activeApp.Unlock()
	return activeApp.current
}

func newApp(guiApp gui.Application, build func() RootView) *app {
	return &app{
		gui:      guiApp,
		build:    build,
		windows:  make(map[string]*windowMount),
		uiThread: gothread.GetId(),
	}
}

func (a *app) onUI() bool {
	return a.uiThread != 0 && a.uiThread == gothread.GetId()
}

func (a *app) postTask(f func()) bool {
	if a.isStopping() {
		return false
	}
	if a.gui == nil {
		f()
		return true
	}
	a.gui.Post(f)
	return true
}

// RequestUpdate coalesces a declarative rebuild onto the UI thread.
func (a *app) RequestUpdate() {
	a.mu.Lock()
	if a.updatePending {
		a.mu.Unlock()
		return
	}
	a.updatePending = true
	a.mu.Unlock()

	if !a.postTask(func() {
		a.runPendingUpdate()
	}) {
		a.mu.Lock()
		a.updatePending = false
		a.mu.Unlock()
	}
}

func (a *app) runPendingUpdate() {
	a.mu.Lock()
	if !a.updatePending {
		a.mu.Unlock()
		return
	}
	a.updatePending = false
	a.mu.Unlock()

	if err := a.rebuild(); err != nil {
		a.fail(err)
	}
}

func (a *app) rebuild() error {
	if a.build == nil {
		return ErrAppBuildNil
	}
	windows, sheet, err := resolveRoot(a.build())
	if err != nil {
		return err
	}
	a.applyStyleSheet(sheet)
	return a.reconcileWindows(windows)
}

// resolveRoot extracts the windows and the optional application style sheet from
// the declarative root: a bare WindowView is one window with no app-level sheet; a
// RootNode carries both.
func resolveRoot(root RootView) ([]WindowView, style.StyleSheet, error) {
	switch root := root.(type) {
	case nil:
		return nil, nil, nil
	case WindowView:
		return []WindowView{root}, nil, nil
	case *WindowView:
		if root == nil {
			return nil, nil, nil
		}
		return []WindowView{*root}, nil, nil
	case RootNode:
		return root.windows, root.styleSheet, nil
	case *RootNode:
		if root == nil {
			return nil, nil, nil
		}
		return root.windows, root.styleSheet, nil
	default:
		return nil, nil, fmt.Errorf("unsupported ui root view %T", root)
	}
}

// applyStyleSheet reconciles the application style sheet onto gui when it changed
// since the last rebuild. A nil sheet reverts to gui's built-in default. The diff
// avoids a redundant app-wide relayout on every rebuild; sheets are compared by
// content because they are value types (== is unsafe on them).
func (a *app) applyStyleSheet(sheet style.StyleSheet) {
	if a.gui == nil || sameSheet(a.appliedSheet, sheet) {
		return
	}
	a.appliedSheet = sheet
	a.gui.SetStyleSheet(sheet)
}

func sameSheet(a, b style.StyleSheet) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return reflect.DeepEqual(a, b)
}

func (a *app) reconcileWindows(views []WindowView) error {
	seen := make(map[string]WindowView, len(views))
	for _, view := range views {
		if view.id == "" {
			return ErrWindowIDEmpty
		}
		if _, exists := seen[view.id]; exists {
			return fmt.Errorf("%w: %q", ErrWindowDuplicate, view.id)
		}
		seen[view.id] = view
	}

	for id, view := range seen {
		mount := a.windows[id]
		if mount == nil {
			var err error
			mount, err = a.createWindow(view)
			if err != nil {
				return err
			}
			a.windows[id] = mount
			continue
		}
		if err := mount.update(view); err != nil {
			return err
		}
	}

	for id, mount := range a.windows {
		if _, exists := seen[id]; exists {
			continue
		}
		delete(a.windows, id)
		mount.destroy()
	}
	return nil
}

func (a *app) createWindow(view WindowView) (*windowMount, error) {
	if a.gui == nil {
		return nil, gui.ErrAppNil
	}

	window, err := a.gui.NewWindow()
	if err != nil {
		return nil, err
	}

	mount := &windowMount{
		runtime: a,
		id:      view.id,
		window:  window,
		root:    newRoot(),
		view:    view,
	}

	if err := mount.applyWindowProperties(view); err != nil {
		window.Destroy()
		return nil, err
	}
	mount.root.mountWindow(window, func() View {
		return mount.view.content
	})

	if err := window.Show(); err != nil {
		window.Destroy()
		return nil, err
	}

	mount.connect()
	return mount, nil
}

func (m *windowMount) connect() {
	m.closeHandle = m.window.ConnectCloseRequest(func(allow *bool) {
		if m.view.onCloseRequest == nil {
			return
		}
		m.view.onCloseRequest(allow)
	})
	m.destroyHandle = m.window.ConnectDestroy(func() {
		m.runtime.windowDestroyed(m)
	})
}

func (m *windowMount) update(view WindowView) error {
	if err := m.applyWindowProperties(view); err != nil {
		return err
	}
	m.view = view
	m.root.updateWindow(m.window, view.content)
	return nil
}

func (m *windowMount) applyWindowProperties(view WindowView) error {
	m.window.SetID(view.id)
	return m.window.SetTitle(view.title)
}

func (m *windowMount) destroy() {
	if m.destroying {
		return
	}
	m.destroying = true
	m.window.Destroy()
}

func (m *windowMount) disconnect() {
	if m.closeHandle != nil {
		m.closeHandle.Disconnect()
		m.closeHandle = nil
	}
	if m.destroyHandle != nil {
		m.destroyHandle.Disconnect()
		m.destroyHandle = nil
	}
}

func (a *app) windowDestroyed(mount *windowMount) {
	if current := a.windows[mount.id]; current == mount {
		delete(a.windows, mount.id)
	}

	mount.disconnect()
	if mount.view.onDestroy != nil {
		mount.view.onDestroy()
	}
	// Quitting when the last window closes is gui.Application's policy
	// (QuitOnLastWindowClosed); the ui runtime does not decide it.
}

func (a *app) destroyAll() {
	for id, mount := range a.windows {
		delete(a.windows, id)
		mount.destroy()
	}
}

func (a *app) fail(err error) {
	if err == nil {
		return
	}

	a.mu.Lock()
	if a.err == nil {
		a.err = err
	}
	a.mu.Unlock()

	a.Quit()
}

// Quit asks the running app to stop; it is idempotent.
func (a *app) Quit() {
	a.mu.Lock()
	if a.stopping {
		a.mu.Unlock()
		return
	}
	a.stopping = true
	a.mu.Unlock()

	if a.gui != nil {
		a.gui.Quit()
	}
}

func (a *app) isStopping() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stopping
}

func (a *app) error() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.err
}
