//go:build android

// Android 排版隔离与 EGL 失败回滚验收，操作说明见 README.md。
// 使用正式平台宿主和 OpenGL ES Painter，不依赖私有状态或测试原生代码。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func expect(pass bool, format string, args ...any) {
	if !pass {
		panic(fmt.Sprintf(format, args...))
	}
}

func main() {
	must(os.Setenv("GOUI_PLAT_PAINTER", "opengl"))
	p, err := platform.NewPlatform(platform.DefaultName(), "org.golanggui.AndroidRendering")
	must(err)
	loop, err := p.NewEventLoop()
	must(err)
	source := p.(platform.WindowSource)
	var size events.SizeEvent
	var painter graphics.Painter
	var context typography.Context
	var small, large typography.TextLayout
	frames := 0
	finishing := false
	duplicateError := ""
	must(source.SetWindowHandler(func(window platform.Window) platform.EventHandler {
		return func(event platform.Event) {
			expect(syscall.Gettid() == syscall.Getpid(), "资源调用未运行在原生 UI 线程")
			switch e := event.(type) {
			case events.SizeEvent:
				size = e
			case events.PaintEvent:
				if finishing {
					return
				}
				if painter == nil {
					context, err = p.NewTypography()
					must(err)
					small, err = context.NewTextLayout("GO", typography.TextFormat{Font: typography.FontInfo{Size: 16}, TextColor: color.RGBA{R: 210, A: 255}}, 200, 100)
					must(err)
					large, err = context.NewTextLayout("GO", typography.TextFormat{Font: typography.FontInfo{Size: 28}, TextColor: color.RGBA{B: 210, A: 255}}, 200, 100)
					must(err)
					verifyLayouts(small, large)
					painter, err = p.NewPainter(window)
					must(err)
					expect(painter.Name() == "OpenGL", "后端发生降级：%s", painter.Name())
					duplicate, duplicateErr := p.NewPainter(window)
					if duplicate != nil {
						duplicate.Destroy()
					}
					expect(duplicateErr != nil, "同一 NativeWindow 意外允许两个 EGL 窗口表面")
					duplicateError = duplicateErr.Error()
					expect(strings.Contains(duplicateError, "create EGL window surface"), "未保留失败阶段：%s", duplicateError)
					verifyGPU(painter)
				}
				painter.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
				painter.Clear(graphics.RGB(244, 247, 250))
				painter.DrawTextLayout(graphics.Point{X: 20, Y: 40}, small)
				painter.DrawTextLayout(graphics.Point{X: 20, Y: 100}, large)
				painter.FillRect(graphics.Rect(20, 160, 80, 40), graphics.RGB(32, 144, 96))
				painter.End()
				frames++
				if frames < 2 {
					must(window.RequestPaint())
					return
				}
				finishing = true
				// Paint 已结束；下一批 UI 工作释放资源，避免在借用目标时同步撤销自身。
				loop.Post(func() {
					painter.Destroy()
					small.Destroy()
					large.Destroy()
					context.Destroy()
					window.Destroy()
					p.Destroy()
					data, err := json.Marshal(map[string]any{"passed": true, "frames": frames, "layoutIsolation": true, "duplicateError": duplicateError, "mainThread": syscall.Gettid() == syscall.Getpid()})
					must(err)
					must(os.WriteFile(filepath.Join(os.TempDir(), "rendering.json"), data, 0600))
					loop.Quit()
				})
			}
		}
	}))
	timer := time.AfterFunc(20*time.Second, func() { loop.Post(func() { panic("Android 绘制验收超时") }) })
	defer timer.Stop()
	loop.Run()
	loop.Destroy()
	expect(finishing && frames == 2, "退出状态错误")
}
func verifyLayouts(small, large typography.TextLayout) {
	w, h := small.MeasureSize()
	bw, bh := large.MeasureSize()
	expect(w > 0 && h > 0 && bw > w && bh > h, "两个字号的测量结果错误")
	region := image.Rect(0, 0, 160, 100)
	before, err := small.Rasterize(1, region, nil)
	must(err)
	verifyColor(before, 0)
	blue, err := large.Rasterize(1, region, nil)
	must(err)
	verifyColor(blue, 2)
	large.SetTextFont(0, 2, typography.FontInfo{Size: 32, Weight: typography.FontWeightBold})
	large.SetTextColor(0, 2, color.RGBA{G: 210, A: 255})
	green, err := large.Rasterize(1, region, nil)
	must(err)
	verifyColor(green, 1)
	after, err := small.Rasterize(1, region, nil)
	must(err)
	aw, ah := small.MeasureSize()
	expect(aw == w && ah == h && bytes.Equal(before.Pixels, after.Pixels), "修改其他 Layout 污染了测量/位图")
}
func verifyColor(bitmap typography.TextBitmap, channel int) {
	count := 0
	for y := 0; y < bitmap.Height; y++ {
		for x := 0; x < bitmap.Width; x++ {
			pixel := bitmap.Pixels[y*bitmap.Stride+x*4:][:4]
			if pixel[3] < 192 {
				continue
			}
			// RGBA 预乘像素：充分覆盖的文字像素只有目标通道，目标值为 210*alpha/255。
			want := (210*int(pixel[3]) + 127) / 255
			for c := 0; c < 3; c++ {
				expected := 0
				if c == channel {
					expected = want
				}
				delta := int(pixel[c]) - expected
				expect(delta >= -1 && delta <= 1, "像素通道错误：%v channel=%d", pixel, channel)
			}
			count++
		}
	}
	expect(count > 0, "没有可验证的文字像素")
}

func verifyGPU(painter graphics.Painter) {
	pixels, err := painter.RenderImage(16, 16, 1, func() { painter.Clear(graphics.RGB(32, 144, 96)) })
	must(err)
	r, g, b, a := pixels.At(8, 8).RGBA()
	got := []uint32{r >> 8, g >> 8, b >> 8, a >> 8}
	want := []uint32{32, 144, 96, 255}
	for i, value := range got {
		delta := int(value) - int(want[i])
		expect(delta >= -1 && delta <= 1, "失败回滚后 GPU 像素错误：%v", got)
	}
}
