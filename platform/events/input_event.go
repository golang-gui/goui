package events

import "github.com/golang-gui/goui/core/geometry"

type Modifiers uint16

const (
	ModifierShift Modifiers = 1 << iota
	ModifierControl
	ModifierAlt
	ModifierSuper
	ModifierCapsLock
	ModifierNumLock
)

type PointerButton uint8

const (
	PointerButtonNone PointerButton = iota
	PointerButtonLeft
	PointerButtonRight
	PointerButtonMiddle
	PointerButtonBack
	PointerButtonForward
)

type PointerButtons uint16

const (
	PointerButtonLeftDown PointerButtons = 1 << iota
	PointerButtonRightDown
	PointerButtonMiddleDown
	PointerButtonBackDown
	PointerButtonForwardDown
)

type PointerEvent struct {
	EventType EventType
	Position  geometry.Point
	Button    PointerButton
	Buttons   PointerButtons
	Modifiers Modifiers
}

func (e PointerEvent) Type() EventType {
	return e.EventType
}

func (e PointerEvent) isEvent() {}

var _ Event = PointerEvent{}
