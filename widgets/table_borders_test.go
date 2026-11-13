package widgets

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"
)

func tableBorderRules() []style.Rule {
	rules := []style.Rule{style.Name("table-view").BackgroundColor(color.Transparent).BorderWidth(0)}
	for _, part := range []string{"frame", "header", "header-separator", "row", "row-alternate", "grid-horizontal", "grid-vertical", "grid-top", "grid-bottom", "grid-left", "grid-right", "outline-horizontal", "outline-vertical"} {
		rules = append(rules, style.Name("table-view").Part(part).BackgroundColor(color.Transparent).BorderWidth(0).Radius(0))
	}
	return rules
}

func tableBorderApp(t *testing.T, rules ...style.Rule) {
	t.Helper()
	old := gui.App
	gui.App = &separatorApp{sheet: style.Sheet(append(tableBorderRules(), rules...)...)}
	t.Cleanup(func() { gui.App = old })
}

// 内部网格与数据四边分别占线带；sRGB预乘RGBA，1x/2x整数色带允许1级量化误差。
func TestTableDataOutlineIndependentOfInternalGrid(t *testing.T) {
	ink := color.NRGBA{R: 255, A: 128}
	tableBorderApp(t,
		style.Name("table-view").Part("grid-top").BorderWidth(4).BorderColor(ink),
		style.Name("table-view").Part("grid-bottom").BorderWidth(2).BorderColor(ink),
		style.Name("table-view").Part("grid-left").BorderWidth(5).BorderColor(ink),
		style.Name("table-view").Part("grid-right").BorderWidth(3).BorderColor(ink),
	)
	v, _, first, _ := tableFixture(2)
	defer v.SetModel(nil)
	first.SetWidth(80)
	last := NewTableColumn("last", "Last")
	last.SetWidth(60)
	v.SetColumns(first, last)
	tableArrange(v, 240, 180)
	row, end := v.realized["r0"], v.realized["r1"]
	if row.Rect().Height != 32 || end.Rect().Height != 34 || row.cells[0].content.Rect().Width != 75 || row.cells[1].Rect().Width-v.columnLineWidth(last) != 57 {
		t.Fatalf("独立轮廓未预留线带：first=%v last=%v first-content=%v last-line=%g", row.Rect(), end.Rect(), row.cells[0].content.Rect(), v.columnLineWidth(last))
	}
	for _, scale := range []float32{1, 2} {
		for _, r := range []*tableRow{row, end} {
			pixels := tablePixels(t, r.Rect().Size, scale, func(p *tablePixelPainter) { r.Paint(p) })
			for _, sample := range []struct {
				x, y int
				want color.RGBA
			}{{0, 1, color.RGBA{R: 128, A: 128}}, {139, 1, color.RGBA{R: 128, A: 128}}, {79, 10, color.RGBA{}}} {
				checkTableEdgePixel(t, pixels.RGBAAt(sample.x*int(scale), sample.y*int(scale)), sample.want)
			}
			bottom := color.RGBA{}
			if r == end {
				bottom = color.RGBA{R: 128, A: 128}
			}
			checkTableEdgePixel(t, pixels.RGBAAt(20*int(scale), (int(r.Rect().Height)-1)*int(scale)), bottom)
			checkTableEdgePixel(t, pixels.RGBAAt(139*int(scale), (int(r.Rect().Height)-1)*int(scale)), color.RGBA{R: 128, A: 128})
		}
	}
}

func TestTableInternalGridDoesNotCreateDataOutline(t *testing.T) {
	tableBorderApp(t,
		style.Name("table-view").Part("grid-horizontal").BorderWidth(2).BorderColor(color.Black),
		style.Name("table-view").Part("grid-vertical").BorderWidth(3).BorderColor(color.Black),
	)
	v, _, first, _ := tableFixture(2)
	defer v.SetModel(nil)
	first.SetWidth(80)
	last := NewTableColumn("last", "Last")
	last.SetWidth(60)
	v.SetColumns(first, last)
	tableArrange(v, 240, 180)
	row, end := v.realized["r0"], v.realized["r1"]
	if row.Rect().Height != 34 || end.Rect().Height != 32 || v.columnLineWidth(first) != 3 || v.columnLineWidth(last) != 0 {
		t.Fatal("内部网格仍兼任末行或末列的外边线")
	}
	pixels := tablePixels(t, row.Rect().Size, 1, func(p *tablePixelPainter) { row.Paint(p) })
	if pixels.RGBAAt(79, 10) != (color.RGBA{A: 255}) || pixels.RGBAAt(139, 10) != (color.RGBA{}) {
		t.Fatal("内部竖线或无轮廓末列的像素错误")
	}
	// 空表的表头底边也由数据轮廓决定，不能借用内部表头分隔线冒充。
	v.SetModel(nil)
	tableArrange(v, 240, 180)
	if v.headers[first].Rect().Height != 32 {
		t.Fatal("空表保留了内部横线占位")
	}
}

func TestTableOutlineWithScrollSlots(t *testing.T) {
	for _, tc := range []struct {
		name          string
		rows          int
		column        float32
		wantX, wantY  bool
		width, height float32
	}{
		{"无条紧包数据", 2, 70, false, false, 148, 102},
		{"仅横条仍在底部", 2, 140, true, false, 240, 180},
		{"仅纵条", 20, 70, false, true, 240, 180},
		{"双条", 20, 140, true, true, 240, 180},
		{"空表仅表头", 0, 70, false, false, 148, 38},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tableBorderApp(t,
				style.Name("table-view").Part("outline-horizontal").BorderWidth(3).BorderColor(color.NRGBA{R: 255, A: 128}),
				style.Name("table-view").Part("outline-vertical").BorderWidth(4).BorderColor(color.NRGBA{R: 255, A: 128}),
			)
			v, _, first, _ := tableFixture(tc.rows)
			defer v.SetModel(nil)
			first.SetMinWidth(tc.column)
			first.SetWidth(tc.column)
			last := NewTableColumn("last", "Last")
			last.SetMinWidth(tc.column)
			last.SetWidth(tc.column)
			v.SetColumns(first, last)
			tableArrange(v, 240, 180)
			info := v.Snapshot()
			if (info.MaxScrollX > 0) != tc.wantX || (info.MaxScrollY > 0) != tc.wantY || v.headerViewport.Rect().X != 4 || v.headerViewport.Rect().Y != 3 {
				t.Fatalf("轮廓未参与布局或滚动条显隐错误：header=%v maxScroll=%g,%g", v.headerViewport.Rect(), info.MaxScrollX, info.MaxScrollY)
			}
			if tc.wantX {
				for _, child := range v.scroll.Children() {
					if bar, ok := child.(*gui.ScrollBar); ok && bar.Visible() && bar.Rect().Width > bar.Rect().Height {
						if v.scroll.Rect().Y+bar.Rect().Y+bar.Rect().Height != 177 {
							t.Fatal("横向滚动条被移到短数据尾部")
						}
					}
				}
			}
			for _, scale := range []float32{1, 2} {
				pixels := tablePixels(t, v.Rect().Size, scale, func(p *tablePixelPainter) { v.Paint(p) })
				for _, point := range []geometry.Point{{X: 1, Y: 1}, {X: tc.width - 1, Y: 10}, {X: 12, Y: tc.height - 1}, {X: tc.width - 1, Y: tc.height - 1}} {
					checkTableEdgePixel(t, pixels.RGBAAt(int(point.X*scale), int(point.Y*scale)), color.RGBA{R: 128, A: 128})
				}
				if !tc.wantX && !tc.wantY && pixels.RGBAAt(200*int(scale), 140*int(scale)) != (color.RGBA{}) {
					t.Fatal("紧包围轮廓错误扩展到整个空白视口")
				}
			}
			if info.ItemCount != tc.rows || len(info.Children) != len(v.visibleColumns)+len(v.realized) {
				t.Fatal("轮廓或滚动槽产生了伪造的语义行列")
			}
		})
	}
}

func TestTableDataOutlineTracksIncrementalLastRow(t *testing.T) {
	tableBorderApp(t,
		style.Name("table-view").Part("grid-horizontal").BorderWidth(2),
		style.Name("table-view").Part("grid-bottom").BorderWidth(8),
	)
	v, model, column, _ := tableFixture(2)
	defer v.SetModel(nil)
	tableArrange(v, 240, 240)
	oldLast := v.realized["r1"]
	content := oldLast.cells[0].content
	model.Modify(func(rows []tableRecord) []tableRecord { return append(rows, tableRecord{"r2", 60, 20}) })
	tableArrange(v, 240, 240)
	if v.realized["r1"] != oldLast || oldLast.cells[0].content != content || oldLast.Rect().Height != 34 || v.realized["r2"].Rect().Height != 40 {
		t.Fatal("追加记录未迁移末行线带，或替换了原单元格")
	}
	model.Modify(func(rows []tableRecord) []tableRecord { return rows[:2] })
	tableArrange(v, 240, 240)
	if oldLast.Rect().Height != 40 || oldLast.cells[0].content != content {
		t.Fatal("删除末行未恢复新末行的轮廓线带")
	}
	v.SetModel(nil)
	tableArrange(v, 240, 240)
	if v.headers[column].Rect().Height != 40 {
		t.Fatal("清空数据未把独立底边迁移到表头")
	}
}

func TestTableOutlineCornerAndZebraStayInTheirAreas(t *testing.T) {
	tableBorderApp(t,
		style.Name("table-view").Part("outline-horizontal").BorderWidth(2).BorderColor(color.NRGBA{B: 255, A: 128}),
		style.Name("table-view").Part("outline-vertical").BorderWidth(2).BorderColor(color.NRGBA{B: 255, A: 128}),
		style.Name("table-view").Part("trough").BackgroundColor(color.NRGBA{R: 255, A: 128}),
		style.Name("table-view").Part("row-alternate").BackgroundColor(color.NRGBA{G: 255, A: 128}),
	)
	v, _, first, _ := tableFixture(20)
	defer v.SetModel(nil)
	first.SetMinWidth(160)
	first.SetWidth(160)
	last := NewTableColumn("last", "Last")
	last.SetMinWidth(160)
	last.SetWidth(160)
	v.SetColumns(first, last)
	tableArrange(v, 240, 180)
	for _, scale := range []float32{1, 2} {
		pixels := tablePixels(t, v.Rect().Size, scale, func(p *tablePixelPainter) { v.Paint(p) })
		checkTableEdgePixel(t, pixels.RGBAAt(235*int(scale), 175*int(scale)), color.RGBA{R: 128, A: 128})
		checkTableEdgePixel(t, pixels.RGBAAt(239*int(scale), 179*int(scale)), color.RGBA{B: 128, A: 128})
	}
	// 关闭溢出后，斑马色只覆盖最后实际列；轮廓外的视口空白保持透明。
	first.SetMinWidth(40)
	first.SetWidth(60)
	last.SetMinWidth(40)
	last.SetWidth(60)
	v.model.(*tableTestModel).Modify(func(rows []tableRecord) []tableRecord { return rows[:2] })
	tableArrange(v, 240, 180)
	row := v.realized["r1"]
	pixels := tablePixels(t, row.Rect().Size, 1, func(p *tablePixelPainter) { row.Paint(p) })
	checkTableEdgePixel(t, pixels.RGBAAt(10, 10), color.RGBA{G: 128, A: 128})
	checkTableEdgePixel(t, pixels.RGBAAt(160, 10), color.RGBA{})
	if v.Snapshot().MaxScrollX != 0 || v.Snapshot().MaxScrollY != 0 {
		t.Fatal("数据缩小后滚动槽没有释放")
	}
}

func TestTableOutlineExtremeAllocation(t *testing.T) {
	tableBorderApp(t,
		style.Name("table-view").Part("outline-horizontal").BorderWidth(12),
		style.Name("table-view").Part("outline-vertical").BorderWidth(20),
	)
	v, _, _, _ := tableFixture(3)
	defer v.SetModel(nil)
	for _, size := range []geometry.Size{{}, {Width: 1, Height: 1}, {Width: 40, Height: 10}} {
		tableArrange(v, size.Width, size.Height)
		for _, rect := range []geometry.Rectangle{v.headerViewport.Rect(), v.scroll.Rect()} {
			if rect.Width < 0 || rect.Height < 0 || rect.X < 0 || rect.Y < 0 || rect.X+rect.Width > size.Width || rect.Y+rect.Height > size.Height {
				t.Fatalf("%s得到越界分配：%v", fmt.Sprint(size), rect)
			}
		}
	}
}
