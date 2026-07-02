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

func (s *State[T]) Connect(fn func()) signal.Handle {
	if fn == nil {
		return nil
	}
	return s.changed.Connect(fn)
}
