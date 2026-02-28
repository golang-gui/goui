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
	Width  uint
	Height uint
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

type ScaleEvent struct {
	WindowEventBase
	ScaleFactor float64
}

func (e ScaleEvent) Type() EventType {
	return Scale
}
