// 帧信号与数值动画窗口验收。
//
// 环境：Linux/Windows/macOS 桌面；记录后端、缩放及 Linux 窗口管理器。
// 启动：go run ./tests/window/gui/animation；-auto 执行公开 API 的时序断言后退出。
// GOUI_PLAT_PAINTER=software/opengl（Windows 另可 direct2d），GOUI_PLAT_SCALE=2。
// 初始：一枚静止圆形、关闭的开关，Popover 未打开，帧观察者不主动请求绘制。
// 操作与预期：
//  1. Start 同时改变圆形大小和位置，平滑前进，停止后 Check 输出 IDLE PASS。
//     动画期间再次 Start 从当前值重定向；Stop 保留当前值，Finish 到终值。
//  2. 点击开关，胶囊滑块在 120ms 内过渡，动画结束后 Check 不出现持续帧。
//  3. Popup 开启圆形忙等待，持续顺时针旋转；再点 Popup 关闭，帧数停止增加。
//     再次打开应恢复；隐藏/恢复窗口、拖动尺寸不出现重入或卡死。
//  4. Snapshot 对同一时刻的控件离屏重绘两次，尺寸/像素一致且不发送 Frame。
//     1x/2x、支持的各后端重复以上操作；不要仅用启动成功代替操作验收。
//
// 自动：检查双动画终值、停止后的空闲、离屏一致、Popover 更新与隐藏停止。
// 副作用：窗口、终端日志；不更改系统设置。复位：重新启动。退出：关闭窗口；
// -auto 最终输出 ANIMATION PASS、退出码 0；断言失败 panic。
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"image"
	"log"
	"runtime"
	"time"

	"github.com/golang-gui/goui/animation"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

type sample struct {
	gui.WidgetBase
	size, position float32
}

func (s *sample) Paint(p gui.Painter) {
	p.FillEllipse(geometry.Point{X: 100 + s.position, Y: 100}, s.size, s.size, graphics.Color{R: .15, G: .45, B: .8, A: 1})
}
func digest(img image.Image) [32]byte {
	h := sha256.New()
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			h.Write([]byte{byte(r >> 8), byte(g >> 8), byte(b >> 8), byte(a >> 8)})
		}
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}
func main() {
	auto := flag.Bool("auto", false, "自动验证帧调度和终值后退出")
	flag.Parse()
	runtime.LockOSThread()
	app, err := gui.NewApplication("org.golang-gui.Animation")
	if err != nil {
		log.Fatal(err)
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	w, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 700, Height: 430}, Chrome: gui.WindowChromeIntegrated})
	if err != nil {
		log.Fatal(err)
	}
	if err = w.SetTitle("GOUI animation GUI"); err != nil {
		log.Fatal(err)
	}
	content := &sample{size: 20}
	content.SetMinSize(geometry.Size{Width: 500, Height: 210})
	size := animation.New(float32(20), animation.Float32)
	position := animation.New(float32(0), animation.Float32)
	size.ConnectUpdate(func(v float32) { content.size = v; content.RequestPaint() })
	position.ConnectUpdate(func(v float32) { content.position = v; content.RequestPaint() })
	frames, popupFrames := 0, 0
	w.ConnectFrame(func(time.Time) { frames++ })
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetSpacing(8)
	button := func(text string, fn func()) *gui.Button {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		b.ConnectClicked(fn)
		toolbar.AddChild(b)
		return b
	}
	start := func() {
		target := float32(80)
		offset := float32(200)
		if size.CurrentValue() > 50 {
			target = 20
			offset = 0
		}
		if err := size.AnimateTo(w, target, 300*time.Millisecond, animation.EaseOutCubic); err != nil {
			panic(err)
		}
		if err := position.AnimateTo(w, offset, 600*time.Millisecond, animation.Linear); err != nil {
			panic(err)
		}
	}
	button("Start", start)
	button("Stop", func() { size.Stop(); position.Stop() })
	button("Finish", func() { size.Finish(); position.Finish() })
	button("Check", func() {
		fmt.Printf("CHECK frames=%d size=%.2f position=%.2f running=%v/%v\n", frames, size.CurrentValue(), position.CurrentValue(), size.Running(), position.Running())
	})
	snapshot := func() {
		before := frames
		first, err := gui.RenderWidget(content, 1)
		if err != nil {
			panic(err)
		}
		second, err := gui.RenderWidget(content, 1)
		if err != nil {
			panic(err)
		}
		if frames != before || first.Bounds() != second.Bounds() || digest(first) != digest(second) {
			panic("offscreen draw advanced frame/value")
		}
		fmt.Println("SNAPSHOT PASS")
	}
	button("Snapshot", snapshot)
	anchor := button("Popup", func() {})
	popup := gui.NewPopover(anchor, &gui.PopoverOptions{Transparent: true})
	spinner := widgets.NewProgressBar()
	spinner.SetShape(widgets.ProgressCircular)
	spinner.SetMinSize(geometry.Size{Width: 60, Height: 60})
	spinner.SetIndeterminate(true)
	popup.SetWidget(spinner)
	popup.SetPlacement(gui.PopoverPlacementBottom)
	popup.ConnectFrame(func(time.Time) { popupFrames++ })
	anchor.ConnectClicked(func() {
		if popup.Visible() {
			popup.Hide()
		} else if err := popup.Show(); err != nil {
			panic(err)
		}
	})
	switcher := widgets.NewSwitch()
	body := gui.NewLinearBox(layout.DirectionVertical)
	body.SetSpacing(16)
	body.SetPadding(20)
	body.AddChild(toolbar)
	body.AddChild(switcher)
	body.AddChild(content)
	w.SetWidget(body)
	w.ConnectDestroy(func() { size.Stop(); position.Stop(); popup.Destroy() })
	if err := w.Show(); err != nil {
		panic(err)
	}
	after := func(delay time.Duration, fn func()) {
		timer := app.NewTimer()
		timer.ConnectTimeout(fn)
		if err := timer.StartOnce(delay); err != nil {
			panic(err)
		}
	}
	if *auto {
		idle := 0
		after(200*time.Millisecond, start)
		after(time.Second, func() {
			if size.Running() || position.Running() || size.CurrentValue() != 80 || position.CurrentValue() != 200 {
				panic("animation endpoint")
			}
			snapshot()
			idle = frames
		})
		after(1200*time.Millisecond, func() {
			if frames-idle > 2 {
				panic("idle frame loop")
			}
			fmt.Printf("IDLE PASS frames=%d\n", frames)
			if err := popup.Show(); err != nil {
				panic(err)
			}
		})
		after(1600*time.Millisecond, func() {
			if popupFrames < 3 {
				panic("popover frame source not running")
			}
			popup.Hide()
			idle = popupFrames
		})
		after(1850*time.Millisecond, func() {
			if popupFrames != idle {
				panic("hidden popup still updating")
			}
			if err := popup.Show(); err != nil {
				panic(err)
			}
		})
		after(2150*time.Millisecond, func() {
			if popupFrames <= idle {
				panic("popup did not resume")
			}
			popup.Hide()
			fmt.Printf("ANIMATION PASS frames=%d popupFrames=%d\n", frames, popupFrames)
			w.Destroy()
		})
	}
	app.Run()
}
