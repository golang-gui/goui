// 表格内部线、数据轮廓、含滚动槽轮廓及整体外框的原生窗口回归，不使用DevServer。
//
// 环境：Linux/X11（记录WM）、Windows、macOS桌面；记录后端和实际缩放。
// 启动：go run ./tests/window/widgets/table_borders [-dark] [-basic]
// 可设置GOUI_PLAT_PAINTER=software/opengl/direct2d、GOUI_PLAT_SCALE=1/2。
// 初始：1120×420 DIP窗口，三张340×220 DIP表；fit仅两行且无滚动条，horizontal
// 仅两行且横滚，both二十行且双轴滚动。列禁止用户调整，模式data。
//
// 操作与逐项预期：
//  1. 等READY，F6：data模式保留现有Grid效果，数据四边闭合，滚动条在框外；
//     fit实际数据右侧／下方空白没有网格或斑马色。
//  2. 先拖both的纵向滑块向下24 DIP，再拖横向滑块向右24 DIP，F7：两个方向
//     都实际滚动，没有选中或激活业务行。horizontal的横条始终固定在视口底部。
//  3. 点击Next依次切到inner、outline、scroll、all，每次按F6：inner没有数据
//     外边线；outline仅包数据且没有内部网格；scroll仅画包含滚动槽的紧包围
//     轮廓；all再绘内部线及整个TableView外框。fit轮廓紧包数据，horizontal／both
//     轮廓包含固定槽位；数据到条之间的空白不画网格／斑马色；交汇角与轨道同色。
//  4. 点击Rebuild和Dark，各按F6：列／单元格身份、滚动位置保留，样式与几何
//     按模式更新。Basic固定亮色。观察原生边线完整、表头对齐、滑块圆角和状态。
//  5. F8：要求五种模式、双向拖动、重建和亮暗检查都完成，PASS并退出0。
//
// F6通过公开快照和RenderWidget断言几何及像素，不替代上述原生外观观察。
// 平台差异：macOS可能需要Fn+功能键；拖动距离按实际PixelScale换成物理像素。
// 副作用：仅窗口、内存与终端日志，不改剪贴板、系统设置或文件。
// 复位：重启。提前关闭退出1，完成F8由UI清理窗口。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

var modes = []string{"data", "inner", "outline", "scroll", "all"}
var edge = color.RGBA{R: 90, G: 100, B: 110, A: 255}

type rows struct{ *gui.SliceListModel[string] }

func (r *rows) RowID(i int) string { return fmt.Sprint(i) }

func sheet(mode string, dark, fallback bool) style.StyleSheet {
	rules := modern.Rules(modern.Options{Dark: dark})
	if fallback {
		rules = basic.Rules()
	}
	inner, data, outline, outer := float32(0), float32(0), float32(0), float32(0)
	switch mode {
	case "data":
		inner, data = 2, 2
	case "inner":
		inner = 2
	case "outline":
		data = 2
	case "scroll":
		outline = 2
	case "all":
		inner, outline, outer = 2, 2, 2
	}
	for _, p := range []struct {
		part  string
		width float32
	}{{"grid-horizontal", inner}, {"grid-vertical", inner}, {"grid-top", data}, {"grid-bottom", data}, {"grid-left", data}, {"grid-right", data}, {"outline-horizontal", outline}, {"outline-vertical", outline}, {"outer-horizontal", outer}, {"outer-vertical", outer}} {
		rules = append(rules, style.Name("table-view").Part(p.part).BorderWidth(p.width).BorderColor(edge))
	}
	rules = append(rules,
		style.Name("table-view").Part("header-separator").BorderWidth(inner).ForegroundColor(edge),
		style.Name("table-view").Part("row-alternate").BackgroundColor(color.RGBA{R: 160, G: 170, B: 180, A: 255}).Radius(0),
	)
	return style.Sheet(rules...)
}

func main() {
	dark, fallback := flag.Bool("dark", false, "dark Modern palette"), flag.Bool("basic", false, "Basic fallback theme")
	flag.Parse()
	if err := run(*dark, *fallback); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run(dark, fallback bool) error {
	models := map[string]*rows{}
	for _, id := range []string{"fit", "horizontal", "both"} {
		n := 2
		if id == "both" {
			n = 20
		}
		values := make([]string, n)
		for i := range values {
			values[i] = fmt.Sprintf("Row %02d", i)
		}
		models[id] = &rows{gui.NewSliceListModel(values)}
	}
	mode, generation := 0, 0
	ready, finished := false, false
	seen := map[string]bool{}
	original := map[string][]*widgets.TableColumn{}
	var failure error
	err := ui.Run("org.golanggui.TableBorders", func(app ui.App) ui.RootView {
		lookup := func(id string) *widgets.TableView { return app.FindWidget("main", id).(*widgets.TableView) }
		fail := func(err error) {
			if failure == nil {
				failure = err
			}
			app.Quit()
		}
		verify := func() error {
			if !ready {
				return errors.New("window is not ready")
			}
			name := modes[mode]
			for _, id := range []string{"fit", "horizontal", "both"} {
				table := lookup(id)
				info := table.Snapshot()
				columns := table.Columns()
				if columns[0] != original[id][0] || columns[1] != original[id][1] || info.ItemCount != models[id].ItemsCount() {
					return errors.New("column identity or row count changed")
				}
				if (info.MaxScrollX > 0) != (id != "fit") || (info.MaxScrollY > 0) != (id == "both") {
					return fmt.Errorf("%s scroll ranges=%g,%g", id, info.MaxScrollX, info.MaxScrollY)
				}
				img, err := gui.RenderWidget(table, 1)
				if err != nil {
					return err
				}
				pixel := func(x, y float32) color.RGBA { return color.RGBAModel.Convert(img.At(int(x), int(y))).(color.RGBA) }
				check := func(x, y float32, want color.RGBA) error {
					if got := pixel(x, y); got != want {
						return fmt.Errorf("%s/%s pixel(%g,%g)=%v want=%v", name, id, x, y, got, want)
					}
					return nil
				}
				bg, _ := gui.ResolveStyle("table-view", "", style.Normal).BackgroundColor()
				background := color.RGBAModel.Convert(bg).(color.RGBA)
				header := info.Children[0].Bounds
				x, y := header.X-info.Bounds.X, header.Y-info.Bounds.Y
				if id == "fit" {
					last := info.Children[len(info.Children)-1].Bounds
					bottom := last.Y + last.Height - info.Bounds.Y
					headerBG, _ := gui.ResolveStyle("table-view", "header", style.Normal).BackgroundColor()
					headerBackground := color.RGBAModel.Convert(headerBG).(color.RGBA)
					wantData, wantOutline := headerBackground, background
					if name == "data" || name == "outline" {
						wantData = edge
					}
					if name == "scroll" || name == "all" {
						wantOutline = edge
					}
					if err := check(x+159, y+8, wantData); err != nil {
						return err
					}
					if err := check(x+80-1, y+8, map[bool]color.RGBA{true: edge, false: headerBackground}[name == "data" || name == "inner" || name == "all"]); err != nil {
						return err
					}
					if name == "scroll" || name == "all" {
						if err := check(x+160, y+8, wantOutline); err != nil {
							return err
						}
						if err := check(x+12, bottom+1, wantOutline); err != nil {
							return err
						}
					}
					if err := check(300, 150, background); err != nil {
						return err
					}
					if err := check(20, 190, background); err != nil {
						return err
					}
				} else {
					inset := float32(0)
					if name == "all" {
						inset = 2
					}
					if name == "scroll" || name == "all" {
						if err := check(339-inset, 10, edge); err != nil {
							return err
						}
						if err := check(10, 219-inset, edge); err != nil {
							return err
						}
					}
					if id == "horizontal" {
						if err := check(20, 180, background); err != nil {
							return err
						}
					}
					if id == "both" {
						track, _ := gui.ResolveStyle("table-view", "trough", style.Normal).BackgroundColor()
						if err := check(335-(x+info.ScrollX), 215-y, color.RGBAModel.Convert(track).(color.RGBA)); err != nil {
							return err
						}
					}
				}
				for _, row := range info.Children[2:] {
					for i, cell := range row.Children {
						if cell.Bounds.X != info.Children[i].Bounds.X || cell.Bounds.Width != info.Children[i].Bounds.Width {
							return errors.New("header/body alignment")
						}
					}
				}
			}
			seen[modes[mode]] = true
			if generation > 0 {
				seen["rebuild"] = true
			}
			bounds := map[string]geometry.Rectangle{}
			for _, id := range []string{"fit", "horizontal", "both", "next", "rebuild", "dark"} {
				bounds[id] = app.FindWidget("main", id).Snapshot().Bounds
			}
			data, _ := json.Marshal(bounds)
			fmt.Printf("CHECK mode=%s generation=%d dark=%t bounds=%s\n", modes[mode], generation, dark, data)
			return nil
		}
		panels := make([]ui.View, 0, 3)
		for _, id := range []string{"fit", "horizontal", "both"} {
			width := float32(200)
			if id == "fit" {
				width = 80
			}
			column := func(key string) *wui.TableColumnView[string] {
				return wui.TableColumn(key, key, func(_ widgets.TableRow, text string) ui.View { return ui.Label(text) }).Width(width).MinWidth(width).Resizable(false)
			}
			table := wui.TableView(models[id], column("Name"), column("Value")).ID(id).MainWeight(0).MinWidth(340).MaxWidth(340).MinHeight(220).MaxHeight(220)
			panels = append(panels, ui.VBox(ui.Label(id), table).Spacing(8).CrossAlign(layout.CrossStart))
		}
		content := ui.VBox(ui.HBox(
			ui.Button("Next").ID("next").OnClick(func() { mode = (mode + 1) % len(modes); app.RequestUpdate() }),
			ui.Button("Rebuild").ID("rebuild").OnClick(func() { generation++; app.RequestUpdate() }),
			ui.Button("Dark").ID("dark").OnClick(func() { dark = !dark; seen["palette"] = true; app.RequestUpdate() }),
			ui.Label(modes[mode]),
		).Spacing(8), ui.HBox(panels...).Spacing(12), ui.Label("F6 checks; drag both scrollbars then F7; Next + F6 through all modes; Rebuild/Dark + F6; F8 finish")).Spacing(12).Padding(16).CrossAlign(layout.CrossStart)
		return ui.Root().StyleSheet(sheet(modes[mode], dark, fallback)).Windows(ui.Window("main").Title("GOUI Table borders").Size(1120, 420).Content(content).Shortcuts(
			ui.Shortcut(ui.KeyF6).OnActivate(func() {
				if err := verify(); err != nil {
					fail(err)
				}
			}),
			ui.Shortcut(ui.KeyF7).OnActivate(func() {
				info := lookup("both").Snapshot()
				if info.ScrollX <= 0 || info.ScrollY <= 0 || lookup("both").Current() != "" || len(lookup("both").Selection()) != 0 {
					fail(errors.New("scrollbar drag or row selection"))
					return
				}
				seen["drag"] = true
				fmt.Println("CHECK drag")
			}),
			ui.Shortcut(ui.KeyF8).OnActivate(func() {
				if err := verify(); err != nil {
					fail(err)
					return
				}
				for _, key := range append(append([]string{}, modes...), "drag", "rebuild", "palette") {
					if !seen[key] {
						fail(fmt.Errorf("missing stage %s", key))
						return
					}
				}
				finished = true
				fmt.Println("PASS table borders")
				app.Quit()
			}),
		).OnAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
			if info := lookup("fit").Snapshot(); ready || info.Bounds.Width != 340 || info.Bounds.X <= 0 {
				return
			}
			resource, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
			if err != nil {
				failure = err
				app.Post(app.Quit)
				return
			}
			defer resource.Destroy()
			bounds := map[string]geometry.Rectangle{}
			for _, id := range []string{"fit", "horizontal", "both"} {
				original[id] = lookup(id).Columns()
				bounds[id] = lookup(id).Snapshot().Bounds
				bounds[id+"-header"] = lookup(id).Snapshot().Children[0].Bounds
			}
			for _, id := range []string{"next", "rebuild", "dark"} {
				bounds[id] = app.FindWidget("main", id).Snapshot().Bounds
			}
			data, _ := json.Marshal(bounds)
			ready = true
			fmt.Printf("READY backend=%T scale=%g bounds=%s\n", resource, p.PixelScale(), data)
		}))
	})
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	if !finished {
		return errors.New("closed before verification completed")
	}
	return nil
}
