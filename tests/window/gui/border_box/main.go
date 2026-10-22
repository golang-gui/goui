// BorderBox client-drawn window border checks, without DevServer.
//
// Environment: interactive Linux/X11, Windows or macOS desktop. Record OS/WM,
// backend and scale. Run:
//
//	go run ./tests/window/gui/border_box [-auto] [-chrome=none|native|integrated]
//
// Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows) and
// GOUI_PLAT_SCALE=1 or 2. Initially a Native control window is on the left;
// the transparent None window on the right has a plain BorderBox root, a white
// rectangular child and 6 DIP transparent margins that still receive pointer input.
// Steps and expected results:
//  1. Hover each edge midpoint and corner; drag the right edge, bottom edge and
//     top-left corner. Native directional cursors appear, resizing preserves
//     content insets, and edge input does not click child controls.
//  2. Drag the right window's HeaderBar, click Count and type border-test.
//     Only the caption moves the window; the log reports counts and input.
//  3. Toggle rounded appearance from the left window: a white rounded border
//     with a 1 DIP gray stroke and transparent corners surrounds a red child.
//     The rectangular child must not cover the corners. Toggle back to plain.
//  4. Toggle resizing and border enabled state: disabling removes resizing and
//     automatic cursors; re-enabling restores them. Resizable=false retains
//     layout margins; Enabled=false also disables child interaction.
//  5. Toggle the nested border: as an ordinary child, BorderBox stops resizing
//     the host. Restore it as root and re-enter an edge; resizing works with
//     no duplicate hit-test subscriptions.
//  6. Unmount and restore: the subtree returns after 500 ms. Repeat step 1.
//  7. Maximize/restore: BorderBox supplies no resize cursor while maximized and
//     resumes after restore. Record macOS None state requests as actually observed.
//  8. With -chrome=native/integrated, BorderBox provides decoration/layout only.
//     These are not None automatic-resize checks; Linux Integrated has host edges.
//
// -auto checks initial safe layout, snapshot subtrees, appearance changes and
// layout stability when resizing is disabled, then prints PASS and exits.
// It does not replace real pointer/keyboard checks; native cursor shapes may vary.
// Side effects: creates, positions, moves and resizes only this program's windows;
// no system settings or clipboard changes. Reset: restart. Exit: Quit or close
// the left window, releasing the right window and timers.
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
