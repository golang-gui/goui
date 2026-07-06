package win32

import (
	"github.com/golang-gui/goui/platform/internal/eventloop"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

const eventLoopWakeMessage = winapi.WM_APP

type EventLoop struct {
	state eventloop.State
}

func newEventLoop(p *Platform) (*EventLoop, error) {
	l := new(EventLoop)
	// Inject task draining into the helper window's wake handler so tasks run
	// under any message pump — the main loop or a nested modal loop (window
	// move/resize, menus, dialogs) — that dispatches the wake message.
	p.setWakeHandler(l.runTasks)
	return l, nil
}

func (l *EventLoop) Post(task func()) {
	if l.state.Post(task) {
		l.wake()
	}
}

func (l *EventLoop) Run() {
	defer l.state.Quit()
	if l.state.Destroyed() {
		return
	}

	for !l.state.Quitting() {
		var msg winapi.MSG
		result, _ := winapi.GetMessage(&msg, 0, 0, 0)
		if result == winapi.FALSE || result == -1 {
			break
		}

		winapi.TranslateMessage(&msg)
		winapi.DispatchMessage(&msg)
	}
	l.state.RunTasks()
}

func (l *EventLoop) Quit() {
	if l.state.Quit() {
		l.wake()
	}
}

func (l *EventLoop) Destroy() {
	l.state.Destroy()
}

func (l *EventLoop) wake() {
	// If the wake message cannot be queued, un-arm the pending flag so the next
	// Post requests a fresh wake instead of assuming one is already scheduled.
	if winapi.PostMessage(platform.helperWindow, eventLoopWakeMessage, 0, 0) != nil {
		l.state.WakeFailed()
	}
}

func (l *EventLoop) runTasks() {
	l.state.RunTasks()
}
