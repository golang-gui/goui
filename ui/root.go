package ui

import (
	"reflect"
	"slices"
	"sync"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

type root struct {
	runtime       *app
	transferring  bool
	reconciling   bool
	mu            sync.Mutex
	root          *node
	window        gui.Window
	build         func() View
	updatePending bool
	destroyHandle signal.Handle
	afterUpdate   []afterUpdateCall
	transferAfter []afterUpdateCall
}

type afterUpdateCall struct {
	owner   *node
	version uint64
	fn      func()
}

type node struct {
	root     *root
	id       string // declaration identity, not the widget's mutable GUI ID
	viewType reflect.Type
	view     WidgetView
	widget   gui.Widget
	state    any
	children []*childTarget
	released bool
	version  uint64
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
	r.reconcile(view)
	r.flushAfterUpdate()
	return r.widget()
}

func (r *root) updateWindow(window gui.Window, view View) gui.Widget {
	r.reconcile(view)
	widget := r.widget()
	if window != nil {
		window.SetWidget(widget)
	}
	r.flushAfterUpdate()
	return widget
}

func (r *root) flushAfterUpdate() {
	if r.runtime != nil && r.runtime.reconcilingWindows {
		return
	}
	calls := r.afterUpdate
	r.afterUpdate = nil
	for _, call := range calls {
		if call.owner != nil && call.owner.root == r && !call.owner.released && call.owner.version == call.version && call.fn != nil {
			call.fn()
		}
	}
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
	r.afterUpdate = nil
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
	r.afterUpdate = nil
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
	if old != nil && !sameWidgetIdentity(old, view) {
		r.release(old, true)
		old = nil
	}

	current := old
	if current == nil {
		current = &node{
			root:     r,
			id:       view.base().id,
			viewType: viewType,
			view:     view,
			baseCtx:  &viewBaseContext{},
			version:  1,
		}
		ctx := &buildContext{root: r, node: current}
		current.widget = view.Mount(ctx)
		if current.widget == nil {
			r.release(current, true)
			return nil
		}
		view.base().mount(current.baseCtx, current.widget)
	}
	if old != nil {
		current.version++
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
	n.root = nil

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

func (ctx *buildContext) Coordinator() *Coordinator {
	return &Coordinator{owner: ctx.node}
}

func (r *root) reconcile(view View) {
	if r.transferring || r.reconciling {
		panic("ui: cannot nest reconciliation or reconcile during child transfer; request a later update")
	}
	r.reconciling = true
	defer func() { r.reconciling = false }()
	r.root = r.updateNode(r.root, view)
}

func (ctx *buildContext) SetState(state any) {
	ctx.node.state = state
}

func (ctx *buildContext) AfterUpdate(fn func()) {
	if fn != nil && !ctx.node.released {
		ctx.root.afterUpdate = append(ctx.root.afterUpdate, afterUpdateCall{
			owner: ctx.node, version: ctx.node.version, fn: fn,
		})
	}
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
	if len(current.nodes) != 0 && !sameWidgetIdentity(current.nodes[0], view) {
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
	if ctx.node.released || target == nil {
		return nil
	}
	// Normalize and reject duplicate local IDs before modifying the target.
	views := make([]WidgetView, len(children))
	ids := make(map[string]bool)
	for i, child := range children {
		views[i] = normalizeView(child)
		if views[i] != nil && views[i].base().id != "" {
			id := views[i].base().id
			if ids[id] {
				panic("ui: duplicate child ID " + id)
			}
			ids[id] = true
		}
	}
	current := ctx.childTarget(target, false)
	if current == nil {
		return nil
	}
	result := make([]gui.Widget, len(children))
	old := slices.Clone(current.nodes)
	byID := make(map[string]*node, len(old))
	for _, n := range old {
		if n.id != "" {
			byID[n.id] = n
		}
	}
	used := make(map[*node]bool, len(old))
	next := make([]*node, 0, len(children))
	position := 0
	for i, view := range views {
		if view == nil {
			continue
		}
		var previous *node
		if id := view.base().id; id != "" {
			previous = byID[id]
		} else if position < len(old) && old[position].id == "" {
			previous = old[position]
		}
		if !sameWidgetIdentity(previous, view) {
			previous = nil
		}
		mounted := ctx.root.updateWidgetNode(previous, view)
		if mounted == nil {
			continue
		}
		if previous == nil {
			target.AddChild(mounted.widget)
		} else {
			used[previous] = true
		}
		next = append(next, mounted)
		result[i] = mounted.widget
		position++
	}
	for _, n := range old {
		if !used[n] {
			widget := n.widget
			ctx.root.release(n, true)
			target.RemoveChild(widget)
		}
	}
	current.nodes = next
	// Relative moves preserve focus, mounts and window resources. Non-managed
	// children remain owned by the target; only the managed list is ordered here.
	var sibling gui.Widget
	for i := len(next) - 1; i >= 0; i-- {
		target.MoveChildBefore(next[i].widget, sibling)
		sibling = next[i].widget
	}
	ctx.forgetEmptyTarget(current)
	return result
}

// Dynamic targets such as recycled list rows must not accumulate empty records.
func (ctx *buildContext) forgetEmptyTarget(target *childTarget) {
	if len(target.nodes) == 0 {
		if target.container != nil && containerWidget(target.container) == ctx.node.widget {
			return
		}
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

func sameWidgetIdentity(old *node, view WidgetView) bool {
	return old != nil && !old.released && view != nil && old.viewType == reflect.TypeOf(view) && old.id == view.base().id
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
