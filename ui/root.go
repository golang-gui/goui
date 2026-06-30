package ui

import (
	"reflect"
	"sync"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

type View interface {
	Build(ctx BuildContext, old gui.Widget) gui.Widget
}

type BuildContext interface {
	UpdateChildren(container gui.Container, children []View)
	Connect(widget gui.Widget, name string, connect func() signal.Handle)
}

type Root struct {
	mu            sync.Mutex
	root          *node
	window        gui.Window
	build         func() View
	updatePending bool
	destroyHandle signal.Handle
}

type node struct {
	viewType reflect.Type
	widget   gui.Widget
	children []*node
	handles  map[connectionKey]signal.Handle
}

type connectionKey struct {
	widget gui.Widget
	name   string
}

type buildContext struct {
	root *Root
	node *node
}

func NewRoot() *Root {
	return &Root{}
}

func MountWindow(window gui.Window, build func() View) *Root {
	root := NewRoot()
	root.MountWindow(window, build)
	return root
}

func (r *Root) Widget() gui.Widget {
	if r.root == nil {
		return nil
	}
	return r.root.widget
}

func (r *Root) Update(view View) gui.Widget {
	r.root = r.updateNode(r.root, view)
	return r.Widget()
}

func (r *Root) UpdateWindow(window gui.Window, view View) gui.Widget {
	widget := r.Update(view)
	if window != nil {
		window.SetWidget(widget)
	}
	return widget
}

func (r *Root) MountWindow(window gui.Window, build func() View) {
	r.unmount(true)

	var destroyHandle signal.Handle
	if window != nil {
		destroyHandle = window.ConnectDestroy(func() {
			r.unmountForWindowDestroy()
		})
	}

	r.mu.Lock()
	r.window = window
	r.build = build
	r.updatePending = false
	r.destroyHandle = destroyHandle
	r.mu.Unlock()

	r.UpdateNow()
}

func (r *Root) RequestUpdate() {
	r.mu.Lock()
	if r.build == nil || r.updatePending {
		r.mu.Unlock()
		return
	}
	r.updatePending = true
	app := gui.App
	r.mu.Unlock()

	if app == nil {
		r.UpdateNow()
		return
	}
	app.Post(func() {
		r.runPendingUpdate()
	})
}

func (r *Root) UpdateNow() gui.Widget {
	r.mu.Lock()
	r.updatePending = false
	r.mu.Unlock()

	return r.updateMounted()
}

func (r *Root) Unmount() {
	r.unmount(true)
}

func (r *Root) runPendingUpdate() {
	r.mu.Lock()
	if !r.updatePending {
		r.mu.Unlock()
		return
	}
	r.updatePending = false
	r.mu.Unlock()

	r.updateMounted()
}

func (r *Root) updateMounted() gui.Widget {
	r.mu.Lock()
	window := r.window
	build := r.build
	r.mu.Unlock()

	if build == nil {
		return r.Widget()
	}
	view := build()
	if window != nil {
		return r.UpdateWindow(window, view)
	}
	return r.Update(view)
}

func (r *Root) unmount(detachWindow bool) {
	r.mu.Lock()
	window := r.window
	destroyHandle := r.destroyHandle
	oldRoot := r.root
	var oldWidget gui.Widget
	if oldRoot != nil {
		oldWidget = oldRoot.widget
	}
	r.window = nil
	r.build = nil
	r.updatePending = false
	r.destroyHandle = nil
	r.root = nil
	r.mu.Unlock()

	if destroyHandle != nil {
		destroyHandle.Disconnect()
	}
	r.release(oldRoot)

	if detachWindow && window != nil && oldWidget != nil && window.Widget() == oldWidget {
		window.SetWidget(nil)
	}
}

func (r *Root) unmountForWindowDestroy() {
	r.mu.Lock()
	destroyHandle := r.destroyHandle
	oldRoot := r.root
	r.window = nil
	r.build = nil
	r.updatePending = false
	r.destroyHandle = nil
	r.root = nil
	r.mu.Unlock()

	if destroyHandle != nil {
		destroyHandle.Disconnect()
	}
	r.releaseHandles(oldRoot)
}

func (r *Root) updateNode(old *node, view View) *node {
	if view == nil {
		r.release(old)
		return nil
	}

	viewType := reflect.TypeOf(view)
	if old != nil && old.viewType != viewType {
		r.release(old)
		old = nil
	}

	current := old
	if current == nil {
		current = &node{viewType: viewType}
	}

	ctx := &buildContext{
		root: r,
		node: current,
	}
	var oldWidget gui.Widget
	if old != nil {
		oldWidget = old.widget
	}
	widget := view.Build(ctx, oldWidget)
	if widget == nil {
		r.release(current)
		return nil
	}

	current.viewType = viewType
	current.widget = widget
	return current
}

func (r *Root) release(n *node) {
	if n == nil {
		return
	}
	if container, ok := n.widget.(gui.Container); ok {
		for _, child := range container.Children() {
			container.RemoveChild(child)
		}
	}
	for _, child := range n.children {
		r.release(child)
	}
	n.children = nil

	for key, handle := range n.handles {
		if handle != nil {
			handle.Disconnect()
		}
		delete(n.handles, key)
	}
}

func (r *Root) releaseHandles(n *node) {
	if n == nil {
		return
	}
	for _, child := range n.children {
		r.releaseHandles(child)
	}
	n.children = nil

	for key, handle := range n.handles {
		if handle != nil {
			handle.Disconnect()
		}
		delete(n.handles, key)
	}
}

func (ctx *buildContext) UpdateChildren(container gui.Container, children []View) {
	if container == nil {
		return
	}

	children = compactViews(children)
	oldNodes := ctx.node.children
	newNodes := make([]*node, 0, len(children))

	index := 0
	for index < len(oldNodes) && index < len(children) {
		if !sameViewType(oldNodes[index], children[index]) {
			break
		}
		child := ctx.root.updateNode(oldNodes[index], children[index])
		if child == nil || child.widget == nil {
			if oldNodes[index] != nil && oldNodes[index].widget != nil {
				container.RemoveChild(oldNodes[index].widget)
			}
			index++
			continue
		}
		newNodes = append(newNodes, child)
		index++
	}

	for _, old := range oldNodes[index:] {
		if old != nil && old.widget != nil {
			container.RemoveChild(old.widget)
		}
		ctx.root.release(old)
	}

	for _, childView := range children[index:] {
		child := ctx.root.updateNode(nil, childView)
		if child == nil || child.widget == nil {
			continue
		}
		container.AddChild(child.widget)
		newNodes = append(newNodes, child)
	}

	ctx.node.children = newNodes
}

func (ctx *buildContext) Connect(widget gui.Widget, name string, connect func() signal.Handle) {
	if widget == nil || name == "" {
		return
	}
	if ctx.node.handles == nil {
		ctx.node.handles = make(map[connectionKey]signal.Handle)
	}
	key := connectionKey{
		widget: widget,
		name:   name,
	}
	if handle := ctx.node.handles[key]; handle != nil {
		handle.Disconnect()
		delete(ctx.node.handles, key)
	}
	if connect == nil {
		return
	}
	handle := connect()
	if handle != nil {
		ctx.node.handles[key] = handle
	}
}

func sameViewType(old *node, view View) bool {
	return old != nil && view != nil && old.viewType == reflect.TypeOf(view)
}

func compactViews(views []View) []View {
	if len(views) == 0 {
		return nil
	}
	compacted := make([]View, 0, len(views))
	for _, view := range views {
		if view != nil {
			compacted = append(compacted, view)
		}
	}
	return compacted
}
