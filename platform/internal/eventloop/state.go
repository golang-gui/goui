package eventloop

import "sync"

// State stores tasks shared between a platform event loop and other
// goroutines. The platform backend is responsible for waking its native loop
// when Post or Quit returns true.
type State struct {
	mutex       sync.Mutex
	tasks       []func()
	quitting    bool
	destroyed   bool
	wakePending bool
}

func (s *State) Post(task func()) bool {
	if task == nil {
		return false
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.quitting || s.destroyed {
		return false
	}

	s.tasks = append(s.tasks, task)
	if s.wakePending {
		return false
	}
	s.wakePending = true
	return true
}

func (s *State) Quit() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.quitting || s.destroyed {
		return false
	}

	s.quitting = true
	if s.wakePending {
		return false
	}
	s.wakePending = true
	return true
}

func (s *State) Destroy() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.tasks = nil
	s.quitting = true
	s.destroyed = true
	s.wakePending = false
}

func (s *State) Take() (tasks []func(), quitting bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	tasks = s.tasks
	s.tasks = nil
	s.wakePending = false
	return tasks, s.quitting
}

func (s *State) Quitting() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.quitting
}

func (s *State) Destroyed() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.destroyed
}

func RunTasks(state *State) {
	tasks, quitting := state.Take()
	if quitting {
		return
	}

	for _, task := range tasks {
		task()
		if state.Quitting() {
			return
		}
	}
}
