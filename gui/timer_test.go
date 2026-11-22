package gui

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// No worker or sleeps: advance time, request a wake, then explicitly run Post.
type timerTestQueue struct {
	app   *application
	s     *timerScheduler
	now   time.Time
	posts []func()
}

func newTimerTestQueue(t *testing.T) *timerTestQueue {
	t.Helper()
	q := &timerTestQueue{now: time.Now()}
	q.s = newTimerScheduler(func(fn func()) { q.posts = append(q.posts, fn) }, func() time.Time { return q.now })
	q.app = &application{timers: q.s}
	t.Cleanup(q.s.finish)
	return q
}

func (q *timerTestQueue) advance(d time.Duration) {
	q.now = q.now.Add(d)
	q.s.postDue()
}

func (q *timerTestQueue) dispatch(t *testing.T) {
	t.Helper()
	if len(q.posts) == 0 {
		t.Fatal("no timer dispatch posted")
	}
	fn := q.posts[0]
	q.posts = q.posts[1:]
	fn()
}

func startTestTimer(t *testing.T, q *timerTestQueue, d time.Duration, once bool, fn func()) *Timer {
	t.Helper()
	timer := q.app.NewTimer()
	timer.ConnectTimeout(fn)
	start := timer.Start
	if once {
		start = timer.StartOnce
	}
	if err := start(d); err != nil {
		t.Fatal(err)
	}
	return timer
}

func TestTimerPublicAPI(t *testing.T) {
	previous := App
	t.Cleanup(func() { App = previous })
	App = nil
	var zero Timer
	zero.Stop()
	if zero.Active() || zero.Start(time.Second) == nil || zero.StartOnce(time.Second) == nil {
		t.Fatal("zero timer must be inactive and reject starting")
	}
	q := newTimerTestQueue(t)
	timer := q.app.NewTimer() // No dependency on global App, which is still nil.
	if timer.Active() || len(q.s.queue) != 0 {
		t.Fatal("NewTimer registered a task")
	}
	calls := 0
	timer.ConnectTimeout(func() {
		calls++
		if timer.Active() {
			t.Fatal("one-shot timer is active in its signal")
		}
	})
	if err := timer.StartOnce(time.Second); err != nil {
		t.Fatal(err)
	}
	run := timer.run
	for _, d := range []time.Duration{0, -1} {
		if timer.Start(d) == nil || timer.run != run {
			t.Fatal("invalid start replaced a valid run")
		}
	}
	q.advance(time.Second)
	q.dispatch(t)
	if calls != 1 || timer.Active() || len(q.s.queue) != 0 {
		t.Fatal("one-shot did not finish")
	}
	if err := timer.Start(time.Second); err != nil {
		t.Fatal(err)
	}
	timer.Stop()
	timer.Stop()
	if err := timer.StartOnce(time.Second); err != nil {
		t.Fatal(err)
	}
	q.advance(time.Second)
	q.dispatch(t)
	if calls != 2 {
		t.Fatal("stop disconnected the signal")
	}
	q.s.close()
	if timer.Start(time.Second) == nil {
		t.Fatal("start after shutdown succeeded")
	}
}

// Verify convenience functions use Application.NewTimer through the interface,
// rather than reaching through a concrete/global application to the scheduler.
type timerFactoryTestApplication struct {
	Application
	created int
}

func (a *timerFactoryTestApplication) NewTimer() *Timer {
	a.created++
	return a.Application.NewTimer()
}

func TestTimerCancelAndRestartPosted(t *testing.T) {
	q := newTimerTestQueue(t)
	calls := 0
	timer := startTestTimer(t, q, time.Second, true, func() { calls++ })
	q.advance(time.Second)
	timer.Stop()
	q.dispatch(t)
	if calls != 0 || len(q.s.queue) != 0 {
		t.Fatal("cancelled posted task ran")
	}
	if err := timer.StartOnce(time.Second); err != nil {
		t.Fatal(err)
	}
	q.advance(time.Second)
	if err := timer.StartOnce(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	q.dispatch(t)
	if calls != 0 {
		t.Fatal("stale wake fired new run early")
	}
	q.advance(2 * time.Second)
	q.dispatch(t)
	if calls != 1 {
		t.Fatal("restarted timer did not fire")
	}
}

func TestTimerCancelSameBatch(t *testing.T) {
	for _, restart := range []bool{false, true} {
		t.Run(fmt.Sprint(restart), func(t *testing.T) {
			q := newTimerTestQueue(t)
			var b *Timer
			startTestTimer(t, q, time.Second, true, func() {
				if restart {
					if err := b.StartOnce(time.Second); err != nil {
						t.Fatal(err)
					}
				} else {
					b.Stop()
				}
			})
			calls := 0
			b = startTestTimer(t, q, 2*time.Second, true, func() { calls++ })
			q.advance(3 * time.Second)
			q.dispatch(t)
			if calls != 0 {
				t.Fatal("invalidated task from same batch ran")
			}
			if restart {
				q.advance(time.Second)
				q.dispatch(t)
				if calls != 1 {
					t.Fatal("replacement task did not run")
				}
			}
		})
	}
}

func TestTimerPeriodicCadence(t *testing.T) {
	q := newTimerTestQueue(t)
	base := q.now
	var timer *Timer
	calls := 0
	timer = startTestTimer(t, q, 100*time.Millisecond, false, func() {
		calls++
		if !timer.Active() {
			t.Fatal("periodic timer inactive in signal")
		}
		if calls == 1 {
			q.now = q.now.Add(30 * time.Millisecond)
		} else {
			q.now = q.now.Add(150 * time.Millisecond)
		}
	})
	run := timer.run
	q.advance(100 * time.Millisecond)
	q.dispatch(t)
	if timer.run != run || !run.deadline.Equal(base.Add(200*time.Millisecond)) {
		t.Fatal("periodic cadence drifted or allocated a new run")
	}
	q.advance(70 * time.Millisecond)
	q.dispatch(t)
	if !run.deadline.Equal(base.Add(400*time.Millisecond)) || calls != 2 {
		t.Fatal("long callback did not skip missed periods")
	}
	q.advance(time.Second)
	q.dispatch(t)
	if calls != 3 || !run.deadline.After(q.now) {
		t.Fatal("delayed dispatch replayed missed periods")
	}
}

func TestTimerCallbackStopsOrRestartsSelf(t *testing.T) {
	q := newTimerTestQueue(t)
	var timer *Timer
	calls := 0
	timer = startTestTimer(t, q, time.Second, false, func() {
		calls++
		if calls == 1 {
			if err := timer.StartOnce(2 * time.Second); err != nil {
				t.Fatal(err)
			}
		} else {
			timer.Stop()
		}
	})
	q.advance(time.Second)
	q.dispatch(t)
	if len(q.s.queue) != 1 || !timer.run.once {
		t.Fatal("old periodic run was renewed after explicit restart")
	}
	q.advance(2 * time.Second)
	q.dispatch(t)
	if calls != 2 || timer.Active() || len(q.s.queue) != 0 {
		t.Fatal("one-shot replacement was renewed")
	}
	var stopped *Timer
	emissions := 0
	stopped = startTestTimer(t, q, time.Second, false, func() { stopped.Stop() })
	stopped.ConnectTimeout(func() { emissions++ })
	q.advance(time.Second)
	q.dispatch(t)
	if stopped.Active() || len(q.s.queue) != 0 || emissions != 1 {
		t.Fatal("Stop should finish current signal but prevent renewal")
	}
}

func TestTimerBatchLimitAndWakeCoalescing(t *testing.T) {
	q := newTimerTestQueue(t)
	calls := 0
	for range timerBatchSize + 1 {
		startTestTimer(t, q, time.Second, true, func() { calls++ })
	}
	q.advance(time.Second)
	for range 100 {
		q.s.postDue()
	}
	if len(q.posts) != 1 {
		t.Fatal("timer wakes were not coalesced")
	}
	q.dispatch(t)
	if calls != timerBatchSize || len(q.s.queue) != 1 {
		t.Fatal("dispatch exceeded its batch limit")
	}
	q.s.postDue()
	q.dispatch(t)
	if calls != timerBatchSize+1 {
		t.Fatal("remaining task was lost")
	}
	q.advance(time.Hour)
	if len(q.posts) != 0 {
		t.Fatal("empty scheduler posted work")
	}
}

func TestTimerNestedDispatch(t *testing.T) {
	q := newTimerTestQueue(t)
	aCalls, bCalls := 0, 0
	a := startTestTimer(t, q, time.Second, false, func() {
		aCalls++
		q.advance(2 * time.Second)
		q.dispatch(t) // Simulate a native nested loop while A's callback waits.
	})
	startTestTimer(t, q, 2*time.Second, true, func() { bCalls++ })
	q.advance(time.Second)
	q.dispatch(t)
	if aCalls != 1 || bCalls != 1 || !a.Active() || len(q.s.queue) != 1 {
		t.Fatal("nested dispatch blocked other timers or reentered the same run")
	}
}

func TestTimerShutdownInvalidatesBatchAndPending(t *testing.T) {
	q := newTimerTestQueue(t)
	loop := &quitCountLoop{}
	a := q.app
	a.loop = loop
	first := startTestTimer(t, q, time.Second, false, a.Quit)
	calls := 0
	second := startTestTimer(t, q, 2*time.Second, true, func() { calls++ })
	q.advance(2 * time.Second)
	q.dispatch(t)
	if calls != 0 || first.Active() || second.Active() || len(q.s.queue) != 0 || loop.quits != 1 {
		t.Fatal("quit failed to invalidate batched work/periodic renewal")
	}
	q = newTimerTestQueue(t)
	startTestTimer(t, q, time.Second, true, func() { calls++ })
	q.advance(time.Second)
	q.s.close()
	q.dispatch(t)
	if calls != 0 {
		t.Fatal("queued dispatch ran after close")
	}
}

func TestTimerHeapRemoval(t *testing.T) {
	q := newTimerTestQueue(t)
	timers := make([]*Timer, 1000)
	for i := range timers {
		timers[i] = startTestTimer(t, q, time.Duration((i*37)%1000+1), true, func() {})
	}
	check := func() {
		for i, run := range q.s.queue {
			if run.index != i || (i > 0 && run.deadline.Before(q.s.queue[(i-1)/2].deadline)) {
				t.Fatal("heap index/order corrupted")
			}
		}
	}
	check()
	for _, timer := range timers {
		run := timer.run
		timer.Stop()
		if run.index != -1 {
			t.Fatal("removed task still has heap index")
		}
		check()
	}
	if len(q.s.queue) != 0 {
		t.Fatal("stopped tasks retained in heap")
	}
}

func TestSettingsUsesSharedTimer(t *testing.T) {
	q := newTimerTestQueue(t)
	source := &settingsTestSource{family: "Before", size: 14}
	s := newSettings(source)
	stop := s.watch(q.app)
	defer stop()
	if len(q.s.queue) != 1 || q.s.queue[0].interval != 2*time.Second {
		t.Fatal("settings did not register shared two-second timer")
	}
	// 六项设置（含合成能力）在初始化和每次 GUI 派发时各读取一次。
	source.family = "After"
	q.advance(2 * time.Second)
	if source.reads != 6 {
		t.Fatal("native settings queried before GUI dispatch")
	}
	q.dispatch(t)
	if s.FontFamily() != "After" || source.reads != 12 {
		t.Fatal("settings not updated on dispatch")
	}
	q.advance(2 * time.Second)
	stop()
	q.dispatch(t)
	if source.reads != 12 {
		t.Fatal("stopped settings watch queried native state")
	}
}

// --- 共享计时调度与工作线程 ---

func receiveTimerPost(t *testing.T, posts <-chan func()) func() {
	t.Helper()
	select {
	case fn := <-posts:
		return fn
	case <-time.After(5 * time.Second):
		t.Fatal("timer worker did not post a dispatch")
		return nil
	}
}

func TestTimerSharedWaiter(t *testing.T) {
	posts := make(chan func(), 8)
	s := newTimerScheduler(func(fn func()) { posts <- fn }, time.Now)
	app := &application{timers: s}
	defer s.finish()
	timers := make([]*Timer, 1000)
	before := runtime.NumGoroutine()
	for i := range timers {
		timers[i] = app.NewTimer()
		if err := timers[i].StartOnce(time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	if runtime.NumGoroutine() > before+1 {
		t.Fatal("creating timers started per-timer goroutines")
	}
	s.start()
	s.start() // One waiting goroutine even if start is repeated.
	calls := 0
	short := app.NewTimer()
	short.ConnectTimeout(func() { calls++ })
	if err := short.StartOnce(time.Millisecond); err != nil {
		t.Fatal(err)
	}
	fn := receiveTimerPost(t, posts)
	if calls != 0 {
		t.Fatal("worker executed user code instead of posting")
	}
	if runtime.NumGoroutine() > before+2 {
		t.Fatal("waiting allocated more than one worker")
	}
	fn()
	if calls != 1 {
		t.Fatal("GUI dispatch did not execute notification")
	}
	for _, timer := range timers {
		timer.Stop()
	}
	// Re-arm after an empty queue; cancelling the earliest run must not affect it.
	if err := short.StartOnce(time.Hour); err != nil {
		t.Fatal(err)
	}
	short.Stop()
	if err := short.StartOnce(time.Millisecond); err != nil {
		t.Fatal(err)
	}
	receiveTimerPost(t, posts)()
	if calls != 2 {
		t.Fatal("worker failed to re-arm after cancellation")
	}
	s.finish()
	select {
	case <-s.done:
	default:
		t.Fatal("worker still running after finish")
	}
}

func TestTimerConcurrentShutdown(t *testing.T) {
	for range 50 {
		posts := make(chan func(), 8)
		s := newTimerScheduler(func(fn func()) { posts <- fn }, time.Now)
		app := &application{timers: s}
		s.start()
		timer := app.NewTimer()
		var closing sync.WaitGroup
		closing.Add(1)
		go func() {
			defer closing.Done()
			s.close()
		}()
		// Timer operations are still serialized on this GUI stand-in thread.
		// Only shutdown races them, as Application.Quit is allowed to do.
		for range 10 {
			_ = timer.Start(time.Nanosecond)
			timer.Stop()
		}
		closing.Wait()
		s.finish()
		for len(posts) != 0 {
			(<-posts)()
		}
		if timer.Active() || len(s.queue) != 0 {
			t.Fatal("shutdown left an active run")
		}
	}
}

func TestTimerApplicationLifecycle(t *testing.T) {
	for _, quitBeforeRun := range []bool{false, true} {
		t.Run(fmt.Sprint(quitBeforeRun), func(t *testing.T) {
			posts := make(chan func(), 8)
			s := newTimerScheduler(func(fn func()) { posts <- fn }, time.Now)
			loop := &settingsTestLoop{posts: posts}
			a := &application{loop: loop, timers: s}
			timer := a.NewTimer()
			calls := 0
			timer.ConnectTimeout(func() { calls++ })
			if err := timer.Start(time.Nanosecond); err != nil {
				t.Fatal(err)
			}
			if s.started {
				t.Fatal("worker started before Run")
			}
			if quitBeforeRun {
				s.close()
				loop.run = func() {}
			} else {
				loop.run = func() { receiveTimerPost(t, posts)() }
			}
			a.Run()
			if timer.Active() || !s.closed || (quitBeforeRun && (calls != 0 || s.started)) || (!quitBeforeRun && calls != 1) {
				t.Fatal("Run lifecycle did not bound timer activity")
			}
		})
	}
}

// Exercise the scheduler directly: no Application, Timer, signals or global App.
func TestTimerSchedulerTaskLifecycle(t *testing.T) {
	now := time.Now()
	var posted func()
	s := newTimerScheduler(func(fn func()) { posted = fn }, func() time.Time { return now })
	defer s.finish()
	schedule := func(previous *timerRun, interval time.Duration, once bool, callback func()) *timerRun {
		t.Helper()
		run, err := s.schedule(previous, interval, once, callback)
		if err != nil {
			t.Fatal(err)
		}
		return run
	}
	flush := func(d time.Duration) {
		t.Helper()
		now = now.Add(d)
		s.postDue()
		if posted == nil {
			t.Fatal("no scheduler dispatch posted")
		}
		fn := posted
		posted = nil
		fn()
	}
	var once *timerRun
	calls := 0
	once = schedule(nil, time.Second, true, func() {
		calls++
		if s.active(once) {
			t.Fatal("one-shot task active inside callback")
		}
	})
	flush(time.Second)
	if calls != 1 || once.callback != nil || once.index != -1 {
		t.Fatal("completed task retained callback or heap membership")
	}

	// Invalid replacements preserve both membership and notification target.
	old := schedule(nil, time.Second, false, func() { calls++ })
	if run, err := s.schedule(old, 0, false, func() {}); run != nil || err == nil || !s.active(old) {
		t.Fatal("invalid duration replaced a task")
	}
	if run, err := s.schedule(old, time.Second, false, nil); run != nil || err == nil || !s.active(old) {
		t.Fatal("nil callback replaced a task")
	}
	replacement := schedule(old, time.Second, true, func() { calls++ })
	if s.active(old) || old.callback != nil || old.index != -1 || len(s.queue) != 1 {
		t.Fatal("replacement did not invalidate/release the previous task")
	}
	flush(time.Second)
	if calls != 2 || s.active(replacement) {
		t.Fatal("replacement notification failed")
	}

	// Cancel an already-extracted task, without asking any owner about validity.
	var cancelled *timerRun
	schedule(nil, time.Second, true, func() { s.cancel(cancelled) })
	cancelled = schedule(nil, 2*time.Second, false, func() { t.Error("cancelled task ran") })
	flush(2 * time.Second)
	if s.active(cancelled) || cancelled.callback != nil || len(s.queue) != 0 {
		t.Fatal("cancelled batch task was renewed or retained its callback")
	}

	// Replacement during a periodic callback prevents renewal of the old task.
	var periodic, next *timerRun
	periodic = schedule(nil, time.Second, false, func() {
		if !s.active(periodic) {
			t.Fatal("periodic task inactive in callback")
		}
		next = schedule(periodic, time.Second, true, func() { calls++ })
	})
	flush(time.Second)
	if s.active(periodic) || !s.active(next) || len(s.queue) != 1 {
		t.Fatal("old periodic task renewed after callback replacement")
	}
	flush(time.Second)
	if calls != 3 || len(s.queue) != 0 {
		t.Fatal("replacement from callback did not complete")
	}

	// Shutdown invalidates pending and in-flight tasks, without notifying owners.
	closing := schedule(nil, time.Second, false, s.close)
	skipped := schedule(nil, 2*time.Second, true, func() { t.Error("callback after shutdown") })
	queued := schedule(nil, time.Hour, false, func() { t.Error("callback after shutdown") })
	flush(2 * time.Second)
	for _, run := range []*timerRun{closing, skipped, queued} {
		if s.active(run) || run.callback != nil || run.index != -1 {
			t.Fatal("shutdown retained an active task or its callback")
		}
	}
}

func BenchmarkTimerScheduler(b *testing.B) {
	for _, count := range []int{1, 100, 1000} {
		b.Run(fmt.Sprintf("StartStop/%d", count), func(b *testing.B) {
			now := time.Now()
			s := newTimerScheduler(func(func()) {}, func() time.Time { return now })
			defer s.finish()
			app := &application{timers: s}
			timers := make([]*Timer, count)
			for i := range timers {
				timers[i] = app.NewTimer()
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				for i := range timers {
					if err := timers[i].Start(time.Duration(i + 1)); err != nil {
						b.Fatal(err)
					}
				}
				for i := range timers {
					timers[i].Stop()
				}
			}
		})
		b.Run(fmt.Sprintf("PeriodicBatch/%d", count), func(b *testing.B) {
			now := time.Now()
			s := newTimerScheduler(func(func()) {}, func() time.Time { return now })
			defer s.finish()
			app := &application{timers: s}
			for range count {
				timer := app.NewTimer()
				timer.ConnectTimeout(func() {})
				if err := timer.Start(time.Second); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				now = now.Add(time.Second)
				for s.queue[0].deadline.Compare(now) <= 0 {
					s.postDue()
					s.dispatch()
				}
			}
		})
	}
}
