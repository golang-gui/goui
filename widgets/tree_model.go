package widgets

import (
	"slices"

	"github.com/golang-gui/goui/core/signal"
)

// TreeModel describes an ordered, acyclic tree on the GUI thread. Empty ID is
// the virtual root. IDs are stable and unique within this model, not Widget IDs.
// Queries must be synchronous, read-only memory access. Items is emitted only
// after a complete mutation. Children returned by a model are read-only.
type TreeModel interface {
	Children(parentID string) []string
	Parent(id string) (parentID string, exists bool)
	Expandable(id string) bool
	ConnectItems(func()) signal.Handle
}

type TreeData[T any] interface {
	TreeModel
	Item(id string) T
}

// TreeChildrenRequester optionally starts an idempotent, nonblocking load.
// The model owns loading, cancellation, errors and stale-response validation.
// Results must be committed and notified on the GUI thread.
type TreeChildrenRequester interface{ RequestChildren(id string) }

type treeRecord[T any] struct {
	parent     string
	children   []string
	value      T
	expandable bool
}

// TreeStore is an in-memory TreeData. It is GUI-thread-affine, not concurrent.
// Explicitly expandable empty branches support lazy loading. Remove recursively
// deletes a subtree; Modify batches notifications, without rollback.
type TreeStore[T any] struct {
	nodes   map[string]*treeRecord[T]
	roots   []string
	changed signal.Signal0
	depth   int
	dirty   bool
}

func NewTreeStore[T any]() *TreeStore[T] {
	return &TreeStore[T]{nodes: make(map[string]*treeRecord[T])}
}
func (m *TreeStore[T]) Children(parentID string) []string {
	if parentID == "" {
		return slices.Clone(m.roots)
	}
	if n := m.nodes[parentID]; n != nil {
		return slices.Clone(n.children)
	}
	return nil
}
func (m *TreeStore[T]) Parent(id string) (string, bool) {
	if n := m.nodes[id]; n != nil {
		return n.parent, true
	}
	return "", false
}
func (m *TreeStore[T]) Expandable(id string) bool {
	n := m.nodes[id]
	return n != nil && (n.expandable || len(n.children) != 0)
}
func (m *TreeStore[T]) Item(id string) T {
	if n := m.nodes[id]; n != nil {
		return n.value
	}
	panic("widgets: unknown tree node " + id)
}
func (m *TreeStore[T]) ConnectItems(fn func()) signal.Handle { return m.changed.Connect(fn) }

func (m *TreeStore[T]) Insert(parentID string, index int, id string, value T) {
	if id == "" || m.nodes[id] != nil {
		panic("widgets: empty or duplicate tree node ID")
	}
	children := &m.roots
	if parentID != "" {
		p := m.nodes[parentID]
		if p == nil {
			panic("widgets: unknown tree parent")
		}
		children = &p.children
	}
	if index < 0 || index > len(*children) {
		panic("widgets: invalid tree insertion index")
	}
	if m.nodes == nil {
		m.nodes = make(map[string]*treeRecord[T])
	}
	m.nodes[id] = &treeRecord[T]{parent: parentID, value: value}
	*children = slices.Insert(*children, index, id)
	m.notify()
}
func (m *TreeStore[T]) Append(parentID, id string, value T) {
	index := len(m.roots)
	if parentID != "" {
		p := m.nodes[parentID]
		if p == nil {
			panic("widgets: unknown tree parent")
		}
		index = len(p.children)
	}
	m.Insert(parentID, index, id, value)
}
func (m *TreeStore[T]) Set(id string, value T) {
	n := m.nodes[id]
	if n == nil {
		panic("widgets: unknown tree node")
	}
	n.value = value
	m.notify()
}
func (m *TreeStore[T]) Remove(id string) {
	n := m.nodes[id]
	if n == nil {
		return
	}
	children := &m.roots
	if n.parent != "" {
		children = &m.nodes[n.parent].children
	}
	*children = slices.Delete(*children, slices.Index(*children, id), slices.Index(*children, id)+1)
	pending := []string{id}
	for len(pending) != 0 {
		next := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		pending = append(pending, m.nodes[next].children...)
		delete(m.nodes, next)
	}
	m.notify()
}
func (m *TreeStore[T]) SetExpandable(id string, expandable bool) {
	n := m.nodes[id]
	if n == nil {
		panic("widgets: unknown tree node")
	}
	if n.expandable == expandable {
		return
	}
	n.expandable = expandable
	m.notify()
}
func (m *TreeStore[T]) Modify(fn func(*TreeStore[T])) {
	m.depth++
	defer func() {
		m.depth--
		if m.depth == 0 && m.dirty {
			m.dirty = false
			m.changed.Emit()
		}
	}()
	fn(m)
}
func (m *TreeStore[T]) notify() {
	if m.depth != 0 {
		m.dirty = true
		return
	}
	m.changed.Emit()
}
