// Windows 原生消息队列 / 文字默认动作回归（显式启动的窗口测试）。
//
// 启动：go run ./tests/window/platform/keydefault
// 环境：Windows User32 窗口站和原生事件循环；不依赖 Painter、字体或像素缩放。
// 初始：创建一个不可见的自有窗口，启用普通文字输入，不抢夺桌面焦点。
// 操作：程序只向自己的 HWND 投递 A 与 S 的原生按键消息；S 被处理，A 不处理。
// 预期：KeyDown A 一次、S（含一次重复）两次；输入法只提交一个 a（CapsLock 时 A），
//
//	没有 s 和重复字符；自动打印 PASS 并退出。5 秒无结果则失败退出。
//
// 这验证 PostMessage -> 原生消息循环 -> KeyEvent -> TranslateMessage/WM_CHAR
// 的衔接，不替代真实键盘布局、死键、AltGr 或中文候选窗的手动验收。
// 副作用：短暂创建隐藏窗口；不修改系统布局、剪贴板或其他应用。
// 复位：重跑。退出：自动退出，异常时可 Ctrl+C。
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
