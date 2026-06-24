package win32

import (
	"unsafe"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

func (w *Window) handlePointerMove(wParam winapi.WPARAM, lParam winapi.LPARAM) {
	position := clientPoint(lParam)
	buttons := pointerButtons(wParam)
	modifiers := pointerModifiers(wParam)
	if !w.trackingMouse {
		w.trackMouseLeave()
		w.emitPointer(events.PointerEnter, events.PointerButtonNone, position, buttons, modifiers)
	}
	w.emitPointer(events.PointerMove, events.PointerButtonNone, position, buttons, modifiers)
}

func (w *Window) handlePointerLeave() {
	w.trackingMouse = false
	position := geometry.Point{
		X: w.lastPointerX,
		Y: w.lastPointerY,
	}
	w.emitPointer(events.PointerLeave, events.PointerButtonNone, position, w.lastButtons, w.lastModifiers)
}

func (w *Window) handlePointerButton(eventType events.EventType, button events.PointerButton, wParam winapi.WPARAM, lParam winapi.LPARAM) {
	w.emitPointer(eventType, button, clientPoint(lParam), pointerButtons(wParam), pointerModifiers(wParam))
}

func (w *Window) handleWheel(horizontal bool, wParam winapi.WPARAM, lParam winapi.LPARAM) {
	position := screenPointToClient(w.hwnd, lParam)
	delta := float32(int16(highWord(uintptr(wParam)))) / float32(winapi.WHEEL_DELTA)
	event := events.WheelEvent{
		Position:  position,
		Mode:      events.WheelDeltaLine,
		Buttons:   pointerButtons(wParam),
		Modifiers: pointerModifiers(wParam),
	}
	if horizontal {
		event.DeltaX = delta
	} else {
		event.DeltaY = -delta
	}
	w.onEvent(event)
}

func (w *Window) handleKey(eventType events.EventType, wParam winapi.WPARAM, lParam winapi.LPARAM) {
	key, location := keyFromVirtualKey(int(wParam), lParam)
	w.updateKeyModifiers(eventType, key)
	w.onEvent(events.KeyEvent{
		EventType: eventType,
		Key:       key,
		Code:      events.KeyCodeUnknown,
		Location:  location,
		Modifiers: w.modifiers,
		Repeat:    eventType == events.KeyDown && (uintptr(lParam)&(1<<30)) != 0,
	})
}

func (w *Window) updateKeyModifiers(eventType events.EventType, key events.Key) {
	bit := modifierForKey(key)
	if bit == 0 {
		return
	}

	switch key {
	case events.KeyShift, events.KeyControl, events.KeyAlt, events.KeySuper:
		if eventType == events.KeyDown {
			w.modifiers |= bit
		} else {
			w.modifiers &^= bit
		}
	}
}

func (w *Window) emitPointer(eventType events.EventType, button events.PointerButton, position geometry.Point, buttons events.PointerButtons, modifiers events.Modifiers) {
	w.lastPointerX = position.X
	w.lastPointerY = position.Y
	w.lastButtons = buttons
	w.lastModifiers = modifiers
	w.onEvent(events.PointerEvent{
		EventType: eventType,
		Position:  position,
		Button:    button,
		Buttons:   buttons,
		Modifiers: modifiers,
	})
}

func (w *Window) trackMouseLeave() {
	event := winapi.TRACKMOUSEEVENT{
		Size:  winapi.DWORD(unsafe.Sizeof(winapi.TRACKMOUSEEVENT{})),
		Flags: winapi.TME_LEAVE,
		Track: w.hwnd,
	}
	if winapi.TrackMouseEvent(&event) != winapi.FALSE {
		w.trackingMouse = true
	}
}

func clientPoint(lParam winapi.LPARAM) geometry.Point {
	return geometry.Point{
		X: float32(int16(lowWord(uintptr(lParam)))),
		Y: float32(int16(highWord(uintptr(lParam)))),
	}
}

func screenPointToClient(hwnd winapi.HWND, lParam winapi.LPARAM) geometry.Point {
	point := winapi.POINT{
		X: winapi.LONG(int16(lowWord(uintptr(lParam)))),
		Y: winapi.LONG(int16(highWord(uintptr(lParam)))),
	}
	winapi.ScreenToClient(hwnd, &point)
	return geometry.Point{
		X: float32(point.X),
		Y: float32(point.Y),
	}
}

func xButton(wParam winapi.WPARAM) events.PointerButton {
	switch highWord(uintptr(wParam)) {
	case winapi.XBUTTON1:
		return events.PointerButtonBack
	case winapi.XBUTTON2:
		return events.PointerButtonForward
	default:
		return events.PointerButtonNone
	}
}

func pointerButtons(wParam winapi.WPARAM) events.PointerButtons {
	var buttons events.PointerButtons
	if wParam&winapi.MK_LBUTTON != 0 {
		buttons |= events.PointerButtonLeftDown
	}
	if wParam&winapi.MK_RBUTTON != 0 {
		buttons |= events.PointerButtonRightDown
	}
	if wParam&winapi.MK_MBUTTON != 0 {
		buttons |= events.PointerButtonMiddleDown
	}
	if wParam&winapi.MK_XBUTTON1 != 0 {
		buttons |= events.PointerButtonBackDown
	}
	if wParam&winapi.MK_XBUTTON2 != 0 {
		buttons |= events.PointerButtonForwardDown
	}
	return buttons
}

func pointerModifiers(wParam winapi.WPARAM) events.Modifiers {
	var mods events.Modifiers
	if wParam&winapi.MK_SHIFT != 0 {
		mods |= events.ModifierShift
	}
	if wParam&winapi.MK_CONTROL != 0 {
		mods |= events.ModifierControl
	}
	return mods
}

func modifierForKey(key events.Key) events.Modifiers {
	switch key {
	case events.KeyShift:
		return events.ModifierShift
	case events.KeyControl:
		return events.ModifierControl
	case events.KeyAlt:
		return events.ModifierAlt
	case events.KeySuper:
		return events.ModifierSuper
	default:
		return 0
	}
}

func keyFromVirtualKey(vk int, lParam winapi.LPARAM) (events.Key, events.KeyLocation) {
	extended := uintptr(lParam)&(1<<24) != 0
	scanCode := (uintptr(lParam) >> 16) & 0xFF

	if 'A' <= vk && vk <= 'Z' {
		return events.Key(int(events.KeyA) + vk - 'A'), events.KeyLocationStandard
	}
	if '0' <= vk && vk <= '9' {
		return events.Key(int(events.Key0) + vk - '0'), events.KeyLocationStandard
	}
	if winapi.VK_F1 <= vk && vk <= winapi.VK_F24 {
		return events.Key(int(events.KeyF1) + vk - winapi.VK_F1), events.KeyLocationStandard
	}
	if winapi.VK_NUMPAD0 <= vk && vk <= winapi.VK_NUMPAD9 {
		return events.Key(int(events.KeyNumpad0) + vk - winapi.VK_NUMPAD0), events.KeyLocationNumpad
	}

	switch vk {
	case winapi.VK_ESCAPE:
		return events.KeyEscape, events.KeyLocationStandard
	case winapi.VK_RETURN:
		if extended {
			return events.KeyNumpadEnter, events.KeyLocationNumpad
		}
		return events.KeyEnter, events.KeyLocationStandard
	case winapi.VK_TAB:
		return events.KeyTab, events.KeyLocationStandard
	case winapi.VK_BACK:
		return events.KeyBackspace, events.KeyLocationStandard
	case winapi.VK_DELETE:
		return events.KeyDelete, events.KeyLocationStandard
	case winapi.VK_INSERT:
		return events.KeyInsert, events.KeyLocationStandard
	case winapi.VK_SNAPSHOT:
		return events.KeyPrintScreen, events.KeyLocationStandard
	case winapi.VK_SPACE:
		return events.KeySpace, events.KeyLocationStandard
	case winapi.VK_PAUSE, winapi.VK_CANCEL:
		return events.KeyPause, events.KeyLocationStandard
	case winapi.VK_LEFT:
		return events.KeyArrowLeft, events.KeyLocationStandard
	case winapi.VK_RIGHT:
		return events.KeyArrowRight, events.KeyLocationStandard
	case winapi.VK_UP:
		return events.KeyArrowUp, events.KeyLocationStandard
	case winapi.VK_DOWN:
		return events.KeyArrowDown, events.KeyLocationStandard
	case winapi.VK_HOME:
		return events.KeyHome, events.KeyLocationStandard
	case winapi.VK_END:
		return events.KeyEnd, events.KeyLocationStandard
	case winapi.VK_PRIOR:
		return events.KeyPageUp, events.KeyLocationStandard
	case winapi.VK_NEXT:
		return events.KeyPageDown, events.KeyLocationStandard
	case winapi.VK_SHIFT:
		if scanCode == 0x36 {
			return events.KeyShift, events.KeyLocationRight
		}
		return events.KeyShift, events.KeyLocationLeft
	case winapi.VK_LSHIFT:
		return events.KeyShift, events.KeyLocationLeft
	case winapi.VK_RSHIFT:
		return events.KeyShift, events.KeyLocationRight
	case winapi.VK_CONTROL:
		if extended {
			return events.KeyControl, events.KeyLocationRight
		}
		return events.KeyControl, events.KeyLocationLeft
	case winapi.VK_LCONTROL:
		return events.KeyControl, events.KeyLocationLeft
	case winapi.VK_RCONTROL:
		return events.KeyControl, events.KeyLocationRight
	case winapi.VK_MENU:
		if extended {
			return events.KeyAlt, events.KeyLocationRight
		}
		return events.KeyAlt, events.KeyLocationLeft
	case winapi.VK_LMENU:
		return events.KeyAlt, events.KeyLocationLeft
	case winapi.VK_RMENU:
		return events.KeyAlt, events.KeyLocationRight
	case winapi.VK_LWIN:
		return events.KeySuper, events.KeyLocationLeft
	case winapi.VK_RWIN:
		return events.KeySuper, events.KeyLocationRight
	case winapi.VK_CAPITAL:
		return events.KeyCapsLock, events.KeyLocationStandard
	case winapi.VK_NUMLOCK:
		return events.KeyNumLock, events.KeyLocationNumpad
	case winapi.VK_SCROLL:
		return events.KeyScrollLock, events.KeyLocationStandard
	case winapi.VK_OEM_MINUS:
		return events.KeyMinus, events.KeyLocationStandard
	case winapi.VK_OEM_PLUS:
		return events.KeyEqual, events.KeyLocationStandard
	case winapi.VK_OEM_4:
		return events.KeyBracketLeft, events.KeyLocationStandard
	case winapi.VK_OEM_6:
		return events.KeyBracketRight, events.KeyLocationStandard
	case winapi.VK_OEM_5:
		return events.KeyBackslash, events.KeyLocationStandard
	case winapi.VK_OEM_1:
		return events.KeySemicolon, events.KeyLocationStandard
	case winapi.VK_OEM_7:
		return events.KeyQuote, events.KeyLocationStandard
	case winapi.VK_OEM_COMMA:
		return events.KeyComma, events.KeyLocationStandard
	case winapi.VK_OEM_PERIOD:
		return events.KeyPeriod, events.KeyLocationStandard
	case winapi.VK_OEM_2:
		return events.KeySlash, events.KeyLocationStandard
	case winapi.VK_OEM_3:
		return events.KeyBackquote, events.KeyLocationStandard
	case winapi.VK_ADD:
		return events.KeyNumpadAdd, events.KeyLocationNumpad
	case winapi.VK_SUBTRACT:
		return events.KeyNumpadSubtract, events.KeyLocationNumpad
	case winapi.VK_MULTIPLY:
		return events.KeyNumpadMultiply, events.KeyLocationNumpad
	case winapi.VK_DIVIDE:
		return events.KeyNumpadDivide, events.KeyLocationNumpad
	case winapi.VK_DECIMAL:
		return events.KeyNumpadDecimal, events.KeyLocationNumpad
	default:
		return events.KeyUnknown, events.KeyLocationStandard
	}
}

func lowWord(v uintptr) uint16 {
	return uint16(v & 0xFFFF)
}

func highWord(v uintptr) uint16 {
	return uint16((v >> 16) & 0xFFFF)
}
