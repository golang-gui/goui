// 持久 Target、外部滚动控制与标签关联窗口验收。
//
// 环境：桌面和正常事件循环；记录系统、Linux WM、实际绘图后端及缩放。
// 启动：go run ./tests/window/ui/scroll_target [-auto] [-basic] [-dark]。
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
//     到底。左右滚动区域互不影响，外部条不计入表格边框或紧包围轮廓。
//  5. F11切换Modern亮暗，检查原有8 DIP默认条与自定义14/6 DIP长条、留白、
//     裁剪、表头和正文对齐。Basic不随Dark变化。窗口缩放后重复操作。
//  6. F12只局部替换右上 TabView，TabBar 对象及外层构建次数不变；标题版本更新，
//     点击B可切换到对应正文，TabBar.View()必须指向新实例，语义仍有两个标签。
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

func tabBounds(info gui.WidgetInfo, text string) (geometry.Rectangle, bool) {
	if info.Role == widgets.RoleTab && info.Text == text && info.Bounds.Width > 0 {
		return info.Bounds, true
	}
	for _, child := range info.Children {
		if bounds, ok := tabBounds(child, text); ok {
			return bounds, true
		}
	}
	return geometry.Rectangle{}, false
}

func tabCount(info gui.WidgetInfo) int {
	count := 0
	if info.Role == widgets.RoleTab {
		count++
	}
	for _, child := range info.Children {
		count += tabCount(child)
	}
	return count
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
	version, builtIn, tabVersion := ui.MakeState(0), ui.MakeState(false), ui.MakeState(0)
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
	builds, sourceBuilds, tabBuilds, nextVersion := 0, 0, 0, 0
	var old *gui.ScrollView
	var originalBar *gui.ScrollBar
	var originalTable *widgets.TableView
	var originalTabs *widgets.TabBar
	var oldTabs *widgets.TabView
	var timer *ui.Timer
	step := 0
	find := func(id string) gui.Widget { return app.FindWidget("scroll", id) }
	bar := func(id string) *gui.ScrollBar { return find(id).(*gui.ScrollBar) }
	source := func() *gui.ScrollView {
		w, _ := find("source").(*gui.ScrollView)
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
		tabs := find("tab-bar").(*widgets.TabBar)
		require(tabs == originalTabs && tabs.View() == find("pages") && len(tabs.View().Pages()) == 2, "标签 Target 或语义内容没有更新")
		require(tabs.Snapshot().Role == widgets.RoleTabBar && tabCount(tabs.Snapshot()) == 2, "TabBar 语义子树丢失或重复标签")
		fmt.Printf("CHECK PASS builds=%d source-builds=%d tab-builds=%d version=%d tab-version=%d\n", builds, sourceBuilds, tabBuilds, version.Get(), tabVersion.Get())
	}
	must(ui.Run("org.golanggui.ScrollTargetVerification", func(current ui.App) ui.RootView {
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
					originalTabs = find("tab-bar").(*widgets.TabBar)
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
					old = source()
					r := bar("long").Snapshot().Bounds
					must(w.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: r.X + 3, Y: r.Y + r.Height - 3}, Button: events.PointerButtonLeft}))
					key(w, events.KeyF8)
				case 3:
					if source() == nil || source() == old || bar("long").PageSize() == 0 {
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
					oldTabs = originalTabs.View()
					key(w, events.KeyF12)
				case 5:
					tabs := originalTabs.View()
					bounds, ready := tabBounds(originalTabs.Snapshot(), "B 1")
					if tabs == oldTabs || !ready {
						return
					}
					require(builds == 1 && tabBuilds == 2 && tabs == find("pages"), "局部替换标签目标重建了引用方")
					position := geometry.Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
					for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
						must(w.DispatchEvent(events.PointerEvent{EventType: kind, Position: position, Button: events.PointerButtonLeft}))
					}
					require(tabs.Current() == tabs.Pages()[1], "新目标的标签无法通过正常输入切换")
					check()
					completed = true
					fmt.Println("SCROLL TARGET PASS")
					timer.Stop()
					app.Quit()
				}
				step++
			})
		}
		hidden := ui.ScrollBarOptions{Policy: ui.ScrollBarHidden}
		left := ui.VBox(
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
				return ui.Modify(ui.ScrollView(ui.Image(picture)).ID(fmt.Sprintf("source-%d", v)).HScrollBarOptions(options).VScrollBarOptions(options)).ID("source")
			}), ui.ScrollBar(layout.DirectionVertical).Target("source").ID("long").Thickness(14).ThumbThickness(6),
				ui.VBox(ui.ScrollBar(layout.DirectionVertical).Target("source").ID("short").MinSize(8, 180).MaxSize(8, 180)).CrossAlign(layout.CrossStretch),
			).Spacing(6).CrossAlign(layout.CrossStretch).MainWeight(1),
			ui.ScrollBar(layout.DirectionHorizontal).Target("source").ID("horizontal"),
		).Spacing(6).CrossAlign(layout.CrossStretch).MainWeight(1)
		right := ui.VBox(
			wui.TabBar().Target("pages").ID("tab-bar"),
			ui.Build(func() ui.View {
				tabBuilds++
				v := tabVersion.Get()
				return ui.Modify(wui.TabView(
					wui.TabPage("a", ui.Label(fmt.Sprintf("Page A version %d", v))).Title(fmt.Sprintf("A %d", v)),
					wui.TabPage("b", ui.Label(fmt.Sprintf("Page B version %d", v))).Title(fmt.Sprintf("B %d", v)),
				).ID(fmt.Sprintf("pages-%d", v))).ID("pages").MinSize(0, 55).MaxSize(0, 55).MainWeight(0)
			}),
			ui.Label("Table: independent external bars"),
			ui.HBox(wui.TableView(model,
				wui.TableColumn("name", "Name", func(_ widgets.TableRow, value string) ui.View { return ui.Label(value) }).Width(300).MinWidth(300),
				wui.TableColumn("description", "Description", func(_ widgets.TableRow, value string) ui.View { return ui.Label(value + " fixed column") }).Width(300).MinWidth(300),
			).ID("table").MainWeight(1).HScrollBarOptions(hidden).VScrollBarOptions(hidden),
				ui.ScrollBar(layout.DirectionVertical).Target("table").ID("table-v"),
			).Spacing(6).CrossAlign(layout.CrossStretch).MainWeight(1),
			ui.ScrollBar(layout.DirectionHorizontal).Target("table").ID("table-h"),
		).Spacing(6).CrossAlign(layout.CrossStretch).MainWeight(1)
		root := ui.Root().Windows(ui.Window("scroll").Title("GOUI ScrollTarget").Size(1000, 640).Center(true).Content(ui.VBox(
			ui.Label("F6 Check   F7 Remove   F8 Replace   F9 Built-in   F10 Reveal   F11 Dark   F12 Tabs"),
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
			ui.Shortcut(ui.KeyF12).OnActivate(func() { tabVersion.Set(tabVersion.Get() + 1) }),
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
