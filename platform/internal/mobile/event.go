package mobile

import "sync"

// EventLoop dispatches work on the GOUI thread while the native host continues
// running its system event loop. Platform backends own this loop; application
// code uses Application.Post/Run/Quit. It is single-use and thread-affine except
// for Post and Quit. It does not create a goroutine or drive a native UI loop.
type EventLoop struct {
	mu      sync.Mutex
	tasks   []func()
	wake    chan struct{}
	quit    chan struct{}
	stopped bool
	running bool
	ran     bool
}

// NewEventLoop creates a queue owned by the calling GOUI thread.
func NewEventLoop() *EventLoop {
	return &EventLoop{wake: make(chan struct{}, 1), quit: make(chan struct{})}
}

// Post appends a task in acceptance order, or drops it after Quit/Destroy.
func (l *EventLoop) Post(task func()) {
	if task == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stopped {
		return
	}
	l.tasks = append(l.tasks, task)
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// Run processes tasks until Quit. Tasks posted by a task run in a later batch;
// callbacks are never invoked with the queue locked. Run may be called once.
func (l *EventLoop) Run() {
	l.mu.Lock()
	if l.ran {
		l.mu.Unlock()
		panic("mobile: event loop may run only once")
	}
	l.ran, l.running = true, true
	l.mu.Unlock()
	defer func() {
		l.mu.Lock()
		l.running = false
		l.stop()
		l.mu.Unlock()
	}()
	for {
		select {
		case <-l.quit:
			return
		case <-l.wake:
		}
		l.mu.Lock()
		tasks := l.tasks
		l.tasks = nil
		l.mu.Unlock()
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
}

// Quit discards pending work and wakes Run. A callback already admitted for
// execution may finish. It is safe from any goroutine and may precede Run.
func (l *EventLoop) Quit() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stop()
}

func (l *EventLoop) stop() {
	if !l.stopped {
		l.stopped = true
		l.tasks = nil
		close(l.quit)
	}
}

// Destroy releases pending work. Run must have returned before this call.
func (l *EventLoop) Destroy() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running {
		panic("mobile: destroy during event loop Run")
	}
	l.stop()
}
