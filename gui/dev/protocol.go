package dev

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
)

var (
	errWindowRequired    = errors.New("event window is required")
	errEventTypeRequired = errors.New("event type is required")
)

type eventRequest struct {
	Window    string          `json:"window"`
	Type      string          `json:"type"`
	EventType *int            `json:"eventType,omitempty"`
	X         float32         `json:"x"`
	Y         float32         `json:"y"`
	Button    json.RawMessage `json:"button,omitempty"`
	Buttons   json.RawMessage `json:"buttons,omitempty"`
	Modifiers json.RawMessage `json:"modifiers,omitempty"`
	DeltaX    float32         `json:"deltaX,omitempty"`
	DeltaY    float32         `json:"deltaY,omitempty"`
	Mode      string          `json:"mode,omitempty"`
	Key       json.RawMessage `json:"key,omitempty"`
	Code      uint32          `json:"code,omitempty"`
	Location  string          `json:"location,omitempty"`
	Repeat    bool            `json:"repeat,omitempty"`
}

func (r eventRequest) event() (events.Event, error) {
	eventType, err := parseEventType(r.Type, r.EventType)
	if err != nil {
		return nil, err
	}

	modifiers, err := parseModifiers(r.Modifiers)
	if err != nil {
		return nil, err
	}

	switch eventType {
	case events.PointerEnter, events.PointerLeave, events.PointerMove, events.PointerDown, events.PointerUp:
		button, err := parsePointerButton(r.Button)
		if err != nil {
			return nil, err
		}
		buttons, err := parsePointerButtons(r.Buttons)
		if err != nil {
			return nil, err
		}
		return events.PointerEvent{
			EventType: eventType,
			Position:  geometry.Point{X: r.X, Y: r.Y},
			Button:    button,
			Buttons:   buttons,
			Modifiers: modifiers,
		}, nil
	case events.Wheel:
		buttons, err := parsePointerButtons(r.Buttons)
		if err != nil {
			return nil, err
		}
		mode, err := parseWheelDeltaMode(r.Mode)
		if err != nil {
			return nil, err
		}
		return events.WheelEvent{
			Position:  geometry.Point{X: r.X, Y: r.Y},
			DeltaX:    r.DeltaX,
			DeltaY:    r.DeltaY,
			Mode:      mode,
			Buttons:   buttons,
			Modifiers: modifiers,
		}, nil
	case events.KeyDown, events.KeyUp:
		key, err := parseKey(r.Key)
		if err != nil {
			return nil, err
		}
		location, err := parseKeyLocation(r.Location)
		if err != nil {
			return nil, err
		}
		return events.KeyEvent{
			EventType: eventType,
			Key:       key,
			Code:      events.KeyCode(r.Code),
			Location:  location,
			Modifiers: modifiers,
			Repeat:    r.Repeat,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported dev event type %d", eventType)
	}
}

func parseEventType(name string, numeric *int) (events.EventType, error) {
	if name != "" {
		switch token(name) {
		case "pointer_enter", "pointerenter", "mouse_enter", "mouseenter":
			return events.PointerEnter, nil
		case "pointer_leave", "pointerleave", "mouse_leave", "mouseleave":
			return events.PointerLeave, nil
		case "pointer_move", "pointermove", "mouse_move", "mousemove":
			return events.PointerMove, nil
		case "pointer_down", "pointerdown", "mouse_down", "mousedown":
			return events.PointerDown, nil
		case "pointer_up", "pointerup", "mouse_up", "mouseup":
			return events.PointerUp, nil
		case "wheel", "mouse_wheel", "mousewheel":
			return events.Wheel, nil
		case "key_down", "keydown":
			return events.KeyDown, nil
		case "key_up", "keyup":
			return events.KeyUp, nil
		default:
			return 0, fmt.Errorf("unknown dev event type %q", name)
		}
	}
	if numeric != nil {
		return events.EventType(*numeric), nil
	}
	return 0, errEventTypeRequired
}

func parsePointerButton(raw json.RawMessage) (events.PointerButton, error) {
	if isEmptyJSON(raw) {
		return events.PointerButtonNone, nil
	}

	var number uint8
	if err := json.Unmarshal(raw, &number); err == nil {
		button := events.PointerButton(number)
		if button <= events.PointerButtonForward {
			return button, nil
		}
		return 0, fmt.Errorf("unknown pointer button %d", number)
	}

	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		return 0, fmt.Errorf("invalid pointer button: %w", err)
	}
	return pointerButtonByName(name)
}

func parsePointerButtons(raw json.RawMessage) (events.PointerButtons, error) {
	if isEmptyJSON(raw) {
		return 0, nil
	}

	var number uint16
	if err := json.Unmarshal(raw, &number); err == nil {
		return events.PointerButtons(number), nil
	}

	var names []string
	if err := json.Unmarshal(raw, &names); err == nil {
		var buttons events.PointerButtons
		for _, name := range names {
			button, err := pointerButtonByName(name)
			if err != nil {
				return 0, err
			}
			buttons |= pointerButtonDown(button)
		}
		return buttons, nil
	}

	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		return 0, fmt.Errorf("invalid pointer buttons: %w", err)
	}
	button, err := pointerButtonByName(name)
	if err != nil {
		return 0, err
	}
	return pointerButtonDown(button), nil
}

func pointerButtonByName(name string) (events.PointerButton, error) {
	switch token(name) {
	case "", "none":
		return events.PointerButtonNone, nil
	case "left":
		return events.PointerButtonLeft, nil
	case "right":
		return events.PointerButtonRight, nil
	case "middle":
		return events.PointerButtonMiddle, nil
	case "back":
		return events.PointerButtonBack, nil
	case "forward":
		return events.PointerButtonForward, nil
	default:
		return 0, fmt.Errorf("unknown pointer button %q", name)
	}
}

func pointerButtonDown(button events.PointerButton) events.PointerButtons {
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

func parseModifiers(raw json.RawMessage) (events.Modifiers, error) {
	if isEmptyJSON(raw) {
		return 0, nil
	}

	var number uint16
	if err := json.Unmarshal(raw, &number); err == nil {
		return events.Modifiers(number), nil
	}

	var names []string
	if err := json.Unmarshal(raw, &names); err == nil {
		var modifiers events.Modifiers
		for _, name := range names {
			modifier, err := modifierByName(name)
			if err != nil {
				return 0, err
			}
			modifiers |= modifier
		}
		return modifiers, nil
	}

	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		return 0, fmt.Errorf("invalid modifiers: %w", err)
	}
	return modifierByName(name)
}

func modifierByName(name string) (events.Modifiers, error) {
	switch token(name) {
	case "", "none":
		return 0, nil
	case "shift":
		return events.ModifierShift, nil
	case "control", "ctrl":
		return events.ModifierControl, nil
	case "alt", "option":
		return events.ModifierAlt, nil
	case "super", "meta", "command", "cmd":
		return events.ModifierSuper, nil
	default:
		return 0, fmt.Errorf("unknown modifier %q", name)
	}
}

func parseWheelDeltaMode(name string) (events.WheelDeltaMode, error) {
	switch token(name) {
	case "", "pixel", "pixels":
		return events.WheelDeltaPixel, nil
	case "line", "lines":
		return events.WheelDeltaLine, nil
	default:
		return 0, fmt.Errorf("unknown wheel delta mode %q", name)
	}
}

func parseKey(raw json.RawMessage) (events.Key, error) {
	if isEmptyJSON(raw) {
		return events.KeyUnknown, nil
	}

	var number uint32
	if err := json.Unmarshal(raw, &number); err == nil {
		return events.Key(number), nil
	}

	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		return 0, fmt.Errorf("invalid key: %w", err)
	}
	return keyByName(name)
}

func keyByName(name string) (events.Key, error) {
	tok := token(name)
	compact := strings.ReplaceAll(tok, "_", "")

	if len(compact) == 1 {
		ch := compact[0]
		if ch >= 'a' && ch <= 'z' {
			return events.KeyA + events.Key(ch-'a'), nil
		}
		if ch >= '0' && ch <= '9' {
			return events.Key0 + events.Key(ch-'0'), nil
		}
	}

	if strings.HasPrefix(compact, "key") {
		return keyByName(compact[3:])
	}
	if strings.HasPrefix(compact, "digit") {
		return keyByName(compact[5:])
	}
	if strings.HasPrefix(compact, "f") {
		number, err := strconv.Atoi(compact[1:])
		if err == nil && 1 <= number && number <= 24 {
			return events.KeyF1 + events.Key(number-1), nil
		}
	}
	if strings.HasPrefix(compact, "numpad") {
		return numpadKeyByName(compact[len("numpad"):], name)
	}

	switch compact {
	case "", "unknown":
		return events.KeyUnknown, nil
	case "escape", "esc":
		return events.KeyEscape, nil
	case "enter", "return":
		return events.KeyEnter, nil
	case "tab":
		return events.KeyTab, nil
	case "backspace":
		return events.KeyBackspace, nil
	case "delete", "del":
		return events.KeyDelete, nil
	case "insert", "ins":
		return events.KeyInsert, nil
	case "space", "spacebar":
		return events.KeySpace, nil
	case "arrowleft", "left":
		return events.KeyArrowLeft, nil
	case "arrowright", "right":
		return events.KeyArrowRight, nil
	case "arrowup", "up":
		return events.KeyArrowUp, nil
	case "arrowdown", "down":
		return events.KeyArrowDown, nil
	case "home":
		return events.KeyHome, nil
	case "end":
		return events.KeyEnd, nil
	case "pageup":
		return events.KeyPageUp, nil
	case "pagedown":
		return events.KeyPageDown, nil
	case "shift":
		return events.KeyShift, nil
	case "control", "ctrl":
		return events.KeyControl, nil
	case "alt", "option":
		return events.KeyAlt, nil
	case "super", "meta", "command", "cmd":
		return events.KeySuper, nil
	case "capslock":
		return events.KeyCapsLock, nil
	case "numlock":
		return events.KeyNumLock, nil
	case "printscreen":
		return events.KeyPrintScreen, nil
	case "scrolllock":
		return events.KeyScrollLock, nil
	case "pause":
		return events.KeyPause, nil
	case "minus":
		return events.KeyMinus, nil
	case "equal", "equals":
		return events.KeyEqual, nil
	case "bracketleft", "leftbracket":
		return events.KeyBracketLeft, nil
	case "bracketright", "rightbracket":
		return events.KeyBracketRight, nil
	case "backslash":
		return events.KeyBackslash, nil
	case "semicolon":
		return events.KeySemicolon, nil
	case "quote", "apostrophe":
		return events.KeyQuote, nil
	case "comma":
		return events.KeyComma, nil
	case "period", "dot":
		return events.KeyPeriod, nil
	case "slash":
		return events.KeySlash, nil
	case "backquote", "grave":
		return events.KeyBackquote, nil
	default:
		return 0, fmt.Errorf("unknown key %q", name)
	}
}

func numpadKeyByName(suffix, original string) (events.Key, error) {
	if len(suffix) == 1 && suffix[0] >= '0' && suffix[0] <= '9' {
		return events.KeyNumpad0 + events.Key(suffix[0]-'0'), nil
	}
	switch suffix {
	case "add", "plus":
		return events.KeyNumpadAdd, nil
	case "subtract", "minus":
		return events.KeyNumpadSubtract, nil
	case "multiply", "asterisk":
		return events.KeyNumpadMultiply, nil
	case "divide", "slash":
		return events.KeyNumpadDivide, nil
	case "decimal", "period", "dot":
		return events.KeyNumpadDecimal, nil
	case "enter":
		return events.KeyNumpadEnter, nil
	default:
		return 0, fmt.Errorf("unknown key %q", original)
	}
}

func parseKeyLocation(name string) (events.KeyLocation, error) {
	switch token(name) {
	case "", "standard":
		return events.KeyLocationStandard, nil
	case "left":
		return events.KeyLocationLeft, nil
	case "right":
		return events.KeyLocationRight, nil
	case "numpad":
		return events.KeyLocationNumpad, nil
	default:
		return 0, fmt.Errorf("unknown key location %q", name)
	}
}

func isEmptyJSON(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) == 0 || bytes.Equal(raw, []byte("null"))
}

func token(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.NewReplacer("-", "_", " ", "_", ".", "_").Replace(value)
	for strings.Contains(value, "__") {
		value = strings.ReplaceAll(value, "__", "_")
	}
	return strings.Trim(value, "_")
}
