// 外部滚动控制与声明式 ScrollGroup 窗口验收。
//
// 环境：桌面和正常事件循环；记录系统、Linux WM、实际绘图后端及缩放。
// 启动：go run ./tests/window/ui/scroll_group [-auto] [-basic] [-dark]。
// GOUI_PLAT_PAINTER=software/opengl、GOUI_PLAT_SCALE=1/2 选择后端和倍率。
// 初始：左侧图片隐藏内置条，配有长、短两条外部纵条和一条横条；右侧表格
// 也隐藏内置条，独立外部条与表格间有6 DIP间隔，列宽保证横向溢出。
// 操作及预期：
//  1. 拖动左侧长／短纵条到两端，两条同步，短条的滑块比例与长条一致；
//     图片完整到达首尾。拖横条并滚轮滚动，外部条与实际偏移始终一致。
//  2. F7移除／恢复左侧宿主：条保留布局且变为无范围，恢复后立即可拖动。
//     F8只局部替换宿主，外部条对象及窗口不重建。拖动保持按下时按F8，再
//     移动／释放，旧手势不得修改新宿主；重新按下后可以正常拖动。
//  3. F9切换左侧内置条，外部条仍可用；内外同时显示时都反映相同实际偏移。
//     F6输出CHECK、对象身份、构建次数、范围、页面长度与BOUNDS。
//  4. 拖动右侧表格外部横／纵条，表头横移与正文一致；F10 Reveal末行后纵条
//     到底。左右两组互不影响，外部条不计入表格边框或紧包围轮廓。
//  5. F11切换Modern亮暗，检查原有8 DIP默认条与自定义14/6 DIP长条、留白、
//     裁剪、表头和正文对齐。Basic不随Dark变化。窗口缩放后重复操作。
//
// -auto使用正常DispatchEvent派发指针／快捷键，按可观察条件推进，45秒超时；
// 自动PASS不代替真实鼠标和视觉检查，不使用DevServer或语义动作捷径。
// 平台差异：窗口装饰和字体按平台变化；不涉及新的原生窗口行为。
// 副作用：测试窗口和stdout，不读写用户文件、剪贴板或系统设置。
// 复位：重新启动。退出：关闭窗口；自动模式完成后退出，提前关闭视为失败。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
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

type inputTarget interface{ DispatchEvent(events.Event) error }

func key(w inputTarget, k events.Key) {
	must(w.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: k}))
	must(w.DispatchEvent(events.KeyEvent{EventType: events.KeyUp, Key: k}))
}
func drag(w inputTarget, bar *gui.ScrollBar) {
	r := bar.Snapshot().Bounds
	from, to := geometry.Point{X: r.X + 3, Y: r.Y + 3}, geometry.Point{X: r.X + 3, Y: r.Y + r.Height - 1}
	if bar.Orientation() == layout.DirectionHorizontal {
		to = geometry.Point{X: r.X + r.Width - 1, Y: r.Y + 3}
	}
	for _, e := range []events.PointerEvent{
		{EventType: events.PointerDown, Position: from, Button: events.PointerButtonLeft},
		{EventType: events.PointerMove, Position: to, Button: events.PointerButtonLeft},
		{EventType: events.PointerUp, Position: to, Button: events.PointerButtonLeft},
	} {
		must(w.DispatchEvent(e))
	}
}

type tableModel struct{ *gui.SliceListModel[string] }

func (m *tableModel) RowID(i int) string { return m.ItemAt(i) }

type painterProbe struct {
	gui.WidgetBase
	backend string
}

func (p *painterProbe) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: 1, Height: 1}))
}

func (p *painterProbe) Paint(draw gui.Painter) {
	if p.backend != "" {
		return
	}
	img, err := draw.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	must(err)
	p.backend = fmt.Sprintf("%T", img)
	img.Destroy()
	requested := os.Getenv("GOUI_PLAT_PAINTER")
	require(requested == "" || strings.Contains(strings.ToLower(p.backend), requested), "绘图后端发生回退")
	fmt.Printf("PAINTER os=%s backend=%s scale=%g\n", runtime.GOOS, p.backend, draw.PixelScale())
}

type probeView struct {
	ui.ViewBase[probeView]
	probe *painterProbe
}

func newProbe(p *painterProbe) *probeView              { v := &probeView{probe: p}; v.Self = v; return v }
func (v *probeView) Mount(ui.BuildContext) gui.Widget  { return v.probe }
func (*probeView) Update(ui.BuildContext, gui.Widget)  {}
func (*probeView) Unmount(ui.BuildContext, gui.Widget) {}

func main() {
	auto := flag.Bool("auto", false, "自动检查并退出")
	basic := flag.Bool("basic", false, "Basic外观")
	dark := flag.Bool("dark", false, "Modern暗色")
	flag.Parse()
	version, builtIn := ui.MakeState(0), ui.MakeState(false)
	picture := image.NewRGBA(image.Rect(0, 0, 640, 1800))
	for y := range 1800 {
		for x := range 640 {
			c := color.RGBA{R: 70, G: 130, B: 180, A: 255}
			if y%100 < 3 || x%100 < 3 {
				c = color.RGBA{R: 220, G: 235, B: 245, A: 255}
			}
			if y > 1760 {
				c = color.RGBA{R: 230, G: 130, B: 60, A: 255}
			}
			picture.SetRGBA(x, y, c)
		}
	}
	rows := make([]string, 80)
	for i := range rows {
		rows[i] = fmt.Sprintf("row-%02d", i)
	}
	model := &tableModel{gui.NewSliceListModel(rows)}
	p := new(painterProbe)
	var app ui.App
	started, completed := false, false
	builds, sourceBuilds, nextVersion := 0, 0, 0
	var old *gui.ScrollView
	var originalBar *gui.ScrollBar
	var originalTable *widgets.TableView
	var timer *ui.Timer
	step := 0
	find := func(id string) gui.Widget { return app.FindWidget("scroll", id) }
	bar := func(id string) *gui.ScrollBar { return find(id).(*gui.ScrollBar) }
	source := func() *gui.ScrollView {
		v := version.Get()
		if v < 0 {
			return nil
		}
		w, _ := find(fmt.Sprintf("source-%d", v)).(*gui.ScrollView)
		return w
	}
	check := func() {
		for _, id := range []string{"long", "short", "horizontal", "table-v", "table-h"} {
			b := bar(id)
			r := b.Snapshot()
			require(r.Range != nil, "滚动条没有Range语义")
			require(r.Range.Value >= 0 && r.Range.Value <= r.Range.Max, "滚动条数值越界")
			fmt.Printf("BOUNDS %s %.1f %.1f %.1f %.1f value=%g max=%g page=%g\n", id, r.Bounds.X, r.Bounds.Y, r.Bounds.Width, r.Bounds.Height, r.Range.Value, r.Range.Max, b.PageSize())
		}
		if sv := source(); sv != nil {
			info := sv.Snapshot()
			require(bar("long").Snapshot().Range.Value == info.ScrollY && bar("short").Snapshot().Range.Value == info.ScrollY && bar("horizontal").Snapshot().Range.Value == info.ScrollX, "图片外部条与实际偏移不一致")
			require(bar("long").PageSize() == sv.ScrollGeometry().Viewport.Height, "页面长度错误")
		}
		table := find("table").(*widgets.TableView)
		info := table.Snapshot()
		require(bar("table-v").Snapshot().Range.Value == info.ScrollY && bar("table-h").Snapshot().Range.Value == info.ScrollX, "表格外部条与实际偏移不一致")
		require(bar("long") == originalBar && table == originalTable, "无关控件被重建")
		fmt.Printf("CHECK PASS builds=%d source-builds=%d version=%d\n", builds, sourceBuilds, version.Get())
	}
	must(ui.Run("org.golanggui.ScrollGroupVerification", func(current ui.App) ui.RootView {
		app = current
		builds++
		if !started {
			started = true
			deadline := time.Now().Add(45 * time.Second)
			timer = app.TickFunc(150*time.Millisecond, func() {
				if find("long") == nil || p.backend == "" || bar("long").Rect().Height <= 0 {
					require(time.Now().Before(deadline), "窗口布局超时")
					return
				}
				if originalBar == nil {
					originalBar = bar("long")
					originalTable = find("table").(*widgets.TableView)
					fmt.Println("READY")
					check()
				}
				if !*auto {
					timer.Stop()
					return
				}
				require(time.Now().Before(deadline), "自动场景超时")
				w := app.FindWindow("scroll")
				switch step {
				case 0:
					require(source() != nil && bar("long").PageSize() > 0, "初始宿主没有关联")
					require(bar("short").Rect().Height == 180, "短条没有保留独立轨道长度")
					drag(w, bar("short"))
					require(source().ScrollY() == source().Snapshot().MaxScrollY, "短条没有拖到底")
					check()
					old = source()
					key(w, events.KeyF7)
				case 1:
					if source() != nil {
						return
					}
					require(bar("long").Snapshot().Range.Max == 0, "移除后保留旧范围")
					old.SetScrollY(0)
					require(bar("long").Snapshot().Range.Value == 0, "旧宿主仍控制外部条")
					key(w, events.KeyF8)
				case 2:
					if source() == nil || bar("long").PageSize() == 0 {
						return
					}
					require(source() != old && builds == 1, "局部恢复重建了外部内容")
					drag(w, bar("long"))
					check()
					r := bar("long").Snapshot().Bounds
					must(w.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: r.X + 3, Y: r.Y + r.Height - 3}, Button: events.PointerButtonLeft}))
					key(w, events.KeyF8)
				case 3:
					if source() == nil || source().ID() != "source-2" || bar("long").PageSize() == 0 {
						return
					}
					require(source().ScrollY() == 0 && builds == 1, "局部替换没有保留外部条")
					r := bar("long").Snapshot().Bounds
					for _, kind := range []events.EventType{events.PointerMove, events.PointerUp} {
						must(w.DispatchEvent(events.PointerEvent{EventType: kind, Position: geometry.Point{X: r.X + 3, Y: r.Y + r.Height - 1}, Button: events.PointerButtonLeft}))
					}
					require(source().ScrollY() == 0, "旧拖动手势修改了新宿主")
					drag(w, bar("long"))
					require(source().ScrollY() == source().Snapshot().MaxScrollY, "替换后无法重新拖动")
					drag(w, bar("horizontal"))
					drag(w, bar("table-h"))
					drag(w, bar("table-v"))
					require(source().ScrollX() == source().Snapshot().MaxScrollX, "图片横条不能到达末端")
					require(originalTable.Snapshot().ScrollX == originalTable.Snapshot().MaxScrollX && originalTable.Snapshot().ScrollY == originalTable.Snapshot().MaxScrollY, "表格外部条不能到达末端")
					check()
					key(w, events.KeyF9)
				case 4:
					if source().ScrollGeometry().VScrollBar.Width == 0 {
						return
					}
					require(bar("long").PageSize() == source().ScrollGeometry().Viewport.Height, "内置条占位后页面未同步")
					check()
					completed = true
					fmt.Println("SCROLL GROUP PASS")
					timer.Stop()
					app.Quit()
				}
				step++
			})
		}
		hidden := ui.ScrollBarOptions{Policy: ui.ScrollBarHidden}
		left := ui.ScrollGroup(ui.VBox(
			ui.Label("Image: external bars / short track"),
			ui.HBox(ui.Build(func() ui.View {
				sourceBuilds++
				v := version.Get()
				if v < 0 {
					return nil
				}
				options := hidden
				if builtIn.Get() {
					options = ui.ScrollBarOptions{}
				}
				return ui.ScrollView(ui.Image(picture)).ID(fmt.Sprintf("source-%d", v)).HScrollBarOptions(options).VScrollBarOptions(options)
			}), ui.ScrollBar(layout.DirectionVertical).ID("long").Thickness(14).ThumbThickness(6),
				ui.VBox(ui.ScrollBar(layout.DirectionVertical).ID("short").MinSize(8, 180).MaxSize(8, 180)).CrossAlign(layout.CrossStretch),
			).Spacing(6).CrossAlign(layout.CrossStretch).MainWeight(1),
			ui.ScrollBar(layout.DirectionHorizontal).ID("horizontal"),
		).Spacing(6).CrossAlign(layout.CrossStretch).MainWeight(1))
		right := ui.ScrollGroup(ui.VBox(
			ui.Label("Table: independent external bars"),
			ui.HBox(wui.TableView(model,
				wui.TableColumn("name", "Name", func(_ widgets.TableRow, value string) ui.View { return ui.Label(value) }).Width(300).MinWidth(300),
				wui.TableColumn("description", "Description", func(_ widgets.TableRow, value string) ui.View { return ui.Label(value + " fixed column") }).Width(300).MinWidth(300),
			).ID("table").MainWeight(1).HScrollBarOptions(hidden).VScrollBarOptions(hidden),
				ui.ScrollBar(layout.DirectionVertical).ID("table-v"),
			).Spacing(6).CrossAlign(layout.CrossStretch).MainWeight(1),
			ui.ScrollBar(layout.DirectionHorizontal).ID("table-h"),
		).Spacing(6).CrossAlign(layout.CrossStretch).MainWeight(1))
		root := ui.Root().Windows(ui.Window("scroll").Title("GOUI ScrollGroup").Size(1000, 640).Center(true).Content(ui.VBox(
			ui.Label("F6 Check   F7 Remove/restore   F8 Replace   F9 Built-in   F10 Reveal   F11 Dark"),
			ui.HBox(left, right).Spacing(18).CrossAlign(layout.CrossStretch).MainWeight(1), newProbe(p),
		).Padding(16).Spacing(12).CrossAlign(layout.CrossStretch)).Shortcuts(
			ui.Shortcut(ui.KeyF6).OnActivate(check),
			ui.Shortcut(ui.KeyF7).OnActivate(func() {
				if version.Get() < 0 {
					version.Set(0)
				} else {
					version.Set(-1)
				}
			}),
			ui.Shortcut(ui.KeyF8).OnActivate(func() { nextVersion++; version.Set(nextVersion) }),
			ui.Shortcut(ui.KeyF9).OnActivate(func() { builtIn.Set(!builtIn.Get()) }),
			ui.Shortcut(ui.KeyF10).OnActivate(func() { find("table").(*widgets.TableView).Reveal("row-79") }),
			ui.Shortcut(ui.KeyF11).OnActivate(func() { *dark = !*dark; app.RequestUpdate() }),
		))
		if !*basic {
			root = root.StyleSheet(modern.Sheet(modern.Options{Dark: *dark}))
		}
		return root
	}))
	if *auto {
		require(completed, "自动验收提前退出")
	}
}
