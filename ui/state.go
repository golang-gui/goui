package ui

import (
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/core/state"
)

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
	// State lives in the view tree with no App handle in scope, so it reaches the
	// running app through the active-runtime singleton — the one internal use of it.
	if rt := currentApp(); rt != nil {
		rt.RequestUpdate()
	}
}
