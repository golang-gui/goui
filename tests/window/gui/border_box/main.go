// BorderBox 自绘窗口边框验收，不使用 DevServer。
//
// 环境：可交互的 Linux/X11、Windows、macOS 桌面；记录 OS/WM、后端、缩放。
// 启动：go run ./tests/window/gui/border_box [-auto] [-chrome=none|native|integrated]
// GOUI_PLAT_PAINTER=software/opengl（Windows 另可 direct2d），GOUI_PLAT_SCALE=1/2。
// 初始：左侧 Native 控制窗口；右侧透明 None 窗口，以无外观 BorderBox 为根，
// 子内容为白底直角面板，外围 6 DIP 透明留白。透明留白仍接受鼠标输入。
//
// 操作与逐项预期：
//  1. 在右窗的四边中段、四角移动鼠标，再分别拖动右边、下边、左上角。
//     预期：对应原生方向光标；窗口大小变化且内容始终内缩，不发生控件点击。
//  2. 拖动右窗 HeaderBar，点击“计数”并在输入框输入 border-test。
//     预期：只有标题区域移动窗口，输入和点击正常，日志打印计数及输入文本。
//  3. 左窗点击“圆角外观”：白色圆角边框和 1 DIP 灰描边可见，四角透明；
//     子内容红色，用于检查其矩形边界没有盖住圆角。再次点击回到无外观。
//  4. 切换“允许缩放”和“禁用边框”：缩放/自动光标关闭；重新开启恢复。
//     Resizable 关闭时仍保留布局留白；Enabled 关闭时子控件也不能交互。
//  5. 点击“嵌套边框”：BorderBox 改为普通容器的子控件，不再缩放宿主。
//     再次点击恢复根控件，重新进入边缘能缩放，不能残留重复命中订阅。
//  6. 点击“卸载并恢复”：右窗暂时移除子树，500ms 后重新挂载，再重复步骤 1。
//  7. 点击“最大化/恢复”：最大化时 BorderBox 不再提供缩放光标，恢复后生效。
//     macOS None 的状态请求以系统实际支持为准，不能把请求当成已发生状态。
//  8. -chrome=native/integrated：BorderBox 仅装饰/布局，窗口自身装饰仍由宿主管理。
//     这些模式不属于 None 自动缩放验收；Linux Integrated 仍有自身缩放边缘。
//
// -auto：显示后断言默认安全布局、Snapshot 子树、切换外观及关闭缩放不改变布局，
// 打印 PASS 后退出；不替代真实鼠标/键盘操作。平台控制的光标图案可不同。
// 副作用：仅创建、定位、移动和缩放本程序窗口；不改系统设置、剪贴板。
// 复位：重新启动。退出：左窗“退出”或关闭左窗，自动释放右窗与计时器。
package main

import (
	"flag"
	"fmt"
	"image/color"
	"os"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	auto := flag.Bool("auto", false, "assert layout and exit")
	chrome := flag.String("chrome", "none", "none, native or integrated")
	flag.Parse()
	mode, ok := map[string]gui.WindowChromeMode{"none": gui.WindowChromeNone, "native": gui.WindowChromeNative, "integrated": gui.WindowChromeIntegrated}[*chrome]
	if !ok {
		return fmt.Errorf("unknown chrome %q", *chrome)
	}
	app, err := gui.NewApplication("org.golang-gui.BorderBoxTest")
	if err != nil {
		return err
	}
	app.SetStyleSheet(style.Sheet(append(gui.DefaultStyleRules(),
		style.Name("transparent-host").BackgroundColor(color.Transparent),
		style.Name("frame-demo").BackgroundColor(color.White).BorderColor(color.Gray{Y: 128}).BorderWidth(1).Radius(20),
		style.Name("content-demo").BackgroundColor(color.White),
		style.Name("content-red").BackgroundColor(color.RGBA{R: 245, G: 210, B: 210, A: 255}),
	)...))
	control, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 300, Height: 460}})
	if err != nil {
		return err
	}
	defer control.Destroy()
	tool, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 440, Height: 340}, Chrome: mode, Transparent: true})
	if err != nil {
		return err
	}
	defer tool.Destroy()
	control.SetTitle("GOUI BorderBox - controls")
	tool.SetTitle("GOUI BorderBox - subject")
	tool.SetStyleName("transparent-host")
	control.ConnectDestroy(app.Quit)
	button := func(text string, fn func()) *gui.Button {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		b.ConnectClicked(fn)
		return b
	}
	b := gui.NewBorderBox()
	b.SetID("frame")
	content := new(paintedColumn)
	contentLayout := layout.NewLinearLayout(layout.DirectionVertical)
	contentLayout.CrossAlign = layout.CrossStretch
	contentLayout.Spacing = 12
	content.SetLayoutManager(contentLayout)
	content.SetStyleName("content-demo")
	header := gui.NewHeaderBar()
	header.SetChild(gui.NewLabel("BorderBox：拖动标题移动窗口"))
	content.AddChild(header)
	input := gui.NewTextInput()
	input.ConnectText(func(text string) { fmt.Printf("INPUT %q\n", text) })
	content.AddChild(input)
	status := gui.NewLabel("四边/四角：缩放；默认边框不可见")
	count := 0
	content.AddChild(button("计数", func() {
		count++
		status.SetText(fmt.Sprintf("计数=%d", count))
		fmt.Printf("CLICK count=%d\n", count)
	}))
	content.AddChild(status)
	b.SetChild(content)
	tool.SetWidget(b)
	panel := gui.NewLinearBox(layout.DirectionVertical)
	panel.SetPadding(12)
	panel.SetSpacing(12)
	panel.SetCrossAlign(layout.CrossStretch)
	panel.AddChild(gui.NewLabel("右侧 None：BorderBox 为根"))
	styled := false
	panel.AddChild(button("圆角外观", func() {
		styled = !styled
		if styled {
			b.SetStyleName("frame-demo")
			content.SetStyleName("content-red")
		} else {
			b.SetStyleName("")
			content.SetStyleName("content-demo")
		}
		fmt.Printf("STYLE rounded=%v\n", styled)
	}))
	panel.AddChild(button("允许缩放：切换", func() { b.SetResizable(!b.Resizable()); fmt.Printf("RESIZABLE %v\n", b.Resizable()) }))
	panel.AddChild(button("禁用边框：切换", func() { b.SetEnabled(!b.Enabled()); fmt.Printf("ENABLED %v\n", b.Enabled()) }))
	nested := false
	container := gui.NewLinearBox(layout.DirectionVertical)
	container.SetCrossAlign(layout.CrossStretch)
	panel.AddChild(button("嵌套边框：切换", func() {
		tool.SetWidget(nil)
		nested = !nested
		if nested {
			b.SetMainWeight(1)
			container.AddChild(b)
			tool.SetWidget(container)
		} else {
			container.RemoveChild(b)
			tool.SetWidget(b)
		}
		fmt.Printf("NESTED %v\n", nested)
	}))
	restore := app.NewTimer()
	defer restore.Stop()
	var detached gui.Widget
	restore.ConnectTimeout(func() {
		if detached != nil {
			tool.SetWidget(detached)
			detached = nil
			fmt.Println("REMOUNT")
		}
	})
	panel.AddChild(button("卸载并恢复", func() {
		if detached == nil {
			detached = tool.Widget()
			tool.SetWidget(nil)
			_ = restore.StartOnce(500 * time.Millisecond)
		}
	}))
	panel.AddChild(button("最大化/恢复", func() {
		if tool.State() == gui.WindowStateNormal {
			tool.RequestState(gui.WindowStateMaximized)
		} else {
			tool.RequestState(gui.WindowStateNormal)
		}
	}))
	panel.AddChild(button("退出", app.Quit))
	control.SetWidget(panel)
	if err := control.SetPosition(nil, geometry.Point{X: 80, Y: 80}); err != nil {
		return err
	}
	if err := control.Show(); err != nil {
		return err
	}
	if err := tool.SetPosition(nil, geometry.Point{X: 440, Y: 140}); err != nil {
		return err
	}
	if err := tool.Show(); err != nil {
		return err
	}
	fmt.Printf("OS=%s chrome=%s painter=%q scale=%q\n", runtime.GOOS, *chrome, os.Getenv("GOUI_PLAT_PAINTER"), os.Getenv("GOUI_PLAT_SCALE"))
	last := geometry.Size{}
	lastState := gui.WindowStateUnknown
	frame := tool.ConnectFrame(func(time.Time) {
		if state := tool.State(); state != lastState {
			lastState = state
			fmt.Printf("STATE %v\n", state)
		}
		if tool.Widget() != nil && tool.Widget().Rect().Size != last {
			last = tool.Widget().Rect().Size
			fmt.Printf("SIZE %v content=%v\n", last, content.Rect())
		}
	})
	defer frame.Disconnect()
	var result error
	check := app.NewTimer()
	defer check.Stop()
	phase := 0
	var defaultContent geometry.Rectangle
	check.ConnectTimeout(func() {
		if !*auto {
			return
		}
		if phase == 1 {
			if content.Rect() != defaultContent {
				result = fmt.Errorf("disabling resize moved content: %v -> %v", defaultContent, content.Rect())
				app.Quit()
				return
			}
			b.SetStyleName("frame-demo")
			phase++
			result = check.StartOnce(200 * time.Millisecond)
			if result != nil {
				app.Quit()
			}
			return
		}
		if phase == 2 {
			if r := content.Rect(); r.X <= 6 || r.Y <= 6 || r.Width >= defaultContent.Width || r.Height >= defaultContent.Height {
				result = fmt.Errorf("rounded safe inset was not arranged: %v", r)
			} else {
				fmt.Println("PASS default layout/snapshot/Normal state; disabled layout stable; rounded safe inset")
			}
			app.Quit()
			return
		}
		if tool.State() != gui.WindowStateNormal {
			result = fmt.Errorf("newly shown floating window state=%v, want Normal", tool.State())
		} else if info := b.Snapshot(); info.Role != gui.RoleBox || len(info.Children) != 1 || info.Children[0].Bounds.X < info.Bounds.X+6 {
			result = fmt.Errorf("snapshot/safe content invalid: %+v", info)
		} else if r := content.Rect(); r.X != 6 || r.Y != 6 || r.Width != b.Rect().Width-12 || r.Height != b.Rect().Height-12 {
			result = fmt.Errorf("default content inset=%v frame=%v", r, b.Rect())
		} else {
			defaultContent = content.Rect()
			b.SetResizable(false)
			phase++
			result = check.StartOnce(200 * time.Millisecond)
			if result == nil {
				return
			}
		}
		app.Quit()
	})
	if *auto {
		if err := check.StartOnce(700 * time.Millisecond); err != nil {
			return err
		}
	}
	app.Run()
	return result
}

// Layout-only LinearBox deliberately does not paint a surface. This local
// example widget paints its own background without changing child styles.
type paintedColumn struct{ gui.WidgetBase }

func (p *paintedColumn) AddChild(child gui.Widget) { p.WidgetBase.AddChild(p, child) }
func (p *paintedColumn) Paint(painter gui.Painter) {
	if bg, ok := gui.ResolveStyle(p.StyleName(), style.PartDefault, style.Normal).BackgroundColor(); ok && bg != nil {
		painter.FillRect(geometry.Rectangle{Size: p.Rect().Size}, graphics.ColorOf(bg))
	}
}
