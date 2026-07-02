package ui

import (
	"reflect"
	"sync"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

type root struct {
	mu            sync.Mutex
	root          *node
	window        gui.Window
	build         func() View
	updatePending bool
	destroyHandle signal.Handle
}

type node struct {
	viewType reflect.Type
	view     View
	widget   gui.Widget
	state    any
	children []*node
}

type buildContext struct {
	root *root
	node *node
}

func newRoot() *root {
	return &root{}
}

func (r *root) widget() gui.Widget {
	if r.root == nil {
		return nil
	}
	return r.root.widget
}

func (r *root) update(view View) gui.Widget {
	r.root = r.updateNode(r.root, view)
	return r.widget()
}

func (r *root) updateWindow(window gui.Window, view View) gui.Widget {
	widget := r.update(view)
	if window != nil {
		window.SetWidget(widget)
	}
	return widget
}

func (r *root) mountWindow(window gui.Window, build func() View) {
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

	r.updateNow()
}

func (r *root) requestUpdate() {
	r.mu.Lock()
	if r.build == nil || r.updatePending {
		r.mu.Unlock()
		return
	}
	r.updatePending = true
	app := gui.App
	r.mu.Unlock()

	if app == nil {
		r.updateNow()
		return
	}
	app.Post(func() {
		r.runPendingUpdate()
	})
}

func (r *root) updateNow() gui.Widget {
	r.mu.Lock()
	r.updatePending = false
	r.mu.Unlock()

	return r.updateMounted()
}

func (r *root) unmountWindow() {
	r.unmount(true)
}

func (r *root) runPendingUpdate() {
	r.mu.Lock()
	if !r.updatePending {
		r.mu.Unlock()
		return
	}
	r.updatePending = false
	r.mu.Unlock()

	r.updateMounted()
}

func (r *root) updateMounted() gui.Widget {
	r.mu.Lock()
	window := r.window
	build := r.build
	r.mu.Unlock()

	if build == nil {
		return r.widget()
	}
	view := build()
	if window != nil {
		return r.updateWindow(window, view)
	}
	return r.update(view)
}

func (r *root) unmount(detachWindow bool) {
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
	r.release(oldRoot, true)

	if detachWindow && window != nil && oldWidget != nil && window.Widget() == oldWidget {
		window.SetWidget(nil)
	}
}

func (r *root) unmountForWindowDestroy() {
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
	r.release(oldRoot, false)
}

func (r *root) updateNode(old *node, view View) *node {
	if view == nil {
		r.release(old, true)
		return nil
	}

	viewType := reflect.TypeOf(view)
	if old != nil && old.viewType != viewType {
		r.release(old, true)
		old = nil
	}

	current := old
	if current == nil {
		current = &node{
			viewType: viewType,
			view:     view,
		}
		ctx := &buildContext{root: r, node: current}
		current.widget = view.Mount(ctx)
		if current.widget == nil {
			return nil
		}
	}

	ctx := &buildContext{root: r, node: current}
	current.viewType = viewType
	current.view = view
	view.Update(ctx, current.widget)
	return current
}

func (r *root) release(n *node, detachWidgets bool) {
	if n == nil {
		return
	}

	container, _ := n.widget.(gui.Container)
	for _, child := range n.children {
		childWidget := child.widget
		r.release(child, detachWidgets)
		if detachWidgets && container != nil && childWidget != nil {
			container.RemoveChild(childWidget)
		}
	}
	n.children = nil

	if detachWidgets && container != nil {
		for _, child := range container.Children() {
			container.RemoveChild(child)
		}
	}

	if n.view != nil && n.widget != nil {
		ctx := &buildContext{root: r, node: n}
		n.view.Unmount(ctx, n.widget)
	}
	n.view = nil
	n.widget = nil
	n.state = nil
}

func (ctx *buildContext) State() any {
	return ctx.node.state
}

func (ctx *buildContext) SetState(state any) {
	ctx.node.state = state
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
			var oldWidget gui.Widget
			if oldNodes[index] != nil {
				oldWidget = oldNodes[index].widget
			}
			ctx.root.release(oldNodes[index], true)
			if oldWidget != nil {
				container.RemoveChild(oldWidget)
			}
			index++
			continue
		}
		newNodes = append(newNodes, child)
		index++
	}

	for _, old := range oldNodes[index:] {
		var oldWidget gui.Widget
		if old != nil {
			oldWidget = old.widget
		}
		ctx.root.release(old, true)
		if oldWidget != nil {
			container.RemoveChild(oldWidget)
		}
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
