// Widget 组合状态、Box 样式与外部自定义 Widget 验收，不使用 DevServer。
// 环境：桌面会话与系统字体；本轮重点 Linux X11 + 合成窗口管理器。
// 启动：go run ./tests/window/gui/widget_state [-auto]；分别设置
// GOUI_PLAT_PAINTER=software/opengl、GOUI_PLAT_SCALE=1/2。
// 初始：Integrated、Modern 亮色；有背景/边框的 LinearBox 与 WrapBox，
// 自绘色块、普通按钮、CheckButton、Switch，以及独立 StyleScope 内的色块。
// 自动步骤：经 Window.DispatchEvent 移动、按下、释放、Tab；断言组合状态、
// Snapshot、取消/禁用、Checked 保留、样式尺寸变化与透明色合成，PASS 后可手动继续。
// 操作与预期：
//  1. 悬停色块变蓝，按下变深蓝；释放后选中、变宽。悬停/按下/焦点/选中可共存。
//  2. Tab/Shift+Tab 选择控件。按钮的键盘边框不被悬停遮住；空格/回车仍正常激活。
//  3. 复选和开关的 Checked 与显示一致。点击 Disable 再点击色块不会切换业务值；
//     重新启用不恢复过期的 Pressed。颜色反馈不影响外侧窗口边缘缩放与标题拖动。
//  4. Theme 切换亮暗，Box 不丢背景/圆角/内缩边框；最下方局部样式始终独立。
//     样式只作用于各控件自身，子 Label 不继承 Box 字体/颜色；圆角不裁剪子树。
//  5. 改变窗口尺寸，WrapBox 换行但背景包含间距和 Padding。1x/2x 重复检查。
//
// 像素断言：RenderWidget 离屏输出为预乘 sRGB RGBA；在空白 Padding 内取样，
// 透明红色叠加到固定灰白背景，RGB 允许 1/255 舍入误差。不使用字体金图。
// 离屏断言不能代替原生窗口视觉检查；Windows/macOS 运行本例需另行记录结果。
// 副作用：只创建窗口、输出日志，不改剪贴板或系统设置。复位：重启。
// 退出：关闭窗口；-auto 在 PASS 后退出。断言失败、超时或提前关闭返回失败。
package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"math"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

// 应用包实现 Widget；无需调用 WidgetBase.Paint，也无需读取 Snapshot 来绘制。
type badge struct {
	gui.WidgetBase
	notifications, paints int
}

func newBadge() *badge {
	b := new(badge)
	b.SetFocusable(true)
	click := gui.NewClickEventController()
	click.ConnectPressed(func(_ gui.EventContext, v bool) { b.SetState(style.Pressed, v) })
	click.ConnectClicked(func(gui.EventContext) {
		v := !b.State().Has(style.Checked)
		b.SetState(style.Checked|style.Selected, v)
		fmt.Printf("badge-click checked=%v\n", v)
	})
	b.AddEventController(click)
	return b
}
func (b *badge) Measure(c layout.Constraint) layout.Measurement {
	size, _ := gui.WidgetStyle(b, "sample", "").FontSize()
	return layout.Measured(c.Clamp(geometry.Size{Width: size * 6, Height: 40}))
}
func (b *badge) StyleChanged()       { b.notifications++ }
func (b *badge) Paint(p gui.Painter) { b.paints++; gui.PaintWidgetStyle(p, b, "sample") }
func (b *badge) Snapshot() gui.WidgetInfo {
	v := b.WidgetBase.Snapshot()
	v.Role = gui.RoleButton
	v.Text = "Custom state badge"
	if v.Enabled {
		v.Actions = append(v.Actions, gui.ActionClick)
	}
	return v
}

func sheet(dark bool) style.StyleSheet {
	rules := modern.Rules(modern.Options{Dark: dark})
	panelColor := color.RGBA{R: 240, G: 244, B: 248, A: 255}
	if dark {
		panelColor = color.RGBA{R: 40, G: 44, B: 48, A: 255}
	}
	rules = append(rules,
		style.Name("panel").BackgroundColor(panelColor).BorderColor(color.Gray{160}).BorderWidth(2).Radius(12),
		style.Name("chips").BackgroundColor(color.NRGBA{R: 240, G: 40, B: 60, A: 64}).BorderColor(color.Gray{140}).BorderWidth(1).Radius(8),
		style.Name("sample").BackgroundColor(color.Gray{160}).Radius(8).FontSize(18),
		style.Name("sample").State(style.Hovered).BackgroundColor(color.RGBA{R: 60, G: 140, B: 220, A: 255}),
		style.Name("sample").State(style.Pressed).BackgroundColor(color.RGBA{R: 20, G: 70, B: 150, A: 255}),
		style.Name("sample").State(style.Checked).FontSize(26),
		style.Name("sample").State(style.Hovered|style.Checked).BorderColor(color.RGBA{G: 100, A: 255}).BorderWidth(2),
		style.Name("sample").State(style.FocusVisible).BorderColor(color.Black).BorderWidth(2),
		style.Name("sample").State(style.Disabled).BackgroundColor(color.Gray{200}),
	)
	return style.Sheet(rules...)
}
func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
func require(ok bool, args ...any) {
	if !ok {
		log.Fatal(args...)
	}
}
func button(text string, fn func()) *gui.Button {
	b := gui.NewButton()
	b.SetChild(gui.NewLabel(text))
	b.ConnectClicked(fn)
	return b
}
func main() {
	runtime.LockOSThread()
	auto := flag.Bool("auto", false, "自动断言后退出")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.WidgetState")
	must(err)
	app.SetStyleSheet(sheet(false))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 760, Height: 580}, Chrome: gui.WindowChromeIntegrated, Center: true})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("GOUI Widget state / Box style"))
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	header := gui.NewHeaderBar()
	header.SetChild(gui.NewLabel("Box backgrounds / simultaneous Widget.State"))
	root.AddChild(header)
	panel := gui.NewLinearBox(layout.DirectionVertical)
	panel.SetStyleName("panel")
	panel.SetPadding(geometry.UniformInsets(16))
	panel.SetSpacing(12)
	panel.AddChild(gui.NewLabel("Hover / press / click the custom badge"))
	sample := newBadge()
	panel.AddChild(sample)
	chips := gui.NewWrapBox(layout.DirectionHorizontal)
	chips.SetStyleName("chips")
	chips.SetPadding(geometry.UniformInsets(16))
	chips.SetSpacing(8)
	check := widgets.NewCheckButton()
	check.ConnectChange(func(state widgets.CheckState) { fmt.Printf("check-change state=%v\n", state) })
	check.SetChild(gui.NewLabel("Checked"))
	chips.AddChild(check)
	sw := widgets.NewSwitch()
	sw.SetChild(gui.NewLabel("Switch"))
	chips.AddChild(sw)
	chips.AddChild(button("Normal button", func() {}))
	panel.AddChild(chips)
	root.AddChild(panel)
	tools := gui.NewLinearBox(layout.DirectionHorizontal)
	tools.SetPadding(geometry.UniformInsets(12))
	tools.SetSpacing(8)
	dark := false
	tools.AddChild(button("Disable / enable", func() { panel.SetEnabled(!panel.Enabled()) }))
	tools.AddChild(button("Theme", func() { dark = !dark; app.SetStyleSheet(sheet(dark)) }))
	tools.AddChild(button("Reset", func() {
		sample.SetState(style.Checked|style.Selected, false)
		check.SetChecked(false)
		sw.SetChecked(false)
		panel.SetEnabled(true)
	}))
	root.AddChild(tools)
	scope := gui.NewStyleScope()
	scope.SetStyleSheet(style.Sheet(style.Name("sample").BackgroundColor(color.RGBA{R: 70, G: 170, B: 100, A: 255}).Radius(6).FontSize(20)))
	local := newBadge()
	scope.SetChild(local)
	root.AddChild(scope)
	status := gui.NewLabel("Waiting for checks")
	root.AddChild(status)
	win.SetWidget(root)
	done, stage := false, 0
	deadline := time.Now().Add(20 * time.Second)
	timer := app.NewTimer()
	timer.ConnectTimeout(func() {
		require(time.Now().Before(deadline), "TIMEOUT stage=", stage)
		if sample.paints == 0 || !win.Focused() {
			return
		}
		point := sample.Snapshot().Bounds.Center()
		pointer := func(kind events.EventType) {
			must(win.DispatchEvent(events.PointerEvent{EventType: kind, Position: point, Button: events.PointerButtonLeft}))
		}
		switch stage {
		case 0:
			require(math.Abs(float64(sample.Rect().Width-108)) < .01, "initial width ", sample.Rect())
			img, err := gui.RenderWidget(root, 1)
			must(err)
			r := chips.Snapshot().Bounds
			got := color.RGBAModel.Convert(img.At(int(r.X+14), int(r.Y+14))).(color.RGBA)
			for i, pair := range [][2]uint8{{got.R, 240}, {got.G, 193}, {got.B, 201}, {got.A, 255}} {
				require(math.Abs(float64(pair[0])-float64(pair[1])) <= 1, "premultiplied alpha channel=", i, " got=", got)
			}
			pointer(events.PointerMove)
			require(sample.State().Has(style.Hovered) && panel.State().Has(style.Hovered), "ancestor hover missing")
		case 1:
			pointer(events.PointerDown)
			require(sample.State().Has(style.Hovered|style.Pressed|style.Focused), "combined press=", sample.State())
			require(sample.Snapshot().Pressed, "snapshot press missing")
		case 2:
			pointer(events.PointerUp)
			require(sample.State().Has(style.Checked|style.Selected) && !sample.State().Has(style.Pressed), "release=", sample.State())
		case 3:
			require(math.Abs(float64(sample.Rect().Width-156)) < .01, "state font did not relayout ", sample.Rect())
			must(win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyTab}))
			require(win.SetFocusedWidget(sample), "restore focus")
			require(sample.State().Has(style.FocusVisible|style.Focused|style.Checked), "focus hint lost")
			pointer(events.PointerDown)
			panel.SetEnabled(false)
			require(sample.State() == style.Disabled|style.Checked|style.Selected, "disable transient state=", sample.State())
		case 4:
			pointer(events.PointerUp)
			require(sample.State().Has(style.Checked), "disabled input changed business state")
			panel.SetEnabled(true)
			dark = true
			app.SetStyleSheet(sheet(dark))
			check.SetChecked(true)
			sw.SetChecked(true)
		case 5:
			require(check.State().Has(style.Checked) && sw.State().Has(style.Checked), "selection state not synchronized")
			bg, _ := gui.WidgetStyle(local, "sample", "").BackgroundColor()
			require(bg == color.RGBA{R: 70, G: 170, B: 100, A: 255}, "scope lost after theme change")
			done = true
			timer.Stop()
			status.SetText("PASS — manual input and window resize remain available")
			fmt.Printf("PASS backend/scale from environment; state=%v notifications=%d paints=%d\n", sample.State(), sample.notifications, sample.paints)
			if *auto {
				app.Quit()
			}
			return
		}
		status.SetText(fmt.Sprintf("Stage %d | state=%v | notifications=%d", stage, sample.State(), sample.notifications))
		stage++
	})
	must(timer.Start(180 * time.Millisecond))
	must(win.Show())
	app.Run()
	require(done, "window closed before PASS")
}
