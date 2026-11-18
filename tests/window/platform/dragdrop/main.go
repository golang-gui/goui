// Drag and Drop 原生文件传输窗口验证。
//
// 环境：Linux/X11、Windows 或 macOS 桌面，需可操作的文件管理器；记录 OS、
// Linux 桌面环境/WM、实际 Painter 和缩放比例。每个平台分别验收。
// 启动：先在独立临时目录准备一个可识别内容的文件，然后按场景启动：
//
//	go run ./tests/window/platform/dragdrop -expect /absolute/test/file
//	go run ./tests/window/platform/dragdrop -source /absolute/test/file
//	go run ./tests/window/platform/dragdrop -source /absolute/test/file -result cancel
//	go run ./tests/window/platform/dragdrop -source /absolute/test/file -result reject
//	go run ./tests/window/platform/dragdrop -source /absolute/test/file -preview=false
//
// Windows PowerShell 可使用绝对路径；-expect 与 -source 不能同时指定。
// 初始：640×360 原生窗口；接收模式为绿色客户区，源模式为蓝色客户区。
// 窗口标题和终端说明模式、实际 Painter 及缩放；本例不使用 GUI Widget。
//
// 操作与预期：
//  1. -expect 模式：从文件管理器将准备的单个文件拖到绿色区域。
//     Enter/Motion 提供 Files 和 Copy；释放后打印与 -expect 相同的绝对
//     路径、一次 PASS，并以退出码 0 结束。路径不符或读取失败应非零退出。
//  2. 不带 -expect 重启接收模式，连续拖入文件或目录；每次打印实际路径，
//     窗口保持打开。将文件释放到窗口外或按 Esc，不得打印新的 PASS。
//  3. -source 默认模式：按住蓝色区域并移动超过 4 DIP，拖入文件管理器的
//     空目录。Begin/End 各一次，End 为 Copy、退出码 0；独立比较目标文件
//     与源文件内容，并确认源仍存在。程序只上报传输结果，不核对文件内容。
//  4. -result cancel：从蓝色区域开始拖动，按 Esc。End 仅一次、Action=0，
//     退出码 0，目标目录不应新增文件。Linux/Windows 要求 canceled=true；
//     macOS 若原生只报告 Action=0，程序打印说明，需人工确认确实按了 Esc。
//  5. -result reject：拖动后释放到无接收者区域。End 仅一次、Action=0、
//     canceled=false、退出码 0，目标目录不应新增文件。
//  6. -source 默认显示 48 DIP 蓝黄自定义预览，四周透明、黑色十字
//     (12,12) DIP 应位于指针位置；1x/2x 核对逻辑尺寸、热点和透明边缘。
//     以 -preview=false 重启，确认使用原生回退且文件传输结果不变。
//
// 平台差异：原生回退、指针和文件管理器外观由系统决定，分别记录；
// 无头测试、交叉编译不计为桌面互操作通过。
// 副作用：程序仅读取路径，不复制、移动或删除文件，不改写剪贴板。
// 文件管理器的复制由操作者触发，目标副本由操作者清理。
// 复位：重启程序，步骤 3–5 每次使用空目标目录。退出：关闭窗口；
// -expect 成功及 -source 收到终态会自动退出，失败以非零退出码报告。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/events"
)

func main() {
	runtime.LockOSThread()
	expect := flag.String("expect", "", "absolute file path expected from the native drop")
	source := flag.String("source", "", "absolute file path to offer to an external target")
	want := flag.String("result", "copy", "source result expected: copy, cancel, reject")
	preview := flag.Bool("preview", true, "show a 48 DIP image with a marked 12 DIP hotspot in source mode")
	flag.Parse()
	if *expect != "" && *source != "" {
		fmt.Fprintln(os.Stderr, "-expect and -source are mutually exclusive")
		os.Exit(2)
	}
	var err error
	if *source != "" {
		path, pathErr := filepath.Abs(*source)
		if pathErr != nil {
			err = pathErr
		} else {
			err = runSource(path, *want, *preview)
		}
	} else {
		if *want != "copy" {
			fmt.Fprintln(os.Stderr, "-result requires -source")
			os.Exit(2)
		}
		if *expect != "" {
			*expect, err = filepath.Abs(*expect)
		}
		if err == nil {
			err = run(*expect)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(expected string) error {
	p, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.DragDropProbe")
	if err != nil {
		return err
	}
	defer p.Destroy()
	loop, err := p.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	var active dragdrop.Offer
	var resultErr error
	var received bool
	var canvas nativeCanvas
	w, err := p.NewWindow(geometry.Size{Width: 640, Height: 360}, func(event events.Event) {
		canvas.event(event)
		switch e := event.(type) {
		case events.CloseEvent:
			loop.Quit()
		case events.DragOfferEvent:
			switch e.EventType {
			case events.DragEnter, events.DragMotion:
				if slices.Contains(e.Offer.Formats(), dragdrop.FormatFiles) && e.Actions&dragdrop.Copy != 0 && (!e.Forced || e.Suggested == dragdrop.Copy) {
					*e.ActionReply = dragdrop.Copy
				}
				fmt.Printf("offer %d %v at %v, formats=%v\n", e.Offer.ID(), e.EventType, e.Position, e.Offer.Formats())
			case events.DragLeave:
				fmt.Printf("offer %d leave\n", e.Offer.ID())
				// Drop 已提交的读取可能异步完成。
			case events.DragDrop:
				active = e.Offer
				offer := active // Read 可能通过同步 DragData 回调清空 active。
				if err := offer.Read(dragdrop.FormatFiles); err != nil {
					resultErr = fmt.Errorf("read files: %w", err)
					_ = offer.Finish(0)
					loop.Quit()
				}
			}
		case events.DragDataEvent:
			if active == nil || e.OfferID != active.ID() {
				resultErr = fmt.Errorf("unexpected offer data id %d", e.OfferID)
				loop.Quit()
				return
			}
			if e.Err != nil || e.Data == nil {
				resultErr = fmt.Errorf("read offer %d: %v", e.OfferID, e.Err)
				_ = active.Finish(0)
				loop.Quit()
				return
			}
			files, ok := e.Data.Files()
			if !ok || len(files) == 0 {
				resultErr = fmt.Errorf("offer %d had no files", e.OfferID)
				_ = active.Finish(0)
				loop.Quit()
				return
			}
			fmt.Printf("files: %q\n", files)
			if expected != "" && (len(files) != 1 || files[0] != expected) {
				resultErr = fmt.Errorf("files %q, expected [%q]", files, expected)
				_ = active.Finish(0)
				loop.Quit()
				return
			}
			if err := active.Finish(dragdrop.Copy); err != nil {
				resultErr = err
				loop.Quit()
				return
			}
			active = nil
			received = true
			fmt.Println("PASS: native file drop accepted")
			if expected != "" {
				loop.Post(loop.Quit) // 先让原生 Drop 返回，再释放 Surface。
			}
		}
	}, platform.WindowOptions{})
	if err != nil {
		return err
	}
	defer w.Destroy()
	if err := canvas.init(p, w, false); err != nil {
		return err
	}
	defer canvas.painter.Destroy()
	if err := w.(platform.WindowManagement).SetTitle("GOUI DnD receiver"); err != nil {
		return err
	}
	capability, err := p.NewDragDrop(w)
	if err != nil {
		return err
	}
	defer capability.Destroy()
	if err := capability.SetFormats([]dragdrop.Format{dragdrop.FormatFiles}); err != nil {
		return err
	}
	if err := w.(platform.WindowManagement).Show(); err != nil {
		return err
	}
	loop.Run()
	if resultErr != nil {
		return resultErr
	}
	if expected != "" && !received {
		return fmt.Errorf("window closed without expected native file drop")
	}
	return nil
}
