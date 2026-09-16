package gui

import (
	"fmt"
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
	source.family = "After"
	q.advance(2 * time.Second)
	if source.reads != 4 {
		t.Fatal("native settings queried before GUI dispatch")
	}
	q.dispatch(t)
	if s.FontFamily() != "After" || source.reads != 8 {
		t.Fatal("settings not updated on dispatch")
	}
	q.advance(2 * time.Second)
	stop()
	q.dispatch(t)
	if source.reads != 8 {
		t.Fatal("stopped settings watch queried native state")
	}
}
