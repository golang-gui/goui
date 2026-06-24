package cocoa

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

func updateTrackingAreas(self NSView) {
	if window := windowForView(self); window != nil {
		window.updateTrackingArea()
	}
}

func mouseEntered(self NSView, event NSEvent) {
	if window := windowForView(self); window != nil {
		window.emitPointer(events.PointerEnter, events.PointerButtonNone, event)
	}
}

func mouseExited(self NSView, event NSEvent) {
	if window := windowForView(self); window != nil {
		window.emitPointer(events.PointerLeave, events.PointerButtonNone, event)
	}
}

func mouseMoved(self NSView, event NSEvent) {
	if window := windowForView(self); window != nil {
		window.emitPointer(events.PointerMove, events.PointerButtonNone, event)
	}
}

func mouseDragged(self NSView, event NSEvent) {
	mouseMoved(self, event)
}

func rightMouseDragged(self NSView, event NSEvent) {
	mouseMoved(self, event)
}

func otherMouseDragged(self NSView, event NSEvent) {
	mouseMoved(self, event)
}

func mouseDown(self NSView, event NSEvent) {
	if window := windowForView(self); window != nil {
		window.emitPointer(events.PointerDown, events.PointerButtonLeft, event)
	}
}

func mouseUp(self NSView, event NSEvent) {
	if window := windowForView(self); window != nil {
		window.emitPointer(events.PointerUp, events.PointerButtonLeft, event)
	}
}

func rightMouseDown(self NSView, event NSEvent) {
	if window := windowForView(self); window != nil {
		window.emitPointer(events.PointerDown, events.PointerButtonRight, event)
	}
}

func rightMouseUp(self NSView, event NSEvent) {
	if window := windowForView(self); window != nil {
		window.emitPointer(events.PointerUp, events.PointerButtonRight, event)
	}
}

func otherMouseDown(self NSView, event NSEvent) {
	if window := windowForView(self); window != nil {
		window.emitPointer(events.PointerDown, pointerButton(event.ButtonNumber()), event)
	}
}

func otherMouseUp(self NSView, event NSEvent) {
	if window := windowForView(self); window != nil {
		window.emitPointer(events.PointerUp, pointerButton(event.ButtonNumber()), event)
	}
}

func scrollWheel(self NSView, event NSEvent) {
	if window := windowForView(self); window != nil {
		mode := events.WheelDeltaLine
		if event.HasPreciseScrollingDeltas() {
			mode = events.WheelDeltaPixel
		}
		window.onEvent(events.WheelEvent{
			Position:  positionInView(self, event),
			DeltaX:    float32(event.ScrollingDeltaX()),
			DeltaY:    -float32(event.ScrollingDeltaY()),
			Mode:      mode,
			Buttons:   window.buttons,
			Modifiers: modifiersFromFlags(event.ModifierFlags()),
		})
	}
}

func (w *Window) updateTrackingArea() {
	if !w.view.Valid() {
		return
	}
	if w.trackingArea.Valid() {
		w.view.RemoveTrackingArea(w.trackingArea)
		w.trackingArea = NSTrackingArea{}
	}
	options := NSTrackingMouseEnteredAndExited |
		NSTrackingMouseMoved |
		NSTrackingActiveAlways |
		NSTrackingInVisibleRect
	area := NSTrackingAreaClassId.Alloc().InitWith(w.view.Bounds(), options, w.view, NSObject{})
	w.view.AddTrackingArea(area)
	area.Release()
	w.trackingArea = area
}

func (w *Window) emitPointer(eventType events.EventType, button events.PointerButton, event NSEvent) {
	flag := pointerButtonFlag(button)
	if eventType == events.PointerDown {
		w.buttons |= flag
	} else if eventType == events.PointerUp {
		w.buttons &^= flag
	}
	w.onEvent(events.PointerEvent{
		EventType: eventType,
		Position:  positionInView(w.view, event),
		Button:    button,
		Buttons:   w.buttons,
		Modifiers: modifiersFromFlags(event.ModifierFlags()),
	})
}

func windowForView(view NSView) *Window {
	if window, ok := windowMap[view.Window()]; ok {
		return window
	}
	return nil
}

func positionInView(view NSView, event NSEvent) geometry.Point {
	point := view.ConvertPointFromView(event.LocationInWindow(), NSView{})
	bounds := view.Bounds()
	return geometry.Point{
		X: float32(point.X),
		Y: float32(bounds.Size.Height - point.Y),
	}
}

func pointerButton(button NSInteger) events.PointerButton {
	switch button {
	case 2:
		return events.PointerButtonMiddle
	case 3:
		return events.PointerButtonBack
	case 4:
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

func modifiersFromFlags(flags NSEventModifierFlags) events.Modifiers {
	var mods events.Modifiers
	if flags&NSEventModifierFlagShift != 0 {
		mods |= events.ModifierShift
	}
	if flags&NSEventModifierFlagControl != 0 {
		mods |= events.ModifierControl
	}
	if flags&NSEventModifierFlagOption != 0 {
		mods |= events.ModifierAlt
	}
	if flags&NSEventModifierFlagCommand != 0 {
		mods |= events.ModifierSuper
	}
	if flags&NSEventModifierFlagCapsLock != 0 {
		mods |= events.ModifierCapsLock
	}
	if flags&NSEventModifierFlagNumericPad != 0 {
		mods |= events.ModifierNumLock
	}
	return mods
}
