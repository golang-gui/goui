package events

import "github.com/golang-gui/goui/platform/common"

type WindowEventBase struct {
	EventBase
	Window common.Window
	Native Event
}

type CloseEvent struct {
	WindowEventBase
}

func (e CloseEvent) Type() EventType {
	return Close
}
