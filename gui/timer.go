package gui

import (
	"errors"
	"time"

	"github.com/golang-gui/goui/core/signal"
)

// Timer emits timeout notifications on its application's GUI thread. Create it
// with Application.NewTimer, TimeoutFunc or TickFunc. Its zero value cannot be
// started. All methods must be called on the GUI thread; use Application.Post
// from other goroutines. A Timer must not be copied after use.
//
// An active timer is retained by the scheduler. Call Stop when it is no longer
// needed; stopping preserves signal connections. Application exit stops all
// timers. Timer is for ordinary notifications, not frame-synchronized animation.
type Timer struct {
	app     *application
	timeout signal.Signal0
	run     *timerRun // application-managed handle; task contents belong to the scheduler
}

// ConnectTimeout connects a GUI-thread listener. Stop does not interrupt an
// already-started signal emission, including its remaining listeners.
func (t *Timer) ConnectTimeout(fn func()) signal.Handle { return t.timeout.Connect(fn) }

// Start starts periodic notifications, replacing any previous run. The interval
// must be positive. Deadlines follow a fixed cadence from this call; missed
// periods are skipped, not replayed. The next period is scheduled only after
// the current signal returns, so a periodic run does not reenter itself.
// An invalid interval leaves an existing run intact. A timer not created by an
// Application, or whose application has exited, cannot be started.
func (t *Timer) Start(interval time.Duration) error {
	if t.app == nil {
		return errors.New("timer: not created by an application")
	}
	return t.app.startTimer(t, interval, false)
}

// StartOnce replaces any previous run with one notification after a positive
// delay. The timer becomes inactive before its signal is emitted. Like Start,
// this may be called before Application.Run and never rebinds the application.
func (t *Timer) StartOnce(delay time.Duration) error {
	if t.app == nil {
		return errors.New("timer: not created by an application")
	}
	return t.app.startTimer(t, delay, true)
}

// Stop cancels the current run, including a notification not yet emitted. It is
// safe to call repeatedly or from a timeout listener. It does not disconnect
// listeners, interrupt the current emission, or affect other timers.
func (t *Timer) Stop() {
	if t.app != nil {
		t.app.stopTimer(t)
	}
}

// Active reports whether a run is active and its application has not exited.
// Periodic timers remain active during their timeout signal.
func (t *Timer) Active() bool { return t.app != nil && t.app.timerActive(t) }
