// 字形范围的真实绘图后端验证。
//
// 环境：桌面连接及 GlyphContext（当前 Linux/Pango），DejaVu Sans。
// 启动：GOUI_PLAT_PAINTER=opengl go run ./tests/window/graphics/glyph_ranges -painter OpenGL
// 可改为 GOUI_PLAT_PAINTER=software ... -painter Software。
// 初始：创建不显示的原生窗口和真正的绘图上下文；仍依赖桌面，不能作为包测试。
// 自动操作：在 1/1.25/1.5/2 倍分别离屏绘制完整文字及逐 cluster 绘制的文字。
// 预期：全部像素的预乘 RGBA 各分量误差至多 1/255（GPU 纹理采样／8-bit 转换），
// 包含斜体外伸、半透明墨迹和透明背景；不放宽字体／布局位置误差。
// 输出实际 Painter 名称及各缩放最大误差；任一断言失败返回非零。
// 平台差异：无 GlyphContext 的后端报能力缺失，不当成通过。-painter 防止静默回退。
// 无文件写入、剪贴板或桌面输入；结束释放窗口／上下文并自动退出。复位：重启。
package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/typography"
)

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	name := flag.String("painter", "", "required actual painter name")
	flag.Parse()
	if err := run(*name); err != nil {
		log.Fatal(err)
	}
}

func run(want string) error {
	plat, err := platform.NewPlatform(platform.DefaultName(), "org.golanggui.GlyphRanges")
	if err != nil {
		return err
	}
	defer plat.Destroy()
	loop, err := plat.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	win, err := plat.NewWindow(geometry.Size{Width: 1200, Height: 180}, func(platform.Event) {}, platform.WindowOptions{Chrome: platform.WindowChromeNative})
	if err != nil {
		return err
	}
	defer win.Destroy()
	p, err := plat.NewPainter(win)
	if err != nil {
		return err
	}
	defer p.Destroy()
	if want != "" && want != p.Name() {
		return fmt.Errorf("painter=%s, required %s", p.Name(), want)
	}
	ctx, err := plat.NewTypography()
	if err != nil {
		return err
	}
	defer ctx.Destroy()
	glyphCtx, ok := ctx.(typography.GlyphContext)
	if !ok {
		return fmt.Errorf("%s does not supply GlyphContext", ctx.Name())
	}
	l, err := glyphCtx.NewGlyphLayout("f    f    f    e\u0301    ffi    ", typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans", Size: 37, Style: typography.FontStyleItalic}, TextColor: color.NRGBA{R: 170, B: 90, A: 180}})
	if err != nil {
		return err
	}
	defer l.Destroy()
	m, err := l.GlyphMetrics()
	if err != nil {
		return err
	}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		a, err := p.RenderImage(1200, 180, scale, func() { p.DrawTextLayout(geometry.Point{X: 20, Y: 20}, l) })
		if err != nil {
			return err
		}
		b, err := p.RenderImage(1200, 180, scale, func() {
			for _, i := range m.VisualOrder {
				c := m.Clusters[i]
				p.DrawTextGlyphs(geometry.Point{X: 20 + c.X, Y: 20}, l, c.Start, c.Length, nil)
			}
		})
		if err != nil {
			return err
		}
		maxError := 0
		for y := 0; y < 180; y++ {
			for x := 0; x < 1200; x++ {
				ca, cb := color.RGBAModel.Convert(a.At(x, y)).(color.RGBA), color.RGBAModel.Convert(b.At(x, y)).(color.RGBA)
				for _, d := range []int{int(ca.R) - int(cb.R), int(ca.G) - int(cb.G), int(ca.B) - int(cb.B), int(ca.A) - int(cb.A)} {
					if d < 0 {
						d = -d
					}
					maxError = max(maxError, d)
					if d > 1 {
						return fmt.Errorf("%s scale %v pixel(%d,%d): whole=%v range=%v", p.Name(), scale, x, y, ca, cb)
					}
				}
			}
		}
		fmt.Printf("PASS painter=%s typography=%s scale=%v max_byte_error=%d\n", p.Name(), ctx.Name(), scale, maxError)
	}
	return nil
}
