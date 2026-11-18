package main

import (
	"fmt"
	"strings"

	"github.com/golang-gui/goui/platform/events"
)

func keyEventTypeName(typ events.EventType) string {
	switch typ {
	case events.KeyDown:
		return "KeyDown"
	case events.KeyUp:
		return "KeyUp"
	default:
		return fmt.Sprintf("EventType(%d)", typ)
	}
}

func keyLocationName(location events.KeyLocation) string {
	switch location {
	case events.KeyLocationStandard:
		return "Standard"
	case events.KeyLocationLeft:
		return "Left"
	case events.KeyLocationRight:
		return "Right"
	case events.KeyLocationNumpad:
		return "Numpad"
	default:
		return fmt.Sprintf("KeyLocation(%d)", location)
	}
}

func keyName(key events.Key) string {
	switch key {
	case events.KeyUnknown:
		return "Unknown"
	case events.KeyEscape:
		return "Escape"
	case events.KeyEnter:
		return "Enter"
	case events.KeyTab:
		return "Tab"
	case events.KeyBackspace:
		return "Backspace"
	case events.KeyDelete:
		return "Delete"
	case events.KeyInsert:
		return "Insert"
	case events.KeySpace:
		return "Space"
	case events.KeyArrowLeft:
		return "ArrowLeft"
	case events.KeyArrowRight:
		return "ArrowRight"
	case events.KeyArrowUp:
		return "ArrowUp"
	case events.KeyArrowDown:
		return "ArrowDown"
	case events.KeyHome:
		return "Home"
	case events.KeyEnd:
		return "End"
	case events.KeyPageUp:
		return "PageUp"
	case events.KeyPageDown:
		return "PageDown"
	case events.KeyShift:
		return "Shift"
	case events.KeyControl:
		return "Control"
	case events.KeyAlt:
		return "Alt"
	case events.KeySuper:
		return "Super"
	case events.KeyWin:
		return "Win"
	case events.KeyOption:
		return "Option"
	case events.KeyAltGraph:
		return "AltGraph"
	case events.KeyCommand:
		return "Command"
	case events.KeyCapsLock:
		return "CapsLock"
	case events.KeyNumLock:
		return "NumLock"
	case events.KeyPrintScreen:
		return "PrintScreen"
	case events.KeyScrollLock:
		return "ScrollLock"
	case events.KeyPause:
		return "Pause"
	case events.KeyF1:
		return "F1"
	case events.KeyF2:
		return "F2"
	case events.KeyF3:
		return "F3"
	case events.KeyF4:
		return "F4"
	case events.KeyF5:
		return "F5"
	case events.KeyF6:
		return "F6"
	case events.KeyF7:
		return "F7"
	case events.KeyF8:
		return "F8"
	case events.KeyF9:
		return "F9"
	case events.KeyF10:
		return "F10"
	case events.KeyF11:
		return "F11"
	case events.KeyF12:
		return "F12"
	case events.KeyF13:
		return "F13"
	case events.KeyF14:
		return "F14"
	case events.KeyF15:
		return "F15"
	case events.KeyF16:
		return "F16"
	case events.KeyF17:
		return "F17"
	case events.KeyF18:
		return "F18"
	case events.KeyF19:
		return "F19"
	case events.KeyF20:
		return "F20"
	case events.KeyF21:
		return "F21"
	case events.KeyF22:
		return "F22"
	case events.KeyF23:
		return "F23"
	case events.KeyF24:
		return "F24"
	case events.KeyA:
		return "A"
	case events.KeyB:
		return "B"
	case events.KeyC:
		return "C"
	case events.KeyD:
		return "D"
	case events.KeyE:
		return "E"
	case events.KeyF:
		return "F"
	case events.KeyG:
		return "G"
	case events.KeyH:
		return "H"
	case events.KeyI:
		return "I"
	case events.KeyJ:
		return "J"
	case events.KeyK:
		return "K"
	case events.KeyL:
		return "L"
	case events.KeyM:
		return "M"
	case events.KeyN:
		return "N"
	case events.KeyO:
		return "O"
	case events.KeyP:
		return "P"
	case events.KeyQ:
		return "Q"
	case events.KeyR:
		return "R"
	case events.KeyS:
		return "S"
	case events.KeyT:
		return "T"
	case events.KeyU:
		return "U"
	case events.KeyV:
		return "V"
	case events.KeyW:
		return "W"
	case events.KeyX:
		return "X"
	case events.KeyY:
		return "Y"
	case events.KeyZ:
		return "Z"
	case events.Key0:
		return "0"
	case events.Key1:
		return "1"
	case events.Key2:
		return "2"
	case events.Key3:
		return "3"
	case events.Key4:
		return "4"
	case events.Key5:
		return "5"
	case events.Key6:
		return "6"
	case events.Key7:
		return "7"
	case events.Key8:
		return "8"
	case events.Key9:
		return "9"
	case events.KeyMinus:
		return "Minus"
	case events.KeyEqual:
		return "Equal"
	case events.KeyBracketLeft:
		return "BracketLeft"
	case events.KeyBracketRight:
		return "BracketRight"
	case events.KeyBackslash:
		return "Backslash"
	case events.KeySemicolon:
		return "Semicolon"
	case events.KeyQuote:
		return "Quote"
	case events.KeyComma:
		return "Comma"
	case events.KeyPeriod:
		return "Period"
	case events.KeySlash:
		return "Slash"
	case events.KeyBackquote:
		return "Backquote"
	case events.KeyNumpad0:
		return "Numpad0"
	case events.KeyNumpad1:
		return "Numpad1"
	case events.KeyNumpad2:
		return "Numpad2"
	case events.KeyNumpad3:
		return "Numpad3"
	case events.KeyNumpad4:
		return "Numpad4"
	case events.KeyNumpad5:
		return "Numpad5"
	case events.KeyNumpad6:
		return "Numpad6"
	case events.KeyNumpad7:
		return "Numpad7"
	case events.KeyNumpad8:
		return "Numpad8"
	case events.KeyNumpad9:
		return "Numpad9"
	case events.KeyNumpadAdd:
		return "NumpadAdd"
	case events.KeyNumpadSubtract:
		return "NumpadSubtract"
	case events.KeyNumpadMultiply:
		return "NumpadMultiply"
	case events.KeyNumpadDivide:
		return "NumpadDivide"
	case events.KeyNumpadDecimal:
		return "NumpadDecimal"
	case events.KeyNumpadEnter:
		return "NumpadEnter"
	default:
		return fmt.Sprintf("Key(%d)", key)
	}
}

func pointerEventTypeName(typ events.EventType) string {
	switch typ {
	case events.PointerEnter:
		return "PointerEnter"
	case events.PointerLeave:
		return "PointerLeave"
	case events.PointerMove:
		return "PointerMove"
	case events.PointerDown:
		return "PointerDown"
	case events.PointerUp:
		return "PointerUp"
	default:
		return fmt.Sprintf("EventType(%d)", typ)
	}
}

func pointerButtonName(button events.PointerButton) string {
	switch button {
	case events.PointerButtonNone:
		return "None"
	case events.PointerButtonLeft:
		return "Left"
	case events.PointerButtonRight:
		return "Right"
	case events.PointerButtonMiddle:
		return "Middle"
	case events.PointerButtonBack:
		return "Back"
	case events.PointerButtonForward:
		return "Forward"
	default:
		return fmt.Sprintf("PointerButton(%d)", button)
	}
}

func pointerButtonsName(buttons events.PointerButtons) string {
	const known = events.PointerButtonLeftDown |
		events.PointerButtonRightDown |
		events.PointerButtonMiddleDown |
		events.PointerButtonBackDown |
		events.PointerButtonForwardDown

	names := make([]string, 0, 5)
	if buttons&events.PointerButtonLeftDown != 0 {
		names = append(names, "Left")
	}
	if buttons&events.PointerButtonRightDown != 0 {
		names = append(names, "Right")
	}
	if buttons&events.PointerButtonMiddleDown != 0 {
		names = append(names, "Middle")
	}
	if buttons&events.PointerButtonBackDown != 0 {
		names = append(names, "Back")
	}
	if buttons&events.PointerButtonForwardDown != 0 {
		names = append(names, "Forward")
	}
	if extra := buttons &^ known; extra != 0 {
		names = append(names, fmt.Sprintf("PointerButtons(0x%x)", uint16(extra)))
	}
	if len(names) == 0 {
		return "None"
	}
	return strings.Join(names, "|")
}

func modifiersName(modifiers events.Modifiers) string {
	const known = events.ModifierShift |
		events.ModifierControl |
		events.ModifierAlt |
		events.ModifierSuper |
		events.ModifierCommand |
		events.ModifierWin |
		events.ModifierOption |
		events.ModifierAltGraph

	names := make([]string, 0, 6)
	if modifiers&events.ModifierShift != 0 {
		names = append(names, "Shift")
	}
	if modifiers&events.ModifierControl != 0 {
		names = append(names, "Control")
	}
	if modifiers&events.ModifierAlt != 0 {
		names = append(names, "Alt")
	}
	if modifiers&events.ModifierSuper != 0 {
		names = append(names, "Super")
	}
	if modifiers&events.ModifierCommand != 0 {
		names = append(names, "Command")
	}
	if modifiers&events.ModifierWin != 0 {
		names = append(names, "Win")
	}
	if modifiers&events.ModifierOption != 0 {
		names = append(names, "Option")
	}
	if modifiers&events.ModifierAltGraph != 0 {
		names = append(names, "AltGraph")
	}
	if extra := modifiers &^ known; extra != 0 {
		names = append(names, fmt.Sprintf("Modifiers(0x%x)", uint16(extra)))
	}
	if len(names) == 0 {
		return "None"
	}
	return strings.Join(names, "|")
}

func wheelDeltaModeName(mode events.WheelDeltaMode) string {
	switch mode {
	case events.WheelDeltaPixel:
		return "Pixel"
	case events.WheelDeltaLine:
		return "Line"
	default:
		return fmt.Sprintf("WheelDeltaMode(%d)", mode)
	}
}
