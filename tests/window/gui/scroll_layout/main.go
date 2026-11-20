// ScrollView 方向与布局窗口验收。
//
// 环境：三平台桌面；记录系统、后端和缩放，Linux Integrated 需要 X11 合成器。
// 启动：go run ./tests/window/gui/scroll_layout；可指定 GOUI_PLAT_PAINTER 和
// GOUI_PLAT_SCALE=1/2。初始为长表单、浅色、Integrated 窗口。
// 操作与预期：
//
//  1. 左侧表单仅有竖滚动条；缩窄/放宽窗口，文字和按钮按视口宽度换行，
//     不出现横滚动条。真实拖动窗口边框后先按 F6，再滚动并按 F6：内容右边距
//     应保持一致，不能因滚动多占用或少占用一条滚动条宽度。
//     滚到底部，能完整看到 END 标记；拖滑块回顶部。
//
//  2. 右侧 Both 样例是 100×100 DIP 视口中的 96×200 DIP 图片。
//     应同时显示两条滚动条，默认内容区 92×92，最大偏移 X=4、Y=108。
//     拖动两条滑块，边界完整且可命中；仅看窗口启动不算通过。
//
//  3. Horizontal 样例只能横滚；Shift+滚轮可移动图片，普通竖滚轮不移动它。
//     左侧输入框能正常输入，聚焦后滚动条仍可拖动，焦点不改变滚动方向。
//
//  4. F7 切换短/长表单：短内容应无滚动条，恢复长内容后重新出现；再滚到底。
//     F8 最大化、F9 恢复，换行和滚动范围更新，不保留旧的多余滚动条。
//
//  5. 每项操作后按 F6，输出 SCROLL LAYOUT PASS 和各视口/偏移的公开几何断言。
//     F11 切换亮暗后重复检查；1x/2x 分别验收，默认轨道与滑块都为8 DIP，
//     对应8/16物理像素，没有额外的两侧留白。
//
//  6. F10 定位到表单中的 Nested CrossStretch 面板，按 F6 记录外层偏移和内层 bounds。
//     将鼠标置于内层列表中滚动，再按 F6：仅 nested 偏移改变，form 不变。
//     滚到内层首尾后继续滚动，form 仍不变。鼠标移到面板左侧文字后滚动，
//     form 应正常滚动。F4 将内层切为短内容，F10 重新定位并按 F6 记录；
//     在内层短内容上滚动，form 应改变，nested 保持 0。F4 恢复长内容。
//
//  7. F5 只查询表单在当前紧约束及其他宽松约束下的测量结果，不安排新尺寸；
//     应输出 REMEASURE PASS，视口、内容及滚动范围不变。再滚动并按 F6，
//     右边距仍不变。窗口变宽/变窄、F11 切换亮暗后重复此项。
//
//  8. F3 循环 Both 样例的默认、自定义、Hidden、Always 配置，每次 F6。
//     自定义横条 10/6/3 DIP、纵条 20/8/4 DIP（轨道/滑块/额外间距），
//     内容区 76×87；轨道和间距不盖内容。拖两轴滑块含留白区域，F6 输出偏移。
//     Hidden 无条和占位但滚轮仍有效；Always 即使 F2 切到 20×20 短图片也有
//     双条和完整滑块，点击不滚动。Auto 配合短图片无条。F2 恢复长图片。
//     拖动中保持按下并按 F3，之后移动/释放不能再用旧抓取位置改变偏移。
//
//  9. F1 临时替换 Both 内容，在真实布局中先请求远处 Reveal，再 SetScrollY(0)。
//     下一事件循环应输出 SCROLL INTENT PASS 并恢复原内容；视口不能被旧请求拉走。
//     若 5 秒内没有结果，视为失败，不把 READY／窗口启动算作通过。
//
//  10. F12循环Both交汇角的默认透明、蓝色实心、蓝底红边圆角样式；每次F6输出
//     CORNER PASS。默认角落透出容器底色，圆角不出现方形补块；轨道外观不变。
//     配合F3的不同宽度／间隔、Hidden及F2短内容检查角区出现／消失，再F11切换亮暗。
//
// 平台差异：字体影响换行阈值；Linux 无桌面合成时 Integrated 回退 Native。
// 副作用：窗口和日志，不读写剪贴板/文件、不启动 DevServer、不改系统设置。
// 复位：F9 恢复普通状态，F7 恢复长表单，F4 恢复长内层列表；重启复位偏移。
// 退出：关闭窗口。
package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"runtime"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
)

type backendProbe struct {
	gui.WidgetBase
	reported bool
}

func (p *backendProbe) Paint(painter gui.Painter) {
	if p.reported {
		return
	}
	p.reported = true
	img, err := painter.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	if err != nil {
		panic(err)
	}
	fmt.Printf("backend=%T scale=%g\n", img, painter.PixelScale())
	img.Destroy()
}

func picture(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			c := color.RGBA{R: 70, G: 140, B: 190, A: 255}
			if x < 2 || y < 2 || x >= width-2 || y >= height-2 || x%20 == 0 || y%20 == 0 {
				c = color.RGBA{R: 230, G: 240, B: 250, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// 在真实分配期间发起导航和显式滚动；不接触 ScrollView 私有状态。
type scrollOrderContent struct {
	gui.WidgetBase
	once func()
}

func (c *scrollOrderContent) ContentSize() geometry.Size {
	return geometry.Size{Width: 50, Height: 1000}
}
func (c *scrollOrderContent) LayoutVisible(geometry.Size, geometry.Point) {
	if fn := c.once; fn != nil {
		c.once = nil
		fn()
	}
}

func main() {
	runtime.LockOSThread()
	app, err := gui.NewApplication("org.golang-gui.ScrollLayout")
	if err != nil {
		log.Fatal(err)
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 960, Height: 680}, Chrome: gui.WindowChromeIntegrated})
	if err != nil {
		log.Fatal(err)
	}
	defer win.Destroy()
	if err := win.SetTitle("GOUI Scroll layout"); err != nil {
		log.Fatal(err)
	}
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	header := gui.NewHeaderBar()
	header.SetChild(gui.NewLabel("Scroll layout — F6 check / F10 nested / F4 short"))
	root.AddChild(header)
	row := gui.NewLinearBox(layout.DirectionHorizontal)
	row.SetCrossAlign(layout.CrossStretch)
	row.SetPadding(geometry.UniformInsets(12))
	row.SetSpacing(16)
	row.SetMainWeight(1)
	root.AddChild(row)
	form := gui.NewScrollView()
	form.SetDirection(gui.ScrollVertical)
	content := gui.NewLinearBox(layout.DirectionVertical)
	content.SetCrossAlign(layout.CrossStretch)
	content.SetSpacing(12)
	content.SetPadding(geometry.UniformInsets(12))
	paragraph := gui.NewLabel(strings.Repeat("有限宽度用于换行，纵向自然展开。 ", 12))
	paragraph.SetWrapMode(gui.WrapWordChar)
	content.AddChild(paragraph)
	input := gui.NewTextInput()
	input.SetText("输入文字，随后拖动滚动条")
	content.AddChild(input)
	chips := gui.NewWrapBox(layout.DirectionHorizontal)
	chips.SetSpacing(8)
	chips.SetLineSpacing(8)
	for i := range 12 {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(fmt.Sprintf("Wrap %02d", i+1)))
		chips.AddChild(b)
	}
	content.AddChild(chips)
	var longRows []gui.Widget
	for i := range 30 {
		label := gui.NewLabel(fmt.Sprintf("滚动行 %02d：自然高度，不消费整个表单高度", i+1))
		label.SetWrapMode(gui.WrapWordChar)
		longRows = append(longRows, label)
		content.AddChild(label)
	}
	panel := new(gui.WidgetBase)
	panel.SetMinSize(geometry.Size{Height: 110})
	panel.SetMaxSize(geometry.Size{Height: 110}) // 固定面板高度，内层 ScrollView 才有有限视口。
	panel.SetLayoutManager(layout.NewFillLayout())
	stretch := gui.NewLinearBox(layout.DirectionHorizontal)
	stretch.SetCrossAlign(layout.CrossStretch)
	stretch.SetPadding(geometry.UniformInsets(16))
	leaf := gui.NewLabel("Nested CrossStretch: 110 DIP panel")
	leaf.SetMainWeight(1)
	stretch.AddChild(leaf)
	nested := gui.NewScrollView()
	nested.SetDirection(gui.ScrollVertical)
	nested.SetMinSize(geometry.Size{Width: 160})
	nested.SetMaxSize(geometry.Size{Width: 160})
	nested.SetMainWeight(0)
	nestedContent := gui.NewLinearBox(layout.DirectionVertical)
	nestedContent.SetCrossAlign(layout.CrossStretch)
	var nestedRows []gui.Widget
	for i := range 16 {
		label := gui.NewLabel(fmt.Sprintf("内层滚动 %02d", i+1))
		nestedContent.AddChild(label)
		nestedRows = append(nestedRows, label)
	}
	nested.SetChild(nestedContent)
	stretch.AddChild(nested)
	panel.AddChild(panel, stretch)
	content.AddChild(panel)
	tail := gui.NewLabel("END — 应能完整滚动到此标记")
	content.AddChild(tail)
	form.SetChild(content)
	row.AddChild(form)
	right := gui.NewLinearBox(layout.DirectionVertical)
	right.SetSpacing(12)
	right.AddChild(gui.NewLabel("Both: 96×200 / 100×100"))
	both := gui.NewScrollView()
	both.SetMainWeight(0)
	both.SetMinSize(geometry.Size{Width: 100, Height: 100})
	both.SetMaxSize(geometry.Size{Width: 100, Height: 100})
	bothImage := gui.NewImage(picture(96, 200))
	both.SetChild(bothImage)
	right.AddChild(both)
	right.AddChild(gui.NewLabel("Horizontal: Shift+wheel"))
	horizontal := gui.NewScrollView()
	horizontal.SetDirection(gui.ScrollHorizontal)
	horizontal.SetMainWeight(0)
	horizontal.SetMinSize(geometry.Size{Width: 100, Height: 100})
	horizontal.SetMaxSize(geometry.Size{Width: 100, Height: 100})
	horizontal.SetChild(gui.NewImage(picture(200, 96)))
	right.AddChild(horizontal)
	probe := new(backendProbe)
	probe.SetMinSize(geometry.Size{Width: 1, Height: 1})
	right.AddChild(probe)
	row.AddChild(right)
	win.SetWidget(root)
	win.SetMinSize(geometry.Size{Width: 560, Height: 500})
	bind := func(key gui.Key, fn func()) {
		s := gui.NewShortcut(gui.KeyGesture{Key: key})
		s.ConnectActivate(fn)
		win.Shortcuts().AddShortcut(s)
	}
	checkingIntent := false
	bind(gui.KeyF1, func() {
		if checkingIntent {
			return
		}
		checkingIntent = true
		original := both.Child()
		probe := new(scrollOrderContent)
		probe.once = func() {
			both.ScrollIntoView(geometry.Rect(0, 700, 1, 1))
			both.SetScrollY(0)
			app.Post(func() {
				if both.ScrollY() != 0 {
					log.Fatalf("旧 Reveal 覆盖新偏移：%g", both.ScrollY())
				}
				fmt.Println("SCROLL INTENT PASS")
				both.SetChild(original)
				checkingIntent = false
			})
		}
		both.SetChild(probe)
	})
	barMode, shortPicture := 0, false
	bind(gui.KeyF2, func() {
		shortPicture = !shortPicture
		width, height := 96, 200
		if shortPicture {
			width, height = 20, 20
		}
		bothImage.SetImage(picture(width, height))
		fmt.Printf("short-picture=%t\n", shortPicture)
	})
	bind(gui.KeyF3, func() {
		barMode = (barMode + 1) % 4
		h, v := gui.ScrollBarOptions{}, gui.ScrollBarOptions{}
		switch barMode {
		case 1:
			h = gui.ScrollBarOptions{Thickness: 10, ThumbThickness: 6, Spacing: 3}
			v = gui.ScrollBarOptions{Thickness: 20, ThumbThickness: 8, Spacing: 4}
		case 2:
			h.Policy, v.Policy = gui.ScrollBarHidden, gui.ScrollBarHidden
		case 3:
			h.Policy, v.Policy = gui.ScrollBarAlways, gui.ScrollBarAlways
		}
		both.SetHScrollBarOptions(h)
		both.SetVScrollBarOptions(v)
		fmt.Printf("bar-mode=%d h=%+v v=%+v\n", barMode, both.HScrollBarOptions(), both.VScrollBarOptions())
	})
	nestedShort := false
	bind(gui.KeyF4, func() {
		nestedShort = !nestedShort
		for _, label := range nestedRows[1:] {
			label.SetVisible(!nestedShort)
		}
		fmt.Printf("nested-short=%v\n", nestedShort)
	})
	bind(gui.KeyF5, func() {
		before, bounds := form.Snapshot(), content.Rect()
		form.Measure(layout.Tight(form.Rect().Size))
		form.Measure(layout.Loose(geometry.Size{Width: 320, Height: 240}))
		after := form.Snapshot()
		if content.Rect() != bounds || after.MaxScrollX != before.MaxScrollX || after.MaxScrollY != before.MaxScrollY {
			panic(fmt.Sprintf("remeasure changed allocated geometry: content=%v -> %v max=%g/%g -> %g/%g", bounds, content.Rect(), before.MaxScrollX, before.MaxScrollY, after.MaxScrollX, after.MaxScrollY))
		}
		fmt.Println("REMEASURE PASS")
	})
	cornerMode, dark := 0, false
	applyTheme := func() {
		rules := modern.Rules(modern.Options{Dark: dark})
		corner := style.Name("scroll-view").Part("corner").BackgroundColor(color.RGBA{B: 200, A: 255}).BorderWidth(0).Radius(0)
		if cornerMode == 2 {
			corner = corner.BorderColor(color.RGBA{R: 200, A: 255}).BorderWidth(1).Radius(4)
		}
		if cornerMode != 0 {
			rules = append(rules, corner)
		}
		app.SetStyleSheet(style.Sheet(rules...))
	}
	bind(gui.KeyF6, func() {
		if checkingIntent {
			fmt.Println("SCROLL INTENT 正在等待布局")
			return
		}
		b := both.Snapshot()
		viewport := geometry.Size{Width: 92, Height: 92}
		if barMode == 1 {
			viewport = geometry.Size{Width: 76, Height: 87}
		}
		if barMode == 2 || shortPicture && barMode != 3 {
			viewport = geometry.Size{Width: 100, Height: 100}
		}
		width, height := float32(96), float32(200)
		if shortPicture {
			width, height = 20, 20
		}
		g := both.ScrollGeometry()
		if b.MaxScrollX != max(0, width-viewport.Width) || b.MaxScrollY != max(0, height-viewport.Height) || g.Viewport != (geometry.Rectangle{Size: viewport}) {
			panic(fmt.Sprintf("coupled bars: mode=%d short=%t geometry=%+v max=%g/%g", barMode, shortPicture, g, b.MaxScrollX, b.MaxScrollY))
		}
		fmt.Printf("OPTIONS PASS mode=%d short=%t geometry=%+v offset=%g/%g\n", barMode, shortPicture, g, b.ScrollX, b.ScrollY)
		if corner := g.Corner; corner.Width > 0 && corner.Height > 0 {
			img, err := gui.RenderWidget(both, 0)
			if err != nil {
				panic(err)
			}
			scale := float32(img.Bounds().Dx()) / both.Rect().Width
			got := color.RGBAModel.Convert(img.At(int((corner.X+corner.Width/2)*scale), int((corner.Y+corner.Height/2)*scale))).(color.RGBA)
			want := color.RGBA{}
			if cornerMode != 0 {
				want = color.RGBA{B: 200, A: 255}
			}
			if got != want {
				panic(fmt.Sprintf("corner mode=%d color=%v want=%v", cornerMode, got, want))
			}
		}
		fmt.Printf("CORNER PASS mode=%d rect=%v\n", cornerMode, g.Corner)
		f := form.Snapshot()
		wantWidth := form.Rect().Width
		for _, child := range form.Children() {
			if bar, ok := child.(*gui.ScrollBar); ok && bar.Visible() {
				if bar.Orientation() != layout.DirectionVertical || bar.Rect().Width != 8 {
					panic("vertical form exposed a horizontal bar or wrong width")
				}
				wantWidth -= 8
			}
		}
		if f.MaxScrollX != 0 || content.Rect().Width != wantWidth || math.Abs(float64(tail.Rect().Y+tail.Rect().Height+12-content.Rect().Height)) > 0.001 || panel.Rect().Height != 110 {
			panic(fmt.Sprintf("form geometry: content=%v panel=%v tail=%v width=%g", content.Rect(), panel.Rect(), tail.Rect(), wantWidth))
		}
		h := horizontal.Snapshot()
		if h.MaxScrollX != 100 || h.MaxScrollY != 0 || horizontal.ScrollY() != 0 {
			panic(fmt.Sprintf("horizontal ranges: %g/%g", h.MaxScrollX, h.MaxScrollY))
		}
		n := nested.Snapshot()
		if n.MaxScrollX != 0 || (n.MaxScrollY == 0) != nestedShort {
			panic(fmt.Sprintf("nested ranges: %g/%g short=%v", n.MaxScrollX, n.MaxScrollY, nestedShort))
		}
		fmt.Printf("NESTED WHEEL form=%g nested=%g max=%g short=%v\n", form.ScrollY(), nested.ScrollY(), n.MaxScrollY, nestedShort)
		fmt.Printf("SCROLL LAYOUT PASS form=%v content=%v offset=%g max=%g both=%g/%g horizontal=%g\n", form.Snapshot().Bounds, content.Rect(), f.ScrollY, f.MaxScrollY, b.ScrollX, b.ScrollY, h.ScrollX)
		for _, item := range []struct {
			name string
			view *gui.ScrollView
		}{{"form", form}, {"both", both}, {"horizontal", horizontal}, {"nested", nested}} {
			r := item.view.Snapshot().Bounds
			fmt.Printf("BOUNDS %s %g %g %g %g\n", item.name, r.X, r.Y, r.Width, r.Height)
		}
	})
	short := false
	bind(gui.KeyF7, func() {
		short = !short
		for _, label := range longRows {
			label.SetVisible(!short)
		}
		chips.SetVisible(!short)
		paragraph.SetVisible(!short)
		fmt.Printf("short=%v\n", short)
	})
	bind(gui.KeyF8, func() { win.RequestState(gui.WindowStateMaximized) })
	bind(gui.KeyF9, func() { win.RequestState(gui.WindowStateNormal) })
	bind(gui.KeyF10, func() { form.ScrollIntoView(panel.Rect()) })
	bind(gui.KeyF11, func() {
		dark = !dark
		applyTheme()
	})
	bind(gui.KeyF12, func() {
		cornerMode = (cornerMode + 1) % 3
		applyTheme()
		fmt.Printf("corner-mode=%d\n", cornerMode)
	})
	presented := ""
	win.ConnectAfterPaint(func(_ gui.Painter, _ geometry.Rectangle) {
		if form.Rect().Width <= 0 {
			return
		}
		state := fmt.Sprintf("mode=%d corner=%d short=%t form=%v dark=%t nested-short=%t geometry=%+v", barMode, cornerMode, shortPicture, form.Rect().Size, dark, nestedShort, both.ScrollGeometry())
		if state != presented {
			presented = state
			fmt.Println("PRESENT", state)
		}
	})
	win.ConnectDestroy(app.Quit)
	if err := win.Show(); err != nil {
		log.Fatal(err)
	}
	app.Run()
}
