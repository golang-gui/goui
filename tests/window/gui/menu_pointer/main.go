// 菜单统一活动项、离开清理与键盘接管的窗口回归。
//
// 环境：原生桌面、字体和绘制后端；Linux 使用独立 X11／窗口管理器与合成器。
// 启动：go run ./tests/window/gui/menu_pointer；-auto 自动断言后退出。
// GOUI_PLAT_PAINTER 选择 software/opengl（Windows 另有 direct2d），GOUI_PLAT_SCALE 验证1/2。
// Linux 没有 IME 服务时可设 XMODIFIERS=@im=none；应用倍率不代替真实 HiDPI 验收。
// 初始：约半秒后弹出 First、Second、禁用项、分隔线及 Last；点击宿主按钮可重新打开。
// 操作与逐项预期：
//  1. 在各行文字、快捷键和留白之间快速移动，最多只有一项高亮，Snapshot.Selected 相符。
//  2. 移到禁用项、分隔线或菜单内顶部留白，高亮立即清空；重新进入可用行恢复。
//  3. 移出整个弹窗或经过阴影区域，高亮立即清空，不能等重新进入后才验证。
//     保持窗口不关闭，移回 Second，其高亮恢复，物理 Hovered 与活动项分别正确。
//  4. Home/End/方向键切换活动项；指针静止不抢回选择，下一次移动或按下才接管。
//     点击或 Enter 仅执行当前可用操作；禁用项不执行、不关闭菜单。
//  5. 关闭、重开菜单不保留旧活动项，Esc 可关闭；反复操作不得崩溃。
//
// 自动模式通过 Window/Popover.DispatchEvent 逐步检查快照及实际后端的离屏像素。
// 普通模式供真实鼠标／键盘操作；READY 不改变活动项，-auto 的 READY 后短暂保留 Second。
// 像素：在行左侧内2 DIP、中高处采样，避开文本及圆角；RenderWidget(1) 使用 DIP 尺寸。
// 转为非预乘 NRGBA，与指定不透明 sRGB 背景精确比较，误差0；不比较跨平台字体金图。
// 记录 OS、实际后端、倍率和窗口管理器，自动输入不替代原生输入／IME 场景。
// 副作用：仅本进程窗口、Popover 和终端输出，不改剪贴板、文件或系统设置，不用 DevServer。
// 复位：重启或点击按钮重开菜单；退出：关闭宿主窗口，-auto 在 PASS 后自动退出。
// 自动模式提前退出视为失败；只操作本例程自己的窗口。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"runtime"
	"sort"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
func check(ok bool, message string) {
	if !ok {
		log.Fatal(message)
	}
}
func main() {
	runtime.LockOSThread()
	auto := flag.Bool("auto", false, "自动验证输入和绘制后退出")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.MenuPointerRegression")
	must(err)
	normal, highlight := color.NRGBA{R: 240, G: 240, B: 240, A: 255}, color.NRGBA{R: 36, G: 104, B: 172, A: 255}
	rules := gui.DefaultStyleRules()
	rules = append(rules, style.Name("menu-item").BackgroundColor(normal), style.Name("menu-item").State(style.Hovered).BackgroundColor(highlight), style.Name("menu-item").State(style.Pressed).BackgroundColor(highlight), style.Name("menu-item").State(style.Disabled).BackgroundColor(normal))
	app.SetStyleSheet(style.Sheet(rules...))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 640, Height: 360}})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("GOUI menu pointer regression"))
	anchor := gui.NewButton()
	anchor.SetChild(gui.NewLabel("Menu highlight regression"))
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.SetCrossAlign(layout.CrossStretch)
	box.AddChild(anchor)
	box.AddChild(gui.NewLabel("Move across text/padding; alternate pointer and keyboard navigation."))
	probe := &scaleProbe{}
	box.AddChild(probe)
	win.SetWidget(box)
	model := gui.NewMenu()
	activated := ""
	for _, name := range []string{"First", "Second", "Disabled", "Last"} {
		name := name
		if name == "Last" {
			model.AppendSeparator()
		}
		item := model.Append(name, func() { activated = name })
		if name == "Second" {
			command := gui.NewCommand()
			command.ConnectActivate(func() { activated = name })
			shortcut := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyF6})
			shortcut.SetCommand(command)
			win.Shortcuts().AddShortcut(shortcut)
			item.SetCommand(command)
		}
		if name == "Disabled" {
			item.SetEnabled(false)
		}
	}
	menu := gui.NewPopoverMenu(anchor)
	menu.SetMenu(model)
	must(win.Show())
	win.SetFocusedWidget(anchor)
	var popup gui.Popover
	key := func(k events.Key) {
		for _, kind := range []events.EventType{events.KeyDown, events.KeyUp} {
			must(win.DispatchEvent(events.KeyEvent{EventType: kind, Key: k}))
		}
	}
	rows := func() []gui.WidgetInfo {
		var found []gui.WidgetInfo
		var walk func(gui.WidgetInfo)
		walk = func(i gui.WidgetInfo) {
			if i.Role == gui.RoleMenuItem || i.Role == gui.RoleMenuSeparator {
				found = append(found, i)
			}
			for _, child := range i.Children {
				walk(child)
			}
		}
		walk(popup.Widget().Snapshot())
		sort.Slice(found, func(i, j int) bool { return found[i].Bounds.Y < found[j].Bounds.Y })
		return found
	}
	verify := func(want string) {
		rendered, err := gui.RenderWidget(popup.Widget(), 1)
		must(err)
		origin := popup.Widget().Snapshot().Bounds.Pos
		painted, selected := 0, 0
		for _, row := range rows() {
			if row.Role == gui.RoleMenuSeparator {
				check(!row.Selected, "separator selected")
				continue
			}
			r := row.Bounds
			p := geometry.Point{X: r.X + 2, Y: r.Y + r.Height/2}
			x, y := int(p.X-origin.X), int(p.Y-origin.Y)
			check(image.Pt(x, y).In(rendered.Bounds()), "pixel sample outside rendered panel")
			got := color.NRGBAModel.Convert(rendered.At(x, y)).(color.NRGBA)
			lit := got == highlight
			if lit {
				painted++
			}
			if row.Selected {
				selected++
			}
			expected := row.Text == want && want != ""
			if lit != expected || row.Selected != expected {
				log.Fatalf("row %q: pixel=%v selected=%t; want active %q", row.Text, got, row.Selected, want)
			}
			if !lit && got != normal {
				log.Fatalf("unexpected row background %q: %v", row.Text, got)
			}
		}
		count := 0
		if want != "" {
			count = 1
		}
		check(painted == count && selected == count, "painted and semantic selection count differ")
	}
	complete := false
	open := app.NewTimer()
	showMenu := func() {
		must(menu.ShowAt(geometry.Point{X: 12, Y: 36}))
		check(len(win.Popovers()) == 1, "menu not open")
		popup = win.Popovers()[0]
	}
	anchor.ConnectClicked(showMenu)
	open.ConnectTimeout(showMenu)
	must(open.StartOnce(500 * time.Millisecond))
	ready := app.NewTimer()
	ready.ConnectTimeout(func() {
		if *auto {
			check(popup != nil && len(rows()) == 5, "menu rows not realized")
			move := func(index int, text bool) {
				r := rows()[index].Bounds
				p := geometry.Point{X: r.X + 2, Y: r.Y + r.Height/2}
				if text {
					r = rows()[index].Children[0].Bounds
					p = geometry.Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
					check(gui.Pick(popup.Widget(), geometry.Point{X: p.X - popup.Widget().Snapshot().Bounds.X, Y: p.Y - popup.Widget().Snapshot().Bounds.Y}).Snapshot().Role == gui.RoleLabel, "text move did not hit Label")
				}
				must(popup.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: p}))
			}
			move(0, false)
			verify("First")
			move(1, true)
			verify("Second")
			hint := rows()[1].Children
			check(len(hint) == 2 && hint[1].Text == "F6", "shortcut text column missing")
			r := hint[1].Bounds
			must(popup.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}}))
			verify("Second")
			for i := 0; i < 20; i++ {
				move(0, true)
				verify("First")
				move(1, true)
				verify("Second")
			}
			key(events.KeyHome)
			verify("First")
			move(1, true)
			verify("Second")
			move(1, false)
			verify("Second")
			move(1, true)
			verify("Second")
			move(2, true)
			verify("")
			move(3, false)
			verify("")
			move(0, true)
			verify("First")
			must(popup.DispatchEvent(events.PointerEvent{EventType: events.PointerLeave}))
			verify("")
			for _, row := range rows() {
				check(!row.Hovered, "physical hover survived leaving the popup")
			}
			r = rows()[1].Children[0].Bounds
			must(popup.DispatchEvent(events.PointerEvent{EventType: events.PointerEnter, Position: geometry.Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}}))
			verify("Second")
			body := popup.Widget().Snapshot().Bounds
			must(popup.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: body.X + 1, Y: body.Y + 1}}))
			verify("")
			key(events.KeyEnd)
			verify("Last")
			move(1, true)
			verify("Second")
			must(popup.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: body.X - 1, Y: body.Y - 1}}))
			verify("")
			move(1, true)
			verify("Second")
		}
		log.Printf("READY painter=%s scale=%g typography=%s", probe.painter, probe.scale, app.Typography().Name())
	})
	must(ready.StartOnce(1000 * time.Millisecond))
	if *auto {
		finish := app.NewTimer()
		finish.ConnectTimeout(func() {
			key(events.KeyEnter)
			check(activated == "Second" && !menu.Visible(), "Enter activated stale item")
			must(menu.ShowAt(geometry.Point{X: 12, Y: 36}))
			check(len(win.Popovers()) == 1, "reopen failed")
			verify("")
			key(events.KeyEscape)
			check(!menu.Visible(), "Escape failed")
			log.Print("PASS rapid label/padding hover, single painted/semantic selection, immediate leave/blank/outside clear, keyboard takeover, activation and reopen")
			complete = true
			win.Destroy()
		})
		must(finish.StartOnce(2400 * time.Millisecond))
	}
	app.Run()
	if *auto && !complete {
		log.Fatal("verification aborted")
	}
}

type scaleProbe struct {
	gui.WidgetBase
	scale   float32
	painter string
}

func (p *scaleProbe) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: 1, Height: 1}))
}
func (p *scaleProbe) Paint(draw gui.Painter) {
	if p.scale != 0 {
		return
	}
	p.scale = draw.PixelScale()
	img, err := draw.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	must(err)
	defer img.Destroy()
	p.painter = fmt.Sprintf("%T", img)
}
