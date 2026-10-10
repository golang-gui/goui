// ScrollView 方向与布局窗口验收。
//
// 环境：三平台桌面；记录系统、后端和缩放，Linux Integrated 需要 X11 合成器。
// 启动：go run ./tests/window/gui/scroll_layout；可指定 GOUI_PLAT_PAINTER 和
// GOUI_PLAT_SCALE=1/2。初始为长表单、浅色、Integrated 窗口。
// 操作与预期：
//  1. 左侧表单仅有竖滚动条；缩窄/放宽窗口，文字和按钮按视口宽度换行，
//     不出现横滚动条。滚到底部，能完整看到 END 标记；拖滑块回顶部。
//  2. 右侧 Both 样例是 100×100 DIP 视口中的 96×200 DIP 图片。
//     应同时显示两条滚动条，内容区 92×92，最大偏移 X=4、Y=108。
//     拖动两条滑块，边界完整且可命中；仅看窗口启动不算通过。
//  3. Horizontal 样例只能横滚；Shift+滚轮可移动图片，普通竖滚轮不移动它。
//     左侧输入框能正常输入，聚焦后滚动条仍可拖动，焦点不改变滚动方向。
//  4. F7 切换短/长表单：短内容应无滚动条，恢复长内容后重新出现；再滚到底。
//     F8 最大化、F9 恢复，换行和滚动范围更新，不保留旧的多余滚动条。
//  5. 每项操作后按 F6，输出 SCROLL LAYOUT PASS 和各视口/偏移的公开几何断言。
//     F11 切换亮暗后重复检查；1x/2x 分别验收，8 DIP 滚动条宽为 8/16 像素。
//  6. F10 定位到表单中的 Nested CrossStretch 面板，按 F6 记录外层偏移和内层 bounds。
//     将鼠标置于内层列表中滚动，再按 F6：仅 nested 偏移改变，form 不变。
//     滚到内层首尾后继续滚动，form 仍不变。鼠标移到面板左侧文字后滚动，
//     form 应正常滚动。F4 将内层切为短内容，F10 重新定位并按 F6 记录；
//     在内层短内容上滚动，form 应改变，nested 保持 0。F4 恢复长内容。
//
// 平台差异：字体影响换行阈值；只有 Linux 自绘 Integrated 内缩窗口内容。
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
	row.SetPadding(12)
	row.SetSpacing(16)
	row.SetMainWeight(1)
	root.AddChild(row)
	form := gui.NewScrollView()
	form.SetDirection(gui.ScrollVertical)
	content := gui.NewLinearBox(layout.DirectionVertical)
	content.SetCrossAlign(layout.CrossStretch)
	content.SetSpacing(12)
	content.SetPadding(12)
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
	panel.SetLayoutManager(layout.NewFillLayout())
	stretch := gui.NewLinearBox(layout.DirectionHorizontal)
	stretch.SetCrossAlign(layout.CrossStretch)
	stretch.SetPadding(16)
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
	both.SetChild(gui.NewImage(picture(96, 200)))
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
	nestedShort := false
	bind(gui.KeyF4, func() {
		nestedShort = !nestedShort
		for _, label := range nestedRows[1:] {
			label.SetVisible(!nestedShort)
		}
		fmt.Printf("nested-short=%v\n", nestedShort)
	})
	bind(gui.KeyF6, func() {
		b := both.Snapshot()
		if b.MaxScrollX != 4 || b.MaxScrollY != 108 {
			panic(fmt.Sprintf("coupled bars: max=%g/%g, want 4/108", b.MaxScrollX, b.MaxScrollY))
		}
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
	short, dark := false, false
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
		app.SetStyleSheet(modern.Sheet(modern.Options{Dark: dark}))
	})
	win.ConnectDestroy(app.Quit)
	if err := win.Show(); err != nil {
		log.Fatal(err)
	}
	app.Run()
}
