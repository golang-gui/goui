package ui

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/style"

	"github.com/xuges/gothread"
)

// RootView is the root declarative view.
type RootView interface {
	rootView()
}

// RootNode is the optional application-level declarative node. It carries app-wide
// declarations (currently the style sheet) and presentation hosts. Return it from
// the Run build closure when you need app-level configuration; otherwise return a
// WindowView or SurfaceView directly.
type RootNode struct {
	styleSheet style.StyleSheet
	windows    []WindowView
	surface    *SurfaceView
}

// Root creates an application-level root node.
func Root() RootNode {
	return RootNode{}
}

func (r RootNode) rootView() {}

// StyleSheet sets the application style sheet. It is reconciled onto the running
// application on each rebuild, so swapping it re-skins the app at runtime; a nil
// sheet reverts to the built-in default.
func (r RootNode) StyleSheet(sheet style.StyleSheet) RootNode {
	r.styleSheet = sheet
	return r
}

// Windows sets the top-level windows of the application.
func (r RootNode) Windows(windows ...WindowView) RootNode {
	r.windows = windows
	return r
}

// App is the running application handle passed into the Run build closure. It is
// the running *app itself: valid for the whole run and safe to stash,
// capture in handlers, or call from any goroutine. After Quit, work is not
// dispatched; result-bearing commands report ErrAppStopped as documented.
type App interface {
	// Post runs f on the UI thread asynchronously.
	Post(func())
	// Sync runs f on the UI thread and waits, running inline when already on the
	// UI thread.
	Sync(func())
	// Quit asks the running app to stop.
	Quit()
	// RequestUpdate coalesces an application-wide declarative rebuild on the UI
	// thread. Use property bindings or ComposeView.Build for local updates.
	RequestUpdate()
	// FindWindow borrows the currently mounted GUI window with id. Empty or
	// missing IDs, an unmounting window, and a stopped app return nil. Lookup is
	// synchronized, but the returned object remains GUI-thread-affine: use it
	// in an event callback or put both lookup and use inside Post/Sync.
	// It does not create a window or transfer ownership. Do not mutate its ID or
	// declaratively managed content. A recreated ID denotes a new object; an old
	// reference is not retargeted. A registered hidden window is discoverable
	// during AfterUpdate, so it can be prepared before its first Show.
	FindWindow(id string) gui.Window
	// FindSurface borrows any mounted host by ID, including a managed Window.
	// Threading, identity and lifetime rules are the same as FindWindow.
	FindSurface(id string) gui.Surface
	// FindWidget borrows the currently mounted widget with id in a host's
	// content tree. Missing IDs return nil
	FindWidget(hostID, widgetID string) gui.Widget
	// FindCommand borrows a published command from a host's default group.
	// It does not search descendant groups or create commands. Empty/missing IDs,
	// an unmounting host or a stopped app return nil. The returned object stays
	// GUI-thread-affine, like FindWidget; look it up again after definition changes.
	FindCommand(hostID, commandID string) *gui.Command
	// Clipboard returns the system clipboard view (never nil).
	Clipboard() Clipboard
	// Settings returns the system settings view (never nil).
	Settings() Settings
	// FileDialog returns the system file dialog view (never nil).
	FileDialog() FileDialog
	// OpenURL asks the system's registered handler to open an absolute URL,
	// including custom schemes. It executes on the GUI thread and returns the
	// native request result, not external app completion. It may block during
	// dispatch. After Quit, or if exit discards the request, returns ErrAppStopped.
	OpenURL(rawURL string) error
	// OpenPath opens a file or directory with its default application. Relative
	// paths use the working directory at execution; no shell expansion is done.
	// Threading, request completion and exit semantics are the same as OpenURL.
	OpenPath(path string) error
	// TimeoutFunc starts a one-shot GUI-thread callback. The returned timer can
	// be stopped or restarted with the same callback. See Timer for lifetime rules.
	// Panics if delay is not positive or fn is nil; after Quit returns an inert timer.
	TimeoutFunc(delay time.Duration, fn func()) *Timer
	// TickFunc starts periodic GUI-thread callbacks, first after interval.
	// Panics if interval is not positive or fn is nil; after Quit returns an inert timer.
	TickFunc(interval time.Duration, fn func()) *Timer
}

var (
	ErrAppStopped      = errors.New("ui: application has stopped")
	ErrAppRunOnce      = errors.New("ui: Run may be called only once per process")
	ErrAppBuildNil     = errors.New("ui app build function is nil")
	ErrSurfaceIDEmpty  = errors.New("ui surface id is empty")
	ErrWindowIDEmpty   = errors.New("ui window id is empty")
	ErrWindowDuplicate = errors.New("ui window id is duplicated")
)

// Run prepares the application and builds its initial declarative tree during
// native UI activation. It waits until shutdown completes. build receives App, which it
// may stash, capture in handlers, or call from other goroutines.
// Call Run from main. Desktop dispatch keeps its startup OS thread; mobile main
// waits while native UI callbacks build and update views. Use App.Post/Sync for
// background work. Initialization and declarative-build errors are returned.
// System settings and mlang language changes automatically request a coalesced
// rebuild. Resource getters should be called inside build, not resolved once
// outside it. Change language on the UI thread, or inside App.Post.
// appId is the immutable application identity (for example "com.example.Editor"),
// not a window ID. Empty leaves it unspecified. It is passed to gui.NewApplication
// before build runs and does not participate in declarative updates.
func Run(appId string, build func(app App) RootView) error {
	if build == nil {
		return ErrAppBuildNil
	}
	if err := activeRun(); err != nil {
		return err
	}

	guiApp, err := gui.NewApplication(appId)
	if err != nil {
		return err
	}

	var a *app
	a = newApp(guiApp, func() RootView { return build(a) })
	return a.run()
}

// run binds native UI lifetimes before dispatch begins. Final waiter release
// happens after Run returns, independently of the startup thread's identity.
func (a *app) run() error {
	defer a.finish()
	a.uiThread.Store(0)
	var languageHandle, settingsHandle signal.Handle
	activated := false
	activate := a.gui.ConnectActivate(func() {
		if activated || a.isStopping() {
			return
		}
		activated = true
		a.uiThread.Store(int64(gothread.GetId()))
		languageHandle = mlang.ConnectChanged(a.RequestUpdate)
		if settings := a.gui.Settings(); settings != nil {
			settingsHandle = settings.ConnectChanged(a.RequestUpdate)
		}
		if err := a.rebuild(); err != nil {
			a.fail(err)
			a.destroyAll()
		}
	})
	defer activate.Disconnect()
	shutdown := a.gui.ConnectShutdown(func() {
		if settingsHandle != nil {
			settingsHandle.Disconnect()
		}
		if languageHandle != nil {
			languageHandle.Disconnect()
		}
		a.release()
	})
	defer shutdown.Disconnect()
	if err := a.gui.Run(); err != nil {
		return err
	}
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
	if f == nil || a.isStopping() {
		return
	}
	if a.onUI() {
		f()
		return
	}

	done := make(chan struct{})
	if !a.postTask(func() {
		defer close(done)
		if !a.isStopping() {
			f()
		}
	}) {
		return
	}
	select {
	case <-done:
	case <-a.finished:
		// Run has returned, so no in-flight UI callback can still be writing
		// results. Queued work that did not execute is discarded.
	}
}

func (a *app) OpenURL(rawURL string) (err error) {
	err = ErrAppStopped
	a.Sync(func() { err = a.gui.OpenURL(rawURL) })
	return
}

func (a *app) OpenPath(path string) (err error) {
	err = ErrAppStopped
	a.Sync(func() { err = a.gui.OpenPath(path) })
	return
}

func (a *app) FindWindow(id string) (window gui.Window) {
	if id == "" {
		return nil
	}
	a.Sync(func() {
		if mount := a.windows[id]; mount != nil && !mount.destroying {
			window = mount.window
		}
	})
	return
}

// mountedHost is read only on the UI thread. IDs share one namespace.
func (a *app) mountedHost(id string) *hostMount {
	if id == "" {
		return nil
	}
	if m := a.windows[id]; m != nil && !m.destroying {
		return m.hostMount
	}
	if m := a.surface; m != nil && m.id == id && !m.destroying {
		return m
	}
	return nil
}

func (a *app) FindSurface(id string) (host gui.Surface) {
	a.Sync(func() {
		if m := a.mountedHost(id); m != nil {
			host = m.host
		}
	})
	return
}

func (a *app) FindWidget(hostID, widgetID string) (widget gui.Widget) {
	a.Sync(func() {
		if m := a.mountedHost(hostID); m != nil {
			widget = gui.FindWidget(m.host, widgetID)
		}
	})
	return
}

func (a *app) FindCommand(hostID, commandID string) (command *gui.Command) {
	if hostID == "" || commandID == "" {
		return nil
	}
	a.Sync(func() {
		if m := a.mountedHost(hostID); m != nil && m.commands != nil {
			command = m.commands.find(commandID)
		}
	})
	return
}

type app struct {
	mu               sync.Mutex
	gui              gui.Application
	build            func() RootView
	windows          map[string]*windowMount
	surface          *hostMount
	updatingHosts    bool // guards the whole batch, including completion callbacks and Show
	reconcilingHosts bool // defer completion callbacks until every host tree is updated
	appliedSheet     style.StyleSheet
	fullUpdate       bool
	dirty            map[*node]updateKind
	dirtyOrder       []*node
	updateDepth      int
	batch            uint64
	updatePending    bool
	stopping         bool
	finished         chan struct{} // closed only after run returns, not on a Quit request
	err              error
	uiThread         atomic.Int64
}

type windowMount struct {
	*hostMount
	window         gui.Window
	view           WindowView
	appliedMinSize geometry.Size // last explicit minimum forwarded; dedupes native calls
	closeHandle    signal.Handle
}

var activeApp struct {
	sync.Mutex
	ran bool // one-shot: set on the first Run, never cleared (the loop is consumed)
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

func newApp(guiApp gui.Application, build func() RootView) *app {
	a := &app{
		gui:      guiApp,
		build:    build,
		windows:  make(map[string]*windowMount),
		finished: make(chan struct{}),
	}
	a.uiThread.Store(int64(gothread.GetId()))
	return a
}

func (a *app) onUI() bool {
	thread := a.uiThread.Load()
	return thread != 0 && thread == int64(gothread.GetId())
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
	if !a.stopping {
		a.fullUpdate = true
	}
	a.mu.Unlock()
	a.scheduleUpdate()
}

func (a *app) runPendingUpdate() {
	a.mu.Lock()
	a.updatePending = false
	if a.stopping || a.updateDepth != 0 {
		a.mu.Unlock()
		return
	}
	full := a.fullUpdate
	jobs := make([]nodeUpdate, 0, len(a.dirtyOrder))
	for _, n := range a.dirtyOrder {
		jobs = append(jobs, nodeUpdate{n, a.dirty[n]})
	}
	a.fullUpdate = false
	a.dirty = nil
	a.dirtyOrder = nil
	a.mu.Unlock()
	var err error
	if full {
		err = a.rebuildWith(jobs)
	} else if len(jobs) > 0 {
		err = a.reconcileBatch(rootDeclaration{}, jobs, false)
	}
	if err != nil {
		a.fail(err)
	}
}

func (a *app) rebuild() error { return a.rebuildWith(nil) }

func (a *app) rebuildWith(jobs []nodeUpdate) error {
	defer a.holdUpdates()()
	if a.build == nil {
		return ErrAppBuildNil
	}
	declaration, err := resolveRoot(a.build())
	if err != nil {
		return err
	}
	a.applyStyleSheet(declaration.styleSheet)
	return a.reconcileBatch(declaration, jobs, true)
}

type rootDeclaration struct {
	windows    []WindowView
	surface    *SurfaceView
	styleSheet style.StyleSheet
}

// resolveRoot reads application declarations without creating native resources.
func resolveRoot(view RootView) (rootDeclaration, error) {
	switch v := view.(type) {
	case nil:
		return rootDeclaration{}, nil
	case WindowView:
		return rootDeclaration{windows: []WindowView{v}}, nil
	case *WindowView:
		if v != nil {
			return resolveRoot(*v)
		}
	case SurfaceView:
		return rootDeclaration{surface: &v}, nil
	case *SurfaceView:
		if v != nil {
			return resolveRoot(*v)
		}
	case RootNode:
		return rootDeclaration{windows: v.windows, surface: v.surface, styleSheet: v.styleSheet}, nil
	case *RootNode:
		if v != nil {
			return resolveRoot(*v)
		}
	default:
		return rootDeclaration{}, fmt.Errorf("unsupported ui root view %T", view)
	}
	return rootDeclaration{}, nil
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
	return a.reconcileBatch(rootDeclaration{windows: views}, nil, true)
}

func (a *app) reconcileBatch(declaration rootDeclaration, jobs []nodeUpdate, full bool) error {
	views := declaration.windows
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

	if v := declaration.surface; v != nil {
		if v.id == "" {
			return ErrSurfaceIDEmpty
		}
		if _, conflict := seen[v.id]; conflict {
			return fmt.Errorf("ui: Surface and Window share host ID %q", v.id)
		}
	}

	if a.updatingHosts {
		return fmt.Errorf("ui: cannot nest host reconciliation; request a later update")
	}
	defer a.holdUpdates()()
	a.batch++
	a.updatingHosts, a.reconcilingHosts = true, true
	defer func() { a.updatingHosts, a.reconcilingHosts = false, false }()
	if full && a.surface != nil && (declaration.surface == nil || a.surface.id != declaration.surface.id) {
		old := a.surface
		a.surface = nil
		old.destroy()
	}
	mounted := make([]*windowMount, 0, len(views))
	created := make([]*windowMount, 0, len(views))
	for _, view := range views {
		mount := a.windows[view.id]
		if mount == nil {
			var err error
			mount, err = a.createWindow(view)
			if err != nil {
				return err
			}
			created = append(created, mount)
		} else if err := mount.update(view); err != nil {
			return err
		}
		mounted = append(mounted, mount)
	}

	var system *hostMount
	if full && declaration.surface != nil {
		system = a.surface
		if system == nil {
			var err error
			system, err = a.createSurface(*declaration.surface)
			if err != nil {
				return err
			}
		} else {
			system.update(*declaration.surface)
		}
	}

	if full {
		for id, mount := range a.windows {
			if _, exists := seen[id]; exists {
				continue
			}
			delete(a.windows, id)
			mount.destroy()
		}
	}
	localRoots := a.processNodeUpdates(jobs)
	// Completion callbacks can transfer nodes and update the declarations. No
	// tree may still consume this rebuild's old declarations after that point.
	a.reconcilingHosts = false
	if system != nil && !system.destroying {
		system.root.flushAfterUpdate()
	}
	for _, mount := range mounted {
		if !mount.destroying {
			mount.root.flushAfterUpdate()
		}
	}
	for _, r := range localRoots {
		r.flushAfterUpdate()
	}
	for _, mount := range created {
		if !mount.destroying {
			if err := mount.window.Show(); err != nil {
				// Completion may have transferred retained content into this
				// window. Keep ownership until the caller's normal cleanup.
				return err
			}
		}
	}
	return nil
}

func (a *app) createWindow(view WindowView) (*windowMount, error) {
	if a.gui == nil {
		return nil, gui.ErrAppNil
	}

	window, err := a.gui.NewWindow(&view.options)
	if err != nil {
		return nil, err
	}

	mount := &windowMount{hostMount: newHostMount(a, window, view.commonView()), window: window, view: view}

	if err := mount.applyWindowProperties(view); err != nil {
		window.Destroy()
		return nil, err
	}
	// Register the hidden host before mounting its tree. AfterUpdate callbacks
	// may position the window or adopt a node from another root;
	// both need the same ordinary ID lookup and ownership as visible windows.
	a.windows[view.id] = mount
	mount.connect()
	mount.root.mountHost(window, func() View {
		return mount.contentView()
	})
	return mount, nil
}

func (m *windowMount) connect() {
	m.hostMount.connect(func() { m.runtime.windowDestroyed(m) })
	m.closeHandle = m.window.ConnectCloseRequest(func(allow *bool) {
		if fn := m.view.onCloseRequest; fn != nil {
			fn(allow)
		}
	})
}

func (m *windowMount) update(view WindowView) error {
	if view.options != m.view.options {
		return fmt.Errorf("window %q: Size, Chrome, Transparent, SkipTaskbar and Center are creation-only; use a new window ID to change them", view.id)
	}
	if err := m.applyWindowProperties(view); err != nil {
		return err
	}
	m.view = view
	m.hostMount.update(view.commonView())
	return nil
}

func (m *windowMount) applyWindowProperties(view WindowView) error {
	m.window.SetID(view.id)
	if err := m.window.SetTitle(view.title); err != nil {
		return err
	}
	// Forward only when the resolved minimum actually changes, so routine
	// rebuilds do not issue redundant native calls. A declaration removed by a
	// rebuild resolves to the zero size, which clears the platform hint.
	if min := view.mergedMinSize(); min != m.appliedMinSize {
		m.appliedMinSize = min
		m.window.SetMinSize(min)
	}
	return nil
}

func (m *windowMount) disconnect() {
	m.hostMount.disconnect()
	if m.closeHandle != nil {
		m.closeHandle.Disconnect()
		m.closeHandle = nil
	}
}

func (a *app) windowDestroyed(mount *windowMount) {
	mount.destroying = true
	if current := a.windows[mount.id]; current == mount {
		delete(a.windows, mount.id)
	}

	// End declarative lifetimes before detaching window bindings or invoking
	// user callbacks. Those notifications may try to activate a borrowed command.
	if mount.root != nil {
		mount.root.unmountForHostDestroy()
	}
	mount.disconnect()
	if mount.view.onDestroy != nil {
		mount.view.onDestroy()
	}
	// Quitting when the last window closes is gui.Application's policy
	// (QuitOnLastWindowClosed); the ui runtime does not decide it.
}

func (a *app) destroyAll() {
	if m := a.surface; m != nil {
		a.surface = nil
		m.destroy()
	}
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
	a.fullUpdate = false
	a.dirty = nil
	a.dirtyOrder = nil
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

// release ends declarative lifetimes on the native UI thread. It does not
// release Sync waiters: the callback causing shutdown may still be in progress.
func (a *app) release() {
	a.mu.Lock()
	a.stopping = true
	a.updatePending = false
	a.fullUpdate = false
	a.dirty = nil
	a.dirtyOrder = nil
	a.mu.Unlock()
	if m := a.surface; m != nil {
		m.root.unmountForHostDestroy()
		m.shortcuts.clear()
	}
	for _, mount := range a.windows {
		mount.root.unmountForHostDestroy()
		mount.shortcuts.clear()
	}
}

// finish only touches Go synchronization. The native loop has acknowledged
// callback completion, so it is safe to release waiters on the startup thread.
func (a *app) finish() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopping = true
	select {
	case <-a.finished:
	default:
		close(a.finished)
	}
}

// stop combines both boundaries for a runtime without a native Run lifecycle.
func (a *app) stop() { a.release(); a.finish() }

func (a *app) error() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.err
}
