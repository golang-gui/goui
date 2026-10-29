// Label 格式化文本窗口验收，不依赖 DevServer。
// 环境：Linux X11/Windows/macOS 桌面及系统字体；记录实际 Painter、排版后端和缩放。
// 启动：go run ./tests/window/gui/label [-auto] [-capture-dir DIR]。
// GOUI_PLAT_PAINTER=software/opengl（Windows 另测 direct2d），GOUI_PLAT_SCALE=1/2。
// DIR 必须是仓库外已有目录；-auto 完成 Basic/Modern Light/Dark 后输出 PASS 并退出。
// 普通启动同样先自动断言，再保留窗口供手动操作；请待 PASS 后操作。
// 初始：Integrated 窗口、Basic；原样文本、嵌套粗斜体/语义颜色、混合字号、装饰、
// UTF-8 参数、错误降级，以及仍显示原始语法的 TextInput/只读 TextView。
// 操作与预期：
//  1. 查看三种主题：大字号中的 warning/bold 不缩小，small/big 明确改变字号；
//     中文、组合字符、emoji、阿拉伯字形保持连续，无分段拼接或换行错位。
//  2. 点击 Basic/Light/Dark 切换主题，再点 Check。文字和局部颜色跟随主题，
//     AB...G 的 Label 与独立 native reference 行的字体、颜色、大小和装饰一致。
//  3. 把窗口缩窄再放宽，长 Label 正常换行，括号/空格保留；编辑框可输入原始语法，
//     只读 TextView 的模型仍包含标记；本轮没有选择 Label 或复制的能力。
//  4. 自动步骤/Check 比较同一排版后端独立构造 TextLayout 的尺寸、基线和 RGBA 像素，
//     并检查错误原样显示、源串/纯文本和 Snapshot，无原生输入/IME 验收承诺。
//
// 像素：透明背景、预乘 RGBA、同倍率/坐标逐像素相等，不跨字体/平台使用金图。
// 副作用：仅窗口和日志；指定目录时写 PNG，不访问剪贴板或修改系统设置。
// 复位：重启。退出：关闭窗口；-auto 提前关闭、断言失败或 20 秒超时均为失败。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/textformat"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
func check(ok bool, message string) {
	if !ok {
		log.Fatal(message)
	}
}

type reference struct {
	gui.WidgetBase
	native typography.TextLayout
}

func (r *reference) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: 360, Height: 58}))
}
func (r *reference) Paint(p gui.Painter) {
	if r.native != nil {
		p.DrawTextLayout(geometry.Point{}, r.native)
	}
}

type probe struct {
	*gui.Label
	reported bool
}

func (p *probe) Paint(painter gui.Painter) {
	if !p.reported {
		p.reported = true
		img, err := painter.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		must(err)
		log.Printf("OS=%s painter=%T scale=%g", runtime.GOOS, img, painter.PixelScale())
		img.Destroy()
	}
	p.Label.Paint(painter)
}

func main() {
	auto := flag.Bool("auto", false, "自动断言后退出")
	capture := flag.String("capture-dir", "", "仓库外已有的截图目录")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.LabelFormat")
	must(err)
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 920, Height: 700}, Chrome: gui.WindowChromeIntegrated})
	must(err)
	defer win.Destroy()
	win.SetTitle("GOUI Label format")
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	root.SetPadding(geometry.UniformInsets(14))
	root.SetSpacing(10)
	title := &probe{Label: gui.NewLabel("GOUI Label format — Basic / Light / Dark")}
	header := gui.NewHeaderBar()
	header.SetChild(title)
	root.AddChild(header)
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetSpacing(8)
	root.AddChild(toolbar)
	plain := gui.NewLabel(`Raw: a < b & C:\Users\Example \{bold:literal\}`)
	root.AddChild(plain)
	heading := gui.NewLabel(`\{format\}Heading 24 pt — \{warning:warning \{bold:bold\}\} / \{italic:italic\}`)
	heading.SetStyleName("heading")
	heading.SetWrapMode(gui.WrapWordChar)
	root.AddChild(heading)
	mixed := gui.NewLabel(`\{format\}Size: \{small:12 pt\} / \{big:28 pt\} / \{underline:下划线\} / \{strikethrough:删除线\}`)
	root.AddChild(mixed)
	template := `\{format\}File: \{bold:%s\} — é / 中文 / العربية / 🙂`
	file := `C:\data\{format\}\{bold:literal\}.txt`
	message := gui.NewLabel(fmt.Sprintf(template, textformat.EscapeArg(template, file)))
	message.SetWrapMode(gui.WrapWordChar)
	root.AddChild(message)
	badSource := `\{format\}\{bold:unclosed`
	bad := gui.NewLabel(badSource)
	root.AddChild(bad)
	source := `\{format\}A\{bold:B\}\{italic:C\}\{red:D\}\{underline:E\}\{strikethrough:F\}\{big:G\}`
	sample := gui.NewLabel(source)
	sample.SetStyleName("sample")
	sample.SetMinSize(geometry.Size{Width: 360, Height: 58})
	sample.SetMaxSize(geometry.Size{Width: 360, Height: 58})
	ref := &reference{}
	ref.ConnectUnmount(func() {
		if ref.native != nil {
			ref.native.Destroy()
			ref.native = nil
		}
	})
	row := gui.NewLinearBox(layout.DirectionHorizontal)
	row.SetSpacing(20)
	row.SetCrossAlign(layout.CrossStart)
	row.AddChild(sample)
	row.AddChild(ref)
	root.AddChild(row)
	input := gui.NewTextInput()
	input.SetText(source)
	root.AddChild(input)
	view := gui.NewTextView()
	view.SetReadOnly(true)
	view.SetMaxSize(geometry.Size{Height: 62})
	view.Model().SetText(source)
	root.AddChild(view)
	status := gui.NewLabel("Checking native text layout and source/plain contracts…")
	root.AddChild(status)
	win.SetWidget(root)
	mode := 0
	apply := func(index int) {
		mode = index
		rules := basic.Rules()
		if index > 0 {
			rules = modern.Rules(modern.Options{Dark: index == 2})
		}
		rules = append(rules, style.Name("heading").FontSize(24), style.Name("sample").FontSize(20), style.Name("small").FontSize(12), style.Name("big").FontSize(28), style.Name("red").ForegroundColor(color.RGBA{R: 180, A: 255}))
		app.SetStyleSheet(style.Sheet(rules...))
	}
	apply(0)
	verify := func() {
		check(sample.Text() == source && sample.PlainText() == "ABCDEFG" && sample.Snapshot().Text == "ABCDEFG", "source/plain/Snapshot mismatch")
		check(!sample.Focusable() && input.Text() == source && view.Model().Text() == source, "Label became selectable or editor interpreted format")
		check(bad.PlainText() == badSource, "invalid format was partially displayed")
		check(message.PlainText() == "File: "+file+" — é / 中文 / العربية / 🙂", "argument was interpreted")
		parsedMeta, ok := sample.Snapshot().Attributes[gui.LabelInfoKey].(gui.LabelInfo)
		check(ok && parsedMeta.Formatted && parsedMeta.FormatError == "", "missing format metadata")
		// 基础字体取当前主题；范围预期按固定 ABCDEFG 字节位置手工构造，不调用解析器。
		base := gui.ResolveWidgetStyle(sample, "label", style.PartDefault, style.Normal)
		family, _ := base.FontFamily()
		ink, _ := base.ForegroundColor()
		font := typography.FontInfo{Family: family, Size: 20}
		if ref.native != nil {
			ref.native.Destroy()
		}
		ref.native, err = app.Typography().NewTextLayout("ABCDEFG", typography.TextFormat{Font: font, TextColor: ink}, 360, 58)
		must(err)
		bold := font
		bold.Weight = typography.FontWeightBold
		ref.native.SetTextFont(1, 1, bold)
		italic := font
		italic.Style = typography.FontStyleItalic
		ref.native.SetTextFont(2, 1, italic)
		ref.native.SetTextColor(3, 1, color.RGBA{R: 180, A: 255})
		ref.native.SetUnderline(4, 1, true)
		ref.native.SetStrikethrough(5, 1, true)
		big := font
		big.Size = 28
		ref.native.SetTextFont(6, 1, big)
		mw, mh := ref.native.MeasureSize()
		lines, _ := ref.native.MeasureMetrics()
		measured := sample.Measure(layout.Tight(sample.Rect().Size))
		check(len(lines) > 0 && measured.HasBaseline && math.Abs(float64(measured.Baseline-lines[0].Baseline)) < .001, "mixed-font baseline differs from native reference")
		check(mw > 0 && mh > 0, "reference layout empty")
		a, e := gui.RenderWidget(sample, 0)
		must(e)
		b, e := gui.RenderWidget(ref, 0)
		must(e)
		check(a.Bounds() == b.Bounds(), "reference raster bounds differ")
		for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
			for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
				ar, ag, ab, aa := a.At(x, y).RGBA()
				br, bg, bb, ba := b.At(x, y).RGBA()
				check(ar == br && ag == bg && ab == bb && aa == ba, fmt.Sprintf("native reference pixels differ theme=%d at %d,%d: %v / %v", mode, x, y, a.At(x, y), b.At(x, y)))
			}
		}
		status.SetText(fmt.Sprintf("CHECK PASS theme=%d typography=%s native baseline=%.2f", mode, app.Typography().Name(), measured.Baseline))
		log.Printf("CHECK PASS theme=%d typography=%s baseline=%g", mode, app.Typography().Name(), measured.Baseline)
		if *capture != "" {
			img, e := gui.RenderWidget(root, 0)
			must(e)
			f, e := os.Create(filepath.Join(*capture, fmt.Sprintf("label-%d.png", mode)))
			must(e)
			must(png.Encode(f, img))
			must(f.Close())
		}
		ref.RequestPaint()
	}
	for i, name := range []string{"Basic", "Light", "Dark"} {
		index := i
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(name))
		b.ConnectClicked(func() { apply(index) })
		toolbar.AddChild(b)
	}
	button := gui.NewButton()
	button.SetChild(gui.NewLabel("Check"))
	button.ConnectClicked(verify)
	toolbar.AddChild(button)
	must(win.Show())
	done := false
	step := 0
	deadline := time.Now().Add(20 * time.Second)
	timer := app.NewTimer()
	defer timer.Stop()
	timer.ConnectTimeout(func() {
		check(time.Now().Before(deadline), "label verification timed out")
		if sample.Rect().Width <= 0 || ref.Rect().Width <= 0 {
			return
		}
		if step%2 == 0 {
			apply(step / 2)
		} else {
			verify()
		}
		step++
		if step == 6 {
			done = true
			timer.Stop()
			log.Printf("PASS Label formatting: three themes, native range pixels, editor literals, source/plain/Snapshot")
			if *auto {
				win.Destroy()
			}
		}
	})
	must(timer.Start(180 * time.Millisecond))
	app.Run()
	check(done, "window closed before verification completed")
}
