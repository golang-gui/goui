// Tooltip 窗口验证，需要真实桌面、窗口管理器、字体及原生排版。
// 启动：go run ./tests/window/gui/tooltip [-inspect|-manual]；Linux 无 IME 服务可设 XMODIFIERS=@im=none。
// 通过 GOUI_PLAT_PAINTER 选择后端，GOUI_PLAT_SCALE=1/1.5/2 验证缩放；日志给出实际排版后端。
// 初始窗口包含 Basic / Modern Light / Modern Dark 三组按钮、禁用按钮和文本输入框。
// 自动通过 Window.DispatchEvent 验证延迟显示、语义、非模态及焦点、正常点击和滚轮、
// 禁用控件、Escape、更新文字、隐藏、Tab 导航和模态 Popover 切换。
// -inspect 在自动断言通过后保留窗口：悬停三组按钮观察配色、圆角、文字换行；
// 点击按钮计数增加，输入框仍可输入，Tab 可导航；移动窗口到屏幕边缘检查提示翻转。
// 提示不应追随同一按钮内的小幅移动；移出、点击或按键后关闭，不应吞掉原控件操作。
// 每次显示均等待 700 ms。
// 不要求点击提示自身穿透到下方控件，不模拟重放输入。真实桌面观察补充合成与系统事件路径。
// -manual 从启动起只接受真实桌面输入，不运行模拟输入断言，也不输出 PASS；按上述步骤验收。
// Windows 原生 Popup 显示可能生成当前位置的鼠标事件，覆盖模拟位置；不要把真实光标和模拟
// 光标不一致导致的自动失败算作通过，必要时使用 -manual 独立验证真实操作并分别记录结果。
// 不改系统设置、文件或剪贴板；重启复位。默认成功自动关闭，-inspect 关闭窗口退出；
// 断言失败或自动完成前关闭以非零退出。不依赖 DevServer。
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
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
	inspect := flag.Bool("inspect", false, "keep the window open after checks")
	manual := flag.Bool("manual", false, "use real desktop input without synthetic steps")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.TooltipVerification")
	must(err)
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 900, Height: 450}, Chrome: gui.WindowChromeIntegrated})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("GOUI Tooltip Verification"))
	row := gui.NewLinearBox(layout.DirectionHorizontal)
	row.SetPadding(20)
	row.SetSpacing(20)
	row.SetCrossAlign(layout.CrossStretch)
	var buttons, disabled [3]*gui.Button
	var fields [3]*gui.TextInput
	var originalTips [3]string
	clicks, wheels := 0, 0
	status := gui.NewLabel("Hover a button; Tab shows keyboard help.")
	for i, name := range []string{"Basic", "Modern Light", "Modern Dark"} {
		box := gui.NewLinearBox(layout.DirectionVertical)
		box.SetSpacing(12)
		box.SetPadding(12)
		box.SetCrossAlign(layout.CrossStretch)
		box.AddChild(gui.NewLabel(name))
		b := gui.NewButton()
		b.SetName(name + " action")
		b.SetChild(gui.NewLabel("Open document"))
		b.SetTooltip(name + ": Open a UTF-8 document.\nThis is a multiline tooltip with normal word wrapping.")
		originalTips[i] = b.Tooltip()
		b.ConnectClicked(func() { clicks++; status.SetText(fmt.Sprintf("Clicks: %d", clicks)) })
		box.AddChild(b)
		buttons[i] = b
		d := gui.NewButton()
		d.SetChild(gui.NewLabel("Unavailable action"))
		d.SetName(name + " disabled")
		d.SetTooltip("Select a document before using this action.")
		d.SetEnabled(false)
		box.AddChild(d)
		disabled[i] = d
		field := gui.NewTextInput()
		field.SetName(name + " input")
		field.SetPlaceholder("Type here")
		field.SetTooltip("Enter a document name. This input keeps its own focus and text.")
		box.AddChild(field)
		fields[i] = field
		scope := gui.NewStyleScope()
		sheet := basic.Sheet()
		if i > 0 {
			sheet = modern.Sheet(modern.Options{Dark: i == 2})
		}
		scope.SetStyleSheet(sheet)
		background := gui.NewBorderBox()
		background.SetStyleName("window")
		background.SetChild(box)
		scope.SetChild(background)
		scope.SetMainWeight(1)
		row.AddChild(scope)
	}
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.SetCrossAlign(layout.CrossStretch)
	box.AddChild(row)
	box.AddChild(status)
	probe := new(painterProbe)
	box.AddChild(probe)
	wheel := gui.NewWheelEventController()
	wheel.ConnectScroll(func(gui.EventContext, events.WheelEvent) { wheels++ })
	box.AddEventController(wheel)
	win.SetWidget(box)
	must(win.Show())
	move := func(w gui.Widget) {
		must(win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: w.Snapshot().Bounds.Center()}))
	}
	key := func(k events.Key) {
		for _, kind := range []events.EventType{events.KeyDown, events.KeyUp} {
			must(win.DispatchEvent(events.KeyEvent{EventType: kind, Key: k}))
		}
	}
	visibleTip := func() bool {
		for _, p := range win.Popovers() {
			if hasRole(p.Widget().Snapshot(), gui.RoleTooltip) {
				check(!p.Modal() && !p.Widget().Focusable(), "tooltip became interactive/modal")
				return true
			}
		}
		return false
	}
	if *manual {
		ready := app.NewTimer()
		ready.ConnectTimeout(func() {
			log.Printf("READY manual Tooltip; OS=%s typography=%s painter=%s scale=%g", runtime.GOOS, app.Typography().Name(), probe.painter, probe.scale)
		})
		must(ready.StartOnce(200 * time.Millisecond))
		app.Run()
		ready.Stop()
		return
	}
	type step struct {
		delay time.Duration
		run   func()
	}
	steps := []step{
		{300 * time.Millisecond, func() { check(win.Focused(), "window not activated"); move(buttons[0]) }},
		{100 * time.Millisecond, func() { check(!visibleTip(), "tooltip appeared before delay") }},
		{700 * time.Millisecond, func() {
			check(visibleTip() && win.FocusedWidget() == nil, "tooltip missing or stole focus")
			p := buttons[0].Snapshot().Bounds.Center()
			for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
				must(win.DispatchEvent(events.PointerEvent{EventType: kind, Position: p, Button: events.PointerButtonLeft}))
			}
			check(clicks == 1 && !visibleTip(), "tooltip swallowed the normal click")
			move(disabled[1])
		}},
		{800 * time.Millisecond, func() {
			check(visibleTip() && !disabled[1].Enabled(), "disabled widget help missing")
			must(win.DispatchEvent(events.WheelEvent{Position: disabled[1].Snapshot().Bounds.Center(), DeltaY: 40}))
			check(wheels == 1 && !visibleTip(), "tooltip swallowed the wheel")
			move(buttons[2])
		}},
		{800 * time.Millisecond, func() {
			check(visibleTip(), "dark scoped tooltip missing")
			key(events.KeyEscape)
			check(!visibleTip(), "Escape did not close tooltip")
			move(buttons[2])
		}},
		{800 * time.Millisecond, func() {
			check(!visibleTip(), "Escape reopened tooltip without leaving")
			buttons[2].SetTooltip("Updated tooltip text")
		}},
		{800 * time.Millisecond, func() {
			check(visibleTip(), "updated tooltip did not appear")
			buttons[2].SetVisible(false)
			check(!visibleTip(), "hidden widget kept tooltip")
			buttons[2].SetVisible(true)
			win.SetFocusedWidget(nil)
			key(events.KeyTab)
		}},
		{800 * time.Millisecond, func() {
			check(win.FocusedWidget() == buttons[0] && buttons[0].FocusVisible() && visibleTip(), "Tab help missing")
			key(events.KeyEscape)
			win.SetFocusedWidget(fields[0])
			must(win.CommitText("text"))
			check(fields[0].Text() == "text", "tooltip changed text input")
			move(buttons[1])
		}},
		{800 * time.Millisecond, func() {
			check(visibleTip(), "modal transition fixture missing")
			p := gui.NewPopover(buttons[1], nil)
			p.SetWidget(gui.NewLabel("Modal content"))
			p.SetModal(true)
			must(p.Show())
			check(!visibleTip(), "old tooltip covered modal content")
			p.Destroy()
		}},
	}
	index, complete := 0, false
	started := time.Now()
	timer := app.NewTimer()
	defer timer.Stop()
	timer.ConnectTimeout(func() {
		if index < len(steps) {
			if time.Since(started) < steps[index].delay {
				return
			}
			steps[index].run()
			index++
			started = time.Now()
			return
		}
		timer.Stop()
		complete = true
		for i, b := range buttons {
			b.SetTooltip(originalTips[i])
		}
		check(probe.scale > 0 && probe.painter != "", "painter probe did not run")
		log.Printf("PASS Tooltip: delay, click/wheel, disabled, Escape, text, hide, keyboard, modal; OS=%s typography=%s painter=%s scale=%g", runtime.GOOS, app.Typography().Name(), probe.painter, probe.scale)
		if !*inspect {
			win.Destroy()
		}
	})
	must(timer.Start(50 * time.Millisecond))
	app.Run()
	check(complete, "tooltip verification aborted")
}

type painterProbe struct {
	gui.WidgetBase
	painter string
	scale   float32
}

func (p *painterProbe) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: 1, Height: 1}))
}

func (p *painterProbe) Paint(draw gui.Painter) {
	if p.scale != 0 {
		return
	}
	p.scale = draw.PixelScale()
	img, err := draw.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	must(err)
	p.painter = fmt.Sprintf("%T", img)
	img.Destroy()
}

func hasRole(info gui.WidgetInfo, role gui.Role) bool {
	if info.Role == role {
		return true
	}
	for _, child := range info.Children {
		if hasRole(child, role) {
			return true
		}
	}
	return false
}
