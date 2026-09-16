package gui

import (
	"container/heap"
	"errors"
	"sync"
	"time"
)

const timerBatchSize = 64

// timerRun is a scheduler-owned task. A nil callback means cancelled or complete,
// including while outside the heap awaiting dispatch. Automatic periods reuse
// the task; replacement creates a new one. No public Timer state is involved.
type timerRun struct {
	callback func()
	deadline time.Time
	interval time.Duration
	once     bool
	index    int // protected by scheduler.mu; -1 when not in the heap
}

type timerHeap []*timerRun

func (h timerHeap) Len() int           { return len(h) }
func (h timerHeap) Less(i, j int) bool { return h[i].deadline.Before(h[j].deadline) }
func (h timerHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
}
func (h *timerHeap) Push(value any) {
	run := value.(*timerRun)
	run.index = len(*h)
	*h = append(*h, run)
}
func (h *timerHeap) Pop() any {
	i := len(*h) - 1
	run := (*h)[i]
	(*h)[i] = nil
	*h = (*h)[:i]
	run.index = -1
	return run
}

// timerScheduler schedules callbacks through post, without knowing their owners
// or notification mechanisms. Only wait owns the underlying time.Timer. The
// mutex protects task/queue/lifecycle state; callbacks run unlocked via post.
type timerScheduler struct {
	mu      sync.Mutex
	queue   timerHeap
	closed  bool
	started bool
	pending bool // one posted dispatch which has not started yet
	changed chan struct{}
	done    chan struct{}
	post    func(func())
	now     func() time.Time
}

func newTimerScheduler(post func(func()), now func() time.Time) *timerScheduler {
	return &timerScheduler{post: post, now: now, changed: make(chan struct{}, 1), done: make(chan struct{})}
}

func (s *timerScheduler) notify() {
	select {
	case s.changed <- struct{}{}:
	default:
	}
}

// schedule validates and replaces a task under one lock. previous, if non-nil,
// must belong to this scheduler. Invalid input preserves the previous task.
func (s *timerScheduler) schedule(previous *timerRun, interval time.Duration, once bool, callback func()) (*timerRun, error) {
	if interval <= 0 {
		return nil, errors.New("timer: duration must be positive")
	}
	if callback == nil {
		return nil, errors.New("timer: callback must not be nil")
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, errors.New("timer: scheduler has closed")
	}
	s.cancelLocked(previous)
	run := &timerRun{callback: callback, deadline: s.now().Add(interval), interval: interval, once: once, index: -1}
	heap.Push(&s.queue, run)
	s.mu.Unlock()
	s.notify()
	return run, nil
}

func (s *timerScheduler) cancel(run *timerRun) {
	s.mu.Lock()
	s.cancelLocked(run)
	s.mu.Unlock()
	s.notify()
}

func (s *timerScheduler) cancelLocked(run *timerRun) {
	if run == nil {
		return
	}
	run.callback = nil
	if run.index >= 0 {
		heap.Remove(&s.queue, run.index)
	}
}

func (s *timerScheduler) active(run *timerRun) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.closed && run != nil && run.callback != nil
}

func (s *timerScheduler) renew(run *timerRun) {
	s.mu.Lock()
	if s.closed || run.callback == nil {
		run.callback = nil
		s.mu.Unlock()
		return
	}
	run.deadline = nextTimerDeadline(run.deadline, s.now(), run.interval)
	heap.Push(&s.queue, run)
	s.mu.Unlock()
	s.notify()
}

func (s *timerScheduler) start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed && !s.started {
		s.started = true
		go s.wait()
	}
}

// close may be called from any goroutine. It invalidates pending tasks without
// invoking callbacks or accessing their owners. In-flight dispatches also check
// closed; a callback already claimed by dispatch may finish.
func (s *timerScheduler) close() {
	s.mu.Lock()
	s.closed = true
	for _, run := range s.queue {
		run.index = -1
		run.callback = nil
	}
	s.queue = nil
	s.mu.Unlock()
	s.notify()
}

// finish is used after the native loop returns; Quit itself must not wait.
func (s *timerScheduler) finish() {
	s.close()
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	if started {
		<-s.done
	}
}

func (s *timerScheduler) wait() {
	defer close(s.done)
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		timer.Stop()
		var tick <-chan time.Time
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return
		}
		if !s.pending && len(s.queue) != 0 {
			timer.Reset(s.queue[0].deadline.Sub(s.now()))
			tick = timer.C
		}
		s.mu.Unlock()
		select {
		case <-s.changed:
		case <-tick:
			s.postDue()
		}
	}
}

// postDue rechecks the queue after a possibly stale wakeup. The posted task owns
// no individual tasks, so cancellation can release callbacks before dispatch.
func (s *timerScheduler) postDue() {
	s.mu.Lock()
	if s.closed || s.pending || len(s.queue) == 0 || s.queue[0].deadline.After(s.now()) {
		s.mu.Unlock()
		return
	}
	s.pending = true
	s.mu.Unlock()
	s.post(s.dispatch)
}

func (s *timerScheduler) dispatch() {
	var batch [timerBatchSize]*timerRun
	n := 0
	s.mu.Lock()
	if !s.closed {
		now := s.now()
		for n < len(batch) && len(s.queue) != 0 && !s.queue[0].deadline.After(now) {
			batch[n] = heap.Pop(&s.queue).(*timerRun)
			n++
		}
	}
	s.pending = false
	s.mu.Unlock()
	// Allow other timers to dispatch during a callback's native nested loop.
	// Each periodic run stays outside the heap until its own signal returns.
	s.notify()
	for _, run := range batch[:n] {
		s.mu.Lock()
		callback := run.callback
		if s.closed {
			callback = nil
		}
		if s.closed || run.once {
			run.callback = nil
		}
		s.mu.Unlock()
		if callback == nil {
			continue
		}
		callback()
		if !run.once {
			s.renew(run)
		}
	}
}

func nextTimerDeadline(previous, now time.Time, interval time.Duration) time.Time {
	// Avoid multiplying a potentially large missed-period count by interval.
	return now.Add(interval - now.Sub(previous)%interval)
}
