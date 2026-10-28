package ui

import (
	"sort"
	"sync/atomic"

	"github.com/golang-gui/goui/gui"
)

type updateKind uint32

const (
	updateBindings updateKind = 1 << iota
	updateBuild
)

type nodeUpdate struct {
	node *node
	kind updateKind
}

// requestNode only touches immutable identity and the application's queue.
// The current root, declaration and widget are resolved on the UI thread.
func (a *app) requestNode(n *node, kind updateKind) {
	a.mu.Lock()
	if !a.stopping {
		if a.dirty == nil {
			a.dirty = make(map[*node]updateKind)
		}
		if _, exists := a.dirty[n]; !exists {
			a.dirtyOrder = append(a.dirtyOrder, n)
		}
		a.dirty[n] |= kind
	}
	a.mu.Unlock()
	a.scheduleUpdate()
}
func (a *app) scheduleUpdate() {
	a.mu.Lock()
	if a.stopping || a.updatePending || a.updateDepth != 0 || (!a.fullUpdate && len(a.dirty) == 0) {
		a.mu.Unlock()
		return
	}
	a.updatePending = true
	a.mu.Unlock()
	if !a.postTask(a.runPendingUpdate) {
		a.mu.Lock()
		a.updatePending = false
		a.mu.Unlock()
	}
}
func (a *app) holdUpdates() func() {
	a.mu.Lock()
	a.updateDepth++
	a.mu.Unlock()
	return func() {
		a.mu.Lock()
		a.updateDepth--
		a.mu.Unlock()
		a.scheduleUpdate()
	}
}
func (n *node) requestUpdate(kind updateKind) {
	if n.alive.Load() && n.runtime != nil {
		n.runtime.requestNode(n, kind)
	}
}
func (ctx *buildContext) RequestUpdate() { ctx.node.requestUpdate(updateBuild) }

func (a *app) processNodeUpdates(jobs []nodeUpdate) []*root {
	depth := func(n *node) int {
		d := 0
		for n.parent != nil {
			n = n.parent
			d++
		}
		return d
	}
	sort.SliceStable(jobs, func(i, j int) bool { return depth(jobs[i].node) < depth(jobs[j].node) })
	var roots []*root
	for _, job := range jobs {
		n := job.node
		r := n.root
		if n.released || r == nil || r.transferring {
			continue
		}
		found := false
		for _, other := range roots {
			if other == r {
				found = true
				break
			}
		}
		if !found {
			roots = append(roots, r)
		}
		if n.builtBatch == a.batch {
			continue
		}
		if job.kind&updateBuild != 0 {
			r.updateLocal(n)
		} else if n.boundBatch != a.batch {
			n.applyBindings()
			n.boundBatch = a.batch
		}
	}
	return roots
}

// Subscription callbacks may race with release, so liveness is independent of
// the UI-thread-only node state.
type updateLife struct{ alive atomic.Bool }

func (r *root) updateLocal(n *node) {
	if r.reconciling || r.transferring || n.released {
		return
	}
	r.reconciling = true
	defer func() { r.reconciling = false }()
	epoch := r.epoch
	old := n.widget
	depth := 0
	for child, p := n, n.parent; p != nil && p.body == child; child, p = p, p.parent {
		depth++
	}
	current := r.updateDeclaration(n, n.view, n.parent, depth)
	if current == nil || n.released || n.root != r || r.epoch != epoch {
		return
	}
	// Compositions forward their body's output without building their parents.
	output := n
	for output.parent != nil && output.parent.body == output {
		output.parent.widget = output.widget
		output = output.parent
	}
	if old == output.widget {
		return
	}
	live := func() bool { return !output.released && output.root == r && r.epoch == epoch }
	if output.target == nil {
		if r.root == output && r.window != nil {
			r.window.SetWidget(output.widget)
		}
	} else if target := output.target; target.bin != nil {
		target.bin.SetChild(output.widget)
	} else {
		if old != nil {
			target.container.RemoveChild(old)
		}
		if !live() {
			return
		}
		if output.widget != nil {
			target.container.AddChild(output.widget)
		}
		if !live() {
			return
		}
		var sibling gui.Widget
		for i := len(target.nodes) - 1; i >= 0; i-- {
			if widget := target.nodes[i].widget; widget != nil {
				target.container.MoveChildBefore(widget, sibling)
				if !live() {
					return
				}
				sibling = widget
			}
		}
	}
	if live() && output.slotApply != nil {
		output.slotApply(output.widget)
	}
}
