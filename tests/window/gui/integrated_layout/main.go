// Linux Integrated 四边布局避让验证。
//
// 环境：Linux X11 与合成窗口管理器；记录桌面、实际后端及缩放。
// 启动：go run ./tests/window/gui/integrated_layout
// GOUI_PLAT_PAINTER=software/opengl、GOUI_PLAT_SCALE=1/2 可分别验证。
// 初始：Integrated、40 DIP HeaderBar；根布局没有 Padding，内容双向溢出。
// 操作与预期：
//  1. 观察右侧及底部滚动条，宽度均完整为 8 DIP，外侧四边只显示窗口背景。
//     拖动两条滑块到末尾，再回到开头：最外侧仍完整可见且可命中。
//  2. 在四边及四角拖动缩放；HeaderBar 外侧顶部边缘用于缩放，标题内部用于
//     移动。点击标题中的按钮不会移动窗口。边缘没有 Widget 输入穿透。
//  3. F7 将标题高度在 40/64 DIP 间切换；窗口控制按钮保持垂直居中，右侧距离
//     不随避让重复增加。F8 最大化、F9 恢复：最大化去掉四边留白，恢复后重现。
//  4. 点击输入框输入 ASCII/中文，Tab 切换焦点；候选窗跟随实际光标位置。
//     F10 全屏、F9 恢复，滚动条和标题不残留旧偏移，控制按钮全屏隐藏。
//  5. 各状态按 F6 检查公开 Rect/Snapshot 与两条滚动条边界，输出 LAYOUT PASS；
//     F11 切换亮暗后重复检查。1x/2x 重复操作，1x 滚动条宽 8 像素，2x 宽 16。
//
// 平台差异：仅 Linux GUI 自绘 Integrated 自动内缩；Windows/macOS 不增加边缘。
// Linux 无合成器时可能退回 Native，会打印“不适用”，不能记为 Linux 验收通过。
// 副作用：仅窗口及日志，不自动修改剪贴板、文件或系统设置，不使用 DevServer。
// 复位：F9 恢复普通状态，重新启动重置滚动/高度。退出：关闭主窗口。
package main

import (
	"fmt"
	"image"
	"log"
	"math"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
)

type backendProbe struct {
	gui.WidgetBase
	scale    float32
	reported bool
}

func (b *backendProbe) Paint(p gui.Painter) {
	b.scale = p.PixelScale()
	if !b.reported {
		b.reported = true
		img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		if err != nil {
			panic(err)
		}
		fmt.Printf("backend=%T scale=%g\n", img, b.scale)
		img.Destroy()
	}
}

func main() {
	runtime.LockOSThread()
	app, err := gui.NewApplication("org.golang-gui.IntegratedLayout")
	if err != nil {
		log.Fatal(err)
	}
	dark := false
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 760, Height: 560}, Chrome: gui.WindowChromeIntegrated})
	if err != nil {
		log.Fatal(err)
	}
	defer win.Destroy()
	if err := win.SetTitle("GOUI Integrated layout"); err != nil {
		log.Fatal(err)
	}
	var info gui.ChromeInfo
	win.Chrome().ConnectInfo(func(v gui.ChromeInfo) { info = v })
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	header := gui.NewHeaderBar()
	header.SetPadding(4)
	height := float32(40)
	row := gui.NewLinearBox(layout.DirectionHorizontal)
	row.SetSpacing(12)
	row.AddChild(gui.NewLabel("Integrated layout — F6 check / F7 height"))
	toggleHeight := func() {
		height = 104 - height
		header.SetMinSize(geometry.Size{Height: height})
	}
	button := gui.NewButton()
	button.SetPadding(4)
	button.SetChild(gui.NewLabel("40 / 64 DIP"))
	button.ConnectClicked(toggleHeight)
	row.AddChild(button)
	probe := &backendProbe{scale: 1}
	probe.SetMinSize(geometry.Size{Width: 1, Height: 1})
	row.AddChild(probe)
	header.SetChild(row)
	root.AddChild(header)
	content := gui.NewLinearBox(layout.DirectionVertical)
	content.SetMinSize(geometry.Size{Width: 1000})
	content.SetSpacing(8)
	input := gui.NewTextInput()
	input.SetText("Type here — no root Padding")
	content.AddChild(input)
	for i := range 45 {
		content.AddChild(gui.NewLabel(fmt.Sprintf("Row %02d — F8 maximize / F9 normal / F10 fullscreen / F11 theme", i)))
	}
	scroll := gui.NewScrollView()
	scroll.SetChild(content)
	root.AddChild(scroll)
	win.SetWidget(root)
	// 初始尺寸需要保留双向溢出；显式最小尺寸保持窗口尺寸语义。
	win.SetMinSize(geometry.Size{Width: 500, Height: 300})
	bind := func(key gui.Key, fn func()) {
		s := gui.NewShortcut(gui.KeyGesture{Key: key})
		s.ConnectActivate(fn)
		win.Shortcuts().AddShortcut(s)
	}
	bind(gui.KeyF6, func() {
		snapshot := win.Snapshot()
		inset := float32(0)
		if runtime.GOOS == "linux" && info.Enabled && info.Mode == gui.WindowChromeIntegrated && win.Transparent() {
			if win.State() != gui.WindowStateMaximized && win.State() != gui.WindowStateFullscreen {
				inset = float32(math.Ceil(float64(5*probe.scale))) / probe.scale
			}
		} else if runtime.GOOS == "linux" {
			fmt.Println("Linux 自绘 Integrated 不适用：未建立所需窗口样式")
			return
		}
		want := geometry.Rect(inset, inset, snapshot.Bounds.Width-2*inset, snapshot.Bounds.Height-2*inset)
		if root.Rect() != want || snapshot.Widget.Bounds != want {
			panic(fmt.Sprintf("root=%v snapshot=%v want=%v", root.Rect(), snapshot.Widget.Bounds, want))
		}
		for _, child := range scroll.Children() {
			bar, ok := child.(*gui.ScrollBar)
			if !ok || !bar.Visible() {
				continue
			}
			r := bar.Snapshot().Bounds
			if r.Intersect(want) != r || bar.Orientation() == layout.DirectionVertical && r.Width != 8 ||
				bar.Orientation() == layout.DirectionHorizontal && r.Height != 8 {
				panic(fmt.Sprintf("scrollbar outside layout: %v", r))
			}
		}
		if info.ControlsBounds.Width > 0 && info.ControlsBounds.Height > 0 && win.State() != gui.WindowStateFullscreen &&
			info.ControlsBounds.Center().Y != header.Snapshot().Bounds.Center().Y {
			panic(fmt.Sprintf("controls=%v header=%v", info.ControlsBounds, header.Snapshot().Bounds))
		}
		fmt.Printf("LAYOUT PASS state=%v root=%v controls=%v\n", win.State(), root.Rect(), info.ControlsBounds)
	})
	bind(gui.KeyF7, toggleHeight)
	bind(gui.KeyF8, func() { win.RequestState(gui.WindowStateMaximized) })
	bind(gui.KeyF9, func() { win.RequestState(gui.WindowStateNormal) })
	bind(gui.KeyF10, func() { win.RequestState(gui.WindowStateFullscreen) })
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
