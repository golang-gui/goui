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

func lowWord(v uintptr) uint16 {
	return uint16(v & 0xFFFF)
}

func highWord(v uintptr) uint16 {
	return uint16((v >> 16) & 0xFFFF)
}
