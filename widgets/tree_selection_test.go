package widgets

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

func TestTreeSelectionAdjacencyUsesExpandedLogicalOrder(t *testing.T) {
	v, _ := treeFixture()
	defer v.SetModel(nil)
	v.SetExpandedIDs([]string{"root", "a"})
	v.SetSelectionMode(SelectionMultiple)
	v.SetSelection([]string{"root", "a", "nested", "other"})
	treeLayout(v, 140, 0)
	for _, tc := range []struct {
		id       string
		selected bool
		previous bool
		next     bool
	}{
		{"root", true, false, true}, {"a", true, true, true}, {"nested", true, true, false},
		{"b", false, true, true}, {"other", true, false, false},
	} {
		row := v.realized[tc.id].bound
		if row.Selected != tc.selected || row.PreviousSelected != tc.previous || row.NextSelected != tc.next {
			t.Fatalf("%s的绑定快照=%+v", tc.id, row)
		}
		at, _, ok := v.RowAt(v.realized[tc.id].Rect().Pos.Add(geometry.Point{X: 10, Y: 2}))
		if !ok || at != row {
			t.Fatal("RowAt与绑定快照的邻接信息不一致", at, row)
		}
	}
	// 不同层级的相邻行连续连接；未选中行仍如实报告两侧的事实。
	if v.realized["root"].bound.Depth == v.realized["a"].bound.Depth || len(v.Snapshot().Children) != 5 {
		t.Fatal("用例没有覆盖跨层级或语义行数量改变")
	}
}

func TestTreeSelectionAdjacencyRebindsOnlyChangedNeighbors(t *testing.T) {
	v, _ := treeFixture()
	defer v.SetModel(nil)
	v.SetExpandedIDs([]string{"root", "a"})
	v.SetSelectionMode(SelectionMultiple)
	calls := map[string]int{}
	v.SetRowStyle(func(row TreeRow) string {
		calls[row.ID]++
		return fmt.Sprintf("row-%t-%t", row.PreviousSelected, row.NextSelected)
	})
	v.SetSelection([]string{"root", "a"})
	treeLayout(v, 140, 0)
	root := v.realized["root"]
	content, rect := root.content, root.Rect()
	clear(calls)
	v.SetSelection([]string{"root"})
	treeLayout(v, 140, 0)
	if len(calls) != 3 || calls["root"] != 1 || calls["a"] != 1 || calls["nested"] != 1 {
		t.Fatal("未更新相邻行，或重新绑定了无关行", calls)
	}
	if v.realized["root"] != root || root.content != content || root.Rect() != rect || root.StyleName() != "row-false-false" {
		t.Fatal("邻接变化替换了控件／几何，或没有更新RowStyle")
	}
	info := root.Snapshot()
	if info.Role != RoleTreeItem || !info.Selected || !slices.Contains(info.Actions, gui.ActionClick) || len(info.Children) != 1 {
		t.Fatal("选区分组改变了行语义或交互能力", info)
	}
}

func TestTreeSelectionAdjacencySurvivesVirtualizationAndStructure(t *testing.T) {
	v, m := treeFixture()
	defer v.SetModel(nil)
	v.SetExpandedIDs([]string{"root", "a"})
	v.SetSelectionMode(SelectionMultiple)
	v.SetSelection([]string{"root", "a", "nested", "b", "other"})
	treeLayout(v, 28, 0)
	if !v.realized["root"].bound.NextSelected || len(v.realized) != 1 {
		t.Fatal("离屏邻居没有参与计算，或为此实现了离屏行")
	}
	treeLayout(v, 28, 56)
	shell := v.realized["nested"]
	if shell == nil || !shell.bound.PreviousSelected || !shell.bound.NextSelected || len(v.realized) != 1 {
		t.Fatal("视口边缘被错误当作选区边缘")
	}
	v.SetSelection(nil)
	treeLayout(v, 28, 112)
	if v.realized["other"] != shell || shell.bound.PreviousSelected || shell.bound.NextSelected || shell.bound.Selected {
		t.Fatal("复用壳保留了旧选区位置", shell.bound)
	}
	v.SetSelection([]string{"a", "nested", "b"})
	v.SetExpanded("a", false)
	treeLayout(v, 112, 0)
	if row := v.realized["a"].bound; row.PreviousSelected || !row.NextSelected {
		t.Fatal("折叠后没有按新的平铺顺序计算", row)
	}
	m.Remove("b")
	treeLayout(v, 112, 0)
	if row := v.realized["a"].bound; row.PreviousSelected || row.NextSelected {
		t.Fatal("删除邻居后仍连接旧选区", row)
	}
	last := v.realized["a"]
	v.SetModel(nil)
	if last.bound != (TreeRow{}) || len(v.realized) != 0 {
		t.Fatal("解绑没有清除邻接状态", last.bound)
	}
}

func TestTreeSelectionAdjacencyRejectsReentrantStyleResult(t *testing.T) {
	v, _ := treeFixture()
	defer v.SetModel(nil)
	v.SetExpanded("root", true)
	v.SetSelectionMode(SelectionMultiple)
	v.SetSelection([]string{"root", "a"})
	v.SetRowStyle(func(row TreeRow) string {
		if row.ID == "root" && row.NextSelected {
			v.SetSelection([]string{"root"})
			return "obsolete-neighbor"
		}
		return "current-neighbor"
	})
	treeLayout(v, 112, 0)
	if v.realized["root"].StyleName() == "obsolete-neighbor" {
		t.Fatal("回调改变邻接状态后提交了旧结果")
	}
	treeLayout(v, 112, 0)
	if row := v.realized["root"]; row.bound.NextSelected || row.StyleName() != "current-neighbor" {
		t.Fatal("后续布局没有修正绑定快照", row.bound, row.StyleName())
	}
}

type selectionPixelPainter struct{ *treeStylePixelPainter }

func (p *selectionPixelPainter) SnapRect(rect geometry.Rectangle) geometry.Rectangle {
	return p.native.SnapRect(rect)
}

func selectionPixels(t *testing.T, v *TreeView, scale float32) *image.RGBA {
	t.Helper()
	return tablePixels(t, v.Rect().Size, scale, func(p *tablePixelPainter) {
		for _, child := range v.list.Children() {
			row := child.(*treeRow)
			r := row.Rect()
			p.native.SetTransform(geometry.Translate(r.X, r.Y))
			p.native.SetClipRect(graphics.Rect(r.X, r.Y, r.Width, r.Height))
			row.Paint(&selectionPixelPainter{&treeStylePixelPainter{p}})
		}
	})
}

// 不透明与半透明sRGB预乘色带内取样；接合处逐像素断言，无抗锯齿／字体金图容差。
func TestTreeSelectionConnectedCornersAndSingleRows(t *testing.T) {
	old := gui.App
	t.Cleanup(func() { gui.App = old })
	gui.App = &separatorApp{sheet: style.Sheet(
		style.Name("selection-row").BackgroundColor(color.Transparent).BorderWidth(0).Radius(8),
		style.Name("selection-row").Part("selected").BackgroundColor(color.NRGBA{B: 255, A: 128}),
		style.Name("selection-row").Part("content").BackgroundColor(color.Transparent).BorderWidth(0).Radius(0),
	)}
	for _, selected := range [][]string{{"root"}, {"root", "a"}, {"root", "a", "nested"}, {"root", "nested"}} {
		for _, scale := range []float32{1, 2} {
			t.Run(fmt.Sprintf("%v/%gx", selected, scale), func(t *testing.T) {
				v, _ := treeFixture()
				defer v.SetModel(nil)
				v.SetExpandedIDs([]string{"root", "a"})
				v.SetSelectionMode(SelectionMultiple)
				v.SetSelection(selected)
				v.SetRowStyle(func(TreeRow) string { return "selection-row" })
				layoutHorizontalTree(v, 80, 140, 0, 0)
				pixels := selectionPixels(t, v, scale)
				for i, id := range []string{"root", "a", "nested", "b", "other"} {
					center := color.RGBA{}
					if slices.Contains(selected, id) {
						center = color.RGBA{B: 128, A: 128}
					}
					if got := pixels.RGBAAt(int(40*scale), int(float32(i*28+14)*scale)); got != center {
						t.Fatalf("行%s中心=%v，预期%v", id, got, center)
					}
					for _, edge := range []int{0, 27} {
						want := color.RGBA{}
						neighbor := i - 1
						if edge == 27 {
							neighbor = i + 1
						}
						ids := []string{"root", "a", "nested", "b", "other"}
						if slices.Contains(selected, id) && neighbor >= 0 && neighbor < len(ids) && slices.Contains(selected, ids[neighbor]) {
							want = color.RGBA{B: 128, A: 128}
						}
						for _, x := range []int{0, 79} {
							if got := pixels.RGBAAt(x*int(scale), (i*28+edge)*int(scale)); got != want {
								t.Fatalf("行%s edge=%d x=%d像素=%v，预期%v", id, edge, x, got, want)
							}
						}
					}
				}
			})
		}
	}
}

func TestTreeSelectionKeepsIndependentBorderFocusAndContent(t *testing.T) {
	old := gui.App
	t.Cleanup(func() { gui.App = old })
	gui.App = &separatorApp{sheet: style.Sheet(
		style.Name("selection-row").BackgroundColor(color.Transparent).BorderWidth(0).Radius(8),
		style.Name("selection-row").Part("selected").BackgroundColor(color.NRGBA{B: 255, A: 128}).BorderColor(color.RGBA{R: 255, A: 255}).BorderWidth(2),
		style.Name("selection-row").Part("current").BackgroundColor(color.Transparent).BorderColor(color.RGBA{R: 255, G: 255, A: 255}).BorderWidth(1),
		style.Name("selection-row").Part("content").BackgroundColor(color.Transparent).BorderColor(color.RGBA{G: 255, A: 255}).BorderWidth(2).BorderSides(style.BorderTop).Radius(0),
	)}
	v, _ := treeFixture()
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(v)
	defer owner.Destroy()
	v.SetExpandedIDs([]string{"root", "a"})
	v.SetSelectionMode(SelectionMultiple)
	v.SetSelection([]string{"root", "a", "nested"})
	v.SetRowStyle(func(TreeRow) string { return "selection-row" })
	v.SetCurrent("a")
	layoutHorizontalTree(v, 80, 140, 0, 0)
	for _, scale := range []float32{1, 2} {
		pixels := selectionPixels(t, v, scale)
		for _, sample := range []struct {
			x, y int
			want color.RGBA
		}{
			{0, 28, color.RGBA{B: 128, A: 128}}, // 中间行背景连接，但圆形单行边框不占据角外。
			{32, 28, color.RGBA{R: 255, A: 255}},
			{0, 42, color.RGBA{R: 255, A: 255}},
			{32, 42, color.RGBA{B: 128, A: 128}},
			{45, 28, color.RGBA{G: 255, A: 255}}, // content仍在展开槽之后独立叠加。
		} {
			if got := pixels.RGBAAt(sample.x*int(scale), sample.y*int(scale)); got != sample.want {
				t.Fatalf("scale=%g (%d,%d)=%v，预期%v", scale, sample.x, sample.y, got, sample.want)
			}
		}
	}
	if err := owner.(gui.ModalTarget).DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyTab}); err != nil {
		t.Fatal(err)
	}
	row := v.realized["a"]
	if !v.Focused() || !v.FocusVisible() || row.Focused() || row.Snapshot().Focused {
		t.Fatal("选区组合伪造行焦点，或丢失树的真实键盘焦点")
	}
	pixels := selectionPixels(t, v, 1)
	if got := pixels.RGBAAt(32, 28); got != (color.RGBA{R: 255, G: 255, A: 255}) {
		t.Fatal("current框未独立叠加", got)
	}
	if radius, _ := gui.ResolveWidgetStyle(row, "selection-row", "selected", row.State()).CornerRadius(); radius != geometry.UniformCornerRadius(8) {
		t.Fatal("绘制修改了共享样式值", radius)
	}
}

func TestTreeSelectionNormalizesOuterRadiusBeforeJoining(t *testing.T) {
	old := gui.App
	t.Cleanup(func() { gui.App = old })
	gui.App = &separatorApp{sheet: style.Sheet(
		style.Name("selection-row").BackgroundColor(color.Transparent).BorderWidth(0).Radius(100),
		style.Name("selection-row").Part("selected").BackgroundColor(color.NRGBA{B: 255, A: 128}),
		style.Name("selection-row").Part("content").BackgroundColor(color.Transparent).BorderWidth(0).Radius(0),
	)}
	v, _ := treeFixture()
	defer v.SetModel(nil)
	v.SetExpanded("root", true)
	v.SetSelectionMode(SelectionMultiple)
	v.SetSelection([]string{"root", "a"})
	v.SetRowStyle(func(TreeRow) string { return "selection-row" })
	layoutHorizontalTree(v, 80, 140, 0, 0)
	// 28 DIP行高将原100 DIP半径归一到14；像素(1,9)四角均在圆内。
	// 先去掉另一端再归一会错误增大外缘半径，使该点完全落到圆外。
	for _, scale := range []float32{1, 2} {
		pixels := selectionPixels(t, v, scale)
		if got := pixels.RGBAAt(int(scale), int(9*scale)); got != (color.RGBA{B: 128, A: 128}) {
			t.Fatalf("scale=%g外缘半径被接合行为改变：%v", scale, got)
		}
	}
}

func TestTreeSelectionTranslucentBorderKeepsSingleBackgroundPass(t *testing.T) {
	old := gui.App
	t.Cleanup(func() { gui.App = old })
	gui.App = &separatorApp{sheet: style.Sheet(
		style.Name("selection-row").BackgroundColor(color.Transparent).BorderWidth(0).Radius(8),
		style.Name("selection-row").Part("selected").BackgroundColor(color.NRGBA{B: 255, A: 128}).
			BorderColor(color.NRGBA{R: 255, A: 128}).BorderWidth(2),
		style.Name("selection-row").Part("content").BackgroundColor(color.Transparent).BorderWidth(0).Radius(0),
	)}
	v, _ := treeFixture()
	defer v.SetModel(nil)
	v.SetExpandedIDs([]string{"root", "a"})
	v.SetSelectionMode(SelectionMultiple)
	v.SetSelection([]string{"root", "a", "nested"})
	v.SetRowStyle(func(TreeRow) string { return "selection-row" })
	layoutHorizontalTree(v, 80, 140, 0, 0)
	for _, scale := range []float32{1, 2} {
		pixels := selectionPixels(t, v, scale)
		for _, point := range [][2]int{{32, 28}, {0, 42}} {
			// 标准库image/draw.Over独立预期：16位合成后截为8位，蓝为63而非四舍五入的64。
			if got := pixels.RGBAAt(point[0]*int(scale), point[1]*int(scale)); got != (color.RGBA{R: 128, B: 63, A: 192}) {
				t.Fatalf("scale=%g边框合成=%v", scale, got)
			}
		}
		if got := pixels.RGBAAt(0, int(28*scale)); got != (color.RGBA{B: 128, A: 128}) {
			t.Fatal("圆角单行框之外背景被重复叠加", got)
		}
	}
}

func TestTreeSelectionFractionalHeightHasNoAlphaSeam(t *testing.T) {
	old := gui.App
	t.Cleanup(func() { gui.App = old })
	gui.App = &separatorApp{sheet: style.Sheet(
		style.Name("selection-row").BackgroundColor(color.Transparent).BorderWidth(0).Radius(8),
		style.Name("selection-row").Part("selected").BackgroundColor(color.NRGBA{B: 255, A: 128}),
		style.Name("selection-row").Part("content").BackgroundColor(color.Transparent).BorderWidth(0).Radius(0),
	)}
	for _, scale := range []float32{1, 2} {
		v, _ := treeFixture()
		v.SetExpandedIDs([]string{"root", "a"})
		v.SetSelectionMode(SelectionMultiple)
		v.SetSelection([]string{"root", "a", "nested"})
		v.SetRowStyle(func(TreeRow) string { return "selection-row" })
		layoutHorizontalTree(v, 80, 140, 0, 0)
		for _, row := range v.realized {
			row.SetMinSize(geometry.Size{Height: 31.5})
		}
		layoutHorizontalTree(v, 80, 140, 0, 0)
		pixels := selectionPixels(t, v, scale)
		for _, boundary := range []float32{31.5, 63} {
			for y := int(boundary*scale) - 2; y <= int(boundary*scale)+2; y++ {
				for x := 0; x < int(80*scale); x++ {
					if got := pixels.RGBAAt(x, y); got != (color.RGBA{B: 128, A: 128}) {
						t.Fatalf("scale=%g 接合%g像素(%d,%d)=%v", scale, boundary, x, y, got)
					}
				}
			}
		}
		v.SetModel(nil)
	}
}
