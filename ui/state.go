package ui

import (
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/core/state"
)

// State stores a synchronized value and publishes explicit change notifications.
// Use BindText/BindSelection, or Observe on a composition, to update consumers.
// Get does not track dependencies; Set and Update do not rebuild the application.
// Keep the State at a stable address after connecting it to a View.
type State[T any] struct {
	state state.State[T]
}

func MakeState[T any](value T) (s State[T]) {
	s.state = state.Make(value)
	return
}

func (s *State[T]) Get() T {
	return s.state.Get()
}

func (s *State[T]) Connect(fn func()) signal.Handle {
	return s.state.Connect(fn)
}

func (s *State[T]) Set(value T) {
	s.state.Set(value)
}

// Update atomically replaces the value via f (read-modify-write) and emits
// one notification; see core/state.State.Update.
func (s *State[T]) Update(f func(before T) (after T)) {
	s.state.Update(f)
}
