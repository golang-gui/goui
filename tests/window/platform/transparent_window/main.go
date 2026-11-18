// 透明 Window 与 Popup 的交互绘制对照。
// 环境：三平台合成桌面，GOUI_PLAT_PAINTER=software/opengl（Windows 另有 direct2d），1x/2x。
// 启动：go run ./tests/window/platform/transparent_window
// 初始：透明窗口与浮窗显示半透明圆角色块。
// 操作与预期：分别点击两个 Surface，内容应在半透明色块与全透明清除之间切换；
// 缩放及重复点击不得留下上帧内容或黑色背景。能力不足时必须报告原生错误。
// 副作用：仅创建窗口/浮窗。复位：重启。退出：关闭主窗口。
package main

import (
	"log"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

func init() { runtime.LockOSThread() }

type scene struct {
	surface platform.Surface
	painter graphics.Painter
	size    events.SizeEvent
	hidden  bool
}

func (s *scene) event(event platform.Event) {
	switch e := event.(type) {
	case events.SizeEvent:
		s.size = e
	case events.PointerEvent:
		if e.EventType == events.PointerDown && s.surface != nil {
			s.hidden = !s.hidden
			s.surface.RequestPaint()
		}
	case events.PaintEvent:
		if s.painter == nil || s.size.Width <= 0 || s.size.Height <= 0 {
			return
		}
		p := s.painter
		p.Begin(s.size.PixelWidth, s.size.PixelHeight, s.size.PixelWidth/s.size.Width)
		p.Clear(graphics.Color{})
		if !s.hidden {
			body := graphics.Rect(24, 24, s.size.Width-48, s.size.Height-48)
			p.DrawBoxShadow(body, geometry.UniformCornerRadius(20), graphics.BoxShadow{Color: graphics.Color{A: .35}, Offset: geometry.Point{Y: 4}, BlurRadius: 12})
			p.FillRoundRect(body, 20, graphics.Color{R: .15 * .65, G: .5 * .65, B: .95 * .65, A: .65})
			p.FillRoundRect(graphics.Rect(40, 40, 64, 48), 12, graphics.Color{R: .5, G: .3 * .5, A: .5})
			p.FillRoundRect(graphics.Rect(120, 40, 64, 48), 12, graphics.Color{R: .3, G: 1, A: 1})
		}
		p.End()
	}
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	plat, err := platform.NewPlatform(platform.DefaultName(), "")
	if err != nil {
		return err
	}
	defer plat.Destroy()
	loop, err := plat.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	mainScene, popupScene := &scene{}, &scene{}
	win, err := plat.NewWindow(geometry.Size{Width: 440, Height: 300}, func(e platform.Event) {
		if _, ok := e.(events.CloseEvent); ok {
			loop.Quit()
			return
		}
		mainScene.event(e)
	}, platform.WindowOptions{Chrome: platform.WindowChromeNone, Transparent: true})
	if err != nil {
		return err
	}
	defer win.Destroy()
	mainScene.surface = win
	win.(platform.WindowManagement).SetTitle("Transparent Window + Popup — click to clear")
	mainScene.painter, err = plat.NewPainter(win)
	if err != nil {
		return err
	}
	defer mainScene.painter.Destroy()
	popup, err := plat.NewPopup(win, geometry.Size{Width: 300, Height: 200}, popupScene.event, platform.PopupOptions{Transparent: true})
	if err != nil {
		return err
	}
	defer popup.Destroy()
	popupScene.surface = popup
	popupScene.painter, err = plat.NewPainter(popup)
	if err != nil {
		return err
	}
	defer popupScene.painter.Destroy()
	if err = win.(platform.WindowManagement).Show(); err != nil {
		return err
	}
	popup.SetPosition(300, 100)
	if err = popup.Show(); err != nil {
		return err
	}
	log.Printf("Window: %s, Popup: %s; alpha enabled independently on both", mainScene.painter.Name(), popupScene.painter.Name())
	loop.Run()
	return nil
}
