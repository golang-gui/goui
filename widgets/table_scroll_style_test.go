package widgets

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

func TestTableBarOptionsGeometryAndInput(t *testing.T) {
	tableBorderApp(t)
	v, _, first, _ := tableFixture(20)
	defer v.SetModel(nil)
	last := NewTableColumn("last", "Last")
	for _, c := range []*TableColumn{first, last} {
		c.SetMinWidth(140)
		c.SetWidth(140)
	}
	v.SetColumns(first, last)
	notifications := 0
	first.ConnectResize(func(float32) { notifications++ })
	last.ConnectResize(func(float32) { notifications++ })
	h := gui.ScrollBarOptions{Thickness: 10, ThumbThickness: 6, Spacing: 3}
	vertical := gui.ScrollBarOptions{Thickness: 20, ThumbThickness: 8, Spacing: 4}
	v.SetHScrollBarOptions(h)
	v.SetVScrollBarOptions(vertical)
	tableArrange(v, 240, 180)
	if v.HScrollBarOptions() != (gui.ScrollBarOptions{Thickness: 10, ThumbThickness: 6, Spacing: 3}) || v.VScrollBarOptions() != vertical {
		t.Fatal("表格没有转发配置")
	}
	want := gui.ScrollGeometry{Viewport: geometry.Rect(0, 0, 216, 135), HScrollBar: geometry.Rect(0, 138, 216, 10), VScrollBar: geometry.Rect(220, 0, 20, 135), Corner: geometry.Rect(220, 138, 20, 10)}
	if g := v.scroll.ScrollGeometry(); g != want || v.headerViewport.Rect() != geometry.Rect(0, 0, 216, 32) {
		t.Fatalf("自定义槽位造成表头／正文不齐：geometry=%+v header=%v", g, v.headerViewport.Rect())
	}
	for _, row := range v.realized {
		for i, cell := range row.cells {
			if cell.Rect().X != v.headers[v.visibleColumns[i]].Rect().X || cell.Rect().Width != 140 {
				t.Fatal("单元格没有沿用表头的实际列几何")
			}
		}
	}
	v.SetSelection([]string{"r0"})
	v.SetCurrent("r0")
	d, host := new(gui.EventDispatcher), &tabInputHost{root: v}
	// 正文右侧间距：视口止于216，轨道始于220，不能清选、排序或激活。
	dispatchTabPointer(t, d, host, events.PointerDown, 218, 50)
	dispatchTabPointer(t, d, host, events.PointerUp, 218, 50)
	if len(v.Selection()) != 1 || v.Current() != "r0" {
		t.Fatal("间距被当成正文点击")
	}
	v.SetVScrollBarOptions(gui.ScrollBarOptions{Policy: gui.ScrollBarHidden})
	tableArrange(v, 240, 180)
	if v.scroll.ScrollGeometry().VScrollBar != (geometry.Rectangle{}) || v.scroll.Snapshot().MaxScrollY == 0 || first.Width() != 140 || last.Width() != 140 || notifications != 0 {
		t.Fatal("隐藏内置条改变了基础列宽／通知，或禁止了滚动")
	}
}

func tableVerticalBar(t *testing.T, v *TableView) *gui.ScrollBar {
	t.Helper()
	for _, child := range v.scroll.Children() {
		if b, ok := child.(*gui.ScrollBar); ok && b.Orientation() == layout.DirectionVertical {
			return b
		}
	}
	t.Fatal("缺少内部纵向ScrollBar")
	return nil
}

func TestTableScrollStyleNameFollowsOwner(t *testing.T) {
	old := gui.App
	want := map[string]color.RGBA{
		"table-view": {R: 200, A: 255},
		"orders":     {G: 200, A: 255},
		"test":       {B: 200, A: 255},
	}
	var rules []style.Rule
	for name, ink := range want {
		rules = append(rules, style.Name(name).Part("corner").BackgroundColor(ink).BorderWidth(0).Radius(0))
	}
	rules = append(rules, style.Name("scroll-view").Part("corner").BackgroundColor(color.RGBA{R: 255, B: 255, A: 255}))
	gui.App = &separatorApp{sheet: style.Sheet(rules...)}
	t.Cleanup(func() { gui.App = old })
	v, _, column, _ := tableFixture(100)
	defer v.SetModel(nil)
	column.SetMinWidth(300)
	column.SetWidth(300)
	for _, name := range []string{"table-view", "orders", "test", "table-view"} {
		if name == "table-view" {
			v.SetStyleName("")
		} else {
			v.SetStyleName(name)
		}
		tableArrange(v, 240, 180)
		if v.scroll.StyleName() != name || tableVerticalBar(t, v).StyleName() != name {
			t.Fatalf("内部滚动部件未随所属表格切换：scroll=%q bar=%q want=%q", v.scroll.StyleName(), tableVerticalBar(t, v).StyleName(), name)
		}
		corner := v.scroll.ScrollGeometry().Corner
		if corner.Width <= 0 || corner.Height <= 0 {
			t.Fatal("未建立双轴交汇角")
		}
		pixels := tablePixels(t, v.scroll.Rect().Size, 1, func(p *tablePixelPainter) { v.scroll.Paint(p) })
		got := pixels.RGBAAt(int(corner.X+corner.Width/2), int(corner.Y+corner.Height/2))
		if got != want[name] {
			t.Fatalf("corner未使用所属样式或未随Name切换：name=%s got=%v want=%v", name, got, want[name])
		}
	}
}

// sRGB预乘RGBA；整数色带在1x/2x满覆盖，半透明合成只允许1级8位量化误差。
func TestTableHeaderScrollGapPixels(t *testing.T) {
	old := gui.App
	gui.App = &separatorApp{sheet: style.Sheet(
		style.Name("table-view").BackgroundColor(color.Transparent).BorderWidth(0),
		style.Name("table-view").Part("frame").BackgroundColor(color.Transparent).BorderWidth(0).Radius(0),
		// 本组验证有包围框时的槽位补画；框外范围在独立端点回归中覆盖。
		style.Name("table-view").Part("outline-vertical").BorderWidth(1),
		style.Name("table-view").Part("header").BackgroundColor(color.NRGBA{R: 255, A: 128}).BorderWidth(0),
		style.Name("table-view").Part("header-separator").ForegroundColor(color.NRGBA{B: 255, A: 128}).BorderWidth(2),
	)}
	t.Cleanup(func() { gui.App = old })
	v, _, _, _ := tableFixture(100)
	defer v.SetModel(nil)
	tableArrange(v, 240, 180)
	header := v.headerViewport.Rect()
	x := int(header.X+header.Width) + 2
	if x >= 240 {
		t.Fatal("没有建立纵向滚动槽空位")
	}
	for _, scale := range []float32{1, 2} {
		pixels := tablePixels(t, v.Rect().Size, scale, func(p *tablePixelPainter) { v.Paint(p) })
		for _, sample := range []struct {
			x, y int
			want color.RGBA
		}{
			{x, 2, color.RGBA{R: 128, A: 128}},
			{x, int(header.Height) - 1, color.RGBA{R: 64, B: 128, A: 192}},
			{12, 12, color.RGBA{}},                    // 已有表头由其自身绘制，父控件不得重复铺底。
			{x, int(header.Height) + 4, color.RGBA{}}, // 不把表头背景伸入表体。
		} {
			checkTableEdgePixel(t, pixels.RGBAAt(sample.x*int(scale), sample.y*int(scale)), sample.want)
		}
	}
	requests := 0
	v.columns[0].SetSortable(true)
	v.ConnectSortRequest(func(string, SortOrder) { requests++ })
	d, host := new(gui.EventDispatcher), &tabInputHost{root: v}
	dispatchTabPointer(t, d, host, events.PointerDown, float32(x), 4)
	dispatchTabPointer(t, d, host, events.PointerUp, float32(x), 4)
	if requests != 0 || v.Current() != "" || len(v.Selection()) != 0 {
		t.Fatal("表头装饰补位区被当成列或内容行")
	}
	// 无需纵向滚动时不虚构补位区，也不扩展实际单元格边线。
	v.SetModel(nil)
	tableArrange(v, 240, 180)
	pixels := tablePixels(t, v.Rect().Size, 1, func(p *tablePixelPainter) { v.Paint(p) })
	if pixels.RGBAAt(238, 4) != (color.RGBA{}) {
		t.Fatal("无纵向滚动仍绘制补位区")
	}
}

func TestTableScrollBarUsesOwnerPartsAndPointerStates(t *testing.T) {
	old := gui.App
	trough, normal, hover, pressed := color.RGBA{R: 230, G: 230, B: 230, A: 255}, color.RGBA{R: 80, A: 255}, color.RGBA{G: 80, A: 255}, color.RGBA{B: 80, A: 255}
	gui.App = &separatorApp{sheet: style.Sheet(
		style.Name("orders").BackgroundColor(color.White).BorderWidth(0),
		style.Name("orders").Part("frame").BackgroundColor(color.Transparent).BorderWidth(0).Radius(0),
		style.Name("orders").Part("trough").BackgroundColor(trough).BorderWidth(0).Radius(0),
		style.Name("orders").Part("thumb").BackgroundColor(normal).BorderWidth(0).Radius(2),
		style.Name("orders").Part("thumb").State(style.Hovered).BackgroundColor(hover),
		style.Name("orders").Part("thumb").State(style.Pressed).BackgroundColor(pressed),
		style.Name("test").BackgroundColor(color.White).BorderWidth(0),
		style.Name("test").Part("frame").BackgroundColor(color.Transparent).BorderWidth(0).Radius(0),
		style.Name("test").Part("trough").BackgroundColor(color.RGBA{G: 200, A: 255}).BorderWidth(0).Radius(0),
		style.Name("scroll-view").Part("trough").BackgroundColor(color.RGBA{R: 255, B: 255, A: 255}),
	)}
	t.Cleanup(func() { gui.App = old })
	v, _, _, _ := tableFixture(100)
	defer v.SetModel(nil)
	v.SetStyleName("orders")
	tableArrange(v, 240, 180)
	b := tableVerticalBar(t, v)
	check := func(want color.RGBA) {
		t.Helper()
		pixels := tablePixels(t, b.Rect().Size, 1, func(p *tablePixelPainter) { b.Paint(p) })
		if pixels.RGBAAt(4, 10) != want || pixels.RGBAAt(0, int(b.Rect().Height)-1) != trough {
			t.Fatalf("滚动条没有使用所属Part/方形轨道：thumb=%v corner=%v", pixels.RGBAAt(4, 10), pixels.RGBAAt(0, int(b.Rect().Height)-1))
		}
	}
	check(normal)
	d, host := new(gui.EventDispatcher), &tabInputHost{root: v}
	x, y := v.scroll.Rect().X+b.Rect().X+4, v.scroll.Rect().Y+b.Rect().Y+10
	dispatchTabPointer(t, d, host, events.PointerMove, x, y)
	check(hover)
	dispatchTabPointer(t, d, host, events.PointerDown, x, y)
	check(pressed)
	dispatchTabPointer(t, d, host, events.PointerMove, x, y+30)
	dispatchTabPointer(t, d, host, events.PointerUp, x, y+30)
	if v.scroll.ScrollY() <= 0 || b.Snapshot().Role != gui.RoleScrollBar {
		t.Fatal("样式整合破坏了真实滚动输入或滚动条语义")
	}
	v.SetStyleName("test")
	tableArrange(v, 240, 180)
	pixels := tablePixels(t, b.Rect().Size, 1, func(p *tablePixelPainter) { b.Paint(p) })
	if pixels.RGBAAt(0, int(b.Rect().Height)-1) != (color.RGBA{G: 200, A: 255}) {
		t.Fatal("切换所属表格样式后，滚动条仍使用旧Name或全局Part")
	}
}

func TestTableExternalScrollBarKeepsHeaderAndBodyAligned(t *testing.T) {
	tableBorderApp(t)
	v, _, first, _ := tableFixture(30)
	defer v.SetModel(nil)
	first.SetMinWidth(400)
	first.SetWidth(400)
	v.SetHScrollBarOptions(gui.ScrollBarOptions{Policy: gui.ScrollBarHidden})
	v.SetVScrollBarOptions(gui.ScrollBarOptions{Policy: gui.ScrollBarHidden})
	tableArrange(v, 240, 180)
	h, vertical := gui.NewScrollBar(layout.DirectionHorizontal), gui.NewScrollBar(layout.DirectionVertical)
	handle := v.BindScroll(layout.DirectionHorizontal, h)
	defer handle.Disconnect()
	handleY := v.BindScroll(layout.DirectionVertical, vertical)
	defer handleY.Disconnect()
	v.scroll.SetScrollX(80)
	v.scroll.SetScrollY(100)
	if h.Snapshot().Range.Value != 80 || vertical.Snapshot().Range.Value != 100 || h.PageSize() != 240 || vertical.PageSize() != 148 {
		t.Fatal("表格外部条没有按正文视口同步")
	}
	if v.headerRow.Rect().X != -80 || v.scroll.ScrollGeometry().HScrollBar.Width != 0 || v.scroll.ScrollGeometry().VScrollBar.Height != 0 || first.Width() != 400 {
		t.Fatal("外部关联改变表头对齐、列宽或隐藏槽位")
	}
}
