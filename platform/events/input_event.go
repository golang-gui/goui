package events

import "github.com/golang-gui/goui/core/geometry"

type Modifiers uint16

const (
	ModifierShift Modifiers = 1 << iota
	ModifierControl
	ModifierAlt
	ModifierSuper
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

type KeyLocation uint8

const (
	KeyLocationStandard KeyLocation = iota
	KeyLocationLeft
	KeyLocationRight
	KeyLocationNumpad
)

type Key uint32

const (
	KeyUnknown Key = iota
	KeyEscape
	KeyEnter
	KeyTab
	KeyBackspace
	KeyDelete
	KeyInsert
	KeySpace
	KeyArrowLeft
	KeyArrowRight
	KeyArrowUp
	KeyArrowDown
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyShift
	KeyControl
	KeyAlt
	KeySuper
	KeyCapsLock
	KeyNumLock
	KeyPrintScreen
	KeyScrollLock
	KeyPause
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyF13
	KeyF14
	KeyF15
	KeyF16
	KeyF17
	KeyF18
	KeyF19
	KeyF20
	KeyF21
	KeyF22
	KeyF23
	KeyF24
	KeyA
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ
	Key0
	Key1
	Key2
	Key3
	Key4
	Key5
	Key6
	Key7
	Key8
	Key9
	KeyMinus
	KeyEqual
	KeyBracketLeft
	KeyBracketRight
	KeyBackslash
	KeySemicolon
	KeyQuote
	KeyComma
	KeyPeriod
	KeySlash
	KeyBackquote
	KeyNumpad0
	KeyNumpad1
	KeyNumpad2
	KeyNumpad3
	KeyNumpad4
	KeyNumpad5
	KeyNumpad6
	KeyNumpad7
	KeyNumpad8
	KeyNumpad9
	KeyNumpadAdd
	KeyNumpadSubtract
	KeyNumpadMultiply
	KeyNumpadDivide
	KeyNumpadDecimal
	KeyNumpadEnter
)

type KeyCode uint32

const KeyCodeUnknown KeyCode = 0

type KeyEvent struct {
	EventType EventType
	Key       Key
	Code      KeyCode
	Location  KeyLocation
	Modifiers Modifiers
	Repeat    bool
}

func (e KeyEvent) Type() EventType {
	return e.EventType
}

func (e KeyEvent) isEvent() {}

var (
	_ Event = PointerEvent{}
	_ Event = WheelEvent{}
	_ Event = KeyEvent{}
)
