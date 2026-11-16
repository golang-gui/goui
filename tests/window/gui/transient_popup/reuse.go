package main

import (
	"errors"
	"image"
	"image/color"
	"log"
	"math"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/widgets"
)

// 仅有相应原生能力的平台安装附加检查，不为其他平台伪造 DPI 消息。
var nativeDPIProbe func(platform.Popup)
var nativePixelProbe func(platform.Popup, geometry.Size)

// 低层契约用原生 Popup 验证，避免把 GUI 的请求值误当作原生实际值。
func verifyNativeReuse(app gui.Application, win gui.Window) {
	for _, transparent := range []bool{false, true} {
		var popup platform.Popup
		var painter graphics.Painter
		paints, enabled := 0, false
		var err error
		popup, err = app.Platform().NewPopup(win.PlatformWindow(), geometry.Size{Width: 80, Height: 35}, func(event platform.Event) {
			if _, paint := event.(events.PaintEvent); !paint || !enabled || painter == nil {
				return
			}
			logical, pixels, err := popup.ClientSize()
			must(err)
			painter.Begin(pixels.Width, pixels.Height, pixels.Width/logical.Width)
			painter.Clear(graphics.RGB(17, 34, 51))
			scale := pixels.Width / logical.Width
			painter.FillRect(graphics.Rect(9/scale, 11/scale, 8/scale, 8/scale), graphics.RGB(255, 0, 0))
			painter.FillRect(graphics.Rect((pixels.Width-19)/scale, (pixels.Height-23)/scale, 8/scale, 8/scale), graphics.RGB(0, 255, 0))
			painter.End()
			paints++
		}, platform.PopupOptions{Transparent: transparent})
		// 无 alpha 环境只将明确的透明能力缺口记为跳过，其他错误仍失败。
		if transparent && (errors.Is(err, platform.ErrUnsupported) || errors.Is(err, platform.ErrUnavailable)) {
			log.Printf("NATIVE SKIP alpha: %v", err)
			continue
		}
		must(err)
		painter, err = app.Platform().NewPainter(popup)
		must(err)
		if nativeDPIProbe != nil {
			nativeDPIProbe(popup)
		}
		handle := popup.NativeHandle()
		for i := range 16 {
			must(popup.Hide())
			// 尺寸变化前覆盖所有帧外资源入口；它们不能提前绑定旧窗口缓冲。
			source := image.NewRGBA(image.Rect(0, 0, 2, 2))
			source.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
			texture, err := painter.NewImage(source)
			must(err)
			must(texture.Update(source))
			_, err = painter.RenderImage(8, 8, 1, func() { painter.Clear(graphics.RGB(3, 5, 7)) })
			must(err)
			texture.Destroy()
			request := geometry.Size{Width: 80.25, Height: 35.25}
			if i%4 == 1 {
				request = geometry.Size{Width: 220.5, Height: 130.5}
			}
			popup.SetSize(request.Width, request.Height)
			logical, pixels, err := popup.ClientSize()
			must(err)
			scale := pixels.Width / logical.Width
			// 独立预期：小数 DIP 只在平台边界向上包含到物理像素；浮点换算容差 0.001 像素。
			if math.Abs(float64(pixels.Width)-math.Ceil(float64(request.Width*scale))) > .001 ||
				math.Abs(float64(pixels.Height)-math.Ceil(float64(request.Height*scale))) > .001 || popup.NativeHandle() != handle {
				log.Fatalf("NATIVE FAIL request=%v logical=%v pixels=%v handle=%x want=%x", request, logical, pixels, popup.NativeHandle(), handle)
			}
			popup.SetPosition(float32(20+i%3*70), 50)
			enabled = false
			must(popup.Show())
			enabled = true
			before := paints
			must(popup.Repaint())
			if paints <= before {
				log.Fatalf("NATIVE FAIL Repaint 没有同步进入原生绘制回调：alpha=%v iteration=%d before=%d after=%d", transparent, i, before, paints)
			}
			actual, backing, err := popup.ClientSize()
			must(err)
			if nativePixelProbe != nil {
				nativePixelProbe(popup, pixels)
			}
			if actual != logical || backing != pixels {
				log.Fatal("NATIVE FAIL 显示改变了已确认的原生尺寸")
			}
		}
		enabled = false
		log.Printf("NATIVE PASS alpha=%v backend=%s handle=%x paints=%d", transparent, painter.Name(), handle, paints)
		painter.Destroy()
		popup.Destroy()
		_, _, err = popup.ClientSize()
		if !errors.Is(err, platform.ErrUnavailable) || !errors.Is(popup.Repaint(), platform.ErrUnavailable) {
			log.Fatal("NATIVE FAIL 销毁后的查询或绘制没有返回 ErrUnavailable")
		}
	}
}

func startReuseVerification(app gui.Application, win gui.Window, menus [3]*gui.MenuButton, lists [3]*widgets.DropDown) {
	step := 0
	movedAt := -1
	var previous gui.PopoverTarget
	var bounds geometry.Rectangle
	frames := make(map[gui.Root]int)
	seen := make(map[gui.Widget]gui.Root)
	timer := app.NewTimer()
	timer.ConnectTimeout(func() {
		if previous != nil {
			if !previous.Visible() || previous.Widget().Rect() != bounds {
				log.Fatalf("REUSE FAIL 显示后几何跳动：before=%v after=%v", bounds, previous.Widget().Rect())
			}
			must(win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyEscape}))
			if previous.Visible() {
				log.Fatal("REUSE FAIL Esc 未关闭当前浮层")
			}
			previous = nil
		}
		if step == 60 {
			timer.Stop()
			log.Printf("REUSE PASS presentations=%d controls=%d", step, len(seen))
			win.Destroy()
			return
		}
		if step%18 == 0 && movedAt != step {
			area, err := win.WorkAreaAt(geometry.Point{})
			must(err)
			position := geometry.Point{X: 100, Y: 100}
			if step%36 == 18 {
				position = geometry.Point{X: max(0, area.Width-680-10), Y: max(0, area.Height-460-40)}
			}
			must(win.SetPosition(nil, position))
			movedAt = step
			// 让窗口管理器在下一次输入前处理移动；Show 仍自行查询实际 owner 原点。
			return
		}
		index := step % 3
		var anchor gui.Widget = menus[index]
		if step%2 != 0 {
			anchor = lists[index]
		}
		known := seen[anchor]
		before := frames[known]
		for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
			must(win.DispatchEvent(events.PointerEvent{EventType: kind, Position: anchor.Snapshot().Bounds.Center(), Button: events.PointerButtonLeft}))
		}
		popovers := win.Popovers()
		if len(popovers) != 1 || popovers[0].Anchor() != anchor {
			log.Fatalf("REUSE FAIL 当前浮层数量或锚点错误：step=%d count=%d", step, len(popovers))
		}
		previous = popovers[0]
		bounds = previous.Widget().Rect()
		if bounds.Width <= 0 || bounds.Height <= 0 || len(previous.Widget().Snapshot().Children) == 0 {
			log.Fatal("REUSE FAIL Show 返回时未完成当前内容布局")
		}
		if known == nil {
			current := previous.Widget().Root()
			seen[anchor] = current
			if _, watched := frames[current]; !watched {
				frames[current] = 0
				current.ConnectAfterPaint(func(gui.Painter, geometry.Rectangle) { frames[current]++ })
			}
		} else if previous.Widget().Root() != known || frames[known] <= before {
			log.Fatalf("REUSE FAIL 重开没有同步绘制当前内容：step=%d frames=%d before=%d", step, frames[known], before)
		}
		log.Printf("REUSE step=%d anchor=%v body=%v role=%s", step, anchor.Snapshot().Bounds, bounds, previous.Widget().Snapshot().Role)
		step++
	})
	must(timer.Start(100 * time.Millisecond))
}
