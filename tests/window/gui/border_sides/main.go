// 可选择边框的跨后端桌面验收。
//
// 环境：Linux独立X11（Software/OpenGL）、Windows（Direct2D/OpenGL/Software）、
// macOS（OpenGL/Software）。启动：go run ./tests/window/gui/border_sides [-auto]。
// GOUI_PLAT_PAINTER、GOUI_PLAT_SCALE选择后端／缩放；自动模式拒绝后端静默回退。
// 初始：4×4边集合按0..15排列，Top=1、Right=2、Bottom=4、Left=8；蓝色背景、
// 浅灰4DIP边框和不相同的四角。第一格无边框，最后一格四边完整。
// 操作：点击Alpha切换半透明边框，Rounded切换圆角／直角；缺边背景不能退缩，
// L形与三边转角连续、缺边端自然收窄、半透明接缝不能重复变深。
// Width切换4／52DIP，粗边框不能越出缺边另一端的圆角；内部消失时填满外轮廓。
// 点击Popover显示三边弹窗，保持打开时切换上述设置，圆角／边框应同步更新。
// 悬停、按下、移出First/Middle/Last，共享边框只画一次，顶部／底部连续。
// 调整窗口大小后重复；Tab焦点与点击仍生效，Snapshot不新增装饰动作。
// 自动：真实宿主RenderWidget在1x/2x逐一断言16种边的直边、中心及透明外部，
// 覆盖透明合成、4／52DIP边宽、动态样式、圆角切换；通过Window.DispatchEvent点击Alpha，
// 验证Popover显示／隐藏，输出后端及PASS后退出。颜色为预乘sRGB，满覆盖
// 像素只允许2级8位量化差异；AA外缘不跨平台套用同一金图。
// 平台差异：字体／AA曲线可略不同，系统窗口边框不受样式边集合影响。
// 副作用：仅本进程窗口／Popover、日志；不修改系统，不用DevServer、不写图片。
// 复位：重启。退出：关闭主窗口；提前关闭自动模式视为验收失败。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
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

func check(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

type surfaces struct{ gui.WidgetBase }

func cellRect(n int) geometry.Rectangle {
	return geometry.Rect(12+float32(n%4)*140, 8+float32(n/4)*72, 120, 56)
}

func (s *surfaces) Paint(p gui.Painter) {
	for n := 0; n < 16; n++ {
		resolved := gui.ResolveStyle(fmt.Sprintf("border-%d", n), "", s.State())
		gui.PaintStyledBox(p, cellRect(n), resolved)
	}
}

func main() {
	auto := flag.Bool("auto", false, "自动断言后退出")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.BorderSides")
	if err != nil {
		log.Fatal(err)
	}
	alpha, rounded, width := uint8(255), true, float32(4)
	setStyle := func() {
		radius := geometry.CornerRadius{TopLeft: 16, BottomRight: 8, BottomLeft: 4}
		if !rounded {
			radius = geometry.CornerRadius{}
		}
		rules := modern.Rules(modern.Options{})
		for n := 0; n < 16; n++ {
			rules = append(rules, style.Name(fmt.Sprintf("border-%d", n)).CornerRadius(radius).
				BackgroundColor(color.NRGBA{R: 60, G: 110, B: 160, A: 128}).
				BorderColor(color.NRGBA{R: 230, G: 230, B: 230, A: alpha}).BorderWidth(width).BorderSides(style.BorderSides(n)))
		}
		rules = append(rules, style.Name("border-popup").CornerRadius(radius).BackgroundColor(color.White).
			BorderColor(color.NRGBA{R: 90, G: 110, B: 150, A: alpha}).BorderWidth(4).
			BorderSides(style.BorderAll&^style.BorderTop).
			Shadow(style.Shadow{Color: color.NRGBA{A: 100}, Offset: geometry.Point{Y: 4}, BlurRadius: 12}))
		app.SetStyleSheet(style.Sheet(rules...))
	}
	setStyle()
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 620, Height: 490}, Center: true})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("GOUI BorderSides"))
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(geometry.UniformInsets(16))
	root.SetSpacing(12)
	root.AddChild(gui.NewLabel("Masks 0..15 · Top 1 / Right 2 / Bottom 4 / Left 8"))
	probe := new(surfaces)
	probe.SetMinSize(geometry.Size{Width: 564, Height: 288})
	root.AddChild(probe)
	group := gui.NewButtonGroup()
	for _, text := range []string{"First", "Middle", "Last"} {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		group.AddChild(b)
	}
	root.AddChild(group)
	row := gui.NewLinearBox(layout.DirectionHorizontal)
	row.SetSpacing(12)
	button := func(text string, fn func()) *gui.Button {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		b.ConnectClicked(fn)
		row.AddChild(b)
		return b
	}
	cycle := button("Alpha", func() {
		alpha = 255 - alpha + 128
		setStyle()
		fmt.Printf("ALPHA %d\n", alpha)
	})
	button("Rounded", func() {
		rounded = !rounded
		setStyle()
		fmt.Printf("ROUNDED %t\n", rounded)
	})
	button("Width", func() {
		width = 56 - width
		setStyle()
		fmt.Printf("WIDTH %g\n", width)
	})
	show := button("Popover", func() {})
	root.AddChild(row)
	status := gui.NewLabel("50% background alpha · one border width/color")
	root.AddChild(status)
	popup := gui.NewPopover(show, &gui.PopoverOptions{Transparent: true})
	defer popup.Destroy()
	popup.SetStyleName("border-popup")
	content := gui.NewLinearBox(layout.DirectionVertical)
	content.SetPadding(geometry.UniformInsets(24))
	content.AddChild(gui.NewLabel("Three sides · open top"))
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
	fmt.Println("READY BorderSides")
	sample := func() {
		for _, scale := range []float32{1, 2} {
			im, err := gui.RenderWidget(probe, scale)
			must(err)
			for n := 0; n < 16; n++ {
				r := cellRect(n)
				borderAt := func(x, y float32) bool {
					sides := style.BorderSides(n)
					return sides&style.BorderTop != 0 && y < width ||
						sides&style.BorderRight != 0 && x > r.Width-width ||
						sides&style.BorderBottom != 0 && y > r.Height-width ||
						sides&style.BorderLeft != 0 && x < width
				}
				for _, point := range []struct {
					x, y float32
					side style.BorderSides
				}{
					{60, 2, style.BorderTop}, {118, 28, style.BorderRight},
					{60, 54, style.BorderBottom}, {2, 28, style.BorderLeft}, {60, 28, 0},
				} {
					ref := image.NewRGBA(image.Rect(0, 0, 1, 1))
					draw.Draw(ref, ref.Bounds(), image.NewUniform(color.NRGBA{R: 60, G: 110, B: 160, A: 128}), image.Point{}, draw.Src)
					if borderAt(point.x, point.y) {
						draw.Draw(ref, ref.Bounds(), image.NewUniform(color.NRGBA{R: 230, G: 230, B: 230, A: alpha}), image.Point{}, draw.Over)
					}
					x, y := int((r.X+point.x)*scale), int((r.Y+point.y)*scale)
					got, want := color.RGBAModel.Convert(im.At(x, y)).(color.RGBA), ref.RGBAAt(0, 0)
					for i, v := range []uint8{got.R, got.G, got.B, got.A} {
						w := []uint8{want.R, want.G, want.B, want.A}[i]
						check(int(v)-int(w) >= -2 && int(v)-int(w) <= 2,
							fmt.Sprintf("%gx边集合%d/边%d像素(%d,%d)=%v，期望%v", scale, n, point.side, x, y, got, want))
					}
				}
				if !rounded {
					// 缺边相邻的直角端点也要满覆盖，防止只测边中点漏掉接缝缺口。
					for _, corner := range []struct {
						x, y  float32
						sides style.BorderSides
					}{{2, 2, style.BorderTop | style.BorderLeft}, {118, 2, style.BorderTop | style.BorderRight},
						{2, 54, style.BorderBottom | style.BorderLeft}, {118, 54, style.BorderBottom | style.BorderRight}} {
						_, _, _, got := im.At(int((r.X+corner.x)*scale), int((r.Y+corner.y)*scale)).RGBA()
						want := uint32(128 * 257)
						if borderAt(corner.x, corner.y) {
							want = uint32(alpha)*257 + want*(65535-uint32(alpha)*257)/65535
						}
						check(int(got)-int(want) >= -2*257 && int(got)-int(want) <= 2*257,
							fmt.Sprintf("集合%d/%gx直角端点%v透明度%v，期望%v", n, scale, corner, got, want))
					}
				}
			}
			_, _, _, outside := im.At(0, 0).RGBA()
			check(outside == 0, "边框绘制越界")
		}
		check(probe.Snapshot().Role == gui.RoleWidget && len(probe.Snapshot().Actions) == 0, "边框装饰改变了语义")
	}
	step := 0
	if *auto {
		timer := app.NewTimer()
		timer.ConnectTimeout(func() {
			switch step {
			case 0:
				s := capture.Snapshot()
				check(len(s.Roots) > 0, "没有真实绘制记录")
				if requested := os.Getenv("GOUI_PLAT_PAINTER"); requested != "" {
					check(strings.EqualFold(s.Roots[0].Backend, requested), "后端发生静默回退")
				}
				fmt.Printf("BACKEND %s SCALE %s\n", s.Roots[0].Backend, os.Getenv("GOUI_PLAT_SCALE"))
				sample()
				point := cycle.Snapshot().Bounds.Center()
				for _, kind := range []events.EventType{events.PointerMove, events.PointerDown, events.PointerUp} {
					buttons := events.PointerButtons(0)
					if kind == events.PointerDown {
						buttons = events.PointerButtonLeftDown
					}
					must(win.DispatchEvent(events.PointerEvent{EventType: kind, Position: point, Button: events.PointerButtonLeft, Buttons: buttons}))
				}
				check(alpha == 128, "鼠标点击未切换透明度")
			case 1:
				sample()
				must(popup.Show())
			case 2:
				check(popup.Visible(), "Popover未显示")
				width = 52
				setStyle()
			case 3:
				sample()
				check(popup.Visible(), "动态样式使Popover消失")
				rounded = false
				setStyle()
			case 4:
				sample()
				popup.Hide()
			case 5:
				check(!popup.Visible(), "Popover未隐藏")
				timer.Stop()
				fmt.Println("PASS BorderSides / 16 masks / alpha / widths 4,52 / 1x 2x / popup")
				win.Destroy()
			}
			step++
		})
		must(timer.Start(400 * time.Millisecond))
	}
	app.Run()
	if *auto {
		check(step == 6, "验收前关闭窗口")
	}
}
