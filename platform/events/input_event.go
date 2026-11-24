package events

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
)

// Key is an alias of the shared logical key type.
type Key = common.Key

// Modifiers is an alias of the shared native modifier mask.
type Modifiers = common.Modifiers

const (
	ModifierShift    = common.ModifierShift
	ModifierControl  = common.ModifierControl
	ModifierAlt      = common.ModifierAlt
	ModifierSuper    = common.ModifierSuper
	ModifierCommand  = common.ModifierCommand
	ModifierWin      = common.ModifierWin
	ModifierOption   = common.ModifierOption
	ModifierAltGraph = common.ModifierAltGraph
)

const (
	KeyUnknown        = common.KeyUnknown
	KeyEscape         = common.KeyEscape
	KeyEnter          = common.KeyEnter
	KeyTab            = common.KeyTab
	KeyBackspace      = common.KeyBackspace
	KeyDelete         = common.KeyDelete
	KeyInsert         = common.KeyInsert
	KeySpace          = common.KeySpace
	KeyArrowLeft      = common.KeyArrowLeft
	KeyArrowRight     = common.KeyArrowRight
	KeyArrowUp        = common.KeyArrowUp
	KeyArrowDown      = common.KeyArrowDown
	KeyHome           = common.KeyHome
	KeyEnd            = common.KeyEnd
	KeyPageUp         = common.KeyPageUp
	KeyPageDown       = common.KeyPageDown
	KeyShift          = common.KeyShift
	KeyControl        = common.KeyControl
	KeyAlt            = common.KeyAlt
	KeySuper          = common.KeySuper
	KeyCapsLock       = common.KeyCapsLock
	KeyNumLock        = common.KeyNumLock
	KeyPrintScreen    = common.KeyPrintScreen
	KeyScrollLock     = common.KeyScrollLock
	KeyPause          = common.KeyPause
	KeyF1             = common.KeyF1
	KeyF2             = common.KeyF2
	KeyF3             = common.KeyF3
	KeyF4             = common.KeyF4
	KeyF5             = common.KeyF5
	KeyF6             = common.KeyF6
	KeyF7             = common.KeyF7
	KeyF8             = common.KeyF8
	KeyF9             = common.KeyF9
	KeyF10            = common.KeyF10
	KeyF11            = common.KeyF11
	KeyF12            = common.KeyF12
	KeyF13            = common.KeyF13
	KeyF14            = common.KeyF14
	KeyF15            = common.KeyF15
	KeyF16            = common.KeyF16
	KeyF17            = common.KeyF17
	KeyF18            = common.KeyF18
	KeyF19            = common.KeyF19
	KeyF20            = common.KeyF20
	KeyF21            = common.KeyF21
	KeyF22            = common.KeyF22
	KeyF23            = common.KeyF23
	KeyF24            = common.KeyF24
	KeyA              = common.KeyA
	KeyB              = common.KeyB
	KeyC              = common.KeyC
	KeyD              = common.KeyD
	KeyE              = common.KeyE
	KeyF              = common.KeyF
	KeyG              = common.KeyG
	KeyH              = common.KeyH
	KeyI              = common.KeyI
	KeyJ              = common.KeyJ
	KeyK              = common.KeyK
	KeyL              = common.KeyL
	KeyM              = common.KeyM
	KeyN              = common.KeyN
	KeyO              = common.KeyO
	KeyP              = common.KeyP
	KeyQ              = common.KeyQ
	KeyR              = common.KeyR
	KeyS              = common.KeyS
	KeyT              = common.KeyT
	KeyU              = common.KeyU
	KeyV              = common.KeyV
	KeyW              = common.KeyW
	KeyX              = common.KeyX
	KeyY              = common.KeyY
	KeyZ              = common.KeyZ
	Key0              = common.Key0
	Key1              = common.Key1
	Key2              = common.Key2
	Key3              = common.Key3
	Key4              = common.Key4
	Key5              = common.Key5
	Key6              = common.Key6
	Key7              = common.Key7
	Key8              = common.Key8
	Key9              = common.Key9
	KeyMinus          = common.KeyMinus
	KeyEqual          = common.KeyEqual
	KeyBracketLeft    = common.KeyBracketLeft
	KeyBracketRight   = common.KeyBracketRight
	KeyBackslash      = common.KeyBackslash
	KeySemicolon      = common.KeySemicolon
	KeyQuote          = common.KeyQuote
	KeyComma          = common.KeyComma
	KeyPeriod         = common.KeyPeriod
	KeySlash          = common.KeySlash
	KeyBackquote      = common.KeyBackquote
	KeyNumpad0        = common.KeyNumpad0
	KeyNumpad1        = common.KeyNumpad1
	KeyNumpad2        = common.KeyNumpad2
	KeyNumpad3        = common.KeyNumpad3
	KeyNumpad4        = common.KeyNumpad4
	KeyNumpad5        = common.KeyNumpad5
	KeyNumpad6        = common.KeyNumpad6
	KeyNumpad7        = common.KeyNumpad7
	KeyNumpad8        = common.KeyNumpad8
	KeyNumpad9        = common.KeyNumpad9
	KeyNumpadAdd      = common.KeyNumpadAdd
	KeyNumpadSubtract = common.KeyNumpadSubtract
	KeyNumpadMultiply = common.KeyNumpadMultiply
	KeyNumpadDivide   = common.KeyNumpadDivide
	KeyNumpadDecimal  = common.KeyNumpadDecimal
	KeyNumpadEnter    = common.KeyNumpadEnter
	KeyCommand        = common.KeyCommand
	KeyWin            = common.KeyWin
	KeyOption         = common.KeyOption
	KeyAltGraph       = common.KeyAltGraph
	KeyMenu           = common.KeyMenu
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
