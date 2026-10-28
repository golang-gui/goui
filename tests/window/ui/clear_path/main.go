// 自定义圆角窗口、Root.AfterPaint 与 UI 重建验收。
//
// 环境：三平台可交互桌面；记录后端、缩放，Linux 需 X11 合成器。
// 启动：go run ./tests/window/ui/clear_path [-auto]
// GOUI_PLAT_PAINTER=software/opengl，Windows 另可 direct2d；GOUI_PLAT_SCALE=1/2。
// 初始：居中的透明 None 窗口，蓝色自绘背景铺满，18 DIP 圆角，顶部 HeaderBar。
// 操作与预期：
//  1. 观察蓝底到四边及圆弧内侧，四角外侧透出桌面；拖动标题可以移动窗口。
//  2. Toggle corners 在直角/圆角切换；Rebuild 增加 generation，背景无残影。
//     UI 根对象保持相同，AfterPaint 使用最新闭包，不叠加旧连接。
//  3. Popup 显示带文字的透明 Popover；绿色收尾标记在内容之上，角部透明。
//     点击外部关闭，重新打开仍执行 AfterPaint；不修改 Popover 内建布局策略。
//  4. -auto 检查完整 Rect/Snapshot、最新闭包、Popover 隐藏/重显及销毁后退出，
//     输出 GUI AUTO PASS / DESTROY PASS；自动结果不代替步骤 1–3 的真实输入和外观。
//
// 平台差异：None 不提供原生边框/缩放；这里只用 HeaderBar 请求原生移动。
// 透明圆角不自动改变输入区域；macOS 可能另外提供系统阴影。
// 副作用：仅临时窗口，不写文件或系统设置、不操作剪贴板。
// 复位：重新启动。退出：Quit 或关闭请求；释放拥有的 Popover 和窗口。
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

type surface struct{ gui.WidgetBase }

func newSurface() *surface {
	s := new(surface)
	m := layout.NewLinearLayout(layout.DirectionVertical)
	m.CrossAlign = layout.CrossStretch
	s.SetLayoutManager(m)
	return s
}
func (s *surface) AddChild(child gui.Widget) { s.WidgetBase.AddChild(s, child) }
func (s *surface) Paint(p gui.Painter) {
	r := s.Rect()
	p.FillRect(geometry.Rect(0, 0, r.Width, r.Height), graphics.RGB(125, 185, 225))
}
func (s *surface) Snapshot() gui.WidgetInfo {
	i := s.WidgetBase.Snapshot()
	i.Role = gui.RoleBox
	return i
}

type surfaceView struct {
	ui.ViewBase[surfaceView]
	children []ui.View
}

func panel(children ...ui.View) *surfaceView {
	v := &surfaceView{children: children}
	v.Self = v
	return v
}
func (*surfaceView) Mount(ui.BuildContext) gui.Widget { return newSurface() }
func (v *surfaceView) Update(ctx ui.BuildContext, w gui.Widget) {
	ctx.UpdateChildren(w.(*surface), v.children)
}
func (*surfaceView) Unmount(ui.BuildContext, gui.Widget) {}

func corners(p gui.Painter, b geometry.Rectangle, r float32) {
	r = min(r, b.Width/2, b.Height/2)
	if r <= 0 {
		return
	}
	// 闭合边放在表面外，避免切点处的零宽尖端让 AA 擦除直边像素。
	padding := 2 / p.PixelScale()
	path := graphics.MoveTo(-padding, -padding).LineTo(r, -padding).
		LineTo(r, 0).ArcTo(r, r, 0, 0, 0, 0, r).LineTo(-padding, r).Close()
	p.Save()
	defer p.Restore()
	for _, c := range [][4]float32{{b.X, b.Y, 1, 1}, {b.X + b.Width, b.Y, -1, 1}, {b.X + b.Width, b.Y + b.Height, -1, -1}, {b.X, b.Y + b.Height, 1, -1}} {
		p.SetTransform(geometry.Translate(c[0], c[1]).Scale(c[2], c[3]))
		p.ClearPath(path)
	}
}

func main() {
	auto := flag.Bool("auto", false, "自动验证后退出")
	flag.Parse()
	generation, observed, painted, popupPaints := 1, 0, 0, 0
	rounded, started, complete := true, false, false
	var app ui.App
	var popup gui.Popover
	var original gui.Widget
	openPopup := func() {
		if popup == nil {
			anchor := app.FindWidget("custom", "popup")
			popup = gui.NewPopover(anchor, &gui.PopoverOptions{Transparent: true})
			label := gui.NewLabel("Popover AfterPaint")
			label.SetMinSize(geometry.Size{Width: 260, Height: 90})
			popup.SetWidget(label)
			popup.SetAutoDismiss(true)
			popup.ConnectDismissRequest(popup.Hide)
			popup.ConnectAfterPaint(func(p gui.Painter, b geometry.Rectangle) {
				popupPaints++
				p.FillRect(geometry.Rect(8, 8, b.Width-16, 3), graphics.RGB(30, 150, 70))
				corners(p, b, 12)
			})
		}
		if err := popup.Show(); err != nil {
			panic(err)
		}
	}
	err := ui.Run("org.golang-gui.ClearPathUI", func(a ui.App) ui.RootView {
		app = a
		if !started {
			started = true
			if *auto {
				step, attempts, previousPopup := 0, 0, 0
				app.TickFunc(80*time.Millisecond, func() {
					attempts++
					if attempts > 100 {
						panic("GUI AUTO timeout")
					}
					w := app.FindWindow("custom")
					if w == nil || observed != generation {
						return
					}
					snap := w.Snapshot()
					want := geometry.Rect(0, 0, snap.Bounds.Width, snap.Bounds.Height)
					if w.Widget().Rect() != want || snap.Widget.Bounds != want {
						panic("内容未铺满")
					}
					if original != nil && original != w.Widget() {
						panic("重建替换了根对象")
					}
					original = w.Widget()
					switch step {
					case 0:
						generation++
						rounded = false
						app.RequestUpdate()
					case 1:
						generation++
						rounded = true
						app.RequestUpdate()
					case 2:
						openPopup()
					case 3:
						if popupPaints == 0 {
							return
						}
						previousPopup = popupPaints
						popup.Hide()
					case 4:
						openPopup()
					case 5:
						if popupPaints <= previousPopup {
							return
						}
						popup.Destroy()
						popup = nil
						complete = true
						fmt.Printf("GUI AUTO PASS generation=%d frames=%d popupFrames=%d\n", observed, painted, popupPaints)
						w.Destroy()
					}
					step++
				})
			}
		}
		captured := generation
		radius := float32(0)
		if rounded {
			radius = 18
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{})).Windows(
			ui.Window("custom").Title("GOUI ClearPath UI").Chrome(ui.WindowChromeNone).Transparent(true).Center(true).Size(540, 320).
				Content(panel(
					ui.HeaderBar(ui.Label("Custom rounded window — drag here")),
					ui.VBox(
						ui.Label(fmt.Sprintf("generation=%d | round=%t", generation, rounded)),
						ui.HBox(
							ui.Button("Toggle corners").OnClick(func() { rounded = !rounded; generation++; app.RequestUpdate() }),
							ui.Button("Rebuild").OnClick(func() { generation++; app.RequestUpdate() }),
							ui.Button("Popup").ID("popup").OnClick(openPopup),
							ui.Button("Quit").OnClick(app.Quit),
						).Spacing(8),
						ui.Label("The custom blue background reaches all four edges."),
					).Padding(16).Spacing(12),
				).ID("surface")).
				OnAfterPaint(func(p gui.Painter, b geometry.Rectangle) {
					observed = captured
					painted++
					if painted == 1 {
						img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
						if err != nil {
							panic(err)
						}
						fmt.Printf("painter=%T scale=%g\n", img, p.PixelScale())
						img.Destroy()
					}
					corners(p, b, radius)
				}).OnDestroy(func() {
				if popup != nil {
					popup.Destroy()
					popup = nil
				}
				fmt.Println("DESTROY PASS")
			}),
		)
	})
	if err != nil {
		log.Fatal(err)
	}
	if *auto && !complete {
		log.Fatal("FAIL: automatic verification did not finish")
	}
}
