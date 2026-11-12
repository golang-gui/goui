// 表格收缩、所属Part和完整内容边线的声明式窗口回归，不使用DevServer。
//
// 环境：Linux/X11（记录WM）、Windows或macOS桌面，记录实际后端与缩放。
// 启动：go run ./tests/window/widgets/table_contracts [-dark]
// GOUI_PLAT_PAINTER=software/opengl（Windows另有direct2d），GOUI_PLAT_SCALE=2。
// 初始：Native 820×620 DIP，两张不同StyleName的表；A有12行、145.5 DIP高，
// B只有一行、100.25 DIP高。两表均关闭视口外框，只绘完整单元格网格。
//
// 操作与逐项预期：
//  1. 等READY，按F6；A表头字号大于B，两张表颜色独立；B右侧/下方留白没有网格。
//     上边/首列左边及末行底边/末列右边完整，文字和内容不盖线。
//  2. 点击Narrow，按F6：列收缩但不低于MinWidth，无横向滚动；基础宽度不变。
//     点击Below min，按F6：列全部到达下限后才横向滚动。点击Wide，按F6：
//     恢复基础宽度和末列伸展；这些操作不输出用户resize。
//  3. 点击Rebuild、Palette后分别按F6：列和单元格身份保留，A文字度量/颜色改变，
//     B保持自己的字体；两表背景按各自Part更新，实际内容框线仍完整。
//  4. Wide状态点击A的正文行，按End，再按F6：末行完整位于小数视口内，
//     ScrollY等于MaxScrollY，尾部余白小于1 DIP；应用没有调整表格高度。
//  5. 点击Finish（或F8）：要求上述状态都已通过；输出PASS并退出0。
//     缺步骤、任何断言失败或提前关闭退出1。F6的像素断言来自Software重绘实际
//     子树；原生窗口的线条、裁剪和文字仍须按上述步骤观察，不把离屏当桌面验收。
//
// 平台差异：macOS功能键被系统占用时点击按钮后用Fn+F6，End可用原生键注入。
// 副作用：仅本测试窗口、内存数据和日志，不读写文件、剪贴板或系统配置。
// 复位：重启。退出：Finish/F8或关闭窗口，UI负责销毁窗口及其控件树。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

type rows struct{ *gui.SliceListModel[string] }

func (r *rows) RowID(i int) string { return fmt.Sprintf("r%d", i) }

var edge = color.RGBA{R: 90, G: 100, B: 110, A: 255}

func main() {
	dark := flag.Bool("dark", false, "use the dark Modern palette")
	flag.Parse()
	if err := run(*dark); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func palette(swapped bool) (color.RGBA, color.RGBA) {
	a, b := color.RGBA{R: 225, G: 234, B: 244, A: 255}, color.RGBA{R: 244, G: 232, B: 220, A: 255}
	if swapped {
		return b, a
	}
	return a, b
}

func sheet(dark, swapped bool) style.StyleSheet {
	base := modern.Rules(modern.Options{Dark: dark})
	rules := slices.Clone(base)
	a, b := palette(swapped)
	for i, name := range []string{"orders", "test"} {
		for _, r := range base {
			if r.Sel.Name == "table-view" {
				r.Sel.Name = name
				rules = append(rules, r)
			}
		}
		bg, font := a, float32(22)
		if swapped {
			font = 26
		}
		if i == 1 {
			bg, font = b, 11
		}
		rules = append(rules,
			style.Name(name).BorderWidth(0),
			style.Name(name).Part("outer-horizontal").BorderWidth(0), style.Name(name).Part("outer-vertical").BorderWidth(0),
			style.Name(name).Part("header").BackgroundColor(bg).BorderWidth(0).Radius(0),
			style.Name(name).Part("header-text").FontSize(font).ForegroundColor(color.RGBA{R: 30, G: 40, B: 50, A: 255}),
			style.Name(name).Part("header-separator").BorderWidth(1).ForegroundColor(edge),
			style.Name(name).Part("row").BorderWidth(0).Radius(0),
		)
		for _, part := range []string{"grid-horizontal", "grid-vertical", "grid-top", "grid-left"} {
			rules = append(rules, style.Name(name).Part(part).BorderColor(edge).BorderWidth(1))
		}
	}
	return style.Sheet(rules...)
}

func run(dark bool) error {
	values := make([]string, 12)
	for i := range values {
		values[i] = fmt.Sprintf("Row %02d", i)
	}
	a, b := &rows{gui.NewSliceListModel(values)}, &rows{gui.NewSliceListModel([]string{"One real row"})}
	stage, generation, swapped := "wide", 0, false
	seen := map[string]bool{}
	var original []*widgets.TableColumn
	resizes, ready, finished := 0, false, false
	var failure error
	err := ui.Run("org.golanggui.TableContracts", func(app ui.App) ui.RootView {
		fail := func(err error) {
			if failure == nil {
				failure = err
			}
			app.Quit()
		}
		lookup := func(id string) *widgets.TableView { return app.FindWidget("main", id).(*widgets.TableView) }
		verify := func() error {
			ta, tb := lookup("a"), lookup("b")
			info, other := ta.Snapshot(), tb.Snapshot()
			if !ready || info.Role != widgets.RoleTable || len(info.Children) < 3 || len(other.Children) != 3 || resizes != 0 {
				return errors.New("table semantics, readiness or resize count")
			}
			base, minima := []float32{160, 180, 140}, []float32{120, 60, 40}
			columns := ta.Columns()
			sum := float32(0)
			for i, c := range columns {
				width := info.Children[i].Bounds.Width
				if c != original[i] || c.Width() != base[i] || width < minima[i] {
					return fmt.Errorf("column %d identity/base/min: %g/%g", i, c.Width(), width)
				}
				sum += width
			}
			switch stage {
			case "narrow":
				if info.MaxScrollX != 0 || sum >= 480 {
					return fmt.Errorf("narrow allocation: sum=%g maxX=%g", sum, info.MaxScrollX)
				}
			case "tiny":
				if sum != 220 || info.MaxScrollX <= 0 {
					return fmt.Errorf("minimum allocation: sum=%g maxX=%g", sum, info.MaxScrollX)
				}
			case "wide":
				if info.MaxScrollX != 0 || sum <= 480 || info.Children[0].Bounds.Width != 160 || info.Children[1].Bounds.Width != 180 {
					return fmt.Errorf("restored allocation: sum=%g maxX=%g", sum, info.MaxScrollX)
				}
			}
			if info.Children[0].Bounds.Height <= other.Children[0].Bounds.Height || info.Bounds.Height != 145.5 || other.Bounds.Height != 100.25 {
				return errors.New("header Part metrics or fractional allocation")
			}
			for _, row := range info.Children[3:] {
				for i, cell := range row.Children {
					if cell.Bounds.X != info.Children[i].Bounds.X || cell.Bounds.Width != info.Children[i].Bounds.Width {
						return errors.New("header/cell allocation differs")
					}
				}
			}
			for index, table := range []*widgets.TableView{ta, tb} {
				data := table.Snapshot()
				count := len(table.Columns())
				img, err := gui.RenderWidget(table, 1)
				if err != nil {
					return err
				}
				header := data.Children[0].Bounds
				point := func(x, y float32) color.RGBA { return color.RGBAModel.Convert(img.At(int(x), int(y))).(color.RGBA) }
				x := header.X - data.Bounds.X
				if stage != "tiny" || index == 1 {
					if point(x, 0) != edge || point(x, header.Height/2) != edge {
						return errors.New("missing top/left cell edge")
					}
				}
				wantA, wantB := palette(swapped)
				want := wantA
				if index == 1 {
					want = wantB
				}
				if got := point(x+header.Width-8, 8); got != want {
					return fmt.Errorf("table %d header Part: %v want %v", index, got, want)
				}
				if index == 1 {
					bg, _ := gui.ResolveStyle("test", "", style.Normal).BackgroundColor()
					expected := color.RGBAModel.Convert(bg).(color.RGBA)
					last := data.Children[count].Bounds
					bottom := last.Y - data.Bounds.Y + last.Height
					if point(250, 0) != expected || point(250, header.Height+8) != expected || point(10, bottom+5) != expected {
						return errors.New("grid/frame extends into empty viewport")
					}
				}
			}
			if stage == "wide" && ta.Current() == "r11" {
				found := false
				for _, row := range info.Children[3:] {
					if row.Attributes[widgets.TableInfoKey].(widgets.TableInfo).RowID == "r11" {
						gap := info.Bounds.Y + info.Bounds.Height - (row.Bounds.Y + row.Bounds.Height)
						if gap < 0 || gap >= 1 || info.ScrollY != info.MaxScrollY {
							return fmt.Errorf("last row clipped: gap=%g scroll=%g/%g", gap, info.ScrollY, info.MaxScrollY)
						}
						found = true
					}
				}
				if !found {
					return errors.New("last row not realized")
				}
				seen["end"] = true
			}
			seen[stage] = true
			if generation > 0 {
				seen["rebuild"] = true
			}
			if swapped {
				seen["palette"] = true
			}
			fmt.Printf("CHECK stage=%s generation=%d palette=%t scroll=%g/%g columns=%g resizes=%d\n", stage, generation, swapped, info.ScrollY, info.MaxScrollY, sum, resizes)
			return nil
		}
		finish := func() {
			if err := verify(); err != nil {
				fail(err)
				return
			}
			for _, s := range []string{"narrow", "tiny", "wide", "rebuild", "palette", "end"} {
				if !seen[s] {
					fail(fmt.Errorf("missing verified stage %s", s))
					return
				}
			}
			finished = true
			fmt.Println("PASS table contracts")
			app.Quit()
		}
		column := func(id, title string, width, minimum float32, expand bool) *wui.TableColumnView[string] {
			return wui.TableColumn(id, title, func(_ widgets.TableRow, text string) ui.View { return ui.Label(text) }).Width(width).MinWidth(minimum).Expand(expand).OnResize(func(float32) { resizes++ })
		}
		width := float32(620)
		if stage == "narrow" {
			width = 340
		}
		if stage == "tiny" {
			width = 180
		}
		tableA := wui.TableView(a, column("name", "Name", 160, 120, false), column("kind", "Kind", 180, 60, false), column("note", "Notes", 140, 40, true)).ID("a").Style("orders").MainWeight(0).MinWidth(width).MaxWidth(width).MinHeight(145.5).MaxHeight(145.5)
		tableB := wui.TableView(b, column("name", "Name", 100, 40, false), column("kind", "Kind", 100, 40, false)).ID("b").Style("test").MainWeight(0).MinWidth(360).MaxWidth(360).MinHeight(100.25).MaxHeight(100.25)
		button := func(id, title string, fn func()) ui.View { return ui.Button(title).ID(id).OnClick(fn) }
		change := func(value string) { stage = value; app.RequestUpdate() }
		content := ui.VBox(
			ui.HBox(button("narrow", "Narrow", func() { change("narrow") }), button("tiny", "Below min", func() { change("tiny") }), button("wide", "Wide", func() { change("wide") }), button("rebuild", "Rebuild", func() { generation++; app.RequestUpdate() }), button("palette", "Palette", func() { swapped = !swapped; app.RequestUpdate() }), button("finish", "Finish", finish)).Spacing(8),
			ui.Label("A: independent header Parts; fractional viewport"), tableA,
			ui.Label("B: one real row; blank space stays clear"), tableB,
			ui.Label("F6 checks — click A then End — Finish requires every stage"),
		).Spacing(12).Padding(16).CrossAlign(layout.CrossStart)
		return ui.Root().StyleSheet(sheet(dark, swapped)).Windows(ui.Window("main").Title("GOUI Table contracts").Size(820, 620).Content(content).
			Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(func() {
				if err := verify(); err != nil {
					fail(err)
				}
			}), ui.Shortcut(ui.KeyF8).OnActivate(finish)).
			OnAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
				if ready {
					return
				}
				// 首次原生尺寸到达前可能绘制零宽帧；按实际完整分配判断就绪。
				if info := lookup("a").Snapshot(); info.Bounds.Width != width || info.Bounds.X <= 0 {
					return
				}
				resource, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
				if err != nil {
					failure = err
					app.Post(app.Quit)
					return
				}
				defer resource.Destroy()
				original = lookup("a").Columns()
				ready = true
				bounds := map[string]geometry.Rectangle{}
				for _, id := range []string{"a", "b", "narrow", "tiny", "wide", "rebuild", "palette", "finish"} {
					bounds[id] = app.FindWidget("main", id).Snapshot().Bounds
				}
				data, _ := json.Marshal(bounds)
				fmt.Printf("READY backend=%T scale=%g dark=%t bounds=%s\n", resource, p.PixelScale(), dark, data)
			}))
	})
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	if !finished {
		return errors.New("closed before all table contracts were verified")
	}
	return nil
}
