package ui

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/golang-gui/goui/gui"
)

// BindTarget declares a persistent reference to a managed Widget output in the
// current UI Root. T is a concrete Widget pointer or a capability interface.
// property is a stable name local to the View. Call during Build/Update; resolution
// runs after the batch is mounted, before AfterUpdate, without rebuilding users.
// Missing or empty IDs supply the zero value. Duplicate or incompatible targets
// panic. apply owns its business connections and must release them on a zero
// value; removing the declaration or releasing its owner clears a resolved target.
// Owner updates apply the latest callback; unrelated updates apply only changed
// targets. Callbacks run on the GUI thread and must not start reconciliation.
func BindTarget[T any](ctx BuildContext, property, id string, apply func(T)) {
	if property == "" || apply == nil {
		panic("ui: target binding requires a property name and apply function")
	}
	c := ctx.Coordinator()
	if c == nil || c.owner == nil || c.owner.released || c.owner.root == nil {
		return
	}
	n := c.owner
	if !n.collecting {
		panic("ui: BindTarget requires an active View update")
	}
	if n.targetBindings == nil {
		n.targetBindings = make(map[string]*targetBinding)
	}
	b := n.targetBindings[property]
	if b == nil {
		b = &targetBinding{owner: n, property: property}
		n.targetBindings[property] = b
		n.targetOrder = append(n.targetOrder, property)
		b.rehome(n.root)
	}
	b.revision++
	b.id, b.version, b.pending = id, n.version, true
	b.expected = reflect.TypeFor[T]()
	b.accepts = func(w gui.Widget) bool { _, ok := w.(T); return ok }
	b.apply = func(w gui.Widget) {
		defer pauseReadTracking()()
		var value T
		if w != nil {
			value = w.(T)
		}
		apply(value)
	}
	n.root.targetsDirty = true
}

type targetBinding struct {
	owner, targetOwner *node
	registry           *root
	property, id       string
	version, revision  uint64
	expected           reflect.Type
	accepts            func(gui.Widget) bool
	apply              func(gui.Widget)
	target             gui.Widget
	applied, pending   bool
}

func (b *targetBinding) rehome(r *root) {
	if old := b.registry; old != r {
		if old != nil {
			if i := slices.Index(old.targets, b); i >= 0 {
				old.targets = slices.Delete(old.targets, i, i+1)
			}
			old.targetsDirty = true
		}
		b.registry = r
		if r != nil {
			r.targets = append(r.targets, b)
			r.targetsDirty = true
		}
	}
}

func (b *targetBinding) clear() {
	b.revision++
	apply, resolved := b.apply, b.target != nil
	b.rehome(nil)
	b.owner, b.targetOwner, b.target = nil, nil, nil
	b.apply, b.accepts = nil, nil
	if resolved && apply != nil {
		apply(nil)
	}
}

func (b *targetBinding) invalidate() {
	if b.target == nil {
		return
	}
	b.revision++
	b.target, b.targetOwner = nil, nil
	b.applied, b.pending = true, false
	if b.registry != nil {
		b.registry.targetsDirty = true
	}
	apply := b.apply
	if apply != nil {
		apply(nil)
	}
}

func (n *node) finishTargets() {
	for _, property := range slices.Clone(n.targetOrder) {
		if b := n.targetBindings[property]; b != nil && b.version != n.version {
			delete(n.targetBindings, property)
			n.targetOrder = slices.DeleteFunc(n.targetOrder, func(s string) bool { return s == property })
			b.clear()
		}
	}
}

func (n *node) releaseTargets() {
	bindings, order := n.targetBindings, n.targetOrder
	n.targetBindings, n.targetOrder = nil, nil
	for _, property := range order {
		bindings[property].clear()
	}
}

// A target can leave a managed slot without being destroyed. Stop its business
// connections before running its Unmount callback or detaching its GUI output.
func (r *root) invalidateTarget(n *node) {
	for _, b := range slices.Clone(r.targets) {
		if b.targetOwner == n {
			b.invalidate()
		}
	}
}

type targetOutput struct {
	owner     *node
	widget    gui.Widget
	duplicate bool
}

type targetResolution struct {
	binding  *targetBinding
	revision uint64
	output   targetOutput
}

func (r *root) flushTargets() {
	if !r.targetsDirty || r.transferring || r.reconciling || r.committingTargets || r.runtime != nil && r.runtime.reconcilingHosts {
		return
	}
	r.targetsDirty = false
	if len(r.targets) == 0 {
		return
	}
	r.committingTargets = true
	defer func() { r.committingTargets = false }()
	epoch := r.epoch
	bindings := slices.Clone(r.targets)
	outputs := make(map[string]targetOutput)
	for _, b := range bindings {
		if b.id != "" {
			outputs[b.id] = targetOutput{}
		}
	}
	var visit func(*node)
	visit = func(n *node) {
		if n == nil || n.released || n.root != r {
			return
		}
		if _, own := n.view.(WidgetView); own && !nilWidget(n.widget) && !destroyedWidget(n.widget) {
			if output, wanted := outputs[n.widget.ID()]; wanted {
				if output.widget != nil && output.widget != n.widget {
					output.duplicate = true
				} else {
					output.owner, output.widget = n, n.widget
				}
				outputs[n.widget.ID()] = output
			}
		}
		visit(n.body)
		for _, slot := range n.children {
			for _, child := range slot.nodes {
				visit(child)
			}
		}
	}
	visit(r.root)
	resolutions := make([]targetResolution, 0, len(bindings))
	// Validate the final batch before invoking any new binding callbacks.
	for _, b := range bindings {
		if b.owner == nil || b.owner.released || b.owner.root != r || b.registry != r {
			continue
		}
		output := outputs[b.id]
		if output.duplicate {
			panic(fmt.Sprintf("ui: target %q for %T(%q).%s has duplicate Widget outputs", b.id, b.owner.view, b.owner.id, b.property))
		}
		if output.widget != nil && !b.accepts(output.widget) {
			panic(fmt.Sprintf("ui: target %q for %T(%q).%s is %T, want %v", b.id, b.owner.view, b.owner.id, b.property, output.widget, b.expected))
		}
		resolutions = append(resolutions, targetResolution{b, b.revision, output})
	}
	for _, next := range resolutions {
		b, output := next.binding, next.output
		if r.epoch != epoch {
			return
		}
		if b.owner == nil || b.owner.released || b.owner.root != r || b.registry != r || b.revision != next.revision {
			continue
		}
		if output.owner != nil && (output.owner.released || output.owner.root != r || destroyedWidget(output.widget)) {
			b.invalidate()
			continue
		}
		if b.applied && !b.pending && b.target == output.widget {
			continue
		}
		b.target, b.targetOwner = output.widget, output.owner
		b.applied, b.pending = true, false
		b.apply(output.widget)
	}
}
