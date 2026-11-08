package mobile

import "sync"

// EventLoop sends its queue to the native UI loop. Run only waits on the startup
// goroutine; Dispatch and native observations execute on the system UI thread.
// Post and Quit are safe from any goroutine. A loop may run only once.
type EventLoop struct {
	mu      sync.Mutex
	tasks   []func()
	wake    func()
	current func() bool
	quit    chan struct{}

	stopped     bool
	running     bool
	ran         bool
	scheduled   bool
	dispatching bool
	ready       bool
	finished    bool
	delivering  int
}

// NewEventLoop binds a native wakeup and an owning-thread query. wake must
// schedule Dispatch asynchronously, including when called on the owning thread.
func NewEventLoop(wake func(), current func() bool) *EventLoop {
	if wake == nil || current == nil {
		panic("mobile: native event loop hooks are required")
	}
	return &EventLoop{wake: wake, current: current, quit: make(chan struct{})}
}

// Post appends a task in acceptance order. Tasks posted by a task run in a later
// batch. No callback runs before Run or after Quit has rejected admission.
func (l *EventLoop) Post(task func()) {
	if task == nil {
		return
	}
	l.mu.Lock()
	if l.stopped {
		l.mu.Unlock()
		return
	}
	l.tasks = append(l.tasks, task)
	wake := l.schedule()
	l.mu.Unlock()
	if wake {
		l.wake()
	}
}

func (l *EventLoop) schedule() bool {
	if !l.ran || l.scheduled || l.finished {
		return false
	}
	l.scheduled = true
	return true
}

// Run starts native dispatch and waits until the native loop acknowledges Quit.
// It must not block the native UI thread, whose event loop owns execution.
func (l *EventLoop) Run() {
	if l.current() {
		panic("mobile: Run must not block the native UI thread")
	}
	l.mu.Lock()
	if l.ran {
		l.mu.Unlock()
		panic("mobile: event loop may run only once")
	}
	l.ran, l.running = true, true
	wake := l.schedule()
	l.mu.Unlock()
	if wake {
		l.wake()
	}
	<-l.quit
	l.mu.Lock()
	l.running = false
	l.mu.Unlock()
}

// Dispatch drains one batch on the native UI thread. Backends call it only
// from their native wakeup callback; nested native loops defer queued Go work.
func (l *EventLoop) Dispatch() {
	if !l.current() {
		panic("mobile: dispatch outside the native UI thread")
	}
	l.mu.Lock()
	l.scheduled = false
	if l.dispatching || l.delivering != 0 || l.finished {
		l.mu.Unlock()
		return
	}
	if l.stopped {
		l.finish()
		l.mu.Unlock()
		return
	}
	if !l.ran {
		l.mu.Unlock()
		return
	}
	l.ready, l.dispatching = true, true
	tasks := l.tasks
	l.tasks = nil
	l.mu.Unlock()
	defer func() {
		l.mu.Lock()
		l.dispatching = false
		wake := false
		if l.stopped || len(l.tasks) != 0 {
			wake = l.schedule()
		}
		l.mu.Unlock()
		if wake {
			l.wake()
		}
	}()
	for _, task := range tasks {
		l.mu.Lock()
		stopped := l.stopped
		l.mu.Unlock()
		if stopped {
			return
		}
		task()
	}
}

// Deliver executes a native observation inline once dispatch is ready. Calls
// before Run or from another thread enter the same queue as Post.
func (l *EventLoop) Deliver(task func()) {
	if task == nil {
		return
	}
	if !l.current() {
		l.Post(task)
		return
	}
	l.mu.Lock()
	if l.stopped {
		l.mu.Unlock()
		return
	}
	if !l.ready {
		l.mu.Unlock()
		l.Post(task)
		return
	}
	l.delivering++
	l.mu.Unlock()
	defer func() {
		l.mu.Lock()
		l.delivering--
		wake := false
		if l.stopped || len(l.tasks) != 0 {
			wake = l.schedule()
		}
		l.mu.Unlock()
		if wake {
			l.wake()
		}
	}()
	task()
}

// Quit discards queued work. During Run, completion is acknowledged by a later
// native dispatch, after the current native callback has returned.
func (l *EventLoop) Quit() {
	l.mu.Lock()
	l.stopped = true
	l.tasks = nil
	wake := false
	if !l.ran {
		l.finish()
	} else {
		wake = l.schedule()
	}
	l.mu.Unlock()
	if wake {
		l.wake()
	}
}

func (l *EventLoop) finish() {
	if !l.finished {
		l.finished = true
		close(l.quit)
	}
}

// Destroy releases pending Go work after Run returns. Native startup facilities
// remain owned by the platform host, independently of Activity/Scene instances.
func (l *EventLoop) Destroy() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running {
		panic("mobile: destroy during event loop Run")
	}
	l.stopped = true
	l.tasks = nil
	l.finish()
}
