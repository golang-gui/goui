package widgets

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

func completeTableEdgeSheet() style.StyleSheet {
	ink := color.NRGBA{R: 255, A: 128}
	return style.Sheet(
		style.Name("table-view").BackgroundColor(color.Transparent).BorderWidth(0),
		style.Name("table-view").Part("header").BackgroundColor(color.Transparent).BorderWidth(0),
		style.Name("table-view").Part("row").BackgroundColor(color.Transparent).BorderWidth(0).Radius(0),
		style.Name("table-view").Part("grid-horizontal").BorderColor(ink).BorderWidth(2),
		style.Name("table-view").Part("grid-vertical").BorderColor(ink).BorderWidth(3),
		style.Name("table-view").Part("grid-top").BorderColor(ink).BorderWidth(4),
		style.Name("table-view").Part("grid-left").BorderColor(ink).BorderWidth(5),
		style.Name("table-view").Part("header-separator").ForegroundColor(ink).BorderWidth(2),
	)
}

// 整数DIP色带在1x/2x满覆盖；比较sRGB预乘RGBA，容差仅1级8位量化。
// 同色半透明角点应只合成一次，自绘内容不得覆盖任何边线。
func TestTableCompleteCellEdgesLayoutAndPixels(t *testing.T) {
	old := gui.App
	gui.App = &separatorApp{sheet: completeTableEdgeSheet()}
	t.Cleanup(func() { gui.App = old })
	for _, scale := range []float32{1, 2} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			v, _, c, _ := tableFixture(2)
			defer v.SetModel(nil)
			c.SetWidth(80)
			other := NewTableColumn("other", "Other")
			other.SetWidth(60)
			v.SetColumns(c, other)
			tableArrange(v, 200, 200)
			row, h := v.realized["r0"], v.headers[c]
			content := row.cells[0].content.(*tableTestContent)
			if content.Rect() != geometry.Rect(5, 0, 72, 32) || h.Rect().Height != 38 || h.label.Rect().X != 13 {
				t.Fatalf("首列左边/表头上边没有独立预留：content=%v header=%v label=%v", content.Rect(), h.Rect(), h.label.Rect())
			}
			content.ink = graphics.ColorOf(color.RGBA{G: 255, A: 255})
			body := tablePixels(t, row.Rect().Size, scale, func(p *tablePixelPainter) {
				row.Paint(p)
				p.native.SetTransform(geometry.Translate(content.Rect().X, content.Rect().Y))
				content.Paint(p)
			})
			header := tablePixels(t, h.Rect().Size, scale, func(p *tablePixelPainter) { h.Paint(p) })
			line, green := color.RGBA{R: 128, A: 128}, color.RGBA{G: 255, A: 255}
			for _, sample := range []struct {
				x, y int
				want color.RGBA
			}{{0, 0, line}, {1, 20, line}, {0, 33, line}, {79, 33, line}, {139, 20, line}, {5, 0, green}, {76, 31, green}, {145, 10, color.RGBA{}}} {
				got := body.RGBAAt(sample.x*int(scale), sample.y*int(scale))
				checkTableEdgePixel(t, got, sample.want)
			}
			for _, point := range [][2]int{{0, 0}, {79, 0}, {0, 10}, {79, 10}, {0, 37}, {79, 37}} {
				checkTableEdgePixel(t, header.RGBAAt(point[0]*int(scale), point[1]*int(scale)), line)
			}
			if header.RGBAAt(10*int(scale), 10*int(scale)) != (color.RGBA{}) {
				t.Fatal("边线侵入表头内容区域")
			}
		})
	}
}

func checkTableEdgePixel(t *testing.T, got, want color.RGBA) {
	t.Helper()
	for i, x := range []uint8{got.R, got.G, got.B, got.A} {
		y := []uint8{want.R, want.G, want.B, want.A}[i]
		if int(x)-int(y) > 1 || int(y)-int(x) > 1 {
			t.Fatalf("边线像素%v，预期%v；可能重复合成或内容盖线", got, want)
		}
	}
}

func TestTableLeftEdgeFollowsFirstVisibleColumn(t *testing.T) {
	old := gui.App
	gui.App = &separatorApp{sheet: completeTableEdgeSheet()}
	t.Cleanup(func() { gui.App = old })
	v, m, c, _ := tableFixture(2)
	defer v.SetModel(nil)
	c.SetWidth(80)
	other := NewTableColumn("other", "Other")
	other.SetWidth(60)
	other.SetDelegate(&tableTestDelegate{model: m})
	v.SetColumns(c, other)
	for _, change := range []string{"initial", "reorder", "hide", "restore"} {
		switch change {
		case "reorder":
			v.SetColumns(other, c)
		case "hide":
			other.SetVisible(false)
		case "restore":
			other.SetVisible(true)
		}
		tableArrange(v, 200, 200)
		for i, cell := range v.realized["r0"].cells {
			want := float32(0)
			if i == 0 {
				want = 5
			}
			if cell.content.Rect().X != want || v.headers[cell.column].label.Rect().X != want+8 {
				t.Fatalf("%s：左边线没有跟随首个可见列", change)
			}
		}
	}
	c.SetMinWidth(0)
	c.SetWidth(1)
	other.SetVisible(false)
	tableArrange(v, 1, 1)
	if cell := v.realized["r0"]; cell != nil {
		for _, c := range cell.cells {
			if c.content.Rect().Width < 0 || c.content.Rect().Height < 0 {
				t.Fatal("极窄列得到负的内容分配")
			}
		}
	}
}

func TestEmptyTableHasOnlyHeaderEdges(t *testing.T) {
	old := gui.App
	gui.App = &separatorApp{sheet: completeTableEdgeSheet()}
	t.Cleanup(func() { gui.App = old })
	v, _, c, _ := tableFixture(0)
	defer v.SetModel(nil)
	c.SetWidth(80)
	tableArrange(v, 240, 180)
	if len(v.realized) != 0 {
		t.Fatal("空模型虚构了内容行")
	}
	h := v.headers[c]
	pixels := tablePixels(t, h.Rect().Size, 1, func(p *tablePixelPainter) { h.Paint(p) })
	checkTableEdgePixel(t, pixels.RGBAAt(0, 0), color.RGBA{R: 128, A: 128})
}
