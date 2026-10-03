// Windows 消息绑定的 Go -> Win32 -> Go 重入回归，不使用 DevServer。
//
// 环境：Windows x64 桌面会话，无绘制后端或缩放要求。仓库根目录启动：
//
//	go run ./tests/window/platform/message_reentry
//
// 初始：创建一个不可见的原生窗口；本例仍依赖 Windows 消息系统，须显式运行。
// 无需手动操作：另一线程同步 SendMessage，主线程在 GetMessage/PeekMessage
// 内派发该消息，窗口回调强制增长 Go 栈，并向队列投递带固定标记的回复。
// 预期：两种取消息接口的 MSG 地址在回调前后不变，回复 HWND/消息号/参数正确；
// 两种接口各自使用独立缓冲区，避免其中一个的逃逸掩盖另一个的问题。
// 同时验证 DispatchMessage 的真实窗口回调，以及 GetMessage 的 WM_QUIT
// 正常返回和无效 HWND 的 -1/错误返回。
// 全部断言成功打印 PASS 后退出。旧绑定可能因 MSG 地址改变而断言失败或原生崩溃。
// 副作用：仅本进程的隐藏窗口、线程消息和日志，不改变桌面设置或用户数据。
// 复位：重新启动。正常自动退出；超时 15 秒会输出错误并非零退出。
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
