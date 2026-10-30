package ui

import (
	"reflect"
	"sync/atomic"

	"github.com/golang-gui/goui/core/signal"
	"github.com/xuges/gothread"
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

// Read reads a source and tracks it during a ComposeView.Build. The source
// must be a stable non-nil pointer, and must report every change to its value.
// Reads inside the source's getter are not tracked separately. Outside a build,
// Read only reads the value; the source retains its own threading contract.
func Read[T any](source ReadState[T]) T {
	validateObservable(source)
	trackRead(source)
	defer pauseReadTracking()()
	return source.Get()
}

// Only the UI thread mutates a capture. Background readers inspect its
// immutable thread ID before accessing any node or dependency collection.
type readCapture struct {
	owner    *node
	root     *root
	version  uint64
	epoch    uint64
	threadID int
	sources  map[Observable]struct{}
}

var activeReads atomic.Pointer[readCapture]

func (c *readCapture) live() bool {
	return !c.owner.released && c.owner.root == c.root && c.owner.version == c.version && c.root.epoch == c.epoch
}

func trackRead(source Observable) {
	c := activeReads.Load()
	if c == nil || c.threadID != gothread.GetId() || !c.live() {
		return
	}
	if _, seen := c.sources[source]; seen {
		return
	}
	if c.sources == nil {
		c.sources = make(map[Observable]struct{})
	}
	c.sources[source] = struct{}{}
	// Connect before reading, so a concurrent writer either changes the value
	// we read or requests another update. Subscription setup is untracked.
	c.owner.subscribe(source, updateBuild)
}

// A worker must not suspend the UI thread's active capture. The no-capture
// path also avoids querying the OS thread for ordinary state reads/writes.
func pauseReadTracking() func() {
	c := activeReads.Load()
	if c == nil || c.threadID != gothread.GetId() {
		return func() {}
	}
	activeReads.Store(nil)
	return func() { activeReads.Store(c) }
}

func (n *node) buildComposition(view ComposeView, ctx BuildContext) View {
	c := &readCapture{owner: n, root: n.root, version: n.version, epoch: n.root.epoch, threadID: gothread.GetId()}
	previous := activeReads.Swap(c)
	defer activeReads.Store(previous)
	content := view.Build(ctx)
	if c.live() {
		// Commit only after a successful return. A panic retains the previous
		// set; finishBindings then discards newly connected, unused sources.
		n.tracked = c.sources
	}
	return content
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
	defer pauseReadTracking()()
	sub.handle = source.Connect(func() {
		if sub.active.Load() {
			n.requestUpdate(updateKind(sub.kind.Load()))
		}
	})
	if sub.handle == nil {
		panic("ui: observable returned a nil connection")
	}
	// Connect is external code and can release the owner before returning.
	if n.released || n.subscriptions[source] != sub {
		sub.active.Store(false)
		sub.handle.Disconnect()
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
	for source := range n.tracked {
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
			delete(n.subscriptions, source)
			sub.active.Store(false)
			if sub.handle != nil {
				sub.handle.Disconnect()
			}
			if n.released {
				return
			}
		}
	}
}
func (n *node) releaseBindings() {
	subscriptions := n.subscriptions
	n.subscriptions = nil
	n.tracked = nil
	n.bindings = nil
	n.bindingOrder = nil
	n.observed = nil
	for _, sub := range subscriptions {
		sub.active.Store(false)
		if sub.handle != nil {
			sub.handle.Disconnect()
		}
	}
}
func (n *node) applyBindings() {
	defer pauseReadTracking()()
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
	fn := func() {
		defer pauseReadTracking()()
		apply(source.Get())
	}
	n.bindings[property] = valueBinding{source: source, apply: fn, version: n.version}
	fn()
}
