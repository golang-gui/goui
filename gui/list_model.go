package gui

import (
	"sync"

	"github.com/golang-gui/goui/core/signal"
)

// SliceListModel is a generic, concurrency-safe ListModel backed by a plain
// slice. Every mutation emits a change notification; batch mutations go
// through Modify, which emits exactly once. Out-of-range indices panic with
// slice semantics.
type SliceListModel[T any] struct {
	mu      sync.RWMutex
	items   []T
	changed signal.Signal0
}

// NewSliceListModel returns a SliceListModel over the given items (the slice
// is copied; later SetItems replaces the contents).
func NewSliceListModel[T any](items []T) *SliceListModel[T] {
	return &SliceListModel[T]{items: append([]T(nil), items...)}
}

// ItemsCount implements ListModel.
func (m *SliceListModel[T]) ItemsCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

// ItemAt implements ListModel.
func (m *SliceListModel[T]) ItemAt(index int) any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.items[index]
}

// Modify applies f to the current items atomically and emits a single change
// notification. f runs with the model locked: it must not call back into the
// model; mutate the provided slice in place or return a new one. Use it for
// batch mutations (multiple inserts/removes/updates → one notification).
func (m *SliceListModel[T]) Modify(f func(prev []T) (after []T)) {
	m.mu.Lock()
	m.items = f(m.items)
	m.mu.Unlock()
	m.changed.Emit()
}

// Insert inserts item at index, shifting later items. It panics if index is
// out of range [0, ItemsCount()].
func (m *SliceListModel[T]) Insert(index int, item T) {
	m.mu.Lock()
	m.items = append(m.items, *new(T))
	copy(m.items[index+1:], m.items[index:])
	m.items[index] = item
	m.mu.Unlock()
	m.changed.Emit()
}

// Append appends item at the end.
func (m *SliceListModel[T]) Append(item T) {
	m.mu.Lock()
	m.items = append(m.items, item)
	m.mu.Unlock()
	m.changed.Emit()
}

// Remove deletes the item at index. It panics if index is out of range.
func (m *SliceListModel[T]) Remove(index int) {
	m.mu.Lock()
	m.items = append(m.items[:index], m.items[index+1:]...)
	m.mu.Unlock()
	m.changed.Emit()
}

// Set replaces the item at index. It panics if index is out of range.
func (m *SliceListModel[T]) Set(index int, item T) {
	m.mu.Lock()
	m.items[index] = item
	m.mu.Unlock()
	m.changed.Emit()
}

// SetItems replaces all items in one go.
func (m *SliceListModel[T]) SetItems(items []T) {
	m.mu.Lock()
	m.items = append([]T(nil), items...)
	m.mu.Unlock()
	m.changed.Emit()
}

// ConnectItems implements ListModel.
func (m *SliceListModel[T]) ConnectItems(f func()) signal.Handle {
	return m.changed.Connect(f)
}
