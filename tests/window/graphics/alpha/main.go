// 预乘颜色窗口验证
//
// 环境：Linux / Windows / macOS 桌面；记录后端、桌面环境、缩放比例。
// 启动：go run ./tests/window/graphics/alpha
// 使用 GOUI_PLAT_PAINTER=software/opengl（Windows 还可 direct2d），
// GOUI_PLAT_SCALE=1 或 2。加 -transparent 验证透明呈现，需要桌面合成器。
// 初始：白底，各行左侧是原生画刷／文字，右侧是参考图片／文字。
//
// 操作与逐项预期：
//  1. 比较 Fill、Stroke、Gradient、Shadow 行左右内容。填充和阴影内部的
//     粉红色必须与参考图片一致；描边与参考色条一致；渐变中点和端点不偏暗。
//  2. 比较 Text 行两组 MMMM：左侧为默认半透明颜色，右侧为局部文字颜色，
//     颜色／透明度应一致。Half/Zero/Half 行的中间一组保留位置但完全不可见。
//  3. 在 1x、2x 下分别运行上述步骤；允许边缘栅格化细微差异，内部颜色不能变。
//  4. -transparent 模式下点击依次切换：对照场景 → 半透明 Clear → 清零后
//     全窗口 FillRect → 清零后全窗口 DrawImage → 对照场景。后三种画面必须
//     相同，桌面背景透过的程度也必须一致；日志给出当前模式。
//
// 平台差异：透明模式使用无装饰窗口，不提供拖动／缩放模拟；普通模式使用原生装饰。
// 副作用：不修改系统设置、剪贴板或文件。复位：重启。退出：Esc 或关闭普通窗口。
package main

import (
	"flag"
	"image"
	"image/color"
	"image/draw"
	"log"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	transparent := flag.Bool("transparent", false, "transparent frameless window; Esc exits")
	flag.Parse()
	if err := run(*transparent); err != nil {
		log.Fatal(err)
	}
}

func run(transparent bool) error {
	plat, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.AlphaTest")
	if err != nil {
		return err
	}
	defer plat.Destroy()
	loop, err := plat.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	ty, err := plat.NewTypography()
	if err != nil {
		return err
	}
	defer ty.Destroy()
	var labels []typography.TextLayout
	defer func() {
		for _, label := range labels {
			label.Destroy()
		}
	}()
	ink := color.NRGBA{R: 255, A: 128}
	for _, text := range []string{"Fill", "Stroke", "Gradient", "Shadow", "Text", "Half/Zero/Half", "MMMM", "MMMM", "MMMMMMMMMMMM"} {
		fg := color.Color(color.Black)
		if len(labels) >= 6 {
			fg = ink
		}
		layout, err := ty.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "sans", Size: 20}, TextColor: fg}, 400, 50)
		if err != nil {
			return err
		}
		labels = append(labels, layout)
	}
	labels[7].SetTextColor(0, 4, ink)
	labels[8].SetTextColor(4, 4, color.Transparent)
	var win platform.Window
	var p graphics.Painter
	var swatch, gradient graphics.Image
	var size events.SizeEvent
	mode := 0
	c := graphics.ColorOf(ink)
	paint := func() {
		if p == nil || size.Width <= 0 || size.Height <= 0 {
			return
		}
		p.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
		defer p.End()
		full := graphics.Rect(0, 0, size.Width, size.Height)
		if mode != 0 {
			p.Clear(graphics.Color{})
			switch mode {
			case 1:
				p.Clear(c)
			case 2:
				p.FillRect(full, c)
			case 3:
				p.DrawImage(full, swatch)
			}
			return
		}
		p.Clear(graphics.RGB(255, 255, 255))
		for i := 0; i < 6; i++ {
			p.DrawTextLayout(graphics.Point{X: 16, Y: float32(16 + i*62)}, labels[i])
		}
		p.FillRect(graphics.Rect(190, 16, 120, 40), c)
		p.DrawImage(graphics.Rect(340, 16, 120, 40), swatch)
		p.DrawRect(graphics.Rect(194, 82, 112, 32), 8, c)
		p.DrawImage(graphics.Rect(340, 78, 120, 8), swatch)
		p.FillRect(graphics.Rect(190, 140, 120, 40), graphics.LinearGradient{Start: graphics.Point{X: 190}, End: graphics.Point{X: 310}, StartColor: graphics.RGBA(255, 0, 0, 64), EndColor: graphics.RGBA(0, 0, 255, 192)})
		p.DrawImage(graphics.Rect(340, 140, 120, 40), gradient)
		p.DrawBoxShadow(graphics.Rect(190, 202, 120, 40), 4, graphics.BoxShadow{Color: c, BlurRadius: 4})
		p.DrawImage(graphics.Rect(340, 202, 120, 40), swatch)
		p.DrawTextLayout(graphics.Point{X: 190, Y: 264}, labels[6])
		p.DrawTextLayout(graphics.Point{X: 340, Y: 264}, labels[7])
		p.DrawTextLayout(graphics.Point{X: 190, Y: 326}, labels[8])
	}
	chrome := platform.WindowChromeNative
	if transparent {
		chrome = platform.WindowChromeNone
	}
	win, err = plat.NewWindow(geometry.Size{Width: 600, Height: 410}, func(e platform.Event) {
		switch e := e.(type) {
		case events.SizeEvent:
			size = e
		case events.CloseEvent:
			loop.Quit()
		case events.KeyEvent:
			if e.Key == events.KeyEscape {
				loop.Quit()
			}
		case events.PointerEvent:
			if transparent && e.EventType == events.PointerDown && win != nil {
				mode = (mode + 1) % 4
				log.Printf("mode=%d (0=compare, 1=Clear, 2=FillRect, 3=Image)", mode)
				if err := win.RequestPaint(); err != nil {
					log.Print(err)
				}
			}
		case events.PaintEvent:
			paint()
		}
	}, platform.WindowOptions{Chrome: chrome, Transparent: transparent})
	if err != nil {
		return err
	}
	defer win.Destroy()
	if err = win.SetTitle("GOUI premultiplied alpha — Esc exits"); err != nil {
		return err
	}
	p, err = plat.NewPainter(win)
	if err != nil {
		return err
	}
	defer p.Destroy()
	src := image.NewNRGBA(image.Rect(0, 0, 120, 40))
	draw.Draw(src, src.Bounds(), image.NewUniform(ink), image.Point{}, draw.Src)
	swatch, err = p.NewImage(src)
	if err != nil {
		return err
	}
	defer swatch.Destroy()
	ref := image.NewRGBA(src.Bounds())
	for y := 0; y < 40; y++ {
		for x := 0; x < 120; x++ {
			t := float64(x) + .5
			t /= 120
			ref.SetRGBA(x, y, color.RGBA{R: byte(64*(1-t) + .5), B: byte(192*t + .5), A: byte(64*(1-t) + 192*t + .5)})
		}
	}
	gradient, err = p.NewImage(ref)
	if err != nil {
		return err
	}
	defer gradient.Destroy()
	log.Printf("Painter=%s transparent=%v; Esc exits", p.Name(), transparent)
	if err = win.Show(); err != nil {
		return err
	}
	loop.Run()
	return nil
}
