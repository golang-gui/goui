// Windows Go -> Win32 -> Go message-binding reentry regression, without DevServer.
//
// Environment: Windows x64 desktop session; no rendering backend/scale requirement.
// Run from the repository root:
//
//	go run ./tests/window/platform/message_reentry
//
// A hidden native window still depends on the Windows message system and must
// be run explicitly. No manual input is required: another thread calls synchronous
// SendMessage, GetMessage/PeekMessage dispatch it on the main thread, and the
// callback grows the Go stack before posting a tagged reply.
// Expect each MSG address to remain stable across callbacks and correct reply
// HWND/message/parameters. Each retrieval API has independent storage so one
// buffer's escape cannot hide the other's problem. Also check DispatchMessage
// callbacks, GetMessage's normal WM_QUIT result and -1/error for an invalid HWND.
// All assertions print PASS and exit. The old binding could fail address assertions
// or crash natively. Side effects: this process's hidden windows, thread messages
// and logs only, without desktop settings or user-data changes. Reset: restart.
// Exit: automatic; a 15-second timeout reports an error and exits nonzero.
package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

const (
	probeMessage = winapi.WM_APP + 71
	replyMessage = winapi.WM_APP + 72
	marker       = 0x13579
)

var stackSum uint64

//go:noinline
func growStack(depth int) uint64 {
	var data [4096]byte
	for i := range data {
		data[i] = byte(depth + i)
	}
	var child uint64
	if depth != 0 {
		child = growStack(depth - 1)
	}
	// Read the whole array after recursion to keep every frame on the stack.
	for _, value := range data {
		child += uint64(value)
	}
	return child
}

func windowProc(hwnd winapi.HWND, msg winapi.UINT, wParam winapi.WPARAM, lParam winapi.LPARAM) winapi.LRESULT {
	if msg == probeMessage {
		runtime.GC()
		stackSum = growStack(128)
		if err := winapi.PostMessage(hwnd, replyMessage, marker, 0); err != nil {
			panic(err)
		}
		return 0
	}
	return winapi.DefWindowProc(hwnd, msg, wParam, lParam)
}

func assertReply(hwnd winapi.HWND, msg winapi.MSG, before, after uintptr) {
	if before != after {
		panic("MSG moved during native callback")
	}
	if msg.Hwnd != hwnd || msg.Message != replyMessage || msg.WParam != marker {
		panic(fmt.Sprintf("corrupt reply: %#v", msg))
	}
}

// Keep these receivers separate: GetMessage's escape must not make an unsafe
// PeekMessage appear safe just because they share the same local MSG.
//
//go:noinline
func getReply(hwnd winapi.HWND) {
	var msg winapi.MSG
	before := uintptr(unsafe.Pointer(&msg))
	result, err := winapi.GetMessage(&msg, hwnd, replyMessage, replyMessage)
	if err != nil || result <= 0 {
		panic(fmt.Sprintf("GetMessage: result=%d error=%v", result, err))
	}
	assertReply(hwnd, msg, before, uintptr(unsafe.Pointer(&msg)))
}

//go:noinline
func peekReply(hwnd winapi.HWND) {
	var msg winapi.MSG
	before := uintptr(unsafe.Pointer(&msg))
	for {
		msg = winapi.MSG{}
		result, err := winapi.PeekMessage(&msg, hwnd, replyMessage, replyMessage, winapi.PM_REMOVE)
		if err != nil {
			panic(err)
		}
		if result != 0 {
			break
		}
		runtime.Gosched()
	}
	assertReply(hwnd, msg, before, uintptr(unsafe.Pointer(&msg)))
}

//go:noinline
func dispatchProbe(hwnd winapi.HWND) {
	msg := winapi.MSG{Hwnd: hwnd, Message: probeMessage}
	before := uintptr(unsafe.Pointer(&msg))
	winapi.DispatchMessage(&msg)
	if before != uintptr(unsafe.Pointer(&msg)) {
		panic("MSG moved during DispatchMessage callback")
	}
	getReply(hwnd)
}

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	timeout := time.AfterFunc(15*time.Second, func() {
		fmt.Fprintln(os.Stderr, "FAIL: message reentry timeout")
		os.Exit(1)
	})
	defer timeout.Stop()
	instance, err := winapi.GetModuleHandle(nil)
	if err != nil {
		panic(err)
	}
	name := syscall.StringToUTF16Ptr("GOUI Message Reentry Regression")
	class := winapi.WNDCLASSEX{Size: winapi.Sizeof_WNDCLASSEX, Instance: instance,
		ClassName: name, WndProc: winapi.MakeWindowProc(windowProc)}
	if _, err := winapi.RegisterClassEx(&class); err != nil {
		panic(err)
	}
	defer winapi.UnregisterClass(name, instance)
	hwnd, err := winapi.CreateWindowEx(0, name, name, 0, 0, 0, 1, 1, 0, 0, instance, nil)
	if err != nil {
		panic(err)
	}
	defer winapi.DestroyWindow(hwnd)
	// Independent native sender; the production binding under test is the receiver.
	send := syscall.NewLazyDLL("user32.dll").NewProc("SendMessageW")
	for _, peek := range []bool{false, true} {
		for iteration := 0; iteration < 8; iteration++ {
			done := make(chan struct{})
			go func() {
				send.Call(uintptr(hwnd), probeMessage, 0, 0)
				close(done)
			}()
			if peek {
				peekReply(hwnd)
			} else {
				getReply(hwnd)
			}
			<-done
		}
		fmt.Printf("PASS: message reentry peek=%v\n", peek)
	}
	dispatchProbe(hwnd)
	fmt.Println("PASS: DispatchMessage reentry")
	var msg winapi.MSG
	if result, err := winapi.GetMessage(&msg, winapi.HWND(0x1234), 0, 0); result != -1 || err == nil || err == syscall.Errno(0) {
		panic(fmt.Sprintf("invalid HWND: result=%d error=%v", result, err))
	}
	winapi.PostQuitMessage(37)
	if result, err := winapi.GetMessage(&msg, 0, 0, 0); result != 0 || err != nil || msg.WParam != 37 {
		panic(fmt.Sprintf("WM_QUIT: result=%d error=%v msg=%#v", result, err, msg))
	}
	fmt.Println("PASS: GetMessage error and quit contracts")
}
