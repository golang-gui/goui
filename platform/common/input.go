package common

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

// Key identifies a logical key, independently of a physical keyboard position.
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
	// KeyMenu is the dedicated context-menu key, when provided by the keyboard.
	KeyMenu
)
