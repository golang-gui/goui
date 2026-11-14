//go:generate go run generate.go

// 原生预览失败降级验收，不使用DevServer。
//
// 环境：Linux/X11、Windows、macOS可交互桌面；记录后端、WM和倍率。
// 启动：go run ./tests/window/gui/drag_preview [-handoff] [-close-source] [-invalid]
// 默认两个窗口，左SOURCE有蓝色矩形预览，右TARGET接受同一Go对象；
// -text独立验证便携文本，目标仅接收当前协商格式。
// 设置GOUI_GENERATE_DIR为仓库外绝对路径，执行go generate ./tests/window/gui/drag_preview。
// 然后运行生成目录/tests/window/gui/drag_preview/inject.py，并用--repo指定GOUI仓库。
// 该脚本生成Go overlay，分别模拟预览创建失败与部分资源创建后失败；
// 正常构建、故障构建均执行相同步骤，不设置生产环境故障开关。
//
// 操作与逐项预期：
//  1. 左窗按住Drag，移过阈值后拖到右窗释放：一次按下即可成功Move；
//     Begin/End各一次、Drop一次，Go对象身份或文本不变，无Click或错误。
//  2. 再从左窗起拖，保持左键按住并按Esc，再释放：第二次End为取消，无Drop。
//  3. 再次拖入右窗：第三次成功，证明会话／指针清理后能立即重复使用。
//     自动断言、打印PASS并退出0；提前关闭或断言失败退出1。
//  4. -handoff使用DragEventController交接同一次手势，重复1至3。
//  5. -close-source单独运行：第一次原生Begin后自动销毁源窗，必须结束且不Drop。
//  6. -invalid单独运行：预览Scale=-1；第一次尝试必须报参数错误，不发原生Begin。
//     session故障overlay配合-startup-error，确认真正的原生启动错误仍会返回。
//
// 故障构建应只有动作光标，正常构建应显示蓝色预览。macOS先激活源窗口；
// 全无图像NSDraggingItem源必须通过实际Drop验收，不能仅凭绑定／编译推定。
// macOS可用与程序ID一致的org.golanggui.DragPreview测试.app启动；自动化须先
// 确认窗口出现在实际画面中，再起拖，并等待TARGET日志后才在目标上释放。
// 输入始终经正常平台分发；程序只读Snapshot，不调用语义动作或伪造原生事件。
// 副作用：仅本例窗口和日志，不写文件／剪贴板／设置。复位：重启；退出：完成流程。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
)

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL", err)
		os.Exit(1)
	}
}

func run() error {
	handoff := flag.Bool("handoff", false, "Use an explicit pointer gesture handoff")
	closeSource := flag.Bool("close-source", false, "Destroy the source during native drag")
	invalid := flag.Bool("invalid", false, "Reject an invalid preview parameter")
	startupError := flag.Bool("startup-error", false, "Expect an injected native session error")
	textMode := flag.Bool("text", false, "Transfer portable text instead of a local object")
	flag.Parse()
	if *invalid && (*handoff || *closeSource) {
		return errors.New("invalid mode must run separately")
	}
	app, err := gui.NewApplication("org.golanggui.DragPreview")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	options := &gui.WindowOptions{Size: geometry.Size{Width: 360, Height: 220}}
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
	_ = src.SetTitle("GOUI preview SOURCE")
	_ = dst.SetTitle("GOUI preview TARGET")
	src.ConnectCloseRequest(func(*bool) { app.Quit() })
	dst.ConnectCloseRequest(func(*bool) { app.Quit() })
	button := gui.NewButton()
	button.SetChild(gui.NewLabel("Drag: drop, Escape, drop again"))
	button.SetMinSize(geometry.Size{Height: 80})
	column := gui.NewLinearBox(layout.DirectionVertical)
	column.SetCrossAlign(layout.CrossStretch)
	column.SetPadding(geometry.UniformInsets(12))
	column.AddChild(button)
	src.SetWidget(column)
	receiver := gui.NewButton()
	receiver.SetChild(gui.NewLabel("TARGET: same object and text"))
	dst.SetWidget(receiver)
	value := new(int)
	*value = 42
	data := new(gui.DragData)
	data.SetLocal("preview-probe", value)
	if *textMode {
		data.SetText("preview payload")
	}
	pixels := image.NewRGBA(image.Rect(0, 0, 60, 36))
	for y := range 36 {
		for x := range 60 {
			pixels.SetRGBA(x, y, color.RGBA{R: 30, G: 100, B: 220, A: 255})
		}
	}
	preview := gui.DragPreview{Image: pixels, Scale: 1, Hotspot: geometry.Point{X: -10, Y: -10}}
	if *invalid {
		preview.Scale = -1
	}
	begins, ends, drops, clicks, pointerEnds, pointerCancels := 0, 0, 0, 0, 0, 0
	finished := false
	var failure error
	fail := func(err error) { failure = err; app.Post(app.Quit) }
	button.ConnectClicked(func() { clicks++ })
	source := gui.NewDragSource()
	source.SetActions(gui.DragMove)
	source.ConnectPrepare(func(r *gui.DragPrepare) { r.Data, r.Preview = data, preview })
	source.ConnectBegin(func() {
		begins++
		fmt.Printf("BEGIN %d\n", begins)
		if *closeSource {
			app.Post(src.Destroy)
		}
	})
	source.ConnectEnd(func(r gui.DragResult) {
		ends++
		fmt.Printf("END %d %+v drops=%d\n", ends, r, drops)
		if *invalid || *startupError {
			if begins != 0 || ends != 1 || drops != 0 || r.Err == nil {
				fail(errors.New("invalid preview started a native drag or lost its error"))
				return
			}
			finished = true
			app.Post(app.Quit)
			return
		}
		if err := r.Validate(); err != nil || r.Err != nil || begins != ends {
			fail(fmt.Errorf("invalid/failed result: %+v validate=%v", r, err))
			return
		}
		if *closeSource {
			if ends != 1 || drops != 0 || !r.Canceled {
				fail(errors.New("source destruction did not cancel the session"))
				return
			}
			finished = true
		} else if ends == 2 {
			if !r.Canceled || drops != 1 {
				fail(errors.New("Escape did not cancel exactly one drag"))
				return
			}
		} else {
			want := 1
			if ends == 3 {
				want = 2
			}
			if r.Action != gui.DragMove || r.Canceled || drops != want || !r.LocalDrop {
				fail(errors.New("drop did not complete the original drag"))
				return
			}
			finished = ends == 3
		}
		if finished {
			app.Post(app.Quit)
		}
	})
	if *handoff {
		gesture := gui.NewDragEventController()
		gesture.SetThreshold(4)
		gesture.ConnectUpdate(func(_ gui.EventContext, _ gui.DragInfo) {
			if err := source.Begin(gesture, data, preview); err != nil {
				fail(err)
			}
		})
		gesture.ConnectEnd(func(_ gui.EventContext, _ gui.DragInfo) {
			pointerEnds++
		})
		gesture.ConnectCanceled(func(_ gui.GestureCancelReason) {
			pointerCancels++
		})
		button.AddEventController(gesture)
	} else {
		button.AddEventController(source)
	}
	format := gui.LocalFormat("preview-probe")
	if *textMode {
		format = gui.DragFormatText
	}
	target := gui.NewDropTarget(format)
	target.SetActions(gui.DragMove)
	target.ConnectEnter(func(r *gui.DragMotion) { fmt.Printf("TARGET %d format=%s\n", begins, r.Format) })
	target.ConnectDrop(func(r *gui.DropRequest) {
		got, ok := r.Data.Local("preview-probe")
		text, textOK := r.Data.Text()
		if (*textMode && (!textOK || text != "preview payload")) || (!*textMode && (!ok || got != value)) || r.Action != gui.DragMove {
			fail(errors.New("payload identity, text or action changed"))
			return
		}
		drops++
		r.Accepted = true
		fmt.Printf("DROP %d\n", drops)
	})
	target.ConnectError(fail)
	receiver.AddEventController(target)
	for i, w := range []gui.Window{src, dst} {
		ready := false
		w.ConnectAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
			if ready {
				return
			}
			ready = true
			position, _ := w.Position(nil)
			bounds := w.Widget().Snapshot().Bounds
			if i == 0 {
				bounds = button.Snapshot().Bounds
			}
			blob, _ := json.Marshal(struct {
				Position geometry.Point
				Bounds   geometry.Rectangle
			}{position, bounds})
			resource, err := p.NewImage(pixels)
			if err != nil {
				fail(err)
				return
			}
			defer resource.Destroy()
			fmt.Printf("READY %d scale=%g geometry=%s backend=%T\n", i, p.PixelScale(), blob, resource)
		})
		if err := w.Show(); err != nil {
			return err
		}
		if err := w.SetPosition(nil, geometry.Point{X: 60 + float32(i)*450, Y: 100}); err != nil {
			return err
		}
	}
	if err := app.Run(); err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	if !finished || clicks != 0 || pointerEnds != 0 || pointerCancels != 0 {
		return fmt.Errorf("unfinished/extra input: finished=%t clicks=%d pointer=%d/%d", finished, clicks, pointerEnds, pointerCancels)
	}
	fmt.Println("PASS drag preview")
	return nil
}
