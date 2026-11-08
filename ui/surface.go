package ui

import (
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

// SurfaceView declares the system-provided main presentation host. It owns no
// window-management requests. Rebuilds with the same ID retain its GUI host.
type SurfaceView struct {
	id           string
	content      View
	commands     []*CommandView
	shortcuts    []*ShortcutView
	onDestroy    func()
	onAfterPaint func(gui.Painter, geometry.Rectangle)
}

// Surface declares a system host with a nonempty application-local identity.
// It is acquired during activation; unsupported platforms return an error from Run.
func Surface(id string) SurfaceView { return SurfaceView{id: id} }

func (SurfaceView) rootView() {}

// Content sets the root widget declaration.
func (v SurfaceView) Content(content View) SurfaceView { v.content = content; return v }

// Commands declares the host's default command group, shared with its shortcuts.
func (v SurfaceView) Commands(commands ...*CommandView) SurfaceView {
	v.commands = slices.Clone(commands)
	return v
}

// Shortcuts declares bindings on the host's ordinary shortcut controller.
func (v SurfaceView) Shortcuts(bindings ...*ShortcutView) SurfaceView {
	v.shortcuts = slices.Clone(bindings)
	return v
}

// OnDestroy handles final host destruction, not a temporary drawing-target loss.
func (v SurfaceView) OnDestroy(fn func()) SurfaceView { v.onDestroy = fn; return v }

// OnAfterPaint draws after content and before presentation. The Painter is borrowed
// synchronously. Rebuilds replace the callback without adding another connection.
func (v SurfaceView) OnAfterPaint(fn func(gui.Painter, geometry.Rectangle)) SurfaceView {
	v.onAfterPaint = fn
	return v
}

// Surface sets the application's single system-host declaration. Windows can be
// declared separately; the platform determines which hosts it can actually create.
func (r RootNode) Surface(surface SurfaceView) RootNode { r.surface = &surface; return r }

// hostMount holds only common declarative content and connection lifetimes.
// windowMount adds creation options, management properties and close requests.
type hostMount struct {
	runtime          *app
	id               string
	host             gui.Surface
	root             *root
	view             SurfaceView
	shortcuts        shortcutBindings
	commands         *commandGroup
	afterPaintHandle signal.Handle
	destroyHandle    signal.Handle
	destroying       bool
}

func newHostMount(a *app, host gui.Surface, view SurfaceView) *hostMount {
	m := &hostMount{runtime: a, id: view.id, host: host, root: newRoot(), view: view}
	m.root.runtime = a
	return m
}

func (m *hostMount) contentView() View {
	content := &hostContentView{mount: m, child: m.view.content, shortcuts: m.view.shortcuts}
	content.Self = content
	return CommandGroup(content).Commands(m.view.commands...)
}

func (m *hostMount) connect(onDestroy func()) {
	m.afterPaintHandle = m.host.ConnectAfterPaint(func(p gui.Painter, bounds geometry.Rectangle) {
		if fn := m.view.onAfterPaint; fn != nil {
			fn(p, bounds)
		}
	})
	m.destroyHandle = m.host.ConnectDestroy(onDestroy)
}

func (m *hostMount) update(view SurfaceView) {
	paint := m.view.onAfterPaint != nil || view.onAfterPaint != nil
	m.view = view
	m.root.updateHost(m.host, m.contentView())
	if paint && !m.destroying {
		_ = m.host.RequestPaint()
	}
}

func (m *hostMount) destroy() {
	if m.destroying {
		return
	}
	m.destroying = true
	m.host.Destroy()
}

func (m *hostMount) disconnect() {
	m.shortcuts.clear()
	if m.afterPaintHandle != nil {
		m.afterPaintHandle.Disconnect()
		m.afterPaintHandle = nil
	}
	if m.destroyHandle != nil {
		m.destroyHandle.Disconnect()
		m.destroyHandle = nil
	}
}

func (a *app) createSurface(view SurfaceView) (*hostMount, error) {
	if a.gui == nil {
		return nil, gui.ErrAppNil
	}
	host, err := a.gui.NewSurface()
	if err != nil {
		return nil, err
	}
	host.SetID(view.id)
	m := newHostMount(a, host, view)
	a.surface = m
	m.connect(func() { a.surfaceDestroyed(m) })
	m.root.mountHost(host, m.contentView)
	return m, nil
}

func (a *app) surfaceDestroyed(m *hostMount) {
	m.destroying = true
	if a.surface == m {
		a.surface = nil
	}
	m.root.unmountForHostDestroy()
	m.disconnect()
	if m.view.onDestroy != nil {
		m.view.onDestroy()
	}
}

// Root-level bindings use the same ComposeView lifetime as ordinary content.
type hostContentView struct {
	ComposeBase[hostContentView]
	mount     *hostMount
	child     View
	shortcuts []*ShortcutView
}

func (v *hostContentView) Build(ctx BuildContext) View {
	ctx.SetState(v.mount)
	v.mount.commands, _ = FindAncestor[*commandGroup](ctx)
	if len(v.shortcuts) != 0 || v.mount.shortcuts.controller != nil {
		v.mount.shortcuts.update(v.mount.host.Shortcuts(), v.shortcuts, ctx)
	}
	if ctx.State() != v.mount || v.mount.destroying {
		return nil
	}
	return v.child
}

func (*hostContentView) Object(ctx BuildContext) any { return ctx.State() }

func (*hostContentView) Release(ctx BuildContext) {
	if mount, ok := ctx.State().(*hostMount); ok {
		mount.commands = nil
		mount.shortcuts.clear()
	}
}

func (w WindowView) commonView() SurfaceView {
	return SurfaceView{id: w.id, content: w.content, commands: w.commands, shortcuts: w.shortcuts,
		onDestroy: w.onDestroy, onAfterPaint: w.onAfterPaint}
}
