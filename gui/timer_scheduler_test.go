package gui

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

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
