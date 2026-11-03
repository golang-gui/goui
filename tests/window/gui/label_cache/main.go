// Label 排版与文字图片复用回归。
//
// 环境：桌面会话／原生事件循环。Linux 需独立 X11 会话及窗口管理器，独占测试桌面。
// 启动：GOUI_PLAT_PAINTER=opengl GOUI_PLAT_SCALE=1 go run ./tests/window/gui/label_cache
// 另验 software、倍率 2；Windows 可另验 direct2d。默认自动断言后退出，-hold 保留窗口。
// 初始：普通与格式化文字各两份；每组上方是被测 Label，下方是独立参考 Label。
// 自动步骤与预期：
//  1. 预热后重复请求布局，排版创建／尺寸修改／文字栅格化／图片创建计数均不增加。
//  2. 上方 Label 在 Paint 前查询四种不同约束。显示像素须与未查询的参考精确一致；
//     查询预热后重复绘制不得产生排版、位图或图片重建。
//  3. 重复替换文字属性等价的样式表，计数不增加。改变前景色时复用 TextLayout，
//     文字像素必须改变且仍等于参考；格式化范围颜色必须正确保留。
//  4. 增大字体时必须重建并显示新像素；切换换行／对齐后两组仍与参考一致。
//
// 像素：比较同后端、同倍率、预乘 RGBA 的离屏结果，不使用跨平台字体金图。
// 参考 Label 独立排版；比较前重建参考，防止双方沿用同一份错误缓存。
// 计数范围在截图调用之间划分；Direct2D 原生文字可能不产生文字位图计数。
// -hold 下可拖动窗口边缘，检查换行与裁剪，悬停文字不应闪烁；关闭窗口退出。
// 副作用：仅创建本程序窗口，不操作剪贴板／网络／系统设置，不写文件。重启复位。
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/diagnostics"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

type queryLabel struct {
	*gui.Label
	queries bool
}

func (w *queryLabel) Paint(p gui.Painter) {
	if w.queries {
		w.Measure(layout.Loose(geometry.Size{Width: 130, Height: 300}))
		w.Measure(layout.Loose(geometry.Size{Width: 270, Height: 200}))
		w.Measure(layout.Loose(geometry.Size{Width: 270, Height: 82}))
		w.Measure(layout.Unbounded())
	}
	w.Label.Paint(p)
}

type frameProbe struct {
	gui.WidgetBase
	paints int
	scale  float32
	image  graphics.Image
}

func (w *frameProbe) Paint(p gui.Painter) {
	w.paints++
	w.scale = p.PixelScale()
	if w.image == nil {
		var err error
		w.image, err = p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		must(err)
		name := strings.ToLower(fmt.Sprintf("%T", w.image))
		requested := os.Getenv("GOUI_PLAT_PAINTER")
		check(requested == "" || strings.Contains(name, requested+"."), "unexpected backend "+name)
		fmt.Printf("BACKEND %s scale=%g\n", name, w.scale)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func check(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func sheet(size float32, ink color.Color, decoration float32) style.StyleSheet {
	rules := gui.DefaultStyleRules()
	rules = append(rules, style.Name("label").FontSize(size).ForegroundColor(ink).Radius(decoration),
		style.Name("accent").ForegroundColor(color.RGBA{R: 220, G: 80, B: 50, A: 255}),
		style.Name("bold").FontWeight(style.FontWeightBold))
	return style.Sheet(rules...)
}
func rgba(w gui.Widget, scale float32) []byte {
	img, err := gui.RenderWidget(w, scale)
	must(err)
	return append([]byte(nil), graphics.CopyToBitmap(img, graphics.PixelFormatRGBA, nil).Pixels...)
}
func delta(a, b diagnostics.Resources) diagnostics.Resources {
	return diagnostics.Resources{
		LayoutsCreated: b.LayoutsCreated - a.LayoutsCreated, LayoutsDestroyed: b.LayoutsDestroyed - a.LayoutsDestroyed,
		LayoutsResized: b.LayoutsResized - a.LayoutsResized, TextRasterizations: b.TextRasterizations - a.TextRasterizations,
		ImagesCreated: b.ImagesCreated - a.ImagesCreated, ImagesDestroyed: b.ImagesDestroyed - a.ImagesDestroyed,
		TextCacheHits: b.TextCacheHits - a.TextCacheHits, TextCacheInvalidations: b.TextCacheInvalidations - a.TextCacheInvalidations,
	}
}
func stable(d diagnostics.Resources) bool {
	return d.LayoutsCreated == 0 && d.LayoutsDestroyed == 0 && d.LayoutsResized == 0 && d.TextRasterizations == 0 && d.ImagesCreated == 0 && d.ImagesDestroyed == 0 && d.TextCacheInvalidations == 0
}

func main() {
	hold := flag.Bool("hold", false, "自动断言后保留窗口供观察")
	flag.Parse()
	app, err := gui.NewApplication("org.golanggui.LabelCache")
	must(err)
	app.SetStyleSheet(sheet(12, color.Black, 0))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 580, Height: 420}, Center: true})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("GOUI Label cache regression"))
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(geometry.UniformInsets(16))
	root.SetSpacing(8)
	texts := []string{"Stable text / 中文 / é / אבג", `\{format\}Plain \{accent:color \{bold:bold\}\} words 中文 long line with wrapping`}
	var queries []*queryLabel
	var references []*gui.Label
	for _, text := range texts {
		q := &queryLabel{Label: gui.NewLabel(text)}
		r := gui.NewLabel(text)
		for _, l := range []*gui.Label{q.Label, r} {
			l.SetMinSize(geometry.Size{Width: 420, Height: 82})
			l.SetMaxSize(geometry.Size{Width: 420, Height: 82})
			l.SetWrapMode(gui.WrapWordChar)
		}
		root.AddChild(q)
		root.AddChild(r)
		queries, references = append(queries, q), append(references, r)
	}
	probe := &frameProbe{}
	probe.SetMinSize(geometry.Size{Width: 1, Height: 1})
	probe.ConnectUnmount(func() {
		if probe.image != nil {
			probe.image.Destroy()
			probe.image = nil
		}
	})
	root.AddChild(probe)
	win.SetWidget(root)
	must(win.Show())
	compare := func() []byte {
		var all []byte
		for i, q := range queries {
			r := references[i]
			// 独立重建参考，不允许双方复用同一份错误状态。
			text := r.Text()
			r.SetText("reference reset")
			r.SetText(text)
			a, b := rgba(q, probe.scale), rgba(r, probe.scale)
			check(bytes.Equal(a, b), fmt.Sprintf("reference pixels differ for label %d", i))
			all = append(all, a...)
		}
		visible := false
		for i := 3; i < len(all); i += 4 {
			visible = visible || all[i] != 0
		}
		check(visible, "empty text pixels")
		return all
	}
	var capture *diagnostics.Capture
	var before diagnostics.Resources
	var pixels []byte
	phase, loops, ticks, pending := 0, 0, 0, 0
	request := func() { pending = probe.paints + 1; must(win.RequestPaint()) }
	report := func(name string) diagnostics.Resources {
		after := capture.Snapshot().Resources
		d := delta(before, after)
		data, err := json.Marshal(d)
		must(err)
		fmt.Printf("PHASE %s %s\n", name, data)
		before = after
		return d
	}
	timer := app.NewTimer()
	done := false
	timer.ConnectTimeout(func() {
		ticks++
		check(ticks < 400, "frame watchdog expired")
		if probe.paints == 0 || probe.paints < pending {
			return
		}
		switch phase {
		case 0:
			if ticks < 6 {
				return
			}
			pixels = compare()
			// 参考重建请求了布局，先完成它再开始计数。
			phase = 1
			request()
		case 1:
			capture, err = diagnostics.Start(app, diagnostics.Options{Widgets: true})
			must(err)
			before = capture.Snapshot().Resources
			phase = 2
			root.RequestLayout()
			request()
		case 2:
			loops++
			if loops < 10 {
				root.RequestLayout()
				request()
				return
			}
			check(stable(report("same_layout_10")), "repeated layout rebuilt text resources")
			for _, q := range queries {
				q.queries = true
			}
			loops, phase = 0, 3
			request()
		case 3:
			before = capture.Snapshot().Resources
			phase = 4
			request()
		case 4:
			loops++
			if loops < 10 {
				request()
				return
			}
			check(stable(report("query_and_paint_10")), "measurement queries invalidated rendered text")
			check(bytes.Equal(pixels, compare()), "measurement queries changed pixels")
			for _, q := range queries {
				q.queries = false
			}
			// 切换回普通约束，按下一阶段相同的样式失效路径预热。
			phase = 5
			app.SetStyleSheet(sheet(12, color.Black, 0))
			request()
		case 5:
			before = capture.Snapshot().Resources
			loops, phase = 0, 6
			app.SetStyleSheet(sheet(12, color.Black, 1))
			request()
		case 6:
			loops++
			if loops < 10 {
				app.SetStyleSheet(sheet(12, color.Black, float32(loops%2)))
				request()
				return
			}
			check(stable(report("equivalent_style_10")), "equivalent style rebuilt text resources")
			phase = 7
			app.SetStyleSheet(sheet(12, color.RGBA{R: 30, G: 90, B: 200, A: 255}, 0))
			request()
		case 7:
			d := report("color")
			check(d.LayoutsCreated == 0 && d.LayoutsDestroyed == 0, "color change replaced text layout")
			next := compare()
			check(!bytes.Equal(pixels, next), "new foreground did not change pixels")
			pixels = next
			before = capture.Snapshot().Resources
			phase = 8
			app.SetStyleSheet(sheet(16, color.RGBA{R: 30, G: 90, B: 200, A: 255}, 0))
			request()
		case 8:
			d := report("font")
			check(d.LayoutsCreated > 0 && d.LayoutsDestroyed > 0, "font change did not replace text layout")
			check(!bytes.Equal(pixels, compare()), "font change left old pixels")
			for i, q := range queries {
				q.SetWrapMode(gui.WrapChar)
				q.SetTextAlign(gui.TextAlignCenter)
				references[i].SetWrapMode(gui.WrapChar)
				references[i].SetTextAlign(gui.TextAlignCenter)
			}
			phase = 9
			request()
		case 9:
			compare()
			capture.Stop()
			timer.Stop()
			done = true
			fmt.Println("PASS layout/style/query/color/font/wrap/reference pixels")
			if !*hold {
				win.Destroy()
			}
		}
	})
	must(timer.Start(50 * time.Millisecond))
	app.Run()
	if !done {
		log.Fatal("window closed before verification completed")
	}
}
