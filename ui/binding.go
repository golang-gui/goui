package ui

import (
	"reflect"
	"sync/atomic"

	"github.com/golang-gui/goui/core/signal"
)

// Observable publishes change notifications. Sources must be stable, non-nil
// pointers. Connect/Disconnect run on the UI thread; notifications may come
// from any goroutine and must not require synchronous UI work.
type Observable interface{ Connect(func()) signal.Handle }

// ReadState exposes a current value and change notifications. Get runs on the
// UI thread. Implementations own any synchronization with background writers.
type ReadState[T any] interface {
	Observable
	Get() T
}

// BindState is a readable value that accepts user edits.
type BindState[T any] interface {
	ReadState[T]
	Set(T)
}

type valueBinding struct {
	source  Observable
	apply   func()
	version uint64
}
type nodeSubscription struct {
	handle signal.Handle
	active atomic.Bool
	kind   atomic.Uint32
}

func validateObservable(source Observable) {
	if source == nil {
		panic("ui: observable must be a non-nil pointer")
	}
	v := reflect.ValueOf(source)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		panic("ui: observable must be a non-nil pointer")
	}
}
func (n *node) subscribe(source Observable, kind updateKind) {
	validateObservable(source)
	if n.subscriptions == nil {
		n.subscriptions = make(map[Observable]*nodeSubscription)
	}
	if sub := n.subscriptions[source]; sub != nil {
		// Retain every use during collection, including notifications emitted
		// by Get before finishBindings computes the final source set.
		sub.kind.Store(sub.kind.Load() | uint32(kind))
		return
	}
	sub := new(nodeSubscription)
	sub.active.Store(true)
	sub.kind.Store(uint32(kind))
	n.subscriptions[source] = sub
	sub.handle = source.Connect(func() {
		if sub.active.Load() {
			n.requestUpdate(updateKind(sub.kind.Load()))
		}
	})
	if sub.handle == nil {
		panic("ui: observable returned a nil connection")
	}
}
func (n *node) beginBindings() {
	n.collecting = true
	n.bindingOrder = n.bindingOrder[:0]
	n.observed = n.view.base().observables
	for _, source := range n.observed {
		n.subscribe(source, updateBuild)
	}
}
func (n *node) finishBindings() {
	n.collecting = false
	if n.released || len(n.subscriptions) == 0 {
		return
	}
	wanted := make(map[Observable]updateKind)
	for _, source := range n.observed {
		wanted[source] |= updateBuild
	}
	for key, b := range n.bindings {
		if b.version != n.version {
			delete(n.bindings, key)
		} else {
			wanted[b.source] |= updateBindings
		}
	}
	for source, sub := range n.subscriptions {
		if kind := wanted[source]; kind != 0 {
			sub.kind.Store(uint32(kind))
		} else {
			sub.active.Store(false)
			if sub.handle != nil {
				sub.handle.Disconnect()
			}
			delete(n.subscriptions, source)
		}
	}
}
func (n *node) releaseBindings() {
	for _, sub := range n.subscriptions {
		sub.active.Store(false)
		if sub.handle != nil {
			sub.handle.Disconnect()
		}
	}
	n.subscriptions = nil
	n.bindings = nil
	n.bindingOrder = nil
	n.observed = nil
}
func (n *node) applyBindings() {
	r := n.root
	for _, key := range n.bindingOrder {
		if n.released || n.root != r {
			return
		}
		if b, ok := n.bindings[key]; ok {
			b.apply()
		}
	}
}

// BindValue registers and immediately applies one property during a View's
// Build/Update. property is a stable name local to the View, not a widget ID.
// Undeclared bindings are removed after that update. Later notifications apply
// the current bindings without replaying the View's unbound declarations.
// apply is UI-thread-affine and must not write back into source.
func BindValue[T any](ctx BuildContext, property string, source ReadState[T], apply func(T)) {
	if property == "" || apply == nil {
		panic("ui: binding requires a property name and apply function")
	}
	validateObservable(source)
	c := ctx.Coordinator()
	if c == nil || c.owner == nil || c.owner.released {
		return
	}
	n := c.owner
	if !n.collecting {
		panic("ui: BindValue requires an active View update")
	}
	n.subscribe(source, updateBindings)
	if n.bindings == nil {
		n.bindings = make(map[string]valueBinding)
	}
	if old, ok := n.bindings[property]; !ok || old.version != n.version {
		n.bindingOrder = append(n.bindingOrder, property)
	}
	fn := func() { apply(source.Get()) }
	n.bindings[property] = valueBinding{source: source, apply: fn, version: n.version}
	fn()
}
