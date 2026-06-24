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

func point(x, y int32) geometry.Point {
	return geometry.Point{
		X: float32(x),
		Y: float32(y),
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
