package android

import (
	"fmt"
	"sync/atomic"
	"syscall"

	"github.com/goexlib/cgo"
	native "github.com/golang-gui/goui/platform/android/libs/android"
	"github.com/golang-gui/goui/platform/internal/eventfd"
	"github.com/golang-gui/goui/platform/internal/mobile"
)

// EventLoop connects the shared Go queue to Android's process-lived dispatcher.
type EventLoop struct {
	*mobile.EventLoop
	dispatcher *nativeDispatcher
}

// nativeDispatcher survives Activity replacement. It owns the wakeup; EventLoop
// owns the Go queue and unbinds it after Run returns.
type nativeDispatcher struct {
	wakeup   *eventfd.EventFD
	callback uintptr
	thread   atomic.Int64
	loop     atomic.Pointer[mobile.EventLoop]
}

func (d *nativeDispatcher) initialize() error {
	wakeup, err := eventfd.New()
	if err != nil {
		return fmt.Errorf("android: create dispatcher wakeup: %w", err)
	}
	d.wakeup = wakeup
	d.callback = cgo.NewCallback(d.onWake)
	if native.LooperAddFd(native.LooperForThread(), int32(wakeup.FD()), native.LooperPollCallback, native.LooperEventInput, d.callback, 0) < 0 {
		_ = wakeup.Close()
		d.wakeup = nil
		return fmt.Errorf("android: register native dispatcher")
	}
	d.thread.Store(int64(syscall.Gettid()))
	return nil
}

func newEventLoop(d *nativeDispatcher) (*EventLoop, error) {
	loop := mobile.NewEventLoop(d.wake, d.isCurrent)
	if !d.loop.CompareAndSwap(nil, loop) {
		return nil, fmt.Errorf("android: dispatcher already has an event loop")
	}
	return &EventLoop{EventLoop: loop, dispatcher: d}, nil
}
func (l *EventLoop) Destroy() {
	l.EventLoop.Destroy()
	l.dispatcher.loop.CompareAndSwap(l.EventLoop, nil)
}
func (d *nativeDispatcher) isCurrent() bool { return d.thread.Load() == int64(syscall.Gettid()) }
func (d *nativeDispatcher) wake() {
	if err := d.wakeup.Signal(); err != nil {
		panic(fmt.Errorf("android: wake main looper: %w", err))
	}
}
func (d *nativeDispatcher) onWake(_ int32, _ int32, _ uintptr) int32 {
	withNativeThread(func() {
		if err := d.wakeup.Drain(); err != nil {
			panic(fmt.Errorf("android: drain main looper wakeup: %w", err))
		}
		if loop := d.loop.Load(); loop != nil {
			loop.Dispatch()
		}
	})
	return 1
}
