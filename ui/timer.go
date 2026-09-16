package ui

import (
	"time"

	"github.com/golang-gui/goui/gui"
)

// Timer is a reusable runtime timer created by App.TimeoutFunc or App.TickFunc.
// All methods may be called from any goroutine; operations run synchronously on
// the UI thread, and the callback always runs there. A Timer must not be copied.
//
// Restarting preserves the original callback, even when switching between
// periodic and one-shot operation. Capture persistent application state when the
// callback needs fresh values; rebuilding a View does not replace the callback.
//
// Timers are not View declarations. Create them in event handlers or one-time
// initialization, not unconditionally during Build. Call Stop when the task is
// no longer needed; removing a View does not stop it. Application exit stops all
// timers. Callbacks do not request updates automatically: use State.Set/Update or
// App.RequestUpdate. This timer is not a frame-synchronized animation clock.
//
// A nil, zero-value or exited-app Timer is inert: Stop and valid starts are no-ops,
// and Active is false. Non-positive durations panic, including on inert timers.
type Timer struct {
	app   *app
	timer *gui.Timer // created/accessed on the UI thread; never exposed to callers
}

func (a *app) TimeoutFunc(delay time.Duration, fn func()) *Timer {
	return a.timerFunc(delay, fn, true)
}

func (a *app) TickFunc(interval time.Duration, fn func()) *Timer {
	return a.timerFunc(interval, fn, false)
}

func (a *app) timerFunc(duration time.Duration, fn func(), once bool) *Timer {
	checkTimerDuration(duration)
	if fn == nil {
		panic("ui: timer callback must not be nil")
	}
	t := &Timer{app: a}
	a.Sync(func() {
		t.timer = a.gui.NewTimer()
		t.timer.ConnectTimeout(fn)
		t.startOnUI(duration, once)
	})
	return t
}

// Start replaces the current run with periodic notifications, first after
// interval. Missed periods are skipped, not replayed. It preserves the callback.
// A non-positive interval panics without changing the previous run.
func (t *Timer) Start(interval time.Duration) { t.start(interval, false) }

// StartOnce replaces the current run with one notification after delay. The
// timer is inactive before the callback runs. A non-positive delay panics without
// changing the previous run. Calling it again restarts the full delay.
func (t *Timer) StartOnce(delay time.Duration) { t.start(delay, true) }

func (t *Timer) start(duration time.Duration, once bool) {
	checkTimerDuration(duration)
	if t == nil || t.app == nil {
		return
	}
	t.app.Sync(func() {
		if t.timer != nil {
			t.startOnUI(duration, once)
		}
	})
}

func (t *Timer) startOnUI(duration time.Duration, once bool) {
	// The factory guarantees a bound GUI timer and a non-nil callback; duration
	// was validated on the caller's goroutine. The remaining start failure is
	// application shutdown (which can race this call), a UI-layer no-op.
	if once {
		_ = t.timer.StartOnce(duration)
	} else {
		_ = t.timer.Start(duration)
	}
}

// Stop cancels pending notifications but does not interrupt a callback already
// running or forget the callback. It is idempotent and may be called from inside
// the callback; Start or StartOnce can subsequently reuse the timer.
func (t *Timer) Stop() {
	if t == nil || t.app == nil {
		return
	}
	t.app.Sync(func() {
		if t.timer != nil {
			t.timer.Stop()
		}
	})
}

// Active reports the current run's state on the UI thread. Periodic timers stay
// active inside their callbacks; one-shot timers do not. It is false after Quit.
func (t *Timer) Active() (active bool) {
	if t == nil || t.app == nil {
		return false
	}
	t.app.Sync(func() { active = t.timer != nil && t.timer.Active() })
	return
}

func checkTimerDuration(duration time.Duration) {
	if duration <= 0 {
		panic("ui: timer duration must be positive")
	}
}
