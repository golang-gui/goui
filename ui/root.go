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
	afterPending  bool
	epoch         uint64
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
	view     View
	widget   gui.Widget // own WidgetView widget, or the composition's output
	state    any
	parent   *node
	body     *node
	children []*childTarget
	released bool
	version  uint64
	baseCtx  *viewBaseContext // persistent cross-cutting signal context, reused across rebuilds
}

type buildContext struct {
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
	epoch := r.epoch
	r.reconcile(view)
	widget := r.widget()
	if window != nil && r.epoch == epoch {
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
	r.epoch++
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
	r.epoch++
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
	return r.updateDeclaration(old, view, nil, 0)
}

func (r *root) updateDeclaration(old *node, view View, parent *node, depth int) *node {
	validateView(view)
	epoch := r.epoch
	if view == nil {
		r.release(old, true)
		return nil
	}
	if depth >= 64 {
		panic("ui: composition depth exceeded")
	}
	if old != nil && !sameViewIdentity(old, view) {
		r.release(old, true)
		old = nil
		if r.epoch != epoch || parent != nil && parent.released {
			return nil
		}
	}
	current := old
	if current == nil {
		current = &node{
			root:     r,
			id:       view.base().id,
			viewType: reflect.TypeOf(view),
			view:     view,
			parent:   parent,
			version:  1,
		}
	} else {
		current.version++
		current.view = view
		current.parent = parent
	}
	completed := false
	if old == nil {
		// A fresh node is not yet owned by its parent's slot. Release its
		// partial state if construction panics before it can be installed.
		defer func() {
			if !completed {
				r.release(current, true)
			}
		}()
	}
	ctx := &buildContext{node: current}
	live := func() bool {
		return r.epoch == epoch && !current.released && current.root == r && (parent == nil || !parent.released)
	}
	if widgetView, ok := view.(WidgetView); ok {
		if old == nil {
			current.widget = widgetView.Mount(ctx)
			if !live() || nilWidget(current.widget) {
				r.release(current, true)
				return nil
			}
			current.baseCtx = &viewBaseContext{}
			view.base().mount(current.baseCtx, current.widget)
		}
		widgetView.Update(ctx, current.widget)
		if live() {
			view.base().update(current.baseCtx, current.widget)
		}
	} else {
		content := view.(ComposeView).Build(ctx)
		if live() {
			body := r.updateDeclaration(current.body, content, current, depth+1)
			if !live() {
				r.release(body, true)
			} else {
				current.body = body
				current.widget = nil
				if body != nil {
					current.widget = body.widget
				}
			}
		}
	}
	if !live() {
		r.release(current, true)
		return nil
	}
	completed = true
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
	r.release(n.body, detachWidgets)
	n.body = nil

	ctx := &buildContext{node: n}
	if view, ok := n.view.(WidgetView); ok && !nilWidget(n.widget) {
		view.Unmount(ctx, n.widget)
	} else if view, ok := n.view.(ObjectView); ok {
		view.Release(ctx)
	}
	n.view = nil
	n.widget = nil
	n.state = nil
	n.parent = nil
}

func (ctx *buildContext) State() any {
	return ctx.node.state
}

// A retained context borrows the node, never the Root at its creation. Each
// delayed row build resolves ownership again after explicit transfers.
func (ctx *buildContext) beginBuild() (*root, func()) {
	r := ctx.node.root
	if ctx.node.released || r == nil {
		return nil, func() {}
	}
	if r.transferring {
		panic("ui: cannot build children during transfer")
	}
	if r.reconciling {
		return r, func() {}
	}
	r.reconciling = true
	return r, func() {
		r.reconciling = false
		r.scheduleAfterUpdate()
	}
}

func (r *root) scheduleAfterUpdate() {
	if r.afterPending || len(r.afterUpdate) == 0 || gui.App == nil {
		return
	}
	r.afterPending = true
	gui.App.Post(func() {
		r.afterPending = false
		if r.reconciling || r.transferring {
			r.scheduleAfterUpdate()
			return
		}
		r.flushAfterUpdate()
	})
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
	epoch := r.epoch
	current := r.updateNode(r.root, view)
	if r.epoch == epoch {
		r.root = current
	} else {
		r.release(current, true)
	}
}

func (ctx *buildContext) SetState(state any) {
	if !ctx.node.released {
		ctx.node.state = state
	}
}

func (ctx *buildContext) AfterUpdate(fn func()) {
	if fn != nil && !ctx.node.released && ctx.node.root != nil {
		ctx.node.root.afterUpdate = append(ctx.node.root.afterUpdate, afterUpdateCall{
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
	r, finish := ctx.beginBuild()
	defer finish()
	if r == nil {
		return nil
	}
	current := ctx.childTarget(target, true)
	if current == nil {
		return nil
	}
	validateView(child)
	epoch := r.epoch
	var previous *node
	var oldWidget gui.Widget
	if len(current.nodes) != 0 {
		previous, oldWidget = current.nodes[0], current.nodes[0].widget
	}
	mounted := r.updateDeclaration(previous, child, ctx.node, 0)
	if !ctx.live(r, epoch) {
		r.release(mounted, true)
		return nil
	}
	var widget gui.Widget
	current.nodes = nil
	if mounted != nil {
		current.nodes = []*node{mounted}
		widget = mounted.widget
	}
	if oldWidget != widget {
		target.SetChild(widget)
		if !ctx.live(r, epoch) {
			return nil
		}
	}
	if len(current.nodes) == 0 {
		ctx.forgetEmptyTarget(current)
		return nil
	}
	return widget
}

func (ctx *buildContext) UpdateChildren(target Container, children []View) []gui.Widget {
	r, finish := ctx.beginBuild()
	defer finish()
	if r == nil {
		return nil
	}
	if ctx.node.released || target == nil {
		return nil
	}
	// Identity belongs to the declaration, including non-widget compositions.
	ids := make(map[string]bool)
	for _, child := range children {
		validateView(child)
		if child != nil && child.base().id != "" {
			id := child.base().id
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
	oldWidgets := make(map[*node]gui.Widget, len(old))
	byID := make(map[string]*node, len(old))
	for _, n := range old {
		oldWidgets[n] = n.widget
		if n.id != "" {
			byID[n.id] = n
		}
	}
	used := make(map[*node]bool, len(old))
	next := make([]*node, 0, len(children))
	epoch := r.epoch
	cleanup := func() {
		for _, n := range next {
			if !used[n] {
				r.release(n, true)
			}
		}
	}
	installed := false
	defer func() {
		if !installed {
			cleanup()
		}
	}()
	position := 0
	for i, view := range children {
		if view == nil {
			continue
		}
		var previous *node
		if id := view.base().id; id != "" {
			previous = byID[id]
		} else if position < len(old) && old[position].id == "" {
			previous = old[position]
		}
		if !sameViewIdentity(previous, view) {
			previous = nil
		}
		mounted := r.updateDeclaration(previous, view, ctx.node, 0)
		if !ctx.live(r, epoch) {
			r.release(mounted, true)
			cleanup()
			return nil
		}
		if mounted == nil {
			continue
		}
		if previous != nil {
			used[previous] = true
		}
		next = append(next, mounted)
		result[i] = mounted.widget
		position++
	}
	for _, n := range old {
		if !used[n] {
			r.release(n, true)
			if !ctx.live(r, epoch) {
				cleanup()
				return nil
			}
		}
	}
	current.nodes = next
	installed = true
	for _, n := range old {
		widget := oldWidgets[n]
		if widget != nil && (!used[n] || widget != n.widget) {
			target.RemoveChild(widget)
			if !ctx.live(r, epoch) {
				return nil
			}
		}
	}
	for _, n := range next {
		if n.widget != nil && (!used[n] || oldWidgets[n] != n.widget) {
			target.AddChild(n.widget)
			if !ctx.live(r, epoch) {
				return nil
			}
		}
	}
	// Relative moves preserve focus, mounts and window resources. Non-managed
	// children remain owned by the target; only the managed list is ordered here.
	var sibling gui.Widget
	for i := len(next) - 1; i >= 0; i-- {
		if widget := next[i].widget; widget != nil {
			target.MoveChildBefore(widget, sibling)
			if !ctx.live(r, epoch) {
				return nil
			}
			sibling = widget
		}
	}
	ctx.forgetEmptyTarget(current)
	return result
}

func (ctx *buildContext) live(r *root, epoch uint64) bool {
	return !ctx.node.released && ctx.node.root == r && r.epoch == epoch
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
		if detach && widget != nil {
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

func sameViewIdentity(old *node, view View) bool {
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

func validateView(view View) {
	if view == nil {
		return
	}
	_, widget := view.(WidgetView)
	_, compose := view.(ComposeView)
	if widget == compose {
		panic("ui: a declaration must implement exactly one of WidgetView or ComposeView")
	}
	if compose {
		base := view.base()
		if base.fields.Mask&^uint64(1<<viewID) != 0 || len(base.shortcuts) != 0 || base.dragSource != nil || base.dropTarget != nil || base.onContextMenu != nil {
			panic("ui: widget modifiers require a WidgetView; apply them to the composition's content")
		}
	}
}
