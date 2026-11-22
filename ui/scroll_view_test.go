package ui

import (
	"fmt"
	"image"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

func TestScrollViewDragAutoScrollDefaultsAndUpdates(t *testing.T) {
	root := newRoot()
	sv := root.update(ScrollView(Label("content"))).(*gui.ScrollView)
	if !sv.DragAutoScroll() {
		t.Fatal("drag auto scrolling should default to enabled")
	}
	root.update(ScrollView(Label("content")).DragAutoScroll(false))
	if sv.DragAutoScroll() {
		t.Fatal("declaration did not disable drag auto scrolling")
	}
	root.update(ScrollView(Label("content")))
	if !sv.DragAutoScroll() {
		t.Fatal("omitted value should restore the default")
	}
}

func TestScrollViewBarOptionsUpdateAndRestore(t *testing.T) {
	r := newRoot()
	defer r.update(nil)
	h := ScrollBarOptions{Policy: ScrollBarAlways, Thickness: 10, ThumbThickness: 6, Spacing: 3}
	v := ScrollBarOptions{Policy: ScrollBarHidden, Thickness: 20, ThumbThickness: 8, Spacing: 4}
	build := func() *ScrollViewView {
		return ScrollView(Label("保留内容")).HScrollBarOptions(h).VScrollBarOptions(v)
	}
	sv := r.update(build()).(*gui.ScrollView)
	children, content := sv.Children(), sv.Child()
	if sv.HScrollBarOptions() != h || sv.VScrollBarOptions() != v {
		t.Fatal("声明未应用到两轴配置")
	}
	if r.update(build()) != sv || sv.Child() != content {
		t.Fatal("相同声明重建了滚动宿主或内容")
	}
	r.update(ScrollView(Label("保留内容")))
	want := ScrollBarOptions{Thickness: 8, ThumbThickness: 8}
	if sv.HScrollBarOptions() != want || sv.VScrollBarOptions() != want {
		t.Fatal("省略配置没有恢复默认")
	}
	for i, child := range sv.Children() {
		if child != children[i] {
			t.Fatal("切换配置重建了内部结构控件")
		}
	}
}

func TestScrollViewDirectionUpdatesAndRestoresDefault(t *testing.T) {
	root := newRoot()
	sv := root.update(ScrollView(Label("content")).Direction(ScrollVertical)).(*gui.ScrollView)
	if sv.Direction() != gui.ScrollVertical {
		t.Fatal("vertical declaration was not applied")
	}
	if got := root.update(ScrollView(Label("content")).Direction(ScrollHorizontal)); got != sv || sv.Direction() != gui.ScrollHorizontal {
		t.Fatal("direction update recreated the widget or was not applied")
	}
	root.update(ScrollView(Label("content")))
	if sv.Direction() != gui.ScrollBoth {
		t.Fatal("omitted direction did not restore ScrollBoth")
	}
}

func TestScrollViewMountsAndUpdatesChild(t *testing.T) {
	root := newRoot()

	widget := root.update(ScrollView(Label("hello")))
	sv, ok := widget.(*gui.ScrollView)
	if !ok {
		t.Fatalf("updated %T, want *gui.ScrollView", widget)
	}
	child, ok := sv.Child().(*gui.Label)
	if !ok || child.Text() != "hello" {
		t.Fatalf("unexpected scroll view child: %T %v", sv.Child(), child)
	}

	// Same view type at the same slot: widget reused, content updated.
	updated := root.update(ScrollView(Label("world")))
	if updated != sv {
		t.Fatal("scroll view at the same slot should be reused")
	}
	child = sv.Child().(*gui.Label)
	if child.Text() != "world" {
		t.Fatalf("child label was not updated: %q", child.Text())
	}
}

func TestScrollViewRemovesChild(t *testing.T) {
	root := newRoot()
	sv := root.update(ScrollView(Label("x"))).(*gui.ScrollView)
	if sv.Child() == nil {
		t.Fatal("child should be set")
	}

	root.update(ScrollView(nil))
	if sv.Child() != nil {
		t.Fatal("child should be removed when the slot is empty")
	}
}

func TestScrollViewWeightCanBeDisabledAndRestored(t *testing.T) {
	root := newRoot()
	sv := root.update(ScrollView(nil)).(*gui.ScrollView)
	if sv.MainWeight() != 1 {
		t.Fatal("unset weight overwrote the viewport default")
	}
	root.update(ScrollView(nil).MainWeight(0))
	if sv.MainWeight() != 0 {
		t.Fatal("explicit zero did not disable viewport expansion")
	}
	root.update(ScrollView(nil))
	if sv.MainWeight() != 1 {
		t.Fatal("removing the modifier did not restore the viewport default")
	}
}

func TestScrollViewFillsSingleStretchedHBoxViewport(t *testing.T) {
	root := newRoot()
	widget := root.update(HBox(ScrollView(Label("content"))).CrossAlign(layout.CrossStretch)).(*gui.LinearBox)

	widget.Measure(layout.Tight(geometry.Size{Width: 800, Height: 600}))
	widget.Arrange(geometry.Rect(0, 0, 800, 600))

	sv := widget.Children()[0].(*gui.ScrollView)
	if got := sv.Rect(); got != geometry.Rect(0, 0, 800, 600) {
		t.Fatalf("single ScrollView should fill an HBox viewport, got %v", got)
	}
}

func TestScrollViewSplitsTwoStretchedHBoxViewports(t *testing.T) {
	root := newRoot()
	widget := root.update(HBox(
		ScrollView(Label("left")),
		ScrollView(Label("right")),
	).CrossAlign(layout.CrossStretch)).(*gui.LinearBox)

	widget.Measure(layout.Tight(geometry.Size{Width: 800, Height: 600}))
	widget.Arrange(geometry.Rect(0, 0, 800, 600))

	children := widget.Children()
	if len(children) != 2 {
		t.Fatalf("expected two ScrollViews, got %d children", len(children))
	}
	if got := children[0].Rect(); got != geometry.Rect(0, 0, 400, 600) {
		t.Fatalf("left ScrollView should take half the viewport, got %v", got)
	}
	if got := children[1].Rect(); got != geometry.Rect(400, 0, 400, 600) {
		t.Fatalf("right ScrollView should take half the viewport, got %v", got)
	}
}

func TestScrollViewSplitsTwoVBoxViewportsByDefault(t *testing.T) {
	root := newRoot()
	widget := root.update(VBox(
		ScrollView(Image(image.NewRGBA(image.Rect(0, 0, 100, 20)))),
		ScrollView(Image(image.NewRGBA(image.Rect(0, 0, 100, 20)))),
	)).(*gui.LinearBox)

	widget.Measure(layout.Tight(geometry.Size{Width: 800, Height: 600}))
	widget.Arrange(geometry.Rect(0, 0, 800, 600))

	children := widget.Children()
	if len(children) != 2 {
		t.Fatalf("expected two ScrollViews, got %d children", len(children))
	}
	if children[0].Rect().Height != 300 || children[1].Rect().Height != 300 {
		t.Fatalf("VBox ScrollViews should split the height, got %v and %v", children[0].Rect(), children[1].Rect())
	}
	if children[0].Rect().Y != 0 || children[1].Rect().Y != 300 {
		t.Fatalf("VBox ScrollViews should be stacked, got %v and %v", children[0].Rect(), children[1].Rect())
	}
	if children[0].Rect().Width != 100 || children[1].Rect().Width != 100 {
		t.Fatalf("VBox ScrollViews should preserve their content widths, got %v and %v", children[0].Rect(), children[1].Rect())
	}
}

func scrollTargetImage(height int) View {
	return Image(image.NewRGBA(image.Rect(0, 0, 600, height)))
}

func arrangeScrollTarget(widget gui.Widget) {
	widget.Measure(layout.Tight(geometry.Size{Width: 240, Height: 160}))
	widget.Arrange(geometry.Rect(0, 0, 240, 160))
}

func TestScrollTargetReusesBindingsAndIndependentLayout(t *testing.T) {
	r := newRoot()
	defer r.update(nil)
	build := func() View {
		return HBox(
			ScrollView(scrollTargetImage(1000)).ID("source").HScrollBarOptions(ScrollBarOptions{Policy: ScrollBarHidden}).VScrollBarOptions(ScrollBarOptions{Policy: ScrollBarHidden}),
			ScrollBar(layout.DirectionVertical).Target("source").Thickness(14).ThumbThickness(6).MaxSize(14, 100).Style("external"),
			ScrollBar(layout.DirectionVertical).Target("source"),
		).Spacing(6).CrossAlign(layout.CrossStart)
	}
	widget := r.update(build())
	arrangeScrollTarget(widget)
	sv := widget.Children()[0].(*gui.ScrollView)
	bar := widget.Children()[1].(*gui.ScrollBar)
	other := widget.Children()[2].(*gui.ScrollBar)
	s := findNode(r.root, bar).state.(*scrollBarViewState)
	connection := s.binding
	if len(widget.Children()) != 3 || len(r.targets) != 2 {
		t.Fatal("Target 增加了布局 Widget 或关联丢失")
	}
	bar.Arrange(geometry.Rect(0, 0, 14, 100))
	sv.SetScrollY(60)
	if bar.Snapshot().Range.Value != 60 || other.Snapshot().Range.Value != 60 || bar.PageSize() != sv.ScrollGeometry().Viewport.Height || bar.StyleName() != "external" || bar.Thickness() != 14 || bar.ThumbThickness() != 6 {
		t.Fatal("Target 同步改变独立布局或样式")
	}
	for range 3 {
		r.update(build())
		arrangeScrollTarget(widget)
	}
	if s.binding != connection || widget.Children()[1] != bar || sv.ScrollY() != 60 {
		t.Fatal("普通重建重复绑定或重置偏移")
	}
	if len(widget.Snapshot().Children) != 3 || widget.Snapshot().Children[1].Role != gui.RoleScrollBar || widget.Snapshot().Children[1].Range == nil {
		t.Fatal("外部条没有出现在自己的语义树位置")
	}
	r.update(nil)
	sv.SetScrollY(80)
	if len(r.targets) != 0 || s.binding != nil || bar.Snapshot().Range.Max != 0 {
		t.Fatal("释放后仍保留关联")
	}
}

func TestScrollTargetLocalHostReplacement(t *testing.T) {
	version := MakeState(0)
	outerBuilds, sourceBuilds := 0, 0
	rt, fake := bindingApp(t, func() RootView {
		outerBuilds++
		return Window("main").Content(HBox(
			Build(func() View {
				sourceBuilds++
				v := version.Get()
				if v < 0 {
					return nil
				}
				return Modify(ScrollView(scrollTargetImage(1000)).ID(fmt.Sprintf("source-%d", v))).ID("source")
			}),
			ScrollBar(layout.DirectionVertical).Target("source").ID("external"),
		).CrossAlign(layout.CrossStretch))
	})
	host := rt.windows["main"].host
	root := host.Widget()
	arrangeScrollTarget(root)
	first := gui.FindWidget(host, "source").(*gui.ScrollView)
	bar := gui.FindWidget(host, "external").(*gui.ScrollBar)
	first.SetScrollY(50)
	if bar.Snapshot().Range.Value != 50 {
		t.Fatal("初始宿主未绑定")
	}
	version.Set(-1)
	fake.runPosted()
	if bar.Snapshot().Range.Max != 0 || bar.PageSize() != 0 {
		t.Fatal("局部移除后保留旧范围")
	}
	first.SetScrollY(90)
	if bar.Snapshot().Range.Value != 0 {
		t.Fatal("旧宿主继续控制外部条")
	}
	version.Set(1)
	fake.runPosted()
	arrangeScrollTarget(root)
	second := gui.FindWidget(host, "source").(*gui.ScrollView)
	second.SetScrollY(70)
	if outerBuilds != 1 || sourceBuilds != 3 || gui.FindWidget(host, "external") != bar || first == second || bar.Snapshot().Range.Value != 70 {
		t.Fatal("局部晚出现宿主需要重建外部条才能关联", outerBuilds, sourceBuilds)
	}
	version.Set(2)
	fake.runPosted()
	arrangeScrollTarget(root)
	third := gui.FindWidget(host, "source").(*gui.ScrollView)
	third.SetScrollY(100)
	second.SetScrollY(120)
	if bar.Snapshot().Range.Value != 100 || outerBuilds != 1 {
		t.Fatal("同轮替换没有按最终宿主关联")
	}
}

func TestScrollTargetIsolationAndAmbiguity(t *testing.T) {
	r := newRoot()
	defer r.update(nil)
	root := r.update(VBox(
		ScrollBar(layout.DirectionVertical),
		VBox(
			ScrollView(scrollTargetImage(1000)).ID("first"), ScrollBar(layout.DirectionVertical).Target("first"),
			VBox(ScrollView(scrollTargetImage(800)).ID("second"), ScrollBar(layout.DirectionVertical).Target("second")),
		),
	))
	outer := root.Children()[1]
	inner := outer.Children()[2]
	first, second := outer.Children()[0].(*gui.ScrollView), inner.Children()[0].(*gui.ScrollView)
	first.Arrange(geometry.Rect(0, 0, 100, 100))
	second.Arrange(geometry.Rect(0, 0, 100, 100))
	first.SetScrollY(40)
	second.SetScrollY(90)
	if outer.Children()[1].Snapshot().Range.Value != 40 || inner.Children()[1].Snapshot().Range.Value != 90 || root.Children()[0].Snapshot().Range.Max != 0 {
		t.Fatal("不同 Target 串联或无 Target 的条获得了范围")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("重复目标 ID 没有报告歧义")
			}
		}()
		r.update(VBox(VBox(ScrollView(nil).ID("same")), VBox(ScrollView(nil).ID("same")), ScrollBar(layout.DirectionVertical).Target("same")))
	}()
}

func TestScrollTargetHostTransfer(t *testing.T) {
	a, b := transferRoots()
	defer a.update(nil)
	defer b.update(nil)
	view := ScrollView(scrollTargetImage(1000)).ID("moved")
	source := a.update(VBox(view)).(*gui.LinearBox)
	target := b.update(VBox(ScrollBar(layout.DirectionVertical).Target("moved"))).(*gui.LinearBox)
	sv := source.Children()[0].(*gui.ScrollView)
	bar := target.Children()[0].(*gui.ScrollBar)
	coordinator := &Coordinator{owner: findNode(a.root, source)}
	if err := coordinator.TransferChild(sv, target, func() error {
		source.RemoveChild(sv)
		target.AddChild(sv)
		return nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	a.flushAfterUpdate()
	b.flushAfterUpdate()
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	sv.SetScrollY(65)
	if bar.Snapshot().Range.Value != 65 || len(a.targets) != 0 || len(b.targets) != 1 {
		t.Fatal("移交后没有刷新目标 Root 的引用")
	}
}

func TestScrollTargetLocalBarAxisChange(t *testing.T) {
	axis := MakeState(layout.DirectionVertical)
	rt, fake := bindingApp(t, func() RootView {
		return Window("main").Content(HBox(
			ScrollView(scrollTargetImage(1000)).ID("source"),
			Build(func() View { return ScrollBar(axis.Get()).Target("source").ID("bar") }),
		).CrossAlign(layout.CrossStretch))
	})
	host := rt.windows["main"].host
	arrangeScrollTarget(host.Widget())
	sv := gui.FindWidget(host, "source").(*gui.ScrollView)
	bar := gui.FindWidget(host, "bar").(*gui.ScrollBar)
	sv.SetScrollX(50)
	sv.SetScrollY(80)
	if bar.Snapshot().Range.Value != 80 {
		t.Fatal("竖向关联错误")
	}
	axis.Set(layout.DirectionHorizontal)
	fake.runPosted()
	if gui.FindWidget(host, "bar") != bar || bar.Snapshot().Range.Direction != layout.DirectionHorizontal || bar.Snapshot().Range.Value != 50 || bar.PageSize() != sv.ScrollGeometry().Viewport.Width {
		t.Fatal("局部改变方向没有保留控件并切换目标轴")
	}
	sv.SetScrollY(100)
	if bar.Snapshot().Range.Value != 50 {
		t.Fatal("旧轴仍控制滚动条")
	}
}

func TestScrollTargetSubtreeTransferPreservesConnection(t *testing.T) {
	a, b := transferRoots()
	defer a.update(nil)
	defer b.update(nil)
	source := a.update(VBox(HBox(ScrollView(scrollTargetImage(1000)).ID("source"), ScrollBar(layout.DirectionVertical).Target("source")).ID("pair"))).(*gui.LinearBox)
	target := b.update(VBox()).(*gui.LinearBox)
	subtree := source.Children()[0]
	sv := subtree.Children()[0].(*gui.ScrollView)
	bar := subtree.Children()[1].(*gui.ScrollBar)
	state := findNode(a.root, bar).state.(*scrollBarViewState)
	connection := state.binding
	coordinator := &Coordinator{owner: a.root}
	if err := coordinator.TransferChild(subtree, target, func() error { source.RemoveChild(subtree); target.AddChild(subtree); return nil }, nil); err != nil {
		t.Fatal(err)
	}
	a.flushAfterUpdate()
	b.flushAfterUpdate()
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	sv.SetScrollY(85)
	if bar.Snapshot().Range.Value != 85 || state.binding != connection {
		t.Fatal("整棵子树移交破坏既有关联")
	}
}
