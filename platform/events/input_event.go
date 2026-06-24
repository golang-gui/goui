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

type WheelDeltaMode uint8

const (
	WheelDeltaPixel WheelDeltaMode = iota
	WheelDeltaLine
)

type WheelEvent struct {
	Position  geometry.Point
	DeltaX    float32
	DeltaY    float32
	Mode      WheelDeltaMode
	Buttons   PointerButtons
	Modifiers Modifiers
}

func (e WheelEvent) Type() EventType {
	return Wheel
}

func (e WheelEvent) isEvent() {}

var (
	_ Event = PointerEvent{}
	_ Event = WheelEvent{}
)
