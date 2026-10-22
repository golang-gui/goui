// Windows native message-queue and default text-action window regression.
//
// Run: go run ./tests/window/platform/keydefault
// Environment: Windows User32 window station and native event loop. No Painter,
// fonts or rendering scale are involved. It creates an invisible owned window
// with normal text input, without taking desktop focus.
// The program posts A and S native key messages only to its own HWND: S is
// handled and A is not. Expect one A KeyDown, two S KeyDowns including a repeat,
// and exactly one text commit a (A with CapsLock), with no s or duplicate text.
// It prints PASS and exits; lack of a result within five seconds fails.
// This checks PostMessage -> native loop -> KeyEvent -> TranslateMessage/WM_CHAR;
// real layouts, dead keys, AltGr and Chinese candidates need manual checks.
// Side effects: one temporary hidden window; no system layout, clipboard or
// other-application changes. Reset: rerun. Exit: automatic, or Ctrl+C on failure.
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("PASS: native key consumption suppresses WM_CHAR; unhandled text commits once")
}

func run() error {
	p, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.KeyDefaultProbe")
	if err != nil {
		return err
	}
	defer p.Destroy()
	loop, err := p.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	a, s := 0, 0
	text := ""
	var failure error
	w, err := p.NewWindow(geometry.Size{Width: 100, Height: 100}, func(event events.Event) {
		if e, ok := event.(events.KeyEvent); ok && e.EventType == events.KeyDown {
			switch e.Key {
			case events.KeyA:
				a++
			case events.KeyS:
				s++
				e.PreventDefault()
			}
		}
	}, platform.WindowOptions{})
	if err != nil {
		return err
	}
	defer w.Destroy()
	im, err := p.NewInputMethod(w, func(r platform.InputMethodResult) {
		if r.Kind == platform.InputMethodCommit {
			text += r.Text
			loop.Post(func() {
				if a != 1 || s != 2 || strings.ToLower(text) != "a" {
					failure = fmt.Errorf("downs A=%d S=%d commit=%q", a, s, text)
				}
				loop.Quit()
			})
		}
	})
	if err != nil {
		return err
	}
	defer im.Destroy()
	im.SetEnabled(true)
	loop.Post(func() {
		hwnd := winapi.HWND(w.NativeHandle())
		for _, key := range []struct {
			vk    winapi.WPARAM
			param winapi.LPARAM
		}{{'A', 1}, {'S', 1}, {'S', 1 | 1<<30}} {
			if err := winapi.PostMessage(hwnd, winapi.WM_KEYDOWN, key.vk, key.param); err != nil {
				failure = err
				loop.Quit()
				return
			}
		}
		for _, key := range []winapi.WPARAM{'A', 'S'} {
			if err := winapi.PostMessage(hwnd, winapi.WM_KEYUP, key, 1|1<<30|1<<31); err != nil {
				failure = err
				loop.Quit()
				return
			}
		}
	})
	done := make(chan struct{})
	defer close(done)
	go func() {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
			loop.Post(func() { failure = fmt.Errorf("timed out: A=%d S=%d commit=%q", a, s, text); loop.Quit() })
		case <-done:
		}
	}()
	loop.Run()
	return failure
}
