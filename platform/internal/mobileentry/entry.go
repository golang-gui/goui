// Package mobileentry coordinates native entry and the application Go thread.
package mobileentry

import (
	"runtime"
	"sync"
)

var process entryPoint

var enterThread func() func()

// RegisterMain registers the application's entry point. The mobile build tool
// generates this call in a main-package init function. It must be called exactly
// once with a non-nil function, before the native host requests startup.
func RegisterMain(main func()) { process.register(main) }

type entryPoint struct {
	mu      sync.Mutex
	main    func()
	started bool
}

func (e *entryPoint) register(main func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if main == nil || e.main != nil {
		panic("mobile: main must be registered exactly once and must not be nil")
	}
	e.main = main
}

// start never waits for main. All callers observe the same process entry point,
// including native callbacks which race or arrive after main has returned.
func (e *entryPoint) start() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.main == nil {
		return false
	}
	if e.started {
		return true
	}
	e.started = true
	main := e.main
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if enterThread != nil {
			leave := enterThread()
			defer leave()
		}
		main()
	}()
	return true
}

// NativeRun and NativeCreate are installed by platform packages during init.
// They receive native entry on the system thread; neither is a window protocol.
var NativeRun func() int
var NativeCreate func(activity, saved, size uintptr)

// SetThread attaches platform facilities to the application Go thread.
func SetThread(enter func() func()) { enterThread = enter }

// Start schedules application main once and never waits for it.
func Start() bool { return process.start() }

// Run enters a platform main or schedules the registered application directly.
func Run() int {
	if NativeRun != nil {
		return NativeRun()
	}
	if Start() {
		return 0
	}
	return -1
}

// Create receives Android's fixed native entry after package initialization.
func Create(activity, saved, size uintptr) {
	if NativeCreate == nil {
		panic("mobile: Android native entry is not registered")
	}
	NativeCreate(activity, saved, size)
}
