package ui

import (
	"errors"
	"fmt"
	"runtime"
	"sync"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/dev"

	"github.com/xuges/gothread"
)

var App app

var (
	ErrAppRunning      = errors.New("ui app is already running")
	ErrAppBuildNil     = errors.New("ui app build function is nil")
	ErrWindowIDEmpty   = errors.New("ui window id is empty")
	ErrWindowDuplicate = errors.New("ui window id is duplicated")
)

type app struct {
	devAddr string
}

type appRuntime struct {
	mu            sync.Mutex
	gui           gui.Application
	build         func() RootView
	windows       map[string]*windowMount
	devServer     *dev.Server
	updatePending bool
	stopping      bool
	err           error
	uiThread      int
}

type windowMount struct {
	runtime       *appRuntime
	id            string
	window        gui.Window
	root          *root
	view          WindowView
	closeHandle   signal.Handle
	destroyHandle signal.Handle
	destroying    bool
}

var activeAppRuntime struct {
	sync.Mutex
	runtime *appRuntime
}

func (a app) DevPort(addr string) app {
	a.devAddr = addr
	return a
}

func (a app) Run(build func() RootView) error {
	if build == nil {
		return ErrAppBuildNil
	}
	if currentAppRuntime() != nil {
		return ErrAppRunning
	}

	runtime.LockOSThread()

	guiApp, err := gui.NewApplication()
	if err != nil {
		return err
	}

	rt := newAppRuntime(guiApp, build)
	if err := setActiveAppRuntime(rt); err != nil {
		return err
	}
	defer clearActiveAppRuntime(rt)

	if len(a.devAddr) != 0 {
		rt.devServer, err = dev.ListenAndServe(guiApp, a.devAddr)
		if err != nil {
			return err
		}
		defer rt.stopDevServer()
	}

	if err := rt.rebuild(); err != nil {
		rt.destroyAll()
		return err
	}

	guiApp.Run()
	return rt.error()
}

func (app) Post(f func()) {
	if f == nil {
		return
	}
	rt := currentAppRuntime()
	if rt == nil {
		return
	}
	rt.post(f)
}

func (app) Sync(f func()) {
	if f == nil {
		return
	}
	rt := currentAppRuntime()
	if rt == nil || rt.onUI() {
		f()
		return
	}

	done := make(chan struct{})
	if !rt.post(func() {
		defer close(done)
		f()
	}) {
		return
	}
	<-done
}

func (app) RequestUpdate() {
	rt := currentAppRuntime()
	if rt == nil {
		return
	}
	rt.requestUpdate()
}

func (app) Quit() {
	rt := currentAppRuntime()
	if rt == nil {
		return
	}
	rt.quit()
}

func newAppRuntime(guiApp gui.Application, build func() RootView) *appRuntime {
	return &appRuntime{
		gui:      guiApp,
		build:    build,
		windows:  make(map[string]*windowMount),
		uiThread: gothread.GetId(),
	}
}

func setActiveAppRuntime(rt *appRuntime) error {
	activeAppRuntime.Lock()
	defer activeAppRuntime.Unlock()

	if activeAppRuntime.runtime != nil {
		return ErrAppRunning
	}
	activeAppRuntime.runtime = rt
	return nil
}

func clearActiveAppRuntime(rt *appRuntime) {
	activeAppRuntime.Lock()
	defer activeAppRuntime.Unlock()

	if activeAppRuntime.runtime == rt {
		activeAppRuntime.runtime = nil
	}
}

func currentAppRuntime() *appRuntime {
	activeAppRuntime.Lock()
	defer activeAppRuntime.Unlock()
	return activeAppRuntime.runtime
}

func (rt *appRuntime) onUI() bool {
	return rt.uiThread != 0 && rt.uiThread == gothread.GetId()
}

func (rt *appRuntime) post(f func()) bool {
	if rt.isStopping() {
		return false
	}
	if rt.gui == nil {
		f()
		return true
	}
	rt.gui.Post(f)
	return true
}

func (rt *appRuntime) requestUpdate() {
	rt.mu.Lock()
	if rt.updatePending {
		rt.mu.Unlock()
		return
	}
	rt.updatePending = true
	rt.mu.Unlock()

	if !rt.post(func() {
		rt.runPendingUpdate()
	}) {
		rt.mu.Lock()
		rt.updatePending = false
		rt.mu.Unlock()
	}
}

func (rt *appRuntime) runPendingUpdate() {
	rt.mu.Lock()
	if !rt.updatePending {
		rt.mu.Unlock()
		return
	}
	rt.updatePending = false
	rt.mu.Unlock()

	if err := rt.rebuild(); err != nil {
		rt.fail(err)
	}
}

func (rt *appRuntime) rebuild() error {
	if rt.build == nil {
		return ErrAppBuildNil
	}
	windows, err := rootWindows(rt.build())
	if err != nil {
		return err
	}
	return rt.reconcileWindows(windows)
}

func rootWindows(root RootView) ([]WindowView, error) {
	switch root := root.(type) {
	case nil:
		return nil, nil
	case WindowView:
		return []WindowView{root}, nil
	case *WindowView:
		if root == nil {
			return nil, nil
		}
		return []WindowView{*root}, nil
	default:
		return nil, fmt.Errorf("unsupported ui root view %T", root)
	}
}

func (rt *appRuntime) reconcileWindows(views []WindowView) error {
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
		mount := rt.windows[id]
		if mount == nil {
			var err error
			mount, err = rt.createWindow(view)
			if err != nil {
				return err
			}
			rt.windows[id] = mount
			continue
		}
		if err := mount.update(view); err != nil {
			return err
		}
	}

	for id, mount := range rt.windows {
		if _, exists := seen[id]; exists {
			continue
		}
		delete(rt.windows, id)
		mount.destroy()
	}
	return nil
}

func (rt *appRuntime) createWindow(view WindowView) (*windowMount, error) {
	if rt.gui == nil {
		return nil, gui.ErrAppNil
	}

	window, err := rt.gui.NewWindow()
	if err != nil {
		return nil, err
	}

	mount := &windowMount{
		runtime: rt,
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

func (rt *appRuntime) windowDestroyed(mount *windowMount) {
	if current := rt.windows[mount.id]; current == mount {
		delete(rt.windows, mount.id)
	}

	mount.disconnect()
	if mount.view.onDestroy != nil {
		mount.view.onDestroy()
	}
	if len(rt.windows) == 0 {
		rt.quit()
	}
}

func (rt *appRuntime) destroyAll() {
	for id, mount := range rt.windows {
		delete(rt.windows, id)
		mount.destroy()
	}
}

func (rt *appRuntime) stopDevServer() {
	if rt == nil || rt.devServer == nil {
		return
	}
	dev := rt.devServer
	rt.devServer = nil
	_ = dev.Close()
}

func (rt *appRuntime) fail(err error) {
	if err == nil {
		return
	}

	rt.mu.Lock()
	if rt.err == nil {
		rt.err = err
	}
	rt.mu.Unlock()

	rt.quit()
}

func (rt *appRuntime) quit() {
	rt.mu.Lock()
	if rt.stopping {
		rt.mu.Unlock()
		return
	}
	rt.stopping = true
	rt.mu.Unlock()

	if rt.gui != nil {
		rt.gui.Quit()
	}
}

func (rt *appRuntime) isStopping() bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.stopping
}

func (rt *appRuntime) error() error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.err
}
