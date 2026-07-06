package test

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

func TestKeyEventContract(t *testing.T) {
	event := events.KeyEvent{
		EventType: events.KeyDown,
		Key:       events.KeyA,
		Code:      events.KeyCodeUnknown,
		Location:  events.KeyLocationStandard,
		Modifiers: events.ModifierShift | events.ModifierControl,
		Repeat:    true,
	}

	if got := event.Type(); got != events.KeyDown {
		t.Fatalf("KeyEvent.Type() = %v, want %v", got, events.KeyDown)
	}
	if event.Key != events.KeyA {
		t.Fatalf("KeyEvent.Key = %v", event.Key)
	}
	if event.Code != events.KeyCodeUnknown {
		t.Fatalf("KeyEvent.Code = %v", event.Code)
	}
	if event.Location != events.KeyLocationStandard {
		t.Fatalf("KeyEvent.Location = %v", event.Location)
	}
	if event.Modifiers != events.ModifierShift|events.ModifierControl {
		t.Fatalf("KeyEvent.Modifiers = %v", event.Modifiers)
	}
	if !event.Repeat {
		t.Fatal("KeyEvent.Repeat = false")
	}
}

func TestKeyEventsManual(t *testing.T) {
	if os.Getenv("GOUI_TEST_KEY") == "" {
		t.Skip("set GOUI_TEST_KEY=1 to run the manual key event test")
	}
	skipWithoutDisplay(t)

	target := keyEventTarget(t)
	timeout := keyEventTimeout(t)

	plat, err := getPlatform()
	if err != nil {
		t.Fatal(err)
	}

	var (
		eventLoop platform.EventLoop
		window    platform.Window
		destroyed bool
	)
	keyEvents := make(chan events.KeyEvent, 64)
	closed := make(chan struct{}, 1)

	runOnMainThread(func() {
		eventLoop, err = plat.NewEventLoop()
		if err != nil {
			return
		}

		window, err = plat.NewWindow(800, 600, func(event platform.Event) {
			switch event := event.(type) {
			case events.KeyEvent:
				select {
				case keyEvents <- event:
				default:
				}
			case events.CloseEvent:
				select {
				case closed <- struct{}{}:
				default:
				}
				window.Destroy()
				destroyed = true
				eventLoop.Quit()
			}
		})
		if err != nil {
			return
		}

		if err = window.SetTitle("goui key event test"); err != nil {
			return
		}
		err = window.Show()
	})
	if eventLoop != nil {
		defer func() {
			runOnMainThread(eventLoop.Destroy)
		}()
	}
	if err != nil {
		if window != nil {
			runOnMainThread(window.Destroy)
		}
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() {
		timer := time.NewTimer(timeout)
		defer timer.Stop()

		seen := 0
		for seen < target {
			select {
			case event := <-keyEvents:
				seen++
				t.Logf(
					"key[%d/%d] type=%s key=%s location=%s repeat=%v modifiers=%s",
					seen,
					target,
					keyEventTypeName(event.Type()),
					keyName(event.Key),
					keyLocationName(event.Location),
					event.Repeat,
					modifiersName(event.Modifiers),
				)
			case <-closed:
				result <- fmt.Errorf("window closed after %d/%d key events", seen, target)
				return
			case <-timer.C:
				result <- fmt.Errorf("timed out after %s with %d/%d key events", timeout, seen, target)
				eventLoop.Quit()
				return
			}
		}

		eventLoop.Post(func() {
			if !destroyed {
				window.Destroy()
				destroyed = true
			}
			eventLoop.Quit()
		})
		result <- nil
	}()

	runOnMainThread(eventLoop.Run)

	if !destroyed {
		runOnMainThread(window.Destroy)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func keyEventTarget(t *testing.T) int {
	t.Helper()

	const defaultTarget = 6
	value := os.Getenv("GOUI_TEST_KEY_EVENTS")
	if value == "" {
		return defaultTarget
	}

	target, err := strconv.Atoi(value)
	if err != nil || target <= 0 {
		t.Fatalf("GOUI_TEST_KEY_EVENTS must be a positive integer, got %q", value)
	}
	return target
}

func keyEventTimeout(t *testing.T) time.Duration {
	t.Helper()

	const defaultTimeout = 20 * time.Second
	value := os.Getenv("GOUI_TEST_KEY_TIMEOUT")
	if value == "" {
		return defaultTimeout
	}

	timeout, err := time.ParseDuration(value)
	if err != nil {
		seconds, secondsErr := strconv.Atoi(value)
		if secondsErr != nil {
			t.Fatalf("GOUI_TEST_KEY_TIMEOUT must be a Go duration or seconds, got %q", value)
		}
		timeout = time.Duration(seconds) * time.Second
	}
	if timeout <= 0 {
		t.Fatalf("GOUI_TEST_KEY_TIMEOUT must be positive, got %q", value)
	}
	return timeout
}

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
