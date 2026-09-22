package win32

import (
	"github.com/golang-gui/goui/platform/events"
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

		dispatchMessage(&msg)
	}
	l.state.RunTasks()
}

// TranslateMessage queues WM_CHAR/WM_DEADCHAR, so application key handling
// must run first. Process ordinary GOUI key messages exactly once here; the
// window procedure still handles messages delivered by native nested loops.
// IME-owned VK_PROCESSKEY messages stay on the normal Windows dispatch path.
func dispatchMessage(msg *winapi.MSG) {
	switch msg.Message {
	case winapi.WM_KEYDOWN, winapi.WM_SYSKEYDOWN, winapi.WM_KEYUP, winapi.WM_SYSKEYUP:
		if w := windowMap[msg.Hwnd]; w != nil && msg.WParam != winapi.VK_PROCESSKEY {
			var kind events.EventType
			if msg.Message == winapi.WM_KEYDOWN || msg.Message == winapi.WM_SYSKEYDOWN {
				kind = events.KeyDown
			} else {
				kind = events.KeyUp
			}
			if w.handleKey(kind, msg.WParam, msg.LParam) || w.hwnd == 0 {
				return
			}
			winapi.TranslateMessage(msg)
			if w.integrated && (msg.Message == winapi.WM_SYSKEYDOWN || msg.Message == winapi.WM_SYSKEYUP) {
				winapi.DefWindowProc(msg.Hwnd, msg.Message, msg.WParam, msg.LParam)
			}
			return
		}
	}
	winapi.TranslateMessage(msg)
	winapi.DispatchMessage(msg)
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
