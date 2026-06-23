package common

// EventLoop runs native platform events and tasks posted to its owning thread.
//
// Run and Destroy must be called on the thread that owns the platform.
// Post and Quit may be called from any goroutine while the loop is alive.
// Destroy must not run concurrently with any other method.
type EventLoop interface {
	// Destroy releases resources owned by the event loop. Run must not be
	// active when it is called.
	Destroy()

	// Post schedules task for execution on the owning thread.
	Post(task func())

	// Run processes native events and posted tasks until Quit is requested.
	Run()

	// Quit requests Run to return and wakes the native event loop.
	Quit()
}
