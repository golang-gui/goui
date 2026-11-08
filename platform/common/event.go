package common

// EventLoop coordinates native platform events and tasks on the UI thread.
//
// Desktop Run drives native events on the calling UI thread. Mobile Run waits
// on the startup goroutine while the system UI thread dispatches Post tasks and
// native callbacks; it must not block that system thread. Destroy follows Run
// on the same startup path and must not race other loop methods.
// Post and Quit may be called from any goroutine while the loop is alive.
// Destroy must not run concurrently with any other method.
type EventLoop interface {
	// Destroy releases resources owned by the event loop. Run must not be
	// active when it is called.
	Destroy()

	// Post schedules task for execution on the UI thread. Mobile startup may
	// enqueue tasks before Run; they wait until native dispatch starts.
	Post(task func())

	// Run starts dispatch and waits until Quit has finished admitted callbacks.
	Run()

	// Quit requests Run to return and wakes the native event loop.
	Quit()
}
