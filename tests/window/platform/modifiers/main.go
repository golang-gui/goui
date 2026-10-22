// Native left/right modifier event checks.
//
// Environment: Windows/macOS/Linux desktop and keyboard. On Linux record WM,
// keyboard layout and XIM. No Painter, fonts, rendering backend or scale is used;
// status appears in the window title and terminal.
// Run: go run ./tests/window/platform/modifiers -modifier control
// Choose shift/control/alt/system; macOS alt is Option and system is Command,
// Windows system is Win and Linux system is Super. -reverse exchanges left/right.
// Initially idle: F8 starts the ordinary sequence; Escape resets. Disable Sticky Keys.
// Steps and expected results:
//  1. Release all modifiers, focus the window and press F8. Press left, press
//     right, release left, press S, release right, then F8. Assert Down/Down/Up,
//     side and modifier bits. S retains the modifier; final Up and F8 do not.
//     Completion prints PASS.
//  2. In another window hold both sides, return by pointer and press S to start.
//     Release left, press S, release right, press F8: same expectations without
//     relying on this window receiving the initial key presses.
//  3. Lose focus, release keys externally, return and repeat step 1 with F8:
//     no stale modifier remains.
//  4. Repeat each modifier and -reverse. If right Alt is AltGr on Linux, ordinary
//     Alt checks do not apply to it; changing the layout does not verify the original.
//
// System shortcuts such as Win/Super+S may be intercepted. Mark undelivered
// combinations uncovered, not passed. macOS may require Fn+F8.
// Side effects: this window only; no settings/clipboard changes or IME text input
// request. Reset: Escape. Exit: close the window; any FAIL gives exit 1, otherwise 0.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

func main() {
	runtime.LockOSThread()
	name := flag.String("modifier", "control", "shift/control/alt/system")
	reverse := flag.Bool("reverse", false, "start with the right key")
	flag.Parse()
	if err := run(*name, *reverse); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(name string, reverse bool) error {
	key, mods := events.KeyControl, events.ModifierControl
	switch name {
	case "control":
	case "shift":
		key, mods = events.KeyShift, events.ModifierShift
	case "alt":
		key, mods = events.KeyAlt, events.ModifierAlt
		if runtime.GOOS == "darwin" {
			key, mods = events.KeyOption, events.ModifierOption
		}
	case "system":
		switch runtime.GOOS {
		case "windows":
			key, mods = events.KeyWin, events.ModifierWin
		case "darwin":
			key, mods = events.KeyCommand, events.ModifierCommand
		default:
			key, mods = events.KeySuper, events.ModifierSuper
		}
	default:
		return fmt.Errorf("unknown modifier %q", name)
	}
	p, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.ModifierProbe")
	if err != nil {
		return err
	}
	defer p.Destroy()
	loop, err := p.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	first, other := events.KeyLocationLeft, events.KeyLocationRight
	if reverse {
		first, other = other, first
	}
	type expected struct {
		typ      events.EventType
		key      events.Key
		location events.KeyLocation
		mods     events.Modifiers
	}
	sequence := []expected{
		{events.KeyDown, key, first, mods},
		{events.KeyDown, key, other, mods},
		{events.KeyUp, key, first, mods},
		{events.KeyDown, events.KeyS, events.KeyLocationStandard, mods},
		{events.KeyUp, key, other, 0},
		{events.KeyDown, events.KeyF8, events.KeyLocationStandard, 0},
	}
	step := -1
	var failure error
	var w platform.Window
	report := func(message string) {
		fmt.Println(message)
		if w != nil {
			_ = w.SetTitle("GOUI modifiers / " + name + " / " + message)
		}
	}
	fail := func(message string) {
		failure = fmt.Errorf("FAIL: %s", message)
		step = -1
		report(failure.Error())
	}
	w, err = p.NewWindow(geometry.Size{Width: 650, Height: 180}, func(event events.Event) {
		switch e := event.(type) {
		case events.CloseEvent:
			loop.Quit()
		case events.FocusEvent:
			if !e.Focused {
				step = -1
				report("focus lost; F8 fresh / S held")
			}
		case events.KeyEvent:
			fmt.Printf("event type=%v key=%v location=%v mods=%d repeat=%v\n", e.EventType, e.Key, e.Location, e.Modifiers, e.Repeat)
			if e.Key == events.KeyEscape && e.EventType == events.KeyDown {
				step = -1
				report("reset; F8 fresh / S held")
				return
			}
			if step < 0 {
				if e.EventType != events.KeyDown || e.Repeat {
					return
				}
				switch e.Key {
				case events.KeyF8:
					if e.Modifiers != 0 {
						fail(fmt.Sprintf("start has modifiers=%d", e.Modifiers))
						return
					}
					step = 0
				case events.KeyS:
					if e.Modifiers != mods {
						fail(fmt.Sprintf("held start modifiers=%d want=%d", e.Modifiers, mods))
						return
					}
					step = 2
				default:
					return
				}
				report(fmt.Sprintf("armed step=%d reverse=%v", step, reverse))
				return
			}
			if e.Key != key && !((e.Key == events.KeyF8 || e.Key == events.KeyS) && e.EventType == events.KeyDown) {
				return
			}
			want := sequence[step]
			if e.EventType != want.typ || e.Key != want.key || e.Location != want.location || e.Modifiers != want.mods || e.Repeat {
				fail(fmt.Sprintf("step %d got %v/%v/%v/%d repeat=%v; want %+v", step, e.EventType, e.Key, e.Location, e.Modifiers, e.Repeat, want))
				return
			}
			step++
			if step == len(sequence) {
				step = -1
				report("PASS; F8 fresh / S held")
			} else {
				report(fmt.Sprintf("step %d OK", step))
			}
		}
	}, platform.WindowOptions{})
	if err != nil {
		return err
	}
	defer w.Destroy()
	report("F8 fresh / S held")
	if err = w.Show(); err != nil {
		return err
	}
	loop.Run()
	return failure
}
