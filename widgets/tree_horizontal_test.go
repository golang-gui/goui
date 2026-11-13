package widgets

import (
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

type treeHorizontalDelegate struct {
	*treeTestDelegate
	widths map[string]float32
}

func (d *treeHorizontalDelegate) Bind(row TreeRow, widget gui.Widget) {
	d.treeTestDelegate.Bind(row, widget)
	child := widget.(*TreeExpander).Child()
	child.SetLayoutManager(&splitTestLayout{desired: geometry.Size{Width: d.widths[row.ID], Height: 20}})
}

func horizontalTree() (*TreeView, *treeHorizontalDelegate) {
	v, _ := treeFixture()
	d := &treeHorizontalDelegate{treeTestDelegate: &treeTestDelegate{view: v}, widths: map[string]float32{"root": 600, "a": 20, "b": 80, "other": 50}}
	v.SetDelegate(d)
	v.SetExpanded("root", true)
	return v, d
}

func layoutHorizontalTree(v *TreeView, width, height, x, y float32) {
	v.Arrange(geometry.Rect(0, 0, width, height))
	v.LayoutVisible(geometry.Size{Width: width, Height: height}, geometry.Point{X: x, Y: y})
}

func TestTreeHorizontalRowsUseViewportForHitAndSnapshot(t *testing.T) {
	v, _ := horizontalTree()
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(v)
	defer owner.Destroy()
	v.SetRowInsets(12, 18)
	layoutHorizontalTree(v, 200, 112, 0, 0)
	short := v.realized["a"]
	extent := v.ContentSize()
	if extent.Width != 658 {
		t.Fatal("自然宽度应为600+28前缀及两侧内缩30", extent)
	}
	for _, x := range []float32{0, 30, 458} {
		layoutHorizontalTree(v, 200, 112, x, 0)
		if v.realized["a"] != short || short.Rect() != (geometry.Rect(12, 28, 170, 28)) || short.content.Rect().X != -x {
			t.Fatal("横向滚动移动了行外壳或替换了内容", x, short.Rect(), short.content.Rect())
		}
		row, bounds, ok := v.RowAt(geometry.Point{X: 100, Y: 42})
		if !ok || row.ID != "a" || bounds != short.Rect() {
			t.Fatal("短行空白不可命中或RowAt几何不一致", row, bounds, ok)
		}
		picked := gui.Pick(v, geometry.Point{X: 100, Y: 42})
		for picked != nil && picked != short {
			picked = picked.Parent()
		}
		if picked != short {
			t.Fatal("真实命中路径没有经过短行")
		}
		input := owner.(gui.ModalTarget)
		for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
			if err := input.DispatchEvent(events.PointerEvent{EventType: kind, Button: events.PointerButtonLeft,
				Position: geometry.Point{X: 100, Y: 42}}); err != nil {
				t.Fatal(err)
			}
		}
		if v.Current() != "a" || len(v.Selection()) != 1 || v.Selection()[0] != "a" {
			t.Fatal("横向滚动后的正常点击未选择短行", v.Current(), v.Selection())
		}
		info := v.Snapshot().Children[1]
		if info.Role != RoleTreeItem || info.Bounds != short.Snapshot().Bounds || info.Bounds.X != 12 || info.Bounds.Width != 170 || !info.Selected {
			t.Fatal("行语义仍使用内容宽度", info)
		}
	}
	for _, x := range []float32{0, 11, 182, 199} {
		if _, _, ok := v.RowAt(geometry.Point{X: x, Y: 42}); ok {
			t.Fatal("内缩外的空白仍命中行", x)
		}
	}
	if v.ContentSize() != extent {
		t.Fatal("仅横向滚动改变了内容范围", v.ContentSize(), extent)
	}
}

func TestTreeHorizontalContentDecorationScrollsInsideFixedRow(t *testing.T) {
	old := gui.App
	t.Cleanup(func() { gui.App = old })
	gui.App = &separatorApp{sheet: style.Sheet(
		style.Name("scroll-row").BackgroundColor(color.White).BorderWidth(0).Radius(0),
		style.Name("scroll-row").Part("content").BackgroundColor(color.Transparent).
			BorderColor(color.Black).BorderWidth(2).BorderSides(style.BorderTop|style.BorderLeft).Radius(0),
	)}
	v, _ := horizontalTree()
	defer v.SetModel(nil)
	v.SetRowStyle(func(TreeRow) string { return "scroll-row" })
	v.SetRowInsets(12, 18)
	for _, scale := range []float32{1, 2} {
		for _, x := range []float32{12, 60} {
			layoutHorizontalTree(v, 200, 112, x, 0)
			row := v.realized["a"]
			pixels := tablePixels(t, row.Rect().Size, scale, func(p *tablePixelPainter) { row.Paint(&treeStylePixelPainter{p}) })
			// sRGB预乘不透明黑白；整数DIP实线内部逐像素断言，无字体或抗锯齿容差。
			for px := 0; px < int(170*scale); px++ {
				want := color.RGBA{255, 255, 255, 255}
				if float32(px) >= (40-x)*scale {
					want = color.RGBA{A: 255}
				}
				if got := pixels.RGBAAt(px, int(scale)); got != want {
					t.Fatalf("scale=%g scrollX=%g x=%d像素=%v，预期%v", scale, x, px, got, want)
				}
			}
			if x == 60 && pixels.RGBAAt(0, int(14*scale)) != (color.RGBA{255, 255, 255, 255}) {
				t.Fatal("已滚出左侧的content边框被错误固定到视口")
			}
		}
	}
}

func TestTreeRowInsetsNormalizeAndClipNarrowViewport(t *testing.T) {
	v, _ := horizontalTree()
	defer v.SetModel(nil)
	v.SetRowInsets(12, 18)
	layoutHorizontalTree(v, 10, 56, 0, 0)
	if r := v.realized["root"].Rect(); r.X != 10 || r.Width != 0 {
		t.Fatal("内缩超过视口时没有裁空", r)
	}
	if _, _, ok := v.RowAt(geometry.Point{X: 5, Y: 14}); ok {
		t.Fatal("零宽行可命中")
	}
	v.SetRowInsets(float32(math.NaN()), -4)
	if left, right := v.RowInsets(); left != 0 || right != 0 {
		t.Fatal("非法内缩未归零", left, right)
	}
	v.SetRowInsets(float32(math.Inf(1)), 8)
	layoutHorizontalTree(v, 200, 56, 40, 0)
	if r := v.realized["root"].Rect(); r.X != 0 || r.Width != 192 {
		t.Fatal("归一化后布局错误", r)
	}
}
