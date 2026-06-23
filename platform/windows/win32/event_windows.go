package win32

import (
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/internal/eventloop"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

type Event struct {
	events.EventBase
	Hwnd    winapi.HWND
	WParam  winapi.WPARAM
	LParam  winapi.LPARAM
	Result  winapi.LRESULT
	Message winapi.UINT
}

func (e *Event) Type() events.EventType {
	return events.Native
}

const eventLoopWakeMessage = winapi.WM_APP

type EventLoop struct {
	state eventloop.State
}

func newEventLoop() (*EventLoop, error) {
	return new(EventLoop), nil
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
			return
		}

		if msg.Hwnd == platform.helperWindow && msg.Message == eventLoopWakeMessage {
			eventloop.RunTasks(&l.state)
			continue
		}

		winapi.TranslateMessage(&msg)
		winapi.DispatchMessage(&msg)
	}
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
	_ = winapi.PostMessage(platform.helperWindow, eventLoopWakeMessage, 0, 0)
}
