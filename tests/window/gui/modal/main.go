// 通用 ModalScope 桌面验收，不使用 DevServer。
//
// 环境：Linux 独立 X11 会话、Windows 或 macOS 桌面；通过 GOUI_PLAT_PAINTER
// 选择后端，通过 GOUI_PLAT_SCALE 选择倍率。启动：go run ./tests/window/gui/modal
// 可加 -auto，使用正常 Window.DispatchEvent 路径完成断言后退出；仍需真实输入验收。
// 初始：Integrated 窗口，后台编辑框和计数按钮，确认层隐藏，所有计数为 0。
//
// 操作与预期：
//  1. 在后台编辑框输入文字并按 F5：窗口 F5 增加；点击背景计数也正常增加。
//  2. 点 Open modal：首个焦点进入确认层；在内部输入中文，后台文本保持原值。
//  3. 连续 Tab/Shift+Tab：焦点仅在内部循环；按 F5 和 F6，窗口/祖先计数不增加。
//  4. 点击、滚动遮罩外或被覆盖的后台内容：后台不响应；Inside count 仅增加内部计数。
//  5. 点 Popup：内部原生 Popover 可交互，Esc 先关闭 Popover，确认层继续显示；
//     再按 Esc 关闭确认层，焦点回到打开前的后台控件；F5 随后恢复。
//  6. 在卡片外遮罩空白处拖动：Integrated/None 支持范围内窗口可移动；卡片不能
//     被误当成 Caption，点击遮罩不关闭。双击 Caption 沿用最大化/恢复语义。
//  7. 调整窗口大小、最大化/还原，再次打开/关闭确认层，文字和输入仍正常。
//  8. 点 Theme 切换亮暗；模态行为不变，卡片、遮罩和圆角边缘无裁剪异常。
//
// 平台差异：Native 标题栏不承诺客户区 Caption 拖动；字体、IME 候选窗和原生装饰
// 由系统决定。-auto 不替代真实系统 IME、桌面拖窗和视觉验收。
// 副作用：仅本进程窗口/Popover，不改系统设置或剪贴板。复位：重新启动。
// 退出：关闭窗口或后台 Exit；-auto 输出 PASS 后退出，断言失败退出非零。
// 自动化启动方应另设进程超时，避免原生事件循环异常时无限等待。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/theme/modern"
)

// 自定义全区域层组合普通布局和 ModalScope，不依赖专用宿主或 Dialog 控件。
type confirmation struct {
	gui.WidgetBase
	child  gui.Widget
	card   geometry.Rectangle
	scope  *gui.ModalScope
	dark   bool
	logged bool
}

func newConfirmation(child gui.Widget) *confirmation {
	c := &confirmation{child: child}
	c.SetVisible(false)
	c.WidgetBase.AddChild(c, child)
	c.scope = gui.NewModalScope(c)
	return c
}
func (c *confirmation) Child() gui.Widget { return c.child }
func (c *confirmation) SetChild(child gui.Widget) {
	if c.child == child {
		return
	}
	c.WidgetBase.RemoveChild(c.child)
	c.child = child
	c.WidgetBase.AddChild(c, child)
	c.RequestLayout()
}
func (c *confirmation) Measure(constraint layout.Constraint) layout.Measurement {
	return layout.Measured(constraint.Clamp(geometry.Size{}))
}
func (c *confirmation) Arrange(rect geometry.Rectangle) {
	c.WidgetBase.Arrange(rect)
	available := geometry.Size{Width: max(0, rect.Width-96), Height: max(0, rect.Height-96)}
	m := c.child.Measure(layout.Loose(available))
	c.card = geometry.Rect((rect.Width-m.Width-48)/2, (rect.Height-m.Height-48)/2, m.Width+48, m.Height+48)
	c.child.Arrange(geometry.Rect(c.card.X+24, c.card.Y+24, m.Width, m.Height))
}
func (c *confirmation) Paint(p gui.Painter) {
	p.FillRect(geometry.Rectangle{Size: c.Rect().Size}, graphics.ColorOf(color.NRGBA{A: 110}))
	p.DrawBoxShadow(c.card, 12, graphics.BoxShadow{Color: graphics.ColorOf(color.NRGBA{A: 70}), Offset: geometry.Point{Y: 4}, BlurRadius: 10})
	background := color.RGBA{R: 250, G: 250, B: 252, A: 255}
	if c.dark {
		background = color.RGBA{R: 38, G: 38, B: 42, A: 255}
	}
	p.FillRoundRect(c.card, 12, graphics.ColorOf(background))
	p.DrawRoundRect(c.card, 12, 1, graphics.ColorOf(color.NRGBA{R: 128, G: 128, B: 128, A: 100}))
	if !c.logged {
		c.logged = true
		resource, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		if err != nil {
			panic(err)
		}
		fmt.Printf("FRAME backend=%T scale=%g\n", resource, p.PixelScale())
		resource.Destroy()
	}
}

func button(text string, fn func()) *gui.Button {
	b := gui.NewButton()
	b.SetChild(gui.NewLabel(text))
	b.ConnectClicked(fn)
	return b
}
func vbox(children ...gui.Widget) *gui.LinearBox {
	b := gui.NewLinearBox(layout.DirectionVertical)
	b.SetSpacing(12)
	b.SetCrossAlign(layout.CrossStretch)
	for _, w := range children {
		b.AddChild(w)
	}
	return b
}
func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

func main() {
	auto := flag.Bool("auto", false, "自动断言后退出")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.ModalScope")
	if err != nil {
		log.Fatal(err)
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 820, Height: 610}, Chrome: gui.WindowChromeIntegrated, Center: true})
	if err != nil {
		log.Fatal(err)
	}
	win.SetTitle("GOUI ModalScope")
	background, inside, windowKeys, ancestorKeys := 0, 0, 0, 0
	status := gui.NewLabel("")
	backInput, input := gui.NewTextInput(), gui.NewTextInput()
	backInput.SetText("background")
	var layer *confirmation
	var popup gui.Popover
	report := func() {
		status.SetText(fmt.Sprintf("Background %d | Inside %d | Window F5 %d | Ancestor F6 %d", background, inside, windowKeys, ancestorKeys))
		fmt.Printf("STATE background=%d inside=%d F5=%d F6=%d modal=%v\n", background, inside, windowKeys, ancestorKeys, layer != nil && layer.scope.Active())
	}
	insideCount := button("Inside count", func() { inside++; report() })
	var popupButton *gui.Button
	popupButton = button("Popup", func() {
		if popup != nil {
			popup.Destroy()
		}
		popup = gui.NewPopover(popupButton, nil)
		popup.SetModal(true)
		popup.SetAutoDismiss(true)
		popup.SetWidget(vbox(gui.NewLabel("Dialog-owned popup"), button("Close popup", func() { popup.Hide() })))
		popup.ConnectDismissRequest(func() { popup.Hide() })
		if err := popup.Show(); err != nil {
			panic(err)
		}
		fmt.Println("POPUP shown")
	})
	card := vbox(gui.NewLabel("Modal input boundary"), input, insideCount, popupButton, button("Close modal", func() { layer.SetVisible(false); report() }))
	card.SetMinSize(geometry.Size{Width: 330})
	layer = newConfirmation(card)
	keys := gui.NewKeyEventController()
	keys.SetPhase(gui.PhaseBubble)
	keys.ConnectKeyDown(func(ctx gui.EventContext, e events.KeyEvent) {
		if e.Key == events.KeyEscape {
			layer.SetVisible(false)
			e.PreventDefault()
			ctx.StopPropagation()
			report()
		}
	})
	layer.AddEventController(keys)
	open := button("Open modal", func() { layer.SetVisible(true); report() })
	backCount := button("Background count", func() { background++; report() })
	caption := gui.NewHeaderBar()
	caption.SetChild(gui.NewLabel("GOUI ModalScope — reusable input boundary"))
	content := vbox(gui.NewLabel("Background editor — F5 / F6 counters"), backInput, open, backCount, button("Theme", func() {
		layer.dark = !layer.dark
		app.SetStyleSheet(modern.Sheet(modern.Options{Dark: layer.dark}))
		report()
	}), button("Exit", win.Destroy), status)
	content.SetPadding(geometry.UniformInsets(20))
	content.SetMainWeight(1)
	base := vbox(caption, content)
	base.SetSpacing(0)
	root := gui.NewOverlay()
	root.SetChild(base)
	root.AddOverlay(layer)
	root.SetOverlayFill(layer, true)
	win.SetWidget(root)
	shortcut := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyF5})
	shortcut.ConnectActivate(func() { windowKeys++; report() })
	win.Shortcuts().SetPhase(gui.PhaseCapture)
	win.Shortcuts().AddShortcut(shortcut)
	ancestor := gui.NewKeyEventController()
	ancestor.SetPhase(gui.PhaseCapture)
	ancestor.ConnectKeyDown(func(_ gui.EventContext, e events.KeyEvent) {
		if e.Key == events.KeyF6 {
			ancestorKeys++
			report()
		}
	})
	root.AddEventController(ancestor)
	win.Chrome().ConnectQueryRegion(func(p geometry.Point, r *gui.ChromeRegion) {
		if layer.Visible() && gui.Pick(root, p) == layer {
			*r = gui.ChromeRegionCaption
		}
	})
	win.ConnectDestroy(func() {
		if popup != nil {
			popup.Destroy()
		}
	})
	report()
	if err := win.Show(); err != nil {
		log.Fatal(err)
	}
	step := 0
	timer := app.NewTimer()
	timer.ConnectTimeout(func() {
		if step == 0 {
			for _, item := range []struct {
				name   string
				widget gui.Widget
			}{{"open", open}, {"background", backCount}, {"input", backInput}, {"inside", insideCount}, {"popup", popupButton}} {
				b := item.widget.Snapshot().Bounds
				fmt.Printf("BOUNDS %s %.1f %.1f %.1f %.1f\n", item.name, b.X, b.Y, b.Width, b.Height)
			}
		}
		if !*auto {
			timer.Stop()
			return
		}
		click := func(w gui.Widget) {
			p := w.Snapshot().Bounds.Center()
			for _, kind := range []events.EventType{events.PointerMove, events.PointerDown, events.PointerUp} {
				if err := win.DispatchEvent(events.PointerEvent{EventType: kind, Position: p, Button: events.PointerButtonLeft}); err != nil {
					panic(err)
				}
			}
		}
		key := func(k events.Key) {
			if err := win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: k}); err != nil {
				panic(err)
			}
		}
		switch step {
		case 0:
			click(backInput)
			key(events.KeyF5)
			key(events.KeyF6)
			layer.SetVisible(true)
		case 1:
			require(layer.scope.Active() && win.FocusedWidget() == input, "initial modal focus")
			key(events.KeyF5)
			key(events.KeyF6)
			click(backCount)
			require(background == 0 && windowKeys == 1 && ancestorKeys == 1, "background input escaped")
			click(input)
			if err := win.CommitText("modal 中文"); err != nil {
				panic(err)
			}
			require(input.Text() == "modal 中文" && backInput.Text() == "background", "commit target")
		case 2:
			for range 12 {
				key(events.KeyTab)
				require(win.FocusedWidget() != backInput && win.FocusedWidget() != open, "Tab escaped")
			}
			click(insideCount)
			require(inside == 1, "inside click")
			click(popupButton)
		case 3:
			require(popup != nil && popup.Visible(), "popup did not open")
			key(events.KeyEscape)
		case 4:
			require(!popup.Visible() && layer.scope.Active(), "popup close lost modal")
			key(events.KeyEscape)
		case 5:
			require(!layer.scope.Active() && win.FocusedWidget() == backInput, "focus restore")
			key(events.KeyF5)
			require(windowKeys == 2, "window shortcut restore")
			fmt.Printf("PASS backend=%s scale=%s\n", os.Getenv("GOUI_PLAT_PAINTER"), os.Getenv("GOUI_PLAT_SCALE"))
			timer.Stop()
			win.Destroy()
		}
		step++
	})
	if err := timer.Start(500 * time.Millisecond); err != nil {
		log.Fatal(err)
	}
	app.Run()
	if *auto {
		require(step >= 6, "window exited before checks completed")
	}
}
