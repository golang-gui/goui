package main

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

func prepareNativeTrace() error { return nil }

func startNativeTrace(_ gui.Window, out *traceLog, ready func(error)) (func() error, error) {
	thread := winapi.GetCurrentThreadId()
	var hooks []winapi.HHOOK
	stop := func() (err error) {
		for i := len(hooks) - 1; i >= 0; i-- {
			err = errors.Join(err, winapi.UnhookWindowsHookEx(hooks[i]))
		}
		hooks = nil
		return
	}
	queue := winapi.HOOKPROC(syscall.NewCallback(func(code int32, wp winapi.WPARAM, lp winapi.LPARAM) winapi.LRESULT {
		if code >= 0 {
			m := *(*winapi.MSG)(unsafe.Pointer(uintptr(lp)))
			traceWindowsMessage(out, fmt.Sprintf("queue remove=%d time=%d point=%v", wp, m.Time, m.Pt), m.Hwnd, m.Message, m.WParam, m.LParam)
		}
		return winapi.CallNextHookEx(0, code, wp, lp)
	}))
	proc := winapi.HOOKPROC(syscall.NewCallback(func(code int32, wp winapi.WPARAM, lp winapi.LPARAM) winapi.LRESULT {
		if code >= 0 {
			m := *(*winapi.CWPSTRUCT)(unsafe.Pointer(uintptr(lp)))
			traceWindowsMessage(out, "wndproc", m.Hwnd, m.Message, m.WParam, m.LParam)
		}
		return winapi.CallNextHookEx(0, code, wp, lp)
	}))
	for _, pair := range []struct {
		kind     int32
		callback winapi.HOOKPROC
	}{{winapi.WH_GETMESSAGE, queue}, {winapi.WH_CALLWNDPROC, proc}} {
		h, err := winapi.SetWindowsHookEx(pair.kind, pair.callback, 0, thread)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("install thread hook %d: %w", pair.kind, err), stop())
		}
		hooks = append(hooks, h)
	}
	out.record("ENV", "native=Win32 GUI-thread=%d hooks=WH_GETMESSAGE,WH_CALLWNDPROC source=device/origin native enums; unavailable is not evidence of hardware", thread)
	ready(nil)
	return stop, nil
}

func traceWindowsMessage(out *traceLog, stage string, wnd winapi.HWND, id winapi.UINT, wp winapi.WPARAM, lp winapi.LPARAM) {
	detail := ""
	switch id {
	case winapi.WM_MOUSEWHEEL, winapi.WM_MOUSEHWHEEL:
		detail = fmt.Sprintf(" wheel.raw=%d keys=%#x screen=%d,%d", int16(uint64(wp)>>16), uint16(wp), int16(lp), int16(uint64(lp)>>16))
	case winapi.WM_KEYDOWN, winapi.WM_KEYUP, winapi.WM_SYSKEYDOWN, winapi.WM_SYSKEYUP:
		detail = fmt.Sprintf(" vk=%#x scan=%#x extended=%d previous=%d up=%d repeatCount=%d", wp, uint64(lp)>>16&255, uint64(lp)>>24&1, uint64(lp)>>30&1, uint64(lp)>>31&1, uint16(lp))
	case winapi.WM_HSCROLL, winapi.WM_VSCROLL:
		detail = fmt.Sprintf(" scroll.command=%d position16=%d control=%#x", uint16(wp), uint16(uint64(wp)>>16), lp)
	}
	if id >= 0x100 && id <= 0x109 || id >= 0x200 && id <= 0x20e || id >= 0x240 && id <= 0x24f || id == winapi.WM_HSCROLL || id == winapi.WM_VSCROLL || id == winapi.WM_GESTURE {
		var source winapi.INPUT_MESSAGE_SOURCE
		ok := winapi.GetCurrentInputMessageSource(&source) != 0
		detail += fmt.Sprintf(" source.available=%t source.device=%d source.origin=%d", ok, source.DeviceType, source.OriginId)
	}
	out.record("NATIVE", "win32 %s hwnd=%#x message=%#04x(%s) wParam=%#x lParam=%#x%s", stage, wnd, id, windowsMessageName(id), wp, lp, detail)
}

func windowsMessageName(id winapi.UINT) string {
	switch id {
	case winapi.WM_MOUSEWHEEL:
		return "WM_MOUSEWHEEL"
	case winapi.WM_MOUSEHWHEEL:
		return "WM_MOUSEHWHEEL"
	case winapi.WM_HSCROLL:
		return "WM_HSCROLL"
	case winapi.WM_VSCROLL:
		return "WM_VSCROLL"
	case winapi.WM_KEYDOWN:
		return "WM_KEYDOWN"
	case winapi.WM_KEYUP:
		return "WM_KEYUP"
	case winapi.WM_SYSKEYDOWN:
		return "WM_SYSKEYDOWN"
	case winapi.WM_SYSKEYUP:
		return "WM_SYSKEYUP"
	case winapi.WM_CHAR:
		return "WM_CHAR"
	case winapi.WM_GESTURE:
		return "WM_GESTURE"
	case winapi.WM_GESTURENOTIFY:
		return "WM_GESTURENOTIFY"
	case winapi.WM_POINTERDOWN:
		return "WM_POINTERDOWN"
	case winapi.WM_POINTERUPDATE:
		return "WM_POINTERUPDATE"
	case winapi.WM_POINTERUP:
		return "WM_POINTERUP"
	case winapi.WM_POINTERWHEEL:
		return "WM_POINTERWHEEL"
	case winapi.WM_POINTERHWHEEL:
		return "WM_POINTERHWHEEL"
	case winapi.DM_POINTERHITTEST:
		return "DM_POINTERHITTEST"
	default:
		return "other"
	}
}
