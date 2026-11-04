package main

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/diagnostics"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

// HUD 专项；通过主入口 -hud 选择，默认自动断言，-auto=false 手工运行。
// 环境：独立桌面，Software／OpenGL（Windows 另验 Direct2D），1x／2x。
// 初始：采集已开启，左上 HUD 遮住 Count 按钮的一部分；底部标签显示计数。
//  1. 点击浮层覆盖处的 Count：计数增加，浮层不拦截点击，也不移动其它控件。
//  2. 点击 Animate：约两秒内连续绘制，HUD 的 FPS／毫秒数和后端名称可见更新。
//     数字表示最近 CPU 帧；本例不检查显示刷新率，也不要求精确 60 FPS。
//  3. 停止操作两秒：没有持续帧请求，浮层保留最近读数；不是实时屏幕 FPS。
//  4. Move HUD 移动浮层，Toggle HUD 移除／重新显示，原先覆盖处完整重绘。
//  5. Stop capture：浮层消失，Count／Animate 继续有效；再次开始采集可重新显示。
//  6. 改变窗口大小，检查文字／背景裁剪和倍率；关闭窗口退出，重启复位。
//
// 自动模式补充断言：RenderWidget 结果不受 HUD 影响、后绘制回调仍执行、
// 闲置帧和资源数稳定、采集结果冻结、停止后拒绝 ShowHUD、退出清理。
// 像素比较仅比较同后端同次运行的离屏预乘 RGBA，不使用跨平台字体金图。
// 副作用仅为本例窗口与终端输出，不启动 DevServer、不写文件。
func runHUD(app gui.Application, auto bool) {
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 600, Height: 420}})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("GOUI diagnostics HUD"))
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(geometry.UniformInsets(20))
	root.SetSpacing(12)
	count := 0
	status := gui.NewLabel("Count 0")
	countButton := button("Count (click through HUD)", func() {
		count++
		status.SetText(fmt.Sprintf("Count %d", count))
	})
	root.AddChild(countButton)
	root.AddChild(gui.NewLabel("HUD: recent CPU frames, not display FPS.\nIdle retains the last reading without a refresh loop."))
	root.AddChild(status)
	var capture *diagnostics.Capture
	var hud *diagnostics.HUD
	start := func() {
		if capture == nil || !capture.Snapshot().Active {
			capture, err = diagnostics.Start(app, diagnostics.Options{Widgets: true})
			must(err)
		}
		hud, err = capture.ShowHUD(win)
		must(err)
	}
	root.AddChild(button("Toggle HUD / Start capture", func() {
		if hud != nil {
			hud.Destroy()
			hud = nil
		} else {
			start()
		}
	}))
	root.AddChild(button("Move HUD", func() {
		if hud != nil {
			hud.SetPosition(geometry.Point{X: 320, Y: 50})
		}
	}))
	var frames signal.Handle
	ticks := 0
	animate := func() {
		if frames != nil {
			frames.Disconnect()
		}
		ticks = 0
		var started time.Time
		frames = win.ConnectFrame(func(now time.Time) {
			if started.IsZero() {
				started = now
			}
			ticks++
			if now.Sub(started) < 2*time.Second {
				must(win.RequestPaint())
			} else {
				frames.Disconnect()
				frames = nil
			}
		})
		must(win.RequestPaint())
	}
	root.AddChild(button("Animate 2 seconds", animate))
	root.AddChild(button("Stop capture", func() {
		capture.Stop()
		hud = nil
	}))
	win.SetWidget(root)
	must(win.Show())
	start()
	if !auto {
		app.Run()
		return
	}
	paints := 0
	var scale float32
	win.ConnectAfterPaint(func(p gui.Painter, _ geometry.Rectangle) { paints++; scale = p.PixelScale() })
	var before []byte
	var settled diagnostics.Snapshot
	var frozen diagnostics.Snapshot
	var priorPaints int
	var sampledLayouts uint64
	var sampledAt time.Time
	step := 0
	timer := app.NewTimer()
	timer.ConnectTimeout(func() {
		switch step {
		case 0:
			s := capture.Snapshot()
			require(total(s) > 0 && len(s.Roots) == 1 && s.Roots[0].Completed > 0, "HUD initial frame missing")
			if requested := os.Getenv("GOUI_PLAT_PAINTER"); requested != "" {
				require(strings.EqualFold(requested, s.Roots[0].Backend), "requested backend was not selected")
			}
			img, err := gui.RenderWidget(root, 1)
			must(err)
			before = append([]byte(nil), graphics.CopyToBitmap(img, graphics.PixelFormatRGBA, nil).Pixels...)
			hud.Destroy()
			hud = nil
		case 1:
			img, err := gui.RenderWidget(root, 1)
			must(err)
			require(bytes.Equal(before, graphics.CopyToBitmap(img, graphics.PixelFormatRGBA, nil).Pixels), "HUD leaked into RenderWidget")
			start()
			// 首按钮局部位置加容器偏移，通过正常 PointerEvent 路径验证透传。
			position := countButton.Rect().Pos.Add(root.Rect().Pos).Add(geometry.Point{X: 12, Y: 12})
			for _, kind := range []events.EventType{events.PointerMove, events.PointerDown, events.PointerUp} {
				must(win.DispatchEvent(events.PointerEvent{EventType: kind, Position: position, Button: events.PointerButtonLeft}))
			}
			require(count == 1, "HUD intercepted button input")
			animate()
		case 2:
			sampledLayouts, sampledAt = capture.Snapshot().Resources.LayoutsCreated, time.Now()
		case 3, 4, 5, 6, 7, 8:
			// 等待两秒主动帧结束，不修改内容以免干扰资源复用检查。
		case 9:
			require(ticks > 5 && frames == nil, "animation did not complete")
			limit := uint64(time.Since(sampledAt)/(250*time.Millisecond)) + 1
			require(capture.Snapshot().Resources.LayoutsCreated-sampledLayouts <= limit, "HUD rebuilt text more than four times per second")
			hud.SetPosition(geometry.Point{X: 300, Y: 45})
		case 10:
			settled = capture.Snapshot()
			require(settled.Resources.LayoutsCreated > 0 && settled.Roots[0].Backend != "", "HUD resources or backend missing")
		case 11, 12, 13, 14:
			s := capture.Snapshot()
			require(total(s) == total(settled), "HUD requested idle frames; check external exposure/input")
			require(s.Resources == settled.Resources, "HUD rebuilt resources while idle")
		case 15:
			capture.Stop()
			frozen = capture.Snapshot()
			if _, err := capture.ShowHUD(win); err == nil {
				panic("stopped capture accepted HUD")
			}
			priorPaints = paints
			must(win.RequestPaint())
		case 16:
			require(paints > priorPaints && reflect.DeepEqual(frozen, capture.Snapshot()), "stop changed snapshot or disabled painting")
			start()
			timer.Stop()
			app.Quit() // 保留窗口到 Run 返回，专门检查 StopOwner 路径。
		}
		step++
	})
	must(timer.Start(300 * time.Millisecond))
	app.Run()
	require(step == 17 && !capture.Snapshot().Active, "owner exit did not stop HUD capture")
	// 退出后的旧 HUD 句柄必须安全、幂等，不再连接窗口绘制。
	hud.Destroy()
	hud.Destroy()
	fmt.Printf("PASS HUD ticks=%d paints=%d backend=%s scale=%g\n", ticks, paints, settled.Roots[0].Backend, scale)
}
