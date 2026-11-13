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

type tableHeaderLineRecorder struct {
	gui.Painter
	rects []geometry.Rectangle
}

func (p *tableHeaderLineRecorder) FillRect(rect geometry.Rectangle, _ graphics.Brush) {
	p.rects = append(p.rects, rect)
}

// 关闭分隔线必须不发出填充命令；不能依赖各后端对退化矩形的不同栅格化结果。
func TestTableHeaderSeparatorSkipsEmptyGeometry(t *testing.T) {
	tableBorderApp(t, style.Name("table-view").Part("header-separator").ForegroundColor(color.Black))
	v, _, _, _ := tableFixture(2)
	defer v.SetModel(nil)
	for _, rect := range []geometry.Rectangle{geometry.Rect(0, 32, 100, 0), geometry.Rect(0, 32, 0, 2), geometry.Rect(0, 32, 100, -1)} {
		p := new(tableHeaderLineRecorder)
		tablePaintHeaderLine(p, rect, v, style.Normal)
		if len(p.rects) != 0 {
			t.Fatalf("零线宽或空几何仍发出绘制命令：%v", p.rects)
		}
	}
	p := new(tableHeaderLineRecorder)
	tablePaintHeaderLine(p, geometry.Rect(0, 32, 100, 2), v, style.Normal)
	if len(p.rects) != 1 || p.rects[0] != geometry.Rect(0, 32, 100, 2) {
		t.Fatal("有效分隔线没有按独立预期完整提交")
	}
}

// 表头槽位上方是否延续线条由包围范围决定，数据边线不能伸入框外滚动槽。
// 窄数据还有未使用的表头空白；有包围框时必须与槽位接成同一条连续分隔线。
// 采样sRGB预乘RGBA，在1x/2x整像素色带内取点，容差为1级8位量化。
func TestTableHeaderGapFollowsEnclosingFrame(t *testing.T) {
	for _, scope := range []string{"data", "inner", "outline", "outer", "horizontal-only-frame"} {
		for _, columnWidth := range []float32{70, 180} {
			for _, offset := range []float32{0, 32, 1000} {
				t.Run(fmt.Sprintf("%s/width%g/scroll%g", scope, columnWidth, offset), func(t *testing.T) {
					rules := []style.Rule{
						style.Name("table-view").Part("header").BackgroundColor(color.NRGBA{R: 255, A: 128}),
						style.Name("table-view").Part("header-separator").ForegroundColor(color.NRGBA{B: 255, A: 128}).BorderWidth(2),
					}
					switch scope {
					case "data":
						rules = append(rules, style.Name("table-view").Part("grid-right").BorderWidth(2))
					case "outline":
						rules = append(rules, style.Name("table-view").Part("outline-vertical").BorderWidth(2))
					case "outer":
						rules = append(rules, style.Name("table-view").Part("outer-vertical").BorderWidth(2))
					case "horizontal-only-frame":
						rules = append(rules, style.Name("table-view").Part("outline-horizontal").BorderWidth(2))
					}
					tableBorderApp(t, rules...)
					v, _, first, _ := tableFixture(20)
					defer v.SetModel(nil)
					first.SetMinWidth(columnWidth)
					first.SetWidth(columnWidth)
					last := NewTableColumn("last", "Last")
					last.SetMinWidth(columnWidth)
					last.SetWidth(columnWidth)
					v.SetColumns(first, last)
					tableArrange(v, 240, 180)
					v.scroll.SetScrollX(offset)
					header := v.headerViewport.Rect()
					enclosed := scope == "outline" || scope == "outer"
					for _, scale := range []float32{1, 2} {
						pixels := tablePixels(t, v.Rect().Size, scale, func(p *tablePixelPainter) { v.Paint(p) })
						want := color.RGBA{R: 128, A: 128}
						if enclosed {
							want = color.RGBA{R: 64, B: 128, A: 192}
						}
						// 纵向槽上方：框外模式只有背景，框内模式有分隔线。
						x, y := header.X+header.Width+2, header.Y+header.Height-1
						checkTableEdgePixel(t, pixels.RGBAAt(int(x*scale), int(y*scale)), want)
						if columnWidth == 70 {
							blank := color.RGBA{}
							if enclosed {
								blank = want
							}
							checkTableEdgePixel(t, pixels.RGBAAt(int((header.X+180)*scale), int(y*scale)), blank)
						}
						// 父控件不重复覆盖真实表头，避免半透明背景重复合成。
						checkTableEdgePixel(t, pixels.RGBAAt(int((header.X+10)*scale), int((header.Y+10)*scale)), color.RGBA{})
					}
				})
			}
		}
	}
}

func TestTableHeaderGapStopsAtNearerTightOutline(t *testing.T) {
	tableBorderApp(t,
		style.Name("table-view").Part("header").BackgroundColor(color.NRGBA{R: 255, A: 128}),
		style.Name("table-view").Part("header-separator").ForegroundColor(color.NRGBA{B: 255, A: 128}).BorderWidth(2),
		style.Name("table-view").Part("outline-vertical").BorderWidth(2),
		style.Name("table-view").Part("outer-vertical").BorderWidth(2),
	)
	v, _, first, _ := tableFixture(2)
	defer v.SetModel(nil)
	first.SetWidth(70)
	last := NewTableColumn("last", "Last")
	last.SetWidth(70)
	v.SetColumns(first, last)
	tableArrange(v, 240, 180)
	header := v.headerViewport.Rect()
	pixels := tablePixels(t, v.Rect().Size, 1, func(p *tablePixelPainter) { v.Paint(p) })
	if got := pixels.RGBAAt(180, int(header.Y+header.Height)-1); got != (color.RGBA{}) {
		t.Fatalf("无条时表头补位穿过紧包围边界，延伸到整体外框：%v", got)
	}
}
