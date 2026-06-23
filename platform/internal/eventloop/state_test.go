package eventloop

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestStateRunsTasksInOrder(t *testing.T) {
	var state State
	var order []int

	if !state.Post(func() { order = append(order, 1) }) {
		t.Fatal("first task must request a wake")
	}
	if state.Post(func() { order = append(order, 2) }) {
		t.Fatal("queued tasks must share the pending wake")
	}

	RunTasks(&state)

	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("unexpected task order: %v", order)
	}
}

func TestStatePostWhileRunningRequestsAnotherWake(t *testing.T) {
	var state State
	var ranSecond bool

	state.Post(func() {
		if !state.Post(func() { ranSecond = true }) {
			t.Fatal("task posted while running must request another wake")
		}
	})

	RunTasks(&state)
	if ranSecond {
		t.Fatal("task posted while running must not run in the current batch")
	}

	RunTasks(&state)
	if !ranSecond {
		t.Fatal("second task did not run")
	}
}

func TestStateQuitStopsQueuedTasks(t *testing.T) {
	var state State
	var ran bool

	state.Post(func() { ran = true })
	state.Quit()
	RunTasks(&state)

	if ran {
		t.Fatal("queued task ran after quit")
	}
	if !state.Quitting() {
		t.Fatal("state must remain quitting")
	}
	if state.Post(func() {}) {
		t.Fatal("post after quit must be ignored")
	}
}

func TestStateConcurrentPost(t *testing.T) {
	var state State
	var count atomic.Int32
	var wait sync.WaitGroup

	const taskCount = 100
	wait.Add(taskCount)
	for range taskCount {
		go func() {
			defer wait.Done()
			state.Post(func() {
				count.Add(1)
			})
		}()
	}
	wait.Wait()

	RunTasks(&state)
	if count.Load() != taskCount {
		t.Fatalf("expected %d tasks, got %d", taskCount, count.Load())
	}
}

func TestStateDestroy(t *testing.T) {
	var state State
	var ran bool

	state.Post(func() { ran = true })
	state.Destroy()
	state.Destroy()
	RunTasks(&state)

	if ran {
		t.Fatal("queued task ran after destroy")
	}
	if !state.Destroyed() {
		t.Fatal("state must be destroyed")
	}
	if state.Post(func() {}) {
		t.Fatal("post after destroy must be ignored")
	}
	if state.Quit() {
		t.Fatal("quit after destroy must be ignored")
	}
}
