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

func TestFocusEventContract(t *testing.T) {
	event := events.FocusEvent{Focused: true}
	if got := event.Type(); got != events.Focus {
		t.Fatalf("FocusEvent.Type() = %v, want %v", got, events.Focus)
	}
	if !event.Focused {
		t.Fatal("FocusEvent.Focused = false")
	}
}

func TestFocusEventsManual(t *testing.T) {
	if os.Getenv("GOUI_TEST_FOCUS") == "" {
		t.Skip("set GOUI_TEST_FOCUS=1 to run the manual focus event test")
	}
	skipWithoutDisplay(t)

	target := focusEventTarget(t)
	timeout := focusEventTimeout(t)

	plat, err := getPlatform()
	if err != nil {
		t.Fatal(err)
	}

	var (
		eventLoop platform.EventLoop
		window    platform.Window
		destroyed bool
	)
	focusEvents := make(chan events.FocusEvent, 16)
	closed := make(chan struct{}, 1)

	runOnMainThread(func() {
		eventLoop, err = plat.NewEventLoop()
		if err != nil {
			return
		}

		window, err = plat.NewWindow(800, 600, func(event platform.Event) {
			switch event := event.(type) {
			case events.FocusEvent:
				select {
				case focusEvents <- event:
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

		if err = window.SetTitle("goui focus event test"); err != nil {
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
			case event := <-focusEvents:
				seen++
				t.Logf("focus[%d/%d] focused=%v", seen, target, event.Focused)
			case <-closed:
				result <- fmt.Errorf("window closed after %d/%d focus events", seen, target)
				return
			case <-timer.C:
				result <- fmt.Errorf("timed out after %s with %d/%d focus events", timeout, seen, target)
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

func focusEventTarget(t *testing.T) int {
	t.Helper()

	const defaultTarget = 1
	value := os.Getenv("GOUI_TEST_FOCUS_EVENTS")
	if value == "" {
		return defaultTarget
	}

	target, err := strconv.Atoi(value)
	if err != nil || target <= 0 {
		t.Fatalf("GOUI_TEST_FOCUS_EVENTS must be a positive integer, got %q", value)
	}
	return target
}

func focusEventTimeout(t *testing.T) time.Duration {
	t.Helper()

	const defaultTimeout = 15 * time.Second
	value := os.Getenv("GOUI_TEST_FOCUS_TIMEOUT")
	if value == "" {
		return defaultTimeout
	}

	timeout, err := time.ParseDuration(value)
	if err != nil {
		seconds, secondsErr := strconv.Atoi(value)
		if secondsErr != nil {
			t.Fatalf("GOUI_TEST_FOCUS_TIMEOUT must be a Go duration or seconds, got %q", value)
		}
		timeout = time.Duration(seconds) * time.Second
	}
	if timeout <= 0 {
		t.Fatalf("GOUI_TEST_FOCUS_TIMEOUT must be positive, got %q", value)
	}
	return timeout
}
