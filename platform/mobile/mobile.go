// Package mobile contains the startup support used by mobile platform hosts.
// Applications use gui or ui; the build tool registers their ordinary main.
package mobile

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
