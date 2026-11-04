// 可选绘制／资源诊断验证；诊断自身不依赖 DevServer。
//
// 环境：Linux/X11、Windows 或 macOS 桌面，主线程启动。
// 启动：GOUI_PLAT_PAINTER=opengl GOUI_PLAT_SCALE=1 go run ./tests/window/gui/diagnostics
// 后端可选 software / opengl，Windows 另可选 direct2d；倍率另验 2。
// 默认自动断言并退出；自动模式应独占测试桌面，不并行操作其它窗口。
// -auto=false 保留窗口用于手工操作或协议验证。
// 初始：计数按钮、Popover 按钮、文字和带半透明色块的缓存 DrawingArea。
// 自动检查：开启前后 RenderWidget 的预乘 RGBA 精确相等且非空；离屏不增加 FPS；
// 主窗口及 Popover 均有完成帧；修改文字／DrawingArea 可观察到资源重建；
// 空闲时反复查询不增加帧，停止后结果冻结且后续正常绘制仍能完成。
// 手工：点击 Increment／Popup，检查交互正常；可改变窗口尺寸和系统缩放。
// 协议：-auto=false 并设 GOUI_DEV_ADDR=127.0.0.1:18765；对
// /goui/dev/diagnostics GET 查询，POST {"enabled":true,"widgets":true} 开启，
// 点击按钮后 GET 应有相应计数；不操作 1 秒后 idle=true，帧数保持；
// POST {"enabled":false} 冻结。GET 本身不得开启采集或增加绘制帧。
// FPS 是 CPU 完成帧间隔估计，不是 GPU／显示器刷新率；Direct2D 的文字位图计数可为零。
// 像素比较只比较同后端、同次运行、同倍率的离屏结果，不作跨平台字体金图比较。
// 副作用：创建一个窗口和短暂 Popover；协议模式开放指定的本机端口；不写文件。
// 重启复位；自动模式输出 PASS 后退出，手工模式关闭主窗口退出。
// -hud 切换到 HUD 专项：透传点击、连续帧、位置、闲置、离屏隔离和停止清理。
// HUD 逐项操作及预期见 hud.go；可组合 -hud -auto=false 手工验证。
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	_ "github.com/golang-gui/goui/gui/dev"
	"github.com/golang-gui/goui/gui/diagnostics"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/theme/modern"
)

type probe struct {
	gui.WidgetBase
	image  graphics.Image
	paints int
}

func (w *probe) Paint(p gui.Painter) {
	w.paints++
	if w.image == nil {
		var err error
		w.image, err = p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		must(err)
		actual := strings.ToLower(fmt.Sprintf("%T", w.image))
		if requested := os.Getenv("GOUI_PLAT_PAINTER"); requested != "" {
			require(strings.Contains(actual, requested+"."), "unexpected backend "+actual)
		}
		fmt.Printf("BACKEND image=%s scale=%g\n", actual, p.PixelScale())
	}
	p.FillRect(geometry.Rect(0, 0, w.Rect().Width, w.Rect().Height), graphics.RGBA(50, 120, 200, 180))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

func button(text string, fn func()) *gui.Button {
	b := gui.NewButton()
	b.SetChild(gui.NewLabel(text))
	b.ConnectClicked(fn)
	return b
}

func total(s diagnostics.Snapshot) (n uint64) {
	for _, r := range s.Roots {
		n += r.Frames
	}
	return
}

func main() {
	auto := flag.Bool("auto", true, "自动断言后退出")
	hud := flag.Bool("hud", false, "验证显式性能浮层")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.Diagnostics")
	if err != nil {
		log.Fatal(err)
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	if *hud {
		runHUD(app, *auto)
		return
	}
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 540, Height: 360}})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("GOUI diagnostics"))
	label := gui.NewLabel("Count 0")
	label.SetID("counter")
	count := 0
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(geometry.UniformInsets(20))
	root.SetSpacing(12)
	root.AddChild(label)
	root.AddChild(button("Increment", func() { count++; label.SetText(fmt.Sprintf("Count %d", count)) }))
	var popup gui.Popover
	var open *gui.Button
	showPopup := func() {
		if popup == nil {
			popup = gui.NewPopover(open, nil)
			popup.SetWidget(gui.NewLabel("Separate popover root"))
			popup.SetPlacement(gui.PopoverPlacementBottom)
		} else if popup.Visible() {
			popup.Hide()
			return
		}
		must(popup.Show())
	}
	open = button("Popup", showPopup)
	root.AddChild(open)
	sample := &probe{}
	sample.SetMinSize(geometry.Size{Width: 120, Height: 30})
	sample.ConnectUnmount(func() {
		if sample.image != nil {
			sample.image.Destroy()
			sample.image = nil
		}
	})
	root.AddChild(sample)
	draw := func(p gui.Painter, s geometry.Size) {
		p.FillRoundRect(geometry.Rect(0, 0, s.Width, s.Height), 8, graphics.RGBA(100, 170, 80, 160))
	}
	area := gui.NewDrawingArea(draw)
	area.SetMinSize(geometry.Size{Width: 120, Height: 40})
	root.AddChild(area)
	win.SetWidget(root)
	must(win.Show())
	if !*auto {
		app.Run()
		return
	}
	var capture *diagnostics.Capture
	var frozen diagnostics.Snapshot
	var before []byte
	var idleFrames, priorPaints uint64
	step := 0
	timer := app.NewTimer()
	timer.ConnectTimeout(func() {
		switch step {
		case 0:
			require(sample.paints > 0, "initial paint missing")
			img, err := gui.RenderWidget(root, 1)
			must(err)
			before = append([]byte(nil), graphics.CopyToBitmap(img, graphics.PixelFormatRGBA, nil).Pixels...)
		case 1:
			capture, err = diagnostics.Start(app, diagnostics.Options{Widgets: true})
			must(err)
			img, err := gui.RenderWidget(root, 1)
			must(err)
			require(bytes.Equal(before, graphics.CopyToBitmap(img, graphics.PixelFormatRGBA, nil).Pixels), "diagnostics changed pixels")
			nonempty := false
			for i := 3; i < len(before); i += 4 {
				nonempty = nonempty || before[i] != 0
			}
			require(nonempty, "empty render")
			s := capture.Snapshot()
			require(total(s) == 0 && len(s.Widgets) > 0 && len(s.Roots) == 1 && s.Roots[0].Offscreen == 1, "offscreen counted as presentation")
			must(win.RequestPaint())
		case 2:
			require(total(capture.Snapshot()) > 0, "requested frame missing")
			showPopup()
		case 3:
			s := capture.Snapshot()
			require(len(s.Roots) == 2 && s.Roots[0].Completed > 0 && s.Roots[1].Completed > 0, "popover frame missing")
			label.SetText("Changed text rebuilds native layout")
			area.SetDrawFunc(draw)
			popup.Hide()
		case 4:
			s := capture.Snapshot()
			require(s.Resources.LayoutsCreated > 0 && s.Resources.DrawingGenerations > 0 && s.Resources.ImagesUpdated > 0, "resource work unobserved")
			idleFrames = total(s)
		case 5, 6, 7, 8:
			for range 100 {
				s := capture.Snapshot()
				require(total(s) == idleFrames, fmt.Sprintf("unexpected frame during idle check: before=%d roots=%+v; check external input/exposure", idleFrames, s.Roots))
			}
		case 9:
			s := capture.Snapshot()
			for _, r := range s.Roots {
				require(r.Idle && r.FPS == 0, "idle FPS is nonzero")
			}
			capture.Stop()
			frozen = capture.Snapshot()
			priorPaints = uint64(sample.paints)
			must(win.RequestPaint())
		case 10:
			require(reflect.DeepEqual(frozen, capture.Snapshot()), "stopped result changed")
			require(uint64(sample.paints) > priorPaints, "stopping disabled normal painting")
			frozen.Widgets, frozen.Frames = nil, nil
			data, err := json.Marshal(frozen)
			must(err)
			fmt.Printf("PASS %s\n", data)
			timer.Stop()
			win.Destroy()
		}
		step++
	})
	must(timer.Start(300 * time.Millisecond))
	app.Run()
	require(step == 11, "window closed before verification completed")
}
