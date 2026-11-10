// CheckButton圆角漏像素回归。
//
// 环境：Linux独立X11 Software/OpenGL、Windows Direct2D/OpenGL、macOS OpenGL。
// 启动：go run ./tests/window/widgets/check_button_corners [-auto]；通过
// GOUI_PLAT_PAINTER/GOUI_PLAT_SCALE指定后端/缩放；自动模式拒绝后端静默回退。
// 初始：Modern亮色，选中复选框、三态复选框和单选按钮，各自包含说明文字。
// 操作：1. 悬停/按下/移出取消复选框；松开后选择不变，圆角内无白洞，右下不鼓出。
// 2. 点击复选框，再切换Basic/Modern及亮暗；选择正确切换，复选与单选颜色协调，
// Basic保留自身配色，选中与三态的圆角轮廓平滑且不因切换出现接缝。
// 3. 调整窗口大小后重复以上操作；方框尺寸保持不变，没有角部缺口或移位。
// 自动模式在真实宿主RenderWidget下检查Basic及Modern亮暗、正常/悬停/按下/
// 禁用、选中及三态，在1x/1.25x/1.5x/2x用独立圆方程断言角内不透明、角外透明，
// 并验证Snapshot选择状态与动作；输入经过Window.DispatchEvent，不用DevServer。
// 平台差异：字体、原生窗口装饰、圆角边缘部分覆盖像素允许不同；整颗角内像素
// 使用预乘sRGB/RGBA8，alpha必须255，颜色仅允许1级量化误差，不使用字体金图。
// 副作用：仅创建本进程窗口、向终端输出结果，不写文件或系统设置。
// 复位：重启。退出：关闭窗口；-auto输出PASS后自动退出，提前关闭视为未完成。
package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/diagnostics"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func check(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

func main() {
	auto := flag.Bool("auto", false, "自动断言后退出")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.CheckCorners")
	if err != nil {
		log.Fatal(err)
	}
	dark, basic := false, false
	apply := func() {
		if basic {
			app.SetStyleSheet(nil)
		} else {
			app.SetStyleSheet(modern.Sheet(modern.Options{Dark: dark}))
		}
	}
	apply()
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 440, Height: 210}})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("GOUI CheckButton corner coverage"))
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(geometry.UniformInsets(16))
	root.AddChild(gui.NewLabel("Checked / Mixed — opaque corner coverage"))
	var buttons []*widgets.CheckButton
	for i, text := range []string{"Checked", "Mixed", "Radio"} {
		b := widgets.NewCheckButton()
		b.SetChild(gui.NewLabel(text))
		b.SetChecked(true)
		if i == 1 {
			b.SetCheckState(widgets.CheckMixed)
		} else if i == 2 {
			b.SetGroup(widgets.NewCheckButtonGroup())
		}
		buttons = append(buttons, b)
		root.AddChild(b)
	}
	row := gui.NewLinearBox(layout.DirectionHorizontal)
	for i, text := range []string{"Basic / Modern", "Light / Dark"} {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		b.ConnectClicked(func() {
			if i == 0 {
				basic = !basic
			} else {
				dark = !dark
			}
			apply()
		})
		row.AddChild(b)
	}
	root.AddChild(row)
	win.SetWidget(root)
	capture, err := diagnostics.Start(app, diagnostics.Options{})
	must(err)
	defer capture.Stop()
	must(win.Show())
	done := false
	if *auto {
		timer := app.NewTimer()
		timer.ConnectTimeout(func() {
			roots := capture.Snapshot().Roots
			check(len(roots) > 0, "缺少真实绘制记录")
			backend := roots[0].Backend
			if requested := os.Getenv("GOUI_PLAT_PAINTER"); requested != "" {
				check(strings.EqualFold(backend, requested), "指定后端未生效："+backend)
			}
			for theme := range 3 {
				basic, dark = theme == 0, theme == 2
				apply()
				for i, b := range buttons[:2] {
					for state := range 4 {
						point := b.Snapshot().Bounds.Center()
						b.SetEnabled(true)
						must(win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{}}))
						if state == 1 || state == 2 {
							must(win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point}))
						}
						if state == 2 {
							must(win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: point, Button: events.PointerButtonLeft, Buttons: events.PointerButtonLeftDown}))
						} else if state == 3 {
							b.SetEnabled(false)
						}
						part, selection := "indicator-checked", widgets.CheckChecked
						if i == 1 {
							part, selection = "indicator-mixed", widgets.CheckMixed
						}
						info := b.Snapshot()
						check(b.CheckState() == selection && info.Role == widgets.RoleCheckBox, "选择或Snapshot角色改变")
						check(info.Enabled == (state != 3), "Snapshot禁用状态错误")
						check(info.Attributes[widgets.CheckInfoKey].(widgets.CheckInfo).State == selection, "Snapshot选择状态错误")
						check(b.State().Has(style.Pressed) == (state == 2), "按压状态错误")
						hasClick := false
						for _, action := range info.Actions {
							hasClick = hasClick || action == gui.ActionClick
						}
						check(hasClick == (state != 3), "Snapshot禁用动作错误")
						resolved := gui.WidgetStyle(b, "check-button", part)
						fill, _ := resolved.BackgroundColor()
						ink := color.RGBAModel.Convert(fill).(color.RGBA)
						check(ink.A == 255, "选中背景必须不透明")
						for _, scale := range []float32{1, 1.25, 1.5, 2} {
							img, err := gui.RenderWidget(b, scale)
							must(err)
							// 左上角中心由外框(6,6,18,18)与4DIP半径确定；Label不在此区。
							// 实际控件高度由字体决定，指示器在减去6DIP上下padding后居中。
							x0 := math.Floor(float64(6*scale) + .5)
							indicatorY := (b.Rect().Height - 18) / 2
							y0 := math.Floor(float64(indicatorY*scale) + .5)
							r := float64(4 * scale)
							inside := 0
							for y := int(y0); float64(y+1) <= y0+r; y++ {
								for x := int(x0); float64(x+1) <= x0+r; x++ {
									if math.Hypot(float64(x)-x0-r-.05, float64(y)-y0-r-.05) >= r {
										continue
									}
									inside++
									got := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
									check(got.A == 255, fmt.Sprintf("%s theme=%d state=%d %gx角内(%d,%d)=%v", backend, theme, state, scale, x, y, got))
									for c, v := range []uint8{got.R, got.G, got.B} {
										want := []uint8{ink.R, ink.G, ink.B}[c]
										check(math.Abs(float64(v)-float64(want)) <= 1, "角内颜色错误")
									}
								}
							}
							check(inside > 0, "缺少角内采样")
							check(color.RGBAModel.Convert(img.At(int(x0), int(y0))).(color.RGBA).A == 0, "外角不透明")
						}
						if state == 2 {
							must(win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{}, Buttons: events.PointerButtonLeftDown}))
							must(win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Position: geometry.Point{}, Button: events.PointerButtonLeft}))
						}
					}
					b.SetEnabled(true)
				}
			}
			timer.Stop()
			done = true
			fmt.Printf("PASS check_button_corners backend=%s host_scale=%s\n", backend, os.Getenv("GOUI_PLAT_SCALE"))
			win.Destroy()
		})
		must(timer.Start(400 * time.Millisecond))
	}
	must(app.Run())
	check(!*auto || done, "验收前窗口已关闭")
}
