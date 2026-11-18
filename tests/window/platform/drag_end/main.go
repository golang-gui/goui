// 原生拖放结束结果验证（不是 TabView 跨窗口转移演示）。
//
// 环境：可交互的 Linux/X11、Windows 或 macOS 桌面；记录系统、Linux 桌面/
// 窗口管理器，以及启动日志中的 Painter 和缩放。macOS 在真正的主线程运行。
// 启动：go run ./tests/window/platform/drag_end -target accept -expect accepted
// 初始：蓝色源窗口、绿色目标窗口；-target none 时目标为未注册拖放的灰色窗口。
// 在蓝色客户区按住左键拖动，不拖原生标题栏；预览为 48 DIP，十字热点为 (12,12)。
// 每次运行只验证一次源拖放，End 后自动退出；以下场景分别重新启动。
//
// 操作与预期：
//  1. 接受：拖入绿色区域释放。默认 -expect accepted；Begin/End 各一次，
//     Copy、非取消、无错误、有效结束位置；目标准确收到 "GOUI drag-end probe"。
//  2. 无接收者：-target none -expect unaccepted，拖入灰色区域释放。
//     预期 Action=0、非取消、无错误、有效结束位置，没有目标 Drop。
//     不用桌面或文件管理器代替灰色区域，它们可能支持接收。
//  3. 拒绝：-target format/action/drop（任选其一）-expect unaccepted。
//     分别拒绝格式、动作和最终 Drop；不得成功传输，源结果与第 2 项一致。
//     不分类外部拒绝原因；使用 -local-target true 验证最终命中本进程注册窗口。
//     format/action 拒绝时即使没有 Drop，LocalTarget 也必须为 true。
//  4. Esc：-expect canceled，拖入绿色区域，保持左键按下，按下并松开 Esc，
//     最后松开左键。预期 Begin/End 各一次、Canceled=true、无 Drop、无有效位置。
//  5. 释放位置：-target none -expect unaccepted，释放后立刻将指针移远。
//     对照日志的 Position（源客户区 DIP）与实际释放点，在 1x/2x 分别检查。
//     程序只断言位置有效，不自动宣称坐标正确；不能直接与屏幕物理像素比较。
//  6. 程序取消/源销毁：-interrupt cancel 或 destroy，拖动保持超过一秒。
//     Windows/X11 使用 -expect canceled；AppKit 取消可能延迟到松手，若目标
//     仍成功接受，使用 -expect accepted：成功 Copy 不能被取消标志覆盖。
//     源销毁后的 PositionValid 必须为 false。没有实际触发定时操作会报失败。
//     -interrupt invalid：无输入上下文启动必须失败且无 Begin/End。
//  7. 跨进程：分别以 -role target 和 -role source 启动，源选择相应 -expect。
//     目标记录 payload 检查，源记录结束结果；两侧都需核对。目标手动关闭退出。
//     拖入外部编辑器会插入测试文本，请手动撤销。
//  8. 最终目标（各自重新运行）：同进程 accept/format/action/drop 使用
//     -local-target true；none 灰色区域、跨进程目标和 Esc 使用 -local-target false。
//     另以 -target action -expect unaccepted -local-target false 启动，先进入绿色
//     拒绝区域，再移到所有测试窗口外松手；应报告 false，而非保留历史 hover。
//  9. 长时间拖动：追加 -min-duration 35s，拖入目标后保持按住并停留超过 35 秒再释放。
//     分别以 accept/accepted、none/unaccepted、accept/canceled 验证；最后一种在
//     等待结束后按 Esc 再松手。仍应只有一次正确 End，不得在按住期间报超时。
//     程序按 Begin 到 End 的单调时钟断言最小时长，过早释放/取消不能记作通过。
//  10. Linux 无应答目标：用 -role target 单独启动目标进程，源用 -role source
//     -expect error。进入绿色目标后仅暂停该目标进程（SIGSTOP），移动指针再释放；
//     预期源在释放后有界报错，无有效位置，不误报取消或成功。验证结束必须 SIGCONT
//     恢复目标；源退出后目标处理迟到消息不得因 BadWindow 退出。再启动一个
//     -role source -expect accepted，向这个原目标投递，预期两侧 payload/结果
//     检查通过；最后正常关闭目标。此项只暂停本测试创建的目标，不能暂停桌面或源进程。
//
// 平台差异：X11 超时是错误，不是未接受；AppKit 使用当前 Esc 事件与时间范围
// 识别取消，这不是 AppKit 提供的取消原因保证。程序取消不是撤销已完成的 Drop。
// 副作用：仅创建测试窗口，不写 trace、不修改剪贴板或文件。
// 反馈：启动参数、操作场景、系统/后端/缩放、终端输出及必要的截图。
// 复位：重新启动。退出：关闭窗口或 Ctrl-C；未完成源操作就关闭会非零退出。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"runtime"
	"slices"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

const payload = "GOUI drag-end probe"

func main() {
	runtime.LockOSThread()
	target := flag.String("target", "accept", "accept, none, format, action, drop")
	role := flag.String("role", "both", "both, source, target")
	interrupt := flag.String("interrupt", "", "cancel, destroy, invalid (otherwise empty)")
	expect := flag.String("expect", "accepted", "accepted, canceled, unaccepted, error")
	localTarget := flag.String("local-target", "any", "any, true or false: expected final local target")
	minDuration := flag.Duration("min-duration", 0, "minimum actual native drag duration, e.g. 35s")
	flag.Parse()
	var err error
	if !slices.Contains([]string{"accept", "none", "format", "action", "drop"}, *target) ||
		!slices.Contains([]string{"both", "source", "target"}, *role) ||
		!slices.Contains([]string{"", "cancel", "destroy", "invalid"}, *interrupt) ||
		!slices.Contains([]string{"accepted", "canceled", "unaccepted", "error"}, *expect) ||
		!slices.Contains([]string{"any", "true", "false"}, *localTarget) ||
		*minDuration < 0 || (*role == "target" && (*interrupt != "" || *minDuration != 0)) {
		err = fmt.Errorf("invalid target, role, interrupt or expectation")
	} else {
		fmt.Printf("OS=%s arch=%s Go=%s target=%s role=%s interrupt=%s expect=%s\n", runtime.GOOS, runtime.GOARCH, runtime.Version(), *target, *role, *interrupt, *expect)
		err = run(*target, *role, *interrupt, *expect, *localTarget, *minDuration)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "NOT VERIFIED:", err)
		os.Exit(1)
	}
}

func run(mode, role, interrupt, expect, localTarget string, minDuration time.Duration) error {
	p, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.DragEndProbe")
	if err != nil {
		return err
	}
	defer p.Destroy()
	loop, err := p.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	var resultErr error
	var ends, begins int
	var source platform.Window
	var sourceDrag platform.DragDrop
	var timer *time.Timer
	var cleanups []func()
	var sourceDestroyed bool
	var targetAccepted bool
	var drops int
	var interrupted bool
	var sourceResult dragdrop.Result
	var beganAt time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	makeWindow := func(isSource bool) error {
		var painter graphics.Painter
		var size events.SizeEvent
		var drag platform.DragDrop
		var active dragdrop.Offer
		var down geometry.Point
		var pressed, started bool
		fail := func(err error) { resultErr = err; loop.Post(loop.Quit) }
		w, err := p.NewWindow(geometry.Size{Width: 400, Height: 260}, func(event events.Event) {
			switch e := event.(type) {
			case events.CloseEvent:
				loop.Quit()
			case events.SizeEvent:
				size = e
				if e.Width > 0 {
					fmt.Printf("source=%t client=%gx%g scale=%g\n", isSource, e.Width, e.Height, e.PixelWidth/e.Width)
				}
			case events.PaintEvent:
				if painter == nil || size.Width <= 0 || size.Height <= 0 {
					return
				}
				c := graphics.RGB(45, 145, 90)
				if isSource {
					c = graphics.RGB(45, 100, 200)
				} else if mode == "none" {
					c = graphics.RGB(140, 140, 140)
				}
				painter.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
				painter.Clear(c)
				painter.End()
			case events.PointerEvent:
				if !isSource || drag == nil {
					return
				}
				if e.EventType == events.PointerDown && e.Button == events.PointerButtonLeft {
					pressed, down = true, e.Position
				}
				if e.EventType == events.PointerUp {
					pressed = false
				}
				if e.EventType != events.PointerMove || !pressed || started || e.Buttons&events.PointerButtonLeftDown == 0 {
					return
				}
				dx, dy := e.Position.X-down.X, e.Position.Y-down.Y
				if dx*dx+dy*dy < 16 {
					return
				}
				started, pressed = true, false
				data := new(dragdrop.Data)
				data.SetText(payload)
				if err := drag.Begin(1, data, dragdrop.Copy, dragdrop.Preview{Image: preview(), Scale: 1, Hotspot: geometry.Point{X: 12, Y: 12}}, dragdrop.Feedback{}); err != nil {
					fail(err)
				}
			case events.DragSourceEvent:
				fmt.Printf("source type=%v id=%d result=%+v\n", e.EventType, e.ID, e.Result)
				if e.ID != 1 {
					fail(fmt.Errorf("unexpected source ID %d", e.ID))
					return
				}
				if e.EventType == events.DragSourceBegin {
					begins++
					beganAt = time.Now()
					if interrupt == "cancel" || interrupt == "destroy" {
						timer = time.AfterFunc(time.Second, func() {
							loop.Post(func() {
								interrupted = true
								fmt.Println("interrupt:", interrupt)
								if interrupt == "cancel" {
									sourceDrag.Cancel()
								} else if source != nil {
									sourceDestroyed = true
									source.Destroy()
									source = nil
								}
							})
						})
					}
				} else if e.EventType == events.DragSourceEnd {
					ends++
					sourceResult = e.Result
					elapsed := time.Since(beganAt)
					fmt.Printf("drag elapsed=%s minimum=%s\n", elapsed, minDuration)
					if elapsed < minDuration {
						fail(fmt.Errorf("drag ended too early: %s < %s", elapsed, minDuration))
					}
					if err := e.Result.Validate(); err != nil {
						fail(err)
					}
					loop.Post(loop.Quit)
				}
			case events.DragOfferEvent:
				fmt.Printf("target type=%v position=%+v\n", e.EventType, e.Position)
				if e.EventType == events.DragEnter || e.EventType == events.DragMotion {
					if mode != "action" && mode != "format" && slices.Contains(e.Offer.Formats(), dragdrop.FormatText) && e.Actions&dragdrop.Copy != 0 && (!e.Forced || e.Suggested == dragdrop.Copy) {
						*e.ActionReply = dragdrop.Copy
					}
				} else if e.EventType == events.DragDrop {
					drops++
					if mode == "drop" {
						if err := e.Offer.Finish(0); err != nil {
							fail(err)
						}
						return
					}
					active = e.Offer
					if err := active.Read(dragdrop.FormatText); err != nil {
						fail(err)
					}
				}
			case events.DragDataEvent:
				if active == nil || e.OfferID != active.ID() || e.Err != nil || e.Data == nil {
					fail(fmt.Errorf("invalid target read: %+v", e))
					return
				}
				text, ok := e.Data.Text()
				if !ok || text != payload {
					_ = active.Finish(0)
					fail(fmt.Errorf("expected %q, got %q", payload, text))
					return
				}
				targetAccepted = true
				if err := active.Finish(dragdrop.Copy); err != nil {
					fail(err)
					return
				}
				active = nil
				fmt.Println("PASS target payload (does not validate source end reason)")
			}
		}, platform.WindowOptions{})
		if err != nil {
			return err
		}
		// run 拥有全部回调与资源，在原生事件循环返回后释放。
		cleanups = append(cleanups, func() {
			if !isSource || !sourceDestroyed {
				w.Destroy()
			}
		})
		painter, err = p.NewPainter(w)
		if err != nil {
			return err
		}
		cleanups = append(cleanups, painter.Destroy)
		fmt.Printf("source=%t painter=%s\n", isSource, painter.Name())
		if isSource || mode != "none" {
			drag, err = p.NewDragDrop(w)
			if err != nil {
				return err
			}
			cleanups = append(cleanups, drag.Destroy)
		}
		if isSource {
			source, sourceDrag = w, drag
		} else if drag != nil {
			format := dragdrop.FormatText
			if mode == "format" {
				format = dragdrop.MIMEFormat("application/x-goui-probe-absent")
			}
			if err := drag.SetFormats([]dragdrop.Format{format}); err != nil {
				return err
			}
		}
		title := "GOUI drag-end TARGET " + mode
		if isSource {
			title = "GOUI drag-end SOURCE"
		}
		if err := w.(platform.WindowManagement).SetTitle(title); err != nil {
			return err
		}
		if err := w.(platform.WindowManagement).Show(); err != nil {
			return err
		}
		if desktop, ok := w.(platform.DesktopWindow); ok {
			x := float32(60)
			if !isSource {
				x = 540
			}
			if err := desktop.SetPosition(nil, geometry.Point{X: x, Y: 100}); err != nil {
				fmt.Println("position:", err)
			}
		}
		return nil
	}
	defer func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
		cleanups = nil
	}()
	if role != "target" {
		if err := makeWindow(true); err != nil {
			return err
		}
	}
	if role != "source" {
		if err := makeWindow(false); err != nil {
			return err
		}
	}
	if interrupt == "invalid" && sourceDrag != nil {
		err := sourceDrag.Begin(1, nil, dragdrop.Copy, dragdrop.Preview{}, dragdrop.Feedback{})
		if err == nil || begins != 0 || ends != 0 {
			return fmt.Errorf("Begin outside input should fail without callbacks: %v %d/%d", err, begins, ends)
		}
		fmt.Println("PASS invalid Begin:", err)
		return nil
	}
	loop.Run()
	if resultErr != nil {
		return resultErr
	}
	if role == "target" {
		fmt.Println("Target closed; check the source process result separately.")
		return nil
	}
	if (interrupt == "cancel" || interrupt == "destroy") && !interrupted {
		return fmt.Errorf("interruption did not run; keep the mouse pressed for at least one second")
	}
	if err := verifyResult(expect, role == "both", sourceDestroyed, begins, ends, drops, targetAccepted, sourceResult); err != nil {
		return err
	}
	if localTarget != "any" && sourceResult.LocalTarget != (localTarget == "true") {
		return fmt.Errorf("expected LocalTarget=%s, got %+v", localTarget, sourceResult)
	}
	fmt.Println("PASS public drag result; compare Position with the actual release point separately.")
	return nil
}

func preview() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 48, 48))
	for y := 4; y < 44; y++ {
		for x := 4; x < 44; x++ {
			c := color.RGBA{R: 30, G: 100, B: 200, A: 230}
			if x >= 24 {
				c = color.RGBA{R: 230, G: 180, B: 20, A: 230}
			}
			if (x == 12 && y >= 8 && y <= 16) || (y == 12 && x >= 8 && x <= 16) {
				c = color.RGBA{A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}
