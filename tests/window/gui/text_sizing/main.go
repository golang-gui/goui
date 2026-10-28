// 文本尺寸及单行正文像素回归。环境：桌面、原生字体与排版库。
// 启动：go run ./tests/window/gui/text_sizing [-capture-dir DIR] [-inspect]。
// DIR 必须是仓库外已有目录。Linux 可用 GOUI_PLAT_PAINTER=software/opengl、
// GOUI_PLAT_SCALE=1/1.5/2、XMODIFIERS=@im=none；Windows 另测 Direct2D，macOS 用 Cmd。
// 初始：Basic、Modern Light/Dark 各一组 TextInput、无框 TextView、带统一外框且
// 高度受限的 ScrollView/TextView。比较单行正文与 Placeholder 的同后端 RGBA
// 像素，外框与圆角按各自职责另验；从正常 DispatchEvent/CommitText 输入和删除
// 换行及长文本，断言内容增长（含宿主边距）、96 DIP 上限、滚动范围、收缩复位、
// baseline 与共享模型视图的独立排版。自动成功关闭，提前关闭或失败非零退出。
// -inspect 保留窗口供人工检查，重启复位；不改剪贴板或系统设置。
// 物理 IME 与候选窗仍须人工验收，模拟提交不代表真实输入法通过。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
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

func main() {
	runtime.LockOSThread()
	out := flag.String("capture-dir", "", "existing temporary PNG directory")
	inspect := flag.Bool("inspect", false, "keep completed window open")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.TextSizing")
	must(err)
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 1050, Height: 480}, Chrome: gui.WindowChromeIntegrated})
	must(err)
	defer win.Destroy()
	root := gui.NewLinearBox(layout.DirectionHorizontal)
	root.SetCrossAlign(layout.CrossStretch)
	var inputs [3]*gui.TextInput
	var views, growing [3]*gui.TextView
	var scrolls [3]*gui.ScrollView
	model := gui.NewTextModel("")
	for i, title := range []string{"Basic", "Modern Light", "Modern Dark"} {
		column := gui.NewLinearBox(layout.DirectionVertical)
		column.SetPadding(geometry.UniformInsets(12))
		column.SetSpacing(8)
		column.SetCrossAlign(layout.CrossStretch)
		column.AddChild(&renderingProbe{Label: gui.NewLabel(title)})
		inputs[i] = gui.NewTextInput()
		views[i] = gui.NewTextView()
		views[i].SetWrapMode(gui.WrapNone)
		inputs[i].SetPlaceholder("Find 中文")
		views[i].SetPlaceholder("Find 中文")
		column.AddChild(inputs[i])
		column.AddChild(views[i])
		growing[i] = gui.NewTextView()
		growing[i].SetModel(model)
		growing[i].SetPlaceholder("Multiline")
		scrolls[i] = gui.NewScrollView()
		scrolls[i].SetStyleName("text-view")
		scrolls[i].SetDirection(gui.ScrollVertical)
		scrolls[i].SetMainWeight(0)
		scrolls[i].SetMaxSize(geometry.Size{Height: 96})
		scrolls[i].SetChild(growing[i])
		column.AddChild(scrolls[i])
		scope := gui.NewStyleScope()
		scope.SetMainWeight(1)
		if i == 0 {
			scope.SetStyleSheet(basic.Sheet())
		} else {
			scope.SetStyleSheet(modern.Sheet(modern.Options{Dark: i == 2}))
		}
		scope.SetChild(column)
		root.AddChild(scope)
	}
	win.SetWidget(root)
	must(win.Show())
	primary, _ := gui.ModPrimary.Resolve()
	key := func(k events.Key) {
		for _, kind := range []events.EventType{events.KeyDown, events.KeyUp} {
			mods := events.Modifiers(0)
			if k == events.KeyA {
				mods = primary
			}
			must(win.DispatchEvent(events.KeyEvent{EventType: kind, Key: k, Modifiers: mods}))
		}
	}
	compare := func() {
		win.SetFocusedWidget(nil)
		for i := range inputs {
			check(inputs[i].Rect().Size == views[i].Rect().Size, fmt.Sprintf("single-line bounds %d: input=%v view=%v", i, inputs[i].Rect(), views[i].Rect()))
			a, err := gui.RenderWidget(inputs[i], 0)
			must(err)
			b, err := gui.RenderWidget(views[i], 0)
			must(err)
			check(a.Bounds() == b.Bounds(), "single-line raster size")
			// TextInput 自绘外框，裸 TextView 仅绘正文；只剔除有契约差异的
			// 边框/圆角带，正文及 Placeholder 仍逐像素精确比较。
			s := gui.ResolveWidgetStyle(inputs[i], "text-input", style.PartDefault, style.Normal)
			border, _ := s.BorderWidth()
			radius, _ := s.Radius()
			scale := float64(a.Bounds().Dx()) / float64(inputs[i].Rect().Width)
			// 再排除一物理像素的笔画抗锯齿覆盖，不改变正文颜色的精确断言。
			inset := int(math.Ceil(float64(max(border, radius))*scale)) + 1
			for y := a.Bounds().Min.Y + inset; y < a.Bounds().Max.Y-inset; y++ {
				for x := a.Bounds().Min.X + inset; x < a.Bounds().Max.X-inset; x++ {
					ar, ag, ab, aa := a.At(x, y).RGBA()
					br, bg, bb, ba := b.At(x, y).RGBA()
					check(ar == br && ag == bg && ab == bb && aa == ba, fmt.Sprintf("single-line pixels differ theme=%d at %d,%d", i, x, y))
				}
			}
		}
	}
	step, complete := 0, false
	var oneLine [3]float32
	timer := app.NewTimer()
	defer timer.Stop()
	timer.ConnectTimeout(func() {
		switch step {
		case 0:
			compare()
			for i := range inputs {
				oneLine[i] = scrolls[i].Rect().Height
			}
			win.SetFocusedWidget(growing[0])
			must(win.CommitText("first"))
		case 1:
			for i := range inputs {
				check(scrolls[i].Rect().Height == oneLine[i], "single line changed height")
			}
			must(win.CommitText("\nsecond"))
		case 2:
			for i := range inputs {
				frameInset := scrolls[i].Children()[0].Rect().Y
				expected := 2*oneLine[i] - 2*growing[i].Padding().Vertical() - 2*frameInset
				// 分数 DIP 的加减有浮点舍入；误差上限 0.001 DIP。
				check(math.Abs(float64(scrolls[i].Rect().Height-expected)) < .001, fmt.Sprintf("multiline height %d: got=%g want=%g", i, scrolls[i].Rect().Height, expected))
			}
			must(win.CommitText(strings.Repeat("\nrow", 80)))
		case 3:
			for i := range inputs {
				check(scrolls[i].Rect().Height == 96 && scrolls[i].Snapshot().MaxScrollY > 0, "cap/scroll range")
			}
			key(events.KeyA)
			key(events.KeyBackspace)
		case 4:
			for i := range inputs {
				check(scrolls[i].Rect().Height == oneLine[i] && scrolls[i].ScrollY() == 0 && scrolls[i].Snapshot().MaxScrollY == 0, "shrink left scroll range")
				inputs[i].SetText("Ag中文 é 👩‍💻")
				views[i].Model().SetText("Ag中文 é 👩‍💻")
			}
		case 5:
			compare()
			win.SetFocusedWidget(growing[0])
			must(win.CommitText(strings.Repeat("wrapped ", 100)))
		case 6:
			for i := range inputs {
				check(model.LineCount() == 1 && scrolls[i].Rect().Height == 96 && scrolls[i].Snapshot().MaxScrollY > 0, "soft wrapping failed to grow")
			}
			img, e := gui.RenderWidget(root, 0)
			must(e)
			scale := float32(img.Bounds().Dx()) / root.Rect().Width
			must(win.DispatchEvent(events.SizeEvent{Width: 900, Height: 480, PixelWidth: 900 * scale, PixelHeight: 480 * scale}))
		case 7:
			compare()
			// Equal model, different view width: reflow one view through Measure
			// and verify its peer's native extent is unchanged.
			before := growing[1].ContentSize()
			growing[0].Measure(layout.Loose(geometry.Size{Width: 120, Height: 96}))
			check(growing[1].ContentSize() == before, "shared model leaked reflow")
			// Return to normal host layout before capturing: the measurement
			// above intentionally used a width different from the allocation.
			growing[0].RequestLayout()
		case 8:
			if *out != "" {
				img, e := gui.RenderWidget(root, 0)
				must(e)
				f, e := os.Create(filepath.Join(*out, "text-sizing.png"))
				must(e)
				must(png.Encode(f, img))
				must(f.Close())
			}
			complete = true
			timer.Stop()
			log.Printf("PASS: 9 text sizing stages, Basic/Modern Light/Modern Dark")
			if !*inspect {
				win.Destroy()
			}
		}
		step++
	})
	must(timer.Start(120 * time.Millisecond))
	app.Run()
	check(complete, "window closed before verification")
}

type renderingProbe struct {
	*gui.Label
	reported bool
}

func (p *renderingProbe) Paint(painter gui.Painter) {
	if !p.reported {
		p.reported = true
		img, err := painter.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		must(err)
		log.Printf("OS=%s typography=%s painter image=%T scale=%g", runtime.GOOS, gui.App.Typography().Name(), img, painter.PixelScale())
		img.Destroy()
	}
	p.Label.Paint(painter)
}
