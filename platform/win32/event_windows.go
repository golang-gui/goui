package win32

import (
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/win32/winapi"
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

type EventQueue struct{}

func newEventQueue() (q EventQueue, err error) {
	return
}

func (q EventQueue) Destroy() {

}

func (q EventQueue) Post() {
	winapi.PostMessage(platform.helperWindow, winapi.WM_NULL, 0, 0)
}

func (q EventQueue) Poll() {
	var msg winapi.MSG
	for {
		if ok, _ := winapi.PeekMessage(&msg, 0, 0, 0, winapi.PM_REMOVE); ok != winapi.TRUE {
			break
		}
		if msg.Message != winapi.WM_QUIT {
			winapi.TranslateMessage(&msg)
			winapi.DispatchMessage(&msg)
		}
	}
}

func (q EventQueue) Wait() {
	if ok, _ := winapi.WaitMessage(); ok != winapi.FALSE {
		q.Poll()
	}
}
