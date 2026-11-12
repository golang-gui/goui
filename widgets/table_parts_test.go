package widgets

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"
)

// 同一页面的不同表格按Name+Part独立绘制，不能落回全局表头或行选择器。
func TestTablePartsUseOwnerStyleName(t *testing.T) {
	old := gui.App
	gui.App = &separatorApp{sheet: style.Sheet(
		style.Name("orders").Part("header").BackgroundColor(color.RGBA{R: 255, A: 255}).BorderWidth(0),
		style.Name("orders").Part("row").BackgroundColor(color.RGBA{G: 255, A: 255}).BorderWidth(0),
		style.Name("orders").Part("row-selected").BackgroundColor(color.RGBA{B: 255, A: 255}).BorderWidth(0),
		style.Name("test").Part("header").BackgroundColor(color.RGBA{G: 255, A: 255}).BorderWidth(0),
		style.Name("test").Part("row").BackgroundColor(color.RGBA{R: 255, A: 255}).BorderWidth(0),
	)}
	t.Cleanup(func() { gui.App = old })
	for _, tc := range []struct {
		name        string
		header, row color.RGBA
	}{
		{"orders", color.RGBA{R: 255, A: 255}, color.RGBA{G: 255, A: 255}},
		{"test", color.RGBA{G: 255, A: 255}, color.RGBA{R: 255, A: 255}},
	} {
		v, _, c, _ := tableFixture(1)
		defer v.SetModel(nil)
		v.SetStyleName(tc.name)
		tableArrange(v, 240, 180)
		h, row := v.headers[c], v.realized["r0"]
		header := tablePixels(t, h.Rect().Size, 1, func(p *tablePixelPainter) { h.Paint(p) })
		body := tablePixels(t, row.Rect().Size, 1, func(p *tablePixelPainter) { row.Paint(p) })
		// sRGB、不透明预乘RGBA，整像素内区，无AA，不需要容差。
		if header.RGBAAt(12, 12) != tc.header || body.RGBAAt(12, 12) != tc.row {
			t.Fatalf("%s未使用所属表格Part：header=%v row=%v", tc.name, header.RGBAAt(12, 12), body.RGBAAt(12, 12))
		}
		if h.label.StyleName() != tc.name || h.label.StylePart() != "header-text" {
			t.Fatal("表头文字没有明确的所属Name/Part")
		}
		if tc.name == "orders" {
			v.SetSelection([]string{"r0"})
			body = tablePixels(t, row.Rect().Size, 1, func(p *tablePixelPainter) { row.Paint(p) })
			if body.RGBAAt(12, 12) != (color.RGBA{B: 255, A: 255}) {
				t.Fatal("选中行没有使用独立Part")
			}
		}
	}
}

func TestTableStyleNameUpdatesOwnedPartsWithoutRebindingContent(t *testing.T) {
	old := gui.App
	gui.App = &separatorApp{sheet: style.Sheet(
		style.Name("orders").Part("grid-vertical").BorderWidth(3),
		style.Name("other").Part("grid-vertical").BorderWidth(7),
	)}
	t.Cleanup(func() { gui.App = old })
	v, _, c, d := tableFixture(4)
	defer v.SetModel(nil)
	v.SetStyleName("orders")
	tableArrange(v, 240, 180)
	row := v.realized["r0"]
	content := row.cells[0].content
	before := content.Rect().Width
	binds := d.binds
	v.SetStyleName("other")
	tableArrange(v, 240, 180)
	if content.Rect().Width != before-4 || d.binds != binds || row.cells[0].content != content || v.headers[c].label.StyleName() != "other" {
		t.Fatalf("修改StyleName未更新网格测量，或重绑了业务内容：before=%g after=%g binds=%d/%d", before, content.Rect().Width, binds, d.binds)
	}
	v.SetStyleName("")
	tableArrange(v, 240, 180)
	if v.headers[c].label.StyleName() != "table-view" {
		t.Fatal("清除StyleName未恢复默认部件名称")
	}
}

func TestTablePartsDoNotRenameApplicationCellContent(t *testing.T) {
	v, _, c, d := tableFixture(1)
	defer v.SetModel(nil)
	d.setup = func() gui.Widget { l := gui.NewLabel("cell"); l.SetStyleName("application-cell"); return l }
	c.SetDelegate(d)
	v.SetStyleName("orders")
	tableArrange(v, 240, 180)
	content := v.realized["r0"].cells[0].content
	v.SetStyleName("other")
	tableArrange(v, 240, 180)
	if content.StyleName() != "application-cell" {
		t.Fatal("表格Part污染了应用提供的单元格样式")
	}
}
