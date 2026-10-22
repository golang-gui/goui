// Native clipboard result checks, run explicitly rather than by go test.
//
// Environment: Linux/X11, Windows or macOS. main runs on the platform startup
// thread. This uses the real platform/event loop without a visible window;
// typography and rendering scale do not apply. Run:
//
//	go run ./tests/window/platform/clipboard -write
//
// -write explicitly permits overwriting with disposable text. Use an isolated
// X server, Windows Session 0 or a dedicated test desktop; on macOS use an external
// wrapper that snapshots/restores every clipboard format.
// No input is needed: -write checks empty text, Unicode, repeated multiline
// writes and exactly-once callbacks. For cross-process reads, use:
//
//	go run ./tests/window/platform/clipboard -expect 'previously copied text'
//	go run ./tests/window/platform/clipboard -expect=""
//
// -error timeout/unavailable/failure checks the five-second deadline, cancellation
// on exit and conversion failures respectively; -quit-after 100ms tests exit
// cancellation. PASS and exit 0 require every assertion. An external eight-second
// watchdog does not verify the read deadline. The Linux owner subprogram provides
// external selections including silence, invalid formats and Latin-1.
// Side effects: -write overwrites the real clipboard. X11 ownership is lost at
// exit; restoring text just before exit cannot restore the previous selection.
// Windows/macOS retain the last text. Use isolation or the full-format restoration
// wrapper above. Reset: restart the program/session. Automatic exit is expected;
// Ctrl+C interruption is not a passed check.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/common"
)

func main() {
	write := flag.Bool("write", false, "overwrite clipboard with disposable text and verify round trips")
	expect := flag.String("expect", "", "exact expected external text")
	expectError := flag.String("error", "", "timeout, unavailable or failure")
	quitAfter := flag.Duration("quit-after", 0, "quit while a read is pending")
	flag.Parse()
	if err := run(*write, *expect, *expectError, *quitAfter); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("PASS clipboard")
}

func run(write bool, expect, expectError string, quitAfter time.Duration) error {
	p, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.ClipboardRegression")
	if err != nil {
		return err
	}
	defer p.Destroy()
	loop, err := p.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	c, err := p.NewClipboard()
	if err != nil {
		return err
	}
	fmt.Printf("platform=%s scale=N/A renderer=N/A\n", p.Name())
	if write {
		for _, text := range []string{"", "你好 🧧 é", "one\ntwo\tend"} {
			if err := c.SetText(text); err != nil {
				return fmt.Errorf("write: %w", err)
			}
			calls := 0
			var readError error
			c.RequestText(func(got string, err error) {
				calls++
				if err != nil || got != text {
					readError = fmt.Errorf("round trip %q: got %q, %v", text, got, err)
				}
			})
			if calls != 1 {
				return fmt.Errorf("round trip callbacks: %d", calls)
			}
			if readError != nil {
				return readError
			}
		}
		return nil
	}
	calls := 0
	var got string
	var result error
	start := time.Now()
	loop.Post(func() {
		c.RequestText(func(text string, err error) { calls++; got, result = text, err; loop.Quit() })
	})
	watchdog := time.AfterFunc(8*time.Second, loop.Quit)
	defer watchdog.Stop()
	if quitAfter > 0 {
		timer := time.AfterFunc(quitAfter, loop.Quit)
		defer timer.Stop()
	}
	loop.Run()
	if calls != 1 {
		return fmt.Errorf("read callbacks: %d", calls)
	}
	switch expectError {
	case "":
		if result != nil || got != expect {
			return fmt.Errorf("read got %q, %v; want %q", got, result, expect)
		}
	case "timeout":
		if !errors.Is(result, common.ErrClipboardTimeout) || time.Since(start) < 5*time.Second || time.Since(start) >= 7*time.Second {
			return fmt.Errorf("want five-second read timeout: %v, elapsed %v", result, time.Since(start))
		}
	case "unavailable":
		if !errors.Is(result, common.ErrUnavailable) {
			return fmt.Errorf("want loop-exit cancellation: %v", result)
		}
	case "failure":
		if result == nil || errors.Is(result, common.ErrClipboardTimeout) || errors.Is(result, common.ErrUnavailable) {
			return fmt.Errorf("want conversion failure: %v", result)
		}
	default:
		return fmt.Errorf("unknown error expectation: %s", expectError)
	}
	return nil
}
