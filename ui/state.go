package ui

import (
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/core/state"
)

// State stores a synchronized value and publishes explicit change notifications.
// Get tracks reads during a ComposeView.Build; elsewhere it reads a snapshot.
// BindText/BindSelection update properties without rebuilding their composition.
// Set and Update notify consumers rather than rebuilding the application.
// Keep the State at a stable address after connecting it to a View.
type State[T any] struct {
	state state.State[T]
}

func MakeState[T any](value T) (s State[T]) {
	s.state = state.Make(value)
	return
}

func (s *State[T]) Get() T {
	trackRead(s)
	return s.state.Get()
}

// Peek reads the current value without adding an automatic build dependency.
// It does not remove dependencies established by Get, Read or Observe.
func (s *State[T]) Peek() T { return s.state.Get() }

func (s *State[T]) Connect(fn func()) signal.Handle {
	return s.state.Connect(fn)
}

func (s *State[T]) Set(value T) {
	defer pauseReadTracking()()
	s.state.Set(value)
}

// Update atomically replaces the value via f (read-modify-write) and emits
// one notification; see core/state.State.Update.
func (s *State[T]) Update(f func(before T) (after T)) {
	defer pauseReadTracking()()
	s.state.Update(f)
}
