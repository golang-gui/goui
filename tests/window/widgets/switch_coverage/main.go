// Switch 关闭态轨道漏底回归，不依赖 DevServer。
// 环境：真实桌面事件循环；记录 OS/WM、实际后端和缩放。
// 启动：go run ./tests/window/widgets/switch_coverage [-chrome=native|integrated] [-expect-scale=1]
// 后端：GOUI_PLAT_PAINTER=software/opengl（Windows 可 direct2d）；GOUI_PLAT_SCALE=1/2。
// 初始：48×33 DIP 的 Switch，轨道具有半 DIP 局部顶边；窗口短暂显示后自动验收。
// 自动步骤与预期：
//  1. 等待有效布局和实际帧；指定 expect-scale 时3s内必须观察到目标缩放，拒绝装饰回退。
//  2. Basic/Modern亮色/暗色 × 开/关 × 正常/悬停/按下/禁用 × 1/1.25/1.5/2x，
//     共96组真实后端 RenderWidget。悬停/按下经 Window.DispatchEvent，移出抬起取消。
//  3. 以同后端完整胶囊为可信外轮廓参考，参考满覆盖的像素必须 alpha=255。
//     外缘排除内背景覆盖区域后，预乘 sRGB RGBA8 必须为边框色（无边框时为背景色）
//     乘外形覆盖率，允许2字节量化；保留圆弧AA，远离滑块的轨道核心颜色仅允许1级误差。
//  4. Snapshot 的 checked、enabled、角色及 click 动作保持；取消按压不发送 Change。
//     全部通过输出 COVERAGE PASS 并销毁窗口；提前关闭或断言失败不计通过。
//
// 平台差异：不同后端AA不比较统一金图；逻辑缩放不等同真实HiDPI，编译不等同运行。
// 副作用：仅测试窗口、后端离屏图像和日志；不改系统设置、文件或剪贴板。
// 复位：重启。退出：自动完成；手动关闭会中断验收。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/utils"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

type capsuleReference struct {
	gui.WidgetBase
	width float32
}

func (r *capsuleReference) Paint(p gui.Painter) {
	rect := geometry.Rect(6, 6.5, 36, 20)
	if r.width == 0 {
		p.FillRoundRect(rect, 10, graphics.RGB(255, 255, 255))
	} else {
		p.FillPath(utils.RoundRectInsetPath(p.SnapRect(rect), geometry.UniformCornerRadius(10), geometry.UniformInsets(r.width)), graphics.RGB(255, 255, 255))
	}
}

func main() {
	chromeFlag := flag.String("chrome", "integrated", "native/integrated")
	expectScale := flag.Float64("expect-scale", 0, "核对实际帧缩放；0 不指定")
	flag.Parse()
	chrome := gui.WindowChromeIntegrated
	if *chromeFlag == "native" {
		chrome = gui.WindowChromeNative
	} else if *chromeFlag != "integrated" {
		panic("未知 chrome")
	}
	app, err := gui.NewApplication("org.golang-gui.SwitchCoverage")
	must(err)
	gui.App = app
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 560, Height: 200}, Chrome: chrome})
	must(err)
	must(win.SetTitle("GOUI Switch coverage"))
	win.Chrome().ConnectInfo(func(info gui.ChromeInfo) {
		if info.Mode != chrome {
			panic("装饰模式回退")
		}
	})
	b := widgets.NewSwitch()
	b.SetAnimated(false)
	b.SetMinSize(geometry.Size{Width: 48, Height: 33})
	b.SetMaxSize(geometry.Size{Width: 48, Height: 33})
	changes := 0
	b.ConnectChange(func(bool) { changes++ })
	root := gui.NewLinearBox(layout.DirectionVertical)
	header := gui.NewHeaderBar()
	header.SetChild(gui.NewLabel("Switch opaque track coverage"))
	root.AddChild(header)
	root.AddChild(gui.NewLabel("Automatic Basic / Modern state and alpha checks"))
	root.AddChild(b)
	reference := new(capsuleReference)
	root.AddChild(reference)
	win.SetWidget(root)
	var frameScale float32
	win.ConnectAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
		if frameScale == p.PixelScale() {
			return
		}
		frameScale = p.PixelScale()
		img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		must(err)
		fmt.Printf("BACKEND %T scale=%g chrome=%s\n", img, frameScale, *chromeFlag)
		img.Destroy()
	})
	done := false
	observer := app.NewTimer()
	win.ConnectDestroy(func() {
		observer.Stop()
		if !done {
			panic("验收尚未完成")
		}
		app.Post(func() {
			if !b.Destroyed() {
				panic("窗口销毁后仍有存活开关")
			}
			app.Quit()
		})
	})
	deadline := time.Now().Add(3 * time.Second)
	observer.ConnectTimeout(func() {
		if b.Rect().Size != (geometry.Size{Width: 48, Height: 33}) || frameScale == 0 || *expectScale > 0 && math.Abs(float64(frameScale)-*expectScale) > .001 {
			if time.Now().After(deadline) {
				panic("未观察到预期布局/缩放")
			}
			return
		}
		observer.Stop()
		cases, interior, fringe := 0, 0, 0
		for theme := range 3 {
			var sheet style.StyleSheet
			if theme > 0 {
				sheet = modern.Sheet(modern.Options{Dark: theme == 2})
			}
			app.SetStyleSheet(sheet)
			for _, checked := range []bool{false, true} {
				for state := range 4 {
					b.SetEnabled(true)
					b.SetChecked(checked)
					must(win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{}}))
					point := b.Snapshot().Bounds.Center()
					if state == 1 || state == 2 {
						must(win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point}))
					}
					if state == 2 {
						must(win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: point, Button: events.PointerButtonLeft, Buttons: events.PointerButtonLeftDown}))
					} else if state == 3 {
						b.SetEnabled(false)
					}
					info := b.Snapshot()
					if info.Role != widgets.RoleSwitch || info.Enabled != (state != 3) || info.Attributes[widgets.SwitchInfoKey].(widgets.SwitchInfo).Checked != checked || (len(info.Actions) > 0) != (state != 3) {
						panic("Snapshot状态/动作改变")
					}
					if state != 3 && (len(info.Actions) != 1 || info.Actions[0] != gui.ActionClick) {
						panic("Switch动作不是click")
					}
					part := "track"
					if checked {
						part += "-checked"
					}
					resolved := gui.WidgetStyle(b, "switch", part)
					width, _ := resolved.BorderWidth()
					background, _ := resolved.BackgroundColor()
					ink := background
					if width > 0 {
						ink, _ = resolved.BorderColor()
					}
					for _, scale := range []float32{1, 1.25, 1.5, 2} {
						actual, err := gui.RenderWidget(b, scale)
						must(err)
						bodyX := float64(32 * scale)
						if checked {
							bodyX = float64(16 * scale)
						}
						bodyY := math.Floor(float64(6.5*scale)+.5) + float64(10*scale)
						got := color.RGBAModel.Convert(actual.At(int(bodyX), int(bodyY))).(color.RGBA)
						want := color.RGBAModel.Convert(background).(color.RGBA)
						for i, v := range []uint8{got.R, got.G, got.B, got.A} {
							expected := []uint8{want.R, want.G, want.B, want.A}[i]
							if delta := int(v) - int(expected); delta < -1 || delta > 1 {
								panic(fmt.Sprintf("轨道核心颜色改变：%v，预期%v", got, want))
							}
						}
						reference.Arrange(geometry.Rect(0, 0, 48, 33))
						reference.width = 0
						outer, err := gui.RenderWidget(reference, scale)
						must(err)
						reference.width = width
						inner, err := gui.RenderWidget(reference, scale)
						must(err)
						a, z := checkCoverage(actual, outer, inner, color.RGBAModel.Convert(ink).(color.RGBA), width)
						interior, fringe, cases = interior+a, fringe+z, cases+1
					}
					if state == 2 {
						must(win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{}, Buttons: events.PointerButtonLeftDown}))
						must(win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Position: geometry.Point{}, Button: events.PointerButtonLeft}))
					}
				}
			}
		}
		if changes != 0 || interior == 0 || fringe == 0 {
			panic("取消按压触发Change或缺少采样")
		}
		done = true
		fmt.Printf("COVERAGE PASS cases=%d interior=%d fringe=%d\n", cases, interior, fringe)
		win.Destroy()
	})
	must(win.Show())
	must(observer.Start(80 * time.Millisecond))
	must(app.Run())
}

func checkCoverage(actual, outer, inner image.Image, ink color.RGBA, width float32) (int, int) {
	inside, fringe := 0, 0
	// 先报告主回归的内部漏底，再检查不同原生图元可能采用不同曲线的外缘。
	for y := 0; y < actual.Bounds().Dy(); y++ {
		for x := 0; x < actual.Bounds().Dx(); x++ {
			c := color.RGBAModel.Convert(actual.At(x, y)).(color.RGBA)
			a := color.RGBAModel.Convert(outer.At(x, y)).(color.RGBA).A
			if a == 255 {
				inside++
				if c.A != 255 {
					panic(fmt.Sprintf("轨道形内漏底 (%d,%d)=%v，参考alpha=255", x, y, c))
				}
			}
		}
	}
	for y := 0; y < actual.Bounds().Dy(); y++ {
		for x := 0; x < actual.Bounds().Dx(); x++ {
			c := color.RGBAModel.Convert(actual.At(x, y)).(color.RGBA)
			a := color.RGBAModel.Convert(outer.At(x, y)).(color.RGBA).A
			innerAlpha := color.RGBAModel.Convert(inner.At(x, y)).(color.RGBA).A
			if a > 8 && a < 247 && (width == 0 || innerAlpha == 0) {
				fringe++
				for i, got := range []uint8{c.R, c.G, c.B, c.A} {
					want := (int([]uint8{ink.R, ink.G, ink.B, ink.A}[i])*int(a) + 127) / 255
					if delta := int(got) - want; delta < -2 || delta > 2 {
						panic(fmt.Sprintf("外缘(%d,%d)=%v，预期单次外轮廓覆盖率alpha=%d", x, y, c, a))
					}
				}
			}
		}
	}
	return inside, fringe
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
