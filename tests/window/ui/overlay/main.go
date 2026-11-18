// 声明式 Overlay 验证。
// 环境：三平台桌面，Software/OpenGL（Windows 另有 Direct2D），1x/2x。
// 启动：go run ./tests/window/ui/overlay
// 初始：浮动面板可见，正文可编辑，背景/确认计数为零。
// 操作与预期：点击背景按钮只增加背景计数；面板输入文字后隐藏/重显应保留文字；
// 切换左/右、填满和窗口尺寸，面板应按新空间布局；显示确认遮罩后背景不可点击，
// 取消不增加确认计数，确认只增加一次；亮暗主题均有半透明遮罩和可读正文。
// 副作用：仅创建窗口。复位：重启。退出：关闭窗口。
// 本目录 go test 仅检查独立布局/状态，不替代上述窗口操作。
package main

import (
	"fmt"
	"image/color"
	"log"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/utils"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

type demoState struct {
	showPanel     bool
	left          bool
	fill          bool
	showConfirm   bool
	background    int
	confirmations int
}

type demo struct {
	state ui.State[demoState]
	text  ui.State[string]
}

func newDemo() *demo {
	return &demo{
		state: ui.MakeState(demoState{showPanel: true}),
		text:  ui.MakeState("This text survives hiding the panel"),
	}
}

func main() {
	d := newDemo()
	if err := ui.Run("org.golang-gui.UIOverlayExample", func(app ui.App) ui.RootView {
		settings := app.Settings()
		sheet := demoStyle(modern.Options{
			Dark:        settings.ColorScheme() == ui.ColorSchemeDark,
			AccentColor: settings.AccentColor(),
			FontFamily:  settings.FontFamily(), FontSize: settings.FontSize(),
		})
		return ui.Root().StyleSheet(sheet).Windows(
			ui.Window("ui-overlay").Title("GOUI UI Overlay").
				Chrome(ui.WindowChromeIntegrated).
				Size(920, 680).MinSize(740, 520).Content(d.content()),
		)
	}); err != nil {
		log.Fatal(err)
	}
}

func demoStyle(options modern.Options) style.StyleSheet {
	return style.Sheet(append(modern.Rules(options),
		style.Name("ui-overlay-mask").BackgroundColor(color.NRGBA{A: 128}),
	)...)
}

func (d *demo) change(update func(*demoState)) {
	d.state.Update(func(s demoState) demoState { update(&s); return s })
}

func (d *demo) confirm() {
	d.change(func(s *demoState) {
		if s.showConfirm {
			s.confirmations++
			s.showConfirm = false
		}
	})
}

func (d *demo) content() ui.View {
	s := d.state.Get()
	panelLabel, side, fill := "Show panel", "Right", "Off"
	if s.showPanel {
		panelLabel = "Hide panel"
	}
	if s.left {
		side = "Left"
	}
	if s.fill {
		fill = "On"
	}
	toolbar := ui.HBox(
		ui.Button(panelLabel).ID("toggle-panel").OnClick(func() {
			d.change(func(s *demoState) { s.showPanel = !s.showPanel })
		}),
		ui.Button("Position: "+side).ID("toggle-position").OnClick(func() {
			d.change(func(s *demoState) { s.left = !s.left })
		}),
		ui.Button("Fill: "+fill).ID("toggle-fill").OnClick(func() {
			d.change(func(s *demoState) { s.fill = !s.fill })
		}),
		ui.Button("Confirmation").ID("open-confirm").OnClick(func() {
			d.change(func(s *demoState) { s.showConfirm = true })
		}),
	).Spacing(8)

	rows := make([]ui.View, 36)
	for i := range rows {
		rows[i] = ui.Label(fmt.Sprintf("%02d    Scroll outside the floating panel; this remains ordinary content.", i+1))
	}
	background := ui.VBox(
		ui.Button(fmt.Sprintf("Background clicks: %d", s.background)).ID("background-counter").
			OnClick(func() { d.change(func(s *demoState) { s.background++ }) }),
		ui.ScrollView(ui.VBox(rows...).Spacing(12).Padding(8)).ID("background-scroll"),
	).Spacing(10).Padding(8).CrossAlign(layout.CrossStretch)

	panel := surface(ui.VBox(
		ui.Label("Floating panel"),
		ui.TextInput().BindText(&d.text).ID("panel-input"),
		ui.Label("Hide / show keeps this widget mounted.").Style(modern.MutedText),
		ui.Label("Fill blocks the whole work area.").Style(modern.MutedText),
		ui.Button("Close panel").ID("close-panel").OnClick(func() {
			d.change(func(s *demoState) { s.showPanel = false })
		}),
	).Spacing(10).Padding(16).MinWidth(310).CrossAlign(layout.CrossStretch)).
		Name("floating-panel").Style("popover").MaxWidth(360).Visible(s.showPanel)
	work := ui.Overlay(background,
		ui.OverlayItem(panel).Fill(s.fill).OnPosition(func(available, size geometry.Size, p *geometry.Point) {
			*p = panelPosition(available, size, s.left, s.fill)
		}),
	).ID("work-area").MainWeight(1)
	content := ui.VBox(
		toolbar,
		ui.Label("Resize the window, move the panel, then try Fill and the confirmation layer.").Style(modern.MutedText),
		work,
		ui.Label(fmt.Sprintf("Confirmed actions: %d", s.confirmations)).ID("confirmation-count"),
		ui.Label("This demonstrates pointer blocking, not keyboard modality or focus trapping.").Style(modern.MutedText),
	).Spacing(12).Padding(16).CrossAlign(layout.CrossStretch)

	confirmPanel := surface(ui.VBox(
		ui.Label("Apply this example action?"),
		ui.Label("The mask blocks background clicks and scrolling."),
		ui.Label("Confirm increments the counter; it does not close the app.").Style(modern.MutedText),
		ui.HBox(
			ui.Button("Cancel").ID("cancel-confirm").OnClick(func() {
				d.change(func(s *demoState) { s.showConfirm = false })
			}),
			ui.Button().Content(ui.Label("Confirm").Style(modern.PrimaryContent)).ID("confirm-action").Style(modern.Primary).OnClick(d.confirm),
		).Spacing(8).MainAlign(layout.MainEnd),
	).Padding(20).Spacing(12).CrossAlign(layout.CrossStretch)).Style("popover").ID("confirmation-panel").MaxWidth(480)
	confirmation := ui.Overlay(
		surface(nil).Style("ui-overlay-mask").ID("confirmation-mask"),
		ui.OverlayItem(confirmPanel).OnPosition(func(available, size geometry.Size, p *geometry.Point) {
			*p = centered(available, size)
		}),
	).ID("confirmation-layer").Visible(s.showConfirm)
	// 窗口交互留在内容遮罩之外，HeaderBar 通过既有 Chrome 服务拖动并避让原生按钮。
	return ui.VBox(
		ui.HeaderBar(ui.Label("ui.Overlay — declarative placement and input")).ID("header-bar"),
		ui.Overlay(content, ui.OverlayItem(confirmation).Fill(true)).ID("root-overlay").MainWeight(1),
	).CrossAlign(layout.CrossStretch)
}

func panelPosition(available, size geometry.Size, left, fill bool) geometry.Point {
	if fill {
		return geometry.Point{}
	}
	x := max(0, available.Width-size.Width-12)
	if left {
		x = min(12, max(0, available.Width-size.Width))
	}
	return geometry.Point{X: x, Y: min(12, max(0, available.Height-size.Height))}
}

func centered(available, size geometry.Size) geometry.Point {
	return geometry.Point{X: (available.Width - size.Width) / 2, Y: (available.Height - size.Height) / 2}
}

// 此测试专用 Bin 只绘制自身样式背景，不向子项继承样式或增加 Overlay/布局/输入策略。
type surfaceWidget struct {
	gui.WidgetBase
	child gui.Widget
}

func (w *surfaceWidget) Child() gui.Widget { return w.child }
func (w *surfaceWidget) SetChild(child gui.Widget) {
	if w.child == child {
		return
	}
	if w.child != nil {
		w.RemoveChild(w.child)
	}
	w.child = child
	if child != nil {
		w.WidgetBase.AddChild(w, child)
	}
}
func (w *surfaceWidget) Snapshot() gui.WidgetInfo {
	info := w.WidgetBase.Snapshot()
	info.Role = gui.RoleBox
	return info
}
func (w *surfaceWidget) Paint(p gui.Painter) {
	s := gui.ResolveStyle(w.StyleName(), style.PartDefault, style.Normal)
	bounds := geometry.Rectangle{Size: w.Rect().Size}
	radius, _ := s.CornerRadius()
	if c, ok := s.BackgroundColor(); ok && c != nil {
		p.FillPath(utils.RoundRectPath(bounds, radius), graphics.ColorOf(c))
	}
	width, _ := s.BorderWidth()
	if c, ok := s.BorderColor(); ok && c != nil && width > 0 {
		p.FillPath(utils.RoundRectBorderPath(bounds, radius, width), graphics.ColorOf(c))
	}
}

type surfaceView struct {
	ui.ViewBase[surfaceView]
	child ui.View
}

func surface(child ui.View) *surfaceView {
	v := &surfaceView{child: child}
	v.Self = v
	return v
}
func (v *surfaceView) Build() ui.View { return v }
func (v *surfaceView) Mount(ui.BuildContext) gui.Widget {
	w := new(surfaceWidget)
	w.SetLayoutManager(layout.NewFillLayout())
	return w
}
func (v *surfaceView) Update(ctx ui.BuildContext, widget gui.Widget) {
	ctx.UpdateChild(widget.(*surfaceWidget), v.child)
}
func (*surfaceView) Unmount(ui.BuildContext, gui.Widget) {}
