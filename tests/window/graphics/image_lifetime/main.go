// 图片帧内销毁验证
//
// 环境：Linux / Windows / macOS 桌面。启动：
// GOUI_PLAT_PAINTER=opengl go run ./tests/window/graphics/image_lifetime
// 后端可选 software / opengl，Windows 另有 direct2d；GOUI_PLAT_SCALE=1 或 2。
// 初始：白底，两行色块；上行是 DrawImage 后立即 Destroy 的图片，下行是画刷参考。
// 操作与预期：
//  1. 上下对应色块内部必须同色：左红、右蓝，不应变黑、消失或串色。
//  2. 点击客户区切换红蓝顺序；连续点击及缩放窗口，上下始终一致，无 panic。
//  3. 分别用各后端、1x / 2x 重复；允许纹理边缘细微差异，内部颜色一致。
//
// 每帧创建、绘制、销毁两张图片，且重复 Destroy；结束时 Painter 兜底清理。
// 平台差异：原生窗口装饰不同。副作用：无持久写入。复位：重启。
// 退出：Esc 或关闭窗口。此例是生命周期回归，不用于性能基准。
package main

import (
	"image"
	"image/color"
	"log"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	plat, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.ImageLifetimeTest")
	if err != nil {
		return err
	}
	defer plat.Destroy()
	loop, err := plat.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	var win platform.Window
	var p graphics.Painter
	var size events.SizeEvent
	swapped := false
	paint := func() {
		if p == nil || size.Width <= 0 || size.Height <= 0 {
			return
		}
		p.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
		defer p.End()
		p.Clear(graphics.RGB(255, 255, 255))
		inks := []color.RGBA{{R: 128, A: 128}, {B: 128, A: 128}}
		if swapped {
			inks[0], inks[1] = inks[1], inks[0]
		}
		width, height := (size.Width-60)/2, (size.Height-60)/2
		if width <= 0 || height <= 0 {
			return
		}
		for n, ink := range inks {
			src := image.NewRGBA(image.Rect(0, 0, 2, 2))
			for y := 0; y < 2; y++ {
				for x := 0; x < 2; x++ {
					src.SetRGBA(x, y, ink)
				}
			}
			img, err := p.NewImage(src)
			if err != nil {
				panic(err)
			}
			x := 20 + float32(n)*(width+20)
			p.DrawImage(graphics.Rect(x, 20, width, height), img)
			img.Destroy()
			img.Destroy()
			p.FillRect(graphics.Rect(x, 40+height, width, height), graphics.ColorOf(ink))
		}
	}
	win, err = plat.NewWindow(geometry.Size{Width: 440, Height: 280}, func(e platform.Event) {
		switch e := e.(type) {
		case events.SizeEvent:
			size = e
		case events.PaintEvent:
			paint()
		case events.CloseEvent:
			loop.Quit()
		case events.KeyEvent:
			if e.Key == events.KeyEscape {
				loop.Quit()
			}
		case events.PointerEvent:
			if e.EventType == events.PointerDown && win != nil {
				swapped = !swapped
				if err := win.RequestPaint(); err != nil {
					log.Print(err)
				}
			}
		}
	}, platform.WindowOptions{Chrome: platform.WindowChromeNative})
	if err != nil {
		return err
	}
	defer win.Destroy()
	if err := win.(platform.WindowManagement).SetTitle("GOUI Image Lifetime — top images / bottom reference"); err != nil {
		return err
	}
	p, err = plat.NewPainter(win)
	if err != nil {
		return err
	}
	defer p.Destroy()
	log.Printf("Painter=%s; click to swap colors, Esc exits", p.Name())
	if err := win.(platform.WindowManagement).Show(); err != nil {
		return err
	}
	loop.Run()
	return nil
}
