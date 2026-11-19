// 表格内部线、数据轮廓、含滚动槽轮廓及整体外框的原生窗口回归，不使用DevServer。
//
// 环境：Linux/X11（记录WM）、Windows、macOS桌面；记录后端和实际缩放。
// 启动：go run ./tests/window/widgets/table_borders [-dark] [-basic]
// -bars=default/custom/hidden/always 选择初始配置；F4 循环切换同一组配置。
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
//     每种模式下按F9缩窄both的列，再按F6：仅剩纵条，表头分隔线的范围仍正确；
//     再按F9、F6恢复双条。切换模式前再次横向拖动，检查表头线左端没有缺口。
//     data／inner的表头线止于数据视口，不能突出到框外槽位；all须接到含槽轮廓。
//     outline／scroll已关闭分隔线，表头底部不得出现退化填充造成的残留细线。
//  4. 点击Rebuild和Dark，各按F6：列／单元格身份、滚动位置保留，样式与几何
//     按模式更新。Basic固定亮色。观察原生边线完整、表头对齐、滑块圆角和状态。
//  5. F8：要求五种模式、双向拖动、重建和亮暗检查都完成，PASS并退出0。
//  6. 每种框线模式下 F4/F6 循环全部配置：custom 的横条 10/6/3、纵条 20/8/4
//     DIP（轨道/滑块/间距），表头正文边界同步；间距不延伸网格或斑马色。
//     Hidden 仍可滚轮和键盘滚动，scroll 轮廓不包含不存在的槽位；Always 在
//     fit 中也占双槽，完整滑块无无效反馈。F8 要求每种框线覆盖全部四项配置。
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
	"math"
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
var barModes = []string{"default", "custom", "hidden", "always"}
var edge = color.RGBA{R: 90, G: 100, B: 110, A: 255}

func barOptions(mode string) (h, v ui.ScrollBarOptions) {
	switch mode {
	case "custom":
		h = ui.ScrollBarOptions{Thickness: 10, ThumbThickness: 6, Spacing: 3}
		v = ui.ScrollBarOptions{Thickness: 20, ThumbThickness: 8, Spacing: 4}
	case "hidden":
		h.Policy, v.Policy = ui.ScrollBarHidden, ui.ScrollBarHidden
	case "always":
		h.Policy, v.Policy = ui.ScrollBarAlways, ui.ScrollBarAlways
	}
	return
}

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
	bars := flag.String("bars", "default", "default/custom/hidden/always scrollbar options")
	flag.Parse()
	if err := run(*dark, *fallback, *bars); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run(dark, fallback bool, initialBars string) error {
	barMode := -1
	for i, name := range barModes {
		if name == initialBars {
			barMode = i
		}
	}
	if barMode < 0 {
		return fmt.Errorf("unknown scrollbar options %q", initialBars)
	}
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
	narrow := false
	ready, finished := false, false
	presented := ""
	seen := map[string]bool{}
	original := map[string][]*widgets.TableColumn{}
	var failure error
	err := ui.Run("org.golanggui.TableBorders", func(app ui.App) ui.RootView {
		// 标记本次声明所描述的帧，避免旧回调读取新状态后误报协调已经完成。
		frameState := fmt.Sprintf("mode=%s bars=%s generation=%d dark=%t narrow=%t", modes[mode], barModes[barMode], generation, dark, narrow)
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
				if (info.MaxScrollX > 0) != (id != "fit" && !(id == "both" && narrow)) || (info.MaxScrollY > 0) != (id == "both") {
					return fmt.Errorf("%s scroll ranges=%g,%g", id, info.MaxScrollX, info.MaxScrollY)
				}
				hOptions, vOptions := table.HScrollBarOptions(), table.VScrollBarOptions()
				hSlot := hOptions.Policy == gui.ScrollBarAlways || hOptions.Policy == gui.ScrollBarAuto && info.MaxScrollX > 0
				vSlot := vOptions.Policy == gui.ScrollBarAlways || vOptions.Policy == gui.ScrollBarAuto && info.MaxScrollY > 0
				vSpace := float32(0)
				if vSlot {
					vSpace = vOptions.Thickness + vOptions.Spacing
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
				// 检查整条表头线及其端点，不仅检查框角。使用公共快照推导视口，
				// 正线宽为2 DIP，在带内取样；关闭时允许两个背景色之间的AA混合，
				// 但任何越出背景通道范围的线色都失败，不能把幽灵细线当作有效网格。
				start := x + info.ScrollX
				end := info.Bounds.Width - start
				end -= vSpace
				lastHeader := info.Children[len(columns)-1].Bounds
				dataEnd := lastHeader.X + lastHeader.Width - info.Bounds.X
				lineEnd := min(end, dataEnd)
				if name == "all" && (id != "fit" || hSlot || vSlot) {
					lineEnd = info.Bounds.Width - start
				}
				lineY := y + header.Height - 1
				positive := name == "data" || name == "inner" || name == "all"
				headerInk, _ := gui.ResolveStyle("table-view", "header", style.Normal).BackgroundColor()
				headerColor := color.RGBAModel.Convert(headerInk).(color.RGBA)
				for px := int(math.Ceil(float64(start))); px < int(info.Bounds.Width-start); px++ {
					if positive && float32(px) < lineEnd {
						if err := check(float32(px), lineY, edge); err != nil {
							return err
						}
						continue
					}
					// 数据轮廓的竖边或紧包围竖边可以穿过当前扫描带。
					nearEdge := false
					for _, h := range info.Children[:len(columns)] {
						for _, bound := range []float32{h.Bounds.X - info.Bounds.X, h.Bounds.X + h.Bounds.Width - info.Bounds.X} {
							if math.Abs(float64(float32(px)-bound)) < 3 {
								nearEdge = true
							}
						}
					}
					if nearEdge {
						continue
					}
					got := pixel(float32(px), lineY)
					for channel, value := range []uint8{got.R, got.G, got.B} {
						lo := []uint8{background.R, background.G, background.B}[channel]
						hi := []uint8{headerColor.R, headerColor.G, headerColor.B}[channel]
						if lo > hi {
							lo, hi = hi, lo
						}
						if int(value) < int(lo)-1 || int(value) > int(hi)+1 {
							return fmt.Errorf("%s/%s unexpected header line at (%d,%g): %v", name, id, px, lineY, got)
						}
					}
				}
				if id == "fit" && !hSlot && !vSlot {
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
						right, bottom := float32(339)-inset, float32(219)-inset
						if !hSlot && !vSlot {
							if id == "fit" || id == "both" && narrow {
								right = start + 160 + 1
								if err := check(300, 150, background); err != nil {
									return err
								}
							}
							if id != "both" {
								last := info.Children[len(info.Children)-1].Bounds
								bottom = last.Y + last.Height - info.Bounds.Y + 1
							}
						}
						if err := check(right, 10, edge); err != nil {
							return err
						}
						if err := check(10, bottom, edge); err != nil {
							return err
						}
					}
					if id == "horizontal" {
						if err := check(20, 180, background); err != nil {
							return err
						}
					}
					if hSlot && vSlot {
						track, _ := gui.ResolveStyle("table-view", "trough", style.Normal).BackgroundColor()
						if err := check(340-start-vOptions.Thickness/2, 220-start-hOptions.Thickness/2, color.RGBAModel.Convert(track).(color.RGBA)); err != nil {
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
			seen[modes[mode]+":"+barModes[barMode]] = true
			if narrow {
				seen["narrow:"+modes[mode]] = true
			}
			if generation > 0 {
				seen["rebuild"] = true
			}
			bounds := map[string]geometry.Rectangle{}
			for _, id := range []string{"fit", "horizontal", "both", "next", "rebuild", "dark"} {
				bounds[id] = app.FindWidget("main", id).Snapshot().Bounds
			}
			data, _ := json.Marshal(bounds)
			fmt.Printf("CHECK mode=%s bars=%s generation=%d dark=%t narrow=%t bounds=%s\n", modes[mode], barModes[barMode], generation, dark, narrow, data)
			return nil
		}
		panels := make([]ui.View, 0, 3)
		for _, id := range []string{"fit", "horizontal", "both"} {
			width := float32(200)
			if id == "fit" || id == "both" && narrow {
				width = 80
			}
			column := func(key string) *wui.TableColumnView[string] {
				return wui.TableColumn(key, key, func(_ widgets.TableRow, text string) ui.View { return ui.Label(text) }).Width(width).MinWidth(width).Resizable(false)
			}
			h, v := barOptions(barModes[barMode])
			table := wui.TableView(models[id], column("Name"), column("Value")).HScrollBarOptions(h).VScrollBarOptions(v).ID(id).MainWeight(0).MinWidth(340).MaxWidth(340).MinHeight(220).MaxHeight(220)
			panels = append(panels, ui.VBox(ui.Label(id), table).Spacing(8).CrossAlign(layout.CrossStart))
		}
		content := ui.VBox(ui.HBox(
			ui.Button("Next").ID("next").OnClick(func() { mode = (mode + 1) % len(modes); app.RequestUpdate() }),
			ui.Button("Rebuild").ID("rebuild").OnClick(func() { generation++; app.RequestUpdate() }),
			ui.Button("Dark").ID("dark").OnClick(func() { dark = !dark; seen["palette"] = true; app.RequestUpdate() }),
			ui.Label(modes[mode]+" / bars: "+barModes[barMode]+" (F4)"),
		).Spacing(8), ui.HBox(panels...).Spacing(12), ui.Label("F6 checks; drag both scrollbars then F7; Next + F6 through all modes; Rebuild/Dark + F6; F8 finish")).Spacing(12).Padding(16).CrossAlign(layout.CrossStart)
		return ui.Root().StyleSheet(sheet(modes[mode], dark, fallback)).Windows(ui.Window("main").Title("GOUI Table borders").Size(1120, 420).Content(content).Shortcuts(
			ui.Shortcut(ui.KeyF4).OnActivate(func() { barMode = (barMode + 1) % len(barModes); app.RequestUpdate() }),
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
			ui.Shortcut(ui.KeyF9).OnActivate(func() { narrow = !narrow; app.RequestUpdate() }),
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
				for _, name := range modes {
					for _, bars := range barModes {
						if !seen[name+":"+bars] {
							fail(fmt.Errorf("missing stage %s:%s", name, bars))
							return
						}
					}
					if !seen["narrow:"+name] {
						fail(fmt.Errorf("missing narrow stage %s", name))
						return
					}
				}
				finished = true
				fmt.Println("PASS table borders")
				app.Quit()
			}),
		).OnAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
			state := frameState
			if ready {
				if state != presented {
					presented = state
					fmt.Println("PRESENT", state)
				}
				return
			}
			if info := lookup("fit").Snapshot(); info.Bounds.Width != 340 || info.Bounds.X <= 0 {
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
			presented = state
			fmt.Printf("READY backend=%T scale=%g bounds=%s\n", resource, p.PixelScale(), data)
			fmt.Println("PRESENT", state)
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
