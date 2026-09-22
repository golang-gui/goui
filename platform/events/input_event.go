package events

import "github.com/golang-gui/goui/core/geometry"

// Modifiers reports aggregate native modifier state, without left/right sides.
// A modifier remains active while either side is held. KeyEvent.Location
// identifies the side of the key that changed, not the sides of this mask.
type Modifiers uint16

const (
	ModifierShift Modifiers = 1 << iota
	ModifierControl
	ModifierAlt
	// ModifierSuper is Linux Super.
	ModifierSuper
	// ModifierCommand is macOS Command, distinct from Win and Super.
	ModifierCommand
	// ModifierWin is the Windows logo key, distinct from Super and Command.
	ModifierWin
	// ModifierOption is macOS Option, distinct from Windows/Linux Alt.
	ModifierOption
	// ModifierAltGraph is X11's Level3 modifier, not a synthesized Ctrl+Alt.
	ModifierAltGraph
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
	// Append new keys to preserve existing numeric event/protocol values.
	KeyCommand
	// KeyWin is the Windows logo key. Location identifies its left/right side.
	KeyWin
	// KeyOption is macOS Option, distinct from Alt.
	KeyOption
	// KeyAltGraph is X11 ISO_Level3_Shift; its physical side is unspecified.
	KeyAltGraph
)

type KeyCode uint32

const KeyCodeUnknown KeyCode = 0

type KeyEvent struct {
	EventType EventType
	// Key identifies the logical key, not its physical US keyboard position.
	// A-Z follow the active layout and ignore letter case. Text and composed
	// characters are delivered separately by the input method. Unsupported
	// characters may report KeyUnknown; Code is reserved for physical keys.
	Key       Key
	Code      KeyCode
	Location  KeyLocation
	Modifiers Modifiers
	Repeat    bool
	// Handled is an optional synchronous response supplied by the native key
	// dispatcher. Set it only to true (or call PreventDefault) to prevent that
	// dispatch's remaining default key/text handling. It is not event data and
	// must not be retained for asynchronous replies. Synthetic events may omit
	// it. Stopping GUI propagation alone does not set this response.
	Handled *bool
}

// PreventDefault marks this native key dispatch as handled. It does not stop
// GUI propagation; callers that consume a shortcut must do both.
func (e KeyEvent) PreventDefault() {
	if e.Handled != nil {
		*e.Handled = true
	}
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
