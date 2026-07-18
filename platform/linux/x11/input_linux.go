package x11

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func (w *Window) handlePointerMove(event *xlib.MotionEvent) {
	w.buttons = buttonsFromState(event.State, w.buttons)
	w.onEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  point(event.X, event.Y),
		Button:    events.PointerButtonNone,
		Buttons:   w.buttons,
		Modifiers: modifiersFromState(event.State),
	})
}

func (w *Window) handlePointerCrossing(eventType events.EventType, event *xlib.CrossingEvent) {
	w.buttons = buttonsFromState(event.State, w.buttons)
	w.onEvent(events.PointerEvent{
		EventType: eventType,
		Position:  point(event.X, event.Y),
		Button:    events.PointerButtonNone,
		Buttons:   w.buttons,
		Modifiers: modifiersFromState(event.State),
	})
}

func (w *Window) handleButton(eventType events.EventType, event *xlib.ButtonEvent) {
	if event.Type == xlib.ButtonPress {
		if wheel, ok := wheelEvent(event, buttonsFromState(event.State, w.buttons)); ok {
			w.onEvent(wheel)
			return
		}
	}

	button := pointerButton(event.Button)
	if button == events.PointerButtonNone {
		return
	}

	buttons := buttonsFromState(event.State, w.buttons)
	buttonFlag := pointerButtonFlag(button)
	if eventType == events.PointerDown {
		buttons |= buttonFlag
	} else {
		buttons &^= buttonFlag
	}
	w.buttons = buttons

	w.onEvent(events.PointerEvent{
		EventType: eventType,
		Position:  point(event.X, event.Y),
		Button:    button,
		Buttons:   buttons,
		Modifiers: modifiersFromState(event.State),
	})
}

func (w *Window) handleKey(eventType events.EventType, event *xlib.KeyEvent) {
	// When a text widget is focused, run key-downs through the input method: it
	// turns them into committed text (handler.Commit) or, for keys it does not
	// consume, a plain KeyEvent (see doc/DesignIME.md §4). Key-ups and the
	// no-IME case take the plain path below.
	if eventType == events.KeyDown && w.im != nil && w.im.enabled {
		w.im.handleKey(event)
		return
	}

	key, location := keyFromKeysym(xlib.LookupKeysym(event, 0), event.State, platform.numLockMask)
	w.onEvent(events.KeyEvent{
		EventType: eventType,
		Key:       key,
		Code:      events.KeyCodeUnknown,
		Location:  location,
		Modifiers: keyModifiers(eventType, key, event.State),
		Repeat:    false,
	})
}

func wheelEvent(event *xlib.ButtonEvent, buttons events.PointerButtons) (events.WheelEvent, bool) {
	wheel := events.WheelEvent{
		Position:  point(event.X, event.Y),
		Mode:      events.WheelDeltaLine,
		Buttons:   buttons,
		Modifiers: modifiersFromState(event.State),
	}
	switch event.Button {
	case xlib.Button4:
		wheel.DeltaY = -1
	case xlib.Button5:
		wheel.DeltaY = 1
	case xlib.Button6:
		wheel.DeltaX = -1
	case xlib.Button7:
		wheel.DeltaX = 1
	default:
		return events.WheelEvent{}, false
	}
	return wheel, true
}

// point converts physical X11 pixel coordinates to logical (DIP) coordinates so
// pointer/wheel events share the gui layout coordinate space.
func point(x, y int32) geometry.Point {
	s := currentScale()
	return geometry.Point{
		X: float32(x) / s,
		Y: float32(y) / s,
	}
}

func pointerButton(button uint32) events.PointerButton {
	switch button {
	case xlib.Button1:
		return events.PointerButtonLeft
	case xlib.Button2:
		return events.PointerButtonMiddle
	case xlib.Button3:
		return events.PointerButtonRight
	case xlib.Button8:
		return events.PointerButtonBack
	case xlib.Button9:
		return events.PointerButtonForward
	default:
		return events.PointerButtonNone
	}
}

func pointerButtonFlag(button events.PointerButton) events.PointerButtons {
	switch button {
	case events.PointerButtonLeft:
		return events.PointerButtonLeftDown
	case events.PointerButtonRight:
		return events.PointerButtonRightDown
	case events.PointerButtonMiddle:
		return events.PointerButtonMiddleDown
	case events.PointerButtonBack:
		return events.PointerButtonBackDown
	case events.PointerButtonForward:
		return events.PointerButtonForwardDown
	default:
		return 0
	}
}

func buttonsFromState(state uint32, current events.PointerButtons) events.PointerButtons {
	buttons := current & (events.PointerButtonBackDown | events.PointerButtonForwardDown)
	if state&xlib.Button1Mask != 0 {
		buttons |= events.PointerButtonLeftDown
	}
	if state&xlib.Button2Mask != 0 {
		buttons |= events.PointerButtonMiddleDown
	}
	if state&xlib.Button3Mask != 0 {
		buttons |= events.PointerButtonRightDown
	}
	return buttons
}

func modifiersFromState(state uint32) events.Modifiers {
	var mods events.Modifiers
	if state&xlib.ShiftMask != 0 {
		mods |= events.ModifierShift
	}
	if state&xlib.ControlMask != 0 {
		mods |= events.ModifierControl
	}
	if state&xlib.Mod1Mask != 0 {
		mods |= events.ModifierAlt
	}
	return mods
}

func keyModifiers(eventType events.EventType, key events.Key, state uint32) events.Modifiers {
	mods := keyModifiersFromState(state)
	bit := modifierForKey(key)
	if bit == 0 {
		return mods
	}
	switch key {
	case events.KeyShift, events.KeyControl, events.KeyAlt, events.KeySuper:
		if eventType == events.KeyDown {
			mods |= bit
		} else {
			mods &^= bit
		}
	}
	return mods
}

func keyModifiersFromState(state uint32) events.Modifiers {
	mods := modifiersFromState(state)
	if state&xlib.Mod4Mask != 0 {
		mods |= events.ModifierSuper
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

func keyFromKeysym(keysym xlib.KeySym, state, numLockMask uint32) (events.Key, events.KeyLocation) {
	if 'a' <= keysym && keysym <= 'z' {
		return events.Key(int(events.KeyA) + int(keysym-'a')), events.KeyLocationStandard
	}
	if 'A' <= keysym && keysym <= 'Z' {
		return events.Key(int(events.KeyA) + int(keysym-'A')), events.KeyLocationStandard
	}
	if '0' <= keysym && keysym <= '9' {
		return events.Key(int(events.Key0) + int(keysym-'0')), events.KeyLocationStandard
	}
	if xlib.XK_F1 <= keysym && keysym <= xlib.XK_F24 {
		return events.Key(int(events.KeyF1) + int(keysym-xlib.XK_F1)), events.KeyLocationStandard
	}
	if xlib.XK_KP_0 <= keysym && keysym <= xlib.XK_KP_9 {
		return events.Key(int(events.KeyNumpad0) + int(keysym-xlib.XK_KP_0)), events.KeyLocationNumpad
	}

	switch keysym {
	case xlib.XK_Escape:
		return events.KeyEscape, events.KeyLocationStandard
	case xlib.XK_Print:
		return events.KeyPrintScreen, events.KeyLocationStandard
	case xlib.XK_Scroll_Lock:
		return events.KeyScrollLock, events.KeyLocationStandard
	case xlib.XK_Pause, xlib.XK_Break:
		return events.KeyPause, events.KeyLocationStandard
	case xlib.XK_Return:
		return events.KeyEnter, events.KeyLocationStandard
	case xlib.XK_KP_Enter:
		return events.KeyNumpadEnter, events.KeyLocationNumpad
	case xlib.XK_Tab:
		return events.KeyTab, events.KeyLocationStandard
	case xlib.XK_BackSpace:
		return events.KeyBackspace, events.KeyLocationStandard
	case xlib.XK_Delete:
		return events.KeyDelete, events.KeyLocationStandard
	case xlib.XK_Insert:
		return events.KeyInsert, events.KeyLocationStandard
	case xlib.XK_Left:
		return events.KeyArrowLeft, events.KeyLocationStandard
	case xlib.XK_Right:
		return events.KeyArrowRight, events.KeyLocationStandard
	case xlib.XK_Up:
		return events.KeyArrowUp, events.KeyLocationStandard
	case xlib.XK_Down:
		return events.KeyArrowDown, events.KeyLocationStandard
	case xlib.XK_Home:
		return events.KeyHome, events.KeyLocationStandard
	case xlib.XK_End:
		return events.KeyEnd, events.KeyLocationStandard
	case xlib.XK_Page_Up:
		return events.KeyPageUp, events.KeyLocationStandard
	case xlib.XK_Page_Down:
		return events.KeyPageDown, events.KeyLocationStandard
	case xlib.XK_KP_Home:
		return keypadNavigationKey(events.KeyHome, events.KeyNumpad7, state, numLockMask)
	case xlib.XK_KP_Up:
		return keypadNavigationKey(events.KeyArrowUp, events.KeyNumpad8, state, numLockMask)
	case xlib.XK_KP_Page_Up:
		return keypadNavigationKey(events.KeyPageUp, events.KeyNumpad9, state, numLockMask)
	case xlib.XK_KP_Left:
		return keypadNavigationKey(events.KeyArrowLeft, events.KeyNumpad4, state, numLockMask)
	case xlib.XK_KP_Begin:
		return events.KeyNumpad5, events.KeyLocationNumpad
	case xlib.XK_KP_Right:
		return keypadNavigationKey(events.KeyArrowRight, events.KeyNumpad6, state, numLockMask)
	case xlib.XK_KP_End:
		return keypadNavigationKey(events.KeyEnd, events.KeyNumpad1, state, numLockMask)
	case xlib.XK_KP_Down:
		return keypadNavigationKey(events.KeyArrowDown, events.KeyNumpad2, state, numLockMask)
	case xlib.XK_KP_Page_Down:
		return keypadNavigationKey(events.KeyPageDown, events.KeyNumpad3, state, numLockMask)
	case xlib.XK_KP_Insert:
		return keypadNavigationKey(events.KeyInsert, events.KeyNumpad0, state, numLockMask)
	case xlib.XK_KP_Delete:
		return keypadNavigationKey(events.KeyDelete, events.KeyNumpadDecimal, state, numLockMask)
	case xlib.XK_Shift_L:
		return events.KeyShift, events.KeyLocationLeft
	case xlib.XK_Shift_R:
		return events.KeyShift, events.KeyLocationRight
	case xlib.XK_Control_L:
		return events.KeyControl, events.KeyLocationLeft
	case xlib.XK_Control_R:
		return events.KeyControl, events.KeyLocationRight
	case xlib.XK_Alt_L:
		return events.KeyAlt, events.KeyLocationLeft
	case xlib.XK_Alt_R:
		return events.KeyAlt, events.KeyLocationRight
	case xlib.XK_Super_L:
		return events.KeySuper, events.KeyLocationLeft
	case xlib.XK_Super_R:
		return events.KeySuper, events.KeyLocationRight
	case xlib.XK_Caps_Lock:
		return events.KeyCapsLock, events.KeyLocationStandard
	case xlib.XK_Num_Lock:
		return events.KeyNumLock, events.KeyLocationNumpad
	case ' ':
		return events.KeySpace, events.KeyLocationStandard
	case '-':
		return events.KeyMinus, events.KeyLocationStandard
	case '=':
		return events.KeyEqual, events.KeyLocationStandard
	case '[':
		return events.KeyBracketLeft, events.KeyLocationStandard
	case ']':
		return events.KeyBracketRight, events.KeyLocationStandard
	case '\\':
		return events.KeyBackslash, events.KeyLocationStandard
	case ';':
		return events.KeySemicolon, events.KeyLocationStandard
	case '\'':
		return events.KeyQuote, events.KeyLocationStandard
	case ',':
		return events.KeyComma, events.KeyLocationStandard
	case '.':
		return events.KeyPeriod, events.KeyLocationStandard
	case '/':
		return events.KeySlash, events.KeyLocationStandard
	case '`':
		return events.KeyBackquote, events.KeyLocationStandard
	case xlib.XK_KP_Add:
		return events.KeyNumpadAdd, events.KeyLocationNumpad
	case xlib.XK_KP_Subtract:
		return events.KeyNumpadSubtract, events.KeyLocationNumpad
	case xlib.XK_KP_Multiply:
		return events.KeyNumpadMultiply, events.KeyLocationNumpad
	case xlib.XK_KP_Divide:
		return events.KeyNumpadDivide, events.KeyLocationNumpad
	case xlib.XK_KP_Decimal:
		return events.KeyNumpadDecimal, events.KeyLocationNumpad
	default:
		return events.KeyUnknown, events.KeyLocationStandard
	}
}

func keypadNavigationKey(navigationKey, numlockKey events.Key, state, numLockMask uint32) (events.Key, events.KeyLocation) {
	if numLockMask != 0 && state&numLockMask != 0 {
		return numlockKey, events.KeyLocationNumpad
	}
	return navigationKey, events.KeyLocationNumpad
}
