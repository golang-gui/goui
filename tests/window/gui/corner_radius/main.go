// 四角样式消费者的桌面验收。
//
// 环境：Linux独立X11（Software/OpenGL）、Windows（Direct2D/OpenGL）、macOS OpenGL。
// 启动：go run ./tests/window/gui/corner_radius [-auto]；用 GOUI_PLAT_PAINTER 和
// GOUI_PLAT_SCALE 选择后端／缩放。自动模式检查当前实际后端，不接受静默回退。
// 初始：圆角样式表面、三段 ButtonGroup、BorderBox 和打开弹窗按钮；左上角大圆角，
// 右上直角。点击切换后方向交换，各自背景和边框同步，BorderBox内容保持安全避让。
// 操作：切换样式、悬停／点击分组、打开Popover；弹窗左上与右上角应跟随样式切换，
// 阴影轮廓匹配，没有角部接缝或半透明色变深，透明外角露出后方窗口。
// 调整窗口大小后重复，按钮共享边不能重复加粗，Snapshot角色／动作不变。
// 分组悬停变浅、按下变深，鼠标点击不新增焦点边框；Tab／Shift+Tab显示白色焦点轮廓。
// 按住移出后释放不会点击，边框与共享分隔线保持原位，接缝两端无斜切缺口。
// 浅灰边框外侧应只呈边框与透明区域的AA过渡，不能出现蓝色毛边；
// 自动模式以真实宿主的 RenderWidget 验证切换前后的四角、透明外部与中心颜色，
// 并以独立边框遮罩检查1x／2x外缘颜色，排除蓝色背景泄漏，
// 经 Window.DispatchEvent 检查分组悬停、按下、取消、点击及Tab焦点的状态／像素，
// 在原生Popover显示时再次切换样式，并断言显示／隐藏及销毁，输出 PASS 后退出。
// 平台差异：原生窗口装饰仍归系统／GUI，不受自定义控件圆角影响；字体和AA允许差异。
// 副作用：仅本进程窗口和Popover、终端输出，不写文件／系统设置，不用DevServer。
// 复位：重启。退出：关闭窗口；提前关闭自动模式视为验收未完成。
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
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func check(value bool, message string) {
	if !value {
		panic(message)
	}
}

type surface struct {
	gui.WidgetBase
	paintPart string
}

func (s *surface) Paint(p gui.Painter) {
	resolved := gui.WidgetStyle(s, "corner-surface", "")
	rect := geometry.Rectangle{Size: s.Rect().Size}
	switch s.paintPart {
	case "background":
		gui.PaintStyledBackground(p, rect, resolved)
	case "border":
		gui.PaintStyledBorder(p, rect, resolved)
	default:
		gui.PaintStyledBox(p, rect, resolved)
	}
}
func main() {
	auto := flag.Bool("auto", false, "自动断言后退出")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.CornerStyle")
	if err != nil {
		log.Fatal(err)
	}
	phase := 0
	corners := []geometry.CornerRadius{{TopLeft: 24, TopRight: 0, BottomRight: 12, BottomLeft: 4}, {TopLeft: 0, TopRight: 24, BottomRight: 4, BottomLeft: 12}}
	setStyle := func() {
		rules := modern.Rules(modern.Options{})
		r := corners[phase]
		rules = append(rules,
			style.Name("corner-surface").CornerRadius(r).BackgroundColor(color.RGBA{60, 110, 160, 255}).BorderWidth(4).BorderColor(color.RGBA{230, 230, 230, 255}),
			style.Name("corner-button").CornerRadius(r).BackgroundColor(color.RGBA{100, 140, 190, 255}).ForegroundColor(color.White).BorderWidth(2).BorderColor(color.RGBA{60, 80, 110, 255}),
			style.Name("corner-button").State(style.Hovered).BackgroundColor(color.RGBA{110, 150, 200, 255}),
			style.Name("corner-button").State(style.Pressed).BackgroundColor(color.RGBA{90, 130, 180, 255}),
			style.Name("corner-button").Part("focus").BackgroundColor(color.Transparent).BorderWidth(0),
			style.Name("corner-button").Part("focus").State(style.FocusVisible).BorderColor(color.White).BorderWidth(2),
			style.Name("corner-frame").CornerRadius(r).BackgroundColor(color.RGBA{210, 225, 240, 255}).BorderWidth(3).BorderColor(color.RGBA{80, 100, 140, 255}),
			style.Name("corner-popup").CornerRadius(r).BackgroundColor(color.White).BorderWidth(2).BorderColor(color.RGBA{90, 110, 150, 255}).Shadow(style.Shadow{Color: color.NRGBA{A: 100}, Offset: geometry.Point{Y: 4}, BlurRadius: 12}),
		)
		app.SetStyleSheet(style.Sheet(rules...))
	}
	setStyle()
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 620, Height: 390}})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("GOUI CornerRadius styles"))
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(geometry.UniformInsets(20))
	root.SetSpacing(14)
	root.AddChild(gui.NewLabel("CornerRadius · background / border / shadow"))
	probe := new(surface)
	probe.SetMinSize(geometry.Size{Width: 320, Height: 100})
	root.AddChild(probe)
	group := gui.NewButtonGroup()
	var buttons []*gui.Button
	clicks := [3]int{}
	for i, text := range []string{"First", "Middle", "Last"} {
		button := gui.NewButton()
		button.SetStyleName("corner-button")
		button.SetChild(gui.NewLabel(text))
		group.AddChild(button)
		buttons = append(buttons, button)
		button.ConnectClicked(func() { clicks[i]++ })
	}
	root.AddChild(group)
	frame := gui.NewBorderBox()
	frame.SetStyleName("corner-frame")
	frame.SetChild(gui.NewLabel("BorderBox safe content"))
	root.AddChild(frame)
	row := gui.NewLinearBox(layout.DirectionHorizontal)
	row.SetSpacing(12)
	cycle := gui.NewButton()
	cycle.SetChild(gui.NewLabel("Cycle corners"))
	cycle.ConnectClicked(func() { phase = 1 - phase; setStyle() })
	row.AddChild(cycle)
	show := gui.NewButton()
	show.SetChild(gui.NewLabel("Popover"))
	row.AddChild(show)
	root.AddChild(row)
	popup := gui.NewPopover(show, &gui.PopoverOptions{Transparent: true})
	defer popup.Destroy()
	popup.SetStyleName("corner-popup")
	popup.SetPlacement(gui.PopoverPlacementBottom)
	content := gui.NewLinearBox(layout.DirectionVertical)
	content.SetPadding(geometry.UniformInsets(24))
	content.AddChild(gui.NewLabel("Independent corners and matching shadow"))
	popup.SetWidget(content)
	show.ConnectClicked(func() {
		if popup.Visible() {
			popup.Hide()
		} else {
			must(popup.Show())
		}
	})
	win.SetWidget(root)
	capture, err := diagnostics.Start(app, diagnostics.Options{})
	must(err)
	defer capture.Stop()
	must(win.Show())
	sample := func() {
		image, err := gui.RenderWidget(probe, 1)
		must(err)
		center := color.RGBAModel.Convert(image.At(image.Bounds().Dx()/2, image.Bounds().Dy()/2)).(color.RGBA)
		check(center == (color.RGBA{60, 110, 160, 255}), "表面中心颜色不正确")
		left := color.RGBAModel.Convert(image.At(1, 1)).(color.RGBA)
		right := color.RGBAModel.Convert(image.At(image.Bounds().Dx()-2, 1)).(color.RGBA)
		if phase == 0 {
			check(left.A == 0 && right.A == 255, "首组圆角／直角不正确")
		} else {
			check(left.A == 255 && right.A == 0, "样式切换未更新角部")
		}
		check(probe.Snapshot().Role == gui.RoleWidget, "装饰改变了控件角色")
		for _, scale := range []float32{1, 2} {
			combined, err := gui.RenderWidget(probe, scale)
			must(err)
			probe.paintPart = "border"
			border, err := gui.RenderWidget(probe, scale)
			must(err)
			probe.paintPart = "background"
			background, err := gui.RenderWidget(probe, scale)
			probe.paintPart = ""
			must(err)
			checked := 0
			for y := range combined.Bounds().Dy() {
				for x := range combined.Bounds().Dx() {
					bg := color.RGBAModel.Convert(background.At(x, y)).(color.RGBA)
					want := color.RGBAModel.Convert(border.At(x, y)).(color.RGBA)
					if bg.A < 8 || bg.A > 247 || want.A < 8 || want.A > 247 {
						continue
					}
					got := color.RGBAModel.Convert(combined.At(x, y)).(color.RGBA)
					for i, v := range []uint8{got.R, got.G, got.B, got.A} {
						w := []uint8{want.R, want.G, want.B, want.A}[i]
						check(int(v)-int(w) >= -2 && int(v)-int(w) <= 2,
							fmt.Sprintf("%gx外缘(%d,%d)混入背景：%v，参考%v", scale, x, y, got, want))
					}
					checked++
				}
			}
			check(checked > 0, "没有检查到边框AA外缘")
		}
	}
	// 真实样式与真实宿主的回归：鼠标焦点不能重画完整尾边，
	// 键盘焦点则应在按钮自身分配内形成完整装饰。
	sampleGroup := func() {
		for _, scale := range []float32{1, 2} {
			img, err := gui.RenderWidget(group, scale)
			must(err)
			pixel := func(x, y int, want color.RGBA) {
				got := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
				for i, v := range []uint8{got.R, got.G, got.B, got.A} {
					w := []uint8{want.R, want.G, want.B, want.A}[i]
					check(int(v)-int(w) >= -2 && int(v)-int(w) <= 2,
						fmt.Sprintf("分组%gx像素(%d,%d)=%v，预期%v", scale, x, y, got, want))
				}
			}
			border := color.RGBA{60, 80, 110, 255}
			height := group.Rect().Height
			for i := 1; i < len(buttons); i++ {
				seam := buttons[i].Rect().X * scale
				// 只在同一按钮内完整覆盖的像素上断言纯色；分数坐标
				// 恰好跨两个独立AA层的像素另有合成限制，不当作满覆盖。
				for _, edge := range []float32{0, height - 2} {
					for y := int(math.Ceil(float64(edge * scale))); y < int(math.Floor(float64((edge+2)*scale))); y++ {
						for x := int(math.Ceil(float64(seam - 3*scale))); x < int(math.Floor(float64(seam+3*scale))); x++ {
							if float32(x) < seam && float32(x+1) > seam {
								continue
							}
							pixel(x, y, border)
						}
					}
				}
				y := int(height * scale / 2)
				for x := int(math.Ceil(float64(seam))); x < int(math.Floor(float64(seam+2*scale))); x++ {
					pixel(x, y, border)
				}
				bg := color.RGBA{100, 140, 190, 255}
				if buttons[i-1].State().Has(style.Pressed) {
					bg = color.RGBA{90, 130, 180, 255}
				} else if buttons[i-1].State().Has(style.Hovered) {
					bg = color.RGBA{110, 150, 200, 255}
				}
				for x := int(math.Ceil(float64(seam - 2*scale))); x < int(math.Floor(float64(seam))); x++ {
					pixel(x, y, bg)
				}
			}
		}
	}
	exerciseGroup := func() {
		pointer := func(kind events.EventType, point geometry.Point, held bool) {
			var buttons events.PointerButtons
			if held {
				buttons = events.PointerButtonLeftDown
			}
			must(win.DispatchEvent(events.PointerEvent{EventType: kind, Position: point, Button: events.PointerButtonLeft, Buttons: buttons}))
		}
		for i, button := range buttons {
			point := button.Snapshot().Bounds.Center()
			pointer(events.PointerMove, point, false)
			check(button.State().Has(style.Hovered), "分组未悬停")
			sampleGroup()
			pointer(events.PointerDown, point, true)
			check(button.State().Has(style.Pressed) && button.Focused() && !button.FocusVisible(), "鼠标按下的焦点／按压状态错误")
			check(button.Snapshot().Pressed && button.Snapshot().Focused, "Snapshot缺少按压／焦点状态")
			sampleGroup()
			pointer(events.PointerUp, point, false)
			check(!button.State().Has(style.Pressed) && clicks[i] == 1, "释放未恢复或重复点击")
			sampleGroup()
		}
		point := buttons[1].Snapshot().Bounds.Center()
		pointer(events.PointerDown, point, true)
		pointer(events.PointerMove, geometry.Point{}, true)
		pointer(events.PointerUp, geometry.Point{}, false)
		check(!buttons[1].State().Has(style.Pressed) && clicks[1] == 1, "移出释放仍然点击或按压未清除")
		sampleGroup()
		for i, key := range []events.KeyEvent{
			{EventType: events.KeyDown, Key: events.KeyTab},
			{EventType: events.KeyDown, Key: events.KeyTab, Modifiers: events.ModifierShift},
		} {
			must(win.DispatchEvent(key))
			focused := win.FocusedWidget()
			check(focused == buttons[2-i] && focused.FocusVisible(), "Tab焦点目标或装饰错误")
			img, err := gui.RenderWidget(group, 1)
			must(err)
			r := focused.Rect()
			got := color.RGBAModel.Convert(img.At(int(r.X+r.Width-1), int(r.Height/2))).(color.RGBA)
			check(got == (color.RGBA{255, 255, 255, 255}), "键盘焦点没有补齐完整尾边")
		}
		point = buttons[0].Snapshot().Bounds.Center()
		pointer(events.PointerMove, point, false)
		pointer(events.PointerDown, point, true)
		pointer(events.PointerUp, point, false)
		check(!buttons[0].FocusVisible(), "鼠标点击未移除键盘提示")
		sampleGroup()
	}
	step := 0
	if *auto {
		timer := app.NewTimer()
		timer.ConnectTimeout(func() {
			switch step {
			case 0:
				s := capture.Snapshot()
				check(len(s.Roots) > 0, "缺少真实绘制记录")
				if requested := os.Getenv("GOUI_PLAT_PAINTER"); requested != "" {
					check(strings.EqualFold(s.Roots[0].Backend, requested), "未使用指定后端")
				}
				sample()
				exerciseGroup()
			case 1:
				phase = 1
				setStyle()
			case 2:
				sample()
				must(popup.Show())
			case 3:
				check(popup.Visible(), "Popover未显示")
				phase = 0
				setStyle()
			case 4:
				sample()
				check(popup.Visible(), "切换样式导致Popover消失")
				popup.Hide()
			case 5:
				check(!popup.Visible(), "Popover未隐藏")
				timer.Stop()
				fmt.Println("PASS CornerRadius styles / popup")
				win.Destroy()
			}
			step++
		})
		must(timer.Start(400 * time.Millisecond))
	}
	app.Run()
	if *auto {
		check(step == 6, "验收前窗口已关闭")
	}
}
