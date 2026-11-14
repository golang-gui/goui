// 控制器类型化输入、连续按下与取消验证，不依赖 DevServer。
// 环境：Linux X11、Windows、macOS 交互桌面；GOUI_PLAT_PAINTER 选择后端，
// GOUI_PLAT_SCALE=1/2 检查坐标单位。启动：go run ./tests/window/gui/input_callbacks。
// 初始：两个独立 Probe、一个多行 TextView；所有计数为零。
// 操作与逐项预期：
//  1. 点击 Probe A，日志依次 STATE true、PRESS、STATE false、RELEASE、CLICK。
//     F8 复位后，按住 Shift 快速双击，再第三次按下并保持；必须立即出现
//     PRESS count=3 completed=2，不能等释放才报告。向右拖动超过 4 DIP 后出现
//     CANCEL 与 BEGIN unit=3，继续移动出现 UPDATE unit=3，释放出现 END；没有 CLICK 3。
//  2. F8 后在 B（仅 Click）按住、移出其边界再释放：STATE false 可以先发生，但真实释放仍
//     只有一次 RELEASE，不生成 CLICK 或伪造 CANCEL。A、B 的连续次数各自独立。
//     F8 后按住 B 再按 F8：报告 CANCEL，没有 RELEASE；随后松开不能激活旧点击。
//  3. 在 A 按住 Shift 滚轮：WHEEL 携带局部 DIP 坐标、修饰键和原有 delta/mode。
//     在 A 右击请求菜单；macOS Ctrl+左键也可。日志 REQUEST has=true；
//     聚焦 A 后 Shift+F10 则 REQUEST has=false，修饰键均正确。
//  4. 多行文本依次单击、双击、三击；三击按下立即选逻辑行和存在的 LF，按住再向
//     上／下拖动按整行扩选。F6 在空闲时核对 Probe 的 Pressed 快照和拖动已结束。
//     点击 Check 按钮同样核对且普通 Widget 的无参数 Clicked 仍正常工作。
//
// 断言检查局部坐标、原始输入未被改写、起始拖动信息、次数和状态快照；日志供
// 驱动进一步检查跨回调顺序，不能仅以 READY 作为通过。原生抗锯齿不作金图比较。
// 副作用：仅窗口与日志，不修改系统设置或剪贴板；TextView 只编辑内存内容。
// 复位：F8。退出：关闭主窗口；释放本程序创建的全部资源。
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
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

type probe struct {
	box                                  *gui.BorderBox
	click                                *gui.ClickEventController
	drag                                 *gui.DragEventController
	last                                 gui.ClickInfo
	name                                 string
	completed, canceled, ended, released int
}

func (p *probe) assertPosition(ctx gui.EventContext, position geometry.Point) {
	e, ok := ctx.Event().(events.PointerEvent)
	if !ok {
		return
	}
	origin := p.box.Snapshot().Bounds.Pos
	check(math.Abs(float64(position.X-(e.Position.X-origin.X))) < .001 && math.Abs(float64(position.Y-(e.Position.Y-origin.Y))) < .001, "局部坐标或原始事件坐标被错误转换")
}
func (p *probe) reset() {
	p.click.Reset()
	p.drag.Reset()
	p.completed, p.canceled, p.ended, p.released = 0, 0, 0, 0
	p.last = gui.ClickInfo{}
}

func newProbe(name string) *probe {
	p := &probe{name: name, box: gui.NewBorderBox(), click: gui.NewClickEventController(), drag: gui.NewDragEventController()}
	p.box.SetName("Probe " + name)
	p.box.SetStyleName("button")
	p.box.SetFocusable(true)
	p.box.SetMinSize(geometry.Size{Width: 360, Height: 140})
	p.box.SetMaxSize(geometry.Size{Width: 360, Height: 140})
	label := "Probe A — Shift triple-drag / menu / wheel"
	if name == "B" {
		label = "Probe B — click only / release outside / reset"
	}
	p.box.SetChild(gui.NewLabel(label))
	p.drag.SetThreshold(4)
	p.click.ConnectPressedChanged(func(pressed bool) { p.box.SetState(style.Pressed, pressed); log.Printf("%s STATE %v", name, pressed) })
	p.click.ConnectPressed(func(ctx gui.EventContext, info gui.ClickInfo) {
		p.assertPosition(ctx, info.Position)
		p.last = info
		log.Printf("%s PRESS count=%d completed=%d pos=%g,%g mods=%d buttons=%d", name, info.Count, p.completed, info.Position.X, info.Position.Y, info.Modifiers, info.Buttons)
	})
	p.click.ConnectReleased(func(ctx gui.EventContext, info gui.ClickInfo) {
		p.assertPosition(ctx, info.Position)
		p.released++
		log.Printf("%s RELEASE count=%d mods=%d", name, info.Count, info.Modifiers)
	})
	p.click.ConnectClicked(func(_ gui.EventContext, info gui.ClickInfo) {
		p.completed++
		log.Printf("%s CLICK count=%d completed=%d", name, info.Count, p.completed)
	})
	p.click.ConnectCanceled(func(reason gui.GestureCancelReason) { p.canceled++; log.Printf("%s CANCEL reason=%d", name, reason) })
	p.drag.ConnectBegin(func(_ gui.EventContext, info gui.DragInfo) {
		check(info.Position == p.last.Position && info.Modifiers == p.last.Modifiers && info.Buttons == p.last.Buttons, "拖动起始信息不是原按下信息")
		log.Printf("%s BEGIN unit=%d mods=%d pos=%g,%g", name, p.last.Count, info.Modifiers, info.Position.X, info.Position.Y)
	})
	p.drag.ConnectUpdate(func(ctx gui.EventContext, info gui.DragInfo) {
		p.assertPosition(ctx, info.Position)
		log.Printf("%s UPDATE unit=%d mods=%d", name, p.last.Count, info.Modifiers)
	})
	p.drag.ConnectEnd(func(ctx gui.EventContext, info gui.DragInfo) {
		p.assertPosition(ctx, info.Position)
		p.ended++
		log.Printf("%s END unit=%d mods=%d", name, p.last.Count, info.Modifiers)
	})
	p.drag.ConnectCanceled(func(reason gui.GestureCancelReason) { log.Printf("%s DRAG-CANCEL reason=%d", name, reason) })
	p.box.AddEventController(p.click)
	if name == "A" {
		p.box.AddEventController(p.drag)
	}
	wheel := gui.NewWheelEventController()
	wheel.ConnectScroll(func(ctx gui.EventContext, info gui.WheelInfo) {
		log.Printf("%s WHEEL pos=%g,%g mods=%d delta=%g,%g mode=%d", name, info.Position.X, info.Position.Y, info.Modifiers, info.DeltaX, info.DeltaY, info.Mode)
		ctx.StopPropagation()
	})
	p.box.AddEventController(wheel)
	menu := gui.NewContextMenuEventController()
	menu.ConnectRequest(func(ctx gui.EventContext, info gui.ContextMenuInfo) {
		if info.HasPosition {
			p.assertPosition(ctx, info.Position)
		}
		log.Printf("%s REQUEST has=%v pos=%g,%g mods=%d", name, info.HasPosition, info.Position.X, info.Position.Y, info.Modifiers)
		ctx.StopPropagation()
	})
	p.box.AddEventController(menu)
	return p
}

func main() {
	app, err := gui.NewApplication("org.golang-gui.InputCallbacksVerification")
	must(err)
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 840, Height: 600}, Chrome: gui.WindowChromeIntegrated})
	must(err)
	must(win.SetTitle("GOUI Input Callbacks Verification"))
	_ = win.SetPosition(nil, geometry.Point{X: 100, Y: 100})
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(geometry.UniformInsets(24))
	root.SetSpacing(18)
	root.SetCrossAlign(layout.CrossStretch)
	root.AddChild(gui.NewLabel("Shift + double click + third press/drag; F8 resets; F6 checks."))
	a, b := newProbe("A"), newProbe("B")
	row := gui.NewLinearBox(layout.DirectionHorizontal)
	row.SetSpacing(24)
	row.AddChild(a.box)
	row.AddChild(b.box)
	root.AddChild(row)
	text := gui.NewTextView()
	text.SetName("Triple-click logical lines")
	text.SetModel(gui.NewTextModel("first row\nsecond row\nthird row\nfourth row\n"))
	text.SetMinSize(geometry.Size{Height: 180})
	text.SetMaxSize(geometry.Size{Height: 180})
	root.AddChild(text)
	text.ConnectSelection(func(selection gui.TextSelection) {
		log.Printf("TEXT selection=%d,%d", selection.Anchor, selection.Caret)
	})
	inspect := func() {
		for _, p := range []*probe{a, b} {
			check(!p.click.Pressed() && !p.drag.Dragging() && !p.box.Snapshot().Pressed, "空闲时仍保留 Pressed/Drag 状态")
			log.Printf("CHECK %s completed=%d released=%d canceled=%d ended=%d", p.name, p.completed, p.released, p.canceled, p.ended)
		}
		log.Print("CHECK PASS")
	}
	button := gui.NewButton()
	button.SetChild(gui.NewLabel("Check / F6"))
	button.ConnectClicked(inspect)
	root.AddChild(button)
	keys := gui.NewKeyEventController()
	keys.SetPhase(gui.PhaseCapture)
	keys.ConnectKeyDown(func(ctx gui.EventContext, e events.KeyEvent) {
		switch e.Key {
		case events.KeyF6:
			inspect()
			ctx.StopPropagation()
		case events.KeyF8:
			a.reset()
			b.reset()
			log.Print("RESET")
			ctx.StopPropagation()
		}
	})
	root.AddEventController(keys)
	win.SetWidget(root)
	must(win.Show())
	ready := app.NewTimer()
	ready.ConnectTimeout(func() {
		bounds, _ := json.Marshal(map[string]geometry.Rectangle{"A": a.box.Snapshot().Bounds, "B": b.box.Snapshot().Bounds, "text": text.Snapshot().Bounds, "check": button.Snapshot().Bounds})
		lineHeight := (text.ContentSize().Height - text.Padding().Vertical()) / float32(text.Model().LineCount())
		fmt.Printf("READY OS=%s typography=%s lineHeight=%g padding=%g bounds=%s\n", runtime.GOOS, app.Typography().Name(), lineHeight, text.Padding().Top, bounds)
	})
	must(ready.StartOnce(400 * time.Millisecond))
	win.ConnectDestroy(func() { log.Print("DESTROY PASS") })
	must(app.Run())
	ready.Stop()
	log.Print("EXIT PASS")
}
