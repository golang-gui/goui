package main

import (
	"slices"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// 这些测试只覆盖例程自己的内存数据和布局适配，不创建窗口/事件循环。
func TestInitialModelsAndAssets(t *testing.T) {
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	defer d.motion.Stop()
	if d.list.ItemsCount() != 1000 || d.table.ItemsCount() != 120 || len(d.tabWindows) != 1 {
		t.Fatal("unexpected initial model sizes")
	}
	if !slices.Equal(d.tree.Children("src"), []string{"main", "view"}) {
		t.Fatal("missing tree children")
	}
	first := d.documents["document-1"]
	second := d.documents["document-2"]
	first.SetText("edited")
	if first == second || second.Text() == "edited" {
		t.Fatal("documents share mutable state")
	}
	if d.motion.Running() || d.busy || d.tickTimer != nil {
		t.Fatal("demo must start idle")
	}
	for i, source := range d.icons {
		img := source.Image(32, 32, graphics.RGB(100, 150, 200))
		if img == nil || img.Bounds().Dx() != 32 || img.Bounds().Dy() != 32 {
			t.Fatalf("icon %d: invalid image", i)
		}
		visible := false
		for y := 0; y < 32; y++ {
			for x := 0; x < 32; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				visible = visible || a > 0
			}
		}
		if !visible {
			t.Fatalf("icon %d: blank image", i)
		}
	}
}

func TestTabOrderAndTreeExpansion(t *testing.T) {
	ids := []string{"a", "b", "c"}
	for _, test := range []struct {
		id    string
		index int
		want  []string
	}{{"a", 2, []string{"b", "c", "a"}}, {"c", 0, []string{"c", "a", "b"}}, {"d", 99, []string{"a", "b", "c", "d"}}, {"b", -1, []string{"b", "a", "c"}}} {
		if got := moveID(ids, test.id, test.index); !slices.Equal(got, test.want) {
			t.Fatalf("move %s: %v, want %v", test.id, got, test.want)
		}
	}
	if !slices.Equal(ids, []string{"a", "b", "c"}) {
		t.Fatal("move changed caller-owned slice")
	}
	if got := setMember(ids, "b", false); !slices.Equal(got, []string{"a", "c"}) {
		t.Fatal(got)
	}
	if got := setMember(ids, "a", true); len(got) != 3 {
		t.Fatal("duplicate expansion ID")
	}
	if got := setMember(ids, "d", true); !slices.Equal(got, []string{"a", "b", "c", "d"}) {
		t.Fatal(got)
	}
}

func TestRecordSortAndEditUseStableIDs(t *testing.T) {
	m := newRecordModel()
	m.sort("size", widgets.SortDescending)
	for i := 1; i < m.ItemsCount(); i++ {
		if m.ItemAt(i-1).Size < m.ItemAt(i).Size {
			t.Fatal("sizes not descending")
		}
	}
	id := m.RowID(0)
	m.setNote(id, "kept after reorder")
	m.sort("name", widgets.SortAscending)
	found := false
	for i := 0; i < m.ItemsCount(); i++ {
		r := m.ItemAt(i)
		if r.ID == id {
			found = true
			if r.Note != "kept after reorder" {
				t.Fatal("edit applied to index rather than row ID")
			}
		} else if r.Note != "可编辑备注" {
			t.Fatal("another row was edited")
		}
	}
	if !found {
		t.Fatal("row lost during sorting")
	}
	m.sort("name", widgets.SortNone)
	if m.RowID(0) != "file-000" {
		t.Fatal("SortNone does not restore initial order")
	}
}

type layoutProbe struct {
	gui.WidgetBase
	constraint layout.Constraint
}

func (w *layoutProbe) Measure(c layout.Constraint) layout.Measurement {
	w.constraint = c
	return layout.Measured(geometry.Size{Width: c.Max.Width, Height: 450})
}

func TestFormFlowConstrainsWidthAndScrollsHeight(t *testing.T) {
	w := new(flowWidget)
	child := new(layoutProbe)
	w.SetChild(child)
	w.Measure(layout.Loose(geometry.Size{Width: 500, Height: 300}))
	if child.constraint.Max.Width != 500 || child.constraint.Max.Height != layout.Inf {
		t.Fatal("form children need finite width and unbounded height")
	}
	w.Arrange(geometry.Rect(0, 0, 480, 300))
	w.LayoutVisible(geometry.Size{Width: 480, Height: 300}, geometry.Point{Y: 70})
	if got := child.Rect(); got != geometry.Rect(0, -70, 480, 450) {
		t.Fatalf("arranged %v", got)
	}
	if got := w.ContentSize(); got != (geometry.Size{Width: 480, Height: 450}) {
		t.Fatalf("content size %v", got)
	}
	if child.Parent() != w {
		t.Fatal("helper embedding leaked into parent tree")
	}
	if info := w.Snapshot(); info.Role != gui.RoleBox || len(info.Children) != 1 {
		t.Fatal("flow wrapper omitted semantic subtree")
	}
}

// 只给 BoxView 提供已构造的测试孩子；布局属性由例程真实声明的 Update 应用。
// 不重建声明式协调器，不启动 Application 或文字排版。
type bodyContext struct {
	ui.BuildContext
	children []gui.Widget
}

func (ctx bodyContext) UpdateChildren(target ui.Container, views []ui.View) []gui.Widget {
	if len(views) != len(ctx.children) {
		panic("body declaration and test children differ")
	}
	for _, child := range ctx.children {
		target.AddChild(child)
	}
	return ctx.children
}

func TestBodyUsesAllocatedContentEdges(t *testing.T) {
	// 由宿主决定可用区域；例程不读取 Chrome 私有的边缘宽度，也不重复留白。
	// 同时覆盖无偏移分配与非零内容起点，后者的 Snapshot 仍须使用窗口坐标。
	for _, test := range []struct {
		name   string
		bounds geometry.Rectangle
	}{
		{"narrow-full", geometry.Rect(0, 0, 820, 600)},
		{"narrow-inset", geometry.Rect(5, 5, 810, 590)},
		{"wide-full", geometry.Rect(0, 0, 1120, 800)},
		{"wide-inset", geometry.Rect(5, 5, 1110, 790)},
	} {
		t.Run(test.name, func(t *testing.T) {
			side := gui.NewLinearBox(layout.DirectionVertical)
			side.SetMinSize(geometry.Size{Width: 190})
			side.SetMaxSize(geometry.Size{Width: 190})
			content := gui.NewLinearBox(layout.DirectionVertical)
			content.SetMinSize(geometry.Size{Width: 200, Height: 2000})
			scroll := gui.NewScrollView()
			scroll.SetChild(content)
			view := demoBody(ui.VBox(), ui.VBox())
			body := view.Mount(nil).(*gui.LinearBox)
			view.Update(bodyContext{children: []gui.Widget{side, scroll}}, body)
			body.Measure(layout.Tight(test.bounds.Size))
			body.Arrange(test.bounds)
			if got := side.Rect(); got != geometry.Rect(0, 0, 190, test.bounds.Height) {
				t.Fatalf("navigation allocation %v adds unexpected outer padding", got)
			}
			if got := scroll.Rect(); got != geometry.Rect(190, 0, test.bounds.Width-190, test.bounds.Height) {
				t.Fatalf("scroll allocation %v does not fill the remaining content area", got)
			}
			var bar *gui.ScrollBar
			for _, child := range scroll.Children() {
				if b, ok := child.(*gui.ScrollBar); ok && b.Orientation() == layout.DirectionVertical {
					bar = b
				}
			}
			if bar == nil || !bar.Visible() || bar.Rect().Width != 8 {
				t.Fatal("expected a full 8 DIP vertical scrollbar")
			}
			want := geometry.Rect(test.bounds.X+test.bounds.Width-8, test.bounds.Y, 8, test.bounds.Height)
			if got := bar.Snapshot().Bounds; got != want {
				t.Fatalf("scrollbar bounds %v, want %v at the allocated right edge", got, want)
			}
			// 检查左右两个边缘都能命中完整滚动条，而不只是中心可点。
			for _, x := range []float32{test.bounds.Width - 7.5, test.bounds.Width - .5} {
				if got := gui.Pick(body, geometry.Point{X: x, Y: test.bounds.Height / 2}); got != bar {
					t.Fatalf("scrollbar edge x=%v picked %T, want the scrollbar", x, got)
				}
			}
		})
	}
}
