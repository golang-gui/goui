// 栏内拖动交给原生 DnD 的独立窗口验证；不使用 DevServer。
//
// 环境：Linux/X11、Windows 或 macOS 可交互桌面，记录后端、WM、缩放。
// 启动：go run ./tests/window/gui/drag_handoff -expect accepted
// 可选 -chrome integrated；默认 Native。GOUI_PLAT_SCALE=2 可复测 2x。
// 初始：左侧 SOURCE 含一个拖动条，右侧 TARGET 为接收区；只测试一轮。
//
// 操作与预期：
//  1. -expect local：按住源按钮，水平移动超过 4 DIP 后仍在条内释放。
//     预期本地 Begin/End 各一次，无原生 Begin/End，无 Click。
//  2. 默认 accepted：先在条内水平移动，再向下移出条，保持左键拖入右窗释放。
//     预期本地 Begin 一次且无本地 End/Cancel；原生 Begin/End 各一次；
//     目标收到相同 Go 对象，Move 成功，无额外 Click，预览来自 RenderWidget。
//  3. -expect canceled：同上移出条并进入右窗，保持左键按下，按 Esc 再松手。
//     预期原生结果 Canceled=true、无 Drop、无有效位置、无 Click。
//  4. -expect unaccepted：移出条后在左窗按钮下方的空白客户区释放。
//     预期 Action=0、非取消、位置有效，无 Drop、无 Click。
//  5. -expect click：短点击按钮，只有一次 Click，本地/原生拖动计数均为零。
//
// 平台差异：AppKit 需要激活窗口后再拖；原生预览与取消动画由系统决定。
// Integrated 下拖动条不得移动窗口，空 HeaderBar 区域仍用于移动窗口。
// 副作用：只创建本例窗口，不读写文件、不修改剪贴板或系统设置。
// 复位：重新启动。退出：一次操作结束自动退出；未操作就关闭窗口判为未完成。
// 所有计数和数据身份自动断言；位置精度和预览外观需结合截图独立核对。
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/modern"
)

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	expect := flag.String("expect", "accepted", "accepted, canceled, unaccepted, local, click")
	chrome := flag.String("chrome", "native", "native or integrated")
	flag.Parse()
	if !slices.Contains([]string{"accepted", "canceled", "unaccepted", "local", "click"}, *expect) ||
		!slices.Contains([]string{"native", "integrated"}, *chrome) {
		return fmt.Errorf("invalid expectation or chrome")
	}
	app, err := gui.NewApplication("org.golang-gui.DragHandoffProbe")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	mode := gui.WindowChromeNative
	if *chrome == "integrated" {
		mode = gui.WindowChromeIntegrated
	}
	options := &gui.WindowOptions{Size: geometry.Size{Width: 400, Height: 260}, Chrome: mode}
	src, err := app.NewWindow(options)
	if err != nil {
		return err
	}
	defer src.Destroy()
	dst, err := app.NewWindow(options)
	if err != nil {
		return err
	}
	defer dst.Destroy()
	_ = src.SetTitle("GOUI handoff SOURCE")
	_ = dst.SetTitle("GOUI handoff TARGET")
	src.ConnectCloseRequest(func(*bool) { app.Quit() })
	dst.ConnectCloseRequest(func(*bool) { app.Quit() })
	localBegins, localEnds, localCancels, begins, ends, drops, clicks := 0, 0, 0, 0, 0, 0, 0
	var result gui.DragResult
	var failure error
	finish := func() { app.Post(app.Quit) }
	fail := func(err error) { failure = err; finish() }
	button := gui.NewButton()
	button.SetChild(gui.NewLabel("Drag here, then leave this strip"))
	button.SetMinSize(geometry.Size{Height: 70})
	button.ConnectClicked(func() { clicks++; finish() })
	column := gui.NewLinearBox(layout.DirectionVertical)
	column.SetCrossAlign(layout.CrossStretch)
	column.SetPadding(12)
	column.SetSpacing(12)
	if mode == gui.WindowChromeIntegrated {
		header := gui.NewHeaderBar()
		header.SetChild(gui.NewLabel("Handoff source"))
		column.AddChild(header)
	}
	column.AddChild(button)
	column.AddChild(gui.NewLabel("Release here for unaccepted"))
	src.SetWidget(column)
	receiver := gui.NewButton()
	receiver.SetChild(gui.NewLabel("Drop local object here"))
	dst.SetWidget(receiver)
	value := new(int)
	*value = 42
	source := gui.NewDragSource()
	source.SetActions(gui.DragMove)
	source.ConnectBegin(func() { begins++; fmt.Println("native Begin") })
	source.ConnectEnd(func(r gui.DragResult) {
		ends++
		result = r
		fmt.Printf("native End %+v\n", r)
		finish()
	})
	drag := gui.NewDragEventController()
	drag.SetThreshold(4)
	var hotspot geometry.Point
	drag.ConnectBegin(func(p geometry.Point, _ events.Modifiers) {
		localBegins++
		hotspot = p
		fmt.Println("local Begin")
	})
	drag.ConnectUpdate(func(p geometry.Point, _ events.Modifiers) {
		if p.X >= 0 && p.Y >= 0 && p.X < button.Rect().Width && p.Y < button.Rect().Height {
			return
		}
		bitmap, err := gui.RenderWidget(button, 1)
		if err != nil {
			fail(err)
			return
		}
		data := new(gui.DragData)
		data.SetLocal("handoff", value)
		if err := source.Begin(drag, data, gui.DragPreview{Image: bitmap, Scale: 1, Hotspot: hotspot}); err != nil {
			fail(err)
		}
	})
	drag.ConnectEnd(func(geometry.Point, events.Modifiers) { localEnds++; finish() })
	drag.ConnectCancel(func() { localCancels++ })
	button.AddEventController(drag)
	target := gui.NewDropTarget(gui.LocalFormat("handoff"))
	target.SetActions(gui.DragMove)
	target.ConnectDrop(func(e *gui.DropRequest) {
		drops++
		got, ok := e.Data.Local("handoff")
		if !ok || got != value || e.Action != gui.DragMove {
			fail(fmt.Errorf("incorrect local payload/action"))
			return
		}
		e.Accepted = true
		fmt.Println("Drop: same Go object")
	})
	target.ConnectError(fail)
	receiver.AddEventController(target)
	if err := src.Show(); err != nil {
		return err
	}
	if err := dst.Show(); err != nil {
		return err
	}
	if err := src.SetPosition(nil, geometry.Point{X: 60, Y: 100}); err != nil {
		return err
	}
	if err := dst.SetPosition(nil, geometry.Point{X: 540, Y: 100}); err != nil {
		return err
	}
	fmt.Printf("OS=%s chrome=%s requested painter=%s scale=%s expect=%s\n", runtime.GOOS, *chrome,
		os.Getenv("GOUI_PLAT_PAINTER"), os.Getenv("GOUI_PLAT_SCALE"), *expect)
	app.Run()
	fmt.Printf("counts local=%d/%d cancel=%d native=%d/%d drop=%d click=%d\n", localBegins, localEnds, localCancels, begins, ends, drops, clicks)
	if failure != nil {
		return failure
	}
	if *expect == "click" {
		if clicks != 1 || localBegins+localEnds+localCancels+begins+ends+drops != 0 {
			return fmt.Errorf("incorrect click counters")
		}
	} else if *expect == "local" {
		if localBegins != 1 || localEnds != 1 || localCancels+begins+ends+drops+clicks != 0 {
			return fmt.Errorf("incorrect local drag counters")
		}
	} else {
		if localBegins != 1 || begins != 1 || ends != 1 || localEnds+localCancels+clicks != 0 {
			return fmt.Errorf("incorrect handoff counters")
		}
		if err := result.Validate(); err != nil {
			return err
		}
		switch *expect {
		case "accepted":
			if drops != 1 || result.Action != gui.DragMove || !result.PositionValid {
				return fmt.Errorf("expected successful Move: %+v", result)
			}
		case "canceled":
			if drops != 0 || !result.Canceled {
				return fmt.Errorf("expected cancellation: %+v", result)
			}
		case "unaccepted":
			if drops != 0 || result.Action != 0 || result.Canceled || result.Err != nil || !result.PositionValid {
				return fmt.Errorf("expected unaccepted release: %+v", result)
			}
		}
	}
	fmt.Println("PASS", *expect)
	return nil
}
