package events

import (
	"github.com/golang-gui/goui/platform/common"
)

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

type SizeEvent struct {
	WindowEventBase
	Width  int
	Height int
}

func (e SizeEvent) Type() EventType {
	return Size
}

type PaintEvent struct {
	WindowEventBase
}

func (e PaintEvent) Type() EventType {
	return Paint
}
