// X11 剪贴板外部所有者，只在独立、可丢弃的 X server 中启动。
// go run ./tests/window/platform/clipboard/owner -mode utf8
// READY 后另启上级目录的读取程序：utf8 -> -expect '外部 é'，fallback -> -expect 'é'，
// latin1 -> -expect 'café'，none -> -expect ”，silent -> -error timeout，
// silent 配 -quit-after 100ms -> -error unavailable，format/invalid/incr/refuse
// -> -error failure。fallback 拒绝 TARGETS、UTF8_STRING，只接受 STRING。
// oversized 声明超出 16 MiB 的属性；读取必须报 failure，不截断成功。
// 无可见窗口/缩放，不发送输入；从另一进程响应真实 SelectionRequest。
// 副作用：取代隔离会话的 CLIPBOARD 所有者。Ctrl-C 或终止此测试进程退出；
// 退出后隔离剪贴板内容丢失。复位：重启 owner。不得在日常桌面启动。
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func main() {
	runtime.LockOSThread()
	mode := flag.String("mode", "utf8", "utf8, latin1, fallback, none, silent, format, invalid, incr, refuse, oversized")
	flag.Parse()
	d := xlib.OpenDisplay("")
	if d == 0 {
		fmt.Fprintln(os.Stderr, "no display")
		os.Exit(1)
	}
	defer d.Close()
	s := d.DefaultScreenOfDisplay()
	w := d.CreateWindow(s.Root, 0, 0, 1, 1, 0, int(s.RootDepth), xlib.WindowClassInputOutput, s.RootVisual, 0, nil)
	defer d.DestroyWindow(w)
	selection, targets, utf8 := d.InternAtom("CLIPBOARD", false), d.InternAtom("TARGETS", false), d.InternAtom("UTF8_STRING", false)
	d.SetSelectionOwner(selection, w, 0)
	d.Flush()
	if d.GetSelectionOwner(selection) != w {
		panic("ownership failed")
	}
	fmt.Println("READY", *mode)
	for {
		e := d.NextEvent()
		if e.Type != xlib.SelectionRequest {
			continue
		}
		r := e.SelectionRequestEvent()
		if *mode == "silent" {
			continue
		}
		property := r.Property
		if property == 0 {
			property = r.Target
		}
		if r.Target == targets {
			list := []xlib.Atom{utf8}
			if *mode == "latin1" || *mode == "fallback" {
				list = []xlib.Atom{xlib.AtomString}
			}
			if *mode == "none" {
				list = nil
			}
			if *mode == "fallback" {
				property = 0
			} else {
				d.ChangeProperty(r.Requestor, property, xlib.AtomAtom, 32, xlib.PropModeReplace, cgo.CSlice(list), len(list))
			}
		} else if *mode == "refuse" || (*mode == "fallback" && r.Target == utf8) {
			property = 0
		} else if *mode == "format" || *mode == "incr" {
			value := []xlib.Atom{4}
			typ := r.Target
			if *mode == "incr" {
				typ = d.InternAtom("INCR", false)
			}
			d.ChangeProperty(r.Requestor, property, typ, 32, xlib.PropModeReplace, cgo.CSlice(value), len(value))
		} else {
			data := []byte("外部 é")
			if *mode == "latin1" {
				data = []byte("caf\xe9")
			}
			if *mode == "fallback" {
				data = []byte("\xe9")
			}
			if *mode == "invalid" {
				data = []byte{0xff}
			}
			if *mode == "oversized" {
				data = make([]byte, 1<<20)
				for i := 0; i < 17; i++ {
					mode := xlib.PropModeAppend
					if i == 0 {
						mode = xlib.PropModeReplace
					}
					d.ChangeProperty(r.Requestor, property, r.Target, 8, mode, cgo.CSlice(data), len(data))
				}
			} else {
				d.ChangeProperty(r.Requestor, property, r.Target, 8, xlib.PropModeReplace, cgo.CSlice(data), len(data))
			}
		}
		var reply xlib.Event
		sent := reply.SelectionEvent()
		sent.Type, sent.Requestor, sent.Selection, sent.Target, sent.Property, sent.Time = xlib.SelectionNotify, r.Requestor, r.Selection, r.Target, property, r.Time
		d.SendEvent(r.Requestor, false, 0, &reply)
		d.Flush()
	}
}
