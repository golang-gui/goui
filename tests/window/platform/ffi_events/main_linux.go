// Xlib FFI pointer regression. Requires a dedicated X11 display; no WM needed.
// Run: go run ./tests/window/platform/ffi_events [-reads=4096]. Opens one private
// unmapped window, sends numbered ClientMessage events only to that window,
// then checks PeekEvent and NextEvent from fresh goroutine stacks at varying
// frame depths. Xlib calls are strictly serialized; each executing goroutine
// locks its OS thread. Every event must retain its display, window, atom and
// payload before reaching XFilterEvent. No clipboard/settings/files change.
// The window/display are released on exit; timeout or mismatch fails. PASS
// exits automatically; rerun resets. Linux/X11 only, not GUI input acceptance.
package main

import (
	"flag"
	"fmt"
	"runtime"
	"time"

	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func main() {
	runtime.LockOSThread()
	reads := flag.Int("reads", 4096, "number of event reads per operation")
	flag.Parse()
	d := xlib.OpenDisplay("")
	if d == 0 {
		panic("X11 display unavailable")
	}
	defer d.Close()
	screen := d.DefaultScreenOfDisplay()
	w := d.CreateWindow(screen.Root, 0, 0, 1, 1, 0, int(screen.RootDepth), 1, screen.RootVisual, 0, nil)
	if w == 0 {
		panic("create private test window")
	}
	defer d.DestroyWindow(w)
	atom := d.InternAtom("GOUI_FFI_POINTER_PROBE", false)
	for _, peek := range []bool{false, true} {
		for i := 0; i < *reads; i++ {
			var expected xlib.Event
			*expected.ClientMessageEvent() = xlib.ClientMessageEvent{Type: xlib.ClientMessage, Display: d, Window: w, MessageType: atom, Format: 32, L: [5]int64{int64(i + 1), 0x12345678}}
			var pin runtime.Pinner
			pin.Pin(&expected) // Isolate the output-buffer regression from SendEvent.
			status := d.SendEvent(w, false, 0, &expected)
			pin.Unpin()
			if status == 0 {
				panic("send private event failed")
			}
			d.Flush()
			result := make(chan xlib.Event)
			go func(depth int) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				result <- readAtDepth(d, depth, peek)
			}(i % 64)
			var actual xlib.Event
			select {
			case actual = <-result:
			case <-time.After(5 * time.Second):
				panic("native read deadline exceeded")
			}
			check := func(event *xlib.Event) {
				got := event.ClientMessageEvent()
				if got.Type != xlib.ClientMessage || got.Display != d || got.Window != w || got.MessageType != atom || got.Format != 32 || got.L[0] != int64(i+1) || got.L[1] != 0x12345678 {
					panic(fmt.Sprintf("peek=%t iteration=%d: invalid native event: %+v", peek, i, got))
				}
			}
			check(&actual)
			if xlib.FilterEvent(&actual, 0) {
				panic("private event unexpectedly filtered")
			}
			if peek {
				consumed := d.NextEvent()
				check(&consumed)
			}
		}
		fmt.Printf("PASS Xlib event buffer peek=%t reads=%d\n", peek, *reads)
	}
}

//go:noinline
func readAtDepth(d xlib.Display, depth int, peek bool) (event xlib.Event) {
	var padding [256]byte
	padding[0] = byte(depth)
	if depth > 0 {
		event = readAtDepth(d, depth-1, peek)
	} else if peek {
		event = d.PeekEvent()
	} else {
		event = d.NextEvent()
	}
	runtime.KeepAlive(padding)
	return
}
