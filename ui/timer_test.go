package ui

import (
	"runtime"
	"testing"
	"time"
)

func timerPanic(fn func()) (value any) {
	defer func() { value = recover() }()
	fn()
	return
}

func TestTimerInvalidArguments(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// This fake panics on NewTimer: invalid arguments must be rejected before
	// calling the GUI factory or posting work, on the caller's goroutine.
	a := newApp(newWindowTestApplication(), nil)
	defer a.stop()
	for _, duration := range []time.Duration{0, -time.Second} {
		for _, factory := range []func(time.Duration, func()) *Timer{a.TimeoutFunc, a.TickFunc} {
			if got := timerPanic(func() { factory(duration, func() {}) }); got != "ui: timer duration must be positive" {
				t.Fatalf("unexpected duration panic: %v", got)
			}
		}
		var timer *Timer
		if timerPanic(func() { timer.Start(duration) }) != "ui: timer duration must be positive" ||
			timerPanic(func() { timer.StartOnce(duration) }) != "ui: timer duration must be positive" {
			t.Fatal("invalid restart duration did not panic")
		}
	}
	for _, factory := range []func(time.Duration, func()) *Timer{a.TimeoutFunc, a.TickFunc} {
		if got := timerPanic(func() { factory(time.Second, nil) }); got != "ui: timer callback must not be nil" {
			t.Fatalf("unexpected callback panic: %v", got)
		}
	}
}

func TestTimerInert(t *testing.T) {
	a := newApp(newWindowTestApplication(), nil)
	a.Quit()
	a.stop()
	once, periodic := a.TimeoutFunc(time.Second, func() {}), a.TickFunc(time.Second, func() {})
	if once == nil || periodic == nil {
		t.Fatal("exited app returned a nil timer")
	}
	for _, timer := range []*Timer{nil, {}, once, periodic} {
		timer.Stop()
		timer.Start(time.Second)
		timer.StartOnce(time.Second)
		timer.Stop()
		if timer.Active() {
			t.Fatal("inert timer became active")
		}
	}
}
