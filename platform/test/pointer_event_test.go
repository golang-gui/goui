package test

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

func TestPointerEventContract(t *testing.T) {
	for _, typ := range []events.EventType{
		events.PointerEnter,
		events.PointerLeave,
		events.PointerMove,
		events.PointerDown,
		events.PointerUp,
	} {
		event := events.PointerEvent{
			EventType: typ,
			Position:  geometry.Point{X: 12, Y: 34},
			Button:    events.PointerButtonLeft,
			Buttons:   events.PointerButtonLeftDown | events.PointerButtonRightDown,
			Modifiers: events.ModifierShift | events.ModifierControl,
		}

		if got := event.Type(); got != typ {
			t.Fatalf("PointerEvent.Type() = %v, want %v", got, typ)
		}
		if event.Position != (geometry.Point{X: 12, Y: 34}) {
			t.Fatalf("PointerEvent.Position = %+v", event.Position)
		}
		if event.Button != events.PointerButtonLeft {
			t.Fatalf("PointerEvent.Button = %v", event.Button)
		}
		if event.Buttons != events.PointerButtonLeftDown|events.PointerButtonRightDown {
			t.Fatalf("PointerEvent.Buttons = %v", event.Buttons)
		}
		if event.Modifiers != events.ModifierShift|events.ModifierControl {
			t.Fatalf("PointerEvent.Modifiers = %v", event.Modifiers)
		}
	}
}

func TestPointerEventsManual(t *testing.T) {
	if os.Getenv("GOUI_TEST_POINTER") == "" {
		t.Skip("set GOUI_TEST_POINTER=1 to run the manual pointer event test")
	}
	skipWithoutDisplay(t)

	target := pointerEventTarget(t)
	timeout := pointerEventTimeout(t)

	plat, err := getPlatform()
	if err != nil {
		t.Fatal(err)
	}

	var (
		eventLoop platform.EventLoop
		window    platform.Window
		destroyed bool
	)
	pointerEvents := make(chan events.PointerEvent, 64)
	closed := make(chan struct{}, 1)

	runOnMainThread(func() {
		eventLoop, err = plat.NewEventLoop()
		if err != nil {
			return
		}

		window, err = plat.NewWindow(func(event platform.Event) {
			switch event := event.(type) {
			case events.PointerEvent:
				select {
				case pointerEvents <- event:
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

		if err = window.SetTitle("goui pointer event test"); err != nil {
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
			case event := <-pointerEvents:
				seen++
				t.Logf(
					"pointer[%d/%d] type=%s pos=(%.1f, %.1f) button=%s buttons=%s modifiers=%s",
					seen,
					target,
					pointerEventTypeName(event.Type()),
					event.Position.X,
					event.Position.Y,
					pointerButtonName(event.Button),
					pointerButtonsName(event.Buttons),
					modifiersName(event.Modifiers),
				)
			case <-closed:
				result <- fmt.Errorf("window closed after %d/%d pointer events", seen, target)
				return
			case <-timer.C:
				result <- fmt.Errorf("timed out after %s with %d/%d pointer events", timeout, seen, target)
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

func pointerEventTarget(t *testing.T) int {
	t.Helper()

	const defaultTarget = 6
	value := os.Getenv("GOUI_TEST_POINTER_EVENTS")
	if value == "" {
		return defaultTarget
	}

	target, err := strconv.Atoi(value)
	if err != nil || target <= 0 {
		t.Fatalf("GOUI_TEST_POINTER_EVENTS must be a positive integer, got %q", value)
	}
	return target
}

func pointerEventTimeout(t *testing.T) time.Duration {
	t.Helper()

	const defaultTimeout = 15 * time.Second
	value := os.Getenv("GOUI_TEST_POINTER_TIMEOUT")
	if value == "" {
		return defaultTimeout
	}

	timeout, err := time.ParseDuration(value)
	if err != nil {
		seconds, secondsErr := strconv.Atoi(value)
		if secondsErr != nil {
			t.Fatalf("GOUI_TEST_POINTER_TIMEOUT must be a Go duration or seconds, got %q", value)
		}
		timeout = time.Duration(seconds) * time.Second
	}
	if timeout <= 0 {
		t.Fatalf("GOUI_TEST_POINTER_TIMEOUT must be positive, got %q", value)
	}
	return timeout
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
		events.ModifierCapsLock |
		events.ModifierNumLock

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
	if modifiers&events.ModifierCapsLock != 0 {
		names = append(names, "CapsLock")
	}
	if modifiers&events.ModifierNumLock != 0 {
		names = append(names, "NumLock")
	}
	if extra := modifiers &^ known; extra != 0 {
		names = append(names, fmt.Sprintf("Modifiers(0x%x)", uint16(extra)))
	}
	if len(names) == 0 {
		return "None"
	}
	return strings.Join(names, "|")
}
