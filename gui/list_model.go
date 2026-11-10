package gui

import (
	"sync"

	"github.com/golang-gui/goui/core/signal"
)

// ListModel is the structural layer of a list view: item count and change
// notifications. It knows nothing about widgets and nothing about item data —
// data access lives in ListData[T], so a ListView can drive any model without
// knowing what the items are.
type ListModel interface {
	// ItemsCount returns the total number of items.
	ItemsCount() int
	// ConnectItems subscribes to any model change. Handlers run synchronously
	// on mutation; disconnect via the returned handle.
	ConnectItems(f func()) signal.Handle
}

// ListChange replaces Removed items at Position with Added items. Positions
// refer to the list before this change; retained later items shift accordingly.
// Reset discards the range and requests a full reload. A zero-length change
// refreshes data without replacing item identities.
type ListChange struct {
	Position, Removed, Added int
	Reset                    bool
}

// ListModelChanges optionally describes precise changes to a ListModel.
// ListView subscribes to this signal instead of ConnectItems. Emit after the
// mutation on the GUI thread. Coalesced or stale notifications must use Reset
// when they cannot describe one change from the observer's preceding state.
type ListModelChanges interface {
	ListModel
	ConnectItemsChanged(func(ListChange)) signal.Handle
}

// ListData is the data-access layer of a list: a ListModel plus typed item
// access. Delegates and declarative wrappers consume it to fetch the item at
// an index with compile-time type safety (ListData[int] cannot be fed a
// model of strings).
type ListData[T any] interface {
	ListModel
	// ItemAt returns the data of the item at index.
	ItemAt(index int) T
}

// SliceListModel is a generic, concurrency-safe ListData backed by a plain
// slice. Every mutation emits a change notification; batch mutations go
// through Modify, which emits exactly once. Out-of-range indices panic with
// slice semantics.
type SliceListModel[T any] struct {
	mu       sync.RWMutex
	items    []T
	revision uint64
	changed  signal.Signal2[ListChange, uint64]
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

// ItemAt implements ListData.
func (m *SliceListModel[T]) ItemAt(index int) T {
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
	m.revision++
	revision := m.revision
	m.mu.Unlock()
	m.changed.Emit(ListChange{Reset: true}, revision)
}

// Insert inserts item at index, shifting later items. It panics if index is
// out of range [0, ItemsCount()].
func (m *SliceListModel[T]) Insert(index int, item T) {
	m.mu.Lock()
	m.items = append(m.items, *new(T))
	copy(m.items[index+1:], m.items[index:])
	m.items[index] = item
	m.revision++
	revision := m.revision
	m.mu.Unlock()
	m.changed.Emit(ListChange{Position: index, Added: 1}, revision)
}

// Append appends item at the end.
func (m *SliceListModel[T]) Append(item T) {
	m.mu.Lock()
	index := len(m.items)
	m.items = append(m.items, item)
	m.revision++
	revision := m.revision
	m.mu.Unlock()
	m.changed.Emit(ListChange{Position: index, Added: 1}, revision)
}

// Remove deletes the item at index. It panics if index is out of range.
func (m *SliceListModel[T]) Remove(index int) {
	m.mu.Lock()
	m.items = append(m.items[:index], m.items[index+1:]...)
	m.revision++
	revision := m.revision
	m.mu.Unlock()
	m.changed.Emit(ListChange{Position: index, Removed: 1}, revision)
}

// Set replaces the item at index. It panics if index is out of range.
func (m *SliceListModel[T]) Set(index int, item T) {
	m.mu.Lock()
	m.items[index] = item
	m.revision++
	revision := m.revision
	m.mu.Unlock()
	m.changed.Emit(ListChange{Position: index, Removed: 1, Added: 1}, revision)
}

// SetItems replaces all items in one go.
func (m *SliceListModel[T]) SetItems(items []T) {
	m.mu.Lock()
	m.items = append([]T(nil), items...)
	m.revision++
	revision := m.revision
	m.mu.Unlock()
	m.changed.Emit(ListChange{Reset: true}, revision)
}

// ConnectItems implements ListModel.
func (m *SliceListModel[T]) ConnectItems(f func()) signal.Handle {
	return m.changed.Connect(func(ListChange, uint64) { f() })
}

// ConnectItemsChanged reports exact ranges for Insert, Append, Remove and Set.
// Modify and SetItems reset the list. It shares the ordinary Items signal;
// stale emissions after a reentrant mutation become resets, never old ranges.
func (m *SliceListModel[T]) ConnectItemsChanged(f func(ListChange)) signal.Handle {
	return m.changed.Connect(func(change ListChange, revision uint64) {
		m.mu.RLock()
		current := m.revision
		m.mu.RUnlock()
		if revision != current {
			change = ListChange{Reset: true}
		}
		f(change)
	})
}
