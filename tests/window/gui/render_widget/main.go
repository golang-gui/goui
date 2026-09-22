// RenderWidget 窗口验证；不使用 DevServer。
//
// 环境：Linux/X11、Windows 或 macOS 桌面，GUI 主线程。
// 启动：GOUI_PLAT_PAINTER=opengl go run ./tests/window/gui/render_widget
// 后端可选 software / opengl，Windows 另可选 direct2d；GOUI_PLAT_SCALE=1/2。
// -check：首次窗口绘制后自动执行断言，验证后续重绘，再关闭窗口退出。
// 指定后端时会根据实际创建的图片资源检查后端，静默 fallback 不计为通过。
//
// 初始：源控件包含文字、蓝色图片、图标、圆角背景、半透明红色块；下方显示导出图。
// 操作与预期：点击 Capture 交替以 1x/2x 导出，图片物理尺寸相应翻倍；
// 源控件不移动、不消失，文字/图标完整，透明角落透出窗口背景。
// 连续点击并缩放窗口，原图及导出图继续正常显示，不串色、不 panic。
// 自动断言检查内部 RGBA 样本、透明角、尺寸、嵌套拒绝、Paint panic 后恢复及资源复用。
// 文字边缘允许平台抗锯齿差异，不做跨平台金图比较。
// 副作用：仅创建一个窗口，不写文件。重启复位；关闭窗口退出。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/icon"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
)

type sample struct {
	gui.WidgetBase
	label      *gui.Label
	picture    *gui.Image
	icon       *gui.Icon
	pixels     image.Image
	native     graphics.Image
	paints     int
	panicPaint bool
	nested     bool
}

func newSample() *sample {
	pixels := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for i := 0; i < len(pixels.Pix); i += 4 {
		pixels.Pix[i+2], pixels.Pix[i+3] = 255, 255
	}
	source, err := icon.NewImage(pixels)
	if err != nil {
		panic(err)
	}
	s := &sample{label: gui.NewLabel("Text / image / icon"), picture: gui.NewImage(pixels), icon: gui.NewIcon(source), pixels: pixels}
	s.SetMinSize(geometry.Size{Width: 200, Height: 100})
	s.SetMaxSize(geometry.Size{Width: 200, Height: 100})
	s.WidgetBase.AddChild(s, s.label)
	s.WidgetBase.AddChild(s, s.picture)
	s.WidgetBase.AddChild(s, s.icon)
	s.ConnectUnmount(func() {
		if s.native != nil {
			s.native.Destroy()
			s.native = nil
		}
	})
	return s
}

func (s *sample) Arrange(r geometry.Rectangle) {
	s.WidgetBase.Arrange(r)
	s.label.Arrange(geometry.Rect(16, 32, 180, 30))
	s.picture.Arrange(geometry.Rect(16, 70, 16, 16))
	s.icon.Arrange(geometry.Rect(50, 70, 20, 20))
}

func (s *sample) Paint(p gui.Painter) {
	s.paints++
	if s.native == nil {
		var err error
		s.native, err = p.NewImage(s.pixels)
		if err != nil {
			panic(err)
		}
		actual := strings.ToLower(fmt.Sprintf("%T", s.native))
		requested := os.Getenv("GOUI_PLAT_PAINTER")
		if requested != "" && !strings.Contains(actual, requested+".") {
			panic("unexpected backend: " + actual)
		}
		log.Printf("actual image backend=%s", actual)
	}
	if s.nested {
		if _, err := gui.RenderWidget(s, 1); err == nil {
			panic("nested rendering accepted")
		}
	}
	if s.panicPaint {
		panic("intentional paint panic")
	}
	p.FillRoundRect(geometry.Rect(0, 25, 200, 75), 10, graphics.RGBA(100, 160, 200, 128))
	p.FillRect(geometry.Rect(4, 4, 12, 12), graphics.RGBA(255, 0, 0, 128))
	p.DrawImage(geometry.Rect(90, 70, 16, 16), s.native)
}

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	check := flag.Bool("check", false, "run assertions and close the window")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.RenderWidget")
	if err != nil {
		log.Fatal(err)
	}
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 520, Height: 430}})
	if err != nil {
		log.Fatal(err)
	}
	defer win.Destroy()
	if err := win.SetTitle("GOUI RenderWidget"); err != nil {
		panic(err)
	}
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(16)
	root.SetSpacing(12)
	source := newSample()
	preview := gui.NewImage(nil)
	button := gui.NewButton()
	button.SetChild(gui.NewLabel("Capture 1x / 2x"))
	root.AddChild(source)
	root.AddChild(button)
	root.AddChild(preview)
	win.SetWidget(root)
	scale := float32(1)
	capture := func() {
		native := source.native
		source.nested = true
		img, err := gui.RenderWidget(source, scale)
		source.nested = false
		if err != nil {
			panic(err)
		}
		if source.native != native {
			panic("cached resource was replaced")
		}
		if img.Bounds() != image.Rect(0, 0, int(200*scale), int(100*scale)) {
			panic("wrong output size")
		}
		for _, tc := range []struct {
			x, y int
			want color.RGBA
		}{
			{0, 0, color.RGBA{}}, {8, 8, color.RGBA{R: 128, A: 128}}, {95, 75, color.RGBA{B: 255, A: 255}},
		} {
			got := color.RGBAModel.Convert(img.At(tc.x*int(scale), tc.y*int(scale))).(color.RGBA)
			if got != tc.want {
				panic(fmt.Sprintf("sample %d,%d got %v want %v", tc.x, tc.y, got, tc.want))
			}
		}
		preview.SetImage(img)
		log.Printf("RenderWidget %gx passed; output=%v", scale, img.Bounds())
		scale = 3 - scale
	}
	button.ConnectClicked(capture)
	if err := win.Show(); err != nil {
		panic(err)
	}
	timer := app.NewTimer()
	step, before := 0, 0
	timer.ConnectTimeout(func() {
		if source.paints == 0 {
			return
		}
		switch step {
		case 0:
			source.panicPaint = true
			func() {
				defer func() {
					if recover() != "intentional paint panic" {
						panic("wrong paint panic")
					}
				}()
				_, _ = gui.RenderWidget(source, 1)
			}()
			source.panicPaint = false
			capture()
		case 1:
			capture()
			before = source.paints
			if err := win.RequestPaint(); err != nil {
				panic(err)
			}
		case 2:
			if source.paints <= before {
				panic("window did not repaint after export")
			}
			log.Print("PASS: export, panic cleanup, cache reuse and subsequent window painting")
			timer.Stop()
			if *check {
				app.Quit()
			}
		}
		step++
	})
	if err := timer.Start(500 * time.Millisecond); err != nil {
		panic(err)
	}
	app.Run()
}
