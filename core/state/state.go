package state

import (
	"sync"

	"github.com/golang-gui/goui/core/signal"
)

// State stores a value and emits a synchronous notification after every Set.
type State[T any] struct {
	mu      sync.Mutex
	changed signal.Signal0
	value   T
}

func Make[T any](value T) (s State[T]) {
	s.value = value
	return
}

func (s *State[T]) Get() T {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.value
}

func (s *State[T]) Set(value T) {
	s.mu.Lock()
	s.value = value
	s.mu.Unlock()
	s.changed.Emit()
}

// Update atomically replaces the value via f and emits one notification.
// f runs with the state locked: it must not call back into the state. Use it
// for read-modify-write (counters, toggles) instead of Get+Set.
func (s *State[T]) Update(f func(before T) (after T)) {
	s.mu.Lock()
	s.value = f(s.value)
	s.mu.Unlock()
	s.changed.Emit()
}

func (s *State[T]) Connect(fn func()) signal.Handle {
	if fn == nil {
		return nil
	}
	return s.changed.Connect(fn)
}
