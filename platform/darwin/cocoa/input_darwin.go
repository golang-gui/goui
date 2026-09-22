package cocoa

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/darwin/frameworks/iokit"
	"github.com/golang-gui/goui/platform/events"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

// This borrowed NSEvent is usable only during its synchronous down/drag
// dispatch. Nested event/task dispatch expires it, even for another window.
type nativePress struct {
	window *Window
	event  NSEvent
	motion bool
}

var movePress nativePress

func (w *Window) emitEvent(event events.Event) {
	movePress = nativePress{}
	if w.window.Valid() {
		w.onEvent(event)
	}
}

func updateTrackingAreas(self NSView) {
	if window := windowForView(self); window != nil {
		window.updateTrackingArea()
	}
}

func mouseEntered(self NSView, event NSEvent) {
	if window := windowForView(self); window != nil {
		window.emitPointer(events.PointerEnter, events.PointerButtonNone, event)
		window.reapplyCursor()
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
		window.reapplyCursor()
	}
}

func mouseDragged(self NSView, event NSEvent) {
	if window := windowForView(self); window != nil && window.resizeRelease {
		window.trackResize()
		return
	}
	if window := windowForView(self); window != nil {
		window.emitPointerWithDrag(events.PointerMove, events.PointerButtonNone, event, true)
		window.reapplyCursor()
	}
}

func rightMouseDragged(self NSView, event NSEvent) {
	mouseMoved(self, event)
}

func otherMouseDragged(self NSView, event NSEvent) {
	mouseMoved(self, event)
}

func mouseDown(self NSView, event NSEvent) {
	self.Retain()
	defer self.Release()
	if window := windowForView(self); window != nil {
		window.resize = nil
		window.resizeRelease = false
		window.emitPointer(events.PointerDown, events.PointerButtonLeft, event)
	}
}

func mouseUp(self NSView, event NSEvent) {
	if window := windowForView(self); window != nil {
		if window.resizeRelease {
			window.trackResize()
			window.resize = nil
			window.resizeRelease = false
			return
		}
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
		dx, dy := event.ScrollingDeltaX(), -event.ScrollingDeltaY()
		if event.HasPreciseScrollingDeltas() {
			mode = events.WheelDeltaPixel
			scale := pointsPerLogicalUnit(self.Window())
			dx, dy = dx/scale, dy/scale
		}
		window.emitEvent(events.WheelEvent{
			Position:  positionInView(self, event),
			DeltaX:    float32(dx),
			DeltaY:    float32(dy),
			Mode:      mode,
			Buttons:   window.buttons,
			Modifiers: modifiersFromFlags(event.ModifierFlags()),
		})
	}
}

func keyDown(self NSView, event NSEvent) {
	window := windowForView(self)
	if window == nil {
		return
	}
	// While a text widget is focused, route through the input context: text
	// becomes insertText/setMarkedText (commit/preedit); other keys come back via
	// doCommandBySelector and are re-emitted as KeyEvents (see doc/DesignIME.md).
	if window.im != nil && window.im.enabled {
		window.im.interpret(self, event)
		return
	}
	window.emitKey(events.KeyDown, event, event.IsARepeat())
}

func keyUp(self NSView, event NSEvent) {
	if window := windowForView(self); window != nil {
		window.emitKey(events.KeyUp, event, false)
	}
}

func flagsChanged(self NSView, event NSEvent) {
	if window := windowForView(self); window != nil {
		flags := event.ModifierFlags()
		eventType, ok := modifierEventType(event.KeyCode(), flags)
		if !ok {
			return
		}
		window.emitKeyWithModifiers(eventType, event, modifiersFromFlags(flags), false)
	}
}

// flagsChanged reports the changed keycode and the resulting device flags.
// The aggregate Shift/Control/Option/Command flag cannot tell which side was
// released while the other is still down. No previous-window state is needed.
func modifierEventType(code uint16, flags NSEventModifierFlags) (events.EventType, bool) {
	var mask NSEventModifierFlags
	switch code {
	case 54:
		mask = iokit.NX_DEVICERCMDKEYMASK
	case 55:
		mask = iokit.NX_DEVICELCMDKEYMASK
	case 56:
		mask = iokit.NX_DEVICELSHIFTKEYMASK
	case 58:
		mask = iokit.NX_DEVICELALTKEYMASK
	case 59:
		mask = iokit.NX_DEVICELCTLKEYMASK
	case 60:
		mask = iokit.NX_DEVICERSHIFTKEYMASK
	case 61:
		mask = iokit.NX_DEVICERALTKEYMASK
	case 62:
		mask = iokit.NX_DEVICERCTLKEYMASK
	default:
		return 0, false // Preserve the existing treatment of non-momentary keys.
	}
	if flags&mask != 0 {
		return events.KeyDown, true
	}
	return events.KeyUp, true
}

func (w *Window) updateTrackingArea() {
	if !w.view.Valid() {
		return
	}
	if w.trackingArea.Valid() {
		w.view.RemoveTrackingArea(w.trackingArea)
		w.trackingArea = NSTrackingArea{}
	}
	// ActiveAlways preserves tracking in non-key popups and inactive windows;
	// InVisibleRect limits its geometry, independently of activation state.
	options := NSTrackingMouseEnteredAndExited |
		NSTrackingMouseMoved |
		NSTrackingActiveAlways |
		NSTrackingInVisibleRect
	area := NSTrackingAreaClassId.Alloc().InitWith(w.view.Bounds(), options, w.view, NSObject{})
	w.view.AddTrackingArea(area)
	area.Release()
	w.trackingArea = area
}

// reapplyCursor re-sets the window's current cursor after a pointer event, so
// it survives AppKit's own cursor resets during motion. No-op when the window
// has no cursor capability.
func (w *Window) reapplyCursor() {
	if w.cursor != nil {
		w.cursor.reapply()
	}
}

func (w *Window) emitPointer(eventType events.EventType, button events.PointerButton, event NSEvent) {
	w.emitPointerWithDrag(eventType, button, event, false)
}

func (w *Window) emitPointerWithDrag(eventType events.EventType, button events.PointerButton, event NSEvent, drag bool) {
	movePress = nativePress{}
	flag := pointerButtonFlag(button)
	if eventType == events.PointerDown {
		w.buttons |= flag
	} else if eventType == events.PointerUp {
		w.buttons &^= flag
	}
	pointer := events.PointerEvent{
		EventType: eventType,
		Position:  positionInView(w.view, event),
		Button:    button,
		Buttons:   w.buttons,
		Modifiers: modifiersFromFlags(event.ModifierFlags()),
	}
	if eventType == events.PointerDown && button == events.PointerButtonLeft {
		movePress = nativePress{window: w, event: event}
	} else if drag && w.buttons&events.PointerButtonLeftDown != 0 {
		movePress = nativePress{window: w, event: event, motion: true}
	}
	defer func() { movePress = nativePress{} }()
	w.onEvent(pointer)
}

func (w *Window) emitKey(eventType events.EventType, event NSEvent, repeat bool) {
	w.emitKeyWithModifiers(eventType, event, modifiersFromFlags(event.ModifierFlags()), repeat)
}

func (w *Window) emitKeyWithModifiers(eventType events.EventType, event NSEvent, modifiers events.Modifiers, repeat bool) {
	key, location := keyFromMacKeyCode(event.KeyCode())
	// flagsChanged is not a character-bearing event. AppKit raises an
	// exception if charactersIgnoringModifiers is read from it.
	if modifierForKey(key) == 0 {
		key = logicalMacKey(key, location, event.CharactersIgnoringModifiers().UTF8String())
	}
	var handled bool
	w.emitEvent(events.KeyEvent{
		EventType: eventType,
		Key:       key,
		Code:      events.KeyCodeUnknown,
		Location:  location,
		Modifiers: modifiers,
		Repeat:    repeat,
		Handled:   &handled,
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
	scale := pointsPerLogicalUnit(view.Window())
	return geometry.Point{
		X: float32((point.X - bounds.Origin.X) / scale),
		Y: float32((bounds.Origin.Y + bounds.Size.Height - point.Y) / scale),
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
		mods |= events.ModifierOption
	}
	if flags&NSEventModifierFlagCommand != 0 {
		mods |= events.ModifierCommand
	}
	return mods
}

func modifierForKey(key events.Key) events.Modifiers {
	switch key {
	case events.KeyShift:
		return events.ModifierShift
	case events.KeyControl:
		return events.ModifierControl
	case events.KeyOption:
		return events.ModifierOption
	case events.KeyCommand:
		return events.ModifierCommand
	default:
		return 0
	}
}

func keyFromMacKeyCode(code uint16) (events.Key, events.KeyLocation) {
	if key, ok := macKeyCodeMap[code]; ok {
		return key.key, key.location
	}
	return events.KeyUnknown, events.KeyLocationStandard
}

// Letter identities follow the active layout, not the US keycap at keyCode.
// AppKit ignores Control/Option here but preserves Shift; both letter cases
// identify the same key. Non-ASCII text remains IME input, not a guessed A-Z.
// Non-letter keys retain their existing identity (including keypad keys).
func logicalMacKey(physical events.Key, location events.KeyLocation, characters string) events.Key {
	if location == events.KeyLocationNumpad || physical < events.KeyA || physical > events.KeyBackquote {
		return physical
	}
	if len(characters) == 1 {
		c := characters[0]
		if c >= 'a' && c <= 'z' {
			return events.KeyA + events.Key(c-'a')
		}
		if c >= 'A' && c <= 'Z' {
			return events.KeyA + events.Key(c-'A')
		}
	}
	if physical >= events.KeyA && physical <= events.KeyZ {
		return events.KeyUnknown
	}
	return physical
}

type macKey struct {
	key      events.Key
	location events.KeyLocation
}

var macKeyCodeMap = map[uint16]macKey{
	0:   {events.KeyA, events.KeyLocationStandard},
	1:   {events.KeyS, events.KeyLocationStandard},
	2:   {events.KeyD, events.KeyLocationStandard},
	3:   {events.KeyF, events.KeyLocationStandard},
	4:   {events.KeyH, events.KeyLocationStandard},
	5:   {events.KeyG, events.KeyLocationStandard},
	6:   {events.KeyZ, events.KeyLocationStandard},
	7:   {events.KeyX, events.KeyLocationStandard},
	8:   {events.KeyC, events.KeyLocationStandard},
	9:   {events.KeyV, events.KeyLocationStandard},
	11:  {events.KeyB, events.KeyLocationStandard},
	12:  {events.KeyQ, events.KeyLocationStandard},
	13:  {events.KeyW, events.KeyLocationStandard},
	14:  {events.KeyE, events.KeyLocationStandard},
	15:  {events.KeyR, events.KeyLocationStandard},
	16:  {events.KeyY, events.KeyLocationStandard},
	17:  {events.KeyT, events.KeyLocationStandard},
	18:  {events.Key1, events.KeyLocationStandard},
	19:  {events.Key2, events.KeyLocationStandard},
	20:  {events.Key3, events.KeyLocationStandard},
	21:  {events.Key4, events.KeyLocationStandard},
	22:  {events.Key6, events.KeyLocationStandard},
	23:  {events.Key5, events.KeyLocationStandard},
	24:  {events.KeyEqual, events.KeyLocationStandard},
	25:  {events.Key9, events.KeyLocationStandard},
	26:  {events.Key7, events.KeyLocationStandard},
	27:  {events.KeyMinus, events.KeyLocationStandard},
	28:  {events.Key8, events.KeyLocationStandard},
	29:  {events.Key0, events.KeyLocationStandard},
	30:  {events.KeyBracketRight, events.KeyLocationStandard},
	31:  {events.KeyO, events.KeyLocationStandard},
	32:  {events.KeyU, events.KeyLocationStandard},
	33:  {events.KeyBracketLeft, events.KeyLocationStandard},
	34:  {events.KeyI, events.KeyLocationStandard},
	35:  {events.KeyP, events.KeyLocationStandard},
	36:  {events.KeyEnter, events.KeyLocationStandard},
	37:  {events.KeyL, events.KeyLocationStandard},
	38:  {events.KeyJ, events.KeyLocationStandard},
	39:  {events.KeyQuote, events.KeyLocationStandard},
	40:  {events.KeyK, events.KeyLocationStandard},
	41:  {events.KeySemicolon, events.KeyLocationStandard},
	42:  {events.KeyBackslash, events.KeyLocationStandard},
	43:  {events.KeyComma, events.KeyLocationStandard},
	44:  {events.KeySlash, events.KeyLocationStandard},
	45:  {events.KeyN, events.KeyLocationStandard},
	46:  {events.KeyM, events.KeyLocationStandard},
	47:  {events.KeyPeriod, events.KeyLocationStandard},
	48:  {events.KeyTab, events.KeyLocationStandard},
	49:  {events.KeySpace, events.KeyLocationStandard},
	50:  {events.KeyBackquote, events.KeyLocationStandard},
	51:  {events.KeyBackspace, events.KeyLocationStandard},
	53:  {events.KeyEscape, events.KeyLocationStandard},
	54:  {events.KeyCommand, events.KeyLocationRight},
	55:  {events.KeyCommand, events.KeyLocationLeft},
	56:  {events.KeyShift, events.KeyLocationLeft},
	57:  {events.KeyCapsLock, events.KeyLocationStandard},
	58:  {events.KeyOption, events.KeyLocationLeft},
	59:  {events.KeyControl, events.KeyLocationLeft},
	60:  {events.KeyShift, events.KeyLocationRight},
	61:  {events.KeyOption, events.KeyLocationRight},
	62:  {events.KeyControl, events.KeyLocationRight},
	64:  {events.KeyF17, events.KeyLocationStandard},
	65:  {events.KeyNumpadDecimal, events.KeyLocationNumpad},
	67:  {events.KeyNumpadMultiply, events.KeyLocationNumpad},
	69:  {events.KeyNumpadAdd, events.KeyLocationNumpad},
	75:  {events.KeyNumpadDivide, events.KeyLocationNumpad},
	76:  {events.KeyNumpadEnter, events.KeyLocationNumpad},
	78:  {events.KeyNumpadSubtract, events.KeyLocationNumpad},
	79:  {events.KeyF18, events.KeyLocationStandard},
	80:  {events.KeyF19, events.KeyLocationStandard},
	82:  {events.KeyNumpad0, events.KeyLocationNumpad},
	83:  {events.KeyNumpad1, events.KeyLocationNumpad},
	84:  {events.KeyNumpad2, events.KeyLocationNumpad},
	85:  {events.KeyNumpad3, events.KeyLocationNumpad},
	86:  {events.KeyNumpad4, events.KeyLocationNumpad},
	87:  {events.KeyNumpad5, events.KeyLocationNumpad},
	88:  {events.KeyNumpad6, events.KeyLocationNumpad},
	89:  {events.KeyNumpad7, events.KeyLocationNumpad},
	91:  {events.KeyNumpad8, events.KeyLocationNumpad},
	92:  {events.KeyNumpad9, events.KeyLocationNumpad},
	96:  {events.KeyF5, events.KeyLocationStandard},
	97:  {events.KeyF6, events.KeyLocationStandard},
	98:  {events.KeyF7, events.KeyLocationStandard},
	99:  {events.KeyF3, events.KeyLocationStandard},
	100: {events.KeyF8, events.KeyLocationStandard},
	101: {events.KeyF9, events.KeyLocationStandard},
	103: {events.KeyF11, events.KeyLocationStandard},
	105: {events.KeyF13, events.KeyLocationStandard},
	106: {events.KeyF16, events.KeyLocationStandard},
	107: {events.KeyF14, events.KeyLocationStandard},
	109: {events.KeyF10, events.KeyLocationStandard},
	111: {events.KeyF12, events.KeyLocationStandard},
	113: {events.KeyF15, events.KeyLocationStandard},
	114: {events.KeyInsert, events.KeyLocationStandard},
	115: {events.KeyHome, events.KeyLocationStandard},
	116: {events.KeyPageUp, events.KeyLocationStandard},
	117: {events.KeyDelete, events.KeyLocationStandard},
	118: {events.KeyF4, events.KeyLocationStandard},
	119: {events.KeyEnd, events.KeyLocationStandard},
	120: {events.KeyF2, events.KeyLocationStandard},
	121: {events.KeyPageDown, events.KeyLocationStandard},
	122: {events.KeyF1, events.KeyLocationStandard},
	123: {events.KeyArrowLeft, events.KeyLocationStandard},
	124: {events.KeyArrowRight, events.KeyLocationStandard},
	125: {events.KeyArrowDown, events.KeyLocationStandard},
	126: {events.KeyArrowUp, events.KeyLocationStandard},
}
