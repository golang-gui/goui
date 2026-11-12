package widgets

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

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
	v, _, _, _ := tableFixture(100)
	defer v.SetModel(nil)
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
	}
}

// sRGB预乘RGBA；整数色带在1x/2x满覆盖，半透明合成只允许1级8位量化误差。
func TestTableHeaderScrollGapPixels(t *testing.T) {
	old := gui.App
	gui.App = &separatorApp{sheet: style.Sheet(
		style.Name("table-view").BackgroundColor(color.Transparent).BorderWidth(0),
		style.Name("table-view").Part("frame").BackgroundColor(color.Transparent).BorderWidth(0).Radius(0),
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
