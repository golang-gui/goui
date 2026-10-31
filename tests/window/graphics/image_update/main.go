// Image.Update 帧内时序及原生像素验收。
//
// 环境：Linux X11、Windows 或 macOS 桌面；记录实际后端和缩放。
// 启动：go run ./tests/window/graphics/image_update [-auto]
// GOUI_PLAT_PAINTER=software/opengl，Windows 另可 direct2d；GOUI_PLAT_SCALE=1/2。
// 初始：自动执行 1x/1.5x/2x 离屏断言；窗口左侧蓝色或红色半透明块，右侧绿色块。
// 操作与预期：
//  1. 检查 PIXELS PASS：首次绘制前允许连续更新，绘制后更新被拒绝；拒绝不破坏
//     先前绘制，另一张图仍能更新，源像素随后改变不会影响已上传内容。
//  2. 点击窗口或按空格，两侧色块保持位置，左侧红蓝切换，右侧保持绿色；每帧输出
//     FRAME PASS。更新发生在 Begin/End 内，不能出现 context 丢失、崩溃或消失。
//  3. 以各后端和 1x/2x 重启；透明合成、Transform、clip 由离屏像素断言核验。
//     -auto 在两个原生帧提交及其间离屏检查后退出，不代替步骤 2 的视觉验收。
//
// 平台差异：窗口装饰使用原生样式；不依赖透明窗口或桌面合成器。
// 副作用：创建窗口与日志，不写文件、设置或剪贴板。重新启动复位；Escape/关闭退出。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"runtime"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

func main() {
	if err := run(); err != nil {
		log.Fatal("FAIL: ", err)
	}
}

func run() error {
	auto := flag.Bool("auto", false, "完成像素断言及两个原生帧后退出")
	flag.Parse()
	plat, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.ImageUpdate")
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
	var failure error
	var a, b graphics.Image
	frames := 0
	paint := func() {
		if p == nil || size.Width <= 0 || failure != nil {
			return
		}
		if frames == 0 {
			if failure = checkPixels(p); failure != nil {
				loop.Quit()
				return
			}
		}
		p.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
		func() {
			defer p.End()
			p.Clear(graphics.RGB(240, 240, 240))
			ink := color.NRGBA{B: 255, A: 128}
			if frames%2 != 0 {
				ink = color.NRGBA{R: 255, A: 128}
			}
			if failure = a.Update(solid(ink)); failure != nil {
				return
			}
			p.DrawImage(graphics.Rect(20, 20, 180, 120), a)
			// 即使另一幅图片在 A 绘制之后更新，也不解除当前 GL context。
			if failure = b.Update(solid(color.NRGBA{G: 180, A: 255})); failure != nil {
				return
			}
			p.DrawImage(graphics.Rect(220, 20, 180, 120), b)
			failure = rejected(a, solid(ink))
		}()
		if failure != nil {
			loop.Quit()
			return
		}
		frames++
		fmt.Printf("FRAME PASS OS=%s painter=%s scale=%g frame=%d\n", runtime.GOOS, p.Name(), size.PixelWidth/size.Width, frames)
		if frames == 1 {
			// 普通帧之后插入离屏帧，下一普通帧仍能更新相同资源。
			if failure = checkPixels(p); failure != nil {
				loop.Quit()
				return
			}
		}
		if *auto {
			if frames >= 2 {
				loop.Post(loop.Quit)
			} else {
				loop.Post(func() { failure = win.RequestPaint() })
			}
		}
	}
	win, err = plat.NewWindow(geometry.Size{Width: 420, Height: 170}, func(event platform.Event) {
		switch e := event.(type) {
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
			if e.Key == events.KeySpace {
				_ = win.RequestPaint()
			}
		case events.PointerEvent:
			if e.EventType == events.PointerDown && win != nil {
				_ = win.RequestPaint()
			}
		}
	}, platform.WindowOptions{})
	if err != nil {
		return err
	}
	defer win.Destroy()
	_ = win.SetTitle("GOUI Image.Update")
	p, err = plat.NewPainter(win)
	if err != nil {
		return err
	}
	defer p.Destroy()
	a, err = p.NewImage(solid(color.NRGBA{}))
	if err != nil {
		return err
	}
	defer a.Destroy()
	b, err = p.NewImage(solid(color.NRGBA{}))
	if err != nil {
		return err
	}
	defer b.Destroy()
	if err = win.Show(); err != nil {
		return err
	}
	loop.Run()
	if failure != nil {
		return failure
	}
	if frames < 2 {
		return fmt.Errorf("only %d native frames submitted, want at least 2", frames)
	}
	fmt.Println("IMAGE UPDATE PASS")
	return nil
}

func solid(ink color.NRGBA) *image.NRGBA {
	// 非零原点及带父图 stride 的图片，覆盖上传快照和格式转换。
	parent := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	src := parent.SubImage(image.Rect(2, 3, 4, 5)).(*image.NRGBA)
	for y := 3; y < 5; y++ {
		for x := 2; x < 4; x++ {
			src.SetNRGBA(x, y, ink)
		}
	}
	return src
}

func rejected(img graphics.Image, src image.Image) (err error) {
	defer func() {
		if value := recover(); value != nil {
			if !strings.Contains(fmt.Sprint(value), "update image after DrawImage") {
				err = fmt.Errorf("unexpected panic: %v", value)
			}
		} else {
			err = fmt.Errorf("update after DrawImage did not panic")
		}
	}()
	_ = img.Update(src)
	return nil
}

func checkPixels(p graphics.Painter) error {
	a, err := p.NewImage(solid(color.NRGBA{}))
	if err != nil {
		return err
	}
	defer a.Destroy()
	b, err := p.NewImage(solid(color.NRGBA{}))
	if err != nil {
		return err
	}
	defer b.Destroy()
	for _, scale := range []float32{1, 1.5, 2} {
		for frame := 0; frame < 2; frame++ {
			var failure error
			result, err := p.RenderImage(48, 32, scale, func() {
				p.Clear(graphics.Color{})
				p.DrawImage(graphics.Rect(0, 0, 0, 16), a)
				if err := a.Update(image.NewRGBA(image.Rect(0, 0, 3, 2))); err == nil {
					failure = fmt.Errorf("resized update accepted")
					return
				}
				if err := a.Update(nil); err == nil {
					failure = fmt.Errorf("nil update accepted")
					return
				}
				if failure = a.Update(solid(color.NRGBA{R: 255, A: 255})); failure != nil {
					return
				}
				src := solid(color.NRGBA{B: 255, A: 128})
				if failure = a.Update(src); failure != nil {
					return
				}
				for i := range src.Pix {
					src.Pix[i] = 0
				}
				p.SetClipRect(graphics.Rect(2, 2, 10, 10))
				p.DrawImage(graphics.Rect(0, 0, 16, 16), a)
				if failure = rejected(a, solid(color.NRGBA{R: 255, A: 255})); failure != nil {
					return
				}
				if failure = b.Update(solid(color.NRGBA{G: 180, A: 255})); failure != nil {
					return
				}
				p.SetClipRect(graphics.Rectangle{})
				p.SetTransform(geometry.Translate(16, 0))
				p.DrawImage(graphics.Rect(0, 0, 8, 8), b)
			})
			if err != nil {
				return err
			}
			if failure != nil {
				return failure
			}
			// 采样均远离边缘；预乘 8 位通道只允许格式转换/合成的一次量化误差。
			for _, sample := range []struct {
				x, y float32
				want color.RGBA
			}{
				{6, 6, color.RGBA{B: 128, A: 128}}, {1, 6, color.RGBA{}}, {14, 6, color.RGBA{}},
				{20, 4, color.RGBA{G: 180, A: 255}},
			} {
				r, g, b, a := result.At(int(sample.x*scale), int(sample.y*scale)).RGBA()
				got := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
				for i, v := range [4]uint8{got.R, got.G, got.B, got.A} {
					want := [4]uint8{sample.want.R, sample.want.G, sample.want.B, sample.want.A}[i]
					if abs(int(v)-int(want)) > 1 {
						return fmt.Errorf("scale=%g frame=%d at(%g,%g)=%v want=%v", scale, frame, sample.x, sample.y, got, sample.want)
					}
				}
			}
			if err := a.Update(solid(color.NRGBA{})); err != nil {
				return err
			}
		}
	}
	fmt.Printf("PIXELS PASS painter=%s scales=1,1.5,2\n", p.Name())
	return nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
