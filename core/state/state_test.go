package state

import "testing"

func TestStateGetSet(t *testing.T) {
	state := Make("first")

	if got := state.Get(); got != "first" {
		t.Fatalf("initial value = %q, want first", got)
	}

	state.Set("second")
	if got := state.Get(); got != "second" {
		t.Fatalf("updated value = %q, want second", got)
	}
}

func TestStateConnect(t *testing.T) {
	state := Make(1)
	var values []int
	handle := state.Connect(func() {
		values = append(values, state.Get())
	})

	state.Set(2)
	state.Set(3)

	if len(values) != 2 || values[0] != 2 || values[1] != 3 {
		t.Fatalf("unexpected notifications: %v", values)
	}

	handle.Disconnect()
	state.Set(4)
	if len(values) != 2 {
		t.Fatalf("disconnected listener was called: %v", values)
	}
}

func TestZeroStateGetSet(t *testing.T) {
	var state State[int]

	if got := state.Get(); got != 0 {
		t.Fatalf("zero State value = %d, want 0", got)
	}

	state.Set(2)

	if got := state.Get(); got != 2 {
		t.Fatalf("updated zero State value = %d, want 2", got)
	}
}

func TestZeroStateConnect(t *testing.T) {
	var state State[int]
	called := false
	handle := state.Connect(func() {
		called = true
	})

	state.Set(1)
	if !called {
		t.Fatal("zero State listener was not called")
	}

	handle.Disconnect()
	called = false
	state.Set(2)
	if called {
		t.Fatal("disconnected zero State listener was called")
	}
}

func TestStateSetDoesNotHoldValueLockDuringEmit(t *testing.T) {
	state := Make(1)
	updated := false
	state.Connect(func() {
		if updated {
			return
		}
		updated = true
		state.Set(state.Get() + 1)
	})

	state.Set(2)

	if got := state.Get(); got != 3 {
		t.Fatalf("state value = %d, want 3", got)
	}
}
