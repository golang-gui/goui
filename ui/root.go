package ui

import (
	"reflect"
	"slices"
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
	view     WidgetView
	widget   gui.Widget
	state    any
	children []*childTarget
	released bool
	baseCtx  *viewBaseContext // persistent cross-cutting signal context, reused across rebuilds
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
	widgetView := normalizeView(view)
	return r.updateWidgetNode(old, widgetView)
}

func (r *root) updateWidgetNode(old *node, view WidgetView) *node {
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
			baseCtx:  &viewBaseContext{},
		}
		ctx := &buildContext{root: r, node: current}
		current.widget = view.Mount(ctx)
		if current.widget == nil {
			r.release(current, true)
			return nil
		}
		view.base().mount(current.baseCtx, current.widget)
	}

	ctx := &buildContext{root: r, node: current}
	current.viewType = viewType
	current.view = view
	view.Update(ctx, current.widget)
	view.base().update(current.baseCtx, current.widget) // shared id/visibility/style + base signals, framework-driven
	return current
}

func (r *root) release(n *node, detachWidgets bool) {
	if n == nil || n.released {
		return
	}
	n.released = true

	if n.view != nil && n.widget != nil && n.baseCtx != nil {
		n.view.base().unmount(n.baseCtx, n.widget)
	}

	for _, target := range n.children {
		r.releaseChildren(target, 0, detachWidgets)
	}
	n.children = nil

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

// childTarget records only UI-owned nodes for one mounting target. Keeping
// targets in registration order makes teardown deterministic.
type childTarget struct {
	target    any // stable pointer implementing bin or container, never both modes
	bin       Bin
	container Container
	nodes     []*node
}

func (ctx *buildContext) childTarget(target any, single bool) *childTarget {
	if ctx.node.released || target == nil {
		return nil
	}
	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		panic("ui: child mounting target must be a non-nil stable pointer")
	}
	for _, current := range ctx.node.children {
		if current.target == target {
			if (current.bin != nil) != single {
				panic("ui: same target used as both Bin and Container")
			}
			return current
		}
	}
	current := &childTarget{target: target}
	if single {
		current.bin = target.(Bin)
	} else {
		current.container = target.(Container)
	}
	ctx.node.children = append(ctx.node.children, current)
	return current
}

func (ctx *buildContext) UpdateChild(target Bin, child View) gui.Widget {
	current := ctx.childTarget(target, true)
	if current == nil {
		return nil
	}
	view := normalizeView(child)
	if len(current.nodes) != 0 && !sameWidgetViewType(current.nodes[0], view) {
		ctx.root.releaseChildren(current, 0, true)
	}
	if view != nil {
		if len(current.nodes) != 0 {
			ctx.root.updateWidgetNode(current.nodes[0], view)
		} else if mounted := ctx.root.updateWidgetNode(nil, view); mounted != nil {
			current.nodes = []*node{mounted}
			target.SetChild(mounted.widget)
		}
	}
	if len(current.nodes) == 0 {
		ctx.forgetEmptyTarget(current)
		return nil
	}
	return current.nodes[0].widget
}

func (ctx *buildContext) UpdateChildren(target Container, children []View) []gui.Widget {
	current := ctx.childTarget(target, false)
	if current == nil {
		return nil
	}
	result := make([]gui.Widget, len(children))
	index := 0
	for i, child := range children {
		view := normalizeView(child)
		if view == nil {
			continue
		}
		// Without an insertion API, only the same-type prefix can be reused.
		if index < len(current.nodes) && !sameWidgetViewType(current.nodes[index], view) {
			ctx.root.releaseChildren(current, index, true)
		}
		var mounted *node
		if index < len(current.nodes) {
			mounted = ctx.root.updateWidgetNode(current.nodes[index], view)
		} else {
			mounted = ctx.root.updateWidgetNode(nil, view)
			if mounted == nil {
				continue
			}
			current.nodes = append(current.nodes, mounted)
			target.AddChild(mounted.widget)
		}
		result[i] = mounted.widget
		index++
	}
	ctx.root.releaseChildren(current, index, true)
	ctx.forgetEmptyTarget(current)
	return result
}

// Dynamic targets such as recycled list rows must not accumulate empty records.
func (ctx *buildContext) forgetEmptyTarget(target *childTarget) {
	if len(target.nodes) == 0 {
		if index := slices.Index(ctx.node.children, target); index >= 0 {
			ctx.node.children = slices.Delete(ctx.node.children, index, index+1)
		}
	}
}

func (r *root) releaseChildren(target *childTarget, from int, detach bool) {
	for _, child := range target.nodes[from:] {
		widget := child.widget
		r.release(child, detach)
		if detach {
			if target.bin != nil {
				target.bin.SetChild(nil)
			} else {
				target.container.RemoveChild(widget)
			}
		}
	}
	clear(target.nodes[from:])
	target.nodes = target.nodes[:from]
}

func sameWidgetViewType(old *node, view WidgetView) bool {
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

func normalizeView(view View) WidgetView {
	for depth := 0; view != nil; depth++ {
		if widgetView, ok := view.(WidgetView); ok {
			return widgetView
		}
		if depth >= 64 {
			panic("ui: view build depth exceeded")
		}
		view = view.Build()
	}
	return nil
}
