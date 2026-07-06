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

// WakeFailed reverts the pending-wake flag after a backend's wake attempt fails
// to reach the native loop, so the next Post or Quit requests a fresh wake
// instead of assuming one is already scheduled. Already-queued tasks stay
// queued and run on that next successful wake.
func (s *State) WakeFailed() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.wakePending = false
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

// RunTasks drains and runs every task taken from the queue. It does not stop on
// quit: a quit drains the backlog queued before it, and the loop stops pumping
// via Quitting(). Tasks posted after quit are dropped at Post; Destroy clears
// the queue outright.
func (s *State) RunTasks() {
	tasks, _ := s.Take()
	for _, task := range tasks {
		task()
	}
}
