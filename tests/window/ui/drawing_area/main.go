// DrawingArea 软件绘图、缓存和 UI 协调窗口验收；需要真实桌面及字体，不使用 DevServer。
// 启动：go run ./tests/window/ui/drawing_area；-auto 自动走 Window.DispatchEvent。
// GOUI_PLAT_PAINTER=software/opengl（Windows 另可 direct2d），GOUI_PLAT_SCALE=1/2；
// -dark 使用暗色。初始为默认重绘策略、360×160 DIP，启动生成一次，后续普通重绘复用。
// 操作及预期：
//  1. Paint 仅请求窗口重绘，终端 DRAW 计数不变；Rebuild 创建新声明闭包，仍不重画。
//     Regenerate 在 ShouldDraw 设置 true，执行一次最新绘图闭包，色块／线条更新。
//  2. Wider 改变面积，默认重画；先 Veto 再 Wider，旧图按原大小显示或裁剪，不拉伸；
//     Default 恢复默认策略后补画当前尺寸。空白和挖孔保持透明，透出当前窗口背景。
//  3. Each paint 后 Paint 每次请求都重画。Frames on 在每个帧回调请求下一帧，绘制
//     策略仍由 ShouldDraw 决定；Frames off 停止申请，画面空闲后不再增加 DRAW。
//  4. Light / Dark 改变主题并重建 UI，尺寸相同时不自动重画软件内容。Regenerate
//     使用最新函数。回调内部 Save／Transform／clip／ClearPath 不污染控件或兄弟。
//  5. F6/Check 输出 DRAWING CHECK、DIP 边界、实际目标 Image 类型和 PixelScale。
//     DRAW 始终报告 Software Image，目标可为 OpenGL／Direct2D；1x/2x 重复观察。
//
// 自动场景断言缓存、闭包更新、显式请求、否决尺寸变化、主题、逐帧绘图及停止。
// 副作用仅测试窗口、焦点、鼠标与日志；不写用户文件、设置、剪贴板。重启复位，
// 关闭退出；自动验收成功输出 DRAWING AUTO PASS，提前退出或断言失败为失败。
package main

import (
	"flag"
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}

type probe struct {
	gui.WidgetBase
	image graphics.Image
	kind  string
	scale float32
}

func newProbe() *probe {
	p := new(probe)
	p.ConnectUnmount(func() {
		if p.image != nil {
			p.image.Destroy()
			p.image = nil
		}
	})
	return p
}
func (p *probe) Paint(draw gui.Painter) {
	if p.image == nil {
		var err error
		p.image, err = draw.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		must(err)
		p.kind = fmt.Sprintf("%T", p.image)
	}
	p.scale = draw.PixelScale()
}

type probeView struct {
	ui.ViewBase[probeView]
	probe *probe
}

func newProbeView(p *probe) ui.View                    { v := &probeView{probe: p}; v.Self = v; return v }
func (v *probeView) Mount(ui.BuildContext) gui.Widget  { return v.probe }
func (*probeView) Update(ui.BuildContext, gui.Widget)  {}
func (*probeView) Unmount(ui.BuildContext, gui.Widget) {}

func main() {
	auto := flag.Bool("auto", false, "通过窗口事件路径自动验收")
	dark := flag.Bool("dark", false, "初始暗色")
	flag.Parse()
	var app ui.App
	var frames signal.Handle
	started, complete, manual := false, false, false
	generation, observed, draws, queries, policy, step := 1, 0, 0, 0, 0, 0
	width := float32(360)
	var baseline int
	var original *gui.DrawingArea
	target := newProbe()
	area := func() *gui.DrawingArea { return app.FindWidget("drawing", "area").(*gui.DrawingArea) }
	paint := func() { must(app.FindWindow("drawing").RequestPaint()) }
	toggleFrames := func() {
		if frames != nil {
			frames.Disconnect()
			frames = nil
			fmt.Println("FRAMES STOP")
			return
		}
		w := app.FindWindow("drawing")
		frames = w.ConnectFrame(func(time.Time) { must(w.RequestPaint()) })
		must(w.RequestPaint())
		fmt.Println("FRAMES START")
	}
	check := func() {
		a := area()
		if original == nil {
			original = a
		}
		require(a == original, "UI 重建替换 DrawingArea")
		info := a.Snapshot()
		require(info.Role == gui.RoleWidget && len(info.Actions) == 0 && !info.Focusable, "绘图区域伪造了交互语义")
		r := info.Bounds
		fmt.Printf("DRAWING CHECK draws=%d queries=%d generation=%d observed=%d target=%s scale=%g bounds=%g,%g,%g,%g\n", draws, queries, generation, observed, target.kind, target.scale, r.X, r.Y, r.Width, r.Height)
		for _, id := range []string{"paint", "rebuild", "regenerate", "wider", "default", "veto", "each", "frames", "theme"} {
			r := app.FindWidget("drawing", id).Snapshot().Bounds
			fmt.Printf("BOUNDS %s %g %g %g %g\n", id, r.X, r.Y, r.Width, r.Height)
		}
	}
	must(ui.Run("org.golang-gui.DrawingArea", func(a ui.App) ui.RootView {
		app = a
		if !started {
			started = true
			if *auto {
				app.TickFunc(200*time.Millisecond, func() {
					if target.scale == 0 || area().Rect().Width == 0 {
						return
					}
					w := app.FindWindow("drawing")
					click := func(id string) {
						r := app.FindWidget("drawing", id).Snapshot().Bounds
						p := geometry.Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
						for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
							must(w.DispatchEvent(events.PointerEvent{EventType: kind, Position: p, Button: events.PointerButtonLeft}))
						}
					}
					switch step {
					case 0:
						check()
						require(draws > 0, "首次未绘制")
						baseline = draws
						click("paint")
					case 1:
						require(draws == baseline, "普通重绘重新生成")
						click("rebuild")
					case 2:
						require(draws == baseline && observed == 1, "闭包重建隐式重绘")
						click("regenerate")
					case 3:
						require(draws == baseline+1 && observed == 2, "未使用最新绘图闭包")
						click("veto")
						click("wider")
					case 4:
						require(draws == baseline+1 && area().Rect().Width == width, "否决尺寸变化失败")
						click("default")
					case 5:
						require(draws == baseline+2, "恢复默认没有补画")
						click("theme")
					case 6:
						require(draws == baseline+2, "主题／重建隐式失效")
						click("each")
					case 7:
						require(draws > baseline+2, "每次绘制策略未生效")
						baseline = draws
						click("frames")
					case 8:
						require(draws > baseline, "帧驱动未绘制")
						click("frames")
						click("default")
						baseline = draws
					case 9:
						require(draws == baseline, "停止后仍重画")
						click("paint")
					case 10:
						require(draws == baseline, "停止后缓存未复用")
						check()
						complete = true
						fmt.Println("DRAWING AUTO PASS")
						app.Quit()
					}
					step++
				})
			}
		}
		update := func() { app.RequestUpdate() }
		captured := generation
		drawing := ui.DrawingArea(func(p ui.Painter, size geometry.Size) {
			draws++
			observed = captured
			img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
			must(err)
			kind := fmt.Sprintf("%T", img)
			img.Destroy()
			require(strings.Contains(kind, "software"), "回调未使用 Software")
			fmt.Printf("DRAW %d generation=%d size=%gx%g software=%s scale=%g\n", draws, captured, size.Width, size.Height, kind, p.PixelScale())
			// 软件回调使用局部 DIP；色块与竖线每次重画改变，便于观察缓存。
			p.FillRoundRect(geometry.Rectangle{Size: size}.Inset(1), 12, ui.Color{R: .1, G: .25, B: .45, A: .75})
			p.FillEllipse(geometry.Point{X: 65, Y: 75}, 38, 38, ui.Color{R: .5, A: .5})
			p.FillEllipse(geometry.Point{X: 100, Y: 75}, 38, 38, ui.Color{B: .5, A: .5})
			p.Save()
			p.SetClipRect(geometry.Rect(155, 20, 120, 110))
			p.SetTransform(geometry.Translate(155, 20))
			p.FillRect(geometry.Rect(0, 0, float32(20+draws%20*4), 110), ui.Color{R: .2, G: .7, B: .3, A: 1})
			p.Restore()
			p.ClearPath(graphics.MoveTo(size.Width-45, 30).LineTo(size.Width-15, 30).LineTo(size.Width-15, 60).Close())
		}).ID("area").MinSize(width, 160).MaxSize(width, 160).OnShouldDraw(func(redraw *bool) {
			queries++
			switch policy {
			case 1:
				*redraw = false
			case 2:
				*redraw = true
			}
			if manual {
				*redraw = true
				manual = false
			}
		})
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{Dark: *dark})).Windows(ui.Window("drawing").Title("GOUI DrawingArea").Chrome(ui.WindowChromeIntegrated).Size(820, 560).Center(true).Content(
			ui.VBox(ui.HeaderBar(ui.Label("DrawingArea — explicit software redraw")), drawing,
				ui.HWrap(ui.Button("Paint").ID("paint").OnClick(paint), ui.Button("Rebuild").ID("rebuild").OnClick(func() { generation++; update() }), ui.Button("Regenerate").ID("regenerate").OnClick(func() { manual = true; paint() }), ui.Button("Wider").ID("wider").OnClick(func() {
					if width == 360 {
						width = 520
					} else {
						width = 360
					}
					update()
				})).Spacing(8).LineSpacing(8),
				ui.HWrap(ui.Button("Default").ID("default").OnClick(func() { policy = 0; paint() }), ui.Button("Veto").ID("veto").OnClick(func() { policy = 1; paint() }), ui.Button("Each paint").ID("each").OnClick(func() { policy = 2; paint() }), ui.Button("Frames on / off").ID("frames").OnClick(toggleFrames), ui.Button("Light / Dark").ID("theme").OnClick(func() { *dark = !*dark; update() })).Spacing(8).LineSpacing(8),
				ui.Label(fmt.Sprintf("Build generation %d — drawing counters are in stdout", generation)),
				ui.Label("F6 checks. Default caches pixels; Veto retains original size; Each paint regenerates.").WrapMode(gui.WrapWordChar),
				ui.Button("Check (F6)").OnClick(check), newProbeView(target),
			).Padding(16).Spacing(16).CrossAlign(layout.CrossStart).Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(check)),
		).OnDestroy(func() {
			if frames != nil {
				frames.Disconnect()
				frames = nil
			}
		}))
	}))
	if *auto {
		require(complete, "自动验收提前退出")
	}
}
