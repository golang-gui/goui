package test

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

func TestWheelEventContract(t *testing.T) {
	event := events.WheelEvent{
		Position:  geometry.Point{X: 12, Y: 34},
		DeltaX:    1.5,
		DeltaY:    -2.25,
		Mode:      events.WheelDeltaPixel,
		Buttons:   events.PointerButtonLeftDown | events.PointerButtonRightDown,
		Modifiers: events.ModifierShift | events.ModifierControl,
	}

	if got := event.Type(); got != events.Wheel {
		t.Fatalf("WheelEvent.Type() = %v, want %v", got, events.Wheel)
	}
	if event.Position != (geometry.Point{X: 12, Y: 34}) {
		t.Fatalf("WheelEvent.Position = %+v", event.Position)
	}
	if event.DeltaX != 1.5 || event.DeltaY != -2.25 {
		t.Fatalf("WheelEvent delta = (%v, %v)", event.DeltaX, event.DeltaY)
	}
	if event.Mode != events.WheelDeltaPixel {
		t.Fatalf("WheelEvent.Mode = %v", event.Mode)
	}
	if event.Buttons != events.PointerButtonLeftDown|events.PointerButtonRightDown {
		t.Fatalf("WheelEvent.Buttons = %v", event.Buttons)
	}
	if event.Modifiers != events.ModifierShift|events.ModifierControl {
		t.Fatalf("WheelEvent.Modifiers = %v", event.Modifiers)
	}
}

func TestWheelEventsManual(t *testing.T) {
	if os.Getenv("GOUI_TEST_WHEEL") == "" {
		t.Skip("set GOUI_TEST_WHEEL=1 to run the manual wheel event test")
	}
	skipWithoutDisplay(t)

	target := wheelEventTarget(t)
	timeout := wheelEventTimeout(t)

	plat, err := getPlatform()
	if err != nil {
		t.Fatal(err)
	}

	var (
		eventLoop platform.EventLoop
		window    platform.Window
		destroyed bool
	)
	wheelEvents := make(chan events.WheelEvent, 64)
	closed := make(chan struct{}, 1)

	runOnMainThread(func() {
		eventLoop, err = plat.NewEventLoop()
		if err != nil {
			return
		}

		window, err = plat.NewWindow(func(event platform.Event) {
			switch event := event.(type) {
			case events.WheelEvent:
				select {
				case wheelEvents <- event:
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

		if err = window.SetTitle("goui wheel event test"); err != nil {
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
			case event := <-wheelEvents:
				seen++
				t.Logf(
					"wheel[%d/%d] pos=(%.1f, %.1f) delta=(%.2f, %.2f) mode=%s buttons=%s modifiers=%s",
					seen,
					target,
					event.Position.X,
					event.Position.Y,
					event.DeltaX,
					event.DeltaY,
					wheelDeltaModeName(event.Mode),
					pointerButtonsName(event.Buttons),
					modifiersName(event.Modifiers),
				)
			case <-closed:
				result <- fmt.Errorf("window closed after %d/%d wheel events", seen, target)
				return
			case <-timer.C:
				result <- fmt.Errorf("timed out after %s with %d/%d wheel events", timeout, seen, target)
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

func wheelEventTarget(t *testing.T) int {
	t.Helper()

	const defaultTarget = 4
	value := os.Getenv("GOUI_TEST_WHEEL_EVENTS")
	if value == "" {
		return defaultTarget
	}

	target, err := strconv.Atoi(value)
	if err != nil || target <= 0 {
		t.Fatalf("GOUI_TEST_WHEEL_EVENTS must be a positive integer, got %q", value)
	}
	return target
}

func wheelEventTimeout(t *testing.T) time.Duration {
	t.Helper()

	const defaultTimeout = 15 * time.Second
	value := os.Getenv("GOUI_TEST_WHEEL_TIMEOUT")
	if value == "" {
		return defaultTimeout
	}

	timeout, err := time.ParseDuration(value)
	if err != nil {
		seconds, secondsErr := strconv.Atoi(value)
		if secondsErr != nil {
			t.Fatalf("GOUI_TEST_WHEEL_TIMEOUT must be a Go duration or seconds, got %q", value)
		}
		timeout = time.Duration(seconds) * time.Second
	}
	if timeout <= 0 {
		t.Fatalf("GOUI_TEST_WHEEL_TIMEOUT must be positive, got %q", value)
	}
	return timeout
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
