//go:build android || ios

// 移动平台 Typography/Graphics 最小验收，不导入 gui/ui，也不使用 DevServer。
// 环境、维护者构建、操作及逐项预期见 README.md。自动检查先于窗口显示；错误 panic。
// 初始：多语言文本、图形/半透明合成/裁剪/变换和可复用图片；原生窗口出现后重绘。
// 旋转、后台往返应继续显示同一布局和图片；不将截图正常等同于真实触摸/IME 通过。
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"time"

	ffi "github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"golang.org/x/image/font/gofont/goregular"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func symbol(name string) uintptr { p, err := ffi.Dlsym(ffi.RTLD_DEFAULT, name); must(err); return p }

type check struct {
	Name   string
	Passed bool
}

var checks []check

func expect(name string, good bool) {
	checks = append(checks, check{name, good})
	if !good {
		panic("验证失败：" + name)
	}
}

func main() {
	directory := ffi.GoString(ffi.Pointer(ffi.CallRet[uintptr](symbol("probe_data_dir"))))
	thread := ffi.CallRet[uintptr](symbol("probe_thread"))
	expect("GOUI 不占用原生 UI 主线程", ffi.CallRet[int32](symbol("probe_is_main")) == 0)
	must(os.MkdirAll(directory, 0700))
	write := func(name string, value any) {
		data, err := json.MarshalIndent(value, "", "  ")
		must(err)
		p := filepath.Join(directory, name)
		must(os.WriteFile(p+".tmp", data, 0600))
		must(os.Rename(p+".tmp", p))
	}
	backend, err := platform.NewPlatform(platform.DefaultName(), "org.golanggui.mobilepaint")
	must(err)
	defer backend.Destroy()
	expect("默认工厂使用实际移动平台", platform.DefaultName() == runtime.GOOS && backend.Name() == runtime.GOOS)
	same, err := platform.NewPlatform(platform.DefaultName(), "org.golanggui.mobilepaint")
	must(err)
	expect("平台保持进程单例", same == backend)
	other, err := platform.NewPlatform(platform.DefaultName(), "org.golanggui.other")
	expect("不同应用标识不能重用平台单例", other == nil && err != nil)
	_, err = backend.NewWindow(geometry.Size{Width: 320, Height: 480}, nil, common.WindowOptions{})
	expect("逻辑窗口要求先创建事件队列", errors.Is(err, common.ErrUnavailable))
	context, err := newTypography(backend)
	must(err)
	defer context.Destroy()
	font := filepath.Join(directory, "Go-Regular.ttf")
	must(os.WriteFile(font, goregular.TTF, 0600))
	must(context.AddFont(font))
	verifyTypography(context)
	write("checks.json", checks)
	loop, err := backend.NewEventLoop()
	must(err)
	extra, err := backend.NewEventLoop()
	expect("平台不创建第二套事件队列", extra == nil && errors.Is(err, common.ErrUnsupported))
	defer loop.Destroy()
	var size events.SizeEvent
	var painter graphics.Painter
	var cached graphics.Image
	var window common.Window
	frames := 0
	visible := false
	nonce := time.Now().UnixNano()
	gpu := ffi.CallRet[int32](symbol("probe_use_gles")) != 0
	painterName := "software"
	if gpu {
		painterName = "opengl"
	}
	// 测试宿主分别编译两种模式；通过正常环境选项请求对应后端。
	must(os.Setenv("GOUI_PLAT_PAINTER", painterName))
	graphicsChecked := false
	start := time.Now()
	report := func() {
		if window != nil {
			backendName := ""
			if painter != nil {
				backendName = painter.Name()
			}
			write("paint.json", map[string]any{"platform": backend.Name(), "nonce": nonce, "visible": visible, "frames": frames, "width": size.Width, "height": size.Height, "scale": size.PixelWidth / max(size.Width, 1), "surface": window.SurfaceState(), "elapsed": time.Since(start).Seconds(), "checks": len(checks), "typography": context.Name(), "painter": backendName})
		}
	}
	layout, err := context.NewTextLayout("GOUI mobile typography / graphics\n中文 日本語 한국어 العربية हिन्दी 👩‍💻\nCore layout + Go Painter", typography.TextFormat{Font: typography.FontInfo{Size: 18}, TextColor: color.NRGBA{R: 24, G: 48, B: 72, A: 255}, WrapMode: typography.WrapWordChar}, 320, 240)
	must(err)
	defer layout.Destroy()
	window, err = backend.NewWindow(geometry.Size{Width: 320, Height: 480}, func(event events.Event) {
		if ffi.CallRet[uintptr](symbol("probe_thread")) != thread {
			panic("窗口回调迁移了 OS 线程")
		}
		switch e := event.(type) {
		case events.VisibilityEvent:
			visible = e.Visible
			report()
		case events.SurfaceEvent:
			report()
		case events.SizeEvent:
			size = e
			layout.SetSize(max(1, e.Width-32), 240)
		case events.PaintEvent:
			if painter == nil {
				painter, err = backend.NewPainter(window)
				must(err)
			}
			if !graphicsChecked {
				verifyGraphics(painter, context, gpu)
				graphicsChecked = true
				write("checks.json", checks)
			}
			if cached == nil {
				pixels := image.NewRGBA(image.Rect(0, 0, 16, 16))
				for y := 0; y < 16; y++ {
					for x := 0; x < 16; x++ {
						pixels.SetRGBA(x, y, color.RGBA{R: 48, G: 128, B: 224, A: 255})
					}
				}
				cached, err = painter.NewImage(pixels)
				must(err)
			}
			painter.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
			painter.Clear(graphics.RGB(244, 247, 250))
			painter.DrawTextLayout(graphics.Point{X: 16, Y: 20}, layout)
			painter.DrawBoxShadow(graphics.Rect(16, 190, 120, 64), geometry.UniformCornerRadius(12), graphics.BoxShadow{Color: graphics.ColorOf(color.NRGBA{A: 80}), Offset: graphics.Point{Y: 3}, BlurRadius: 6})
			painter.FillRoundRect(graphics.Rect(16, 190, 120, 64), 12, graphics.RGB(32, 144, 96))
			painter.DrawImage(graphics.Rect(152, 190, 64, 64), cached)
			painter.SetClipRect(graphics.Rect(232, 190, 60, 64))
			painter.FillRect(graphics.Rect(220, 180, 100, 100), graphics.RGB(224, 96, 48))
			painter.SetClipRect(graphics.Rectangle{})
			painter.End()
			frames++
			report()
		}
	}, common.WindowOptions{})
	must(err)
	expect("逻辑窗口创建不等待绘制目标", !window.SurfaceState().Ready)
	_, desktop := window.(common.WindowManagement)
	expect("移动窗口不伪造桌面能力", !desktop)
	second, err := backend.NewWindow(geometry.Size{Width: 320, Height: 480}, nil, common.WindowOptions{})
	expect("不创建第二个原生内容窗口", second == nil && errors.Is(err, common.ErrUnsupported))
	must(os.Setenv("GOUI_PLAT_PAINTER", "unknown"))
	rejected, err := backend.NewPainter(window)
	expect("不支持的绘制后端不静默回退", rejected == nil && errors.Is(err, common.ErrUnsupported))
	must(os.Setenv("GOUI_PLAT_PAINTER", painterName))
	if gpu && runtime.GOOS == "ios" {
		early, err := backend.NewPainter(window)
		expect("iOS GPU 未就绪时立即报告不可用", early == nil && errors.Is(err, common.ErrUnavailable))
	}
	if !gpu {
		// 回归普通 GUI 在 Run/Scene 就绪前构造 Painter 的顺序。
		painter, err = backend.NewPainter(window)
		must(err)
		expect("Software 在原生目标就绪前构造", painter != nil && !window.SurfaceState().Ready)
	}
	defer window.Destroy()
	defer func() {
		if painter != nil {
			painter.Destroy()
		}
	}()
	// 仅供旧双线程原型验收：内部内容显示钩子不是 WindowManagement。
	// 正式系统宿主交付与 GUI Surface 接入另行实现，不扩大基础 Window 的契约。
	must(window.(interface{ Show() error }).Show())
	// 测试持续请求绘制，以覆盖旋转撤销与后台停绘；不是生产帧调度实现。
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-ticker.C:
				loop.Post(func() { must(window.RequestPaint()) })
			case <-stop:
				return
			}
		}
	}()
	loop.Run()
}

func verifyTypography(ctx typography.Context) {
	format := typography.TextFormat{Font: typography.FontInfo{Family: "Go", Size: 24}, TextColor: color.NRGBA{R: 160, G: 64, B: 32, A: 128}, WrapMode: typography.WrapWordChar}
	text := "Hello ffi a\u0301\tworld\nsecond line"
	l, err := ctx.NewTextLayout(text, format, 240, 200)
	must(err)
	defer l.Destroy()
	w, h := l.MeasureSize()
	expect("排版有正尺寸", w > 0 && h > 0)
	lines, clusters := l.MeasureMetrics()
	expect("至少两行", len(lines) >= 2)
	expect("交互单元非空", len(clusters) > 0)
	for _, line := range lines {
		expect("行范围和基线有效", line.Start >= 0 && line.Start+line.Length <= len(text) && line.Height > 0 && line.Baseline > 0)
	}
	for _, c := range clusters {
		expect("Cluster 范围和坐标有效", c.Start >= 0 && c.Length > 0 && c.Start+c.Length <= len(text) && !math.IsNaN(float64(c.Width)))
	}
	for _, scale := range []float32{1, 2, 3} {
		region := image.Rect(-8, -6, 240*int(scale), 160*int(scale))
		full, err := l.Rasterize(scale, region, nil)
		must(err)
		expect(fmt.Sprintf("%gx 栅格尺寸", scale), full.Width == region.Dx() && full.Height == region.Dy() && full.Stride == full.Width*4)
		ink := false
		for i := 0; i < len(full.Pixels); i += 4 {
			r, g, b, a := full.Pixels[i], full.Pixels[i+1], full.Pixels[i+2], full.Pixels[i+3]
			if a > 0 {
				ink = true
			}
			if r > a || g > a || b > a {
				panic("非预乘文字像素")
			}
		}
		expect(fmt.Sprintf("%gx 有文字且预乘", scale), ink)
		sub := image.Rect(12, 8, 130, 75)
		small, err := l.Rasterize(scale, sub, nil)
		must(err)
		for y := 0; y < small.Height; y++ {
			start := (sub.Min.Y-region.Min.Y+y)*full.Stride + (sub.Min.X-region.Min.X)*4
			expect("区域栅格与完整裁切相等", bytes.Equal(small.Pixels[y*small.Stride:(y+1)*small.Stride], full.Pixels[start:start+small.Stride]))
		}
		buf := make([]byte, len(full.Pixels))
		again, err := l.Rasterize(scale, region, buf)
		must(err)
		expect("输出缓冲可复用", &again.Pixels[0] == &buf[0] && bytes.Equal(again.Pixels, full.Pixels))
	}
	peer, err := ctx.NewTextLayout(text, format, 240, 200)
	must(err)
	defer peer.Destroy()
	region := image.Rect(0, 0, 240, 160)
	before, err := peer.Rasterize(1, region, nil)
	must(err)
	l.SetTextColor(0, 5, color.NRGBA{G: 160, A: 255})
	l.SetUnderline(0, 5, true)
	l.SetStrikethrough(6, 3, true)
	after, err := l.Rasterize(1, region, nil)
	must(err)
	expect("范围样式改变画面", !bytes.Equal(before.Pixels, after.Pixels))
	untouched, err := peer.Rasterize(1, region, nil)
	must(err)
	expect("不同布局状态隔离", bytes.Equal(before.Pixels, untouched.Pixels))
	l.SetSize(90, 400)
	narrow, _ := l.MeasureMetrics()
	expect("窄宽自动换行", len(narrow) > len(lines))
	placeholder, err := ctx.NewTextLayout("A\ufffcB", format, 240, 100)
	must(err)
	defer placeholder.Destroy()
	placeholder.SetPlaceholder(1, &typography.PlaceholderMetrics{Width: 37, Height: 28, Baseline: 20})
	_, items := placeholder.MeasureMetrics()
	found := false
	for _, c := range items {
		if c.Start == 1 {
			found = true
			expect("占位宽度", math.Abs(float64(c.Width-37)) < .1)
		}
	}
	expect("占位单元存在", found)
}
